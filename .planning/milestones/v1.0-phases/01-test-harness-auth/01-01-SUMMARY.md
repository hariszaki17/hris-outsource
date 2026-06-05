---
phase: 01-test-harness-auth
plan: "01"
subsystem: testing
tags: [playwright, e2e, typescript, docker, postgres, go, vite]

# Dependency graph
requires: []
provides:
  - "@swp/e2e pnpm workspace package with five test:e2e* scripts + root e2e/e2e:ui/e2e:headed convenience scripts"
  - "playwright.config.ts: chromium project, baseURL :4173, MSW off, API :8081, globalSetup/Teardown, trace/screenshot/video on first retry"
  - "docker-compose.e2e.yml: postgres:16 on host port 5433 (tmpfs, ephemeral)"
  - "globalSetup: boots PG:5433 → goose migrate → River migrate → seed → go run ./cmd/api → waits /healthz"
  - "globalTeardown: SIGTERM API + docker compose down -v"
  - "lib/backend.ts: startBackend()/stopBackend() — the full boot/teardown orchestration"
  - "lib/reset-db.ts: resetDb() — TRUNCATE app tables + reseed (truncate-and-reseed isolation)"
  - "lib/personas.ts: PERSONAS map (4 personas with exact seed passwords)"
  - "lib/fixtures.ts: loginAs(page, persona) driving #email/#password on real login screen, storageStateFor() helper"
  - "tests/smoke/harness.spec.ts: smoke test discovered by playwright test --list"
  - "frontend/e2e/README.md: full harness documentation"
affects:
  - 01-02
  - 01-03
  - 01-04
  - 01-05
  - all subsequent phase E2E plans

# Tech tracking
tech-stack:
  added:
    - "@playwright/test ^1.49.0"
    - "pg ^8 (for DB truncate in reset-db.ts)"
    - "@types/pg ^8"
    - "dotenv ^16"
    - "@types/node ^22"
  patterns:
    - "globalSetup/Teardown as the backend stack lifecycle manager"
    - "truncate-and-reseed as the DB isolation strategy (not per-worker transactions)"
    - "typed PERSONAS map with passwords matching backend seed constants"
    - "loginAs() drives the real login screen (true E2E), storageState caches the httpOnly refresh cookie"

key-files:
  created:
    - frontend/e2e/package.json
    - frontend/e2e/tsconfig.json
    - frontend/e2e/docker-compose.e2e.yml
    - frontend/e2e/.env.e2e
    - frontend/e2e/playwright.config.ts
    - frontend/e2e/global-setup.ts
    - frontend/e2e/global-teardown.ts
    - frontend/e2e/lib/backend.ts
    - frontend/e2e/lib/reset-db.ts
    - frontend/e2e/lib/personas.ts
    - frontend/e2e/lib/fixtures.ts
    - frontend/e2e/tests/smoke/harness.spec.ts
    - frontend/e2e/README.md
  modified:
    - frontend/pnpm-workspace.yaml (added "e2e" member)
    - frontend/package.json (added e2e / e2e:ui / e2e:headed root scripts)

key-decisions:
  - "webServer uses `vite dev` (not `vite preview`) to avoid a build step in globalSetup; dev server reads VITE_* env vars at startup"
  - "DB isolation: TRUNCATE app tables + reseed (not per-worker transactions — incompatible with real HTTP server)"
  - "storageStateFor caches httpOnly refresh cookie; access token is in-memory so non-auth specs must re-login or rely on /auth/refresh after page load"
  - "River migrations: best-effort with graceful fallback — warning logged if `river` CLI absent (async not exercised in Phase 1)"
  - "Ed25519 keypair generated fresh per run via `go run ./cmd/seed -genkeys` — never stored on disk"

patterns-established:
  - "Boot sequence: compose up → pg_isready poll → migrate → River migrate → seed → genkeys → go run ./cmd/api → /healthz poll"
  - "lib/backend.ts is the single source of truth for backend lifecycle; globalSetup/Teardown are thin wrappers"
  - "Persona passwords in personas.ts must stay in sync with backend/cmd/seed/seed.go constants"
  - "One test() per Gherkin scenario (individually runnable in --ui mode)"

requirements-completed: [HARN-01]

# Metrics
duration: 5min
completed: 2026-06-04
---

# Phase 1 Plan 01: E2E Harness Skeleton Summary

**Playwright E2E harness with Vite dev server (MSW off) wired to Go API :8081 + ephemeral Postgres :5433; globalSetup boots the full backend stack (goose + River + seed + /healthz); loginAs fixture drives the real login screen; smoke test discoverable in `--ui`**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-06-03T23:08:47Z
- **Completed:** 2026-06-04T00:00:46Z
- **Tasks:** 3
- **Files created:** 13, Files modified: 2

## Accomplishments

- New `@swp/e2e` pnpm workspace package with all 5 required scripts (`test:e2e`, `test:e2e:headed`, `test:e2e:ui`, `test:e2e:debug`, `test:e2e:report`) and root convenience scripts (`pnpm e2e`, `pnpm e2e:ui`, `pnpm e2e:headed`)
- `playwright.config.ts` wires the real Vite dev server (port 4173, `VITE_ENABLE_MSW=false`, `VITE_API_BASE_URL=http://localhost:8081/api/v1`) to the real Go API via globalSetup/Teardown
- `lib/backend.ts` implements the full boot sequence: PG:5433 → goose migrate → River migrate (graceful fallback) → seed → Ed25519 genkeys → `go run ./cmd/api` → poll `/healthz`
- `lib/reset-db.ts` provides `resetDb()` for per-spec isolation via TRUNCATE + reseed
- `lib/personas.ts` + `lib/fixtures.ts`: typed persona registry with exact seed passwords, `loginAs()` drives the real login screen (`#email`/`#password`/submit)
- Smoke spec discovered by `playwright test --list`; `tsc --noEmit` exits 0

