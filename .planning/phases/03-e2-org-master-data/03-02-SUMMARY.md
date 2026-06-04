---
phase: 03-e2-org-master-data
plan: 02
subsystem: backend-org
tags: [go, chi, rest, client-companies, sites, geofence, audit, seed]

# Dependency graph
requires:
  - phase: 03-e2-org-master-data
    plan: 01
    provides: sqlc generated methods (ListClientCompanies, GetClientCompanyByID, CreateClientCompany, UpdateClientCompany, SetClientCompanyStatus, CountActiveSitesForCompany, CreateSite, GetSiteByID, ListSitesForCompany, UpdateSite, DemoteOtherPrimaries, SetSitePrimary, SetSiteStatus) under internal/repository/sqlc/

provides:
  - 8 HTTP endpoints for client companies + sites (GET/POST /client-companies, GET/PATCH /client-companies/{id}, :deactivate/:reactivate, GET/POST /client-companies/{id}/sites, GET/PATCH /sites/{id}, :deactivate)
  - domain.ClientCompany + domain.Site types + filters
  - CompanyRepository port (service/org) + Repository (repository/org) implementation
  - orghttp.Handler mounted in server.go under /api/v1
  - seed: Plaza Senayan SWP-CMP-0021 + primary site SWP-SITE-0001; Mall Kelapa Gading SWP-CMP-0022 + site SWP-SITE-0002

affects:
  - 03-03-service-lines-positions (append routes after ORG slice in server.go; add OrgServiceLines Deps field)
  - 03-04-master-data (same pattern as 03-03)
  - Phase 5 (placements): wire HasLeader + ActivePlacementCount TODOs, COMPANY_HAS_ACTIVE_PLACEMENTS + SITE_HAS_ACTIVE_PLACEMENTS guards

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "org-slice pattern: domain/org.go → repository/org (pool+sqlcgen) → service/org (CompanyRepository port, InTx+audit) → handler/org (chi, httpx.PageResponse) → server.go r.Group{} under authenticated group"
    - "geofence_active derived at DTO boundary: GeoLat != nil && GeoLng != nil (never stored)"
    - "DemoteOtherPrimaries + CreateSite/SetSitePrimary in single tx (INV-5)"
    - "GEOFENCE_RADIUS_INVALID is apperr.Error with HTTPStatus=400 (not 422) — matches spec"
    - "Postgres unique violation detected by error string substring (23505 / duplicate key)"

key-files:
  created:
    - backend/internal/domain/org.go
    - backend/internal/repository/org/companies_repo.go
    - backend/internal/service/org/companies_service.go
    - backend/internal/handler/org/companies_dto.go
    - backend/internal/handler/org/companies_handler.go
  modified:
    - backend/internal/server/server.go (OrgCompanies Deps field + ORG route groups)
    - backend/cmd/api/main.go (orgCompaniesRepo/Svc/Handler wiring)
    - backend/cmd/seed/seed.go (seedClientCompanies: Plaza Senayan SWP-CMP-0021 + Mall Kelapa Gading SWP-CMP-0022)

key-decisions:
  - "OrgCompanies handler package = internal/handler/org; Deps field name = OrgCompanies (*orghttp.Handler). 03-03 should add OrgServiceLines *orgservicehttp.Handler (or similar); 03-04 OrgMasterData — each sibling owns its own Deps field and its own r.Group{} block in server.go"
  - "ORG route groups are placed AFTER the foundations group (line ~94); siblings append AFTER the ORG slice closing brace per the COORDINATION POINT comment in server.go"
  - "DemoteOtherPrimaries passes empty string as exceptID before CreateSite (site ID not yet allocated); this clears ALL primaries then CreateSite sets is_primary=true — safe because the tx is atomic"
  - "GEOFENCE_RADIUS_INVALID HTTPStatus=400 (spec shows 400); apperr uses struct literal with explicit HTTPStatus to bypass the default statusForCode fallback (which would give 422)"
  - "seedClientCompanies uses explicit IDs (SWP-CMP-0021/0022, SWP-SITE-0001/0002) via direct INSERT with ON CONFLICT (id) DO NOTHING — not swp_next_id() — so E2E tests can reference deterministic IDs"
  - "HasLeader=false + ActivePlacementCount=0 hardcoded Phase-3 stubs at repository layer; TODO(Phase-5) comments on all occurrences"
  - "CC-5 active-placement guard stubbed as count=0/no-op with TODO(Phase-5); same for ST-6 site guards"
  - "Partial update in PATCH handler: carries forward current field values when request fields are omitted (nil pointers)"

# COORDINATION CONTRACT for sibling wave-2 plans (03-03, 03-04)

## server.go wiring pattern

```go
// Deps struct — add a new field per sibling:
OrgServiceLines *orgsvclinehttp.Handler  // 03-03
OrgMasterData   *orgmdatahttp.Handler    // 03-04

// Route group — append immediately after the comment:
// "03-03 sibling: append r.Group{} here."
r.Group(func(r chi.Router) {
    r.Use(rbac.RequireRole(...))
    // ... sibling routes
})
```

## cmd/api/main.go wiring pattern

```go
// After the org companies block (orgCompaniesRepo/Svc/Handler):
orgServiceLinesRepo := orgsvclinerepo.New(pool)
orgServiceLinesSvc  := orgsvclinesvc.NewService(orgServiceLinesRepo, txm)
orgServiceLinesHandler := orgsvclinehttp.NewHandler(orgServiceLinesSvc)
// Add to server.Deps literal: OrgServiceLines: orgServiceLinesHandler
```

