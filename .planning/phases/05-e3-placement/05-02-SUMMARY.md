---
phase: 05-e3-placement
plan: 02
subsystem: backend-service-handler
tags: [go, chi, placement, lifecycle, invariants, shift-leader, roster, INV-1, INV-2, INV-3, INV-4, audit, transactional]

# Dependency graph
requires:
  - phase: 05-e3-placement
    plan: 01
    provides: placements/placement_history/shift_leader_assignments tables + 24 sqlc queries + domain types
  - phase: 04-e2-people
    provides: TxRunner/Clock pattern, agreements supersede-before-insert, apperr/audit/httpx kernel
provides:
  - error.details envelope (apperr.Details + ConflictWithDetails + httpx serialization) for INV violations
  - PlacementService (CRUD + lifecycle state machine + INV-1 + transfer/renew atomicity)
  - ShiftLeaderService (INV-2/3/4 via FOR UPDATE locks + leader_scope + roster scope guard)
  - 13 FE-used E3 HTTP endpoints wired under /api/v1 with RequireRole groups
  - seed: SWP-AG-7002/7003/7004 + SWP-PL-5001..5004 (5004 EXPIRING) + SWP-SLA-3001
affects: [05-03 contract tests, 05-04 E2E]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "apperr.Error gains a Details any field + ConflictWithDetails(code, fields, details) helper; httpx errBody serializes error.details (omitempty keeps Phase 1-4 errors byte-identical)"
    - "PlacementService + ShiftLeaderService are mutually referential: SetLeaderService wires the leader svc into the placement svc for current-leader joins + SL-6 auto-vacate"
    - "lifecycle_status derived at the DTO boundary (toPlacementResponse, Asia/Jakarta): persisted ACTIVE+end<=today+30d -> EXPIRING; PENDING_START+start<=today -> ACTIVE"
    - "one Handler struct aggregates placement+leader services -> a single server.Deps.Placement field"

key-files:
  created:
    - backend/internal/repository/placement/placements_repo.go
    - backend/internal/repository/placement/placements_mapping.go
    - backend/internal/repository/placement/shift_leader_repo.go
    - backend/internal/service/placement/placement_service.go
    - backend/internal/service/placement/shift_leader_service.go
    - backend/internal/handler/placement/placements_handler.go
    - backend/internal/handler/placement/placements_dto.go
    - backend/internal/handler/placement/shift_leader_handler.go
    - backend/internal/handler/placement/shift_leader_dto.go
    - backend/internal/handler/placement/roster_handler.go
  modified:
    - backend/internal/platform/apperr/apperr.go
    - backend/internal/platform/httpx/response.go
    - backend/internal/platform/i18n/i18n.go
    - backend/internal/server/server.go
    - backend/cmd/api/main.go
    - backend/cmd/seed/seed.go

key-decisions:
  - "INV-1 = service pre-check (GetActivePlacementForEmployee) + FOR UPDATE re-check (LockEmployeePlacements) inside tx + 23505 unique-index backstop -> all map to 409 INV_1_VIOLATION with error.details.current_placement"
  - "INV-2/3/4 enforced in one InTx with the 05-01 ...ForUpdate locks; PENDING_START does NOT satisfy INV-4 (SL-2/C-2)"
  - "leader_scope read from client_companies.leader_scope; the FE web path is company-scope, so assign/replace/end + roster exercise the company-scope locks (GetActiveLeaderForCompanyForUpdate); site-scope repo method wired for Go tests"
  - "transfer reuses the source site_id (no site_id in the FE transfer request + no primary-site query in 05-01) — validated against the destination company when the company changes"
  - "non-FE list endpoint GET /shift-leader-assignments deferred (not in the 13-hook inventory); GetCurrentLeader service method exists for the detail join but no route mounted"

requirements-completed: [PLC-01, PLC-02, PLC-03, PLC-04]

# Metrics
duration: ~50min
completed: 2026-06-04
---

# Phase 5 Plan 02: E3 Placement Service + Handler Layer Summary

**The 13 FE-used E3 endpoints over the 05-01 data layer: placement CRUD + lifecycle state machine (renew/transfer/end/resign/terminate) with race-proof INV-1..4 enforcement (service pre-check + FOR UPDATE locks + DB partial-unique backstop), transfer/renew atomicity, placement_history + audit on every action, date-derived lifecycle_status at the DTO boundary, shift-leader assign/replace/end with leader_scope, and the company roster with scope guard — response shapes match both `docs/api/E3-placement/openapi.yaml` and the built e3-placement FE components. `make gen` + `go build ./...` + `go vet ./...` + `gofmt -l` all clean; seed boots the E2E personas.**

## Accomplishments

