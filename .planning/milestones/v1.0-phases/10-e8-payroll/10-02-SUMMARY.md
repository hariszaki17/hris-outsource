---
phase: 10-e8-payroll
plan: 02
subsystem: backend-api
tags: [payroll, encryption-at-rest, decrypt-fail, river, async-export, rbac, seed]
dependency-graph:
  requires:
    - 10-01 payslips/components/benefits/audit_notes/export_jobs tables + sqlc Querier
    - 10-01 internal/platform/crypto (Cipher.DecryptPtr + ErrDecrypt) + config.Crypto.PayrollKey
    - internal/platform/jobs (River Client EnqueueTx + NewWorkerClient) — first wired into the API here
    - employees seed ids SWP-EMP-1042/1108/2891/3001 (Phase-4)
  provides:
    - GET /payslips (cursor + employee_id/period/year/status filters; meta.code MISSING_PAYROLL_HISTORY on empty)
    - GET /payslips/{id} (full earnings/deductions/benefits breakdown; {data} envelope)
    - GET/POST /payslips/{id}/audit-notes (append-only, audited in-tx)
    - POST /payslips:export (202 + PayslipExportJob stub; real River PayslipExportWorker completes export_jobs DONE)
    - decrypt-AT-BOUNDARY: DECRYPT_FAIL as a 200 row status (money nulled, breakdown [])
    - jobs.NewInsertOnlyClient wired into cmd/api (transactional-outbox enqueue)
    - PayslipExportWorker registered in jobs.NewWorkerClient
    - seedPayroll fixtures (FINAL rows + garbage-ciphertext DECRYPT_FAIL row + audit notes)
  affects:
    - 10-03 (Go contract tests assert these response shapes + DECRYPT_FAIL + export 202 + EXPORT_TOO_LARGE + RBAC)
    - 10-04 (FE wiring off MSW + Playwright E2E; export E2E polls export_jobs.status=DONE)
tech-stack:
  added:
    - none (River + crypto + sqlc all pre-existing; no heavy xlsx dep — CSV/row-count stand-in artifact)
  patterns:
    - decrypt-at-the-service-boundary via a single decryptMoney seam (DecryptPtr three-case → DECRYPT_FAIL)
    - repo returns RAW *_enc ciphertext on intermediate svc.PayslipRow/LineRow; service decrypts
    - transactional outbox: InsertExportJob(QUEUED) + jobs.EnqueueTx(PayslipExportArgs) in ONE tx
    - pool-backed River worker (first worker whose Work() writes the app DB; constructed in NewWorkerClient)
    - Jobs interface seam (svc.Jobs) so 10-03 can fake River without Postgres
    - custom page envelope (payslipPageResponse) to carry optional meta.code (httpx.PageResponse has no Meta)
    - required-nullable money/scope pointers WITHOUT omitempty → JSON null; breakdown arrays as *[]T (omitted on list, [] on detail)
key-files:
  created:
    - backend/internal/service/payroll/ports.go
    - backend/internal/service/payroll/helpers.go
    - backend/internal/service/payroll/payslip_service.go
    - backend/internal/service/payroll/export_service.go
    - backend/internal/repository/payroll/mapping.go
    - backend/internal/repository/payroll/payslip_repo.go
    - backend/internal/repository/payroll/export_repo.go
    - backend/internal/handler/payroll/handler.go
    - backend/internal/handler/payroll/dto.go
    - backend/internal/handler/payroll/payslip_handler.go
    - backend/internal/handler/payroll/export_handler.go
    - backend/internal/platform/jobs/payslip_export.go
  modified:
    - backend/internal/platform/jobs/jobs.go (register PayslipExportWorker in NewWorkerClient)
    - backend/internal/server/server.go (Deps.Payroll + E8 route group under RequireRole(super_admin, hr_admin))
    - backend/cmd/api/main.go (crypto cipher + jobs.NewInsertOnlyClient + payroll slice + Deps.Payroll)
    - backend/cmd/seed/seed.go (seedPayroll + seedPayslipBreakdown)
