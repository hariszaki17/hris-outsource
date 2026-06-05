---
phase: 09-e7-overtime
plan: 02
subsystem: backend
tags: [go, chi, sqlc, overtime, holidays, state-machine, rbac, bulk, seed]

requires:
  - phase: 09-e7-overtime
    plan: 01
    provides: "overtime/overtime_approvals/holidays migrations + sqlc query set + domain/overtime types (the data layer this slice builds the service/handler on)"
  - phase: 08-e6-leave
    provides: "two-level approval state machine + GuardCompany scope + audit-in-tx + bulk partial-success shape (the closest mirror)"
  - phase: 07-e5-attendance
    provides: "bulk partial-success {succeeded,failed} loop-single + idempotency-at-router pattern"
  - phase: 03-e2-org-master-data
    provides: "overtime_rules + db/queries/org/overtime_rules.sql (reused for OT_BELOW_MIN + reference multiplier, NOT reimplemented)"
provides:
  - "OvertimeService: two-level state machine (confirm/approve-l1/approve-final/reject/withdraw) with *ForUpdate guards + 409s, GuardCompany OUT_OF_SCOPE + SELF_APPROVAL_FORBIDDEN, bulk approve/reject partial-success, OT_BELOW_MIN 422, ClassifyDayType (schedule+holiday, HOLIDAY>RESTDAY>WORKDAY), calculation block (reference multiplier stored not applied INV-2)"
  - "HolidayService CRUD: HOLIDAY_DATE_CLASH (pre-check + 23505 backstop) + HOLIDAY_IN_USE (CountOvertimeUsingHoliday) + in_use_by_overtime flag"
  - "9 overtime + 4 holiday chi handlers; DTOs match openapi byte-for-shape ({data} envelope on GET, PageResponse on list, tier_indicator/calculation.tier_breakdown, in_use_by_overtime)"
  - "E7 routes mounted under RequireRole; main.go wiring; seed fixtures for every E2E scenario"
affects: [09-03-contract-tests, 09-04-e2e]

tech-stack:
  added: []
  patterns:
    - "OvertimeRepo implements BOTH OvertimeRepository + RuleRepository (FindOvertimeRule scans active overtime_rules, line-scoped wins over global default per OR-2)"
    - "SchedulePort = the EXISTING scheduling repo's FindLiveEntryForAgentDate (reused via schedulingsvc.LiveEntry, no import cycle, no new schedule query)"
    - "calculation block computed at read time (single-tier, supersedes null); reference multiplier resolved from the rule's per-tier rate — exposed but NEVER applied to money (INV-2)"
    - "SELF_APPROVAL_FORBIDDEN via apperr.Error{HTTPStatus:403} struct literal (bypasses statusForCode default 422, like leave guardSelf / GEOFENCE_RADIUS_INVALID)"

key-files:
  created:
    - backend/internal/repository/overtime/overtime_repo.go
    - backend/internal/repository/overtime/holiday_repo.go
    - backend/internal/repository/overtime/mapping.go
    - backend/internal/service/overtime/ports.go
    - backend/internal/service/overtime/helpers.go
    - backend/internal/service/overtime/overtime_service.go
    - backend/internal/service/overtime/holiday_service.go
    - backend/internal/handler/overtime/handler.go
    - backend/internal/handler/overtime/overtime_handler.go
    - backend/internal/handler/overtime/holiday_handler.go
    - backend/internal/handler/overtime/dto.go
  modified:
    - backend/internal/server/server.go
    - backend/cmd/api/main.go
    - backend/cmd/seed/seed.go

key-decisions:
  - "[09-02]: OvertimeRepo satisfies both svc.OvertimeRepository + svc.RuleRepository; FindOvertimeRule reuses ListOvertimeRules (no new query) — line-scoped active rule wins, else the NULL-line global default, else any active rule (keeps OT_BELOW_MIN enforceable)"
  - "[09-02]: SchedulePort returns schedulingsvc.LiveEntry (the existing scheduling service type) so the existing scheduleRepo satisfies it verbatim — WORKDAY when a non-day-off live entry exists, else RESTDAY; HOLIDAY from GetHolidayForDate; TierPrecedence resolves"
  - "[09-02]: :confirm honors openapi x-rbac (agent/self) via guardConfirmActor — an agent must be the OT's own agent (else 403); HR/super/leader pass for the web-triggered confirm seam (CONTEXT: web confirm is staff-triggered on seeded candidates)"
  - "[09-02]: ApproveFinal isOverride allows PENDING_L1 bypass (OA-8) + requires note (else 422 OVERRIDE_REASON_REQUIRED); decision recorded as OVERRIDE_APPROVED at level 2"
  - "[09-02]: bulk approve dispatches by actor role — HR→ApproveFinal, leader→ApproveL1 — each id in its OWN tx (Phase-7 atomicity); SELF/OUT_OF_SCOPE/409 land in failed[]; handler 200 (>=1 ok) / 422 (all failed)"
  - "[09-02]: calculation tier_breakdown is single-tier (supersedes null) — the resolved day_type IS the effective tier (precedence applied at record/seed time); multiplier is the rule's per-tier rate as REFERENCE only (INV-2)"
  - "[09-02]: HolidayService.CreateWithID exposes an explicit-id seam (seed/E2E); HOLIDAY_DATE_CLASH = GetHolidayByDateCategory pre-check + 23505 backstop; HOLIDAY_IN_USE = CountOvertimeUsingHoliday>0 before SoftDeleteHoliday; in_use_by_overtime computed per row on list/get"
  - "[09-02]: seed attendance_id left NULL for the AUTO_DETECTED confirm target (no seeded SWP-ATT row to satisfy the FK; web confirm flow does not need the linked attendance)"

