---
phase: 09-e7-overtime
plan: 03
subsystem: backend
tags: [go, contract-tests, chi, httptest, overtime, holidays, drift-gate, rbac, bulk]

requires:
  - phase: 09-e7-overtime
    plan: 02
    provides: "OvertimeService/HolidayService + handler + DTOs + routes (the system under test) — service entry points, error codes, {data}/PageResponse/BulkResult response shapes"
  - phase: 09-e7-overtime
    plan: 01
    provides: "domain/overtime enums + service ports the in-memory fakes implement"
  - phase: 08-e6-leave
    provides: "the Phase-8 leave contract-test harness to copy EXACTLY (fakeTx + fakeTxRunner + in-memory fakes + newHarness + mutable-principal middleware + stubIdempotency + decodeBody snapshot)"
provides:
  - "E7 contract tests = the drift gate replacing server codegen: 35 table-driven tests over the REAL OvertimeService + HolidayService + handler through a chi httptest harness"
  - "overtime_testkit_test.go: fakeTx + fakeTxRunner + fakeOvertimeRepo (dual-port OvertimeRepository+RuleRepository) + fakeHolidayRepo (configurable in-use) + fakeScheduleRepo (SchedulePort) + newHarness(role,company,employee) on chi + stubIdempotency + decodeBody snapshot"
  - "every E7 wire error code asserted: CONFLICT (wrong/terminal state +fields.status), OUT_OF_SCOPE 403, SELF_APPROVAL_FORBIDDEN 403, OVERRIDE_REASON_REQUIRED 422, OT_BELOW_MIN 422 +fields, HOLIDAY_DATE_CLASH 409, HOLIDAY_IN_USE 409; bulk {succeeded,failed} 200/422; cursor envelopes; day_type/tier in responses"
affects: [09-04-e2e]

tech-stack:
  added: []
  patterns:
    - "fakeOvertimeRepo is dual-port (svc.OvertimeRepository + svc.RuleRepository) — mirrors the real OvertimeRepo's dual-port shape so FindOvertimeRule drives OT_BELOW_MIN + the reference multiplier through one fake"
    - "exported-seam contract test: OT_BELOW_MIN + ClassifyDayType are driven through the REAL service methods (h.otSvc) because the openapi returns them on the create/auto-detect path that is OUT of the web HTTP surface — honest, not weakened"
    - "fakeScheduleRepo satisfies svc.SchedulePort returning schedulingsvc.LiveEntry verbatim (cross-epic read port typed on the provider service's exported type — zero glue)"

key-files:
  created:
    - backend/internal/handler/overtime/overtime_testkit_test.go
    - backend/internal/handler/overtime/overtime_handler_test.go
    - backend/internal/handler/overtime/holiday_handler_test.go
  modified: []

key-decisions:
  - "[09-03]: OT_BELOW_MIN + ClassifyDayType asserted through the REAL service methods (h.otSvc.EnforceMinMinutes / ClassifyDayType) not the HTTP router — the openapi returns OT_BELOW_MIN on the OT create/auto-detect path which is mobile/system (OUT of web scope); the contract test pins the apperr wire shape (422 + fields.counted_minutes + fields.min_minutes) at the seam"
  - "[09-03]: harness exposes the real OvertimeService as h.otSvc for the two exported seams; all other assertions go through the chi handler over the fakes (drift gate)"
  - "[09-03]: empLeader fixed to SWP-EMP-1108 (the 09-02 seed's Rudi/PL-5001 own-OT target) so SELF_APPROVAL_FORBIDDEN is asserted on the leader's own employee_id; bulk-approve runs as leader (L1 dispatch) so self/out-of-scope/terminal all land in failed[] in one call"
  - "[09-03]: fakeOvertimeRepo is dual-port (OvertimeRepository + RuleRepository) mirroring the real OvertimeRepo; rules keyed by service_line id with \"\" = global default (line-scoped wins, OR-2)"

patterns-established:
  - "Pattern: exported-seam contract test — when an error code's only production trigger is on an out-of-web-scope path (create/auto-detect), assert it through the REAL exported service seam at the apperr level rather than weakening an HTTP assertion"

requirements-completed: [OVT-01, OVT-02]

duration: 6min
completed: 2026-06-05
---

# Phase 9 Plan 03: E7 Overtime Contract Tests Summary