decisions:
  - "[10-02]: repo NEVER decrypts — returns RAW *_enc ciphertext on svc.PayslipRow/LineRow; the service owns the single decryptMoney seam (DecryptPtr garbage→DECRYPT_FAIL). Domain Payslip only ever carries DECRYPTED money."
  - "[10-02]: DECRYPT_FAIL is whole-payslip — if the summary OR any line fails to open, status=DECRYPT_FAIL + all money nulled + working_days nulled + earnings/deductions/benefits=[] (no partial breakdown), 200 OK not 4xx."
  - "[10-02]: meta.code MISSING_PAYROLL_HISTORY lives in a custom payslipPageResponse envelope (httpx.PageResponse has no Meta field); set only on data:[]."
  - "[10-02]: export artifact is a dependency-light faithful stand-in (CountPayslipsInScope row_count + artifact_ref string) per CONTEXT discretion — no heavy xlsx lib; the job LIFECYCLE (QUEUED→RUNNING→DONE) is the real success criterion."
  - "[10-02]: svc.Jobs interface seam (EnqueueTx) lets 10-03 fake River; the real *jobs.Client satisfies it (compile-checked via Deps wiring)."
  - "[10-02]: PayslipExportWorker is the FIRST River worker whose Work() writes the app DB — constructed WITH *db.Pool in NewWorkerClient (pool only in scope there); registerWorkers stays no-pool for NotificationWorker."
  - "[10-02]: confidential forced true on the returned stub regardless of client input (Wave 2.8 lock); DB DEFAULT already true."
  - "[10-02]: audit-note author = caller's EmployeeID; author_name left nil (E2 lookup deferred) — FE falls back to author_id. Notify stub is a TODO(Phase-11) comment (matches the established no-op pattern in leave/overtime)."
  - "[10-02]: seed DECRYPT_FAIL row writes raw garbage bytea (0xdeadbeef…) into all three *_enc columns so AES-GCM Open genuinely rejects — surfaced honestly at read time, not a trusted status flag."
metrics:
  duration: ~30min
  completed: 2026-06-05
---

# Phase 10 Plan 02: Payroll Services + Handlers + Async Export Summary

E8 payroll API slice: read payslips (list/detail) decrypting money AT THE BOUNDARY and surfacing a row whose ciphertext fails to open as a **200 OK DECRYPT_FAIL** (money nulled, breakdown `[]`); append-only audit notes (list + create, audited in-tx); and the async **Excel export** — `POST /payslips:export` inserts an `export_jobs` QUEUED row AND `EnqueueTx`'s a real River `PayslipExportWorker` in the **same tx** (transactional outbox), returns 202 + the job stub, and the worker completes the job by flipping `export_jobs` to **DONE**. All 5 FE-used endpoints are HR/Super-Admin only (agent/shift_leader → 403). This plan also performed the first wiring of `jobs.Client` into the API process, registered the new worker, and extended the seed.

## What was built

