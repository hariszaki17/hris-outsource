---
phase: 10-e8-payroll
verified: 2026-06-05T00:00:00Z
status: human_needed
score: 4/4 must-haves verified
human_verification:
  - test: "Full Playwright E2E suite runs green (225 passed / 6 skipped / 0 failed)"
    expected: "pnpm --filter @swp/e2e exec playwright test exits 0 with 225 passed, 0 failed; E8 subset shows 16 passed"
    why_human: "E2E requires Docker + real Postgres + River worker boot; cannot re-execute in this environment"
  - test: "Export worker actually completes the job to DONE in the live stack"
    expected: "POST /payslips:export returns 202 + SWP-EXP-… id; pollExportJob returns status DONE with row_count > 0 within 20s"
    why_human: "Requires live Postgres + River worker process; the DB poll helper (pollExportJob) cannot be exercised statically"
---

# Phase 10: E8 Payroll Verification Report

**Phase Goal:** Read-only payslips, audit notes, and async export work against the real BE.
**Verified:** 2026-06-05
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Payslips list and detail render (read-only history); audit notes can be listed and created | VERIFIED | 5 routes wired in server.go under `RequireRole(super_admin, hr_admin)`; PayslipService.List/Get/ListAuditNotes/CreateAuditNote all substantive (313-line service); contract tests green (`go test ./internal/handler/payroll/...` exits 0) |
| 2 | Payslip export returns 202 + a job id and the job completes via the worker | VERIFIED (automated) / ? HUMAN for live proof | export_handler.go returns `http.StatusAccepted`; ExportService does `InsertExportJob(QUEUED)` + `jobs.EnqueueTx(PayslipExportArgs)` in ONE tx; PayslipExportWorker.Work() transitions RUNNING -> DONE via `UpdateExportJobStatus`; export.spec.ts uses `pollExportJob` to assert `export_jobs.status = DONE` with `row_count > 0` — live E2E run documented as 16/16 green but cannot be independently re-run |
| 3 | RBAC restricts payroll visibility appropriately | VERIFIED | `RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin)` wraps all 5 routes (server.go:467); contract tests `TestPayslipReadEndpoints_RBACForbidden` assert 403 for agent + shift_leader on all 5 endpoints; rbac.spec.ts covers FE+BE 403 path |
| 4 | Exhaustive Playwright E2E for E8 features is green | ? HUMAN | 10-04 SUMMARY documents `pnpm exec playwright test` → 225 passed / 6 skipped / 0 failed (16 new E8 tests); cannot independently re-run without Docker + live Postgres |