patterns-established:
  - "Pattern: a repo implementing two service ports (data + rule read-through) to keep the E2 rules reuse inside one constructor without a new query file"
  - "Pattern: cross-epic read port typed on the PROVIDER service's exported type (schedulingsvc.LiveEntry) so the existing repo satisfies it with zero glue"

requirements-completed: [OVT-01, OVT-02]

duration: 11min
completed: 2026-06-05
---

# Phase 9 Plan 02: E7 Overtime Services + Handlers Summary

**The E7 two-level OT approval state machine (confirm→L1→final, reject, withdraw) with *ForUpdate guards + 409s, GuardCompany OUT_OF_SCOPE + SELF_APPROVAL_FORBIDDEN, bulk approve/reject partial-success, OT_BELOW_MIN 422 from the reused E2 overtime_rules, day_type classification (schedule+holiday, HOLIDAY>RESTDAY>WORKDAY), holiday CRUD (HOLIDAY_DATE_CLASH/HOLIDAY_IN_USE), audit-in-tx + notify stub, routed + wired + seeded — matching the E7 openapi byte-for-shape, mirroring Phase-8 leave + Phase-7 attendance.**

## Performance

- **Duration:** 11 min
- **Started:** 2026-06-05T03:58:15Z
- **Completed:** 2026-06-05T04:09:00Z
- **Tasks:** 3
- **Files modified:** 11 created + 3 modified (+ regenerated sqlc)

## Accomplishments

- **Repository + ports:** `OvertimeRepo` (over the 09-01 sqlc overtime/approvals queries) + `HolidayRepo` (holidays queries), with `mapping.go` converting `pgtype.Numeric→*float64` (reference_multiplier, INV-2), `pgtype.Date↔time.Time`, `text[]→[]string`, `pgx.ErrNoRows→domain.ErrNotFound`. `OvertimeRepo.FindOvertimeRule` reuses the E2 `overtime_rules` (line-scoped wins over global default) — NO rule CRUD added.
- **OvertimeService state machine:** `Confirm` (PENDING_AGENT_CONFIRM→PENDING_L1, agent-self guard), `ApproveL1` (→PENDING_HR, GuardCompany + SELF_APPROVAL_FORBIDDEN), `ApproveFinal` (→APPROVED, isOverride PENDING_L1 bypass + OVERRIDE_REASON_REQUIRED 422), `Reject` (→REJECTED, reason min 5), `Withdraw` (→WITHDRAWN, 204) — each `InTx` with `GetOvertimeForUpdate` lock → `stateConflict` (409) → `UpdateOvertimeStatus` → `InsertOvertimeApproval` + `audit.Record` + notify stub.
- **Bulk:** `BulkApprove` (HR→final / leader→L1) / `BulkReject`, each id in its own tx, `{succeeded, failed[]}` via `apperr.As`; handler 200 (≥1 ok) / 422 (all failed).
- **Business rules:** `EnforceMinMinutes` → `OT_BELOW_MIN` 422 with `{counted_minutes, min_minutes}` field errors (exported seam for 09-03); `ClassifyDayType` (GetHolidayForDate→HOLIDAY else live-schedule→WORKDAY else RESTDAY, TierPrecedence). `Calculation` block computed at read time — multiplier is the rule's per-tier rate as REFERENCE only (INV-2).
- **HolidayService CRUD:** clash guard (pre-check + 23505 backstop), in-use guard (CountOvertimeUsingHoliday), `in_use_by_overtime` per-row flag.
- **Handlers + DTOs:** 9 overtime + 4 holiday chi handlers; `Overtime`/`Holiday`/`BulkResult` DTOs match openapi (list = PageResponse top-level, GET = `{data}` envelope, nullable fields pointers-without-omitempty → JSON null).
- **Routes + wiring + seed:** OVERTIME slice in server.go (two RequireRole groups, idempotency-wrapped actions); main.go constructs the slice reusing `scheduleRepo`; `seedHolidays` + `seedOvertime` plant a target for every E2E scenario.

