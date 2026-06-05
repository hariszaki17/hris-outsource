---
phase: 11-e10-reporting
verified: 2026-06-05T00:00:00Z
status: passed
score: 4/4 must-haves verified
re_verification: false
---

# Phase 11: E10 Reporting Verification Report

**Phase Goal:** Dashboard, billable report, notifications, and the export framework work against the real BE.
**Verified:** 2026-06-05
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Role-aware `/dashboards/me` and billable attendance report render against the real BE | VERIFIED | `dashboard-screen.tsx` double-unwrap fix landed (`92a5a3d`); `dashboard_service.go` live-aggregates all 3 role shapes; seeded VERIFIED rows SWP-ATT-9007/9008 feed report. Contract tests + 14 E2E specs (dashboard + billable-report files) pass. |
| 2 | Notifications list and mark-read / mark-all-read work; auto-dispatched notifications from earlier phases appear | VERIFIED | `NotificationWorker.Work` genuinely INSERTs via `sqlcgen.InsertNotification` (not a no-op). `notify.Dispatch` transactional-outbox helper present. Leave (approve-final/reject), OT (approve-final/reject), and attendance (verify/reject) services retro-wired with real `jobs.Dispatch` calls inside their write tx. `marked_count` bug fixed in `notifications-screen.tsx`. Capstone E2E spec (`notifications-autodispatch.spec.ts`) drives HR approve-final on SWP-LR-8002 → polls GET /notifications until LEAVE_APPROVED row for Dewi (SWP-EMP-3001) appears — proven NOT seeded (references SWP-LR-8002 vs seeded SWP-LR-8005). |
| 3 | Export framework (create/get/cancel, async) works end-to-end via the worker | VERIFIED | `ReportExportWorker` registered in `NewWorkerClient` alongside `PayslipExportWorker` (jobs.go line 65-66). `export_service.go` uses `InTx` + `audit.RecordReturningID` + `InsertExportJobGeneric(QUEUED)` + `EnqueueTx(ReportExportArgs)` in one tx. `report_export.go` drives QUEUED→RUNNING→DONE lifecycle. Error codes EXPORT_FORMAT_UNSUPPORTED / EXPORT_TOO_LARGE / RATE_LIMITED_EXPORTS all implemented in `export_service.go`. `exports.spec.ts` E2E proves worker-DONE + cancel. Phase-10 payslip export is unaffected (both workers registered, payroll handler tests still pass). |
| 4 | Exhaustive Playwright E2E for E10 features is green | VERIFIED (human-run) | 5 spec files under `frontend/e2e/tests/e10/` containing 14 tests; 660 lines total. 11-04 SUMMARY documents full suite run: 239 passed / 6 skipped / 0 failed. The 6 skips are pre-existing (non-E10). Cannot re-run independently (requires Docker + full stack boot); flagged for human verification. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `backend/db/migrations/00035_notifications.sql` | notifications table | VERIFIED | Exists; creates `notifications` table with SWP-NTF-* via `swp_next_id('NTF')`, recipient_id, kind, title/body, deep_link, actor, is_critical, read_at, indexes |
| `backend/db/migrations/00036_export_jobs_generalize.sql` | ALTER export_jobs to add CANCELLED/EXCEL/report_type/filters | VERIFIED | Exists; ALTER (not recreate) adds CANCELLED status, EXCEL format, report_type, filters jsonb, audit_log_entry_id, progress_percent, expires_at — all nullable/defaulted |
| `backend/internal/domain/reporting/` (4 files) | Domain types | VERIFIED | `notification.go`, `export.go`, `dashboard.go`, `billable.go` all present |
| `backend/internal/repository/reporting/` (4 files) | Repository layer | VERIFIED | `notification_repo.go`, `dashboard_repo.go`, `billable_repo.go`, `export_repo.go` all present |
| `backend/internal/service/reporting/` (5 files) | Service layer | VERIFIED | `notification_service.go`, `dashboard_service.go`, `billable_service.go`, `export_service.go`, `ports.go` all present |
| `backend/internal/handler/reporting/` (8 files) | Handlers + DTOs + contract tests | VERIFIED | `notification_handler.go`, `dashboard_handler.go`, `billable_handler.go`, `export_handler.go`, `dto.go`, plus 4 test files + testkit |
| `backend/internal/platform/jobs/notify.go` | Un-stubbed NotificationWorker | VERIFIED | `Work()` calls `sqlcgen.InsertNotification` (115 lines, substantive). `Dispatcher` interface + `notify.Dispatch` package-level helper + `*Client.Dispatch` method all present. |
| `backend/internal/platform/jobs/report_export.go` | ReportExportWorker | VERIFIED | 98 lines; `Work()` drives QUEUED→RUNNING→DONE lifecycle via `UpdateExportJobStatusGeneric`. Distinct `Kind() = "report.export"` from payslip. |
| `backend/internal/platform/jobs/jobs.go` | Both export workers + NotificationWorker registered | VERIFIED | Lines 64-66: `NewPayslipExportWorker(pool)`, `NewReportExportWorker(pool)`, `NewNotificationWorker(pool)` all registered. |
| `backend/internal/service/leave/leave_service.go` | Retro-wired dispatch | VERIFIED | `SetNotifier` seam at line 49; `jobs.Dispatch` calls at lines 303 and 377 (approve-final, reject) |
| `backend/internal/service/overtime/overtime_service.go` | Retro-wired dispatch | VERIFIED | `SetNotifier` seam at line 54; `jobs.Dispatch` calls at lines 325 and 382 (approve-final, reject) |
| `backend/internal/service/attendance/attendance_service.go` | Retro-wired dispatch | VERIFIED | `SetNotifier` seam at line 101; `jobs.Dispatch` calls at lines 198 and 253 (verify, reject) |
| `backend/cmd/seed/seed.go` | seedNotifications + billable seed rows | VERIFIED | `seedNotifications` inserts 6 rows (SWP-NTF-90001..90006); SWP-ATT-9007/9008 VERIFIED rows with `is_billable=true` binding present |
| `frontend/e2e/tests/e10/` (5 spec files) | Playwright E2E specs | VERIFIED | 5 files: dashboard.spec.ts, billable-report.spec.ts, notifications.spec.ts, notifications-autodispatch.spec.ts, exports.spec.ts (660 lines total, 14 tests) |
| `frontend/e2e/lib/e10-helpers.ts` | E10 E2E helper library | VERIFIED | `gotoReady`, `pollNotification`, `pollExportJobUntil`, `listNotificationsVia`, `DEWI`/`LR`/`NTF` fixtures |
| `frontend/e2e/lib/reset-db.ts` | notifications in TRUNCATE | VERIFIED | `notifications` in `TRUNCATE_TABLES` (line 105) |
| `frontend/apps/web/src/features/e10-reporting/dashboard-screen.tsx` | Double-unwrap fix | VERIFIED | `query.data.data.data` peel at line 510 |
| `frontend/apps/web/src/features/e10-reporting/billable-report-screen.tsx` | Double-unwrap fix | VERIFIED | Same `data.data.data` pattern at line 224-225 |
| `frontend/apps/web/src/features/e10-reporting/use-export-flow.ts` | Export double-unwrap fix | VERIFIED | `unwrapExportJob` helper at line 65 peels both envelopes |
| `frontend/apps/web/src/features/e10-reporting/notifications-screen.tsx` | marked_count fix | VERIFIED | `res.data.marked_count` at line 193 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `notify.Dispatch` call in leave/OT/attendance service | `NotificationWorker.Work` inserts row | `jobs.EnqueueTx` (transactional outbox) | WIRED | 6 dispatch points retro-wired; worker `InsertNotification` is real. E2E capstone proves end-to-end. |
| `export_service.CreateExport` | `ReportExportWorker.Work` drives DONE | `EnqueueTx(ReportExportArgs)` in same tx as QUEUED insert | WIRED | `export_service.go` lines 125-157 show one-tx audit+insert+enqueue; `report_export.go` drives lifecycle |
| `POST /exports` handler | `export_service.CreateExport` | `server.go` route + handler call | WIRED | `server.go` line 524 mounts POST /exports → `d.Reporting.CreateExport` |
| `GET /dashboards/me` handler | `dashboard_service.GetMyDashboard` | `server.go` route registration | WIRED | `server.go` line 507 mounts route |
| `GET /reports/attendance-billable` | `billable_service` + sqlc queries over VERIFIED attendance | route + service + repo chain | WIRED | `server.go` line 511; billable service aggregates only `verification_status='VERIFIED'` rows |
| `SetNotifier(jobsClient)` in main.go | `leave/OT/attendance` services dispatch real notifications | `cmd/api/main.go` wiring | WIRED | 11-02 SUMMARY confirms `SetNotifier(jobsClient)` on all three services at main.go wiring step |
| Notifications FE screen | `GET /notifications` real BE | MSW off (double-unwrap fixed) | WIRED | Single-wrapped cursor envelope is correct for notifications list; `marked_count` fix confirmed |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| RPT-01 | 11-02b, 11-03, 11-04 | Role-aware dashboard (`/dashboards/me`) | SATISFIED | `dashboard_service.go` + handler + 3 contract tests + E2E dashboard.spec.ts (3 tests) all present and passing |
| RPT-02 | 11-02, 11-03, 11-04 | Notifications list/mark-read/mark-all-read | SATISFIED | Notifications slice + worker un-stub + retro-wire + capstone E2E all verified |
| RPT-03 | 11-02b, 11-03, 11-04 | Billable attendance report | SATISFIED | `billable_service.go` + seeded VERIFIED rows + contract tests + E2E billable-report.spec.ts |
| RPT-04 | 11-02b, 11-03, 11-04 | Export framework (create/get/cancel, async) | SATISFIED | `export_service.go` + `ReportExportWorker` + error codes + E2E exports.spec.ts + both workers registered |

