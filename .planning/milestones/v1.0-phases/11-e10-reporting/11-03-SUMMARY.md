---
phase: 11-e10-reporting
plan: 03
subsystem: api
tags: [e10, reporting, contract-tests, drift-gate, notifications, dashboard, billable, exports, rbac, cursor]

requires:
  - phase: 11-02
    provides: NotificationService + handler (list/mark-read/mark-all-read) + the notify dispatch outbox
  - phase: 11-02b
    provides: DashboardService/BillableService/ExportService + handler (DB↔wire status map, audit.RecordReturningID, export error codes, role-shaped dashboard) + ReportExportArgs outbox
  - phase: 10-e8-payroll
    provides: the Phase-10 payroll testkit harness pattern (fakeTx + fakeTxRunner + fake repos + recording fakeJobs + stubIdempotency + mutable principal) — copied verbatim
provides:
  - reporting drift gate — Go contract tests over the REAL reporting services+handlers (fake repos + recording fakeJobs + mutable-principal chi harness) for all 8 FE-used E10 ops + export error codes + RBAC + cursor envelopes
  - newHarness(role, company, employee) reporting testkit (the reusable E10 contract-test harness)
affects:
  - 11-04 (Playwright E2E: the contract tests are the BE regression gate for the FE wiring; the wire shapes asserted here are what the FE client is generated from)

tech-stack:
  added: []
  patterns:
    - "fakeTx.QueryRow returns a fakeRow scanning a deterministic SWP-AL id so audit.RecordReturningID is exercised honestly through the real export tx (the export path RETURNs an id, unlike payroll's Exec-only audit.Record)"
    - "recording fakeJobs.EnqueueTx captures the ReportExportArgs so the export-create test proves the transactional outbox (exactly one args whose JobID == the 202 body id)"
    - "DB→wire status mapping asserted at the contract boundary: seed export_jobs RUNNING/DONE → assert wire PROCESSING/COMPLETED"

key-files:
  created:
    - backend/internal/handler/reporting/reporting_testkit_test.go
    - backend/internal/handler/reporting/notification_handler_test.go
    - backend/internal/handler/reporting/dashboard_handler_test.go
    - backend/internal/handler/reporting/billable_handler_test.go
    - backend/internal/handler/reporting/export_handler_test.go
  modified: []

key-decisions:
  - "fakeTx needed a real QueryRow (not the payroll panic stub) because audit.RecordReturningID uses INSERT ... RETURNING id via tx.QueryRow; a fakeRow scans the SWP-AL id into the *string dest so the export tx + audit_log_entry_id capture run end-to-end over the REAL service"
  - "Harness mirrors server.go router positions EXACTLY: notifications + dashboard = all 4 roles; billable + exports = super/hr/leader (agent excluded → the agent POST /exports 403 is a real RBAC denial, not a forced assertion)"
  - "verification_rate_pct asserted as a computed range (98.0–99.0 for 612 verified / 8 pending) rather than a hardcoded constant — the service computes verified/(verified+pending)*100; pinning the exact float would be brittle without weakening honesty"

requirements-completed: [RPT-01, RPT-02, RPT-03, RPT-04]

duration: 6min
completed: 2026-06-05
---

# Phase 11 Plan 03: E10 Reporting Contract Tests (Drift Gate) Summary

**The E10 drift gate: Go contract tests over the REAL reporting services + handlers (via in-memory fake repos + a recording fakeJobs + a mutable-principal chi harness) asserting the wire shapes match `docs/api/E10-reporting/openapi.yaml` for all 8 FE-used ops + the export error codes + RBAC + cursor envelopes — 1 testkit + 4 handler test files, `go test ./internal/handler/reporting/...` green, no regressions.**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-06-05T08:17:44Z
- **Completed:** 2026-06-05T08:23:27Z
- **Tasks:** 2
- **Files:** 5 created

## Accomplishments

