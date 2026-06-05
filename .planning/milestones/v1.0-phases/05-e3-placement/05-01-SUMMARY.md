---
phase: 05-e3-placement
plan: 01
subsystem: database
tags: [postgres, goose, sqlc, pgx, placement, shift-leader, partial-unique-index, invariants]

# Dependency graph
requires:
  - phase: 03-e2-org-master-data
    provides: client_companies / client_sites / service_lines / positions tables (FK targets)
  - phase: 04-e2-people
    provides: employees / employment_agreements tables + partial-unique-index + supersede-on-renew pattern
provides:
  - placements table with INV-1 partial unique index + INV-5 site FK (00020)
  - placement_history table (bigserial PK, one row per lifecycle transition) (00021)
  - shift_leader_assignments table + INV-2 (company+site) and INV-3 partial unique indexes (00022)
  - 24 sqlc queries (CRUD + lifecycle + FOR UPDATE locking reads + roster + expiring + chain CTE)
  - domain types Placement / PlacementHistory / ShiftLeaderAssignment / CompanyRosterSummary + filters
affects: [05-02 services+handlers, 05-03 contract tests, 05-04 E2E]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "id allocated via column DEFAULT ('SWP-PL-'||swp_next_id('PL')) — diverges from Phase-4 inline-INSERT allocation but plan-specified"
    - "Recursive CTE for predecessor/successor chain walk (GetPlacementChain)"
    - "status (single) + status__in (CSV → ANY($::text[])) dual-param filtering on lifecycle_status"
    - "denormalizing LEFT JOINs fill *_name display fields (nullable → *string in sqlc)"

key-files:
  created:
    - backend/db/migrations/00020_placements.sql
    - backend/db/migrations/00021_placement_history.sql
    - backend/db/migrations/00022_shift_leader_assignments.sql
    - backend/db/queries/placement/placements.sql
    - backend/db/queries/placement/placement_history.sql
    - backend/db/queries/placement/shift_leader_assignments.sql
    - backend/internal/domain/placement.go
  modified:
    - backend/internal/repository/sqlc/models.go (regenerated)
    - backend/internal/repository/sqlc/querier.go (regenerated)

key-decisions:
  - "id via column DEFAULT (plan-specified) rather than Phase-4 inline INSERT allocation; CreatePlacement/CreateShiftLeaderAssignment omit id from the column list so the DEFAULT fires"
  - "INV-1 partial-index predicate kept verbatim incl. inert 'SCHEDULED' forward-compat term (plan Task 1 DECISION)"
  - "placement_history uses bigserial PK — no SWP id, avoids touching ids.go"

patterns-established:
  - "Pattern: per-placement and per-employee FOR UPDATE locks back the INV-1..4 service checks"
  - "Pattern: roster summary via two GROUP BY queries (RosterSummaryByStatus / RosterSummaryByServiceLine)"

requirements-completed: [PLC-01, PLC-02, PLC-03, PLC-04]

# Metrics
duration: 4min
completed: 2026-06-04
---

# Phase 5 Plan 01: E3 Placement Data Layer Summary

**Race-proof, history-preserving E3 data model: placements (INV-1 partial unique index + INV-5 site FK), placement_history (bigserial transition log), and shift_leader_assignments (INV-2 company+site / INV-3 partial unique indexes), with 24 sqlc queries incl. FOR UPDATE invariant locks, and matching domain types — `make gen` + `go build` + `go vet` all clean.**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-06-04T13:54:47Z
- **Completed:** 2026-06-04T13:58:29Z
- **Tasks:** 3
- **Files modified:** 9 (7 created hand-written + 2 regenerated sqlc)

## Accomplishments
- Three goose migrations (00020/00021/00022) with all four partial unique indexes enforcing INV-1, INV-2 (company + site scope), and INV-3 at the DB level.
- Three sqlc query files (24 named queries) covering placement CRUD, lifecycle transitions, row-locking reads for invariant pre-checks (INV-1/INV-4 + leader INV-2/INV-3), the expiring list, the company roster + two summary GROUP BYs, and a recursive-CTE chain walk.
- `internal/domain/placement.go` with `Placement`, `PlacementHistory`, `ShiftLeaderAssignment`, `CompanyRosterSummary`, and `PlacementFilter` / `ExpiringFilter` / `ShiftLeaderFilter`.
- `make gen` regenerates sqlc with zero hand edits and zero drift; `go build ./...`, `go vet ./...`, and `gofmt -l` are all clean.

## Task Commits

Each task was committed atomically:

1. **Task 1: 00020 placements migration (INV-1 + INV-5)** - `00ebb56` (feat)
2. **Task 2: 00021 placement_history + 00022 shift_leader_assignments** - `9d757a1` (feat)
3. **Task 3: sqlc queries + domain types; make gen + go build clean** - `cd49400` (feat)