**The E7 drift gate replacing server codegen: 35 table-driven Go contract tests over the REAL OvertimeService + HolidayService + handler through a chi httptest harness (fakeTx + in-memory dual-port overtime/rule + holiday + schedule fakes + mutable-principal middleware + stubIdempotency), asserting every state transition + wrong/terminal-state 409, OUT_OF_SCOPE / SELF_APPROVAL_FORBIDDEN 403, OVERRIDE_REASON_REQUIRED + OT_BELOW_MIN 422 with field errors, HOLIDAY_DATE_CLASH / HOLIDAY_IN_USE 409, bulk {succeeded,failed} partial-success 200/422, and the cursor + {data} + calculation/tier_breakdown response shapes — mirroring the Phase-8 leave harness EXACTLY.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-06-05T04:13:54Z
- **Completed:** 2026-06-05T04:20:16Z
- **Tasks:** 2
- **Files modified:** 3 created

## Accomplishments

- **Testkit (overtime_testkit_test.go):** `fakeTx` (Exec no-op so audit-in-tx + InsertOvertimeApproval don't panic) + `fakeTxRunner`; `fakeOvertimeRepo` (dual-port `svc.OvertimeRepository` + `svc.RuleRepository` over shared `records`/`approvals`/`rules` maps — `UpdateOvertimeStatus` mutates so the `*ForUpdate` re-read observes the new state; `FindOvertimeRule` line-scoped-wins-over-global); `fakeHolidayRepo` (configurable `inUse` per id driving the HOLIDAY_IN_USE guard + `in_use_by_overtime` flag; clash via `GetHolidayByDateCategory`); `fakeScheduleRepo` (`svc.SchedulePort` returning `schedulingsvc.LiveEntry`). `newHarness(role,company,employee)` builds the REAL services + handler on a chi router with the same two RequireRole groups + idempotency positions as server.go + a mutable-principal closure middleware; exposes `h.otSvc` for the two exported seams. `decodeBody` snapshots `rr.Body.Bytes()` (re-decode for errCode + errFields, decision [08-03]).
- **State machine + scope (overtime_handler_test.go):** confirm→L1→final chain (each transition asserts the exact status change + the level-1/level-2 approval row), wrong-state 409s on confirm/approve-l1/approve-final/withdraw (each carries `fields.status`), OUT_OF_SCOPE 403 (leader cross-company), SELF_APPROVAL_FORBIDDEN 403 (leader on own OT), OVERRIDE_REASON_REQUIRED 422 + override-bypasses-L1 happy path, reject happy + short-reason 400, withdraw 204/409. GET detail asserts the `{data}` envelope (attendance_id present-and-JSON-null, calculation block with tier_breakdown + supersedes-null + the WORKDAY reference multiplier 1.5, approvals[] timeline); GET list asserts the `{data,next_cursor,has_more}` cursor envelope + leader-scope filtering (cross-company rows absent) + approvals omitted.
- **OT_BELOW_MIN + day_type (overtime_handler_test.go):** `EnforceMinMinutes` driven through the REAL service → 422 `OT_BELOW_MIN` + `fields.counted_minutes`/`fields.min_minutes`; at-or-above-min passes. `ClassifyDayType` HOLIDAY-overrides-WORKDAY precedence + RESTDAY fallback.
- **Bulk partial-success (overtime_handler_test.go):** bulk-approve as a leader (L1 dispatch) with [in-scope PENDING_L1, leader's OWN, CMP-0022, terminal APPROVED] → `succeeded`=[in-scope]; `failed[]` carries self (SELF_APPROVAL_FORBIDDEN) + cross-company (OUT_OF_SCOPE) + terminal (CONFLICT), each with its `error.code`; all-failed → 422. bulk-reject mirror (mix + all-failed 422).
- **Holiday CRUD (holiday_handler_test.go):** list cursor envelope + `in_use_by_overtime` computed per row; create 201 + Location (in_use false) + HOLIDAY_DATE_CLASH 409; update 200; delete 204 (free) + HOLIDAY_IN_USE 409 (configured count > 0, row not deleted).

## Task Commits

1. **Task 1: Testkit (fakes + harness) + transition/scope/409 + GET-shape tests** - `e4551d7` (test)
2. **Task 2: OT_BELOW_MIN + holiday clash/in-use + bulk partial-success tests** - `d0275f5` (test)

## Files Created/Modified

- `backend/internal/handler/overtime/overtime_testkit_test.go` — fakeTx/fakeTxRunner + dual-port fakeOvertimeRepo + fakeHolidayRepo + fakeScheduleRepo + newHarness (chi + mutable principal + stubIdempotency) + decodeBody + seed helpers (seedOvertime/seedRule/seedHoliday).
- `backend/internal/handler/overtime/overtime_handler_test.go` — 25 tests: list/get shapes + cursor + leader scope, confirm/L1/final/reject/withdraw transitions + 409/403/422, OT_BELOW_MIN + ClassifyDayType seams, bulk approve/reject partial-success.
- `backend/internal/handler/overtime/holiday_handler_test.go` — 7 tests: holiday list cursor + in_use flag, create 201/clash 409, update 200, delete 204/in-use 409.

## Decisions Made

- **OT_BELOW_MIN + ClassifyDayType via the REAL exported seam, not the router:** the openapi returns `OT_BELOW_MIN` on the OT create/auto-detect path (mobile/system, OUT of the web HTTP surface), so the contract test drives `h.otSvc.EnforceMinMinutes` / `h.otSvc.ClassifyDayType` directly and asserts the apperr wire shape (422, code, `fields.counted_minutes`/`fields.min_minutes`). Honest — not a weakened HTTP assertion. The harness exposes the real `OvertimeService` as `h.otSvc` for exactly these two seams; everything else goes through the chi handler.
- **`empLeader` = `SWP-EMP-1108`** (the 09-02 seed's Rudi / PL-5001 own-OT target) so SELF_APPROVAL_FORBIDDEN asserts against the leader's own `employee_id`. Bulk-approve runs as the leader (L1 dispatch) so self / out-of-scope / terminal all land in `failed[]` in a single call (the openapi BulkResult example shape).
- **`fakeOvertimeRepo` is dual-port** (`OvertimeRepository` + `RuleRepository`) mirroring the real `OvertimeRepo`; `rules` keyed by service-line id with `""` = the global default (line-scoped wins, OR-2) so `FindOvertimeRule` drives both OT_BELOW_MIN and the reference multiplier through one fake.

## Deviations from Plan

None — plan executed as written. The plan's Task-1 action lists "and a `stubIdempotency` at the server.go router position": it is present and wired on every action route exactly as server.go does, though no test currently exercises a replay (the E7 idempotency replay is exercised end-to-end by 09-04 against the real Postgres store, mirroring the Phase-7 documented seam; the stub is in place so the router shape matches).

## Issues Encountered

None. `go build ./...`, `go vet ./internal/handler/overtime/...`, `gofmt -l` clean; `go test ./internal/handler/overtime/...` = 35 PASS; `go test ./... -count=1` exits 0 (11 packages, no regression in earlier epics).

## User Setup Required

None.

---

## Reference for 09-04 (handoff)

- The contract tests pin: `{data}` envelope on GET detail, `PageResponse {data,next_cursor,has_more}` on list, `BulkResult {succeeded,failed[]}` on bulk, `calculation.tier_breakdown[].supersedes` null + `tier_indicator`, `attendance_id` JSON null on a REQUESTED row, `in_use_by_overtime` on holidays. 09-04 E2E drives the SAME shapes through the real FE + Go + ephemeral Postgres against the seeded `SWP-OT-3000x` / `SWP-HOL-900x` fixtures (09-02 handoff).
- `empLeader` in the tests = `SWP-EMP-1108` (Rudi/PL-5001) — the seed's SELF_APPROVAL_FORBIDDEN + OUT_OF_SCOPE targets (SWP-OT-30004 own, SWP-OT-30005 @ CMP-0022) match the seeded scenario rows.
- OT_BELOW_MIN's only production trigger is the create/auto-detect path (out of web scope) — 09-04 surfaces it via the seeded `SWP-OT-30006` (counted 0 / skipped_too_short) row, not a web create.

## Self-Check: PASSED

All 3 created files present; both task commits (`e4551d7`, `d0275f5`) exist in git. `go test ./... -count=1` exits 0; the overtime package = 35 tests green; no regressions.

---
*Phase: 09-e7-overtime*
*Completed: 2026-06-05*
