# HRIS staging deploy (VPS `103.103.23.210` · `103-103-23-210.sslip.io`)

Deploys the full stack **isolated** alongside the legacy `lumen-*` production on
the shared box. **Footprint:** Postgres 16 · MinIO · Go API · Go worker (River) ·
web SPA. No Redis (River lives in Postgres). Mobile (Expo) is **not** server-deployed.

## Safety guarantees (this is a shared prod host)
- Own compose project `hris`, own network `hris-net`, `hris-*` names, `hris_*` volumes — **nothing references `lumen-*` or their MySQL/redis**.
- Every container port binds **`127.0.0.1` only** — no new public ports. Only the additive nginx vhost on `:443` is publicly reachable.
- `mem_limit` on every service; builds are memory-capped (`docker build --memory`) so they can't OOM the neighbours.
- Host nginx changes are **manual sudo, additive only** — `deploy.sh` never edits nginx/certbot.
- The legacy `lumen_mysql_prod` (our future E9 migration source) is never touched.

## Prereqs
- DNS: `103-103-23-210.sslip.io` → `103.103.23.210` (DuckDNS, done).
- Repo cloned on the VPS; user in `docker` + `sudo` groups (✓ `mightymig`).

## Run
```bash
cd <repo>/deploy
cp .env.prod.example .env.prod      # or let deploy.sh generate secrets
./deploy.sh
```
`deploy.sh`: generates `.env.prod` secrets (PG pw, MinIO secret, payroll key, Ed25519
JWT keypair) if absent → builds images (mem-capped) → starts PG+MinIO → runs
`migrate up` + `migrate river-up` + `seed` → starts api/worker/web → prints the
manual nginx/certbot steps.

### Manual host steps (sudo — printed by the script)
```bash
sudo cp deploy/nginx/hris.conf /etc/nginx/conf.d/hris.conf
sudo nginx -t && sudo nginx -s reload          # zero downtime; neighbours untouched
sudo certbot --nginx -d 103-103-23-210.sslip.io   # TLS for our host only
```

## Verify after deploy (open items — confirm, don't assume)
1. **API health** — liveness is `GET /healthz` (root-level, confirmed in
   `internal/server/server.go`); checked at `http://127.0.0.1:8090/healthz`.
2. **MinIO presigned URLs** (E2 profile photo, E5 selfie) — the trickiest bit.
   Browser hits `https://103-103-23-210.sslip.io/hris-private/<key>?X-Amz-...`,
   nginx proxies to `127.0.0.1:9100`. SigV4 signs host+path+query (not scheme),
   so the path-through vhost should validate — **but test an upload + fetch**. If
   it fails, tune `STORAGE_PUBLIC_ENDPOINT` / `STORAGE_USE_SSL` handling. Core app
   (auth, dashboards, CRUD) does NOT depend on this.
3. **`PAYROLL_ENCRYPTION_KEY`** — generated as base64 32B; verify the expected
   length/encoding in `internal/platform/config` before trusting payroll fields.
4. **CORS / cookies** — `AUTH_COOKIE_SECURE=true` needs TLS live first; same-origin
   (`/api/v1` via nginx) means CORS shouldn't trigger, but `CORS_ALLOWED_ORIGINS`
   is set to the https origin as a backstop.

## Operate
```bash
docker compose -p hris -f docker-compose.prod.yml ps
docker compose -p hris -f docker-compose.prod.yml logs -f api worker
docker compose -p hris -f docker-compose.prod.yml restart api
```

## Update (redeploy a new build)
```bash
git pull && ./deploy.sh        # rebuilds images, recreates api/worker/web
```
(`seed` re-runs on each `deploy.sh`; if that's undesirable after first load,
comment the `seed` line in the one-shot `up` in `deploy.sh`.)

## Teardown (removes our stack only; leaves lumen-* alone)
```bash
docker compose -p hris -f docker-compose.prod.yml down            # keep data
docker compose -p hris -f docker-compose.prod.yml down -v         # + delete volumes
sudo rm /etc/nginx/conf.d/hris.conf && sudo nginx -s reload       # drop the vhost
```
