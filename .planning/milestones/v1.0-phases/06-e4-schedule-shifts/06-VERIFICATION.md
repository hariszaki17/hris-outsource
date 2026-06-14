---
phase: 06-e4-schedule-shifts
verified: 2026-06-05T00:00:00Z
status: human_needed
score: 4/4 must-haves verified
human_verification:
  - test: "Run pnpm e2e headless against a live stack (real Postgres + Go API + Vite dev server)"
    expected: "145 passed, 0 failed — including all 27 E4 tests across shift-masters, schedule-grid, conflict-negatives, bulk-apply, leader-scope specs"
    why_human: "Playwright E2E requires Docker/Postgres boot and a running API; cannot execute programmatically in this environment. The executor's run documented 145 passed / 0 failed (.last-run.json status=passed, failedTests=[]); the test files, wiring fixes, and reset-db extensions are verified on disk. Human re-run confirms the live integration."
---

# Phase 6: E4 Schedule & Shifts Verification Report

**Phase Goal:** Shift masters and scheduling (with conflict checks + bulk apply) work against the real BE.
**Verified:** 2026-06-05
**Status:** human_needed — all automated checks passed; one item requires a live Playwright run to confirm.
**Re-verification:** No — initial verification.

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | HR/leader can manage shift masters and schedule entries (create/update/delete) against the real BE | VERIFIED | 11 E4 endpoints mounted in server.go under correct RBAC groups; shift_master_service.go (391 lines) + schedule_service.go (552 lines) implement full CRUD; `go build ./... && go vet ./...` clean; 21 contract tests pass |
| 2 | Conflict check returns double-shift / over-leave / outside-placement-period violations with the correct codes; bulk apply reports partial success | VERIFIED | All 6 codes (OUTSIDE_PLACEMENT_PERIOD/OUT_OF_SCOPE/SHIFT_DEACTIVATED/SHIFT_NOT_FOR_SERVICE_LINE/SHIFT_OVER_LEAVE/DOUBLE_SHIFT) present in conflict_engine.go with correct HTTP statuses (403/422/422/422/409/409); BulkApply loops per-cell-in-own-tx; 200-if-succeeded>=1 else 422 policy; all 6 codes asserted in contract tests (TestConflict_* suite) — all PASS |
| 3 | Schedule lists are cursor-paginated and scoped (leader sees own company) | VERIFIED | ShiftMasters uses id-desc keyset cursor (ListShiftMasters comment in shift_master_service.go line 80); ListSchedule scoped via GuardCompany (schedule_service.go lines 100, 322); OUT_OF_SCOPE path tested (TestScope_OutOfScope PASS); leader-scope E2E spec covers SCOPE-403-create/list/hr-global |
| 4 | Exhaustive Playwright E2E for E4 features is green | HUMAN_NEEDED | .last-run.json status="passed", failedTests=[]; 27 tests across 5 spec files confirmed on disk (8+7+5+4+3=27); executor's run documents 145 passed / 0 failed on 2026-06-04; cannot independently re-run without live stack |

