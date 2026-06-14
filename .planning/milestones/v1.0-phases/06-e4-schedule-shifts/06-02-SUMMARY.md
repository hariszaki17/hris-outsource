---
phase: 06-e4-schedule-shifts
plan: 02
subsystem: backend
tags: [go, chi, scheduling, conflict-engine, bulk-apply, rbac-scope, audit, seed, e4]

# Dependency graph
requires:
  - phase: 06-e4-schedule-shifts
    plan: 01
    provides: shift_masters / schedule_entries / approved_leave_days tables + scheduling sqlc package + domain.ShiftMaster/ScheduleEntry
  - phase: 05-e3-placement
    provides: placement service structure (TxRunner/Clock/FOR-UPDATE/23505 backstop/audit-in-tx), placements rows (INV-2 anchor), GuardCompany scope pattern, server.go/main.go/seed.go slice conventions
provides:
  - scheduling repository (ShiftMasterRepo + ScheduleRepo over 06-01 sqlc, pgtype.Date boundary conversion)
  - shared ordered 6-check conflict engine (Evaluate) reused by create/update/check/bulk-apply
  - shift-master service (CRUD + deactivate/reactivate, cross_midnight derive, BREAK_OUTSIDE_WINDOW, DUPLICATE_NAME, ALREADY_INACTIVE/ACTIVE)
  - schedule service (Create/Update/Delete + per-cell-atomic BulkApply + side-effect-free Check)
  - scheduling chi handlers + DTOs (11 endpoints, byte-for-shape with openapi)
  - E4 routes mounted under RequireRole groups in server.go; scheduling wired in main.go
  - seed: SWP-SHF-001/002 + 2 in-week schedule entries + 1 approved_leave_days row (SHIFT_OVER_LEAVE fixture)
affects: [06-03 contract-tests, 06-04 e2e, 07-attendance, 08-e6-leave]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single pure conflict evaluator (Evaluate) shared by 4 endpoints — resolves placement once, scope-checks its company FIRST, then short-circuits on the first of six ordered checks; returns code+status+ConflictDetails"
    - "Per-cell tx atomicity for bulk-apply: each (employee×date) cell persisted in its OWN InTx via CreateEntry; a failing cell never rolls back a succeeded one (BulkApply loops, appends to succeeded/failed)"
    - "PATCH nullable-field decode via json.RawMessage probing (explicit null vs absent) for shift_master_id (convert-to-OFF) and break/service-line clears"
    - "Bulk handler status policy: 200 if len(succeeded)>=1 else 422 — same BulkApplyResult body either way"

key-files:
  created:
    - backend/internal/repository/scheduling/mapping.go
    - backend/internal/repository/scheduling/shift_master_repo.go
    - backend/internal/repository/scheduling/schedule_repo.go
    - backend/internal/service/scheduling/conflict_engine.go
    - backend/internal/service/scheduling/shift_master_service.go
    - backend/internal/service/scheduling/schedule_service.go
    - backend/internal/handler/scheduling/shift_master_dto.go
    - backend/internal/handler/scheduling/shift_master_handler.go
    - backend/internal/handler/scheduling/schedule_dto.go
    - backend/internal/handler/scheduling/schedule_handler.go
  modified:
    - backend/internal/server/server.go
    - backend/cmd/api/main.go
    - backend/cmd/seed/seed.go

key-decisions:
  - "Conflict order = scope → placement-period → deactivated → service-line → over-leave → double-shift; placement is resolved first (scope needs its company) and OUTSIDE_PLACEMENT_PERIOD is emitted when no active placement exists at all"
  - "Each conflict code carries its exact HTTP status as an explicit apperr.Error.HTTPStatus override (403/422/422/422/409/409) so statusForCode never re-maps it"
  - "PATCH /schedule re-runs the engine with ForceReplace=true (editing the agent's own existing cell must not self-trigger DOUBLE_SHIFT); status becomes MODIFIED"
  - "DELETE leader-past-date restriction (C-5): GuardCompany first, then a leader clearing work_date<today(Asia/Jakarta) gets 403 FORBIDDEN; HR/super may clear past dates"
  - "Over-leave delivered honestly: Evaluate calls the real approved_leave_days read (FindApprovedLeaveForAgentDate); seed plants SWP-LR-44210 so it is exercisable now (E6/Phase-8 wires the production leave source)"

# Metrics
duration: ~11min
completed: 2026-06-04
---

# Phase 6 Plan 02: E4 Schedule & Shifts Service + Handler Layer Summary

**The heart of the phase: a single ordered 6-check conflict engine shared by create / update / :check / :bulk-apply, plus shift-master CRUD, schedule CRUD, per-cell-atomic bulk-apply, leader scope via GuardCompany, audit-in-tx, a TODO(Phase-11) notify stub, the 11 FE-used E4 routes mounted under the right RBAC groups, main.go wiring, and a seed that makes SHIFT_OVER_LEAVE genuinely fire. `go build ./... && go vet ./...` clean, gofmt clean.**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-06-04T16:07:39Z
- **Completed:** 2026-06-04T16:18:30Z
- **Tasks:** 3
- **Files:** 10 created + 3 modified

