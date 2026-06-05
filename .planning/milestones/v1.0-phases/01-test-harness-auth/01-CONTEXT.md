# Phase 1: Test Harness + Auth - Context

**Gathered:** 2026-06-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Stand up a reusable **full-stack Playwright E2E harness** (real web app ↔ real Go API ↔
ephemeral Postgres, MSW off) and working **real authentication** so the web app can log in
against the backend. Deliverables: the `frontend/e2e` test package + config + backend-stack
boot (migrate/seed), a `backend/cmd/seed` command, the auth endpoints the FE calls
(login/refresh/logout/me already scaffolded; add forgot-password + reset-password), FE auth
wiring (remove the dev-token stub), and exhaustive auth E2E. This phase establishes the
patterns every later phase reuses; it does NOT implement any non-auth epic endpoints.

</domain>

<decisions>
## Implementation Decisions

### Harness topology & modes
- New pnpm workspace package `frontend/e2e` (`@swp/e2e`), Playwright + TypeScript.
- Specs at `frontend/e2e/tests/<epic>/<feature>.spec.ts`, one `test()` per Gherkin scenario so each is individually runnable.
- npm scripts (user requirement): `test:e2e` (headless), `test:e2e:headed` (headful), `test:e2e:ui` (`playwright test --ui`), `test:e2e:debug`, `test:e2e:report`; root convenience `pnpm e2e`, `pnpm e2e:ui`, `pnpm e2e:headed`.
- `playwright.config.ts`: chromium project; headed/UI via CLI flags (same specs run in all modes); `webServer` starts FE preview (`VITE_ENABLE_MSW=false`, `VITE_API_BASE_URL=http://localhost:8081/api/v1`) on 4173; `globalSetup`/`globalTeardown` boot the backend stack; `baseURL=http://localhost:4173`; trace/screenshot/video on first retry.
- `globalSetup`: ephemeral Postgres (port 5433 via `frontend/e2e/docker-compose.e2e.yml`, postgres:16) → goose migrate → River migrate → seed → start Go API on 8081 (test env, test JWT keys, CORS allow 4173, `AUTH_COOKIE_SECURE=false`) → wait `/healthz`; start worker if async tested. Teardown stops services + container.

### Seed data (`backend/cmd/seed`)
- Seed deterministic personas with known passwords for login: `sari.hadi@swp.test` (hr_admin), `rudi.wijaya@swp.test` (shift_leader scoped to client company "Plaza Senayan"), plus a super_admin and an agent.
- Seed minimal supporting data so screens render; later phases extend the same seed module.
- Add a dev Ed25519 keygen helper (e.g. `cmd/api genkeys` or a seed flag) using `auth.GenerateKeypair()`.

### Auth endpoints & transport
- Reuse the scaffolded identity slice: `POST /auth/login`, `/auth/refresh`, `/auth/logout`, `GET /auth/me`. ADD `POST /auth/forgot-password` and `POST /auth/reset-password` per E1 spec.
- Match `docs/api/E1-foundations/openapi.yaml` request/response shapes exactly (the FE client is generated from it).
- Web uses the cookie refresh transport (default, SameSite=Lax). Tests drive the REAL login screen; provide a `loginAs(page, persona)` fixture + per-persona storageState for speed.
- FE: wire `features/auth/login-screen.tsx` (+ forgot/reset screens) to the real `@swp/api-client` hooks; remove the dev-token stub; `installAuth` points at the real BE.

### Auth E2E coverage (exhaustive)
- Translate every Gherkin scenario / case in `docs/epics/E1-foundations/prds/authentication.md` into its own `test()`: successful login → dashboard; wrong credentials (`INVALID_CREDENTIALS`); disabled account (`ACCOUNT_DISABLED`); token refresh; logout; forgot-password + reset-password flows; RBAC/unauthenticated redirect (`UNAUTHENTICATED` → re-auth). Name each test with its scenario/BR-#/C-#.

### Claude's Discretion
- Exact DB-reset mechanism (truncate-and-reseed vs per-worker tx) — pick the cheapest robust option, document it in the harness README.
- Test fixture/file structure details; storageState caching strategy; how the API/worker processes are spawned in globalSetup.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & build rules (read first)
- `.planning/reference/e2e-harness-spec.md` — the exact Playwright harness contract (modes, webServer, globalSetup, seeding, isolation, coverage).
- `.planning/reference/backend-build-conventions.md` — per-endpoint recipe, hard rules, definition of done.
- `.planning/reference/fe-endpoint-inventory.md` — Auth section = the endpoints in scope.

### Auth contract
- `docs/api/E1-foundations/openapi.yaml` — auth ops at lines /auth/login (63), /logout (159), /refresh (177), /forgot-password (225), /reset-password (274), /me (344).
- `docs/api/CONVENTIONS.md` §3 (authentication, 401 vs 403), §11 (error envelope + codes).
- `docs/epics/E1-foundations/prds/authentication.md` — Gherkin AC + cases (the E2E test source).
- `docs/epics/E1-foundations/prds/rbac-roles.md`, `docs/epics/E1-foundations/FEATURE.md` — roles/scope.

### Reference implementation (copy its shape)
- `backend/README.md` — stack + layout.
- `backend/internal/{handler,service,repository}/identity` + `internal/platform/auth` — the auth slice.
- `frontend/apps/web/src/features/auth/*`, `frontend/apps/web/src/lib/auth.ts`, `frontend/apps/web/src/main.tsx` (MSW toggle), `frontend/packages/api-client/src/e1.ts` (auth hooks).
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Backend: `internal/platform/*` kernel (auth/jwt, refresh, password argon2id, apperr→envelope, rbac, httpx, db.TxManager, ids, i18n, jobs); identity slice already implements login/refresh/logout/me.
- `cmd/migrate` (goose, embedded), `cmd/worker` (River), `Makefile` (gen/migrate/run), `docker-compose.yml` (dev Postgres).
- Frontend: Orval client `@swp/api-client` (auth hooks generated), `lib/auth.ts` thin abstraction with `installAuth({baseUrl,getToken,onUnauthenticated})`, `VITE_ENABLE_MSW`/`VITE_API_BASE_URL` toggles, MSW `src/mocks/browser.ts`.

### Established Patterns
- Hand-written chi handlers (NO server codegen). sqlc for queries. Errors via `httpx.WriteError`. Tx-atomic audit + River enqueue. RBAC via `rbac.RequireRole` + scope guards.
- FE: in-memory bearer access token; `SessionUser` permissions derived from role client-side.

### Integration Points
- New `frontend/e2e` workspace member (update root `pnpm-workspace.yaml`/turbo as needed).
- Auth routes mounted in `backend/internal/server/server.go` (public group). forgot/reset handlers added to `handler/identity`.
- `backend/cmd/seed` new entrypoint. FE login/forgot/reset screens wired to real hooks.
</code_context>

<specifics>
## Specific Ideas

- Personas + minimal fixtures align to CONVENTIONS §20 example personas (Sari Hadi · HR Admin; Rudi Wijaya · Shift Leader @ Plaza Senayan) so seeded data matches spec examples.
- `test:e2e:ui` must list each Gherkin scenario as its own runnable case (explicit user ask).
</specifics>

<deferred>
## Deferred Ideas

- Non-auth E1 endpoints (users management, audit log, platform settings) — Phase 2.
- All other epics' endpoints — their respective phases.
- Mobile (React Native) auth — out of milestone scope.
</deferred>

---

*Phase: 01-test-harness-auth*
*Context gathered: 2026-06-04*