**Score:** 4/4 truths structurally verified; 2 truths require human execution of the live stack

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `backend/db/migrations/00033_payslips.sql` | Encrypted payslips + child tables | VERIFIED | 123 lines; `*_enc bytea` columns, no plaintext money, `CHECK (status IN ('FINAL','DECRYPT_FAIL'))`, child tables `payslip_components`/`payslip_benefits`/`payslip_audit_notes` |
| `backend/db/migrations/00034_export_jobs.sql` | Async export-job tracker | VERIFIED | 48 lines; `CHECK (status IN ('QUEUED','RUNNING','DONE','FAILED'))`, `confidential DEFAULT true`, lifecycle timestamps |
| `backend/internal/platform/crypto/crypto.go` | AES-256-GCM helper | VERIFIED | 117 lines; `New`/`NewFromBase64`/`Encrypt`/`Decrypt`/`DecryptPtr`/`ErrDecrypt` — substantive; all crypto unit tests pass |
| `backend/internal/platform/crypto/crypto_test.go` | Crypto unit tests | VERIFIED | Round-trip, garbage, wrong-key, too-short, three-case `DecryptPtr`, base64 — all green |
| `backend/internal/domain/payroll/payroll.go` | Domain types + enums | VERIFIED | `Payslip`/`ExportJob`/line types; `PayslipStatusFinal/DecryptFail`; `ExportJobStatusDone="DONE"` (not COMPLETED — spec wins) |
| `backend/internal/repository/payroll/payslip_repo.go` | Payslip repository | VERIFIED | Exists; returns RAW `*_enc` ciphertext (never decrypts); `pgx.ErrNoRows` → `domain.ErrNotFound` |
| `backend/internal/repository/payroll/export_repo.go` | Export repository | VERIFIED | Exists; `InsertExportJob(WithTx)` + `GetExportJob` + `CountPayslipsInScope` |
| `backend/internal/service/payroll/payslip_service.go` | Payslip service (decrypt boundary) | VERIFIED | 313 lines; `decryptMoney` seam via `DecryptPtr`; `markDecryptFail` (nulls all money, sets `status=DECRYPT_FAIL`, `locked_reason="decrypt_fail"`); 200 not 4xx |
| `backend/internal/service/payroll/export_service.go` | Export service | VERIFIED | 154 lines; period-or-year guard (422); `confidential=true` forced; `EXPORT_TOO_LARGE` (422); transactional outbox (`InsertExportJob` + `EnqueueTx` in same `InTx`) |
| `backend/internal/handler/payroll/payslip_handler.go` | Payslip handler | VERIFIED | 112 lines; `ListPayslips` 200, `GetPayslip` 200, `ListAuditNotes` 200, `CreateAuditNote` 201 + Location |
| `backend/internal/handler/payroll/export_handler.go` | Export handler | VERIFIED | 34 lines; returns `http.StatusAccepted` (202) + `ExportPayslips` location |
| `backend/internal/platform/jobs/payslip_export.go` | PayslipExportWorker | VERIFIED | `Work()` transitions RUNNING → DONE; uses `UpdateExportJobStatus`; `CountPayslipsInScope` for `row_count`; `artifact_ref` stamped |
| `backend/internal/handler/payroll/payroll_testkit_test.go` | Contract test harness | VERIFIED | `fakePayslipRepo`/`fakeExportRepo`/`fakeJobs`/real `*crypto.Cipher`; `seedDecryptFail` writes random garbage so real `crypto.Decrypt` rejects |
| `backend/internal/handler/payroll/payslip_handler_test.go` | Payslip contract tests | VERIFIED | `TestListPayslips_MixedFinalAndDecryptFailAt200`, `TestGetPayslip_DecryptFailNulledEmptyArrays`, `TestListPayslips_EmptyMetaCode`, audit-note tests, RBAC 403 — all green |
| `backend/internal/handler/payroll/export_handler_test.go` | Export contract tests | VERIFIED | 202 + job-enqueue + confidential-true lock; `EXPORT_TOO_LARGE` 422 + NO enqueue; `RULE_VIOLATION` 422; RBAC 403 — all green |
| `frontend/e2e/tests/e8/archive.spec.ts` | Archive E2E spec | VERIFIED | Exists; 4 tests: FINAL list, DECRYPT_FAIL row @200, filters, empty state |
| `frontend/e2e/tests/e8/detail.spec.ts` | Detail E2E spec | VERIFIED | Exists; 2 tests: FINAL breakdown, DECRYPT_FAIL state (banner, "—", no Ekspor) |
| `frontend/e2e/tests/e8/audit-notes.spec.ts` | Audit notes E2E spec | VERIFIED | Exists; 3 tests: list seeded notes, create note, <8-char block |
| `frontend/e2e/tests/e8/export.spec.ts` | Export E2E spec | VERIFIED | Exists; 4 tests incl. the headline `pollExportJob` → DONE proof; `confidential-true` lock; no-scope 422 |
| `frontend/e2e/tests/e8/rbac.spec.ts` | RBAC E2E spec | VERIFIED | Exists; 3 tests: agent + shift_leader no-permission UI + real BE 403; HR 200 positive |
| `frontend/e2e/lib/e8-helpers.ts` | E2E helper + `pollExportJob` | VERIFIED | Exists; `pollExportJob` opens a `pg.Client` on DATABASE_URL and polls `export_jobs` every 250ms until `status IN ('DONE','FAILED')` — real DB poll, not a mock |
| `backend/cmd/migrate/main.go` | `river-up` subcommand | VERIFIED | `rivermigrate.New(...).Migrate(ctx, rivermigrate.DirectionUp, nil)` — programmatic; no external `river` CLI required |
| `frontend/e2e/lib/backend.ts` (worker spawn) | Harness spawns `cmd/worker` | VERIFIED | `spawn('go', ['run', './cmd/worker'], {detached:true, ...})` with `apiEnv` (incl. `PAYROLL_ENCRYPTION_KEY`); SIGTERM-reaped via negative-pid group kill |
| `frontend/e2e/lib/reset-db.ts` (payroll tables) | Payroll TRUNCATE + full-env re-seed | VERIFIED | `payslip_audit_notes`/`payslip_benefits`/`payslip_components`/`payslips`/`export_jobs` in `TRUNCATE_TABLES`; full `.env.e2e` (incl. `PAYROLL_ENCRYPTION_KEY`) passed to re-seed |
| `frontend/e2e/.env.e2e` (`PAYROLL_ENCRYPTION_KEY`) | Key for seed + API + worker | VERIFIED | `PAYROLL_ENCRYPTION_KEY=QUFBQ...` present (base64 32-byte key) |
| `backend/cmd/seed/seed.go` (`SWP-PS-90119` DECRYPT_FAIL row) | Seeded garbage ciphertext row | VERIFIED | `garbage := []byte{0xde, 0xad, 0xbe, 0xef, ...}` inserted for `SWP-PS-90119`; two audit notes seeded; `ON CONFLICT DO NOTHING` idempotent |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `main.go` | `crypto.Cipher` | `crypto.NewFromBase64(cfg.Crypto.PayrollKey)` | WIRED | `payrollCipher` built from env; injected into `NewPayslipService` |
| `main.go` | `jobs.NewInsertOnlyClient` | `jobs.NewInsertOnlyClient(pool)` | WIRED | First API-process jobs client; injected into `NewPayslipService` + `NewExportService` |
| `jobs.go` | `PayslipExportWorker` | `river.AddWorker(workers, NewPayslipExportWorker(pool))` | WIRED | Registered in `NewWorkerClient`; pool-backed (line 61) |
| `server.go` | Payroll routes | `r.Use(rbac.RequireRole(super_admin, hr_admin))` wrapping 5 routes | WIRED | Lines 467-472 confirmed; all 5 ops gated |
| `ExportService.Export` | `InsertExportJob` + `EnqueueTx` | `s.txm.InTx(...)` containing both calls | WIRED | Transactional outbox confirmed in export_service.go |
| `PayslipExportWorker.Work()` | `export_jobs` DONE | `UpdateExportJobStatus({Status:"DONE",...})` | WIRED | payslip_export.go line 79 |
| `backend.ts` harness | `cmd/worker` | `spawn('go', ['run', './cmd/worker'], {detached:true})` | WIRED | Spawned right after API boot; reaps via `process.kill(-pid, 'SIGTERM')` |
| `backend.ts` harness | River migrations | `go run ./cmd/migrate river-up` | WIRED | `runRiverMigrations` calls `river-up` subcommand before worker boot |
| `reset-db.ts` | `seedPayroll` re-execution | Full `.env.e2e` passed to seed subprocess | WIRED | `PAYROLL_ENCRYPTION_KEY` in env prevents early-return skip |
| `pollExportJob` | `export_jobs` table | `pg.Client` + `SELECT status, row_count, artifact_ref FROM export_jobs WHERE id = $1` | WIRED | DB poll every 250ms until terminal; 20s timeout |
| `payslip-detail-screen.tsx` | BE `{data:Payslip}` envelope | `.data.data` inner unwrap with bare fallback | WIRED | Fix applied in commit 9fb2ca9 (rule-1 bug found during E2E) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| PAY-01 | 10-01, 10-02, 10-03, 10-04 | Payslips list/detail (read-only history), audit notes list/create | SATISFIED | 4 of 5 routes serve read ops; `CreateAuditNote` is the only write; contract tests + E2E specs green |
| PAY-02 | 10-01, 10-02, 10-03, 10-04 | Payslip export (async job → 202 + job id) | SATISFIED (live proof human-verifiable) | `POST /payslips:export` returns 202; `PayslipExportWorker` transitions to DONE; `pollExportJob` proves completion in E2E |

