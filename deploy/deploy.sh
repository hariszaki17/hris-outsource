#!/usr/bin/env bash
# HRIS staging deploy — run ON the VPS, from the repo's deploy/ dir.
# Scope: the ISOLATED docker stack only (build + secrets + compose up + migrate
# + seed). It deliberately does NOT touch the host nginx or certbot — those are
# manual sudo steps (printed at the end) so changes to the shared box are
# deliberate and reviewed. Safe to re-run: existing .env.prod is preserved.
set -euo pipefail
cd "$(dirname "$0")"

COMPOSE="docker compose -p hris --env-file .env.prod -f docker-compose.prod.yml"

echo "==> 1/5  Secrets (.env.prod)"
if [[ -f .env.prod ]]; then
  echo "    .env.prod exists — preserving (delete it to regenerate)."
else
  echo "    generating .env.prod ..."
  PG_PW=$(openssl rand -hex 16)
  MINIO_PW=$(openssl rand -hex 16)
  PAYROLL_KEY=$(openssl rand -base64 32)
  # Ed25519 keys as base64-STD of the RAW bytes, matching auth/jwt.go exactly.
  KEYS=$(docker run --rm golang:1.23-alpine sh -c '
    cat > /tmp/k.go <<EOF
package main
import ("crypto/ed25519";"crypto/rand";"encoding/base64";"fmt")
func main(){ pub,priv,_:=ed25519.GenerateKey(rand.Reader)
 fmt.Println(base64.StdEncoding.EncodeToString(priv))
 fmt.Println(base64.StdEncoding.EncodeToString(pub)) }
EOF
    go run /tmp/k.go')
  JWT_PRIV=$(echo "$KEYS" | sed -n 1p)
  JWT_PUB=$(echo "$KEYS"  | sed -n 2p)

  sed \
    -e "s#__CHANGE_ME__#${PG_PW}#g" \
    -e "s#postgres://hris:${PG_PW}@#postgres://hris:${PG_PW}@#" \
    .env.prod.example > .env.prod
  # The example uses the same __CHANGE_ME__ token for PG pw, DSN, payroll, minio;
  # set the distinct ones explicitly so they don't collide.
  python3 - "$PG_PW" "$MINIO_PW" "$PAYROLL_KEY" "$JWT_PRIV" "$JWT_PUB" <<'PY'
import sys,re
pg,minio,payroll,priv,pub=sys.argv[1:6]
s=open(".env.prod").read()
s=re.sub(r"^POSTGRES_PASSWORD=.*$", f"POSTGRES_PASSWORD={pg}", s, flags=re.M)
s=re.sub(r"^DATABASE_URL=.*$", f"DATABASE_URL=postgres://hris:{pg}@postgres:5432/hris?sslmode=disable", s, flags=re.M)
s=re.sub(r"^STORAGE_SECRET_KEY=.*$", f"STORAGE_SECRET_KEY={minio}", s, flags=re.M)
s=re.sub(r"^PAYROLL_ENCRYPTION_KEY=.*$", f"PAYROLL_ENCRYPTION_KEY={payroll}", s, flags=re.M)
s=re.sub(r"^AUTH_JWT_PRIVATE_KEY=.*$", f"AUTH_JWT_PRIVATE_KEY={priv}", s, flags=re.M)
s=re.sub(r"^AUTH_JWT_PUBLIC_KEY=.*$", f"AUTH_JWT_PUBLIC_KEY={pub}", s, flags=re.M)
open(".env.prod","w").write(s)
PY
  chmod 600 .env.prod
  echo "    .env.prod written (chmod 600)."
fi

echo "==> 2/5  Build images (memory-capped — protects neighbour prod)"
docker build -t hris-api:local --memory=1g --memory-swap=2g \
  -f ../backend/Dockerfile ../backend
docker build -t hris-web:local --memory=2g --memory-swap=3g \
  --build-arg VITE_API_BASE_URL=/api/v1 \
  -f ../frontend/apps/web/Dockerfile ../frontend

echo "==> 3/5  Start data services + run migrations + seed (one-shots)"
$COMPOSE up -d postgres minio
$COMPOSE up --no-build minio-init migrate migrate-river seed

echo "==> 4/5  Start app services"
$COMPOSE up -d --no-build api worker web

echo "==> 5/5  Status"
$COMPOSE ps
echo
echo "    api  : curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8090/healthz"
echo "    web  : curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8091/"
echo
cat <<'NEXT'
=========================================================================
DOCKER STACK UP (127.0.0.1 only). Remaining MANUAL host steps (sudo) —
these touch the SHARED nginx, so run them deliberately:

  sudo cp deploy/nginx/hris.conf /etc/nginx/conf.d/hris.conf
  sudo nginx -t                 # MUST pass before reload
  sudo nginx -s reload          # zero-downtime; neighbour vhosts untouched
  sudo certbot --nginx -d swp-hris.duckdns.org   # TLS for our host only

Then verify: https://swp-hris.duckdns.org
=========================================================================
NEXT
