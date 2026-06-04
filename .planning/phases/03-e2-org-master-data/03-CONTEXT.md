# Phase 3: E2 Org & Master Data - Context

**Gathered:** 2026-06-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the FE-used E2 organization + master-data endpoints against the real BE and wire
the E2 org/master-data screens off MSW, proven with exhaustive Playwright E2E. Scope:
client companies + sites (with geofence), service lines + positions, and the master-data
sets (leave types, attendance codes, overtime rules). Employees/agreements/change-requests
are Phase 4 (out of scope here). All-new schema for these entities.
</domain>

<decisions>
## Implementation Decisions

### Scope (exact endpoints — FE-used only; see fe-endpoint-inventory.md E2 "Org & master data")
- Client companies: `GET /client-companies`, `GET /client-companies/{id}`, `POST /client-companies`, `PATCH /client-companies/{id}`, `POST /client-companies/{id}:reactivate`. (FE also calls deactivate? inventory lists reactivate only — implement deactivate too if the detail screen uses it; otherwise follow the inventory. Confirm against the FE.)
- Sites: `GET /client-companies/{id}/sites`, `POST /client-companies/{id}/sites`, `PATCH /sites/{site_id}` (with geofence lat/lng/radius_m).
- Service lines: `GET /service-lines`, `GET /service-lines/{id}`, `POST /service-lines`, `PATCH /service-lines/{id}`, `POST /service-lines/{id}:discontinue`.
- Positions: `GET /service-lines/{id}/positions`, `POST /service-lines/{id}/positions`, `PATCH /positions/{id}`, `DELETE /positions/{id}` (soft-delete).
- Master data: `GET/POST /leave-types`, `PATCH /leave-types/{id}`, `DELETE /leave-types/{id}`; same for `/attendance-codes` and `/overtime-rules`.

### Build approach
- Follow `.planning/reference/backend-build-conventions.md`; copy the identity/foundations slice shape. Hand-written chi handlers; sqlc queries (`make gen`); match `docs/api/E2-identity/openapi.yaml` EXACTLY (FE generated from it).
- New migrations for: `client_companies`, `client_sites` (FK company, geofence cols lat/lng/geofence_radius_m), `service_lines`, `positions` (FK service_line), `leave_types`, `attendance_codes`, `overtime_rules`. Soft-delete (`deleted_at`) + SWP IDs via `swp_next_id` with the right prefixes (CMP, SITE, SVC, POS, LT, AC, OTR) — these already exist in `internal/platform/ids/ids.go`.
- RBAC per spec x-rbac (super_admin/hr_admin for writes; reads may be broader). Audit every write (CONVENTIONS §16.1). Cursor pagination for list endpoints + the spec's filters; picker-shaped lists where the spec/FE expects (CONVENTIONS §18 — e.g. `?service_line=`, `?q=`).
- Status mapping active/discontinued/etc. per spec (UPPERCASE in API where the spec uses it). `:discontinue` / `:deactivate` / `:reactivate` are action endpoints; DELETE = soft-delete.
- Extend `backend/cmd/seed` with companies (incl. "Plaza Senayan" SWP-CMP-0021 referenced by the shift_leader persona), a site, service lines + positions, and master-data rows so the screens + later phases have data.

### E2E coverage (exhaustive)
- One `test()` per Gherkin scenario/case in the E2 PRDs: client-company-directory.md, client-sites-geofence.md, service-lines-positions.md, operational-master-data.md. Cover CRUD + reactivate/discontinue/soft-delete + RBAC negative + validation (422 field errors) + cursor pagination. Run green against the real stack; each test named by scenario/BR-#/C-#.

### Claude's Discretion
- Plan split (suggest: companies+sites / service-lines+positions / master-data(3 sets) / contract tests / FE+E2E). Whether to fold contract tests into each slice or one plan.
- Geofence validation specifics (radius bounds) per the PRD.
</decisions>

<canonical_refs>
## Canonical References

### Scope & rules
- `.planning/reference/fe-endpoint-inventory.md` (E2 "Org & master data")
- `.planning/reference/backend-build-conventions.md`, `.planning/reference/e2e-harness-spec.md`

### Contract & behavior
- `docs/api/E2-identity/openapi.yaml` — client-companies (1202+), sites (1483+,1609+), service-lines (1704+), positions (1878+,1972+), leave-types (2040+), attendance-codes (2205+), overtime-rules (2361+). Match request/response/x-rbac exactly.
- `docs/api/CONVENTIONS.md` §4 IDs, §5 naming, §7 status, §8 cursor, §9 filtering, §11 errors, §16.1 audit, §17 RBAC, §18 pickers.
- `docs/epics/E2-identity/prds/{client-company-directory,client-sites-geofence,service-lines-positions,operational-master-data}.md` — Gherkin AC (E2E source) + BR-#/C-#.
- `docs/epics/E2-identity/FEATURE.md`, `docs/epics/E2-identity/DATA-MAPPING.md` — invariants + legacy mapping.

### Reference implementation
- `backend/internal/{handler,service,repository,domain}/foundations` + `.../identity` (slice shape), `backend/internal/platform/*`, `backend/db/queries/foundations/*`, `backend/db/migrations/00008_*`.
- `backend/cmd/seed/seed.go`. FE screens in `frontend/apps/web/src/features/e2-identity/*` + `pickers/`; hooks in `frontend/packages/api-client/src/e2.ts`. E2E patterns in `frontend/e2e/`.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Platform kernel (httpx cursor/PageResponse, rbac.RequireRole + scope guards, audit.Record, apperr, ids with CMP/SITE/SVC/POS/LT/AC/OTR prefixes, idempotency, db.TxManager). Two reference slices (identity, foundations). E2E harness boots real stack + resetDb + loginAs.

### Established Patterns
- migration (goose) → sqlc queries (`make gen`) → repository (domain mapping, tx writes) → service (apperr codes, audit) → hand-written handler → routes in server.go under RequireRole → Go contract tests → FE wiring + live E2E.

### Integration Points
- New query dir `backend/db/queries/identity/` or a new `backend/db/queries/org/` (sqlc glob `db/queries/*` picks up subdirs). Routes in server.go authenticated group. Seed extension. FE screens already exist (built from .pen) and mostly call hooks via MSW — wire to real BE.
</code_context>

<specifics>
## Specific Ideas
- Seed must include client company "Plaza Senayan" (SWP-CMP-0021) so the Phase-1 shift_leader persona's scope resolves to a real company (also unblocks company_name display deferred in Phase 1).
</specifics>

<deferred>
## Deferred Ideas
- Employees, agreements, change-requests — Phase 4.
- Any E2 endpoint the FE doesn't call yet.
</deferred>

---

*Phase: 03-e2-org-master-data*
*Context gathered: 2026-06-04*
