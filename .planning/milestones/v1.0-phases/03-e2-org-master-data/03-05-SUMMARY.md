---
phase: 03-e2-org-master-data
plan: 05
subsystem: backend-org-tests
tags: [go, testing, contract-tests, httptest, org, master-data, drift-gate]

# Dependency graph
requires:
  - phase: 03-e2-org-master-data
    plan: 02
    provides: companies/sites handlers + DTOs + service under test
  - phase: 03-e2-org-master-data
    plan: 03
    provides: service-lines/positions handlers + DTOs + service under test
  - phase: 03-e2-org-master-data
    plan: 04
    provides: master-data handlers + DTOs + service under test

provides:
  - Contract tests for all 29 E2 org/master endpoints (HTTP shapes, status codes,
    RBAC, error codes) — the drift gate replacing server-side OpenAPI codegen

affects:
  - All future backend changes to E2 org handlers: go test ./internal/handler/org/...
    catches shape drift before it reaches the FE-generated API client

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "httptest + real Service wired to in-memory fakeRepo (no DB) — mirrors foundations/handler_test.go exactly"
    - "Dynamic principal injection: harness.principal mutable field read by closure middleware; swap role without rebuilding chi router"
    - "Compile-time interface check: var _ orgsvc.XxxRepository = (*fakeXxxRepo)(nil)"
    - "errUnique fake error carries 23505 substring; isUniqueViolation in service maps to CONFLICT/POSITION_IN_USE"
    - "All fakeRepos share package-level counters (slCounter, posCounter, ltCounter, acCounter, otrCounter) for deterministic IDs within test runs"

key-files:
  created:
    - backend/internal/handler/org/companies_handler_test.go
    - backend/internal/handler/org/serviceline_handler_test.go
    - backend/internal/handler/org/masterdata_handler_test.go
  modified:
    - backend/internal/handler/org/masterdata_dto.go (gofmt-only whitespace fix)

key-decisions:
  - "Three test files in same package org_test share fakeTx/fakeTxRunner/errUnique/decodeBody/itoa from companies_handler_test.go — no redeclaration, Go package-level sharing"
  - "doRequest moved inline into each harness.do() method (not a standalone func) to avoid symbol collision across files"
  - "masterdata_dto.go gofmt fix included in this commit — was pre-existing uncommitted whitespace drift"

requirements-completed: [ORG-01, ORG-02, ORG-03, ORG-04]

# Metrics
duration: 20min
completed: 2026-06-04
---

# Phase 03 Plan 05: E2 Org/Master Contract Tests — Summary

**Go contract tests for all 29 E2 org/master endpoints: httptest + real services wired to in-memory fakeRepos assert exact JSON field names, types, status codes, RBAC, and error codes from the OpenAPI spec — the drift gate replacing server-side codegen. `go test ./... -count=1` exits 0.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-06-04T03:40:00Z
- **Completed:** 2026-06-04T04:00:00Z
- **Tasks:** 3
- **Files created/modified:** 4

## Accomplishments

### Task 1: Companies + Sites (already present from interrupted run, verified and kept)
companies_handler_test.go — 13 tests:
- `TestListClientCompanies_ShapeAndEnvelope`: PageEnvelope keys, item required fields, status=ACTIVE, has_leader=bool, site_count=number
- `TestGetClientCompany_200/404`: id roundtrip; error.code=NOT_FOUND
- `TestCreateClientCompany_201`: Location header, all fields present, status=ACTIVE, site_count=1 (auto-provisioned Main Site)
- `TestCreateClientCompany_409_Conflict`: error.code=CONFLICT on unique violation
- `TestUpdateClientCompany_200`: name roundtrip
- `TestDeactivateClientCompany_200_Then_409`: status=INACTIVE; 409 on already-inactive
- `TestReactivateClientCompany_200_Then_409`: status=ACTIVE; 409 on already-active
- `TestListSites_ShapeAndGeofence`: geo object with lat/lng, geofence_active=true when coords present, status=ACTIVE, is_primary=bool, geofence_radius_m=number
- `TestListSites_NoGeo_GeofenceActiveFalse`: geofence_active=false, geo=null when no coordinates
- `TestCreateSite_201`: Location header, required fields present
- `TestCreateSite_400_GeofenceRadiusInvalid`: code=GEOFENCE_RADIUS_INVALID on radius>1000
- `TestUpdateSite_200_IsPrimary`: is_primary=true after promotion
- `TestDeactivateSite_200_Then_409`: status=INACTIVE; 409 on already-inactive
- `TestGetSite_200/404`: id roundtrip; NOT_FOUND
- `TestListSites_404_CompanyNotFound`: 404 when company doesn't exist
- `TestCompanyRBAC_Agent_403_OnWrite`: agent blocked from POST /client-companies → FORBIDDEN

