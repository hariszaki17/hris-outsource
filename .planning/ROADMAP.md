# Roadmap: SWP HRIS backend + full-stack E2E (v1.0-be)

## Overview

Make the existing web console work against a real Go backend, epic by epic, in dependency
order. Phase 1 stands up the full-stack Playwright harness and real auth (which gates
everything). Each subsequent phase implements the FE-used endpoints of one epic — matching
the OpenAPI contract, following the platform-kernel patterns and the E1 auth reference
slice — and proves them with exhaustive Playwright E2E (real FE ↔ real BE ↔ ephemeral
Postgres). Scope is strictly the endpoints the FE calls today
(`.planning/reference/fe-endpoint-inventory.md`). Build rules:
`.planning/reference/backend-build-conventions.md`. E2E rules:
`.planning/reference/e2e-harness-spec.md`.

## Phases

- [x] **Phase 1: Test Harness + Auth** - Playwright full-stack harness + real login/refresh/logout/forgot/reset, FE wired to BE (completed 2026-06-04)
- [x] **Phase 2: E1 Foundations** - Users, roles, audit log, platform settings (completed 2026-06-04)
- [x] **Phase 3: E2 Org & Master Data** - Client companies, sites, service lines, positions, leave/attendance/overtime master data (5/6 plans complete — awaiting E2E plan 03-06)
- [x] **Phase 4: E2 People** - Employees, employment agreements, change requests (completed 2026-06-04)
- [ ] **Phase 5: E3 Placement** - Placements lifecycle + shift-leader assignments + roster
- [ ] **Phase 6: E4 Schedule & Shifts** - Shift masters, schedule entries, conflict check, bulk apply
- [x] **Phase 7: E5 Attendance** - Attendance verify/reject (incl. bulk) + corrections
- [x] **Phase 8: E6 Leave** - Leave requests multi-step approval, quotas, calendar
- [ ] **Phase 9: E7 Overtime** - Overtime workflow + public holidays
- [x] **Phase 10: E8 Payroll** - Payslips (read-only) + audit notes + export (completed 2026-06-05)
- [ ] **Phase 11: E10 Reporting & Notifications** - Dashboard, billable report, notifications, export framework

## Phase Details

### Phase 1: Test Harness + Auth
**Goal:** A reusable full-stack Playwright harness and working real authentication; the web app logs in against the real Go API.
**Depends on:** Nothing (first phase)
**Requirements:** HARN-01, HARN-02, AUTH-01, AUTH-02, AUTH-03, AUTH-04
**Success Criteria** (what must be TRUE):
  1. `pnpm e2e`, `pnpm e2e:headed`, and `pnpm e2e:ui` all run the real FE against the real Go API + an ephemeral Postgres (MSW off); each Gherkin scenario is its own runnable case in the Playwright UI.
  2. `backend/cmd/seed` seeds the four personas and minimal data; the harness migrates + seeds automatically in globalSetup.
  3. A user can log in via the web login screen (real `POST /auth/login`), reach the dashboard, refresh the token, and log out; `GET /auth/me` drives the session.
  4. Forgot-password and reset-password screens call the real BE; wrong-credentials / disabled-account / RBAC errors render the correct UI states.
**Plans:** 5/5 plans complete

Plans:
- [ ] 01-01: Playwright harness — `frontend/e2e` package, `playwright.config.ts`, scripts (headless/headed/ui/debug/report), `docker-compose.e2e.yml`, globalSetup/Teardown booting BE+PG+migrate+seed
- [ ] 01-02: `backend/cmd/seed` (personas + minimal fixtures) + dev Ed25519 keygen helper (`cmd/api genkeys` or seed)
- [ ] 01-03: Auth endpoints — extend identity slice with `forgot-password` + `reset-password`; confirm login/refresh/logout/me vs E1 spec
- [ ] 01-04: FE wiring — login/forgot/reset screens → real `@swp/api-client` hooks; remove dev-token stub; `installAuth` against real BE
- [ ] 01-05: Auth E2E — exhaustive per E1 auth Gherkin AC (success, bad creds, disabled, refresh, logout, forgot/reset)

### Phase 2: E1 Foundations
**Goal:** Foundations admin screens (users, audit log, settings) work against the real BE.
**Depends on:** Phase 1
**Requirements:** FND-01, FND-02, FND-03
**Success Criteria** (what must be TRUE):
  1. HR/super-admin can list, create, update users, change role, deactivate/reactivate, and trigger a password reset against the real BE.
  2. The audit log lists entries with filters + cursor pagination and opens an entry detail; every write in this milestone produces an audit row.
  3. Platform settings load on the settings screen.
  4. Exhaustive Playwright E2E for E1 foundations features is green (headless).