- **Repository** (`internal/repository/payroll/`) — `PayslipRepo` (list/get returning RAW `*_enc` ciphertext on `svc.PayslipRow`, components/benefits as `svc.LineRow`, `PayslipExists` + audit-note list/count/insert) + `ExportRepo` (`CountPayslipsInScope`, `InsertExportJob` via `WithTx`, `GetExportJob`). `pgx.ErrNoRows → domain.ErrNotFound`; `pgtype.Date ↔ *time.Time`, `int32 ↔ *int`.
- **Ports + helpers** — `PayslipRepository`/`ExportRepository`/`TxRunner`/`Clock`/`Jobs` (River seam) interfaces; `decryptMoney` (the single DECRYPT_FAIL seam wrapping `Cipher.DecryptPtr`); payslip cursor (paid_on DESC, id) + audit-note cursor (created_at ASC, seq) codecs; principal extraction.
- **PayslipService** — `List` (per-row summary decrypt; any failure → DECRYPT_FAIL row; `missingHistory` flag → meta.code), `Get` (summary + components + benefits decrypt; whole-payslip DECRYPT_FAIL on any line failure with empty arrays; NotFound→404), `ListAuditNotes` (404 guard + cursor), `CreateAuditNote` (text 1–4000, composite `{id}-NOTE-{seq}`, audited in-tx).
- **ExportService** — period-or-year `RULE_VIOLATION` (422), force `confidential=true`, `EXPORT_TOO_LARGE` (422, default threshold 50,000) via `CountPayslipsInScope`, then `InsertExportJob(QUEUED)` + `jobs.EnqueueTx(PayslipExportArgs)` + audit in ONE tx → returns the stub.
- **PayslipExportWorker** (`internal/platform/jobs/payslip_export.go`) — `PayslipExportArgs.Kind()=="payslip.export"`; `Work()` flips export_jobs RUNNING → builds a faithful stand-in (row count + `payroll-export-{id}.csv` ref) → DONE; FAILED + return-error (River retries) on failure. Pool-backed.
- **Handlers + DTOs** (`internal/handler/payroll/`) — `ListPayslips` (top-level cursor envelope + meta.code), `GetPayslip` (`{data}` wrap), `ListAuditNotes`, `CreateAuditNote` (201 + Location), `ExportPayslips` (202 + Location). DTOs match the openapi byte-for-shape: DECRYPT_FAIL money → JSON `null` (pointer no-omitempty); breakdown arrays `*[]T` (omitted on list, `[]` on detail decrypt-fail); only the 5 FE ops (no 405-immutable / PDF / forward-export).
- **Wiring** — `jobs.go` registers the worker in `NewWorkerClient` (pool in scope); `main.go` builds the crypto cipher from `cfg.Crypto.PayrollKey` + `jobs.NewInsertOnlyClient(pool)` (first API-process jobs client) + the payroll slice; `server.go` mounts the 5 routes under a single `RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin)` group with the two POSTs idempotency-wrapped.
- **Seed** — `seedPayroll`: FINAL payslips SWP-PS-90121 (Budi, 2025-12, full breakdown + benefits), 90122 (Rudi, 2025-12), 90123 (Andi, 2025-11), 90124 (Dewi, 2025-11) with money encrypted under `PAYROLL_ENCRYPTION_KEY`; the **DECRYPT_FAIL** row SWP-PS-90119 (raw garbage bytea ciphertext, working_days NULL); two audit notes on 90119 (Sari Hadi). Idempotent (`ON CONFLICT (id) DO NOTHING`; breakdown lines only on fresh insert).

## Reference for 10-03/10-04 (handoff)

- **Response shapes to assert (10-03):**
  - List: top-level `{data:[...], next_cursor, has_more}`; `meta.code: "MISSING_PAYROLL_HISTORY"` ONLY on `data:[]`. A DECRYPT_FAIL row in the list = `status:"DECRYPT_FAIL"`, `decrypt_fail:true`, `gross_*`/`take_home_pay`/`working_days` all `null`, `locked_reason:"decrypt_fail"`. FINAL rows carry the 2-decimal money strings. NO breakdown arrays on the list shape.
  - Detail: `{data:<Payslip>}` (FE unwraps `.data`). FINAL → `earnings`/`deductions`/`benefits` populated (each `{name, value, for_bpjs?, locked_reason?}`); DECRYPT_FAIL → all three are `[]`, money null.
  - Audit notes: list `{data:[...], next_cursor, has_more}` (oldest-first); create → 201 + `PayslipAuditNote` (`{payslip_id}-NOTE-{seq}`).
  - Export: 202 + `{id, status:"QUEUED", format:"XLSX", confidential:true, requested_at, requested_by:{id,name}, scope:{period,year,employee_ids}, poll_url}` + `Location: /api/v1/exports/{id}`.
