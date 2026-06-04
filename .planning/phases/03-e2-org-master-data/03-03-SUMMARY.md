---
phase: 03-e2-org-master-data
plan: 03
subsystem: backend-org
tags: [go, chi, rest, service-lines, positions, audit, seed]

# Dependency graph
requires:
  - phase: 03-e2-org-master-data
    plan: 01
    provides: sqlc generated methods (ListServiceLines, GetServiceLineByID, CreateServiceLine, UpdateServiceLine, SetServiceLineStatus, CountActivePositionsForLine, ListPositionsForLine, GetPositionByID, CreatePosition, UpdatePosition, SetPositionStatus, SoftDeletePosition)
  - phase: 03-e2-org-master-data
    plan: 02
    provides: org package (mapErr/nullStr helpers in companies_repo.go; TxRunner/Clock/pageCursor/isUniqueViolation in companies_service.go; decodeJSON/parseLimit/queryStringPtr/derefString in companies_handler.go); OrgCompanies Deps field; server.go COORDINATION POINT comment

provides:
  - 9 HTTP endpoints for service lines + positions (GET/POST /service-lines, GET/PATCH /service-lines/{id}, :discontinue, GET/POST /service-lines/{id}/positions, PATCH /positions/{id}, DELETE /positions/{id})
  - domain.ServiceLine + domain.Position types + filters
  - ServiceLineRepository port (service/org) + ServiceLineRepository (repository/org) implementation
  - orghttp.ServiceLineHandler mounted in server.go alongside OrgCompanies
  - seed: SWP-SVC-001 Facility Services, SWP-SVC-002 Building Management, SWP-SVC-003 Parking; SWP-POS-014 Petugas Parkir, SWP-POS-015 Koordinator Lokasi (all Parking)

affects:
  - 03-04-master-data (append routes after 03-03 coordination point; add OrgMasterData Deps field)
  - Phase 5 (placements): wire SERVICE_LINE_IN_USE placement guard (TODO Phase-5 in DiscontinueServiceLine) and POSITION_IN_USE placement guard (TODO Phase-5 in SoftDeletePosition)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "org-slice pattern (identical to 03-02): domain/org_serviceline.go → repository/org (ServiceLineRepository) → service/org (ServiceLineService, separate from Service) → handler/org (ServiceLineHandler) → server.go r.Group{} blocks"
    - "package-level reuse: mapErr/nullStr (companies_repo.go), TxRunner/Clock/pageCursor/isUniqueViolation (companies_service.go), decodeJSON/parseLimit/queryStringPtr/derefString (companies_handler.go) — no redeclaration"
    - "SoftDeletePosition returns 204 No Content (w.WriteHeader(http.StatusNoContent))"
    - "SERVICE_LINE_IN_USE: apperr.Conflict when CountActivePositionsForLine > 0; TODO(Phase-5) for placements"
    - "POSITION_IN_USE: mapPosConflict wraps unique-index violations (23505/duplicate key); TODO(Phase-5) for placement guard"
    - "Status UPPERCASE only at DTO boundary (strings.ToUpper); DB stores lowercase"
    - "Seed uses explicit IDs (SWP-SVC-001/002/003, SWP-POS-014/015) with ON CONFLICT (id) DO NOTHING for deterministic E2E"

key-files:
  created:
    - backend/internal/domain/org_serviceline.go
    - backend/internal/repository/org/serviceline_repo.go
    - backend/internal/service/org/serviceline_service.go
    - backend/internal/handler/org/serviceline_dto.go
    - backend/internal/handler/org/serviceline_handler.go
  modified:
    - backend/internal/server/server.go (OrgServiceLines Deps field + 3 route groups + coordination comment for 03-04)
    - backend/cmd/api/main.go (orgServiceLinesRepo/Svc/Handler wiring)
    - backend/cmd/seed/seed.go (seedServiceLines: 3 lines + 2 positions)

