---
phase: 10-e8-payroll
plan: 03
subsystem: backend-api
tags: [payroll, contract-tests, decrypt-fail, async-export, rbac, drift-gate]
dependency-graph:
  requires:
    - 10-02 PayslipService + ExportService + handler.Handler (the system under test)
    - 10-02 svc ports (PayslipRepository / ExportRepository / Jobs) — faked in-memory
    - 10-01 internal/platform/crypto (real Cipher.Encrypt/Decrypt — the DECRYPT_FAIL boundary)
    - Phase-9 overtime contract-test harness (fakeTx + stubIdempotency + mutable-principal chi mount)
  provides:
    - E8 Go contract tests = the drift gate replacing server codegen (list/detail/notes/export pinned to openapi)
    - DECRYPT_FAIL-as-200 asserted honestly through the REAL crypto.Decrypt on garbage ciphertext (not a stub flag)
    - export 202 + transactional-outbox enqueue assertion (recording fakeJobs.EnqueueTx)
  affects:
    - 10-04 (FE wiring + Playwright E2E builds on these now-pinned response shapes)
tech-stack:
  added:
    - none (chi httptest harness + real crypto.Cipher + river.JobArgs fake; all pre-existing)
  patterns:
    - copy the Phase-9 overtime testkit EXACTLY (fakeTx/fakeTxRunner + in-memory fake repos + stubIdempotency at the server.go router position + mutable-principal closure middleware + decodeBody snapshot)
    - real *crypto.Cipher built from a fixed 32-byte key seeds valid ciphertext AND is injected into the service — FINAL decrypts, garbage fails through the genuine AES-GCM Open
    - recording fakeJobs (svc.Jobs) captures EnqueueTx args so the export tx asserts a PayslipExportArgs with the matching JobID
key-files:
  created:
    - backend/internal/handler/payroll/payroll_testkit_test.go
    - backend/internal/handler/payroll/payslip_handler_test.go
    - backend/internal/handler/payroll/export_handler_test.go
  modified: []
decisions:
  - "[10-03]: DECRYPT_FAIL asserted via seedDecryptFail writing RANDOM garbage bytea into all three *_enc columns so the REAL crypto.Decrypt returns ErrDecrypt — the row status is produced by the genuine boundary, never a hardcoded flag (honors the autonomous directive)."
  - "[10-03]: one fixed deterministic 32-byte test key builds a single crypto.Cipher used to BOTH encrypt the FINAL money fixtures AND injected into the PayslipService — so FINAL rows open and garbage rows fail under the same AEAD."
  - "[10-03]: harness mounts ALL 5 ops under one RequireRole(super_admin, hr_admin) group (mirrors server.go); RBAC negatives drive newHarness(agent)/newHarness(shift_leader) and assert 403 before the handler — no in-service scope branch (web archive is global hr/super)."
  - "[10-03]: export 202 confidential-true asserted against a confidential:false REQUEST input (server-coercion proven); the transactional-outbox enqueue asserted via fakeJobs recording exactly one PayslipExportArgs whose JobID == the 202 body id; EXPORT_TOO_LARGE + no-scope assert NO enqueue."
metrics:
  duration: ~15min
  completed: 2026-06-05
---

# Phase 10 Plan 03: E8 Payroll Go Contract Tests Summary

The E8 drift gate that replaces server codegen: three table-driven Go contract-test files that pin every FE-used payroll response to `docs/api/E8-payroll/openapi.yaml` by driving the REAL `PayslipService` + `ExportService` + handler through a chi httptest harness over in-memory fakes. The gnarly assertion — `DECRYPT_FAIL` surfaced as a **200 OK row status** (money null, breakdown `[]`) in BOTH list and detail — is produced honestly: `seedDecryptFail` writes random garbage bytes that the **real** `crypto.Decrypt` rejects (`ErrDecrypt`), so the test asserts the status the boundary genuinely yields, not a stub. The async export proves the **transactional-outbox enqueue** by recording that exactly one `PayslipExportArgs` (matching `JobID`) was `EnqueueTx`'d inside the export tx.

## What was built