REQUIREMENTS.md table still shows "In Progress (10-01..03; 10-04 E2E pending)" — this reflects the state before 10-04 completed and was not updated after. The actual code state is fully delivered.

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `payslip_service.go:30,286` | `// TODO Phase-11` notify stub | Info | Intentional deferred per CONTEXT locked decision: "Audit notes notify = TODO(Phase-11) comment"; matches established convention from leave/overtime/people; NOT a blocker |

No placeholder returns, no `return null`, no empty implementations found in any payroll handler, service, repository, or worker file.

### Honest Deferral Confirmations

The following items were explicitly accepted as out-of-scope per CONTEXT locked decisions — all confirmed correctly modeled:

1. **DECRYPT_FAIL is 200 (not 4xx):** Confirmed. `payslip_service.go` calls `markDecryptFail()` and returns the row with `status=DECRYPT_FAIL`; handler writes `http.StatusOK`. Contract test `TestListPayslips_MixedFinalAndDecryptFailAt200` asserts 200.

2. **Encryption-at-rest via `internal/platform/crypto` AES-256-GCM:** Confirmed. `crypto.go` (117 lines) implements nonce-prepended GCM; key from `config.Crypto.PayrollKey` (`PAYROLL_ENCRYPTION_KEY` env); `NewFromBase64` wired in `main.go`.

