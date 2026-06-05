---
phase: 03-e2-org-master-data
plan: 01
subsystem: database
tags: [postgres, goose, sqlc, migrations, org-master-data]

# Dependency graph
requires:
  - phase: 02-e1-foundations
    provides: swp_next_id allocator (migration 00001), sqlc glob db/queries/*, ids.go prefixes (CMP/SITE/SVC/POS/LT/AC/OTR)

provides:
  - 7 goose migrations (00009–00015): client_companies, client_sites, service_lines, positions, leave_types, attendance_codes, overtime_rules
  - 7 sqlc query files under db/queries/org/ with full CRUD + cursor-list + status/count helpers
  - Generated internal/repository/sqlc/*.sql.go for all 7 entities (Querier interface updated)

affects:
  - 03-02-client-companies-service (imports Querier methods: ListClientCompanies, GetClientCompanyByID, CreateClientCompany, UpdateClientCompany, SetClientCompanyStatus, CountActiveSitesForCompany, CreateSite, GetSiteByID, ListSitesForCompany, UpdateSite, DemoteOtherPrimaries, SetSitePrimary, SetSiteStatus)
  - 03-03-service-lines-positions (imports: ListServiceLines, CreateServiceLine, SetServiceLineStatus, CountActivePositionsForLine, ListPositionsForLine, CreatePosition, SetPositionStatus, SoftDeletePosition)
  - 03-04-master-data (imports: ListLeaveTypes, CreateLeaveType, SoftDeleteLeaveType, ListAttendanceCodes, CreateAttendanceCode, ListOvertimeRules, CreateOvertimeRule)
  - Any phase that runs go run ./cmd/migrate up (tables must exist)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Goose Up/Down migration with CREATE TABLE + unique index WHERE deleted_at IS NULL
    - Partial unique index for one-primary-per-company invariant (INV-5): WHERE is_primary=true AND deleted_at IS NULL
    - sqlc query dir per epic under db/queries/<epic>/ (glob db/queries/* picks up subdirs)
    - Cursor keyset on (created_at DESC, id DESC) with (sqlc.narg(x)::T IS NULL OR ...) idiom
    - ID allocation inline: 'SWP-CMP-' || swp_next_id('CMP') inside INSERT

key-files:
  created:
    - backend/db/migrations/00009_client_companies.sql
    - backend/db/migrations/00010_client_sites.sql
    - backend/db/migrations/00011_service_lines.sql
    - backend/db/migrations/00012_positions.sql
    - backend/db/migrations/00013_leave_types.sql
    - backend/db/migrations/00014_attendance_codes.sql
    - backend/db/migrations/00015_overtime_rules.sql
    - backend/db/queries/org/client_companies.sql
    - backend/db/queries/org/client_sites.sql
    - backend/db/queries/org/service_lines.sql
    - backend/db/queries/org/positions.sql
    - backend/db/queries/org/leave_types.sql
    - backend/db/queries/org/attendance_codes.sql
    - backend/db/queries/org/overtime_rules.sql
    - backend/internal/repository/sqlc/client_companies.sql.go
    - backend/internal/repository/sqlc/client_sites.sql.go
    - backend/internal/repository/sqlc/service_lines.sql.go
    - backend/internal/repository/sqlc/positions.sql.go
    - backend/internal/repository/sqlc/leave_types.sql.go
    - backend/internal/repository/sqlc/attendance_codes.sql.go
    - backend/internal/repository/sqlc/overtime_rules.sql.go
  modified:
    - backend/internal/repository/sqlc/querier.go (extended with new Querier methods)
    - backend/internal/repository/sqlc/models.go (extended with new row/param types)

key-decisions:
  - "client_sites.geo_lat/geo_lng are nullable doubles (not a composite geo type); geofence_active is derived at the DTO boundary (not stored) — matches the openapi spec's readOnly computed field"
  - "ListClientCompanies accepts service_line and has_leader filter params but applies (IS NULL OR TRUE) — no placements/assignments table in Phase 3; service layer sets has_leader=false/active_placement_count=0 as Phase-3 stubs"
  - "ListSitesForCompany primary sort is is_primary DESC (primary site first per spec), with keyset cursor on (created_at, id) as the stable tie-break"
  - "positions.sql adds SoftDeletePosition :exec in addition to SetPositionStatus — DELETE endpoint does a true soft-delete (deleted_at), not just status='inactive'"
  - "ids.go NOT modified — CMP/SITE/SVC/POS/LT/AC/OTR prefixes already existed"

patterns-established:
  - "org-slice query pattern: queries under db/queries/org/<entity>.sql with standard List/Get/Create/Update/SetStatus + SoftDelete :exec where DELETE endpoint applies"
  - "nullable column pattern: geo_lat/geo_lng as double precision nullable (no wrapper type); sqlc.narg(geo_lat)::float8 not needed — sqlc infers float8 from double precision"

requirements-completed: [ORG-01, ORG-02, ORG-03, ORG-04]

# Metrics
duration: 25min
completed: 2026-06-04
---

# Phase 03 Plan 01: E2 Org & Master Data — Data Layer Summary

**7 goose migrations (00009–00015) + 7 sqlc query files under db/queries/org/ covering CRUD/cursor/status/count for client_companies, client_sites (geofence), service_lines, positions, leave_types, attendance_codes, and overtime_rules; make gen + go build clean.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-06-04T10:00:00+07:00
- **Completed:** 2026-06-04T10:02:33+07:00
- **Tasks:** 3
- **Files modified:** 22 (7 migrations, 7 query files, 7 generated .sql.go + models.go + querier.go)

## Accomplishments

- Created all 7 E2 org/master-data tables with correct column shapes, check constraints, soft-delete, and partial unique indexes (including INV-5 one-primary-per-company index)
- Wrote 42 sqlc-annotated queries covering list (cursor), get, create, update, set-status, count, demote-primaries, soft-delete per entity; make gen succeeded with no errors
- `go build ./...` and `go vet ./...` both exit 0; all 7 generated *.sql.go files present in internal/repository/sqlc/

## Generated sqlc Method Signatures (for 03-02..04 reference)

### client_companies
- `ListClientCompanies(ctx, ListClientCompaniesParams) ([]ListClientCompaniesRow, error)`
- `GetClientCompanyByID(ctx, id string) (GetClientCompanyByIDRow, error)`
- `CreateClientCompany(ctx, CreateClientCompanyParams) (CreateClientCompanyRow, error)`
- `UpdateClientCompany(ctx, UpdateClientCompanyParams) (UpdateClientCompanyRow, error)`
- `SetClientCompanyStatus(ctx, SetClientCompanyStatusParams) (SetClientCompanyStatusRow, error)`
- `CountActiveSitesForCompany(ctx, clientCompanyID string) (int64, error)`

### client_sites
- `ListSitesForCompany(ctx, ListSitesForCompanyParams) ([]ListSitesForCompanyRow, error)`
- `GetSiteByID(ctx, id string) (GetSiteByIDRow, error)`
- `CreateSite(ctx, CreateSiteParams) (CreateSiteRow, error)`
- `UpdateSite(ctx, UpdateSiteParams) (UpdateSiteRow, error)`
- `DemoteOtherPrimaries(ctx, DemoteOtherPrimariesParams) error`
- `SetSitePrimary(ctx, id string) (SetSitePrimaryRow, error)`
- `SetSiteStatus(ctx, SetSiteStatusParams) (SetSiteStatusRow, error)`

### service_lines
- `ListServiceLines(ctx, ListServiceLinesParams) ([]ListServiceLinesRow, error)`
- `GetServiceLineByID(ctx, id string) (GetServiceLineByIDRow, error)`
- `CreateServiceLine(ctx, name string) (CreateServiceLineRow, error)`
- `UpdateServiceLine(ctx, UpdateServiceLineParams) (UpdateServiceLineRow, error)`
- `SetServiceLineStatus(ctx, SetServiceLineStatusParams) (SetServiceLineStatusRow, error)`
- `CountActivePositionsForLine(ctx, serviceLineID string) (int64, error)`

### positions
- `ListPositionsForLine(ctx, ListPositionsForLineParams) ([]ListPositionsForLineRow, error)`
- `GetPositionByID(ctx, id string) (GetPositionByIDRow, error)`
- `CreatePosition(ctx, CreatePositionParams) (CreatePositionRow, error)`
- `UpdatePosition(ctx, UpdatePositionParams) (UpdatePositionRow, error)`
- `SetPositionStatus(ctx, SetPositionStatusParams) (SetPositionStatusRow, error)`
- `SoftDeletePosition(ctx, id string) error`

### leave_types
- `ListLeaveTypes(ctx, ListLeaveTypesParams) ([]ListLeaveTypesRow, error)`
- `GetLeaveTypeByID(ctx, id string) (GetLeaveTypeByIDRow, error)`
- `CreateLeaveType(ctx, CreateLeaveTypeParams) (CreateLeaveTypeRow, error)`
- `UpdateLeaveType(ctx, UpdateLeaveTypeParams) (UpdateLeaveTypeRow, error)`
- `SetLeaveTypeStatus(ctx, SetLeaveTypeStatusParams) (SetLeaveTypeStatusRow, error)`
- `SoftDeleteLeaveType(ctx, id string) error`

### attendance_codes
- `ListAttendanceCodes(ctx, ListAttendanceCodesParams) ([]ListAttendanceCodesRow, error)`
- `GetAttendanceCodeByID(ctx, id string) (GetAttendanceCodeByIDRow, error)`
- `CreateAttendanceCode(ctx, CreateAttendanceCodeParams) (CreateAttendanceCodeRow, error)`
- `UpdateAttendanceCode(ctx, UpdateAttendanceCodeParams) (UpdateAttendanceCodeRow, error)`
- `SetAttendanceCodeStatus(ctx, SetAttendanceCodeStatusParams) (SetAttendanceCodeStatusRow, error)`
- `SoftDeleteAttendanceCode(ctx, id string) error`

### overtime_rules
- `ListOvertimeRules(ctx, ListOvertimeRulesParams) ([]ListOvertimeRulesRow, error)`
- `GetOvertimeRuleByID(ctx, id string) (GetOvertimeRuleByIDRow, error)`
- `CreateOvertimeRule(ctx, CreateOvertimeRuleParams) (CreateOvertimeRuleRow, error)`
- `UpdateOvertimeRule(ctx, UpdateOvertimeRuleParams) (UpdateOvertimeRuleRow, error)`
- `SetOvertimeRuleStatus(ctx, SetOvertimeRuleStatusParams) (SetOvertimeRuleStatusRow, error)`
- `SoftDeleteOvertimeRule(ctx, id string) error`

## Task Commits

1. **Task 1: Migrations 00009–00012** - `41d47a9` (feat)
2. **Task 2: Migrations 00013–00015** - `94d868b` (feat)
3. **Task 3: sqlc query files + make gen** - `a504cbc` (feat)

## Files Created/Modified

- `backend/db/migrations/00009_client_companies.sql` — table with leader_scope, npwp, name/npwp unique indexes
- `backend/db/migrations/00010_client_sites.sql` — table with geo_lat/geo_lng/geofence_radius_m, is_primary, one-primary partial unique index
- `backend/db/migrations/00011_service_lines.sql` — table with name unique index
- `backend/db/migrations/00012_positions.sql` — table with (service_line_id, lower(name)) unique index
- `backend/db/migrations/00013_leave_types.sql` — table with name+code unique indexes, default_annual_quota, is_annual, requires_document
- `backend/db/migrations/00014_attendance_codes.sql` — table with code+label unique indexes, 4 boolean flags
- `backend/db/migrations/00015_overtime_rules.sql` — table with nullable service_line_id, min_minutes>=30 check
- `backend/db/queries/org/*.sql` — 7 query files; 42 total sqlc-annotated queries
- `backend/internal/repository/sqlc/*.sql.go` — 7 new generated files + updated querier.go/models.go

## Decisions Made

- `geo_lat`/`geo_lng` stored as nullable `double precision` (not a composite Postgres type); `geofence_active` is derived at DTO boundary — `geo_lat IS NOT NULL AND geo_lng IS NOT NULL`
- `ListClientCompanies` accepts `service_line` and `has_leader` narg params but applies `(IS NULL OR TRUE)` — no placements/assignments table in Phase 3; these are stubbed at the repository layer
- Primary site sort: `ORDER BY is_primary DESC, created_at DESC, id DESC` (primary site always first per spec, with keyset cursor on the sub-sort)
- `ids.go` was NOT modified — all 7 prefixes (CMP/SITE/SVC/POS/LT/AC/OTR) already existed in the file

## Deviations from Plan

None — plan executed exactly as written. `make gen` succeeded on first run with no type-cast workarounds needed (sqlc correctly inferred all types from the migration schema).

## Issues Encountered

None.

## Next Phase Readiness

- Wave-2 plans (03-02 client companies + sites, 03-03 service lines + positions, 03-04 master data) can now import sqlcgen and call all Querier methods listed above
- Migration files are ready for `go run ./cmd/migrate up` — no FK ordering issues (service_lines before positions, client_companies before client_sites, overtime_rules refs service_lines)

---
*Phase: 03-e2-org-master-data*
*Completed: 2026-06-04*