key-decisions:
  - "ServiceLineService is a SEPARATE struct from Service (companies_service.go) in the same org package — parallel-merge clean; no struct extension"
  - "ServiceLineHandler is in the same orghttp package as Handler — OrgServiceLines Deps field type = *orghttp.ServiceLineHandler (no new import alias)"
  - "DiscontinueServiceLine: SERVICE_LINE_IN_USE when CountActivePositionsForLine > 0; also blocks inactive service line from re-discontinuing (ErrNotFound if soft-deleted)"
  - "SoftDeletePosition calls repo.SoftDeletePosition (sets deleted_at) not SetPositionStatus — matches spec 'hard soft-delete' per 03-01 decision"
  - "UpdatePosition partial update: carries forward current Name/Alias when request fields are empty"
  - "Seed uses explicit IDs not swp_next_id() — SWP-SVC-001/002/003, SWP-POS-014/015 match OpenAPI spec examples exactly"

requirements-completed: [ORG-03]

# Metrics
duration: 6min
completed: 2026-06-04
---

# Phase 03 Plan 03: Service Lines + Positions Slice — Summary

**Service-lines + positions backend slice (ORG-03): domain types, sqlc-backed repository, service (SP-1 discontinue with in-use guard, SP-3 dup detection, SP-4 soft-delete, audit on every write), chi handlers (9 endpoints), 3 route groups in server.go with correct RBAC (SL writes super_admin-only, position writes super_admin+hr_admin, reads all roles), seed with 3 canonical service lines + 2 Parking positions. `go build + go vet` clean.**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-06-04T03:16:59Z
- **Completed:** 2026-06-04T03:22:27Z
- **Tasks:** 3
- **Files created/modified:** 8

## Accomplishments

- Created `domain/org_serviceline.go` with `ServiceLine`, `Position`, `ServiceLineFilter`, `PositionFilter` types matching OpenAPI schemas field-for-field
- Implemented `ServiceLineRepository` (repository port in service package) and `ServiceLineRepository` (sqlc-backed impl in repository/org); PositionCount wired to `CountActivePositionsForLine`; reused `mapErr`/`nullStr` helpers from `companies_repo.go` — no redeclaration
- Service implements SP-1..SP-4 rules: `DiscontinueServiceLine` → `SERVICE_LINE_IN_USE` (409) when active positions exist (TODO Phase-5 for placements); `CreatePosition`/`UpdatePosition` → `POSITION_IN_USE` (409) on unique (line, name) violation; `SoftDeletePosition` — calls `SoftDeletePosition` (hard soft-delete via `deleted_at`), TODO Phase-5 placement guard; audit.Record on every write; ClampLimit+1 cursor pagination; isUniqueViolation reused from package
- Handler: `ServiceLineHandler` with 9 methods; `SoftDeletePosition` returns 204; CreateServiceLine/CreatePosition return 201 + Location; status UPPERCASED; cursor decode
- server.go: `OrgServiceLines *orghttp.ServiceLineHandler` Deps field; 3 route groups (reads all roles, SL writes super_admin, position writes super_admin+hr_admin); 9 endpoints including `:discontinue` action suffix; coordination comment for 03-04
- seed.go: idempotent `seedServiceLines()` — SWP-SVC-001 Facility Services + SWP-SVC-002 Building Management + SWP-SVC-003 Parking; SWP-POS-014 Petugas Parkir (Parking Attendant) + SWP-POS-015 Koordinator Lokasi (Parking Supervisor); explicit IDs match OpenAPI spec examples

## Task Commits

1. **Task 1: Domain + repository** - `dfdae9c`
2. **Task 2: Service + handlers/DTOs** - `9e6e489`
3. **Task 3: Routes + deps + seed** - `ba7eb62`

## Files Created / Modified

### Created
- `backend/internal/domain/org_serviceline.go` — ServiceLine, Position, ServiceLineFilter, PositionFilter structs
- `backend/internal/repository/org/serviceline_repo.go` — sqlc-backed ServiceLineRepository; PositionCount from CountActivePositionsForLine; toPosition() mapper helper; compile-time interface check
- `backend/internal/service/org/serviceline_service.go` — ServiceLineService + ServiceLineRepository port; SP-1..4 business rules; audit on every write
- `backend/internal/handler/org/serviceline_dto.go` — serviceLineResponse/positionResponse (status UPPERCASE); position_count in service line DTO
- `backend/internal/handler/org/serviceline_handler.go` — ServiceLineHandler with 9 methods; 204 for soft-delete; 201+Location for creates

