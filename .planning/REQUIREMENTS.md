# Requirements: SWP HRIS backend + E2E (v1.0-be)

**Defined:** 2026-06-03
**Core Value:** Every screen the web app shows today works end-to-end against the real backend.

> Each requirement = "the FE feature works against the real BE, with exhaustive Playwright
> E2E green." Endpoint-level scope is in `.planning/reference/fe-endpoint-inventory.md`;
> behavior is the epic's PRD Gherkin AC. Build approach: `.planning/reference/backend-build-conventions.md`.

## v1 Requirements

### Test Harness & Auth (Phase 1)
- [x] **HARN-01**: Playwright E2E runs the real FE against the real Go API + ephemeral Postgres (MSW off), with headless / headful / UI (`--ui`) modes and per-scenario test cases.
- [x] **HARN-02**: `backend/cmd/seed` seeds deterministic personas (hr_admin Sari Hadi, shift_leader Rudi Wijaya @ Plaza Senayan, super_admin, agent) + minimal data.
- [x] **AUTH-01**: User can log in via the web login screen against the real BE (`POST /auth/login`) and reach the dashboard.
- [x] **AUTH-02**: Access token refresh (`POST /auth/refresh`) and logout (`POST /auth/logout`) work; `GET /auth/me` returns the principal.
- [x] **AUTH-03**: Forgot-password and reset-password flows call the real BE.
- [x] **AUTH-04**: Wrong credentials / disabled account / RBAC produce the correct error states in the UI.

### E1 Foundations & Settings (Phase 2)
- [x] **FND-01**: User management — list/create/update users, change role, deactivate/reactivate, send password reset.
- [x] **FND-02**: Audit log — list + entry detail with filters and cursor pagination.
- [x] **FND-03**: Platform settings read.

### E2 Org & Master Data (Phase 3)
- [ ] **ORG-01**: Client companies — list/detail/create/update/reactivate.
- [ ] **ORG-02**: Sites (per company) — list/create/update, with geofence.
- [ ] **ORG-03**: Service lines + positions — list/detail/create/update/discontinue/soft-delete.
- [ ] **ORG-04**: Master data — leave types, attendance codes, overtime rules (list/create/update/soft-delete).

### E2 People (Phase 4)
- [ ] **PPL-01**: Employees — list/detail/create/update/deactivate/reactivate.
- [ ] **PPL-02**: Employment agreements — list/detail/create/renew/close + attachment upload.
- [ ] **PPL-03**: Change requests — list/detail/approve/reject (HR approval queue).

### E3 Placement (Phase 5)
- [ ] **PLC-01**: Placements — list (incl. expiring)/detail/create with INV-1..4 enforcement.
- [ ] **PLC-02**: Placement lifecycle — renew/transfer/end/resign/terminate.
- [ ] **PLC-03**: Shift-leader assignments — create/replace/end (one leader per company).
- [ ] **PLC-04**: Company roster view.

### E4 Schedule & Shifts (Phase 6)
- [ ] **SCH-01**: Shift masters — list/create/update/deactivate/reactivate.
- [ ] **SCH-02**: Schedule entries — list/create/update/delete.
- [ ] **SCH-03**: Conflict check + bulk apply (double-shift / over-leave / outside-placement guards).

### E5 Attendance (Phase 7)
- [ ] **ATT-01**: Attendance — list/detail, verify/reject, bulk verify/reject.
- [ ] **ATT-02**: Corrections — list/detail/approve/reject.

### E6 Leave (Phase 8)
- [ ] **LVE-01**: Leave requests — list/detail, multi-step approve (L1/final/override)/reject.
- [ ] **LVE-02**: Leave quotas — list/adjust/bulk-grant (quota-exceeded → 422).
- [ ] **LVE-03**: Leave calendar.

### E7 Overtime (Phase 9)
- [ ] **OVT-01**: Overtime — list/detail, confirm/approve(L1/final)/reject/withdraw, bulk approve/reject.
- [ ] **OVT-02**: Public holidays — list/create/update/delete.

### E8 Payroll (Phase 10)
- [ ] **PAY-01**: Payslips — list/detail (read-only history), audit notes list/create.
- [ ] **PAY-02**: Payslip export (async job → 202 + job id).

### E10 Reporting, Notifications & Exports (Phase 11)
- [ ] **RPT-01**: My dashboard (role-aware).
- [ ] **RPT-02**: Billable attendance report.
- [ ] **RPT-03**: Notifications — list/mark-read/mark-all-read.
- [ ] **RPT-04**: Export framework — create/get/cancel (async).

## Out of Scope

| Feature | Reason |
|---------|--------|
| Spec endpoints the FE doesn't call yet | Scope = FE-used only; defer until FE adds them |
| E9 migration | Separate effort, no API |
| Mobile endpoints | Web-first this milestone |
| Server-side OpenAPI codegen | oapi-codegen lacks 3.1 support; hand-written + contract tests |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| HARN-01, HARN-02, AUTH-01..04 | Phase 1 | Pending |
| FND-01..03 | Phase 2 | Pending |
| ORG-01..04 | Phase 3 | Pending |
| PPL-01..03 | Phase 4 | Pending |
| PLC-01..04 | Phase 5 | Pending |
| SCH-01..03 | Phase 6 | Pending |
| ATT-01..02 | Phase 7 | Pending |
| LVE-01..03 | Phase 8 | Pending |
| OVT-01..02 | Phase 9 | Pending |
| PAY-01..02 | Phase 10 | Pending |
| RPT-01..04 | Phase 11 | Pending |

**Coverage:** all v1 requirements mapped to phases ✓

---
*Requirements defined: 2026-06-03*