3. **Notifications are a dispatch STUB (TODO Phase-11):** Confirmed. `payslip_service.go:286` has `// Notify stub (TODO Phase-11)`. No real `EnqueueTx` for notifications. The EXPORT path DOES really enqueue (River `PayslipExportArgs`) — this is not a notification stub, it is the success criterion.

4. **Active payroll runs/edits, PDF, forward-export, agent mobile summaries are out of web scope:** Confirmed. Only the 5 FE-used endpoints are implemented. No `PUT /payslips`, no PDF handler, no agent-scoped routes in server.go.

5. **CONTEXT prose says "COMPLETED" but code uses "DONE":** Decision [10-01] correctly resolves this: openapi `PayslipExportJob.status` uses `DONE` → spec wins. The migration `CHECK (status IN ('QUEUED','RUNNING','DONE','FAILED'))` and worker `Status:"DONE"` are byte-for-byte correct.

### Build + Test Results (Independently Run)

- `go build ./...` — exits 0 (no output)
- `go vet ./...` — exits 0 (no output)
- `go test ./internal/handler/payroll/... ./internal/platform/crypto/... -count=1` — **PASS** (all contract tests + crypto unit tests green, 0 failures)

### Human Verification Required

#### 1. Full Playwright E2E Suite — 225 passed / 0 failed

**Test:** `pnpm --filter @swp/e2e exec playwright test --reporter=line` (requires Docker + running Postgres + API + River worker)
**Expected:** 225 passed / 6 skipped / 0 failed; E8 subset (`tests/e8`) shows 16 passed
**Why human:** Cannot start Docker, Postgres, or the Go processes in this verification environment

#### 2. Export Worker Completes Job to DONE (Live Stack)

**Test:** POST `/api/v1/payslips:export` as HR admin with `{period:"2025-12", format:"XLSX"}`; capture `id` from 202 body; poll `SELECT status, row_count FROM export_jobs WHERE id = $1` until terminal
**Expected:** `status = 'DONE'`, `row_count > 0`, `artifact_ref` like `payroll-export-SWP-EXP-….csv`
**Why human:** Requires the live Postgres + River worker; `pollExportJob` cannot run without a real DB connection and a running worker process

### Gaps Summary

No gaps found. All backend artifacts exist, are substantive, and are wired end-to-end. The `go build` + `go vet` + contract tests pass independently. The only items requiring human verification are the full Playwright E2E run and the live export-worker proof — both of which the executor documented as green (225 passed / 0 failed, 16 E8 tests, export_jobs → DONE) and are structurally confirmed by code inspection.

---

_Verified: 2026-06-05_
_Verifier: Claude (gsd-verifier)_