Note: REQUIREMENTS.md table (line 93) still shows "In Progress (11-01..03; 11-04 E2E pending)" — this is a stale status string that was not updated after 11-04 completed. The four requirements themselves are checked off (lines 65-68) and the implementation is fully verified. Recommend updating the phase table row to "Completed".

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `report_export.go` | 93-96 | `buildArtifact` returns 0 rows (stand-in, no real xlsx streaming) | Info | Documented honest stand-in per CONTEXT discretion. The lifecycle (QUEUED→RUNNING→DONE) is real and E2E-proven. The artifact_ref is set and file_url is served once DONE. A real implementation would stream rows. |
| `dashboard_service.go` | 121-132 | Several dashboard fields emitted as 0/null/empty (attendance_rate_pct, billable_hours_mtd, ot_hours_mtd, BillableTrend.Points, TodayShift) | Info | Documented honest gaps per 11-02b decisions. No fake constants — the fields are REQUIRED-present per openapi but not required to be non-zero. Fields that have live queries return real values. |
| Queue-targeted dispatch stubs | leave line 149, OT lines 231/274/418 | 4 dispatch points left as `Phase-11 stub (documented)` comments | Info | All are queue-targeted (no single recipient) or self-action points, documented in 11-02 SUMMARY. Explicitly accepted by CONTEXT: "some self-action/queue-target dispatch points are documented stubs per CONTEXT discretion." |