## Task Commits

1. **Task 1: Repository + ports + two-level state machine service** - `a81d9af` (feat)
2. **Task 2: Holiday service + overtime/holiday handlers + DTOs** - `bcae138` (feat)
3. **Task 3: Routes (server.go) + wiring (main.go) + seed** - `5bf2906` (feat)

## Files Created/Modified

- `backend/internal/repository/overtime/overtime_repo.go` — OvertimeRepository + RuleRepository over sqlc; FindOvertimeRule reuses E2 rules.
- `backend/internal/repository/overtime/holiday_repo.go` — HolidayRepository (list/get/by-date-category/for-date/insert/update/soft-delete/count-in-use).
- `backend/internal/repository/overtime/mapping.go` — row→domain mappers + Numeric/Date/text[] conversions.
- `backend/internal/service/overtime/ports.go` — Overtime/Holiday/Rule/Schedule/Tx ports + filters + write params.
- `backend/internal/service/overtime/helpers.go` — clampLimit, principal extraction, cursors (OT created_at DESC / holiday date ASC), stateConflict.
- `backend/internal/service/overtime/overtime_service.go` — the state machine + scope/self guards + bulk + OT_BELOW_MIN + ClassifyDayType + calculation.
- `backend/internal/service/overtime/holiday_service.go` — holiday CRUD + clash/in-use guards.
- `backend/internal/handler/overtime/{handler,overtime_handler,holiday_handler,dto}.go` — chi handlers + DTOs.
- `backend/internal/server/server.go` — OVERTIME route slice + Deps.Overtime.
- `backend/cmd/api/main.go` — overtime slice construction + Deps assignment.
- `backend/cmd/seed/seed.go` — seedHolidays + seedOvertime + approval trails.

## Decisions Made

- **OvertimeRepo is dual-port** (OvertimeRepository + RuleRepository): `FindOvertimeRule` scans active `overtime_rules` (line-scoped first, then the NULL-line global default, then any active rule) — reuses the E2 query, no rule CRUD added, OT_BELOW_MIN always enforceable.
- **SchedulePort typed on `schedulingsvc.LiveEntry`** so the existing `scheduleRepo` satisfies it verbatim — no new schedule query, no import cycle. WORKDAY when a non-day-off live entry exists, else RESTDAY; HOLIDAY from the calendar; `TierPrecedence` (HOLIDAY>RESTDAY>WORKDAY) resolves.
- **`:confirm` agent-self seam:** `guardConfirmActor` enforces the openapi x-rbac (an agent must be the OT's own agent → 403); HR/super/leader pass for the web-triggered confirm flow on seeded candidates (CONTEXT).
- **isOverride final** allows the PENDING_L1 bypass (OA-8) + requires a note (else 422 `OVERRIDE_REASON_REQUIRED`); recorded as `OVERRIDE_APPROVED` at level 2.
- **calculation tier_breakdown is single-tier** (supersedes null) — the resolved `day_type` is the effective tier; the multiplier is the rule's per-tier rate as **reference only** (INV-2, no monetary figure anywhere).
- **HolidayService.CreateWithID** exposes an explicit-id seam (seed/E2E); clash = pre-check + 23505 backstop; in-use = count before delete; `in_use_by_overtime` computed per row.
- **Seed `attendance_id` left NULL** for the AUTO_DETECTED confirm target (no SWP-ATT seed row to satisfy the FK; the web confirm flow does not need it).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] main.go verify-grep pattern mismatch (no code change needed)**
- **Found during:** Task 3 verification.
- **Issue:** The plan's Task-3 automated verify greps `cmd/api/main.go` for the literal `d.Overtime`, but `d.` is the receiver name inside `server.go` (routes), not `main.go` (which uses the struct-literal field `Overtime: overtimeHandler`). The grep would always miss in main.go.
- **Fix:** Confirmed the wiring is correct via the real assignment (`Overtime: overtimeHandler` + `overtimehttp.NewHandler(overtimeSvc, holidaySvc)`). No code change — the grep expectation was off, the wiring satisfies the acceptance criterion ("main.go constructs and assigns Deps.Overtime").
- **Files modified:** none (verification-pattern note only).
- **Commit:** n/a.

Otherwise the plan executed as written.

