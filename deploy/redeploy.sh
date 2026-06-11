#!/usr/bin/env bash
# Redeploy helper — run LOCALLY from the repo. Syncs the working tree to the VPS
# and re-runs deploy.sh (rebuilds images, recreates api/worker/web, re-applies
# migrations). Idempotent. Preserves .env.prod (secrets), the nginx vhost, and
# the TLS cert — those are never touched.
#
#   ./deploy/redeploy.sh
#
# Override host/key via env if needed:
#   VPS=user@host KEY=~/.ssh/key ./deploy/redeploy.sh
set -euo pipefail
VPS="${VPS:-mightymig@103.103.23.210}"
KEY="${KEY:-$HOME/.ssh/id_ed25519_staging}"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "==> rsync $REPO_ROOT -> $VPS:~/hris/"
rsync -az \
  --exclude '.git' --exclude 'node_modules' --exclude 'dist' --exclude '.turbo' \
  --exclude '.vite' --exclude 'coverage' --exclude 'test-results' \
  --exclude 'playwright-report' --exclude '*.env.local' --exclude '.env.development.local' \
  --exclude 'docs/design' --exclude '.env.prod' \
  --exclude 'apps/mobile/ios' --exclude 'apps/mobile/android' --exclude '.expo' \
  -e "ssh -i $KEY -o IdentitiesOnly=yes" \
  "$REPO_ROOT/" "$VPS:~/hris/"

echo "==> remote deploy.sh"
ssh -i "$KEY" -o IdentitiesOnly=yes "$VPS" 'cd ~/hris/deploy && bash ./deploy.sh'