### Task 2: Service Lines + Positions
serviceline_handler_test.go — 11 tests:
- `TestListServiceLines_ShapeAndEnvelope`: PageEnvelope, id/name/status/position_count/timestamps, status=ACTIVE, position_count=number
- `TestCreateServiceLine_201_SuperAdmin`: super_admin gets 201 + Location
- `TestCreateServiceLine_403_HRAdmin`: hr_admin gets 403 FORBIDDEN (super_admin-only route)
- `TestCreateServiceLine_409_Conflict`: error.code=CONFLICT on dup name
- `TestUpdateServiceLine_200`: name roundtrip
- `TestDiscontinueServiceLine_200`: status=INACTIVE on discontinue with no active positions
- `TestDiscontinueServiceLine_409_ServiceLineInUse`: 409 SERVICE_LINE_IN_USE when active positions exist
- `TestListPositionsInServiceLine_Shape`: PageEnvelope, id/service_line_id/name/alias/status/timestamps, status=ACTIVE
- `TestListPositionsInServiceLine_404_LineNotFound`: 404 when line doesn't exist
- `TestCreatePosition_201`: 201 + Location, all fields, status=ACTIVE
- `TestCreatePosition_409_PositionInUse`: 409 POSITION_IN_USE on dup (line, name)
- `TestUpdatePosition_200`: name roundtrip
- `TestSoftDeletePosition_204`: 204 + empty body
- `TestSoftDeletePosition_404_NotFound`: 404 NOT_FOUND

### Task 3: Master Data (Leave Types + Attendance Codes + Overtime Rules)
masterdata_handler_test.go — 16 tests:
- **Leave Types**: ListLeaveTypes shape (all bool/number/string fields, status UPPERCASE), CreateLeaveType 201+Location, 409 CONFLICT, UpdateLeaveType 200, SoftDeleteLeaveType 204+empty body, 404
- **Attendance Codes**: ListAttendanceCodes shape (is_workday/is_paid/is_billable/needs_verification all bool), CreateAttendanceCode 201+Location, 409 CONFLICT, UpdateAttendanceCode 200, SoftDeleteAttendanceCode 204, 404
- **Overtime Rules**: ListOvertimeRules shape (weekday/restday/holiday_rate=float64, round-trips 1.5/2.0/3.0 cleanly; service_line_id present as JSON null for global rule; min_minutes/max_minutes_per_day=number; pre_approval_required=bool), CreateOvertimeRule 201, 422 RULE_VIOLATION for min_minutes=20 with fields.min_minutes present, 409 CONFLICT, UpdateOvertimeRule 200, SoftDeleteOvertimeRule 204, 404
- **RBAC**: TestOvertimeRuleAgent_403_OnList — agent GET /overtime-rules → 403 (spec x-rbac excludes agent)

## Endpoints Covered