**Plans:** 4/4 plans complete

Plans:
- [ ] 02-01-PLAN.md (wave 1) — Migration 00008 platform_settings + sqlc queries: users list/update/role/status, audit-log list(+filters)/get, settings read; make gen
- [ ] 02-02-PLAN.md (wave 2, deps 02-01) — Foundations domain/repo/service/handlers + routes: users mgmt, audit-log read, settings; RBAC(super_admin,hr_admin) + idempotency + audit on writes; seed extension
- [ ] 02-03-PLAN.md (wave 3, deps 02-02) — Go contract tests vs E1 openapi: exact shapes, status codes, cursor envelope, RBAC 403
- [ ] 02-04-PLAN.md (wave 3, deps 02-02) — FE wiring (MSW off) + exhaustive Playwright E2E per E1 PRDs (users CRUD/role/status/reset, audit list/filter/paginate/detail, settings, RBAC negatives), green headless

### Phase 3: E2 Org & Master Data
**Goal:** Client companies, sites, service lines, positions, and master data (leave types, attendance codes, overtime rules) work against the real BE.
**Depends on:** Phase 2
**Requirements:** ORG-01, ORG-02, ORG-03, ORG-04
**Success Criteria** (what must be TRUE):
  1. HR can manage client companies (list/detail/create/update/reactivate) and their sites (with geofence) against the real BE.
  2. HR can manage service lines + positions (create/update/discontinue/soft-delete) and the master-data sets (leave types, attendance codes, overtime rules).
  3. Picker endpoints return the picker-shaped lists the FE expects (CONVENTIONS §18).
  4. Exhaustive Playwright E2E for E2 org/master-data features is green.
**Plans:** 6/6 plans complete

Plans:
- [x] 03-01-PLAN.md (wave 1) — Migrations 00009–00015 + sqlc queries for all 7 E2 org/master entities (make gen)
- [x] 03-02-PLAN.md (wave 2, deps 03-01) — Client companies + sites slice (services/handlers/routes/RBAC/audit/geofence) + seed Plaza Senayan (SWP-CMP-0021)
- [x] 03-03-PLAN.md (wave 2, deps 03-01,03-02) — Service lines + positions slice (discontinue/soft-delete/in-use guards) + seed
- [x] 03-04-PLAN.md (wave 2, deps 03-01,03-02) — Master data slice (leave types, attendance codes, overtime rules; min_minutes rule) + seed
- [x] 03-05-PLAN.md (wave 3, deps 03-02..04) — Go contract tests for all E2 org/master endpoints
- [ ] 03-06-PLAN.md (wave 3, deps 03-02..04) — FE wiring (MSW off) + exhaustive Playwright E2E per the 4 E2 PRDs, green headless

### Phase 4: E2 People
**Goal:** Employees, employment agreements, and the change-request approval queue work against the real BE.
**Depends on:** Phase 3
**Requirements:** PPL-01, PPL-02, PPL-03
**Success Criteria** (what must be TRUE):
  1. HR can manage employees (list/detail/create/update/deactivate/reactivate).
  2. HR can manage employment agreements (create/renew/close) and upload an attachment (multipart, ≤10MB).
  3. HR can review and approve/reject change requests; PKWT/PKWTT cross-field rules enforced (422 with field errors).
  4. Exhaustive Playwright E2E for E2 people features is green.
**Plans:** 6/6 plans complete

Plans:
- [ ] 04-01-PLAN.md (wave 1) — Migrations 00016–00019 + sqlc queries (employees, employment_agreements, agreement_attachments, change_requests) + File id prefix; make gen
- [ ] 04-02-PLAN.md (wave 2, deps 04-01) — Employees slice (domain/repo/service/handlers/routes/RBAC/audit, DUPLICATE_NIK) + seed persona employees [shared server.go/main.go/seed.go — sequential]
- [ ] 04-03-PLAN.md (wave 3, deps 04-02) — Agreements + attachments slice (PKWT/PKWTT rules, renew/close, multipart upload + authenticated download) + seed agreement+attachment [sequential]
- [ ] 04-04-PLAN.md (wave 4, deps 04-03) — Change-requests queue slice (list/detail-diff/approve-applies/reject) + seed pending CRs [sequential]
- [ ] 04-05-PLAN.md (wave 5, deps 04-02..04) — Go contract tests for all people endpoints (incl. multipart FileRef + PKWT 422 + CR transitions)
- [ ] 04-06-PLAN.md (wave 5, deps 04-02..04) — FE wiring (MSW off) + exhaustive Playwright E2E per the E2 people PRDs (real file upload), green headless [parallel with 04-05]