- **Platform error envelope** extended additively to carry `error.details` (INVViolationDetails) — `apperr.Error.Details any` + `apperr.ConflictWithDetails(code, fields, details)`; `httpx.errBody.Details json:"details,omitempty"` (Phase 1-4 errors stay byte-identical). 11 E3 i18n codes added in id+en.
- **PlacementService** (consumer-defined `PlacementRepository`, `TxRunner`, `Clock`): list (keyset on status_changed_at desc), dedicated expiring (end_date asc), get (+history_chain +current_leader +NO_SHIFT_LEADER warning), create (INV-1 pre-check + tx lock re-check + 23505 backstop; BR-1b/3/4/5/6; auto-cap warning), update (terminal-immutable + read-only reject), end/resign/terminate (TERMINAL_STATE_IMMUTABLE; terminate company-name confirm), transfer (atomic close+successor), renew (supersede-before-insert).
- **ShiftLeaderService**: INV-2/3/4 under FOR UPDATE locks + leader_scope; assign (replace=true atomic swap), replace (by id), end (MANUAL), GetCurrentLeader, roster (GuardCompany scope, summary by_service_line/by_status), and `autoVacateForEmployeeAtCompany` for SL-6 cascade.
- **Handlers/DTOs**: snake_case responses matching openapi AND the FE component field set; `lifecycle_status` derived at the boundary; one `Handler` aggregating both services.
- **Routes** appended after the Phase-5 marker; `/placements/expiring` registered before `/placements/{id}`. **main.go** wires repos→services→handler with `SetLeaderService`. **Seed** extends with persona agreements + 4 placements + 1 SLA.

## Task Commits

1. **Task 1: error.details envelope + E3 i18n codes** — `f259eb1` (feat)
2. **Task 2: placement repository + service (CRUD, lifecycle, INV-1, transfer/renew)** — `ece6e36` (feat)
3. **Task 3: handlers/DTOs + shift-leader service/repo + routes + main + seed** — `ad4036e` (feat)

## Reference for 05-03 / 05-04

### Endpoint → apperr code map (the contract the contract-tests assert)

| Method + path | Success | Error codes emitted |
| --- | --- | --- |
| `GET /placements` | 200 list | (cursor) CURSOR_MISMATCH 400 |
| `GET /placements/expiring` | 200 list (end_date:asc) | — |
| `GET /placements/{id}` | 200 detail | NOT_FOUND 404 |
| `POST /placements` | 201 +Location | INV_1_VIOLATION 409 (details.current_placement+suggested_actions), COMPANY_INACTIVE 409, PLACEMENT_OUTSIDE_CONTRACT 422, INVALID_REQUEST 400 (end_date/backdate_reason/site_id/agreement_id), RULE_VIOLATION 422 (inactive employee), NOT_FOUND 404 |
| `PATCH /placements/{id}` | 200 | TERMINAL_STATE_IMMUTABLE 409, INVALID_REQUEST 400 (read-only field present), NOT_FOUND 404 |
| `POST /placements/{id}:transfer` | 201 +Location | TERMINAL_STATE_IMMUTABLE 409, COMPANY_INACTIVE 409, RULE_VIOLATION 422 (no actual change), INV_1_VIOLATION 409 (backstop) |
| `POST /placements/{id}:renew` | 201 +Location | TERMINAL_STATE_IMMUTABLE 409, COMPANY_INACTIVE 409, PLACEMENT_PERIOD_OVERLAP 422 (1-day buffer) |
| `POST /placements/{id}:end` | 200 | TERMINAL_STATE_IMMUTABLE 409 |
| `POST /placements/{id}:resign` | 200 | TERMINAL_STATE_IMMUTABLE 409, INVALID_REQUEST 400 (resignation_reason) |
| `POST /placements/{id}:terminate` | 200 | TERMINAL_STATE_IMMUTABLE 409, INVALID_REQUEST 400 (termination_reason<10 / type_company_name_confirm) |
| `POST /shift-leader-assignments` | 201 | INV_4_VIOLATION 409, INV_3_VIOLATION 409, INV_2_VIOLATION 409 (unless replace), COMPANY_INACTIVE 409, LEADER_NOT_ELIGIBLE 422 |
| `POST /shift-leader-assignments/{id}:replace` | 201 | ALREADY_ENDED 409, INV_3_VIOLATION 409, INV_4_VIOLATION 409, LEADER_NOT_ELIGIBLE 422 |
| `POST /shift-leader-assignments/{id}:end` | 200 | ALREADY_ENDED 409 |
| `GET /client-companies/{company_id}/roster` | 200 | OUT_OF_SCOPE 403 (SL other company), NOT_FOUND 404 |

### Response JSON field sets (so 05-04 confirms the FE renders against them)