### Modified
- `backend/internal/server/server.go` — OrgServiceLines Deps field + 3 ORG route groups for service lines/positions + coordination comment for 03-04
- `backend/cmd/api/main.go` — orgServiceLinesRepo/Svc/Handler wiring into server.Deps
- `backend/cmd/seed/seed.go` — seedServiceLines() with 3 service lines + 2 Parking positions (explicit IDs)

## Mounted Routes

| Method | Path | RBAC | Handler |
|--------|------|------|---------|
| GET | /api/v1/service-lines | all roles | ListServiceLines |
| POST | /api/v1/service-lines | super_admin | CreateServiceLine |
| GET | /api/v1/service-lines/{service_line_id} | all roles | GetServiceLine |
| PATCH | /api/v1/service-lines/{service_line_id} | super_admin | UpdateServiceLine |
| POST | /api/v1/service-lines/{service_line_id}:discontinue | super_admin | DiscontinueServiceLine |
| GET | /api/v1/service-lines/{service_line_id}/positions | all roles | ListPositionsInServiceLine |
| POST | /api/v1/service-lines/{service_line_id}/positions | super_admin,hr_admin | CreatePosition |
| PATCH | /api/v1/positions/{position_id} | super_admin,hr_admin | UpdatePosition |
| DELETE | /api/v1/positions/{position_id} | super_admin,hr_admin | SoftDeletePosition → 204 |

## Seeded IDs (deterministic for E2E reference)

| ID | Entity | Name | Alias |
|----|--------|------|-------|
| SWP-SVC-001 | service_line | Facility Services | — |
| SWP-SVC-002 | service_line | Building Management | — |
| SWP-SVC-003 | service_line | Parking | — |
| SWP-POS-014 | position (SWP-SVC-003) | Petugas Parkir | Parking Attendant |
| SWP-POS-015 | position (SWP-SVC-003) | Koordinator Lokasi | Parking Supervisor |

## Decisions Made

- `ServiceLineService` is a separate struct from `Service` (03-02's companies service) in the same `org` package — parallel-merge clean, no struct coupling
- `ServiceLineHandler` lives in the same `orghttp` package — `OrgServiceLines` Deps field uses `*orghttp.ServiceLineHandler`, no new import alias needed
- `DiscontinueServiceLine` checks `CountActivePositionsForLine > 0` → `SERVICE_LINE_IN_USE` before changing status; TODO(Phase-5) adds placement check
- `SoftDeletePosition` uses `repo.SoftDeletePosition` (sets `deleted_at`) not `SetPositionStatus` — matches 03-01's "hard soft-delete" design for the DELETE endpoint
- Seed uses explicit IDs not `swp_next_id()` — matches OpenAPI spec examples exactly for deterministic E2E test references

## Deviations from Plan

None — plan executed exactly as written. Go build + vet clean on first run.

## COORDINATION CONTRACT for 03-04 sibling

### server.go — how to append

Add a new Deps field:
```go
OrgMasterData *orgmdatahttp.Handler  // 03-04 adds this
```

Append a new `r.Group{}` block immediately after the comment:
`// ORG slice end (03-03). 03-04 sibling: append r.Group{} here.`
Do NOT modify the existing ORG service-line groups.

### cmd/api/main.go — how to wire

Add repo→svc→handler construction below the existing service-lines block, then add the new field to `server.Deps{}`.

### cmd/seed/seed.go — how to extend

Add a call to `seedMasterData(ctx, pool)` after `seedServiceLines(ctx, pool)` in the `Seed()` function.

## Self-Check

All files verified:
- `backend/internal/domain/org_serviceline.go` — FOUND
- `backend/internal/repository/org/serviceline_repo.go` — FOUND
- `backend/internal/service/org/serviceline_service.go` — FOUND
- `backend/internal/handler/org/serviceline_dto.go` — FOUND
- `backend/internal/handler/org/serviceline_handler.go` — FOUND
- Commit `dfdae9c` — FOUND
- Commit `9e6e489` — FOUND
- Commit `ba7eb62` — FOUND
- `go build ./... && go vet ./...` — EXIT 0

## Self-Check: PASSED