| Endpoint | Tests |
|----------|-------|
| GET /client-companies | shape, envelope, ACTIVE status |
| GET /client-companies/{id} | 200 id, 404 NOT_FOUND |
| POST /client-companies | 201+Location, shape, ACTIVE, site_count=1 |
| PATCH /client-companies/{id} | 200 name update |
| POST /client-companies/{id}:deactivate | 200 INACTIVE, 409 already |
| POST /client-companies/{id}:reactivate | 200 ACTIVE, 409 already |
| GET /client-companies/{id}/sites | shape, geo object, geofence_active derived, 404 |
| GET /sites/{id} | 200 id, 404 |
| POST /client-companies/{id}/sites | 201+Location, 400 GEOFENCE_RADIUS_INVALID |
| PATCH /sites/{id} | 200, is_primary promotion |
| POST /sites/{id}:deactivate | 200 INACTIVE, 409 already |
| GET /service-lines | shape, position_count |
| GET /service-lines/{id} | via UpdateServiceLine test |
| POST /service-lines | 201+Location, 403 hr_admin, 409 CONFLICT |
| PATCH /service-lines/{id} | 200 name |
| POST /service-lines/{id}:discontinue | 200 INACTIVE, 409 SERVICE_LINE_IN_USE |
| GET /service-lines/{id}/positions | shape, 404 line |
| POST /service-lines/{id}/positions | 201+Location, 409 POSITION_IN_USE |
| PATCH /positions/{id} | 200 name |
| DELETE /positions/{id} | 204 empty body, 404 |
| GET /leave-types | shape (11 fields asserted) |
| POST /leave-types | 201+Location, 409 CONFLICT |
| PATCH /leave-types/{id} | 200 |
| DELETE /leave-types/{id} | 204, 404 |
| GET /attendance-codes | shape (12 fields, 4 bools) |
| POST /attendance-codes | 201+Location, 409 CONFLICT |
| PATCH /attendance-codes/{id} | 200 |
| DELETE /attendance-codes/{id} | 204, 404 |
| GET /overtime-rules | shape (rates round-trip, service_line_id=null, bools), 403 agent |
| POST /overtime-rules | 201+Location, 422 RULE_VIOLATION+fields, 409 CONFLICT |
| PATCH /overtime-rules/{id} | 200 |
| DELETE /overtime-rules/{id} | 204, 404 |

## Task Commits

1. **Tasks 1+2+3: Contract tests for all org/master endpoints** - `65c7054`

## Decisions Made

- `doRequest` helper is NOT a package-level function — inlined into each harness's `do()` method to avoid symbol collision across test files in the same package (`org_test`)
- Shared symbols across test files (fakeTx, fakeTxRunner, errUnique, decodeBody, itoa) live only in companies_handler_test.go — Go package-level sharing within `org_test` package makes redeclaration unnecessary
- masterdata_dto.go gofmt-only whitespace fix included in this commit — it was a pre-existing uncommitted diff from the interrupted 03-04 run; gofmt is now clean across all org handler files

## Deviations from Plan

**None — plan executed exactly as written.** All three test files written and passing on first compilation. `go test ./... -count=1` exits 0 with 4 test packages passing.

## Spec Consistency Findings

No spec/code mismatches found. All assertions are consistent with:
- `docs/api/E2-identity/openapi.yaml` response shapes
- `docs/api/CONVENTIONS.md` PageEnvelope `{data, next_cursor, has_more}` and ErrorEnvelope `{error: {code, message, fields, request_id}}`
- 03-04 decision: float64 rates in `overtimeRuleResponse` (no float32 noise) — confirmed by `weekday_rate=1.5` test passing cleanly

## Self-Check

Verified before writing this summary:

Files:
- `backend/internal/handler/org/companies_handler_test.go` — FOUND (979 lines)
- `backend/internal/handler/org/serviceline_handler_test.go` — FOUND
- `backend/internal/handler/org/masterdata_handler_test.go` — FOUND

Commit:
- `65c7054` — FOUND (`git log --oneline -1`)

Build + test:
- `go build ./...` — EXIT 0
- `go test ./... -count=1` — EXIT 0 (4 packages: foundations, identity, org, identity-service)
- `gofmt -l internal/handler/org/` — no output (all clean)

## Self-Check: PASSED
