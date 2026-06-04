---
phase: 06-e4-schedule-shifts
plan: 03
subsystem: backend
tags: [go, contract-tests, drift-gate, scheduling, conflict-engine, bulk-apply, rbac-scope, e4, httptest]

# Dependency graph
requires:
  - phase: 06-e4-schedule-shifts
    plan: 02
    provides: scheduling Service+Handler (conflict engine, bulk-apply, shift-master CRUD, schedule CRUD), DTO shapes, repo ports
  - phase: 05-e3-placement
    provides: Phase-5 contract-test harness pattern (fakeTx/fakeTxRunner/decodeBody + in-memory fake repo + mutable-principal chi middleware)
provides:
  - scheduling_testkit_test.go — in-memory fakeShiftMasterRepo (nameIndex DUPLICATE_NAME sentinel) + fakeScheduleRepo (placements/approvedLeave/liveEntry maps backing every conflict branch) + newHarness/do over the REAL services+handler
  - shift_master_handler_test.go — list envelope, cross_midnight, DUPLICATE_NAME 409, BREAK_OUTSIDE_WINDOW 422, deactivate/reactivate + ALREADY_INACTIVE 409, leader-write 403
  - schedule_handler_test.go — all six conflict codes (status+code+details), happy create, list envelope, force_replace, :check no-write, bulk-apply 200/422/weekdays_mask, DELETE 204 + leader past-date 403
  - the drift gate: BE E4 response shapes locked to docs/api/E4-shift-scheduling/openapi.yaml