### Phase 5: E3 Placement
**Goal:** Placement as a first-class entity — lifecycle + shift-leader assignments + roster — works against the real BE with invariants enforced.
**Depends on:** Phase 4
**Requirements:** PLC-01, PLC-02, PLC-03, PLC-04
**Success Criteria** (what must be TRUE):
  1. HR can create placements with INV-1..4 enforced (e.g. one active placement per agent → 409 `INV_1_VIOLATION`), and list incl. expiring + detail.
  2. Placement lifecycle actions (renew/transfer/end/resign/terminate) work and write history + audit.
  3. Shift-leader assignment (create/replace/end) enforces exactly one leader per company (409 on violation); roster renders.
  4. Exhaustive Playwright E2E for E3 placement features is green.
**Plans:** 4 plans

Plans:
- [x] 05-01-PLAN.md (wave 1) — Migrations 00020-00022 + sqlc queries (placements, placement_history, shift_leader_assignments) + INV-1/INV-2/INV-3 partial unique indexes + domain types
- [ ] 05-02-PLAN.md (wave 2, deps 05-01) — Services + handlers: INV-1..4 enforcement (DB index + FOR UPDATE locks + leader_scope), lifecycle state machine, transfer/renew atomicity, history+audit, roster, routes/main.go, seed; + error-envelope details extension
- [ ] 05-03-PLAN.md (wave 3, deps 05-02) — Go contract tests vs E3 openapi examples (all invariant 409 envelopes + site-scope leadership)
- [ ] 05-04-PLAN.md (wave 4, deps 05-02,05-03) — FE wiring (MSW off) + exhaustive Playwright E2E per the 5 E3 PRDs (incl. INV-1/2/3/4 negatives + RBAC scope), green headless

### Phase 6: E4 Schedule & Shifts
**Goal:** Shift masters and scheduling (with conflict checks + bulk apply) work against the real BE.
**Depends on:** Phase 5
**Requirements:** SCH-01, SCH-02, SCH-03
**Success Criteria** (what must be TRUE):
  1. HR/leader can manage shift masters and schedule entries (create/update/delete) against the real BE.
  2. Conflict check returns double-shift / over-leave / outside-placement-period violations with the correct codes; bulk apply reports partial success.
  3. Schedule lists are cursor-paginated and scoped (leader sees own company).
  4. Exhaustive Playwright E2E for E4 features is green.
**Plans:** 1/4 plans executed

Plans:
- [ ] 06-01-PLAN.md (wave 1) — Migrations 00023-00025 + sqlc queries (shift_masters, schedule_entries, E4-owned approved_leave_days) + INV-1 partial unique index + domain types
- [ ] 06-02-PLAN.md (wave 2, deps 06-01) — Services + handlers: conflict engine (all 6 codes, honest over-leave), :check dry-run, :bulk-apply partial success, shift-master CRUD + deactivate/reactivate, schedule CRUD, leader scope (GuardCompany), audit + notify stub, routes/main.go, seed
- [ ] 06-03-PLAN.md (wave 3, deps 06-02) — Go contract tests vs E4 openapi (every conflict code, bulk-apply partial-success envelope, scope 403, cursor/list shapes)
- [ ] 06-04-PLAN.md (wave 4, deps 06-02,06-03) — FE wiring (MSW off) + exhaustive Playwright E2E under frontend/e2e/tests/e4/ (shift-master CRUD, grid CRUD, conflict negatives incl. SHIFT_OVER_LEAVE, bulk-apply partial success, leader-scope 403), green headless

### Phase 7: E5 Attendance
**Goal:** Attendance verification (incl. bulk) and corrections work against the real BE.
**Depends on:** Phase 6
**Requirements:** ATT-01, ATT-02
**Success Criteria** (what must be TRUE):
  1. Leader can list attendance (virtualized/cursor), open detail, verify/reject single, and bulk verify/reject with partial-success reporting + idempotency.
  2. Corrections can be listed and approved/rejected; out-of-geofence and rule violations return 422 with correct codes.
  3. Verifications/rejections produce audit rows and notifications.
  4. Exhaustive Playwright E2E for E5 features is green.
**Plans:** 4 plans

