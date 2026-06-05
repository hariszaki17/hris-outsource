---
phase: 10-e8-payroll
plan: 04
subsystem: frontend-e2e
tags: [payroll, e2e, playwright, decrypt-fail, async-export, river-worker, rbac, encryption-at-rest]
dependency-graph:
  requires:
    - 10-02 E8 API slice (list/detail decrypt-at-boundary + DECRYPT_FAIL 200 + audit notes + async export + seed)
    - 10-03 E8 Go contract tests (the now-pinned response shapes)
    - e8-payroll FE screens (archive/detail/audit-note-drawer/export, built from .pen)
    - Phase-9 E2E harness (detached API boot, resetDb, loginAs PERSONAS.*, window.__swp_get_token__, apiAs)
  provides:
    - e8-payroll screens wired off MSW to the real Go BE (archive list+filters, detail breakdown, DECRYPT_FAIL@200, audit notes, RBAC)
    - the harness now boots the River worker (cmd/worker) so async jobs (incl. the payslip export) actually complete
    - cmd/migrate `river-up` subcommand — applies River queue migrations programmatically (no `river` CLI dependency)
    - 16 full-stack Playwright E8 specs green headless against real FE+Go+ephemeral Postgres
    - the export proof: export_jobs row reaches DONE via the REAL worker, asserted by a DB poll (no FE job-status hook)
  affects:
    - Phase 10 is CLOSED — PAY-01 / PAY-02 satisfied end-to-end
tech-stack:
  added:
    - none (Playwright + pg Client + go run ./cmd/worker + rivermigrate — all pre-existing deps)
  patterns:
    - export completion observed via a DB poll (pollExportJob on export_jobs) since E8 has NO FE job-status hook
    - harness spawns cmd/worker detached (own process group, SIGTERM-reaped) mirroring the cmd/api spawn [07-04]
    - River queue migrations applied programmatically via rivermigrate (cmd/migrate river-up) — no external `river` CLI
    - resetDb passes the FULL .env.e2e (incl. PAYROLL_ENCRYPTION_KEY) so seedPayroll re-inserts fixtures after TRUNCATE
    - detail {data}-envelope unwrap with a bare fallback (recurring finding; Phase-8 [08-04] precedent)
    - deep-route auth-restore race dodged by landing on /payroll + waitForToken BEFORE navigating to the detail route
key-files:
  created:
    - frontend/e2e/lib/e8-helpers.ts
    - frontend/e2e/tests/e8/archive.spec.ts
    - frontend/e2e/tests/e8/detail.spec.ts
    - frontend/e2e/tests/e8/audit-notes.spec.ts
    - frontend/e2e/tests/e8/export.spec.ts
    - frontend/e2e/tests/e8/rbac.spec.ts
  modified:
    - frontend/e2e/lib/backend.ts (spawn cmd/worker + programmatic river-up + teardown)
    - frontend/e2e/lib/reset-db.ts (payroll + export_jobs TRUNCATE; full-env re-seed)
    - frontend/e2e/.env.e2e (PAYROLL_ENCRYPTION_KEY)
    - frontend/apps/web/src/features/e8-payroll/payslip-detail-screen.tsx (detail {data}-unwrap fix)
    - backend/cmd/migrate/main.go (river-up subcommand via rivermigrate)
decisions:
  - "[10-04]: payslip-detail-screen unwraps the BE {data:Payslip} envelope with a bare fallback — the E8 openapi declares getPayslip 200 as a BARE Payslip but the handler wraps it (like every epic), so query.data.data was the wrapper not the payslip → avatarInitials(undefined).split crash. Fixed toward the BE (Phase-8 [08-04] precedent). Archive list + audit-note list match their envelopes already (no change)."
  - "[10-04]: the harness boots the River worker (go run ./cmd/worker, detached, apiEnv incl. PAYROLL_ENCRYPTION_KEY) so the PayslipExportArgs job is processed — without it export_jobs stays QUEUED forever. SIGTERM-reaped on teardown (negative-pid process-group kill) mirroring the API spawn."
  - "[10-04]: River queue migrations are applied PROGRAMMATICALLY via a new `cmd/migrate river-up` subcommand (rivermigrate over a pgx pool) — the `river` CLI is not installed and `go run .../cmd/river` is not in the module graph; without the river_queue tables the worker crashed on boot and the export never completed. CLI fallback retained."
  - "[10-04]: resetDb's re-seed now passes the FULL .env.e2e (incl. PAYROLL_ENCRYPTION_KEY) — seedPayroll returns early when the key is unset, so the beforeEach TRUNCATE was wiping the payslip fixtures and never restoring them. The DECRYPT_FAIL garbage row also lives inside seedPayroll, so it too was being skipped."
  - "[10-04]: the export E2E drives the export via apiAs POST /payslips:export (NO FE surface mounts PayrollExportButton — the detail Ekspor entry only fires a QUEUED toast) and proves completion via pollExportJob (export_jobs.status → DONE, row_count > 0) — the REAL worker, not a 202-only nor a mocked completion."
  - "[10-04]: EXPORT_TOO_LARGE is NOT driven from the seed (the 50k-row threshold is unreachable with a handful of fixtures) — it is cited as fully covered by the 10-03 contract test (TestExportPayslips_TooLarge422NoEnqueue); the export spec asserts the honest happy 202+DONE + confidential-true lock + no-scope 422 instead."
  - "[10-04]: reset-db adds payslip_audit_notes/payslip_benefits/payslip_components/payslips/export_jobs to TRUNCATE (app tables only) — River's own river_job/river_* tables are left intact so the running worker keeps its queue."