## Files Created/Modified
- `backend/db/migrations/00020_placements.sql` — placements table; `placements_active_employee_uq` (INV-1); company/employee/status_changed indexes; chain FKs; 9-value `lifecycle_status` CHECK; `ended_reason` CHECK.
- `backend/db/migrations/00021_placement_history.sql` — `placement_history` (bigserial PK) + `placement_history_placement_idx`.
- `backend/db/migrations/00022_shift_leader_assignments.sql` — `shift_leader_assignments`; `sla_active_company_uq` (site_id IS NULL), `sla_active_site_uq` (site_id IS NOT NULL), `sla_active_employee_uq` (INV-3); `vacated_reason` CHECK.
- `backend/db/queries/placement/placements.sql` — see query inventory below.
- `backend/db/queries/placement/placement_history.sql` — `InsertPlacementHistory`, `ListPlacementHistory`.
- `backend/db/queries/placement/shift_leader_assignments.sql` — `ListShiftLeaderAssignments`, `GetActiveLeaderForCompanyForUpdate`, `GetActiveLeaderForSiteForUpdate`, `GetActiveAssignmentForEmployeeForUpdate`, `GetShiftLeaderAssignmentByID`, `CreateShiftLeaderAssignment`, `EndShiftLeaderAssignment`.
- `backend/internal/domain/placement.go` — domain structs + filters.

## Reference for 05-02 (so the repository wires without re-deriving)

### Table / column names
- **placements**: id, employee_id, agreement_id, client_company_id, **site_id**, service_line_id, position_id, start_date, end_date, annual_leave_entitlement_days (`integer`), base_salary_ref_idr (`bigint`), notes, lifecycle_status, status_changed_at, ended_reason, ended_at, termination_reason, resign_at, predecessor_id, successor_id, backdate_reason, created_by, created_at, updated_at, deleted_at.
- **placement_history**: id (`bigserial`), placement_id, action, actor_user_id, reason, effective_date, status_before, status_after, notes, created_at.
- **shift_leader_assignments**: id, client_company_id, site_id (nullable), employee_id, assigned_at, unassigned_at, assigned_by, vacated_reason, notes, created_at, updated_at.

### Exact index names
- `placements_active_employee_uq` — `WHERE lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START','SCHEDULED') AND deleted_at IS NULL` (INV-1 backstop; 23505 on conflict → map to `INV_1_VIOLATION`).
- `sla_active_company_uq` — `WHERE unassigned_at IS NULL AND site_id IS NULL` (INV-2 company).
- `sla_active_site_uq` — `WHERE unassigned_at IS NULL AND site_id IS NOT NULL` (INV-2 site).
- `sla_active_employee_uq` — `WHERE unassigned_at IS NULL` (INV-3).

### sqlc query names (Querier methods)
- placements: `ListPlacements`, `ListExpiringPlacements`, `GetPlacementByID`, `GetPlacementChain`, `GetActivePlacementForEmployee`, `GetActivePlacementForEmployeeAtCompanyForUpdate`, `LockEmployeePlacements`, `CreatePlacement`, `UpdatePlacementFields`, `SetPlacementLifecycle`, `SetPlacementPredecessor`, `SetPlacementSuccessor`, `RosterForCompany`, `RosterSummaryByStatus`, `RosterSummaryByServiceLine`.
- placement_history: `InsertPlacementHistory`, `ListPlacementHistory`.
- shift_leader_assignments: `ListShiftLeaderAssignments`, `GetActiveLeaderForCompanyForUpdate`, `GetActiveLeaderForSiteForUpdate`, `GetActiveAssignmentForEmployeeForUpdate`, `GetShiftLeaderAssignmentByID`, `CreateShiftLeaderAssignment`, `EndShiftLeaderAssignment`.

### Domain struct field names
- `domain.Placement` fields: ID, EmployeeID, AgreementID, ClientCompanyID, SiteID, ServiceLineID, PositionID, StartDate, EndDate(*time.Time), AnnualLeaveEntitlementDays(*int32), BaseSalaryRefIDR(*int64), Notes(*string), LifecycleStatus, StatusChangedAt, EndedReason(*string), EndedAt(*time.Time), TerminationReason(*string), ResignAt(*time.Time), PredecessorID(*string), SuccessorID(*string), BackdateReason(*string), CreatedBy(*string), CreatedAt, UpdatedAt, + denormalized EmployeeName/ClientCompanyName/SiteName/ServiceLineName/PositionName/AgreementType (all `*string`), Warnings([]string).
- `domain.ShiftLeaderAssignment` fields: ID, ClientCompanyID, SiteID(*string), EmployeeID, AssignedAt, UnassignedAt(*time.Time), AssignedBy(*string), VacatedReason(*string), Notes(*string), CreatedAt, UpdatedAt, ClientCompanyName/EmployeeName(*string). Method `Active() bool` = UnassignedAt == nil.
- `domain.CompanyRosterSummary`: TotalActive, TotalScheduled, TotalExpiring (int), ByServiceLine([]RosterServiceLineCount), ByStatus([]RosterStatusCount).
- Filters: `PlacementFilter` (CompanyID, ServiceLineID, EmployeeID, AgreementID, Status, StatusIn[]string, Q, EndDateLTE, IncludeHistory, Limit, CursorStatusChangedAt, CursorID), `ExpiringFilter` (Cutoff, CompanyID, Limit, CursorEndDate, CursorID), `ShiftLeaderFilter` (CompanyID, EmployeeID, ActiveOnly).