Plans:
- [x] 07-01-PLAN.md (wave 1) — Migrations 00026-00027 + sqlc queries (attendance, attendance_corrections) + domain types (FKs to schedule_entries/placements/employees; seeded exception flags + enums)
- [x] 07-02-PLAN.md (wave 2, deps 07-01) — Services + handlers: verify/reject (+bulk partial-success + Idempotency-Key), corrections approve-applies/reject, GuardCompany scope (OUT_OF_SCOPE), VERIFY_OWN_RECORD, terminal-state 409, OUTSIDE_CORRECTION_WINDOW, audit-in-tx + notify stub, routes/main.go, seed
- [x] 07-03-PLAN.md (wave 3, deps 07-02) — Go contract tests vs E5 openapi (every code + bulk partial-success envelope + idempotency replay + scope 403 + 422s + terminal 409 + cursor shapes)
- [x] 07-04-PLAN.md (wave 4, deps 07-02,07-03) — FE wiring (MSW off) + exhaustive Playwright E2E under frontend/e2e/tests/e5/ (list/detail, single + bulk verify/reject + idempotency, corrections approve/reject, scope 403, VERIFY_OWN_RECORD), green headless

### Phase 8: E6 Leave
**Goal:** Leave requests (multi-step approval), quotas, and the calendar work against the real BE.
**Depends on:** Phase 7
**Requirements:** LVE-01, LVE-02, LVE-03
**Success Criteria** (what must be TRUE):
  1. Leave requests can be listed/detailed and moved through L1 / final / override approval and rejection, with the correct state transitions + notifications.
  2. Quotas list, adjust, and bulk-grant work; quota-exceeded returns 422 `QUOTA_EXCEEDED` with field errors.
  3. The leave calendar renders for the requested range.
  4. Exhaustive Playwright E2E for E6 features is green.
**Plans:** 2/4 plans executed

Plans:
- [x] 08-01-PLAN.md (wave 1) — Migrations 00028-00030 + sqlc queries + domain (leave_requests, leave_quotas, leave_approvals)
- [x] 08-02-PLAN.md (wave 2, deps 08-01) — Services + handlers: two-level approval state machine, quota guard/adjust/bulk-grant, INV-3 loop-closer (cancel schedule + populate approved_leave_days in-tx), calendar, scope, audit/notify, routes/main.go, seed
- [x] 08-03-PLAN.md (wave 3, deps 08-02) — Go contract tests vs E6 openapi (transitions + 409s, QUOTA_EXCEEDED/BALANCE_RECHECK 422, OUT_OF_SCOPE 403, bulk-grant partial success, calendar shape)
- [ ] 08-04-PLAN.md (wave 4, deps 08-02,08-03) — FE wiring (MSW off) + exhaustive Playwright E2E under frontend/e2e/tests/e6/ (approvals/quotas/calendar/scope + the INV-3 loop-closer assertion)

### Phase 9: E7 Overtime
**Goal:** Overtime workflow and public holidays work against the real BE.
**Depends on:** Phase 8
**Requirements:** OVT-01, OVT-02
**Success Criteria** (what must be TRUE):
  1. Overtime can be listed/detailed and moved through confirm / L1 / final / reject / withdraw, plus bulk approve/reject with partial success.
  2. Business rules (e.g. OT < 30 min) return 422 with the correct code; holidays can be listed/created/updated/deleted.
  3. Actions produce audit rows + notifications.
  4. Exhaustive Playwright E2E for E7 features is green.
**Plans:** 4 plans

Plans:
- [ ] 09-01-PLAN.md (wave 1) — Migrations 00031-00032 + sqlc queries + domain (overtime, overtime_approvals, holidays)
- [ ] 09-02-PLAN.md (wave 2, deps 09-01) — Services + handlers: OT two-level workflow (confirm/L1/final/reject/withdraw), bulk partial-success, OT_BELOW_MIN, day_type classification, holiday CRUD (clash/in-use), scope + SELF_APPROVAL_FORBIDDEN, audit/notify, routes/main.go, seed
- [ ] 09-03-PLAN.md (wave 3, deps 09-02) — Go contract tests vs E7 openapi (transitions + 409s, OT_BELOW_MIN 422, HOLIDAY_DATE_CLASH/IN_USE, OUT_OF_SCOPE/SELF_APPROVAL_FORBIDDEN 403, bulk partial-success, cursor shapes)
- [ ] 09-04-PLAN.md (wave 4, deps 09-02,09-03) — FE wiring (MSW off) + exhaustive Playwright E2E under frontend/e2e/tests/e7/ (confirm→L1→final, reject, withdraw, bulk partial, OT_BELOW_MIN, holiday CRUD + clash/in-use, scope 403 + SELF_APPROVAL_FORBIDDEN)

### Phase 10: E8 Payroll
**Goal:** Read-only payslips, audit notes, and async export work against the real BE.
**Depends on:** Phase 9
**Requirements:** PAY-01, PAY-02
**Success Criteria** (what must be TRUE):
  1. Payslips list and detail render (read-only history); audit notes can be listed and created.
  2. Payslip export returns 202 + a job id and the job completes via the worker.
  3. RBAC restricts payroll visibility appropriately.
  4. Exhaustive Playwright E2E for E8 features is green.