metrics:
  duration: ~75min
  completed: 2026-06-05
---

# Phase 10 Plan 04: E8 Payroll FE Wiring + Full-Stack Playwright E2E Summary

The e8-payroll screens now drive the **real Go backend** (MSW off) and every E8 Gherkin AC is proven by **16 full-stack Playwright specs** green headless against an ephemeral Postgres — the HR/Super-Admin payroll archive (list + filters), the payslip detail breakdown decrypted from real AES-GCM ciphertext, the **DECRYPT_FAIL row rendered at 200** (banner, "—" money, placeholders — not an error page), append-only audit notes (list + create), the **async export** (202 + `SWP-EXP-…` job id THEN the **REAL River worker** flips `export_jobs` → DONE, proven via a DB poll), and **RBAC 403** for agent/shift_leader on every payroll endpoint. The headline — the worker-completes-job proof — required booting the River worker in the harness (it ran only the API before) and applying River's queue migrations programmatically. Phase 10 is closed: **PAY-01 / PAY-02 satisfied end-to-end**.

## What was built

- **`e8-helpers.ts`** — the seeded `PS`/`PS_NOTE`/`PS_NAME`/`PS_EMP` fixture maps, the `payslipRow` (div.border-b) locator + `expectPayslipRow`/`expectNoPayslipRow`, and the KEY `pollExportJob(jobId)` helper: opens a `pg` Client on the .env.e2e DATABASE_URL and polls `export_jobs` every 250ms until `status IN ('DONE','FAILED')` — how the export E2E proves the worker completed (no FE job-status hook in E8). Re-exports `apiAs`/`errorCode`/`waitForToken` from e5-helpers.
- **Harness worker boot (`backend.ts`)** — spawns `go run ./cmd/worker` (detached, own process group, `apiEnv` incl. `PAYROLL_ENCRYPTION_KEY` + `DATABASE_URL`) right after the API; SIGTERM-reaped on teardown via the negative-pid group kill. `runRiverMigrations` now runs `go run ./cmd/migrate river-up` (programmatic) with the `river` CLI as a fallback.
- **`cmd/migrate river-up`** — a new subcommand applying River's queue migrations via `rivermigrate.New(riverpgxv5.New(pool)).Migrate(DirectionUp)`; no external `river` CLI required (the CLI is absent and `cmd/river` is not in the module graph). Without `river_queue` the worker crashed on boot.
- **`reset-db.ts`** — adds `payslip_audit_notes`/`payslip_benefits`/`payslip_components`/`payslips`/`export_jobs` to the TRUNCATE list (app tables only; River's internal tables untouched) and passes the **full** .env.e2e to the re-seed so `seedPayroll` (which skips entirely without `PAYROLL_ENCRYPTION_KEY`) re-inserts the FINAL fixtures + the DECRYPT_FAIL garbage row + the two audit notes after each TRUNCATE.
- **`.env.e2e`** — `PAYROLL_ENCRYPTION_KEY` (base64 of 32 bytes), read by seed + API + worker so FINAL money decrypts and SWP-PS-90119 garbage fails honestly.
- **FE fix (`payslip-detail-screen.tsx`)** — unwraps the BE `{data:Payslip}` detail envelope with a bare fallback (the openapi declares a bare `Payslip`, the handler wraps it → the screen was reading the wrapper and crashing on `avatarInitials(undefined).split`). Archive list + audit-note list already matched their envelopes — no change.
- **5 spec files (16 tests)** under `frontend/e2e/tests/e8/`:
  - **archive.spec** (4): FINAL list (year 2025), DECRYPT_FAIL row @200, status/period/employee filters, MISSING_PAYROLL_HISTORY empty state.
  - **detail.spec** (2): FINAL full decrypted breakdown (Gaji Pokok/BPJS/PPh 21/benefits + take-home Rp 7.325.000 + source "lumen_swp #44218"); DECRYPT_FAIL state (banner, "—", placeholders, no Ekspor).
  - **audit-notes.spec** (3): list the two seeded notes (drawer-scoped), create a ≥8-char note (composite `SWP-PS-90121-NOTE-1` via apiAs) appearing in the list, <8-char Zod block.
  - **export.spec** (4): the headline 202+`SWP-EXP`+worker→DONE (row_count>0), confidential-true lock, EXPORT_TOO_LARGE coverage note, no-scope 422.
  - **rbac.spec** (3): agent + shift_leader no-permission UI + real BE 403 on list/export/audit-notes; HR 200 positive control.

## Verification

- `pnpm --filter @swp/e2e exec playwright test tests/e8 --reporter=line` → **16 passed** headless.
- Full suite `pnpm --filter @swp/e2e exec playwright test` → **225 passed / 6 skipped / 0 failed** (no e1–e7 regression; the `context canceled` server logs are benign client-aborted in-flight requests, not test failures).
- The export proof is the `export_jobs` row reaching **DONE** (row_count > 0) via the REAL worker — asserted by `pollExportJob`, not a 202-only nor a mocked completion.
- `pnpm --filter @swp/web build` succeeds; `go build ./...` + `go vet ./cmd/migrate` clean; `gofmt -l cmd/migrate/main.go` clean.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] payslip-detail-screen read the {data} wrapper as the Payslip → render crash**
- **Found during:** Task 2 (detail spec — React error boundary "Cannot read properties of undefined (reading 'split')").
- **Issue:** The BE wraps GET /payslips/{id} in `{data:Payslip}` (like every epic) but the E8 openapi declares a BARE `Payslip`, so Orval narrows `query.data.data` to `Payslip` while the real body is one level deeper. `payslip.employee_id`/`period` were undefined → `avatarInitials(undefined).split`.
- **Fix:** Unwrap the inner `{data}` with a bare fallback (Phase-8 [08-04] precedent). Surgical, FE-only.
- **Files modified:** `frontend/apps/web/src/features/e8-payroll/payslip-detail-screen.tsx`
- **Commit:** 9fb2ca9