- **reporting_testkit_test.go** — `newHarness(role, companyID, employeeID)` on chi with the mutable-principal closure middleware + `stubIdempotency` at the server.go router positions, over fake repos (`fakeNotificationRepo` seeded set keyed by recipient with keyset cursor; `fakeDashboardRepo` configurable counts; `fakeBillableRepo` configurable aggregate/summary/pending + countInScope; `fakeExportRepo` insert/get/cancel + countRecent) + the REAL `NotificationService/DashboardService/BillableService/ExportService` + handler + a **recording `fakeJobs`**. Copies the Phase-10 payroll testkit (fakeTx/fakeTxRunner/decodeBody/captureRW idempotency) with one addition: `fakeTx.QueryRow → fakeRow` for `audit.RecordReturningID`.
- **notification_handler_test.go** — GET cursor envelope (`data/next_cursor/has_more`) + scope=self (foreign-recipient row excluded) + `read_state=UNREAD` + `kind` filter + cursor pagination; `:mark-read` flips `read_at` then no-ops (COALESCE) + non-owned → 404; `:mark-all-read` → `marked_count` (own unread only, foreign excluded) + UNREAD-then-empty.
- **dashboard_handler_test.go** — all 4 role shapes: HR (`role_label "HR Admin"`, kpis, 4-row panel with the EXACT openapi deep-link paths), super_admin (same shape, `role_label "Super Admin"`, empty panel), shift_leader (company-scoped `today`/`pending_counts`/company deep links), agent (`recent_attendance`/`pending_requests`/`recent_notifications_unread`). Cache-Control assertion included.
- **billable_handler_test.go** — `summary`+`pending_summary{pending_records,pending_hours_estimate,note}`+`rows[]` (BillableReportRow fields); `verification_rate_pct` null on the empty-after-filters case + present (~98.7) otherwise; leader cross-company → 403 OUT_OF_SCOPE; >1yr range → 422 REPORT_PERIOD_TOO_WIDE with `fields.period_end`.
- **export_handler_test.go** — POST /exports 202 + the recording fakeJobs asserts **exactly one `ReportExportArgs` with `JobID == the 202 body id`** (outbox proof) + `audit_log_entry_id` + echoed filters; `EXPORT_FORMAT_UNSUPPORTED` 422 (PDF, `fields.format`); `EXPORT_TOO_LARGE` 422 (`fields.period_end`); `RATE_LIMITED_EXPORTS` 429; GET /exports/{id} DB RUNNING→PROCESSING + DB DONE→COMPLETED (+file_url/filename/size_bytes) + non-owner 404; `:cancel` QUEUED→CANCELLED + terminal no-op; agent POST /exports → 403.

## Task Commits

1. **Task 1: reporting testkit + notifications/dashboard/billable contract tests** — `437cddd` (test)
2. **Task 2: export framework contract tests (202 + outbox + codes + status map)** — `d619047` (test)

**Plan metadata:** (this commit) `docs(11-03): complete E10 contract-tests plan`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] fakeTx.QueryRow panicked under the export tx (audit.RecordReturningID)**
- **Found during:** Task 2 (first `go test` of export create)
- **Issue:** The Phase-10 payroll testkit's `fakeTx.QueryRow` is a `panic(...)` stub because payroll's `audit.Record` only `tx.Exec`s. The E10 export service calls `audit.RecordReturningID`, which does `INSERT ... RETURNING id` via `tx.QueryRow(...).Scan(&id)` — so the copied stub panicked inside the real export tx.
- **Fix:** Implemented `fakeTx.QueryRow` to return a `fakeRow{id: "SWP-AL-1204520"}` whose `Scan` writes the id into the single `*string` dest. The export create + the `audit_log_entry_id` capture now run end-to-end over the REAL service (no assertion weakened).
- **Files modified:** `backend/internal/handler/reporting/reporting_testkit_test.go`
- **Verification:** `go test ./internal/handler/reporting/...` exits 0.
- **Committed in:** `d619047` (Task 2 commit)

### Environment note (not a deviation)

- **`.planning/` lives at the repo ROOT, not under `backend/`** — the executor cwd is `backend/`, so plan/state/summary paths resolve one level up (same finding as 11-01/11-02/11-02b). Read/wrote with the absolute repo-root path; no functional impact.

**Total deviations:** 1 auto-fixed (1 blocking) + 1 environment note. No architectural changes; no auth gates; no assertions weakened.

## Verification snapshot

- `go build ./...` exits 0.
- `go test ./internal/handler/reporting/...` exits 0 — all E10 contract tests green (dashboard role-aware shapes; billable aggregation + REPORT_PERIOD_TOO_WIDE 422 + leader OUT_OF_SCOPE 403; notifications list/mark-read/mark-all-read incl. marked_count + recipient-scope 404; export create 202 + outbox enqueue + DB→wire PROCESSING/COMPLETED + get + cancel→CANCELLED + EXPORT_FORMAT_UNSUPPORTED/EXPORT_TOO_LARGE/RATE_LIMITED_EXPORTS; RBAC; cursor shapes).
- `go test ./... -count=1` exits 0 — no regressions in any earlier package (payroll/overtime/leave/scheduling/placement/people/identity/crypto all `ok`).

## Self-Check: PASSED

- All 5 created files exist on disk (testkit + 4 handler test files).
- Both task commits exist: `437cddd` (testkit + notifications/dashboard/billable), `d619047` (export framework).
- Greps confirm: `newHarness`, `EXPORT_FORMAT_UNSUPPORTED`, `marked_count`, `ReportExportArgs` present across the test files.
- `go test ./... -count=1` exits 0 (full suite, no regression).

## Next Phase Readiness

- **11-04** (Playwright full-stack E2E, the milestone capstone): the wire shapes asserted here ARE what the FE `@swp/api-client` is generated from. E2E drives: dashboard role-aware (leader own-company vs HR global, deep links), billable report against the seeded VERIFIED rows, notifications list + mark-read + mark-all-read + an AUTO-DISPATCHED notification after a real action, export create → poll until export_jobs DONE (harness-spawned cmd/worker runs ReportExportWorker) + cancel. **reset-db must TRUNCATE notifications + export_jobs** (flagged in 11-02/11-02b), keeping River internal tables intact.

---
*Phase: 11-e10-reporting*
*Completed: 2026-06-05*
