---
phase: 03-e2-org-master-data
plan: 04
subsystem: backend-org
tags: [go, chi, rest, leave-types, attendance-codes, overtime-rules, master-data, audit, seed]

# Dependency graph
requires:
  - phase: 03-e2-org-master-data
    plan: 01
    provides: sqlc generated methods (ListLeaveTypes, CreateLeaveType, UpdateLeaveType, SetLeaveTypeStatus, SoftDeleteLeaveType, GetLeaveTypeByID; same for AttendanceCodes and OvertimeRules)
  - phase: 03-e2-org-master-data
    plan: 02
    provides: org package (mapErr/nullStr helpers in companies_repo.go; TxRunner/Clock/pageCursor/isUniqueViolation in companies_service.go; decodeJSON/parseLimit/queryStringPtr/derefString/coalesce in companies_handler.go); OrgCompanies Deps field; server.go COORDINATION POINT comment
  - phase: 03-e2-org-master-data
    plan: 03
    provides: server.go coordination comment for 03-04; seed.go seedServiceLines() call as insert point; ServiceLineService pattern to mirror

provides:
  - 12 HTTP endpoints for leave types (4), attendance codes (4), overtime rules (4)
  - domain.LeaveType + domain.AttendanceCode + domain.OvertimeRule types + filters
  - MasterDataRepository port (service/org) + MasterDataRepository (repository/org) implementation
  - orghttp.MasterDataHandler mounted in server.go alongside OrgCompanies + OrgServiceLines
  - seed: SWP-LT-001/002, SWP-AC-001/002, SWP-OTR-001

affects:
  - Phase 4 (employees/agreements): can now reference leave_types, attendance_codes FK values
  - Phase 5 (placements): OvertimeRule guard TODO(Phase 7/8) OTR_IN_USE
  - Phase 6 (leave): leave_type_id FK in leave_requests resolves to SWP-LT seeded rows
  - Phase 7 (overtime): overtime_rule_id FK in overtime_requests resolves to SWP-OTR seeded rows

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "org-master-data pattern (identical to 03-03): domain/org_masterdata.go -> repository/org/masterdata_repo.go -> service/org/masterdata_service.go -> handler/org/masterdata_{dto,handler}.go -> server.go 3 r.Group{} blocks"
    - "apperr.Rule('RULE_VIOLATION', fields) for min_minutes<30 (422) — before the tx, no DB round-trip"
    - "float64 in overtimeRuleResponse (not float32) to avoid JSON noise on 1.5/2.0/3.0 serialization"
    - "service_line_id is *string (nullable JSON null) — not omitempty, always serialized"
    - "SoftDelete is hard soft-delete via deleted_at (not SetStatus); TODO Phase 7/8 in-use guards are no-ops in Phase 3"
    - "UPPERCASE status only at DTO boundary (strings.ToUpper); DB stores lowercase"
    - "Seed uses explicit IDs (SWP-LT-001/002, SWP-AC-001/002, SWP-OTR-001) with ON CONFLICT (id) DO NOTHING for deterministic E2E"
    - "derefBool/coalesceB helpers added in masterdata_handler.go (alongside package-level decodeJSON/coalesce from companies_handler.go)"

key-files:
  created:
    - backend/internal/domain/org_masterdata.go
    - backend/internal/repository/org/masterdata_repo.go
    - backend/internal/service/org/masterdata_service.go
    - backend/internal/handler/org/masterdata_dto.go
    - backend/internal/handler/org/masterdata_handler.go
  modified:
    - backend/internal/server/server.go (OrgMasterData Deps field + 3 ORG master-data route groups + end comment)
    - backend/cmd/api/main.go (orgMasterDataRepo/Svc/Handler wiring into server.Deps)
    - backend/cmd/seed/seed.go (seedMasterData: 2 leave types + 2 attendance codes + 1 overtime rule)

key-decisions:
  - "MasterDataService is a separate struct from Service and ServiceLineService in the same org package — parallel-merge clean, no coupling"
  - "MasterDataHandler in same orghttp package — OrgMasterData Deps field type = *orghttp.MasterDataHandler (no new import alias needed)"
  - "min_minutes validation uses apperr.Rule('RULE_VIOLATION', ...) BEFORE the tx (no DB round-trip on invalid input)"
  - "OvertimeRule domain type uses float64 (not float32); repository converts float32 sqlc types to float64 at the mapping boundary"
  - "UpdateOvertimeRule.ServiceLineID: nil pointer = 'keep current' semantics; service carries forward current.ServiceLineID when request field is nil"
  - "3 route groups for master data: (a) LT+AC reads all 4 roles, (b) OTR reads 3 roles (agent excluded per spec x-rbac), (c) all writes super_admin+hr_admin"
  - "Seed explicit IDs not swp_next_id() — matches OpenAPI spec examples exactly for deterministic E2E test references"