**Score:** 3/4 truths fully verifiable programmatically; 4/4 truths satisfied per evidence (E2E truth flagged human-needed for independent re-run confirmation).

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `backend/db/migrations/00023_shift_masters.sql` | shift_masters DDL | VERIFIED | File present on disk |
| `backend/db/migrations/00024_schedule_entries.sql` | schedule_entries DDL + INV-1 partial unique index | VERIFIED | File present on disk |
| `backend/db/migrations/00025_approved_leave_days.sql` | E4-owned over-leave read source | VERIFIED | File present on disk |
| `backend/db/queries/scheduling/shift_masters.sql` | shift-master sqlc queries | VERIFIED | File present on disk |
| `backend/db/queries/scheduling/schedule_entries.sql` | schedule CRUD + conflict queries | VERIFIED | File present on disk |
| `backend/db/queries/scheduling/approved_leave_days.sql` | over-leave lookup | VERIFIED | File present on disk |
| `backend/internal/domain/scheduling.go` | domain entity + filter types | VERIFIED | File present on disk |
| `backend/internal/repository/scheduling/mapping.go` | pgtype boundary conversions | VERIFIED | File present on disk |
| `backend/internal/repository/scheduling/shift_master_repo.go` | ShiftMasterRepo impl | VERIFIED | File present on disk |
| `backend/internal/repository/scheduling/schedule_repo.go` | ScheduleRepo impl (includes FindApprovedLeaveForAgentDate) | VERIFIED | File present on disk |
| `backend/internal/service/scheduling/conflict_engine.go` | Ordered 6-check Evaluate function | VERIFIED | 289 lines; all 6 codes present with correct HTTP status annotations |
| `backend/internal/service/scheduling/shift_master_service.go` | Shift-master CRUD + deactivate/reactivate | VERIFIED | 391 lines; cursor pagination documented |
| `backend/internal/service/scheduling/schedule_service.go` | Schedule CRUD + BulkApply + Check | VERIFIED | 552 lines; BulkApply at line 432; GuardCompany scope at lines 100, 322 |
| `backend/internal/handler/scheduling/shift_master_handler.go` | HTTP handlers for shift masters | VERIFIED | 217 lines |
| `backend/internal/handler/scheduling/shift_master_dto.go` | Shift-master DTOs | VERIFIED | File present on disk |
| `backend/internal/handler/scheduling/schedule_handler.go` | HTTP handlers for schedule | VERIFIED | 276 lines |
| `backend/internal/handler/scheduling/schedule_dto.go` | Schedule DTOs | VERIFIED | File present on disk |
| `backend/internal/handler/scheduling/scheduling_testkit_test.go` | In-memory fake repos + test harness | VERIFIED | File present on disk |
| `backend/internal/handler/scheduling/shift_master_handler_test.go` | Shift-master contract tests | VERIFIED | File present on disk |
| `backend/internal/handler/scheduling/schedule_handler_test.go` | Schedule + conflict contract tests | VERIFIED | File present on disk |
| `backend/internal/server/server.go` | 11 E4 routes mounted under RBAC groups | VERIFIED | Lines 332-349: all 11 endpoints confirmed, correct RequireRole groupings |
| `backend/cmd/api/main.go` | Scheduling slice wired (repos/services/handler) | VERIFIED | Lines 145-164: shiftMasterRepo, scheduleRepo, shiftMasterSvc, scheduleSvc, schedulingHandler all constructed and injected |
| `backend/cmd/seed/seed.go` | SWP-SHF-001/002 + SWP-SCH-6001/6002 + SWP-LR-44210 approved_leave_days | VERIFIED | Fixtures confirmed at lines 1024-1025, 1054-1055, 1066-1072 |
| `frontend/e2e/lib/e4-helpers.ts` | E4 DOM selectors + seed-anchored dates + waitForToken | VERIFIED | 183 lines; file present on disk |
| `frontend/e2e/tests/e4/shift-masters.spec.ts` | 8 shift-master E2E tests | VERIFIED | 261 lines; 8 top-level tests confirmed |
| `frontend/e2e/tests/e4/schedule-grid.spec.ts` | 7 schedule-grid E2E tests | VERIFIED | 216 lines; 7 top-level tests confirmed |
| `frontend/e2e/tests/e4/conflict-negatives.spec.ts` | 5 conflict-negative E2E tests | VERIFIED | 179 lines; 5 top-level tests confirmed |
| `frontend/e2e/tests/e4/bulk-apply.spec.ts` | 4 bulk-apply E2E tests | VERIFIED | 174 lines; 4 top-level tests confirmed |
| `frontend/e2e/tests/e4/leader-scope.spec.ts` | 3 leader-scope E2E tests | VERIFIED | 88 lines; 3 top-level tests confirmed |
| `frontend/apps/web/src/features/e4-scheduling/schedule-overlays.tsx` | FE wiring fix: reads error.details (not conflict_details) | VERIFIED | Line 204: `first?.error?.details` confirmed; comment at lines 200-201 explains the fix |
| `frontend/e2e/lib/reset-db.ts` | E4 tables truncated (schedule_entries, approved_leave_days, shift_masters) | VERIFIED | Lines 79-81: all three E4 tables present in TRUNCATE_TABLES before placements/employees |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `conflict_engine.go` | `approved_leave_days` table | `FindApprovedLeaveForAgentDate` real repo call at line 230 | WIRED | Not mocked; the interface method is called on the live repo in Evaluate(); seed plants SWP-LR-44210 so the branch fires genuinely |
| `schedule_service.go` | `conflict_engine.go` | `Evaluate()` called from CreateEntry/UpdateEntry/BulkApply/CheckEntry | WIRED | `Evaluate` is the shared evaluator; schedule_service imports conflict engine package |
| `main.go` | `scheduling` slice | schedulingRepo/Svc/Handler construction at lines 145-164 | WIRED | All three layers (repo, service, handler) constructed and injected into server Deps struct |
| `server.go` | 11 E4 endpoints | chi router at lines 332-349 under RequireRole groups | WIRED | Shift-master reads + all schedule ops under hr/leader/super_admin; shift-master writes under hr/super_admin only |
| `schedule-overlays.tsx` | real BE `:check` response | `first?.error?.details` at line 204 | WIRED | Bug was fixed (was `conflict_details`); block messages now render for DOUBLE_SHIFT/over-leave |
| `reset-db.ts` | E4 Postgres tables | TRUNCATE_TABLES array lines 79-81 | WIRED | FK-safe order: schedule_entries → approved_leave_days → shift_masters before placements |
| `conflict-negatives.spec.ts` | SWP-LR-44210 seed row | `errorDetails(res.body)?.leave_request_id === 'SWP-LR-44210'` at line 113 | WIRED | E2E asserts the real seeded leave_request_id, not a mock value |

---

## Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| SCH-01 | Shift masters — list/create/update/deactivate/reactivate | SATISFIED | 5 endpoints in server.go (GET list, GET by id, POST, PATCH, deactivate, reactivate); shift_master_service.go implements all ops; TestListShiftMasters_Envelope, TestCreateShiftMaster_*, TestDeactivateReactivate all PASS |
| SCH-02 | Schedule entries — list/create/update/delete | SATISFIED | 4 endpoints in server.go (GET list, POST, PATCH, DELETE); schedule_service.go implements all ops; TestCreateScheduleEntry_Success, TestListSchedule_Envelope, TestDelete_204 all PASS |
| SCH-03 | Conflict check + bulk apply (double-shift / over-leave / outside-placement guards) | SATISFIED | conflict_engine.go implements all 6 ordered checks; BulkApply at schedule_service.go line 432 does per-cell-atomic writes; TestConflict_* suite covers all 6 codes; TestBulkApply_PartialSuccess, TestBulkApply_AllFailed_422, TestBulkApply_WeekdaysMask all PASS |

No orphaned requirements found. REQUIREMENTS.md maps only SCH-01..03 to Phase 6, all checked [x].

---

## Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `schedule_service.go` lines 195, 292, 340 | `TODO(Phase-11): dispatch notification` | INFO | Phase-11 notification stubs — explicitly scoped future work, not a gap for Phase 6. Does not affect any Phase 6 success criterion. |
| `internal/domain/identity.go`, `internal/domain/people.go` | Pre-existing `gofmt -l` drift (pre-Phase-6) | INFO | Logged in deferred-items.md; not introduced by Phase 6 changes; does not affect build or correctness. |

No blockers. No warnings affecting Phase 6 goal.

---

## SHIFT_OVER_LEAVE Honesty Assessment

The over-leave conflict is delivered honestly:

1. `approved_leave_days` is a real Postgres table (migration 00025), not a mock fixture or test double.
2. `conflict_engine.go` line 230 calls `repo.FindApprovedLeaveForAgentDate()` — the real repository interface backed by a real sqlc query (`approved_leave_days.sql`).
3. `seed.go` lines 1066-1072 insert `employee_id=SWP-EMP-3001 / leave_date=monday+3 / leave_request_id=SWP-LR-44210 / leave_type=ANNUAL` into that table via `ON CONFLICT DO NOTHING`.
4. `conflict-negatives.spec.ts` line 113 asserts `errorDetails(res.body)?.leave_request_id === 'SWP-LR-44210'` — the exact seeded value returned by the real query.
5. The `leave_request_id` column carries no FK into E6's `leave_requests` / SWP-LR namespace, so the seed does not pre-empt Phase 8 (E6 Leave). E6 will supersede this table with the production source; the stand-in is documented clearly in the migration and summary.

The SHIFT_OVER_LEAVE path is not mocked at any level of the stack.

---

## Human Verification Required

### 1. Full E2E Suite Re-Run

**Test:** From the project root with Docker running, execute `pnpm --filter web e2e` (or equivalent `pnpm e2e` from `frontend/`) against a fresh DB seeded by `go run ./cmd/seed`.
**Expected:** 145 passed / 0 failed; all 27 E4 tests green across the 5 spec files; no regressions in E1/E2/E3 tests.
**Why human:** Playwright requires a live Postgres container, running Go API server, and Vite dev server. The `.last-run.json` on disk records `status: "passed"` with `failedTests: []` from the executor's run on 2026-06-04. The test files, FE wiring fixes, and harness extensions are all verified present and substantive on disk — but only a live re-run can confirm the full-stack integration is still green after any environment changes.

---

## Commit Trail

All 10 task commits claimed in the SUMMARYs are confirmed present in git history:

| Commit | Plan | Content |
|--------|------|---------|
| `b455d7c` | 06-01 | E4 migrations — shift_masters, schedule_entries, approved_leave_days |
| `f22b9da` | 06-01 | E4 scheduling sqlc query package + regen |
| `2ac1463` | 06-01 | E4 domain types (scheduling.go) |
| `d72b087` | 06-02 | scheduling repo + conflict engine + services |
| `fe701ab` | 06-02 | scheduling handlers + DTOs |
| `e52a7ca` | 06-02 | mount E4 routes + wire main.go + extend seed |
| `0b73100` | 06-03 | E4 test harness + shift-master contract tests |
| `333c63a` | 06-03 | E4 schedule conflict + bulk-apply + scope contract tests |
| `66b11f1` | 06-04 | wire E4 screens off MSW + shift-master/schedule-grid E2E |
| `0db2ea4` | 06-04 | E4 conflict-negative + bulk-apply + leader-scope E2E |

---

## Summary

Phase 6 goal is achieved. The full scheduling slice — migrations, sqlc queries, domain types, repositories, a shared 6-check conflict engine, shift-master and schedule services, 11 HTTP endpoints, contract tests locking response shapes to the openapi, a real `approved_leave_days` seed powering honest SHIFT_OVER_LEAVE, FE wiring fixes, 27 E2E tests, and reset-db isolation — is present, substantive, and wired end-to-end. `go build ./... && go vet ./...` are clean; all 21 contract tests pass. The executor's E2E run documented 145 passed / 0 failed; `.last-run.json` on disk confirms `status: passed`. One human-verification item remains: an independent live Playwright re-run to confirm full-stack integration is still green.

---

_Verified: 2026-06-05_
_Verifier: Claude (gsd-verifier)_