affects: [06-04 e2e (reuses the fake-repo seeding pattern + fixture ids)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "fakeScheduleRepo backs the engine ConflictRepo with three keyed maps (placements[emp], approvedLeave[emp|date], liveEntry[emp|date]) so each of the six ordered checks fires deterministically without touching Postgres"
    - "DUPLICATE_NAME is exercised honestly: fakeShiftMasterRepo.nameIndex returns a sentinel error whose Error() contains '23505' so the real service's isUniqueViolation path maps it (no special-casing in the fake)"
    - "CreateScheduleEntry auto-registers the new row as the live entry for its (agent,date) so a second same-cell write naturally triggers DOUBLE_SHIFT, and SoftDelete clears it — bulk-apply over a range behaves like the DB"
    - "Single newHarness(t, role, leaderCompanyID) builds the real ShiftMasterService+ScheduleService+Handler with a fixed clock; principal is a mutable struct field read by closure middleware (Phase-2/5 dynamic-principal pattern)"

key-files:
  created:
    - backend/internal/handler/scheduling/scheduling_testkit_test.go
    - backend/internal/handler/scheduling/shift_master_handler_test.go
    - backend/internal/handler/scheduling/schedule_handler_test.go
  modified: []

key-decisions:
  - "OUTSIDE_PLACEMENT_PERIOD asserted on code+422 only (no placement_id/start/end details): the engine emits this code precisely when NO placement covers the date, so there is no placement to populate the detail with — matching the honest impl, not weakening the contract. The openapi ConflictDetails for that code are documented as optional ('only fields relevant to the code are populated') and the impl resolves placement-first."
  - "Conflict fixtures use the 06-02 seed ids verbatim (SWP-SHF-001/002, SWP-EMP-1108/2891/3001, SWP-CMP-0021/0022, SWP-LR-44210 ANNUAL, SWP-SCH-6001) so 06-04 E2E and these contract tests assert the same anchors"
  - "Fixed clock = 2026-06-04 12:00 WIB (same instant as the placement tests); leader-past-date DELETE uses 2026-05-01 (before clock) to trip C-5"

# Metrics
duration: ~4min
completed: 2026-06-04
---

# Phase 6 Plan 03: E4 Contract Tests (Drift Gate) Summary

**Three test files in `internal/handler/scheduling/` lock the E4 BE response shapes to the openapi: every one of the six conflict codes asserted with its exact HTTP status + `error.code` + `error.details`, the bulk-apply partial-success envelope (200 mixed / 422 all-failed / weekdays_mask filtering), the leader-scope 403, the `:check` side-effect-free dry-run, force_replace (201 MODIFIED + replaced_entry_id), shift-master CRUD (DUPLICATE_NAME / BREAK_OUTSIDE_WINDOW / ALREADY_INACTIVE), and both list envelopes ({data,next_cursor,has_more} and {data,warnings}). `go test ./... -count=1` is green with zero regressions; `go vet ./...` and `gofmt -l` clean.**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-06-04T16:23:03Z
- **Completed:** 2026-06-04T16:27:21Z
- **Tasks:** 2
- **Files:** 3 created

## Task Commits

1. **Task 1: test harness (fakes) + shift-master contract tests** — `0b73100` (test)
2. **Task 2: schedule conflict + bulk-apply + scope contract tests** — `333c63a` (test)

**Plan metadata:** _(final docs commit — this SUMMARY + STATE + ROADMAP + REQUIREMENTS)_

## Codes & envelopes covered

| Code | HTTP | Asserted details | Test |
|------|------|------------------|------|
| `OUT_OF_SCOPE` | 403 | (code only) | `TestScope_OutOfScope` |
| `OUTSIDE_PLACEMENT_PERIOD` | 422 | (code only — see decision) | `TestConflict_OutsidePlacementPeriod` |
| `SHIFT_DEACTIVATED` | 422 | (code only) | `TestConflict_ShiftDeactivated` |
| `SHIFT_NOT_FOR_SERVICE_LINE` | 422 | `placement_service_line_id` + `shift_service_line_id` | `TestConflict_ShiftNotForServiceLine` |
| `SHIFT_OVER_LEAVE` | 409 | `leave_request_id` + `leave_type` | `TestConflict_ShiftOverLeave` |
| `DOUBLE_SHIFT` | 409 | `existing_entry_id` + `existing_shift_name` | `TestConflict_DoubleShift` |
| `DUPLICATE_NAME` | 409 | `fields.name` | `TestCreateShiftMaster_DuplicateName` |
| `BREAK_OUTSIDE_WINDOW` | 422 | `fields.break_start` | `TestCreateShiftMaster_BreakOutsideWindow` |
| `ALREADY_INACTIVE` | 409 | (code only) | `TestDeactivateReactivate` |

**Envelopes asserted:** shift-master list `{data, next_cursor, has_more}` (each item has `status`∈{ACTIVE,INACTIVE}, `break_minutes`, `in_use_count`, `cross_midnight`); schedule grid `{data, warnings}` (warnings present even when empty); create 201 ScheduleEntry + `warnings:[]`; `BulkApplyResult` `{succeeded:[{id,employee_id,date,status}], failed:[{employee_id,date,error:{code,message,details}}], warnings}` — the `failed[].error.code` nesting the FE consumes.

**Bulk-apply proven:** partial success → 200 with non-empty `succeeded` AND `failed` (one date hits SHIFT_OVER_LEAVE via the approvedLeave map); all-failed (range entirely outside placement window) → 422 same body shape; `weekdays_mask [1..5]` over a Mon–Sun range → exactly 5 attempted cells (succeeded+failed).

**`:check` no-side-effect:** asserts `failed[0].error.code` is set AND `len(repo.entries)` is unchanged before/after (no persistence, no tx, no audit).

## The fake-repo seeding pattern (06-04 can reference)

`scheduling_testkit_test.go` exposes a `harness` with three seed helpers backing the engine's `ConflictRepo`:

- `seedPlacement(emp, plID, companyID, svcLineID, start, end)` → `placements[emp]`; `FindActivePlacementForAgentDate` honours the window (returns `ErrNotFound` when the date is outside → OUTSIDE_PLACEMENT_PERIOD).
- `seedMaster(id, name, start, end, svcLine, active)` → shared catalog; `cross_midnight` derived `end<=start`; `active=false` → SHIFT_DEACTIVATED; tagged `svcLine != placement` → SHIFT_NOT_FOR_SERVICE_LINE.
- `seedApprovedLeave(emp, date, lrID, type)` → `approvedLeave[emp|date]` → SHIFT_OVER_LEAVE (the honest read path, mirroring the seeded `approved_leave_days` row).
- `seedLiveEntry(id, emp, companyID, date, shiftName)` → `liveEntry[emp|date]` + `entries[id]` → DOUBLE_SHIFT (and a deletable row for the DELETE tests).

`CreateScheduleEntry` registers the new row in `liveEntry`, and `SoftDeleteScheduleEntry` clears it, so a bulk range naturally produces DOUBLE_SHIFT on a re-applied cell exactly like the partial-unique index does. 06-04 E2E uses the same fixture ids (SWP-EMP-1108 / 2891 / 3001, SWP-CMP-0021/0022, SWP-LR-44210, SWP-SHF-001/002) so the contract tests and the live E2E assert identical anchors.

## Deviations from Plan

**1. [Contract-honesty] OUTSIDE_PLACEMENT_PERIOD asserted on code+status only (no placement details).**
- **Found during:** Task 2 (writing `TestConflict_OutsidePlacementPeriod`).
- **Issue:** The plan's task text says "details has placement_id/placement_start/placement_end (when a placement exists)". But the 06-02 engine emits `OUTSIDE_PLACEMENT_PERIOD` exactly when **no** placement covers the date (`FindActivePlacementForAgentDate` returns `ErrNotFound`) — so there is no placement object to populate the detail. The openapi example shows those detail fields, but `ConflictDetails` is documented as "only the fields relevant to the specific conflict code are populated" and the schema marks none as required.
- **Resolution:** Asserted `422` + `error.code == OUTSIDE_PLACEMENT_PERIOD` (the plan's own parenthetical "when a placement exists" acknowledges the conditional). Did NOT fabricate a placement just to force a detail the honest code path cannot emit, and did NOT weaken any other assertion. The other detail-bearing codes (SHIFT_NOT_FOR_SERVICE_LINE, SHIFT_OVER_LEAVE, DOUBLE_SHIFT) ARE asserted on their full details. No code change — the impl is correct against the contract.

**2. [gofmt] schedule_handler_test.go reformatted on first write (map literal key alignment).**
- Ran `gofmt -w`; re-verified `gofmt -l` clean. No behaviour change.

No other deviations — no openapi-vs-impl mismatch found; the 06-02 handlers already match the contract byte-for-shape, which is exactly what the drift gate now proves.

## Issues Encountered

None blocking. `go test ./... -count=1` green across all eight handler/service packages (foundations, identity, org, people, placement, scheduling, service/identity) — no regressions in earlier slices. `go vet ./...` and `gofmt -l` clean.

## User Setup Required

None.

---

## Self-Check: PASSED

All 3 created files present on disk (scheduling_testkit_test.go, shift_master_handler_test.go, schedule_handler_test.go); both task commits (0b73100, 333c63a) present in git history. `go test ./... -count=1` exits 0 across all packages; `go vet ./...` + `gofmt -l` clean.

---
*Phase: 06-e4-schedule-shifts*
*Completed: 2026-06-04*
