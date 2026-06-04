# Phase 4: E2 People - Context

**Gathered:** 2026-06-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the FE-used E2 "people" endpoints against the real BE and wire the screens off
MSW, proven with exhaustive Playwright E2E: employees (CRUD + deactivate/reactivate),
employment agreements (CRUD + renew/close + multipart attachment upload), and the
change-request approval queue (list/detail/approve/reject — the HR-side review of
agent-submitted edits). Org & master data (Phase 3) is done and reused (employees reference
positions/companies; agreements reference employees). Placement/scheduling are later phases.
</domain>

<decisions>
## Implementation Decisions

### Scope (FE-used only — fe-endpoint-inventory.md E2 "People")
- Employees: `GET /employees`, `GET /employees/{id}`, `POST /employees`, `PATCH /employees/{id}`, `POST /employees/{id}:deactivate`, `:reactivate`. (NOT `:bulk-deactivate`, NOT the per-employee change-request submit endpoint — defer.)
- Agreements: `GET /agreements`, `GET /agreements/{id}`, `POST /agreements`, `POST /agreements/{id}:renew`, `:close`, `POST /agreements/{id}/attachments` (multipart).
- Change requests (HR queue): `GET /change-requests`, `GET /change-requests/{id}`, `POST /change-requests/{id}:approve`, `:reject`.

### Build approach
- Follow `.planning/reference/backend-build-conventions.md`; mirror the org slice (Phase 3) shape. Hand-written chi handlers; sqlc (`make gen`); match `docs/api/E2-identity/openapi.yaml` EXACTLY.
- New migrations: `employees`, `employment_agreements`, `agreement_attachments` (file metadata), `change_requests`. Soft-delete + SWP ids (EMP, AG, CHG; attachment file id SWP-FILE — add `File` prefix to ids.go if needed). FKs: employee→position/company where the spec models them; agreement→employee.
- RBAC per spec x-rbac (employees/agreements = hr_admin/super_admin writes; change-request approve = hr_admin). Audit every write. Cursor pagination + filters for lists.
- **File uploads (CONVENTIONS §15):** separate multipart route `POST /agreements/{id}/attachments` (`file` + `caption`), max 10MB, allowed types per spec (PDF/JPEG/PNG). Storage is opaque to the client — simplest working approach: store the blob on local disk (configurable dir) OR a bytea column, with metadata row; serve via an authenticated `GET /files/{SWP-FILE-id}` that requires the same auth. Response returns `{id,url,name,size_bytes,mime,uploaded_at}` per §15. Document the storage choice.
- **Agreement period rules (PKWT/PKWTT):** enforce cross-field rules from the PRD (e.g., PKWT end_date−start_date ≤ 5 years → 422 `PKWT_PERIOD_EXCEEDS_MAX`); end<start → 400. Per CONVENTIONS §12.
- **Change-request workflow:** approve applies the proposed change to the target entity (or marks applied) + sets status; reject sets status + reason; both audited + (later) notify. Seed a couple of pending change requests so the queue renders.
- Extend `cmd/seed` with employees (incl. the personas' employee records SWP-EMP-1042/1108/2891 already referenced), at least one agreement + attachment, and pending change-requests.

### E2E coverage (exhaustive)
- One `test()` per Gherkin scenario/case in `docs/epics/E2-identity/prds/employee-profile.md` + `employment-agreement.md` (+ change-request scenarios therein). Cover: employee CRUD + deactivate/reactivate; agreement create/renew/close + attachment upload (real file) + PKWT period validation (422); change-request approve/reject; RBAC negative. Run green against the real stack; named by scenario/BR-#/C-#.

### Claude's Discretion
- File storage mechanism (local dir vs bytea) — pick simplest that passes E2E + matches §15 response.
- Whether change-request "apply on approve" mutates the target now or is stubbed where the target field isn't modeled yet — keep status/workflow correct; note any stubs.
- Plan split (suggest: data layer / employees slice / agreements+attachments slice / change-requests slice / contract tests / FE+E2E).
</decisions>

<canonical_refs>
## Canonical References

### Scope & rules
- `.planning/reference/fe-endpoint-inventory.md` (E2 "People"), `.planning/reference/backend-build-conventions.md`, `.planning/reference/e2e-harness-spec.md`

### Contract & behavior
- `docs/api/E2-identity/openapi.yaml` — employees (76+,266+,390+,437+), change-requests (615+,666+,716+,750+), agreements (807+,958+,1011+,1090+), attachments (1146+). Match exactly.
- `docs/api/CONVENTIONS.md` §7,§8,§9,§11,§12 (cross-field validation),§15 (file uploads),§16.1 audit,§17 RBAC.
- `docs/epics/E2-identity/prds/employee-profile.md`, `employment-agreement.md` — Gherkin AC (E2E source) + BR-#/C-#. Domain grounded in Indonesian labor law (PKWT fixed-term vs PKWTT indefinite).
- `docs/epics/E2-identity/FEATURE.md`, `DATA-MAPPING.md` — invariants + legacy mapping (employees.id vs users.id split).

### Reference implementation
- `backend/internal/{handler,service,repository,domain}/org` (Phase-3 slice to mirror), `.../foundations`, `backend/internal/platform/*`, `backend/db/queries/org/*`, `backend/db/migrations/0000*`.
- `backend/cmd/seed/seed.go`. FE screens: `frontend/apps/web/src/features/e2-identity/{employees-screen,employee-detail-screen,employee-form,employee-overlays,agreements-screen,agreement-detail-screen,agreement-form,change-requests-screen,change-request-overlays}.tsx`; hooks `frontend/packages/api-client/src/e2.ts`. E2E patterns `frontend/e2e/tests/e2/*` + `frontend/e2e/lib/*`.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Platform kernel (httpx cursor/PageResponse, rbac, audit, apperr, ids, idempotency, db.TxManager). Three reference slices (identity, foundations, org). E2E harness boots real stack + resetDb + loginAs + db helpers.
- Phase-3 org tables (companies, positions, service_lines) employees/agreements can FK to.

### Established Patterns
- migration → sqlc (`make gen`) → repository (domain mapping, tx writes) → service (apperr codes, audit) → hand-written handler → routes in server.go under RequireRole → Go contract tests → FE wiring + live E2E. Shared-file edits (server.go/main.go/seed.go) coordinated via markers; run backend slices that share those files SEQUENTIALLY.

### Integration Points
- New query dir `backend/db/queries/people/` (sqlc glob picks it up). Routes in server.go authenticated group. New multipart handler + an authenticated file-download route. Seed extension. FE screens exist (built from .pen), mostly call hooks via MSW — wire to real BE.
</code_context>

<specifics>
## Specific Ideas
- Employee records for the seeded personas (SWP-EMP-1042 Sari Hadi, 1108 Rudi Wijaya, 2891 Budi) should exist so /auth/me employee_id + agreements/change-requests resolve.
- Attachment upload E2E must upload a REAL small file (PDF/PNG fixture) and assert the returned metadata + that download requires auth.
</specifics>

<deferred>
## Deferred Ideas
- `/employees:bulk-deactivate` and the per-employee change-request SUBMIT endpoint (agent/mobile side) — FE web doesn't call them.
- Placement/scheduling that hangs off employees — later phases.
</deferred>

---

*Phase: 04-e2-people*
*Context gathered: 2026-06-04*