## cmd/seed/seed.go extension pattern

```go
// After seedClientCompanies() call in Seed():
if err := seedServiceLines(ctx, pool); err != nil { ... }  // 03-03
if err := seedMasterData(ctx, pool); err != nil { ... }    // 03-04
```

requirements-completed: [ORG-01, ORG-02]

# Metrics
duration: 8min
completed: 2026-06-04
---

# Phase 03 Plan 02: Client Companies + Sites Slice — Summary

**Client companies + sites backend slice: domain types, sqlc-backed repository, service (CC/ST business rules, geofence validation, auto-primary Main Site, atomic primary reassignment, audit), chi handlers, 8 routes in server.go, deps wired in cmd/api, seed inserts Plaza Senayan SWP-CMP-0021 + primary site. `go build + go vet` clean.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-06-04T03:06:00Z
- **Completed:** 2026-06-04T03:13:32Z
- **Tasks:** 3
- **Files created/modified:** 8

## Accomplishments

- Created domain types `ClientCompany` and `Site` with filters, matching the OpenAPI schema field-for-field
- Implemented `CompanyRepository` port (service/org) and `Repository` (repository/org) over the Phase-01 sqlc methods; SiteCount wired to CountActiveSitesForCompany; Phase-5 stubs (HasLeader, ActivePlacementCount) with TODO comments
- Service implements CC-1..CC-5 and ST-1..ST-8 rules: GEOFENCE_RADIUS_INVALID (400), auto Main Site on company create (CC-1c), atomic DemoteOtherPrimaries + SetSitePrimary (INV-5), audit.Record on every write, CC-5 + ST-6 stubbed with TODO(Phase-5)
- Handler: 11 methods (ListClientCompanies, GetClientCompany, CreateClientCompany, UpdateClientCompany, DeactivateClientCompany, ReactivateClientCompany, ListSites, GetSite, CreateSite, UpdateSite, DeactivateSite); geo nested object; geofence_active server-derived; status UPPERCASED; 201+Location on creates; cursor pagination
- server.go: OrgCompanies Deps field; dedicated ORG route group with reads (all three roles) and writes (hr_admin/super_admin); coordination point comments for siblings 03-03 and 03-04
- seed.go: idempotent Plaza Senayan SWP-CMP-0021 + Mall Kelapa Gading SWP-CMP-0022 with primary sites; shift_leader persona FK now resolves

## Task Commits

1. **Task 1+2: Domain types + repo + service + handlers + DTOs** - `aec74dc` + `1cb1c3e`
2. **Task 3: Routes + deps + seed** - `f76340a`

## Files Created / Modified

### Created
- `backend/internal/domain/org.go` — ClientCompany, Site, CompanyFilter, SiteFilter structs
- `backend/internal/repository/org/companies_repo.go` — sqlc-backed CompanyRepository; SiteCount from CountActiveSitesForCompany
- `backend/internal/service/org/companies_service.go` — CompanyRepository port + Service with CC/ST business rules
- `backend/internal/handler/org/companies_dto.go` — request/response structs; geo nested object; geofence_active derived; status UPPERCASED
- `backend/internal/handler/org/companies_handler.go` — Handler{svc} with 11 handler methods; cursor decode; 201+Location

### Modified
- `backend/internal/server/server.go` — OrgCompanies Deps field + ORG route groups (reads + writes) + coordination comments
- `backend/cmd/api/main.go` — orgCompaniesRepo/Svc/Handler wiring into server.Deps
- `backend/cmd/seed/seed.go` — seedClientCompanies() function

## Decisions Made

- `GEOFENCE_RADIUS_INVALID` uses `apperr.Error{HTTPStatus: 400}` struct literal to bypass `statusForCode` fallback (which would default to 422)
- `DemoteOtherPrimaries` called with empty `exceptID` before `CreateSite` (new site ID not allocated yet); all primaries demoted then CreateSite with is_primary=true — safe in a single tx
- Partial update in PATCH handler carries forward current field values when request fields are nil pointers
- Seed uses explicit IDs (SWP-CMP-0021/0022, SWP-SITE-0001/0002) via direct INSERT, not swp_next_id() — deterministic IDs for E2E tests

## Deviations from Plan

None — plan executed exactly as written. Go build + vet clean on first run.

## Coordination Contract for Siblings (03-03, 03-04)

### server.go — how to append

Add a new Deps field:
```go
OrgServiceLines *orgsvclinehttp.Handler  // 03-03 adds this
```

Append a new `r.Group{}` block immediately after the comment `// ORG slice end (03-02). 03-03 sibling: append r.Group{} here.` — do NOT modify the existing ORG companies groups.

### cmd/api/main.go — how to wire

Add repo→svc→handler construction below the existing org companies block, then add the new field to `server.Deps{}`.

### cmd/seed/seed.go — how to extend

Add a call to `seedServiceLines(ctx, pool)` (and similar) after `seedClientCompanies(ctx, pool)` in the `Seed()` function.

## Self-Check

All files verified:
- `backend/internal/domain/org.go` — FOUND
- `backend/internal/repository/org/companies_repo.go` — FOUND
- `backend/internal/service/org/companies_service.go` — FOUND
- `backend/internal/handler/org/companies_dto.go` — FOUND
- `backend/internal/handler/org/companies_handler.go` — FOUND
- Commit `aec74dc` — FOUND
- Commit `1cb1c3e` — FOUND
- Commit `f76340a` — FOUND
- `go build ./... && go vet ./...` — EXIT 0

## Self-Check: PASSED