### sqlc type-mapping quirks (05-02 must handle)
- **id via column DEFAULT**: `CreatePlacement` / `CreateShiftLeaderAssignment` do **not** accept an `id` param — the DB DEFAULT allocates it; `RETURNING` gives the id back.
- **status__in → `[]string`**: sqlc generated `StatusIn []string` for the `text[]` ANY filter; pass a Go slice (empty/nil slice → narg NULL → filter skipped). Repo builds it from the CSV query param.
- **Date columns → `pgtype.Date`**: `ListExpiringPlacements.Cutoff` and all date args/returns are `pgtype.Date`; nullable dates (end_date, ended_at, resign_at, effective_date) are nullable pgtype — map to `*time.Time` in the repo as Phase-4 people repo does.
- **LEFT JOIN columns → pointers**: denormalized *_name and agreement_type columns generated as `*string` (left-join nullable) even though source columns are NOT NULL — repo dereferences with nil-guard.
- **count aggregates → `int64`**: `RosterSummaryBy*` COUNT(*) returns `int64`; domain counts are `int` — cast in the repo.
- **bigserial → `int64`**: `placement_history.id` is `int64` in domain.PlacementHistory.
- **IncludeHistory / ActiveOnly are `arg` (non-null bool)**: pass real booleans, not pointers.

## Decisions Made
- **id allocation strategy:** Followed the plan's literal SQL (column `DEFAULT ('SWP-PL-'||swp_next_id('PL'))`) rather than Phase-4's inline-INSERT allocation. Both are valid; the DEFAULT keeps the CreatePlacement INSERT free of the id column and satisfies the plan's Task-1 acceptance criterion verbatim. Noted as a minor deviation (below).
- **INV-1 predicate verbatim incl. 'SCHEDULED':** Kept per the plan's explicit DECISION — an inert forward-compat backstop that never matches the CHECK-constrained column.
- **GetPlacementChain via recursive CTE:** Walks both predecessor and successor links from the seed placement so 05-02 can build `history_chain` for any node in the chain.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Convention reconciliation] id allocation: column DEFAULT vs inline INSERT**
- **Found during:** Task 1 / Task 3
- **Issue:** The plan's SQL snippet uses a column `DEFAULT ('SWP-PL-'||swp_next_id('PL'))`, but the established Phase-4 repo idiom (00016/00017) allocates the SWP id inline inside the `CreateX` INSERT with a plain `id text PRIMARY KEY`. Mixing both would be incoherent.
- **Fix:** Followed the plan's column-DEFAULT form (it is what the Task-1 acceptance criterion literally checks) and made `CreatePlacement` / `CreateShiftLeaderAssignment` omit `id` from the INSERT column list so the DEFAULT fires. Behaviour is identical (same `swp_next_id` sequence); only the allocation site differs from people.
- **Files modified:** 00020/00022 migrations, placements.sql, shift_leader_assignments.sql
- **Verification:** `make gen` + `go build` clean; generated `CreatePlacementParams` has no `ID` field; RETURNING surfaces the allocated id.
- **Committed in:** `00ebb56` / `cd49400`

---

**Total deviations:** 1 (convention reconciliation; behaviour-neutral)
**Impact on plan:** No scope creep. The divergence from the people slice's id-allocation site is documented so 05-02's repository omits id from create params.

## Issues Encountered
- `gofmt` flagged `internal/domain/placement.go` after first write (struct-field alignment of the denormalized block). Ran `gofmt -w`; rebuilt clean.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Data layer is complete and the migrations apply cleanly via sqlc's schema read. 05-02 can build the repository over the 24 generated Querier methods and the domain types using the reference section above.
- Open for 05-02: repository mapping (pgx.ErrNoRows → domain.ErrNotFound), service-level INV-1..4 enforcement using the FOR UPDATE locks + the 23505 unique-violation backstop, lifecycle state machine, transfer/renew atomicity, roster assembly, routes/seed.
- No blockers.

## Self-Check: PASSED

---
*Phase: 05-e3-placement*
*Completed: 2026-06-04*