- **Error codes:** `EXPORT_TOO_LARGE` 422, `RULE_VIOLATION` 422 (no period AND no year), `NOT_FOUND` 404 (missing payslip on note/detail), `FORBIDDEN` 403 (agent/shift_leader on any payroll endpoint).
- **Export E2E (10-04):** POST returns 202 + a `SWP-EXP-…` job id; the harness worker (already runs for notifications) processes `payslip.export`; assert `export_jobs.status = DONE` via the DB poll helper (no FE job-status hook in E8). The worker also stamps `row_count` + `artifact_ref`. `resetDb` must TRUNCATE `payslips`/`payslip_components`/`payslip_benefits`/`payslip_audit_notes`/`export_jobs` (without breaking River's own tables) — and the harness MUST set `PAYROLL_ENCRYPTION_KEY` (same key for API + seed).
- **Seed targets:** FINAL list/detail/export = SWP-PS-90121..90124; DECRYPT_FAIL render = SWP-PS-90119; audit-note list = SWP-PS-90119 (2 seeded notes).
- **RBAC negatives:** agent + shift_leader → 403 on all 5 endpoints (route-level RequireRole; no in-service scope branch since the web archive is global hr/super).

## Deviations from Plan

**Minor (within Rule 1–3 / Claude's discretion):**

1. **[Rule 3 - blocking] `httpx.PageResponse` has no `Meta` field.** The plan said attach `meta:{code}` to the page envelope; the shared `PageResponse[T]` only has `data/next_cursor/has_more`. Added a local `payslipPageResponse` struct mirroring it + an optional `meta` block. No shared type changed. (List shape stays byte-for-byte; meta only appears on empty.)
2. **Breakdown arrays modeled as `*[]T` (pointer-to-slice), not `[]T`.** The openapi omits `earnings/deductions/benefits` on the LIST shape but requires them (possibly `[]`) on DETAIL. A plain `[]T` with `omitempty` would wrongly omit the empty array on a DECRYPT_FAIL detail. `*[]T` gives: nil → omitted (list), `&[]` → `[]` (detail decrypt-fail), `&[...]` → populated (detail FINAL). Matches the contract exactly.
3. **Audit-note notify is a TODO(Phase-11) comment, not a real EnqueueTx.** CONTEXT/plan mention a "notify stub"; the established convention in leave/overtime/people is a TODO comment (no real notification enqueue until Phase-11). Followed that. The EXPORT path DOES really enqueue (its River job is the success criterion, not a notification).
4. **`author_name` left nil on audit-note create.** No E2 employee-name lookup seam in the service; the FE falls back to `author_id`. The seeded notes carry an explicit `author_name` ("Sari Hadi") so the list render shows the name; live-created notes will show the id until the E2 denorm is wired.

No architectural changes (Rule 4) were needed. No authentication gates encountered.

### Out-of-scope discoveries (logged, NOT fixed)
- The pre-existing `gofmt -l internal/` Phase-1–4 formatting drift noted in 10-01's deferred-items remains untouched; all 10-02 files are gofmt-clean.

## Verification

- `go build ./...` exits 0.
- `go vet ./...` exits 0.
- `make gen` (sqlc) clean (no query changes; regen no-op confirmed).
- `gofmt -l` clean across `internal/{service,handler,repository}/payroll`, `internal/platform/{crypto,jobs}`, `cmd/api/main.go`, `cmd/seed/seed.go`, `internal/server/server.go`.
- `go test ./internal/platform/crypto/` PASS (unchanged 10-01 suite).
- Route sanity: exactly 5 `d.Payroll.*` handlers mounted (list, detail, audit-notes list/create, export) under one `RequireRole(super_admin, hr_admin)` group; the two POSTs are `Idempotency.Handler`-wrapped; no 405-immutable / PDF / forward-export routes.
- jobs.go registers `PayslipExportWorker` (pool-backed) in `NewWorkerClient`; main.go constructs `jobs.NewInsertOnlyClient(pool)` + the crypto cipher.
- seed compiles and is part of the green `go build ./...`.

## Self-Check: PASSED

All 12 created files present on disk; all 3 task commits (12bc85d, 245f1d8, 7892dca) in git log.
