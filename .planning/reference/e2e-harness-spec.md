# E2E harness spec (Playwright, full-stack FE↔BE)

Built in **Phase 1** and reused by every later phase. Tests the real web app against the
real Go API + a real (ephemeral) Postgres — no MSW, no mocks. This is the contract-drift
safety net.

## Location & tooling
- New workspace package: `frontend/e2e/` (pnpm workspace member, `@swp/e2e`).
- Playwright (`@playwright/test`). TypeScript. Reuses `@swp/api-client` types where useful.
- Test files: `frontend/e2e/tests/<epic>/<feature>.spec.ts`, one `test()` per Gherkin
  scenario / case so each is individually runnable in the Playwright UI.

## Required npm scripts (user explicitly wants these)
In `frontend/e2e/package.json` (and surfaced via root/turbo where convenient):
- `"test:e2e"` → `playwright test` (HEADLESS — CI default)
- `"test:e2e:headed"` → `playwright test --headed` (HEADFUL)
- `"test:e2e:ui"` → `playwright test --ui` (Playwright UI mode — run/inspect each case)
- `"test:e2e:debug"` → `playwright test --debug`
- `"test:e2e:report"` → `playwright show-report`
Also add a root convenience: `pnpm e2e`, `pnpm e2e:ui`, `pnpm e2e:headed`.

## playwright.config.ts
- `projects`: at least `chromium` (headless). Headed/UI are driven by the CLI flags above
  (not separate projects) so the same specs run in all three modes. Add a `headed-chromium`
  project only if a test must be visual.
- `webServer`: start the **FE preview** with MSW OFF, pointed at the test API:
  - `VITE_ENABLE_MSW=false`, `VITE_API_BASE_URL=http://localhost:8081/api/v1`
  - command: `pnpm --filter @swp/web preview --port 4173` (or `dev`), `url: http://localhost:4173`, `reuseExistingServer: !process.env.CI`.
- `globalSetup` / `globalTeardown`: bring up the backend stack (below).
- `use.baseURL = http://localhost:4173`. Trace/screenshot/video on first retry.
- DB isolation between specs (see "Isolation").

## globalSetup — boot the backend stack
1. Start an **ephemeral Postgres** (port 5433 to avoid clashing with dev 5432):
   - Prefer testcontainers-go is Go-side; for Playwright use a dedicated
     `docker compose -f frontend/e2e/docker-compose.e2e.yml up -d` (postgres:16) OR
     `docker run` a throwaway. Wait for healthy.
2. Run goose migrations against it: `cd backend && DATABASE_URL=... go run ./cmd/migrate up`.
3. Run River migrations: `river migrate-up --database-url ...` (or programmatic).
4. **Seed** the demo personas + minimal data (see Seeding). Use a Go seed command
   (add `backend/cmd/seed` in Phase 1) run with the test `DATABASE_URL`.
5. Build + start the **Go API** on port 8081 with the test env (test JWT keys, test DB,
   `CORS_ALLOWED_ORIGINS=http://localhost:4173`, `AUTH_COOKIE_SECURE=false`). Wait for `/healthz`.
6. Start the **worker** (`go run ./cmd/worker`) if a test exercises async (notifications/exports).
- globalTeardown: stop API/worker, tear down the Postgres container.

## Seeding (`backend/cmd/seed`)
Seed deterministic fixtures aligned to the spec examples (CONVENTIONS §20 personas):
- Users (one per role, known passwords for login):
  - `sari.hadi@swp.test` — **hr_admin** (Sari Hadi)
  - `rudi.wijaya@swp.test` — **shift_leader** scoped to a client company "Plaza Senayan"
  - a **super_admin** and an **agent** persona too.
- Minimal supporting data so each epic's list/detail screens render: a client company +
  site (Plaza Senayan), a service line + position, an employee, a placement, a shift master,
  some attendance/leave/overtime rows, a payslip, notifications. Each phase extends the seed
  with what its screens need (keep one seed module, grow it per phase).

## Isolation
- Default: truncate-and-reseed (or wrap each spec in a transaction-per-worker / reset
  between files) so specs are independent and parallel-safe. Cheapest robust option:
  a `resetDb()` helper called in `beforeEach`/`globalSetup` that truncates app tables and
  re-applies the seed. Document the choice in the harness README.

## Auth in tests
- A `loginAs(page, persona)` fixture: drives the real login screen (fills email/password,
  submits, waits for dashboard) OR calls `POST /auth/login` and injects the token — prefer
  driving the UI for true E2E, with a fast API-login fixture for non-auth-focused specs.
- Store storageState per persona to speed up the many specs.

## Coverage (decision: EXHAUSTIVE per Gherkin AC)
For every FE feature in scope, translate **every Gherkin scenario and edge case (C-#)**
in the epic's PRD into its own `test()` — happy paths, RBAC/scope (401/403/OUT_OF_SCOPE),
validation (400/422 field errors), conflict (409 INV_*), empty/loading states. Name each
test with its scenario/BR-#/C-# so it's traceable and individually runnable in `--ui`.

## Definition of done (per phase)
The epic's FE screens drive the real BE green in headless mode; `test:e2e:ui` lists each
scenario as its own runnable case; new specs are committed under `frontend/e2e/tests/<epic>/`.