## Task Commits

1. **Task 1: repository + conflict engine + services** — `d72b087` (feat)
2. **Task 2: handlers + DTOs (byte-for-shape)** — `fe701ab` (feat)
3. **Task 3: routes + main.go wiring + seed extension** — `e52a7ca` (feat)

**Plan metadata:** _(final docs commit — this SUMMARY + STATE + ROADMAP + REQUIREMENTS)_

## The conflict engine (the shared evaluator)

`Evaluate(ctx, repo, ConflictInput) (ConflictResult, error)` in `conflict_engine.go` resolves the active placement covering the date FIRST (it is the INV-2 anchor AND the scope source), then runs the six checks **in order**, short-circuiting on the first failure:

| # | Code | HTTP | Trigger | Details emitted |
|---|------|------|---------|-----------------|
| 0 | `OUTSIDE_PLACEMENT_PERIOD` | 422 | no ACTIVE/EXPIRING placement covers the date | `{}` (fields.date) |
| 1 | `OUT_OF_SCOPE` | 403 | `GuardCompany` fails on the placement's company (leader) | `{leader_company_id, agent_company_id}` |
| 2 | (n/a — period already handled at #0) | — | — | — |
| 3 | `SHIFT_DEACTIVATED` | 422 | picked master `is_active=false` | — |
| 4 | `SHIFT_NOT_FOR_SERVICE_LINE` | 422 | master.service_line set & != placement's | `{placement_service_line_id, shift_service_line_id}` |
| 5 | `SHIFT_OVER_LEAVE` | 409 | approved leave covers the date | `{leave_request_id, leave_type}` |
| 6 | `DOUBLE_SHIFT` | 409 | live entry exists & !force_replace | `{existing_entry_id, existing_shift_name}` |

The openapi prose lists OUT_OF_SCOPE as "rule 1" but OUTSIDE_PLACEMENT_PERIOD logically precedes it (you cannot scope-check a company you cannot resolve). The implementation resolves the placement, and **if found** scope-checks its company BEFORE any 422; **if no placement at all**, OUTSIDE_PLACEMENT_PERIOD wins. On a full pass the result snapshots `start_time/end_time/cross_midnight` from the master (day-off → nil/false) and carries `placement_id/company_id/service_line_id` for the write. `force_replace=true` + an existing entry sets `ExistingEntryID` (replace path) instead of blocking.

Each branch sets `Status` explicitly; `ConflictResult.AsError()` builds an `*apperr.Error{Code, Fields, Details, HTTPStatus}` so the byte-for-shape envelope flows straight through `httpx.WriteError`.

## Bulk-apply: 200/422 rule + per-cell tx atomicity

`BulkApply` expands `(employee × date in weekdays_mask)` then calls `CreateEntry` per cell — **each cell in its own `InTx`**. A failing cell appends to `failed[]` (`{employee_id, date, error:{code,message,details}}`) and the loop continues; a succeeding cell appends to `succeeded[]` (`{id, employee_id, date, status}`). One failing date NEVER rolls back the successes. The handler returns **200 when `len(succeeded) >= 1`, else 422**, with the same `BulkApplyResult` body in both cases. `:check` runs the identical expansion through `checkCells` (engine only, **no writes / no tx / no audit / no notify**) and always returns 200.

`weekdays_mask` uses ISO 1=Mon..7=Sun; Go's `time.Weekday` Sunday=0 is remapped to 7. Empty/omitted mask = every day in `[start_date, end_date]`.

## Exact seeded fixtures (for 06-03 / 06-04 to consume)

Dates are computed off **Monday of the current week** (`mondayOfCurrentWeek`, UTC date) so they land inside the visible grid week and inside each placement window, avoiding the Asia/Jakarta-vs-UTC midnight boundary (05-03 TZ note).

**Shift masters** (explicit ids honoured over the column DEFAULT):
- `SWP-SHF-001` "Pagi" 07:00–15:00, break 12:00–13:00, `service_line_id=NULL` (all lines), `cross_midnight=false`
- `SWP-SHF-002` "Malam" 23:00–07:00, no break, `service_line_id=SWP-SVC-003` (Parking), `cross_midnight=true`

**Schedule entries** (Pagi snapshot 07:00–15:00, status SCHEDULED):
- `SWP-SCH-6001` — Rudi `SWP-EMP-1108` / placement `SWP-PL-5001` @ CMP-0021 on **Monday+1 (Tuesday)**
- `SWP-SCH-6002` — Dewi `SWP-EMP-3001` / placement `SWP-PL-5004` @ CMP-0021 on **Monday+2 (Wednesday)**

**Approved-leave day** (SHIFT_OVER_LEAVE fixture):
- `approved_leave_days`: `employee_id=SWP-EMP-3001`, `leave_date=Monday+3 (Thursday)`, `leave_request_id=SWP-LR-44210`, `leave_type=ANNUAL`
- 06-04 should attempt to schedule **EMP-3001 on Monday+3** to assert `SHIFT_OVER_LEAVE` (that date is deliberately NOT taken by `SWP-SCH-6002`).

**Negative-test anchors already in place from 05-02:**
- `DOUBLE_SHIFT`: schedule EMP-1108 on Monday+1 again (SWP-SCH-6001 occupies it) — or any agent twice on one date.
- `OUTSIDE_PLACEMENT_PERIOD`: schedule any seeded agent on a date outside their placement window (e.g. 2030-01-01).
- `OUT_OF_SCOPE` (leader): Rudi (`shift_leader`, scoped to CMP-0021) attempting to schedule **Budi `SWP-EMP-2891`** whose placement `SWP-PL-5002` is at **CMP-0022** → 403.

## Over-leave honesty note

`approved_leave_days` is the **E4-owned** read source (06-01). The conflict engine reads it via the real `FindApprovedLeaveForAgentDate` query — not a faked path — and the seed plants one row, so `SHIFT_OVER_LEAVE` is genuinely exercisable now. E6 (Phase 8) later populates/supersedes this table from the production `leave_requests` source; the `SWP-LR-*` id carries no FK so the namespaces never collide.

## RBAC + routes

11 endpoints mounted in `server.go` after the PLACEMENT slice marker:
- **Reads + ALL schedule ops** under `RequireRole(super_admin, hr_admin, shift_leader)`: `GET /shift-masters`, `GET /shift-masters/{id}`, `GET /schedule`, `POST/PATCH/DELETE /schedule`, `POST /schedule:check`, `POST /schedule:bulk-apply`. Leader scope is enforced **in the service** (`GuardCompany` on the resolved placement company for writes; on `company_id` for the grid read) → 403 `OUT_OF_SCOPE`.
- **Shift-master writes** under `RequireRole(super_admin, hr_admin)`: `POST /shift-masters`, `PATCH /shift-masters/{id}`, `POST /shift-masters/{id}:deactivate`, `:reactivate`.
- `:check` is the ONLY write-path route with **no idempotency wrapper** (side-effect-free); every other write is wrapped with `d.Idempotency.Handler`.
- No out-of-scope endpoints (`/schedule/by-agent`, swap endpoints) — deferred per CONTEXT.

## Response shapes (byte-for-shape with openapi)

- `ShiftMaster`: `break_minutes` derived (break_end − break_start), `status` ACTIVE/INACTIVE from `is_active`, `in_use_count`, list envelope `{data, next_cursor, has_more}` (id-desc keyset cursor).
- `ScheduleEntry`: `work_date` as `YYYY-MM-DD`; create 201 carries `warnings:[]`; grid list returns `{data, warnings:[]}`.
- `BulkApplyResult`: `succeeded:[]{id,employee_id,date,status}`, `failed:[]{employee_id,date,error:{code,message,details}}`, `warnings:[]`. The failed item nests `error` (the contract location the FE reads as `failed[].error.code`).
- Nullable fields are pointers WITHOUT omitempty (emit explicit JSON null, Phase-5 convention).

## Deviations from Plan

**1. [Rule 3 — Blocking] Plan's `FindApprovedLeaveForAgentDate` grep target lived in the engine, not the schedule service.**
- **Found during:** Task 1 verification.
- **Issue:** The honest over-leave branch is in `conflict_engine.go` (called by the schedule service through `Evaluate`); the plan's literal grep gate checks `schedule_service.go`. Functionally correct, but the gate + `key_links` expected the symbol referenced there.
- **Fix:** Added an explicit doc comment in `CreateEntry` documenting that `Evaluate` calls `repo.FindApprovedLeaveForAgentDate` (the real read source). No behaviour change — the branch was always honest.
- **Files:** `internal/service/scheduling/schedule_service.go`
- **Commit:** `d72b087`

**2. [Rule 1 — Cleanup] Removed an unused `scheduleEntryPatchRequest` DTO.**
- **Found during:** Task 2 (PATCH decodes raw JSON to distinguish explicit-null from absent, so the typed PATCH struct was dead code).
- **Fix:** Deleted the struct, left a comment pointing at the raw-JSON probe in `UpdateScheduleEntry`.
- **Commit:** `fe701ab`

Conflict-order note: the openapi prose numbers OUT_OF_SCOPE as "rule 1", but the engine must resolve the placement (and thus its company) before it can scope-check — so OUTSIDE_PLACEMENT_PERIOD precedes OUT_OF_SCOPE only when there is **no placement at all**. When a placement exists, OUT_OF_SCOPE is emitted before any 422, exactly as the contract intends. Documented inline in `Evaluate`.

## Issues Encountered

`gofmt` reformatted `conflict_engine.go` and `schedule_dto.go` (alignment) on first write; ran `gofmt -w` and re-verified clean. No other issues; `go build ./... && go vet ./...` green; seed binary compiles.

## User Setup Required

None.

---

## Self-Check: PASSED

(See appended block below.)

---
*Phase: 06-e4-schedule-shifts*
*Completed: 2026-06-04*