- **`payroll_testkit_test.go`** — copies the Phase-9 overtime harness exactly:
  - `fakeTx` (Exec no-op so `audit.Record` + `InsertAuditNote` run inside `InTx`) + `fakeTxRunner`.
  - `stubIdempotency` at the server.go router position (scoped by principal UserID), `decodeBody` snapshotting `rr.Body.Bytes()` for `errCode`/`errFields` re-decode.
  - `fakePayslipRepo` (in-memory `svc.PayslipRepository`): rows carry RAW `*_enc` ciphertext; `seedFinal` Encrypts money with the harness cipher, `seedDecryptFail` stores random garbage, `seedFinalBreakdown` attaches encrypted components+benefits, `seedNote` plants pre-existing notes; cursor keyset `(paid_on DESC, id DESC)` + filters + audit-note `(created_at ASC, seq ASC)`.
  - `fakeExportRepo` (in-memory `svc.ExportRepository`): configurable `countInScope` drives `EXPORT_TOO_LARGE`; `InsertExportJob` returns a QUEUED job stamping a `SWP-EXP-…` id.
  - `fakeJobs` (`svc.Jobs`): `EnqueueTx` records args into a slice.
  - A real `*crypto.Cipher` from a fixed deterministic 32-byte key (seeds valid ciphertext AND injected into the service).
  - `newHarness(role)` builds the REAL services + handler, mounts the 5 ops under `RequireRole(super_admin, hr_admin)` + idempotency on the two POSTs, with a mutable-principal closure middleware.
- **`payslip_handler_test.go`** — list (mixed FINAL+DECRYPT_FAIL at 200; empty→`meta.code MISSING_PAYROLL_HISTORY`; status + period filters; has_more cursor + page 2); detail (FINAL full breakdown with decrypted values + `for_bpjs` + source; DECRYPT_FAIL nulled money + `working_days` null + `earnings/deductions/benefits == []`; 404); audit-notes (list oldest-first on a DECRYPT_FAIL payslip; create composite `{id}-NOTE-1` + author from principal + Location; blank text 400; missing payslip 404); RBAC 403 for agent + shift_leader on all 4 read/note endpoints.
- **`export_handler_test.go`** — 202 + exact `PayslipExportJob` (`id ^SWP-EXP-\d+$`, status QUEUED, format XLSX, `confidential:true` server-forced despite a `false` input, `requested_by.id`, scope echo, `poll_url` + `Location`) AND exactly one `PayslipExportArgs` with the matching `JobID` enqueued; by-year scope echo; `EXPORT_TOO_LARGE` 422 + `error.fields` + NO enqueue; no period/year → 422 `RULE_VIOLATION` + no enqueue; agent/shift_leader 403 + no enqueue.

## Every openapi E8 surface asserted

- DECRYPT_FAIL-as-200 in **list** (`TestListPayslips_MixedFinalAndDecryptFailAt200`) AND **detail** (`TestGetPayslip_DecryptFailNulledEmptyArrays`) — through the real garbage→`ErrDecrypt` boundary.
- `MISSING_PAYROLL_HISTORY` (`TestListPayslips_EmptyMetaCode`).
- Audit-note composite id + 400 blank + 404 missing (`TestCreateAuditNote_*`) + oldest-first list.
- `EXPORT_TOO_LARGE` 422 (`TestExportPayslips_TooLarge422NoEnqueue`).
- 202 + job-enqueue + confidential-true lock (`TestExportPayslips_202QueuedAndEnqueued`).
- RBAC 403 for agent + shift_leader on all 5 ops.
- (`PAYSLIP_IMMUTABLE` 405 out of scope — the FE never calls a payslip write, per the plan.)

## Deviations from Plan

None — plan executed exactly as written. No bugs/contract mismatches were found in the 10-02 handlers/services: every assertion passed against the real code on the first green run, so no handler edits were required. No architectural changes (Rule 4); no authentication gates.

### Out-of-scope discoveries (logged, NOT fixed)
- The pre-existing `gofmt -l internal/` Phase 1-4 formatting drift (noted in 10-01/10-02 deferred-items) remains untouched; all three 10-03 files are gofmt-clean.

## Verification

- `go test ./internal/handler/payroll/...` exits 0 (all E8 contract tests green).
- `go test ./... -count=1` exits 0 — 13 packages with tests pass, no FAIL/panic (no regressions in earlier epics' contract tests).
- `go vet ./internal/handler/payroll/...` clean.
- `gofmt -l` clean across the three new test files.
- Task-1 + Task-2 automated verification greps (`newHarness`, `seedDecryptFail`, `fakeJobs`/`EnqueueTx`, `DECRYPT_FAIL`, `MISSING_PAYROLL_HISTORY`, `NOTE-`/`audit-notes`, `EXPORT_TOO_LARGE`, `202`, `PayslipExportArgs`, `QUEUED`) all pass.

## Self-Check: PASSED

All 3 created files present on disk; both task commits (4a044f4, d732a75) in git log.