**Plans:** 4/4 plans complete

Plans:
- [ ] 10-01-PLAN.md (wave 1) — Migrations 00033-00034 + sqlc queries (payslips + components/benefits/audit-notes, export_jobs) + domain + the AES-256-GCM crypto helper (encryption-at-rest)
- [ ] 10-02-PLAN.md (wave 2, deps 10-01) — Services + handlers: read payslips (decrypt-at-boundary + DECRYPT_FAIL as a 200 row status), audit notes list/create, async export (River EnqueueTx + PayslipExportWorker → export_jobs DONE; EXPORT_TOO_LARGE), RBAC hr/super, routes/main.go/jobs.go/seed
- [ ] 10-03-PLAN.md (wave 3, deps 10-02) — Go contract tests vs E8 openapi (list/detail incl. DECRYPT_FAIL 200 row, audit notes, export 202 + job enqueue, EXPORT_TOO_LARGE 422, RBAC 403, cursor shapes)
- [ ] 10-04-PLAN.md (wave 4, deps 10-02,10-03) — FE wiring (MSW off) + exhaustive Playwright E2E under frontend/e2e/tests/e8/ (archive+filters, detail breakdown, DECRYPT_FAIL render, audit notes, export 202 + worker-completes-job via export_jobs DB poll, RBAC 403)

### Phase 11: E10 Reporting & Notifications
**Goal:** Dashboard, billable report, notifications, and the export framework work against the real BE.
**Depends on:** Phase 10
**Requirements:** RPT-01, RPT-02, RPT-03, RPT-04
**Success Criteria** (what must be TRUE):
  1. The role-aware dashboard (`/dashboards/me`) and billable attendance report render against the real BE.
  2. Notifications list and mark-read / mark-all-read work; auto-dispatched notifications from earlier phases appear.
  3. The export framework (create/get/cancel, async) works end-to-end via the worker.
  4. Exhaustive Playwright E2E for E10 features is green.
**Plans:** 2/5 plans executed

Plans:
- [ ] 11-01-PLAN.md (wave 1) — Migrations 00035 (notifications) + 00036 (generalize export_jobs) + sqlc reporting queries (notifications/exports/dashboard/billable) + reporting domain; make gen
- [ ] 11-02-PLAN.md (wave 2, deps 11-01) — Notifications service/handler/routes + UN-STUB NotificationWorker + notify.Dispatch helper + retro-wire prior-phase dispatch points (leave/OT/attendance) + seed notifications [owns server.go/main.go/jobs.go/notify.go/seed.go + prior-service edits]
- [ ] 11-02b-PLAN.md (wave 3, deps 11-01,11-02) — Dashboard aggregation (/dashboards/me role-aware) + billable report (/reports/attendance-billable) + generalized export framework (POST/GET/:cancel async, ReportExportWorker, DB↔wire status mapping) [appends after the 11-02 routes]
- [ ] 11-03-PLAN.md (wave 4, deps 11-02,11-02b) — Go contract tests vs E10 openapi (notifications list/mark-read/mark-all-read, dashboard shapes, billable, export 202+outbox + EXPORT_FORMAT_UNSUPPORTED/TOO_LARGE/RATE_LIMITED, RBAC, cursor)
- [ ] 11-04-PLAN.md (wave 5, deps 11-02,11-02b,11-03) — FE wiring (MSW off) + exhaustive Playwright E2E under frontend/e2e/tests/e10/ (dashboard role-aware, billable, notifications + mark-read + REAL auto-dispatch capstone, export create→worker DONE + cancel + PDF-unsupported), green headless

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Test Harness + Auth | 5/5 | Complete    | 2026-06-04 |
| 2. E1 Foundations | 4/4 | Complete    | 2026-06-04 |
| 3. E2 Org & Master Data | 6/6 | Complete    | 2026-06-04 |
| 4. E2 People | 6/6 | Complete    | 2026-06-04 |
| 5. E3 Placement | 4/4 | Complete    | 2026-06-04 |
| 6. E4 Schedule & Shifts | 4/4 | Complete    | 2026-06-04 |
| 7. E5 Attendance | 4/4 | Complete | - |
| 8. E6 Leave | 4/4 | Complete | 2026-06-05 |
| 9. E7 Overtime | 0/4 | Not started | - |
| 10. E8 Payroll | 4/4 | Complete    | 2026-06-05 |
| 11. E10 Reporting & Notifications | 2/5 | In Progress|  |