- **Placement** (list rows + detail + roster rows + transfer/renew predecessor/successor): `id, employee_id, employee_name, agreement_id, agreement_type, client_company_id, client_company_name, site_id, site_name, service_line_id, service_line_name, position_id, position_name, start_date, end_date(null=open), annual_leave_entitlement_days, base_salary_ref_idr, notes, lifecycle_status(derived), status_changed_at, ended_reason, ended_at, termination_reason, resign_at, predecessor_id, successor_id, backdate_reason, created_by, created_at, updated_at, warnings[]`.
- **List/expiring**: `{ data: Placement[], next_cursor, has_more }`.
- **Detail**: `{ placement: Placement, history_chain: PlacementSummary[], current_shift_leader: ShiftLeaderSummary|null }`. PlacementSummary = `id, employee_id, client_company_id, client_company_name, service_line_id, service_line_name, lifecycle_status, start_date, end_date`. ShiftLeaderSummary = `id, client_company_id, client_company_name, employee_id, employee_name, assigned_at, unassigned_at`.
- **Roster**: `{ company_id, company_name, current_shift_leader: ShiftLeaderSummary|null, placements: Placement[], next_cursor, has_more, summary: { total_active, total_scheduled, total_expiring, by_service_line:[{service_line_id, service_line_name, count}], by_status:[{status, count}] } }`.
- **Transfer**: `{ predecessor, successor, vacated_assignment|null, warnings[] }`. **Renew**: `{ predecessor, successor, warnings[] }`.
- **SLA create/replace**: `{ assignment: ShiftLeaderAssignment, replaced_assignment|null }`. SLA end/get: `ShiftLeaderAssignment` (`id, client_company_id, client_company_name, employee_id, employee_name, assigned_at, unassigned_at, assigned_by, vacated_reason, active, notes, created_at, updated_at`).
- **INV error.details** (INVViolationDetails): `invariant, current_placement?(INV-1), current_assignment?(INV-2), existing_assignment?(INV-3), company_id/employee_id/employee_placements_at_company?(INV-4), suggested_actions[]` (end|transfer|replace|end_existing_first|assign_after_placement).

### Seeded IDs (05-04 E2E targets)
- Agreements: SWP-AG-7002 (PKWTT/Sari), SWP-AG-7003 (PKWT/Rudi), SWP-AG-7004 (PKWT/Dewi). (7001/Budi pre-existing.)
- Placements (ACTIVE): SWP-PL-5001 Rudi@CMP-0021/SITE-0001/Parking; SWP-PL-5002 Budi@CMP-0022/SITE-0002/Parking; SWP-PL-5003 Sari@CMP-0021/SITE-0001/BldgMgmt(open-ended); SWP-PL-5004 Dewi@CMP-0021/SITE-0001/Parking (end=today+20d → DTO derives EXPIRING).
- Shift leader: SWP-SLA-3001 Rudi@CMP-0021 (company-scope). Rudi is actively placed at CMP-0021 (SWP-PL-5001) so INV-2/4 hold.
- E2E negatives: `POST /placements` for an already-placed agent → 409 INV_1_VIOLATION; `POST /shift-leader-assignments` for CMP-0021 (has Rudi) → 409 INV_2_VIOLATION; roster of CMP-0022 as Rudi → 403 OUT_OF_SCOPE.

## Deviations from Plan

### Auto-fixed / documented constraints

**1. [Rule 3 - blocking] Transfer destination site_id**
- **Found during:** Task 2 (TransferPlacement).
- **Issue:** placements.site_id is NOT NULL (INV-5) but the FE TransferModal request (placement-overlays.tsx) sends no `new_site_id`, and 05-01 exposes no "primary site for company" query, so a cross-company transfer has no destination site to write.
- **Fix:** Reuse the predecessor's `site_id` for the successor; when the company changes, validate the source site belongs to the destination company before reusing it. Service-line-only transfers (same company) always keep the site. This satisfies the FE flow (which exercises service-line transfers within the same company) without an architectural query addition. A future cross-company transfer with a distinct site needs a destination-site resolver — flagged for E4/E9.
- **Files:** placement_service.go (TransferPlacement).
- **Committed in:** ece6e36.

**2. [Rule 2 - correctness] EndShiftLeaderAssignment optional body**
- **Issue:** openapi marks the `:end` request body `required: false`; an empty body makes the JSON decoder return EOF.
- **Fix:** The end handler decodes best-effort (`_ = decodeJSON`), leaving the request zero-valued (reason→nil→MANUAL) when the body is absent. Matches the spec default.
- **Files:** shift_leader_handler.go.
- **Committed in:** ad4036e.

### Deferred (out-of-scope, logged to deferred-items.md)
- `make verify` fails on `golangci-lint` ("unsupported version of the configuration") — a pre-existing tooling/config version drift unrelated to 05-02. All plan gates (`make gen`, `go build ./...`, `go vet ./...`, `gofmt -l`) pass clean.

**Total deviations:** 2 documented constraints + 1 deferred tooling issue. No scope creep; no architectural change.

## Authentication Gates

None — no external service auth required.

## Self-Check: PASSED

- All 11 created files present on disk; 6 modified files present.
- Commits f259eb1 / ece6e36 / ad4036e all in git history.
- `make gen` (no sqlc drift) + `go build ./...` + `go vet ./...` + `gofmt -l` clean.

---
*Phase: 05-e3-placement*
*Completed: 2026-06-04*
