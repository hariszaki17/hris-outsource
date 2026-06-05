# Milestones

## v1.0 Backend + Full-Stack E2E (Shipped: 2026-06-05)

**Phases completed:** 11 phases, ~50 plans · **Timeline:** 2026-05-30 → 2026-06-05 (~6 days)

**Delivered:** the entire SWP HRIS web console now works end-to-end against a real Go API + ephemeral Postgres — every FE-used endpoint across 11 epics implemented behind the locked OpenAPI contracts and proven by exhaustive full-stack Playwright E2E (final suite: 239 passed / 6 skipped / 0 failed).

**Key accomplishments:**
- **Auth + harness (E1):** real login/refresh/logout/forgot/reset wired off stubs; a reusable headless/headful/UI Playwright harness booting real BE + ephemeral Postgres + seeded personas.
- **Org → People → Placement (E2/E3):** companies/sites/service-lines/positions/master-data, employees/agreements/change-requests, and the project differentiator — Placement as a first-class entity with INV-1..4 enforced (DB partial-unique index + FOR UPDATE row-locking, defense-in-depth).
- **Scheduling + Attendance + Leave + Overtime (E4–E7):** shift masters + a 6-check conflict engine with bulk-apply partial success; attendance verify/reject + corrections (bulk + idempotency); two-level leave approval with the cross-epic INV-3 loop-closer (approval cancels overlapping shifts + populates `approved_leave_days` → drives SHIFT_OVER_LEAVE); overtime workflow + holiday calendar + day-type classification.
- **Payroll + Reporting (E8/E10):** read-only encrypted-at-rest payslips (AES-256-GCM, DECRYPT_FAIL as a 200 row status) with async Excel export via a real River worker; role-aware dashboard, billable report, and the notification loop-closer (un-stubbed worker + dispatch helper retro-wired into leave/OT/attendance so auto-dispatched notifications genuinely appear), plus the generalized `/exports` framework.
- **Quality spine:** a Go contract-test drift gate per slice (the OpenAPI stays the FE's Orval source), full-stack Playwright E2E per Gherkin AC, audit-in-tx on every write, consistent RBAC scope (GuardCompany).

**Tech debt (tracked):** notify dispatch coverage limited to leave/OT/attendance for v1.0 (remaining stubs are nil-safe no-ops); PDF export deferred to v1.1 (Excel only); 6 phases' E2E is executor-run green but pending one independent human `pnpm e2e` pass.

See `.planning/milestones/v1.0-MILESTONE-AUDIT.md` for the full audit (30/30 reqs · 11/11 phases · 5/5 integration seams).

---