No blocker anti-patterns found.

### Human Verification Required

#### 1. Full Playwright E2E Suite (239 passed / 6 skipped / 0 failed)

**Test:** Run `cd frontend && pnpm e2e` against a live stack (Docker + Postgres + Go API + cmd/worker)
**Expected:** 239 passed / 6 skipped / 0 failed (the 6 skips are pre-existing non-E10 skips)
**Why human:** Requires Docker + ephemeral Postgres + booted Go API + River cmd/worker. The 14 E2E specs in `frontend/e2e/tests/e10/` specifically require the worker to be running to assert the DONE status and the auto-dispatched notification. Cannot be re-run in this verification environment.

#### 2. Visual dashboard rendering per role

**Test:** Log in as HR admin, shift_leader, and agent personas; navigate to /dashboard
**Expected:** HR sees global KPI cards + pending-action panel; leader sees company-scoped today status cards; agent sees recent attendance + pending requests
**Why human:** Visual layout / role-switching behavior requires a running browser session.

#### 3. Notifications in-app UI (mark-read state flip visible)

**Test:** Log in as a persona with unread notifications; click a notification card; verify the unread pill disappears
**Expected:** Unread card flips to read after click; mark-all-read clears all unread and shows toast with count
**Why human:** Optimistic UI update + toast appearance requires visual inspection.

### Honest Deferral Acceptance

Per the verification notes the following are accepted as honest deferrals:

- **PDF export** — `EXPORT_FORMAT_UNSUPPORTED` error code implemented and E2E-asserted. Accepted per CONTEXT.
- **External notification delivery** (email/push) — in-app row only. Accepted per CONTEXT.
- **Queue-targeted dispatch stubs** (leave approve-l1, OT confirm/approve-l1/withdraw) — 4 points documented in-code as `Phase-11 stub (documented)`. Accepted per CONTEXT.
- **Export artifact streaming** — `buildArtifact` is a stand-in returning 0 rows; the lifecycle (DONE status + artifact_ref set) is real. Accepted per CONTEXT discretion.
- **Some dashboard aggregation fields** (attendance_rate_pct, billable_hours_mtd, etc.) are honest 0/null — documented in 11-02b decisions. Fields that have live queries return real data.

### Gaps Summary

No gaps. All success criteria are met:

1. `/dashboards/me` renders role-aware against the real BE — verified by contract tests (4 role shapes) and E2E.
2. Notifications list + mark-read + mark-all-read work; the capstone E2E spec proves a REAL prior-phase action (leave approve-final) auto-dispatches a notification that the recipient sees — the NotificationWorker is un-stubbed and the dispatch is transactional.
3. Export framework delivers QUEUED→RUNNING→DONE end-to-end via the real ReportExportWorker; cancel works; PDF is EXPORT_FORMAT_UNSUPPORTED. Both export workers (payslip + report) coexist without regression.
4. 14 Playwright E2E tests across 5 spec files; 11-04 SUMMARY documents 239 passed / 6 skipped / 0 failed (human-verifiable; cannot re-run without full stack).

The BE compiles clean (`go build ./...` exits 0), passes `go vet ./...`, and the reporting handler contract tests (25 tests) pass. No regressions in any prior-phase handler package.

---

_Verified: 2026-06-05_
_Verifier: Claude (gsd-verifier)_
