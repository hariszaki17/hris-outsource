---
phase: 08-e6-leave
verified: 2026-06-05T00:00:00Z
status: human_needed
score: 4/4 must-haves verified (automated); E2E green per executor run — cannot re-run without Docker
re_verification: false
human_verification:
  - test: "Run full Playwright E2E suite (184 tests across e1-e6)"
    expected: "184 passed / 6 skipped / 0 failed — 21 new E6 tests including inv3-loop-closer green"
    why_human: "Requires Docker + live Postgres + frontend build; cannot execute headlessly in this environment"
---

# Phase 8: E6 Leave Verification Report

**Phase Goal:** Leave requests (multi-step approval), quotas, and the calendar work against the real BE.
**Verified:** 2026-06-05
**Status:** human_needed (all automated checks pass; E2E green is per executor's documented run)
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | Leave requests can be listed/detailed and moved through L1/final/override/reject with correct state transitions | VERIFIED | `go test ./internal/handler/leave/... ./internal/service/leave/... -count=1` exits 0; 30 contract tests + 9 service unit tests assert all transitions; 10 routes wired in server.go L396-410 |
| 2  | Quotas list/adjust/bulk-grant work; quota-exceeded returns 422; BALANCE_RECHECK_FAILED is contract-correct | VERIFIED | `BALANCE_RECHECK_FAILED` (line 459 leave_service.go) + `requires_override` field confirmed; `QUOTA_EXCEEDED` seam in quota_service.go; adjust refuses total<used (RULE_VIOLATION 422); bulk-grant partial-success envelope confirmed by contract tests |
| 3  | Leave calendar renders for the requested range | VERIFIED | `GET /leave-calendar` route wired; CalendarService + calendar_handler.go + ListCalendarEntries sqlc query all exist and are substantive; show_pending toggle confirmed in contract tests |
| 4  | Exhaustive Playwright E2E for E6 features is green | HUMAN_NEEDED | All 5 spec files exist and are substantive (689 total lines across 5 specs + e6-helpers); 10 commits verified in git; executor documents 184 passed / 6 skipped / 0 failed — cannot independently re-run (needs Docker + full stack) |

**Score:** 4/4 truths verified (SC4 deferred to human)

---

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `backend/db/migrations/00028_leave_requests.sql` | VERIFIED | Exists; status CHECK enum, SWP-LR id via DEFAULT, routing/balance_check cols, soft-delete, indexes |
| `backend/db/migrations/00029_leave_quotas.sql` | VERIFIED | Exists; total/used/pending triple, unique (employee_id,leave_type_id,period), jsonb cols |
| `backend/db/migrations/00030_leave_approvals.sql` | VERIFIED | Exists; bigserial decision trail, stage/decision CHECK, is_override cols |
| `backend/db/queries/leave/leave_requests.sql` | VERIFIED | Exists |
| `backend/db/queries/leave/leave_quotas.sql` | VERIFIED | Exists |
| `backend/db/queries/leave/leave_approvals.sql` | VERIFIED | Exists |
| `backend/db/queries/leave/leave_calendar.sql` | VERIFIED | Exists |
| `backend/internal/domain/leave/leave.go` | VERIFIED | Exists |
| `backend/internal/service/leave/leave_service.go` | VERIFIED | 531 lines; full state machine with InTx, state guards, INV-3 integration |
| `backend/internal/service/leave/quota_service.go` | VERIFIED | 311 lines; list/adjust/bulk-grant/bulk-preview implemented |
| `backend/internal/service/leave/calendar_service.go` | VERIFIED | Exists |
| `backend/internal/service/leave/ports.go` | VERIFIED | Exists; SchedulePort defined here (correct Go structural typing choice) |
| `backend/internal/service/leave/helpers.go` | VERIFIED | Contains dtoNewStatus mapping CANCELLED_BY_LEAVE→LEAVE |
| `backend/internal/service/leave/leave_service_test.go` | VERIFIED | Exists (unit tests for service-layer) |
| `backend/internal/repository/leave/leave_repo.go` | VERIFIED | 171 lines; substantive |
| `backend/internal/repository/leave/quota_repo.go` | VERIFIED | Exists |
| `backend/internal/repository/leave/mapping.go` | VERIFIED | Exists |
| `backend/internal/handler/leave/handler.go` | VERIFIED | Exists |
| `backend/internal/handler/leave/leave_handler.go` | VERIFIED | 138 lines |
| `backend/internal/handler/leave/quota_handler.go` | VERIFIED | Exists |
| `backend/internal/handler/leave/calendar_handler.go` | VERIFIED | Exists |
| `backend/internal/handler/leave/dto.go` | VERIFIED | Exists |
| `backend/internal/handler/leave/leave_testkit_test.go` | VERIFIED | Exists; fakeScheduleRepo records INV-3 calls |
| `backend/internal/handler/leave/leave_handler_test.go` | VERIFIED | Exists |
| `backend/internal/handler/leave/quota_handler_test.go` | VERIFIED | Exists |
| `backend/internal/handler/leave/calendar_handler_test.go` | VERIFIED | Exists |
| `backend/db/queries/scheduling/approved_leave_days.sql` (modified) | VERIFIED | InsertApprovedLeaveDay + DeleteApprovedLeaveDaysForRequest added; ON CONFLICT upsert present |
| `backend/db/queries/scheduling/schedule_entries.sql` (modified) | VERIFIED | CancelScheduleEntriesForLeave at line 115; writes `CANCELLED_BY_LEAVE` (NOT 'LEAVE'); idempotency guard `status <> 'CANCELLED_BY_LEAVE'` present |
| `backend/cmd/seed/seed.go` (modified) | VERIFIED | seedLeave() present; SWP-LR-8001..8007 seeded; SWP-LR-8007 confirmed as INV-3 overlap target (PENDING_HR, start=end=wednesday, overlapping SWP-SCH-6002) |
| `frontend/e2e/lib/e6-helpers.ts` | VERIFIED | Exists |
| `frontend/e2e/tests/e6/approvals.spec.ts` | VERIFIED | 230 lines |
| `frontend/e2e/tests/e6/quotas.spec.ts` | VERIFIED | 163 lines |
| `frontend/e2e/tests/e6/calendar.spec.ts` | VERIFIED | 72 lines |
| `frontend/e2e/tests/e6/scope-negatives.spec.ts` | VERIFIED | 98 lines |
| `frontend/e2e/tests/e6/inv3-loop-closer.spec.ts` | VERIFIED | 126 lines; asserts pre-DOUBLE_SHIFT, approve-final schedule_impact new_status='LEAVE', post-SHIFT_OVER_LEAVE, CANCELLED_BY_LEAVE GET |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `leave_service.go ApproveFinal` | `approved_leave_days` | `InsertApprovedLeaveDay` inside `InTx` | VERIFIED | Line 248: `s.schedule.InsertApprovedLeaveDay(ctx, tx, ...)` inside the InTx block alongside UpdateLeaveRequestStatus + DeductLeaveQuota |
| `leave_service.go ApproveFinal` | `schedule_entries` | `CancelScheduleEntriesForLeave` inside `InTx` | VERIFIED | Line 230: `s.schedule.CancelScheduleEntriesForLeave(ctx, tx, ...)` in same InTx |
| `CancelScheduleEntriesForLeave` DB write | DTO `new_status='LEAVE'` | `dtoNewStatus()` in helpers.go | VERIFIED | `dtoNewStatus("CANCELLED_BY_LEAVE") == "LEAVE"` confirmed at line 473; called at line 240 |
| `server.go` | `leave handler` | `d.Leave.*` 10 routes | VERIFIED | Lines 396-410: all 10 endpoints mounted with correct RequireRole + Idempotency positions |
| `BALANCE_RECHECK_FAILED` 422 | `requires_override` field | apperr in leave_service.go | VERIFIED | Line 459-463: error code + requires_override field present |
| `fakeScheduleRepo` | INV-3 side-effect assertions | recording pattern in testkit | VERIFIED | fakeScheduleRepo records cancelCalls + insertedDays; used in TestApproveFinal_DeductsAndFiresINV3 |

---

### INV-3 Loop-Closer Honest Delivery Confirmation

**CancelScheduleEntriesForLeave writes valid CHECK value:**
The SQL at `backend/db/queries/scheduling/schedule_entries.sql` line 124 writes `status = 'CANCELLED_BY_LEAVE'`. The migration 00024 CHECK constraint permits exactly `('SCHEDULED','CANCELLED_BY_LEAVE','MODIFIED')`. The raw value `'LEAVE'` is NEVER written to the DB. Confirmed honest.

**`dtoNewStatus()` mapping:**
`helpers.go` line 472-475 maps `"CANCELLED_BY_LEAVE" → "LEAVE"` only at the DTO boundary. The DB value and the DTO value are never conflated. Confirmed honest.

**InsertApprovedLeaveDay writes a real approved_leave_days row:**
`approved_leave_days.sql` lines 18-21 INSERT with ON CONFLICT upsert. The leave_request_id inserted is the real `leave_requests.id` string (from the approved request), not the Phase-6 fixture placeholder. Confirmed honest.

**All INV-3 writes in the same tx as the approval:**
`leave_service.go` ApproveFinal block: InTx wraps UpdateLeaveRequestStatus + DeductLeaveQuota + CancelScheduleEntriesForLeave + InsertApprovedLeaveDay + InsertLeaveApproval. Atomicity confirmed.

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| LVE-01 | 08-02, 08-03, 08-04 | Leave requests — list/detail, multi-step approve (L1/final/override)/reject | SATISFIED | 6 leave-request endpoints implemented + 30 contract tests + 9 E2E approvals tests |
| LVE-02 | 08-02, 08-03, 08-04 | Leave quotas — list/adjust/bulk-grant (quota-exceeded → 422) | SATISFIED | 3 quota endpoints + BALANCE_RECHECK_FAILED + QUOTA_EXCEEDED + RULE_VIOLATION all confirmed; 5 E2E quota tests |
| LVE-03 | 08-02, 08-03, 08-04 | Leave calendar | SATISFIED | 1 calendar endpoint + show_pending toggle + clash detection + 2 E2E calendar tests |

---

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `leave_service.go` L141, L289, L350 | `TODO(Phase-11): enqueue NotificationArgs` | INFO | Accepted deferral per CONTEXT locked decision — notification dispatch is a Phase-11 stub across all epics; not a blocker |

No placeholder components, no empty return null handlers, no stub API routes found.

---

### Build / Test Results (Automated)

- `go build ./...` — EXIT 0 (confirmed live)
- `go vet ./...` — EXIT 0 (confirmed live)
- `go test ./internal/handler/leave/... ./internal/service/leave/... -count=1` — all PASS
  - handler/leave: 19 tests PASS (30 contract tests per SUMMARY, test runner reports 19 test functions; multi-table-driven counts as one per runner)
  - service/leave: 9 tests PASS
  - No regressions in e1..e7 packages (per 08-03 SUMMARY; go test ./... green)

---

### Human Verification Required

#### 1. Full Playwright E2E Suite (E6 + Regression)

**Test:** Start Docker Compose (Postgres + API + FE), run `npx playwright test --workers=1` from `frontend/`
**Expected:** 184 passed / 6 skipped / 0 failed; specifically:
- `e6/approvals.spec.ts` — 9 tests green (L1-forward, HR-final, L1→final E2E, reject, override, PENDING_HR list, no-leader badge)
- `e6/quotas.spec.ts` — 5 tests green (remaining math, adjust happy, adjust refuse 422, bulk-grant preview→apply, balance-recheck→override)
- `e6/calendar.spec.ts` — 2 tests green (empty-default, toggle-ON renders grid+cell)
- `e6/scope-negatives.spec.ts` — 4 tests green (leader cross-company 403, list 403, queue-hidden, HR global 200)
- `e6/inv3-loop-closer.spec.ts` — 1 test green (pre-DOUBLE_SHIFT → approve → post-SHIFT_OVER_LEAVE → GET CANCELLED_BY_LEAVE)
**Why human:** Requires Docker-managed Postgres + API server boot + Vite frontend + live network calls; cannot execute headlessly in this environment

---

### Gaps Summary

No gaps found. All automated checks pass:
- All 33 declared artifacts exist and are substantive (not stubs)
- All 6 key links are wired, including the INV-3 atomicity guarantee
- LVE-01, LVE-02, LVE-03 are satisfied
- `go build`, `go vet`, `go test` are all green
- Notification stubs are an accepted locked deferral (Phase-11)
- The INV-3 loop-closer writes the correct CHECK-valid DB value (`CANCELLED_BY_LEAVE`, not `'LEAVE'`) and maps to the DTO value only at the service boundary
- All 10 commits documented in SUMMARYs are confirmed present in git history

The single human-needed item is re-running the Playwright E2E suite to independently confirm the 184-passed figure the executor documented.

---

_Verified: 2026-06-05_
_Verifier: Claude (gsd-verifier)_