**2. [Rule 3 - Blocking] River queue tables never created → worker crashed → export never completed**
- **Found during:** Task 2 (worker logged `relation "river_queue" does not exist`, `worker fatal`).
- **Issue:** The harness's `runRiverMigrations` depended on a `river` CLI that is not installed (it warned + continued); `go run .../cmd/river` is not in the module graph. So the worker died on boot and `export_jobs` would stay QUEUED.
- **Fix:** Added a `cmd/migrate river-up` subcommand applying River migrations programmatically via `rivermigrate`; the harness now uses it (CLI fallback retained).
- **Files modified:** `backend/cmd/migrate/main.go`, `frontend/e2e/lib/backend.ts`
- **Commit:** 9fb2ca9

**3. [Rule 3 - Blocking] resetDb wiped the payroll fixtures without restoring them**
- **Found during:** Task 2 (detail/audit GET → 404; seedPayroll skipped).
- **Issue:** `seedPayroll` returns early when `PAYROLL_ENCRYPTION_KEY` is unset, and `resetDb`'s re-seed env only carried `DATABASE_URL`+`ENV`. So the `beforeEach` TRUNCATE removed the payslips + the DECRYPT_FAIL row and never restored them.
- **Fix:** resetDb now loads + passes the full .env.e2e to the re-seed.
- **Files modified:** `frontend/e2e/lib/reset-db.ts`
- **Commit:** 9fb2ca9

**4. [Rule 1 - Bug, test-only] deep-route auth-restore race surfaced a transient 500**
- The detail route fired its GET before the in-memory token re-hydrated post-`goto`, aborting it (500 "context canceled"). Dodged by landing on `/payroll` + `waitForToken` BEFORE navigating to the detail route (mirrors e7's pre-hydration pattern). No production change.

### Out-of-scope discoveries (logged, NOT fixed)
- The openapi↔handler mismatch (getPayslip declares bare `Payslip`, handler returns `{data}`) was fixed on the FE toward the BE (matching every other epic). Reconciling the openapi to declare the `{data}` envelope is an authoring task for a spec-cleanup pass, not this E2E plan.
- The pre-existing `gofmt -l internal/` Phase 1-4 formatting drift remains untouched.

No architectural changes (Rule 4) were needed. No authentication gates encountered. Docker was available throughout (no blocker).

## Self-Check: PASSED
