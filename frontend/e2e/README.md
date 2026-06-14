# @swp/e2e — Full-Stack Playwright E2E Harness

Full-stack Playwright test harness for the SWP HRIS web console. Runs the **real** Vite dev server against the **real** Go API and an **ephemeral** Postgres — no MSW, no mocks.

## Quick start

Docker must be running. From `frontend/`:

```bash
pnpm e2e          # headless (CI default)
pnpm e2e:headed   # headed — see the browser
pnpm e2e:ui       # Playwright UI — inspect / re-run individual tests
```

Or, from `frontend/e2e/` directly:

```bash
pnpm test:e2e
pnpm test:e2e:headed
pnpm test:e2e:ui
pnpm test:e2e:debug   # pause + step through a test
pnpm test:e2e:report  # open the last HTML report
```

## Ports

| Service | Port | Notes |
|---|---|---|
| Vite dev server (FE) | 4173 | `VITE_ENABLE_MSW=false`, `VITE_API_BASE_URL=http://localhost:8081/api/v1` |
| Go API (BE) | 8081 | Test env — separate from the dev API on :8080 |
| Ephemeral Postgres | 5433 | `hris_e2e` DB — separate from dev DB on :5432 |

## Boot sequence (`globalSetup`)

1. `docker compose -f docker-compose.e2e.yml up -d` — starts `postgres:16` on :5433 (`tmpfs`, ephemeral)
2. Polls `pg_isready` until healthy (timeout 60 s)
3. `go run ./cmd/migrate up` — applies goose schema migrations
4. `river migrate-up --database-url <url>` — applies River queue tables *(graceful fallback if `river` CLI is absent)*
5. `go run ./cmd/seed` — upserts the four deterministic test personas
6. `go run ./cmd/seed -genkeys` — generates a fresh Ed25519 keypair (ENV-only, never written to disk)
7. `go run ./cmd/api` — starts the Go API on :8081 with the test env + generated keys
8. Polls `GET http://localhost:8081/healthz` until 200 (timeout 60 s)
9. Playwright `webServer` starts `pnpm --filter @swp/web dev --port 4173` with MSW disabled

**Teardown** (`globalTeardown`): SIGTERM the API process → `docker compose … down -v` removes the container and volumes.

## webServer: `dev` instead of `preview`

The config uses `vite dev` (not `vite preview`) to avoid a build step before each run. The dev server reads `VITE_*` environment variables directly at startup, giving us MSW off and the right API base URL without pre-building. This is the right trade-off for a full-stack harness; documented here per `01-CONTEXT.md §Decisions`.

## DB isolation — truncate-and-reseed

**Chosen mechanism:** Each spec file that mutates state calls `resetDb()` in `beforeEach` or `beforeAll`. `resetDb()` (`lib/reset-db.ts`):

1. Connects to the test Postgres via the `pg` npm package
2. Runs `TRUNCATE ... CASCADE` on all app tables (`refresh_tokens`, `idempotency_keys`, `audit_log`, `users`)
3. Re-runs `go run ./cmd/seed` — which is idempotent (upserts personas, skips existing)

**Why not per-worker transactions?** Playwright specs drive a real HTTP server. Wrapping HTTP-driven DB writes in a test transaction requires test-only server hooks, which would pollute production code. Truncate-and-reseed is the cheapest robust option at this scale.

**Why include `users`?** The seed is idempotent (skip-if-exists), so truncating then reseeding always restores all personas. This handles test-deleted users correctly.

## StorageState caching

`lib/fixtures.ts` exports `storageStateFor(personaKey)` returning a path under `frontend/e2e/.auth/<key>.json`.

- **httpOnly refresh cookies ARE captured** in storageState — the cookie transport used by the web app.
- **The in-memory access token is NOT captured** — it lives only in `lib/auth.ts` module memory in the browser.

For non-auth-focused specs that need a post-login starting point:

```ts
test.use({ storageState: storageStateFor('hrAdmin') });
```

Because the access token isn't in storageState, the app will call `/auth/refresh` on the next page load (the httpOnly refresh cookie IS present). Ensure the BE is running and the cookie is valid.

## Test personas

Seeded by `backend/cmd/seed`. Passwords match the constants in `backend/cmd/seed/seed.go`:

| Key | Email | Password | Role |
|---|---|---|---|
| `hrAdmin` | sari.hadi@swp.test | `Pass1ng-Garuda!` | hr_admin |
| `shiftLeader` | rudi.wijaya@swp.test | `Lead3r-Senayan!` | shift_leader (Plaza Senayan) |
| `superAdmin` | super.admin@swp.test | `Sup3r-Admin-2026!` | super_admin |
| `agent` | agent.budi@swp.test | `Ag3nt-Budi-2026!` | agent |

## Writing specs

```ts
import { test, expect } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';

test('US-1 | BR-1: login with valid credentials navigates to dashboard', async ({ page, loginAs }) => {
  await loginAs('hrAdmin');
  await expect(page).toHaveURL('/');
});
```

Place specs at `frontend/e2e/tests/<epic>/<feature>.spec.ts`, one `test()` per Gherkin scenario or edge case so each is individually runnable in `--ui` mode.

## Prerequisites

- Docker running (for the ephemeral Postgres)
- Go 1.23 on PATH (for `go run ./cmd/*`)
- `river` CLI on PATH (optional — fallback logs a warning if absent)
- Node ≥ 22 + pnpm 9

## CI notes

- `CI=true` → `retries: 1`, `reuseExistingServer: false`
- The `docker-compose.e2e.yml` uses `tmpfs` so the container data is in RAM — fast for CI
- Run `pnpm --filter @swp/e2e exec playwright install chromium` if the browser is missing