requirements-completed: [ORG-04]

# Metrics
duration: 12min
completed: 2026-06-04
---

# Phase 03 Plan 04: Operational Master Data (Leave Types + Attendance Codes + Overtime Rules) — Summary

**Leave types, attendance codes, and overtime rules backend slice (ORG-04): domain types, sqlc-backed repository, service (CRUD + min_minutes>=30 RULE_VIOLATION + dup->409 + soft-delete + audit on every write), chi handlers (12 endpoints), 3 route groups in server.go with correct per-op RBAC, seedMasterData() with 5 canonical rows. `go build + go vet + gofmt` all clean.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-06-04T03:25:14Z
- **Completed:** 2026-06-04T03:37:01Z
- **Tasks:** 3
- **Files created/modified:** 8

## Accomplishments

- Created `domain/org_masterdata.go` with `LeaveType`, `AttendanceCode`, `OvertimeRule` domain types and their corresponding Filter structs, matching the OpenAPI schema field-for-field (float64 rates, *string nullable service_line_id, bool flags)
- Implemented `MasterDataRepository` (repository port in service package) and `MasterDataRepository` (sqlc-backed impl in repository/org); reused `mapErr`/`nullStr` helpers from `companies_repo.go` — no redeclaration; `toOvertimeRuleDomain` converts float32 sqlc types to float64
- Service implements OR-1 (min_minutes < 30 → RULE_VIOLATION 422 via apperr.Rule before the tx), dup name/code (unique index) → apperr.Conflict("CONFLICT") (409), soft-delete (hard deleted_at via SoftDeleteLeaveType/AttendanceCode/OvertimeRule), audit.Record on every write; GetLeaveType/GetAttendanceCode getters for partial-update carry-forward
- Handler: `MasterDataHandler` with 12 methods; SoftDelete returns 204; Create returns 201+Location; status UPPERCASED; cursor decode; derefBool/coalesceB local helpers; all three boolean-filter patterns (is_annual, is_billable, service_line query param)
- server.go: `OrgMasterData *orghttp.MasterDataHandler` Deps field; 3 dedicated route groups (LT+AC reads all 4 roles, OTR reads 3 roles excl agent per x-rbac, all writes super_admin+hr_admin with idempotency on POST)
- seed.go: idempotent `seedMasterData()` — 2 leave types (SWP-LT-001 Cuti Tahunan ANNUAL, SWP-LT-002 Cuti Sakit SICK), 2 attendance codes (SWP-AC-001 PRESENT, SWP-AC-002 LATE), 1 overtime rule (SWP-OTR-001 Default OT weekday=1.5 restday=2.0 holiday=3.0)

## Task Commits

1. **Task 1: Domain + repository + service** - `6b53a90`
2. **Task 2: Service extensions + handlers/DTOs** - `05b027e`
3. **Task 3: Routes + deps + seed** - `226d9e4`

## Files Created / Modified

### Created
- `backend/internal/domain/org_masterdata.go` — LeaveType, AttendanceCode, OvertimeRule + Filter structs
- `backend/internal/repository/org/masterdata_repo.go` — sqlc-backed MasterDataRepository; toOvertimeRuleDomain mapper; compile-time interface check
- `backend/internal/service/org/masterdata_service.go` — MasterDataRepository port + MasterDataService; OR-1 rule; dup->409; audit on every write
- `backend/internal/handler/org/masterdata_dto.go` — request/response structs; status UPPERCASE; float64 rates; nullable *string service_line_id
- `backend/internal/handler/org/masterdata_handler.go` — MasterDataHandler with 12 methods; 204 for soft-delete; 201+Location for creates

### Modified
- `backend/internal/server/server.go` — OrgMasterData Deps field + 3 ORG master-data route groups with per-op RBAC + end comment
- `backend/cmd/api/main.go` — orgMasterDataRepo/Svc/Handler wiring into server.Deps
- `backend/cmd/seed/seed.go` — seedMasterData() with 5 canonical rows

## Mounted Routes (12 endpoints)