## Issues Encountered

None. `make gen`, `go build ./...`, `go vet ./...`, and `gofmt -l` on the three E7 dirs are all clean; the full handler/service test suite passes (no regressions; 09-03 adds the overtime contract tests).

## User Setup Required

None — no external service configuration required.

---

## Reference for 09-03 / 09-04 (handoff)

### Service entry points (09-03 contract tests drive these via the real handler)
- `OvertimeService.{List,Get,Confirm,ApproveL1,ApproveFinal,Reject,Withdraw,BulkApprove,BulkReject}` — each returns `(dom.Overtime, Calculation, error)` (Withdraw returns `error`; bulk returns `BulkResult`).
- Exported seams for contract/create-path tests: `OvertimeService.ClassifyDayType(ctx, employeeID, workDate, serviceLineID) (tier, *holidayID)` and `OvertimeService.EnforceMinMinutes(ctx, countedMinutes, serviceLineID) error` (→ `OT_BELOW_MIN` 422).
- `HolidayService.{List,Get,Create,CreateWithID,Update,Delete}`.
- Cursor decoders: `svc.DecodeOvertimeCursor` / `svc.DecodeHolidayCursor`.

### Error codes on the wire (assert these)
- 409 `CONFLICT` (`fields.status`) — wrong/terminal-state transition (confirm/L1/final/reject/withdraw).
- 403 `OUT_OF_SCOPE` — leader acting cross-company (GuardCompany); 403 `SELF_APPROVAL_FORBIDDEN` — approver acting on own OT (struct-literal, bypasses statusForCode).
- 422 `OT_BELOW_MIN` (`fields.counted_minutes`, `fields.min_minutes`); 422 `OVERRIDE_REASON_REQUIRED` (final isOverride without note).
- 409 `HOLIDAY_DATE_CLASH` (create/update dup date+category); 409 `HOLIDAY_IN_USE` (delete referenced by APPROVED OT).
- Bulk: 200 when ≥1 succeeded (failed[] carries per-id code), 422 when all failed.

### Response shapes
- `GET /overtime` → `httpx.PageResponse` at top level (`{data, next_cursor, has_more}`); approvals omitted on list.
- `GET /overtime/{id}` → `{data: <Overtime>}` (FE unwraps `{data}`); approvals[] present; calculation recomputed.
- `Overtime`: `employee{id,name}`, `company{id,name}`, `tier_indicator` (=day_type), `calculation{worked_minutes,counted_minutes,min_minutes_threshold,skipped_too_short,tier_breakdown:[{tier,minutes,multiplier,overtime_rule_id,supersedes:null}]}`; nullable fields are JSON null.
- `Holiday`: `date` (←holiday_date), `in_use_by_overtime` (computed), `applicable_service_lines` ([] = global).

### Seed targets (09-04 E2E selectors / API drivers)
- `SWP-OT-30001` PENDING_AGENT_CONFIRM AUTO_DETECTED @ CMP-0021 (Dewi) — confirm target.
- `SWP-OT-30002` PENDING_L1 WORKDAY @ CMP-0021 — leader L1 (Rudi) target.
- `SWP-OT-30003` PENDING_HR @ CMP-0021 — HR final target.
- `SWP-OT-30004` PENDING_L1, Rudi's OWN (EMP-1108/PL-5001) — SELF_APPROVAL_FORBIDDEN.
- `SWP-OT-30005` PENDING_L1 @ CMP-0022 (Budi/PL-5002) — OUT_OF_SCOPE for Rudi.
- `SWP-OT-30006` PENDING_L1, counted 0 / skipped_too_short — OT_BELOW_MIN.
- `SWP-OT-30007` APPROVED, `SWP-OT-30008` REJECTED — terminal list-filter rows (+ approval trails).
- `SWP-OT-30009` HOLIDAY APPROVED referencing `SWP-HOL-9001` — HOLIDAY_IN_USE source; `SWP-OT-30010` RESTDAY PENDING_L1.
- Holidays: `SWP-HOL-9001` (in-use, delete blocked) + `SWP-HOL-9002` (free, deletable), both NATIONAL, in-range dates.
- TZ: all dates anchored on `mondayOfCurrentWeek` (Asia/Jakarta-safe, clearly in range).

## Self-Check: PASSED

All 11 created files + 3 modified present; the 3 task commits (`a81d9af`, `bcae138`, `5bf2906`) exist in git. `make gen`, `go build ./...`, `go vet ./...` exit 0; `gofmt -l` clean on the three E7 dirs; the existing handler/service test suite passes (no regressions).

---
*Phase: 09-e7-overtime*
*Completed: 2026-06-05*