## Task Commits

1. **Task 1: Scaffold @swp/e2e package** — `2af02ad` (feat)
2. **Task 2: playwright.config + globalSetup/Teardown + DB-reset** — `91d0bf4` (feat)
3. **Task 3: Personas, loginAs fixture, smoke spec, README** — `a1fd86a` (feat)

## Run Commands

```bash
# From frontend/
pnpm e2e            # headless CI
pnpm e2e:headed     # headed
pnpm e2e:ui         # Playwright UI mode

# From frontend/e2e/
pnpm test:e2e
pnpm test:e2e:ui
pnpm test:e2e:headed
pnpm test:e2e:debug
pnpm test:e2e:report
```

## Boot Sequence (globalSetup)

1. `docker compose -f docker-compose.e2e.yml up -d` (postgres:16, port 5433, tmpfs)
2. Poll `pg_isready` until healthy (timeout 60s)
3. `go run ./cmd/migrate up` in `backend/`
4. `river migrate-up --database-url <url>` (best-effort; warns if `river` CLI absent)
5. `go run ./cmd/seed` in `backend/` (idempotent upsert of 4 personas)
6. `go run ./cmd/seed -genkeys` → parse stdout line 1 = `AUTH_JWT_PRIVATE_KEY`, line 2 = `AUTH_JWT_PUBLIC_KEY`
7. Spawn `go run ./cmd/api` with test env + generated keys (HTTP_ADDR=:8081)
8. Poll `GET http://localhost:8081/healthz` until 200 (timeout 60s)

## -genkeys Contract (assumed from 01-02)

`go run ./cmd/seed -genkeys` outputs exactly two lines to stdout — verified from `backend/cmd/seed/main.go`:
- Line 1: `AUTH_JWT_PRIVATE_KEY` (base64 std, 64-byte Ed25519 private key)
- Line 2: `AUTH_JWT_PUBLIC_KEY`  (base64 std, 32-byte Ed25519 public key)

## Persona Passwords

| Key | Email | Password | Role |
|---|---|---|---|
| hrAdmin | sari.hadi@swp.test | Pass1ng-Garuda! | hr_admin |
| shiftLeader | rudi.wijaya@swp.test | Lead3r-Senayan! | shift_leader (Plaza Senayan) |
| superAdmin | super.admin@swp.test | Sup3r-Admin-2026! | super_admin |
| agent | agent.budi@swp.test | Ag3nt-Budi-2026! | agent |

## DB Isolation Mechanism

**Chosen:** TRUNCATE app tables + reseed via `go run ./cmd/seed` (idempotent).

Per-worker transactions rejected: requires test-only server hooks that would pollute production code. Truncate-and-reseed is the cheapest robust option for a real-HTTP-server harness.

## Files Created/Modified

- `frontend/e2e/package.json` — @swp/e2e package with 5 scripts
- `frontend/e2e/tsconfig.json` — ESNext/Bundler/strict TypeScript
- `frontend/e2e/docker-compose.e2e.yml` — postgres:16 on 5433, tmpfs
- `frontend/e2e/.env.e2e` — test stack env vars
- `frontend/e2e/playwright.config.ts` — main Playwright config
- `frontend/e2e/global-setup.ts` — delegates to startBackend()
- `frontend/e2e/global-teardown.ts` — delegates to stopBackend()
- `frontend/e2e/lib/backend.ts` — full boot/teardown orchestration
- `frontend/e2e/lib/reset-db.ts` — TRUNCATE + reseed isolation helper
- `frontend/e2e/lib/personas.ts` — PERSONAS map with typed Persona interface
- `frontend/e2e/lib/fixtures.ts` — loginAs() + storageStateFor() + extended test
- `frontend/e2e/tests/smoke/harness.spec.ts` — smoke spec
- `frontend/e2e/README.md` — harness documentation
- `frontend/pnpm-workspace.yaml` — added "e2e" member
- `frontend/package.json` — added e2e / e2e:ui / e2e:headed root scripts

## Decisions Made

- **webServer uses `vite dev` not `vite preview`** — avoids a build step; dev server reads VITE_* env vars at startup without pre-building. Documented in README.
- **DB isolation: truncate-and-reseed** — per-worker transactions incompatible with real HTTP server without test-only hooks.
- **storageState caches httpOnly refresh cookie only** — access token is in-memory; specs needing API calls must re-login or rely on /auth/refresh after page navigation.
- **River migrations: best-effort** — logs warning if `river` CLI absent; async not exercised in Phase 1.
- **Ed25519 keypair: generated fresh per run** — never stored on disk; parsed from `go run ./cmd/seed -genkeys` stdout.

## Deviations from Plan

None — plan executed exactly as written. The `cmd/seed` command and its `-genkeys` flag already existed in the codebase (built as part of the backend foundation); no implementation was needed on that side.

## Issues Encountered

None.

## Next Phase Readiness

- `frontend/e2e` harness is complete and ready for plan 01-05 (auth E2E specs)
- Depends at runtime on `cmd/seed` (already present) and the Go API auth endpoints (delivered by 01-02 through 01-04)
- `playwright test --list` discovers the smoke spec in all three modes
- `tsc --noEmit` is clean

---
*Phase: 01-test-harness-auth*
*Completed: 2026-06-04*