| Method | Path | RBAC | Handler |
|--------|------|------|---------|
| GET | /api/v1/leave-types | all 4 roles | ListLeaveTypes |
| POST | /api/v1/leave-types | super_admin, hr_admin | CreateLeaveType |
| PATCH | /api/v1/leave-types/{leave_type_id} | super_admin, hr_admin | UpdateLeaveType |
| DELETE | /api/v1/leave-types/{leave_type_id} | super_admin, hr_admin | SoftDeleteLeaveType -> 204 |
| GET | /api/v1/attendance-codes | all 4 roles | ListAttendanceCodes |
| POST | /api/v1/attendance-codes | super_admin, hr_admin | CreateAttendanceCode |
| PATCH | /api/v1/attendance-codes/{attendance_code_id} | super_admin, hr_admin | UpdateAttendanceCode |
| DELETE | /api/v1/attendance-codes/{attendance_code_id} | super_admin, hr_admin | SoftDeleteAttendanceCode -> 204 |
| GET | /api/v1/overtime-rules | super_admin, hr_admin, shift_leader | ListOvertimeRules |
| POST | /api/v1/overtime-rules | super_admin, hr_admin | CreateOvertimeRule |
| PATCH | /api/v1/overtime-rules/{overtime_rule_id} | super_admin, hr_admin | UpdateOvertimeRule |
| DELETE | /api/v1/overtime-rules/{overtime_rule_id} | super_admin, hr_admin | SoftDeleteOvertimeRule -> 204 |

## Seeded IDs (deterministic for E2E reference)

| ID | Entity | Code/Name | Key Flags |
|----|--------|-----------|-----------|
| SWP-LT-001 | leave_type | ANNUAL "Cuti Tahunan" | is_annual=true, quota=12 |
| SWP-LT-002 | leave_type | SICK "Cuti Sakit" | is_annual=false, requires_document=true |
| SWP-AC-001 | attendance_code | PRESENT "Hadir" | is_workday/is_paid/is_billable/needs_verification=true |
| SWP-AC-002 | attendance_code | LATE "Terlambat" | same flags as PRESENT |
| SWP-OTR-001 | overtime_rule | "Default OT" | weekday=1.5 restday=2.0 holiday=3.0 min=30 max=240 pre_approval=true |

## Final ORG Route Groups in server.go

| Slice | Groups | Endpoints |
|-------|--------|-----------|
| 03-02 client-companies+sites | reads (SA/HR/SL), writes (SA/HR) | 8 |
| 03-03 service-lines+positions | reads all, SL writes SA, pos writes SA/HR | 9 |
| 03-04 master-data | LT+AC reads all, OTR reads excl agent, writes SA/HR | 12 |
| **Total ORG endpoints** | | **29** |

## Decisions Made

- `MasterDataService` is a separate struct from `Service` and `ServiceLineService` — parallel-merge clean; all three services coexist in `service/org` package without coupling
- `MasterDataHandler` uses same `orghttp` package as `Handler` and `ServiceLineHandler` — `OrgMasterData` Deps field type `*orghttp.MasterDataHandler`, no new import alias
- `validateMinMinutes` uses `apperr.Rule("RULE_VIOLATION", ...)` — same 422 HTTPStatus as apperr's Rule helper, checked BEFORE the transaction (no unnecessary DB round-trip on invalid input)
- `overtimeRuleResponse` uses `float64` (not `float32`) to avoid JSON encoding artifacts (1.5000001 etc.) — repository converts from sqlc's `float32` to `float64` at mapping boundary
- `UpdateOvertimeRule` partial update: `nil ServiceLineID` in request = "keep current" (carry-forward); zeroed rates/minutes also carry forward from current record
- 3 separate route groups for master data: LT+AC reads include all 4 roles (incl agent), OTR reads exclude agent (spec x-rbac), all writes require super_admin or hr_admin with idempotency wrapper on POST

## Deviations from Plan

None — plan executed exactly as written. Go build + vet + gofmt all clean on first run.

## Self-Check

- `backend/internal/domain/org_masterdata.go` — FOUND
- `backend/internal/repository/org/masterdata_repo.go` — FOUND
- `backend/internal/service/org/masterdata_service.go` — FOUND
- `backend/internal/handler/org/masterdata_dto.go` — FOUND
- `backend/internal/handler/org/masterdata_handler.go` — FOUND
- Commit `6b53a90` — FOUND
- Commit `05b027e` — FOUND
- Commit `226d9e4` — FOUND
- `go build ./... && go vet ./...` — EXIT 0

## Self-Check: PASSED
