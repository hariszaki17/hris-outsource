# hris-outsource — Epic Breakdown

> Master decomposition for the from-scratch rebuild of SWP's HRIS, focused on managing outsourced agents.
> Decomposition method: OODA (this brainstorm) + user-story-mapping (epics → features → PRDs/stories).
> Status: brainstorm complete; epics drafted. Next: feature docs per epic, then PRDs per feature.

---

## 1. Context

**Company:** PT Saranawisesa Properindo (SWP) — an outsourcing provider supplying agents across a range of service types (e.g. facility services, building management, parking). Work is shift-heavy, 24/7, on client sites.

**Why a rebuild (not editing ims-system):** The legacy system (`ims-system`, Laravel Lumen + Next.js, MySQL) is a 200+ table monolith where HRIS is only ~50 tables, the user model is generic multi-tenant, and "placement" is just a string field. The outsource domain (first-class placement, shift leaders) diverges enough that a clean rebuild + data migration beats surgery on the monolith.

**What we keep:** only the HRIS subset. **Migration source:** SWP prod, MySQL DB `lumen_swp`.

---

## 2. Locked decisions

| Area | Decision |
|---|---|
| Tenancy | **Internal only.** Only SWP staff use the system. Client companies are *data*, not tenants/logins. |
| Roles | super admin · HR/placement admin · **shift leader** · agent |
| Placement | An agent placed at a **client company**, in a **position** (free-text), for a **contract period** (the employment agreement may be backfilled — optional at create, §8 2026-06-11); tracked with history. |
| Shift leader | **1 per client company/placement.** Verifies attendance, approves OT & leave, manages roster, sees team dashboards. |
| Shifts | A **work-shift master** (working hours + break) is admin-defined; the shift leader **picks from master** to schedule each agent. |
| Position | **Free-text** designation on the placement (no master, no FK), with a typeahead over existing values. *(service line removed entirely — §8 2026-06-12)* |
| Payroll | **Compute-assist** in v1 — migrate history (read-only) **and** run a monthly payroll: system assembles agent pay from E2/E5/E6/E7, HR posts immutable payslips, **manual** transfer + evidence (no bank/BPJS/tax API). Agent pay = **monthly wage**, not billable hours. *(flipped from data-only 2026-06-11; see §8 E8)* |
| MVP | **All core modules together** (attendance + shift + leave + overtime), one release; build order dependency-driven. |
| Migration | **Everything** from SWP prod incl. payroll + full history. MySQL → Postgres **transform-and-load** (not a copy). |
| Stack | **Backend: Go · Frontend: React · DB: Postgres.** (FE framework + Go libs being finalized.) |

---

## 3. Role model (internal)

- **Super admin** — full system config, user management, master data.
- **HR / Placement admin** — manages employees, placements, master data; oversight & escalation approver.
- **Shift leader** — on-site supervisor for ONE client company; rosters, approvals, verification for that company's agents.
- **Agent** — placed employee; clocks in/out, requests leave/OT, views own schedule.

## 4. Core domain sketch

```
Employee ──< Placement >── ClientCompany
   │            │  (position, period, status, history)
   │            └── ShiftLeaderAssignment (1 leader per company)
   │
Schedule (agent + ShiftMaster + date)  ← shift leader builds from ShiftMaster catalog
   │
Attendance (vs Schedule) · Leave (vs Quota) · Overtime (vs Schedule)
   │
Payroll = migrated history (read-only) + compute-assist run (base E2 ± attendance/leave/OT → posted payslip → manual paid)
```

---

## 5. Epics

Each epic below becomes a **feature document**; each listed feature becomes a **PRD** with stories.

### E1 — Foundations & Platform
**Goal:** a runnable Go API + React app + Postgres with auth, RBAC, and shared infra.
**Features:** project scaffold (Go service, React app, Postgres, migrations) · authentication (login + token/session) · RBAC (roles & permissions) · audit log · API conventions (errors, pagination, validation) · base UI shell & navigation.
**Entities:** User, Role, Permission, AuditLog.
**Depends on:** — (first)

### E2 — Identity, Org & Master Data
**Goal:** model people and the reference data everything hangs on.
**Features:** Employee/Agent profile · Client/Partner Company directory · Leave types · Attendance codes/policies · Overtime rules.
**Entities:** Employee, ClientCompany, LeaveType, AttendanceCode, OvertimeRule. *(Position is free-text on the placement — no master entity; service line removed, §8 2026-06-12.)*
**Depends on:** E1

### E3 — Placement Management ⭐ (differentiator)
**Goal:** place agents at client companies, in a position, for a period; track history; assign the company's shift leader.
**Features:** Agent placement (agent + company + position (free-text) + period; employment agreement may be backfilled — optional at create, see §8 2026-06-11) · Placement lifecycle/status · Re-placement & transfer with history · Shift-leader assignment (1 per company) · Company placement roster view.
**Entities:** Placement (≈ legacy `employee_contract`), ShiftLeaderAssignment.
**Depends on:** E2

### E4 — Shift Configuration & Scheduling
**Goal:** define shift master templates and let shift leaders schedule agents per placement.
**Features:** Work-shift master catalog (hours + breaks; **independent of service line**) · Roster/schedule builder (leader picks from master, assigns agents to dates) · Rotation patterns · Schedule calendar & publish/notify · **Roster-compliance indicators** (holiday-shift badge + holiday day-column tint; missing-weekly-rest flag; >6-consecutive-workday cap warning) — derives from roster + `/holidays`, surfaces to the shift leader (§8 D1/D3).
**Entities:** ShiftMaster, Schedule, RotationPattern.
**Depends on:** E3, E2

### E5 — Attendance
**Goal:** shift-aware on-site clock in/out, verified by the shift leader; corrections.
**Features:** Clock in/out (geo lat/long, WFO flag) · Shift-aware late/early detection (vs schedule) · Shift-leader verification/approval · Attendance corrections + approval · Attendance status (codes) · Attendance dashboard.
**Entities:** Attendance (≈ `attendance_user`), AttendanceCorrection.
**Depends on:** E4, E3

### E6 — Leave Management
**Goal:** leave types, quotas/balances, request + approval (shift leader), documents.
**Features:** Leave quotas/balances (annual/accrual) · Leave request · Approval workflow (shift leader) · Document requirements/upload · Delegate · Leave calendar.
**Entities:** Leave, EmployeeLeaveQuota (LeaveType from E2).
**Depends on:** E2, E3

### E7 — Overtime Tracking
**Goal:** OT capture/request, approval (shift leader), duration calc per rules.
**Features:** OT request · OT detection vs schedule · Approval workflow · OT calculation (per **global** rules) · OT status · OT reporting · **Holiday calendar bootstrap** ("Import {tahun}" prefill → HR confirm + cuti bersama, §8 D4) · **statutory OT multiplier seeding** (PP 35/2021 defaults: workday 1.5×→2×; rest-day/holiday progressive 2×/3×/4×, stored as reference).
**Entities:** Overtime, OvertimeStatus.
**Depends on:** E4, E5, E3

### E8 — Payroll (historical archive + compute-assist runs)
**Goal:** preserve migrated payroll history (read-only) **and** run a monthly **compute-assist** payroll for placed agents. Agent pay = **monthly wage** (E2 base), modulated by verified attendance/leave/OT — **not** billable hours (those bill the client, hours-only, outside).
**Features:** Payslip history & summaries (migrated + generated) · Payroll archive (components/benefits/payments) · **Compute-assist payroll run** (assemble from E2/E5/E6/E7 → review/adjust → post immutable payslips) · **Payment recording** (manual transfer + uploaded evidence) · prior-period carry-forward adjustments.
**Entities:** PayrollRun, Payslip, SalaryComponent, PayrollPayment, PayrollAdjustment, Benefit (migrated history read-only).
**Out of scope (v1):** bank/BPJS/tax API; automatic BPJS/PPh21 calculation engine (statutory lines are editable, config-assisted); client invoicing/rate application; editing posted payslips.
**Depends on:** E2, E9

### E9 — Data Migration (SWP prod → hris-outsource)
**Goal:** transform-and-load everything from MySQL `lumen_swp` into Postgres under the new model; validate & reconcile.
**Features:** Source extraction (MySQL) · Field/entity mapping (legacy role + `company_id` → new model; `placement` string → entity) · Load into Postgres · Identity/role remap · Reconciliation & validation reports · Idempotent re-runs · Cutover runbook.
**v1 mode (resolved 2026-06-02):** **One-shot script — no UI.** Big-bang cutover via full script execution. Blocking items (`decrypt_fail`, `orphan_identity`, `unmatched_placement`) are pre-resolved in code (alias tables + hardcoded crosswalks) or logged + skipped; no human-in-the-loop review queue. Manual inspection if needed via SQL/CLI on the staging DB.
**Depends on:** target schemas from E1–E8 (runs continuously; lands at cutover).

### E10 — Reporting, Exports & Notifications (cross-cutting)
**Goal:** operational reports + notifications across modules.
**Features:** Role-based dashboards · Exports (Excel/PDF) · Notifications (in-app/email: schedule published, approvals pending/advanced/decided, attendance anomalies) · **shift-leader compliance notifications** (agent assigned a holiday shift; agent with no weekly rest / >6 consecutive workdays — §8 D3) · Approval inboxes.
**Depends on:** spans E2–E8, E11.

### E11 — Approvals (configurable multi-line engine, cross-cutting) *(added 2026-06-14)*
**Goal:** a generic, per-company approval engine that routes any request type (leave, overtime, …) through an HR-configured chain, replacing the hardcoded `shift_leader → HR/lead` routing.
**Features:** Approval template management (per company; 2–3 ordered lines, each a multi-user **OR**-set, line 3 optional) · Approval execution engine (instance per request, sequential line advance, self-block, terminal reject, **super-admin bypass**, per-type side-effect hooks) · Approval inbox (line-member "needs my decision" queue, web + mobile).
**Entities:** ApprovalTemplate, ApprovalLine, ApprovalLineMember, ApprovalInstance, ApprovalAction.
**Depends on:** E1 (auth/RBAC/audit), E2 (companies), E3 (company scope); consumed by E6, E7. **Removes:** E2 profile change-request approval (profile edits become instant self-edit).

---

## 6. Build sequencing (one release, dependency-ordered)

```
E1 Foundations
  └─ E2 Identity/Org/Master
       └─ E3 Placement ⭐
            └─ E4 Shift & Scheduling
                 └─ E5 Attendance
                      ├─ E11 Approvals  (engine; before E6/E7 routing)
                      ├─ E6 Leave        (parallel after E3/E2; routes via E11)
                      └─ E7 Overtime     (after E5; routes via E11)
E8 Payroll-data  ─┐
E9 Migration     ─┼─ continuous; E9 lands at cutover, E8/E10 follow data
E10 Reporting    ─┘
```

## 7. Documentation index — all 11 epics drafted ✅

Each epic has a `FEATURE.md` (features + BPMN-style Mermaid workflows) and per-feature `prds/*.md` (user story + Gherkin AC + cases). E2–E8 also have a `DATA-MAPPING.md` (legacy `lumen_swp` → new model); E9 orchestrates those mappings; E10 & E1 read across modules.

| Epic | Feature doc | PRDs | Data map |
|------|-------------|------|----------|
| **E1** Foundations & Platform | [FEATURE](epics/E1-foundations/FEATURE.md) | authentication · rbac-roles · audit-log · platform-conventions | — (uses E2) |
| **E2** Identity, Org & Master | [FEATURE](epics/E2-identity/FEATURE.md) | employee-profile · employment-agreement · client-company-directory · client-sites-geofence · operational-master-data | [✓](epics/E2-identity/DATA-MAPPING.md) |
| **E3** Placement ⭐ | [FEATURE](epics/E3-placement/FEATURE.md) | agent-placement · placement-lifecycle · replacement-transfer · shift-leader-assignment · company-roster | [✓](epics/E3-placement/DATA-MAPPING.md) |
| **E4** Shift & Scheduling | [FEATURE](epics/E4-shift-scheduling/FEATURE.md) | shift-master-catalog · daily-schedule-assignment · schedule-views · schedule-changes-swaps | [✓](epics/E4-shift-scheduling/DATA-MAPPING.md) |
| **E5** Attendance | [FEATURE](epics/E5-attendance/FEATURE.md) | clock-in-out · attendance-evaluation · attendance-verification · attendance-corrections · attendance-records · manual-attendance · attendance-auto-reconcile | [✓](epics/E5-attendance/DATA-MAPPING.md) |
| **E6** Leave | [FEATURE](epics/E6-leave/FEATURE.md) | leave-quota-balances · leave-request · leave-approval · leave-schedule-integration · leave-calendar-views | [✓](epics/E6-leave/DATA-MAPPING.md) |
| **E7** Overtime | [FEATURE](epics/E7-overtime/FEATURE.md) | overtime-rules · overtime-capture · overtime-approval · overtime-records | [✓](epics/E7-overtime/DATA-MAPPING.md) |
| **E8** Payroll (history + compute-assist) | [FEATURE](epics/E8-payroll/FEATURE.md) | payslip-history · payroll-archive · payroll-run · payroll-payment · payroll-period-close | [✓](epics/E8-payroll/DATA-MAPPING.md) |
| **E9** Data Migration *(script-only, no UI v1)* | [FEATURE](epics/E9-migration/FEATURE.md) | extraction-staging · transform-crosswalks · reconciliation-review · load-idempotent · cutover-validation | (orchestrates E2–E8) |
| **E10** Reporting & Notifications | [FEATURE](epics/E10-reporting/FEATURE.md) | notifications · dashboards · attendance-billable-report · export-framework | — (reads modules) |
| **E11** Approvals *(engine, cross-cutting)* | [FEATURE](epics/E11-approvals/FEATURE.md) | approval-template-management · approval-execution · approval-inbox | — (config, no legacy) |

**Totals:** 11 epics · 49 PRDs · 11 feature docs · 7 data-mapping docs.

> Folder layout: `docs/epics/<E#-name>/FEATURE.md` + `prds/<feature>.md` (+ `DATA-MAPPING.md` for E2–E8).

---

## 8. Resolved product decisions — open-items review (2026-05-29)

> Authoritative decision log from the open-items review. Where this conflicts with a per-epic `FEATURE.md` §7 "Still open" list, **this section wins** (per-epic docs reconciled progressively). ✅ = explicitly chosen; *(default)* = sensible default applied — override anytime.

**E1 — Foundations**
- MFA: not in v1 (admin/HR hardening later) *(default)*
- ✅ **Every employee auto-provisions a login at create; phone is the primary login identifier** *(resolved 2026-06-07)* — supersedes the prior "email-less agents: assign an email at provisioning (login stays email+password)" default. Login no longer requires email; the login identifier is the agent's **phone number** (required) or **email** (optional). See the cross-cutting auto-provision decision below.
- Role assignment: super_admin + hr_admin may assign roles *(default)*
- Finance sub-role: none in v1 (HR sees payroll); IDR-only, no multi-currency; bulk actions audited per affected row *(default)*
- ✅ **Login lockout — 5 failed attempts → 15-minute suspension** *(resolved 2026-06-19)*. After 5 consecutive failed login attempts, the account is locked for 15 minutes. No CAPTCHA, no per-IP tracking in v1. Lock is per-user (not per-session). Super admin may manually unlock.
- ✅ **Agents get a web self-service console (reverses "agent is mobile-only")** *(resolved 2026-06-10)* — the `agent` role may now sign in to the **web console** (`apps/web`) and reach a self-service surface at parity with the mobile app (dashboard, **web clock-in/out**, own schedule, own attendance + corrections, leave, overtime, profile, payslip, notifications). This **supersedes** NAVIGATION-AND-RBAC.md §4's "`agent` — none (mobile-only)". It is **frontend-only**: every clock + self endpoint already declares `x-rbac: { roles:[agent], scope:self }` and resolves the agent from the JWT principal. Mechanics: agent routes under the **`/me/*`** prefix; new **`self.*` capability keys** form the `agent` bundle; `agent` joins `WEB_ROLES`; the shell selects the nav backbone by role. **Does not change internal-only tenancy** (agents are SWP staff, not clients — the client portal stays a separate, unratified concern). G0 exception (no agent web `.pen` frames; built pragmatically from `packages/ui` + mobile frames) is recorded in the spec. See [docs/eng/AGENT-WEB-ACCESS.md](eng/AGENT-WEB-ACCESS.md). *(The `roles:[agent]` gating in this bullet is superseded 2026-06-15 — see "agent retired as a role" below; the `/me/*` routes and `self.*` bundle survive, just no longer behind a role.)*
- ✅ **`agent` retired as a role — self-service is an implicit baseline; `users.role` holds only elevations** *(resolved 2026-06-15)* — cross-cutting (E1 auth/RBAC · every `self.*` / `/me/*` endpoint · NAVIGATION-AND-RBAC · AGENT-WEB-ACCESS · E9 migration). Builds on the same-day "one Employee, FIELD vs INTERNAL" decision (E2): because internal SWP staff use the **same** self-service features as field agents (clock-in, leave, overtime, payslip), self-service is no longer an agent-specific capability.
  - **A — `self.*` is a baseline, not a role.** Every `Employee` implicitly carries the `self.*` bundle (the `/me/*` self surface: dashboard, clock in/out, own schedule + attendance + corrections, leave, overtime, profile, payslip, notifications). It is granted by **being an employee**, not by holding a role. This supersedes the 2026-06-10 framing ("`self.*` forms the `agent` bundle; `agent` joins `WEB_ROLES`") — the bundle and `/me/*` routes survive, they are just no longer keyed to a role.
  - **B — `users.role` = elevation only.** The role enum **drops `agent`**; it now holds only elevations: **`lead`, `hr_admin`, `super_admin`** (plus **`shift_leader`**, which stays *derived* from the E3 assignment, never stored). An employee with **no elevation** = a plain self-service employee — either a field agent **or** an internal rank-and-file clerk. Every elevated role **also** carries the `self.*` baseline (an `hr_admin` clocks in too).
  - **C — "agent" is now purely a domain term.** It means an **`employee_type = FIELD`** employee placed at a client. Used in UI/Bahasa ("Agen"), reports, and copy — **never** as an auth role. The field/internal distinction is `employee_type`, not role.
  - **D — RBAC mechanics.** `x-rbac: { roles:[agent], scope:self }` on every self/clock/`/me/*` endpoint becomes **`scope: self` with no role gate** — any authenticated employee principal passes; the JWT principal resolves the employee. Elevation capability keys (`lead.*`, `hr.*`, `admin.*`) layer on top. NAVIGATION-AND-RBAC §4 + the `WEB_ROLES` list reconcile: the shell selects the nav backbone by **highest elevation**, defaulting to the **self-service backbone** when there is none.
  - **E — migration.** E9 emits **no `agent` role**: legacy `users.role = 'agent'` → **no elevation** (baseline only) + `employee_type = FIELD`; legacy internal roles map to their elevation + `employee_type = INTERNAL`.
  - See [E1 FEATURE], [NAVIGATION-AND-RBAC §4], [AGENT-WEB-ACCESS], [E9 DATA-MAPPING], [API CONVENTIONS — `x-rbac`].

**E2 — Identity, Org & Master**
- ✅ **Client sites = first-class entity (reverses "flat, no sub-sites")** *(resolved 2026-06-03)* — a ClientCompany has **one or more `Site`s** (placement locations). Geofence config (**address, lat/lng, `geofence_radius_m` default 100m**) moves **off ClientCompany onto `Site`**. This supersedes the 2026-05-29 "flat (no sub-sites)" decision and DATA-MAPPING G-6's "ignore sub-companies". New feature **F2.6 Client Sites & Geofence**.
- ✅ **Placement targets a Site** *(2026-06-03)* — `Placement.site_id` is **required**; every company has **≥1 Site** (single-location companies get one auto **primary "Main Site"**). E5 geofence always resolves from `placement.site` (no company fallback).
- ✅ **Geofence model = single circle per site** *(2026-06-03)* — one center (lat/lng) + radius; out-of-geofence stays **allowed + flagged** (E5). Multi-circle / polygon are post-v1.
- ✅ **Shift-leader scope = company-wide only** *(revised 2026-06-19)* — `leader_scope` removed from `ClientCompany`; shift leader always covers the entire company. `ShiftLeaderAssignment.site_id` removed. No per-site leaders. *(Supersedes the 2026-06-03 "configurable per company" decision.)*
- ✅ **Employment agreement is document-only** *(resolved 2026-06-19)* — the employment agreement has **no system impact**: no expiry detection, no offboarding cascade, no period validation on placement, no reactivation check. It is purely a document management feature (CRUD). The `employment_agreement_id` and `awaiting_agreement` fields are removed from Placement.
- ✅ **Leave type is un-deletable** *(resolved 2026-06-19)* — master data (leave types) cannot be deleted, only deactivated. Leave entitlements (per-employee) CAN be deleted.
- Agent mobile-editable fields: phone, address, bank (HR-approved) *(default)*
- Agent mobile-editable fields: phone, address, bank (HR-approved) *(default)*
- ~~Service lines: the 3 are seeded but **admin-extendable**~~ — **superseded 2026-06-12** by "Service line removed entirely" (below).
- ✅ **Compensation & annual-leave entitlement are employment-agreement (E2) terms, not placement (E3) terms** *(resolved 2026-06-07)* — under alih-daya law the employment relationship is SWP↔agent; a placement is only a work *designation* to a client. So **base salary** stays the single source on E2 `CompensationRecord`, and the **annual leave entitlement** moves onto the **EmploymentAgreement** as new field `annual_leave_entitlement_days` (int, ≥0, nullable). The duplicated `Placement.annual_leave_entitlement` + `Placement.base_salary_ref` are **removed from E3**; transfer/replacement no longer carry comp overrides. E6 leave-quota already sources the annual entitlement from E2, so this aligns the model. **E3 BR-9 (position is selected per-placement and may differ across companies) is unaffected — only the compensation/leave duplication is removed.** See [E2 employment-agreement PRD] + [E3 FEATURE §4].
- ✅ **Every employee auto-provisions a login; phone-or-email identifier (reverses "data-only employees")** *(resolved 2026-06-07)* — supersedes the E1 "email-less agents: assign an email at provisioning" default. Cross-cutting (E1 auth · E2 employee create · E9 migration):
  - **D1 — auto-provision, 1:1 non-null.** Creating an employee **always** provisions a User in the same step. `Employee`↔`User` is now **1:1 NON-NULL** (no nullable `user_id`). The **"data-only / no-login employee" concept is removed entirely** — and with it the separate `:provision-login` endpoint, the `provision_login` flag, the `has_user`/`HasLogin` filter, the "Tanpa Login" list tab/stat, and the "Punya Akun" column.
  - **D2 — phone-or-email identifier.** The login identifier is **phone number** (**required**, unique, normalized E.164 / Indonesian `+62`) **or email** (optional, unique), plus password. The login endpoint accepts a single `identifier` (phone or email) + password. Schema: add **`users.phone` (unique)**; **`users.email` becomes nullable**.
  - **D3 — temporary credential (unchanged).** Initial credential is a **system-generated temporary password**, shown once to the creating admin, **force-rotated on first login** (already implemented; kept — no HR-typed passwords).
  - **D4 — migration backfill.** E9 (DATA-MAPPING G-5): every legacy employee without a user is **backfilled a User keyed on phone** (email if present). **No null `user_id` post-migration.**
  - See [E1 FEATURE], [E2 employees PRD], [E9 DATA-MAPPING G-5].
- ✅ **Offboard cascade implemented** *(resolved 2026-06-07)* — `berhentikan` atomically closes the active agreement + ends non-terminal placements + revokes login (one transaction); cascaded closes are audit-tagged `caused_by = employee_offboard` (+ `source_employee_id`) for traceability vs a direct standalone close. MVP reason enum = `RESIGNED`/`TERMINATED`/`END_OF_TERM`/`OTHER` (`RETIRED`/`DECEASED`/`ABSCONDED` deferred — needs a `closed_reason` CHECK migration). See [E2 offboarding PRD F2.7].
- ✅ **Employment-agreement MVP simplifications** *(resolved 2026-06-07)* — **no attachment upload** at create (object/bucket storage isn't provisioned for MVP; the agreement-attachments capability — migration 00018 / attachment endpoints — is **deferred post-MVP**; agreements are created without an attached PDF); **no "Save as Draft"** (agreements are created **directly active** — the status set is `active | superseded | closed`, DRAFT was never a real status); the **agreements list shows the joined employee name** alongside the employee id and is **searchable by employee name, employee id, or agreement number** (`q` free-text filter), dropping the successor/"Pengganti" column, the per-row kebab menu, and the filter "Reset" button. See [E2 employment-agreement PRD].
- ✅ **Klien (client company + service line) UI/flow consolidation** *(resolved 2026-06-07)* — four web-console changes in the E2 "Klien" section, no domain/invariant change:
  - **A — edit is a full-page screen, not a drawer.** Client-company **edit** is a dedicated full-page screen launched from the company **detail page** (route `/client-companies/$id/edit`). The previous edit Drawer (`EditClientCompanyDrawer`) is **removed**.
  - **B — list row action = activate/deactivate only.** The client-company **list** drops its per-row kebab menu; the sole row action is **Aktifkan/Nonaktifkan** (still guarded by the active-placement guard, CC-5). Edit moved entirely to the detail page (A).
  - **C — Profil tab no longer duplicates Sites/geofence.** The company **detail "Profil" tab** shows only statutory/billing fields + `leader_scope`; **Sites & geofence live ONLY in the "Lokasi & Site" tab** (F2.6, INV-5).
  - ~~**D — service-line maintenance consolidated on the detail page.**~~ — **superseded 2026-06-12** by "Service line removed entirely" (below); the service-line detail page no longer exists, and position is free-text (no master to maintain).
  - See [E2 F2.3 PRD], [E2 FEATURE §F2.3].
- ~~✅ **Service-line create may seed initial positions atomically** *(resolved 2026-06-08)*~~ — **superseded 2026-06-12** by "Service line removed entirely" (below). Service lines no longer exist as an entity, so there is no `POST /service-lines` and no per-line position seeding; position is now a free-text field with a typeahead over DISTINCT existing values (no master, no FK).
- ✅ **System serves ALL SWP staff, not only field agents — one Employee, FIELD vs INTERNAL category, HQ-as-a-site** *(resolved 2026-06-15)* — cross-cutting (E2 identity · E3 placement · E4 scheduling · E5 attendance · E7 overtime · E9 migration · E10 reporting). SWP's own internal team (HR, super-admins, finance, leads) are first-class users of the same attendance/leave/overtime features as placed agents, so the model does **not** fork "agent" from "employee" — it stays one `Employee` entity, differentiated by a category and by whether a work assignment exists.
  - **A — single Employee + `employee_type`.** Keep the single `EMPLOYEE` table for everyone (already so). Add **`employee_type ∈ {FIELD, INTERNAL}`** (default `FIELD`). `FIELD` = placed at a *client* (today's agent). `INTERNAL` = SWP HQ staff. This is **orthogonal to the auth role**: an `INTERNAL` employee is typically `hr_admin`/`super_admin`/`lead`; a `FIELD` employee is `agent`/`shift_leader`. "Agent" therefore = a `FIELD` employee with an active client placement — not a separate person model.
  - **B — SWP HQ is an internal company.** Generalize the company directory: add **`Company.type ∈ {CLIENT, INTERNAL}`** (CLIENT = today's `companies.role=2` client; INTERNAL = SWP itself). The internal company has **≥1 `Site`** like any company (HQ office, branch offices). Client-facing surfaces — client lists, billing, SLA/billable reporting, `lead`/`shift_leader` company scope — are keyed on **`type = CLIENT`** and **exclude** the internal company.
  - **C — Placement generalizes to "work assignment to any company"; one attendance path.** An `INTERNAL` employee gets a **placement/assignment to an HQ `Site`**, reusing the **same** E3 placement + E4 schedule (office hours as the "shift") + E5 attendance + E7 overtime machinery — **no second code path**. INV-1 "one active placement per agent" generalizes to **per employee**. Attendance always resolves `site + schedule` regardless of category. (Self-service web/mobile already open to `agent`; internal staff reach the same `/me/*` self surface.)
  - **D — geofence is per-site + per-clock-in mode, NOT per-category.** Decouple geofence from `employee_type`. **Site geofence (lat/lng/radius) is optional/nullable** — a site with **no geofence** skips the location check entirely (covers a **mobile/cruise field placement** *and* a fully-remote role). For staff whose site **does** define a geofence but who work from home on a given day, the **clock-in carries a `mode ∈ {ONSITE, REMOTE}`**; `REMOTE` **skips the geofence check** (recorded/flagged as remote, not blocked). Net rule: geofence is enforced **iff** the site defines one **and** `mode = ONSITE`; out-of-geofence stays **allowed + flagged** (unchanged). Both FIELD and INTERNAL employees can be geofenced or not — the category never decides it.
  - **E — leave & overtime already cover internal.** E6 leave has no placement dependency and E7 manual overtime can be filed without one, so internal staff are already served; with C, **E7 auto-detected OT** now also works for internal staff via their HQ schedule + verified attendance.
  - **Reconciliation flags (progressive, §8 wins):** E2 F2.6 — `Site` geofence becomes explicitly **optional** (the "always resolves from `placement.site`" / 100m-default wording now means "when the site defines one"). E3 INV-1 — generalize "agent" → "employee". E5 — drop the "placed agents only" / "clock-in requires geofence" framing; add the `mode` field + nullable-geofence handling. E10 — client/billable rollups filter `Company.type = CLIENT`. **E9 migration** — set `employee_type` (legacy placed agents → `FIELD`; legacy internal `users` without a client placement → `INTERNAL`), and **seed the SWP `INTERNAL` company + its HQ Site(s)**.
  - Open for follow-up: the office-hours "shift" shape for INTERNAL (fixed Mon–Fri window vs reusing the E4 shift master as-is) — resolve when E4 internal scheduling is specced.
  - See [E2 FEATURE §4], [E3 FEATURE §4], [E5 FEATURE §1/§4], [E9 DATA-MAPPING].

**E3 — Placement & Shift-Leader Assignment**
- ✅ **Placement is decoupled from time** *(resolved 2026-06-19)* — `start_date` is always today, `end_date` is always nil (open-ended). No Scheduled state, no future-dated placements, no backdating, no period validation. Placement is a "currently active at company X" record, not a date-bounded contract. END is still supported (manual end by HR). The state machine simplifies to `Active → Ended/Terminated/Resigned/Superseded`.
- ✅ **Position is on Employee, not Placement** *(resolved 2026-06-19)* — `Employee.position` (E2 F2.1) is the single source of truth. Placement no longer has its own position field. Transfer does not change position. The per-placement free-text position (BR-9) is removed; the typeahead endpoint is removed.
- ✅ **Leader scope is company-wide only** *(resolved 2026-06-19 — consolidated into E2 §8)* — `leader_scope` removed; shift leader always covers the entire company.
- ✅ **Shift-leader identity is fully assignment-driven; role + scope are derived, never stored** *(resolved 2026-06-08)*
  - **D1 — derived role + scope.** An employee's effective auth **role** and company **scope** are **derived server-side at request time** from their active E3 `shift_leader_assignments` row (the auth middleware computes them per request). Stored `users.role`, `users.company_id`, and the JWT `cmp` claim are **advisory only**. An employee with an active leader-assignment **is** a `shift_leader` scoped to that one company (consistent with **INV-3**, leader ⇄ exactly one company); an employee without one is a plain `agent`. Assign/revoke therefore takes effect on the leader's **next request** — no re-login, no drift between the two former representations. **Fail-safe:** if no active assignment resolves, scope is stripped and role falls back to `agent` (deny, never escalate).
  - **D2 — single entry point.** Leader assignment (assign / replace / revoke) is managed **only** from the client-company detail page's **"Pemimpin Shift"** tab. The previously-duplicated assignment UI on the placement-detail screen is now **read-only** (links to the company tab); the company-roster "Ganti" action links there too.
  - **D3 — company-detail tabs implemented.** The three formerly-stubbed tabs are now built: **Penempatan Aktif** (active roster), **Pemimpin Shift** (current leader + assign/replace/revoke), **Riwayat** (historical placements).
  - **D4 — E1 reconciliation flagged.** Manually setting `users.role = 'shift_leader'` is **no longer** how a leader is created, and the previously-documented "demoting a user auto-ends their E3 assignment" side-effect is **moot** under the derived model (role for field staff is emergent from the assignment). The **E1 `changeUserRole` spec must be reconciled**.
  - Related earlier same-day fixes folded in: **E2 `GET /client-companies`** is scoped to a shift_leader's own company; **E4 `GET /schedule`** auto-scopes to the leader's company.
  - See [E3 FEATURE §4], [E2 F2.3 PRD], [E1 FEATURE], [E4 FEATURE].
- ✅ **Employment agreement is OPTIONAL at placement creation — "pending agreement" tracking model** *(resolved 2026-06-11 — supersedes E3 F3.1 BR-1, which made the agreement mandatory at create)* — operationally, an agent is frequently placed and **starts work before the PKWT/PKWTT is finalized** (the legal paperwork lags onboarding). So `agreement_id` becomes **optional and nullable** both on the placement-create request and on the `Placement` response model:
  - **A — pending flag, not a lifecycle state.** A placement created without an agreement is flagged with a derived boolean **`awaiting_agreement` (= `agreement_id` is null)**. This is an **orthogonal compliance flag, NOT a new lifecycle state** — the F3.2 state machine is untouched; an `awaiting_agreement` placement can still be ACTIVE / PENDING_START / EXPIRING. List and roster endpoints gain an `?awaiting_agreement=true` filter so HR can work the pending-agreement backlog.
  - **B — period validation only when an agreement is present.** The BR-1b period-within-agreement check (and the `422 PLACEMENT_OUTSIDE_CONTRACT` error + PKWT `end_date` auto-cap) **only runs when an `agreement_id` is supplied**. No agreement → no period check, and `end_date` may be open-ended.
  - **C — backfill endpoint.** A pending agreement is attached later via **`POST /placements/{id}/agreement`** `{ agreement_id }`, which re-runs the BR-1b period validation / PKWT auto-cap and clears `awaiting_agreement`. `404` if the placement is unknown; `422 AGREEMENT_NOT_OWNED` if the agreement doesn't belong to the placement's agent.
  - **D — renew/transfer propagate pending.** Renewing or transferring a placement that has no agreement produces a **successor that is also pending** (null `agreement_id` propagates; no auto-cap) until its own agreement is backfilled.
  - **E — legal basis still required eventually.** Under alih-daya law the employment relationship is SWP↔agent, so a finalized PKWT/PKWTT is still mandatory for the *employment* — this decision only removes it as a **blocking precondition at the placement-create step**; `awaiting_agreement` exists precisely so the outstanding paperwork stays visible and gets closed.
  - See [E3 F3.1 PRD agent-placement BR-1/BR-1b + new backfill BR], [E3 FEATURE §F3.1/§4], [E3 openapi `createPlacement` / `backfillPlacementAgreement`].
- ✅ **New `lead` role — company-scoped operational approver** *(resolved 2026-06-12)* — cross-cutting (E1 auth · E3 placement · E6 leave · E7 overtime). A generic `lead` system role is a company-scoped operational approver over a set of assigned client companies; scope is keyed on company membership only.
  - **A — who/scope.** A lead is **SWP staff** with a **stored** auth role (`users.role = 'lead'`), unlike the derived `shift_leader`. Each lead is assigned to a **set of client companies** via **`lead_assignments`** (one lead covers many companies), and that company set is **resolved per request** into the principal's **`CompanyIDs []string`** scope.
  - **B — arranges placements.** Within its assigned companies, a lead is the routine placement arranger — **creates, transfers, renews, and ends** placements. **HR retains global oversight + override** (and stays the arranger outside any lead's scope).
  - **C — L2/final approver.** A lead is the **Level-2 (final) approver for leave + overtime**, scoped to the agent's company; **L1 stays the on-site `shift_leader`**; **HR remains a global L2**. A lead **cannot approve its own** request (same separation rule as shift_leader).
  - **D — cannot.** A lead **cannot** add employees (tambah karyawan), run payroll, do master-data writes, assign shift-leaders (SLA), or approve bank-account changes (those **escalate to HR**).
  - **E — scope axis reaffirmed** *(2026-06-15)* — lead scope stays the **company set** (`lead_assignments`); there is **no work-type / division axis** (service line is gone, §8 2026-06-12). A "parking lead" is simply a `lead` assigned the client companies whose contracts are parking. **Known limitation:** a single company that mixes work types on one contract (e.g. parking **and** facility agents) cannot be split across two leads — both would scope the whole company. Revisit with a finer axis only if mixed-work-type companies become common.
  - See [E3 FEATURE §2], [E6 leave-approval LA-3], [E7 overtime-approval OA-3], [NAVIGATION-AND-RBAC §4].
- ✅ **Service line removed entirely** *(resolved 2026-06-12)* — the **service-line concept is dropped** from the whole model and every epic that referenced it. Cross-cutting (E2 master data · E3 placement · E4 scheduling · E5 attendance · E6 leave · E7 overtime · E8 payroll · E9 migration · E10 reporting). Supersedes the prior service-line sub-decisions (the 2026-06-07 "Klien consolidation" service-line maintenance bullet, the 2026-06-08 "service-line create seeds positions atomically", and the seeded-3-lines default).
  - **A — no master entity.** `ServiceLine`/`Position` masters, the `SWP-SVC`/`SWP-POS` IDs, the `/service-lines` and `/service-lines/{id}/positions` endpoints, and the service-line nav/master-data surfaces are all removed.
  - **B — position is free-text.** **Position** becomes a **free-text** field on the placement (no master, no FK, no ID), with a **typeahead** that suggests over DISTINCT existing values via `GET /positions:search`. E3 BR-9 (position is per-placement and may differ across companies) is preserved — it is just free-text now.
  - **C — service line dropped from operations.** Placement, scheduling, attendance, leave, and overtime no longer carry or branch on service line. **Shift master is independent** (no service-line scoping on the shift catalog or the schedule shift-picker). **Overtime rules are GLOBAL ONLY** (the service-line scope axis + its precedence are dropped). **Leave coverage-clash is DROPPED entirely** (was service-line-aware). **Holidays are GLOBAL ONLY** (drop `applicable_service_lines` / per-service-line `HolidayCategory` scoping).
  - **D — reporting + payroll by position.** Dashboard org rollups and billable/SLA reports **GROUP BY position** (free-text) instead of service line. Payroll history carries no service-line dimension.
  - **E — migration simplified.** E9 drops the `unclassified_service_line` classification and any service-line transform; legacy service-line strings map to the free-text **position** where applicable, otherwise discarded.
  - The `lead` role is a **company-scoped** operational approver (see above) — there is no service-line RBAC axis.
  - See [API CONVENTIONS §18], [NAVIGATION-AND-RBAC §3.2/§4], [E2 FEATURE], [E3 FEATURE], [E4 FEATURE], [E7 FEATURE], [E9 DATA-MAPPING].

**E4 — Shift & Scheduling**
- ✅ Agent shift-swap / day-off requests: **deferred to post-v1** (v1 = leader-driven schedule edits only; F4.4 swaps drop from v1 scope)
- ✅ Scheduling over approved leave: **blocked**
- ✅ **One shift per agent per day** (no split shifts)
- Shift reminder = evening-before + ~1h prior; bulk "apply shift to date range" helper included; schedule horizon unbounded *(default)*
- ✅ **Shift-master time edits propagate to unrealized schedule entries; frozen by attendance (start@check-in, end@check-out)** *(resolved 2026-06-09)* — when a shift master's `start_at`/`end_at` is edited, the change propagates to all `Schedule` entries with `work_date >= today`, `status != Off`, and not leave-cancelled — but only the unrealized portion: `start_time` freezes when the agent checks in; `end_time`/`cross_midnight` freezes when the agent checks out. An entry with no attendance follows the master live. An entry checked-in-but-not-out has `start_time` frozen while `end_time` still tracks master edits (the open attendance record's shift-end window also updates so lateness/early/auto-close use the live end). An entry with both check-in and check-out is fully frozen. Break times are master-only and not stored on entries. Rationale: each time value freezes exactly when it becomes operationally real (start is judged at check-in, end at check-out); fixes the UI mismatch where a schedule cell showed a stale snapshot while the shift picker showed the current master time. See [E4 FEATURE §4 INV-5], [F4.2 SA-10 + C-7/C-8], [F5.1 CI-9], [E5 FEATURE INV-6].

**E5 — Attendance**
- ✅ Per-site geofence (100m default) · ✅ late grace **15 min** · ✅ unscheduled clock-in: **`FIELD` blocked (`NO_SCHEDULED_SHIFT`) — must be rostered first; `INTERNAL` allowed + flagged** *(reversed 2026-06-18 from "all agents allowed + flagged")* · ✅ **online-only** for v1
- ✅ **Auto-close attendance — cron job closes unfinalized attendance records** (no clock-out past shift end) *(resolved 2026-06-19)*.
- ✅ Billable counts **verified records only**
- ✅ Leaders' own exception records **escalate to HR** (no self-verify)
- ✅ Agent self-correction window **7 days** (older = HR only)
- ✅ Anti-spoofing (mock GPS): **post-v1**
- Early clock-out flagged if >15 min early; no auto-verify SLA (pending stays + reminder) *(default)*
- ✅ **RESOLVED 2026-06-17 — Mobile clock-in photo required; web exempt (reverses "GPS only, no selfie").** **Mobile** clock-in (F5.1, `platform=MOBILE`) demands a **live photo** in addition to GPS: the agent captures it, the client uploads it first (`POST /attendance:photo-upload` → `SWP-FILE-*`) and passes it as `ClockInRequest.photo_id`; a mobile clock-in without a valid/unexpired photo is **rejected `422 PHOTO_REQUIRED`** (no record created). Stored as `Attendance.photo_in_id`. The mobile rule is independent of geofence/`mode` (ONSITE + REMOTE). **Web clock-in/out (`platform=WEB` — web console, HR/SL/on-behalf, internal-staff self) does NOT require a photo** for clock-in **or** clock-out. **Clock-out photo stays optional on every platform** (`photo_out_id`). `platform` defaults to `MOBILE` (fails safe). Online-only (no offline photo queue, consistent with v1). Supersedes the prior "Selfie/QR capture — not chosen (GPS only)" (E5 FEATURE §3). Spec: [clock-in-out PRD CI-10](epics/E5-attendance/prds/clock-in-out.md), [E5 FEATURE §F5.1](epics/E5-attendance/FEATURE.md), `docs/api/E5-attendance` openapi (`ClockInRequest.platform` + `photo_id`, `PHOTO_REQUIRED`).
- ✅ **RESOLVED 2026-06-15 — Corrections route through the E11 engine + NEW_ENTRY + attendance payable + Pengajuan surface.** Attendance corrections (F5.4) **migrate off their E5-native approve/reject onto the E11 approval engine** (`RequestType` extended to include **`CORRECTION`**): HR/SL decide in the **Inbox** like leave/overtime (line-membership gated, no self-approval), and the legacy `/corrections` screen becomes a **read-only list**. A new correction type **`NEW_ENTRY`** lets an agent **create an attendance record for a day with no record / no configured shift** (`attendance_id` nullable, `work_date` added; on approval creates the row flagged `UNSCHEDULED`+`CORRECTED`). Attendance gains **`is_payable`** (nullable) mirroring `approved_leave_days.is_payable` — auto `true` for shift days, `null`→manual SL/HR/super flag (`POST /attendance/{id}:set-payable`) for no-shift days; payroll treats `null` as non-payable. The agent surface moves onto the **Pengajuan** tab (3rd tab beside Cuti/Lembur, web + mobile) with a **searchable attendance picker**. **Supersedes** the prior E5-native shift-leader → HR correction routing (§E5 above / PRD F5.4 CR-2). Spec: [attendance-corrections PRD](epics/E5-attendance/prds/attendance-corrections.md), `docs/api/E5-attendance` + `E11-approvals` openapi.
- ✅ **RESOLVED 2026-06-17 — Attendance Auto-Reconcile (late-roster linking, new feature F5.7).** Flexible clock-in (F5.1/CI-4) stays: an agent may clock in with **no schedule** (record `UNSCHEDULED`, `PENDING`). New **F5.7** closes the loop when the roster is entered **late** — creating a **workable shift** (`is_day_off=false`, `SCHEDULED`, times set) that covers an existing **machine-owned** `UNSCHEDULED` record (same employee + placement) makes the shift retroactively **adopt** the record and **re-derive its truth** (link, lateness, status, flags, verification, payability) exactly as clock-in would have, had the shift existed.
  - **A — window-only matching (multi-shift-safe).** A shift adopts the candidate whose `check_in_at` lies in **`[shift_start − 2h, shift_end + 4h]`** (`4h` reuses the flexible-checkout grace); earliest if several; **no "earliest of the day" fallback** — a check-in outside every shift window genuinely is not this shift's (correct for single- and future multi-shift). Re-derivation reuses the clock-in **15-min grace** so a reconciled record is indistinguishable from one captured against a shift that always existed.
  - **B — human-decided records → lineage only.** A record already `VERIFIED`/`REJECTED`/`ESCALATED` is **never** re-derived; the system only attaches `schedule_id` + the shift snapshot for payroll/reporting lineage and audits it as **`ATTENDANCE_RELINKED`** (distinct from `ATTENDANCE_RECONCILED`). *(Chosen over a full skip — payroll/reporting need the shift tie; the human decision stays authoritative.)*
  - **C — synchronous, same-tx, fail-closed.** Reconcile runs inside the schedule-create transaction (`scheduling.CreateEntry`, also backing `:bulk-apply` per cell); an unexpected reconcile error rolls back the schedule create. Idempotent; `OUTSIDE_GEOFENCE`/`AUTO_CLOSED` survive; no new endpoint, no migration (all columns exist). **No reconcile** on schedule edit / time-change / `is_day_off` / `CANCELLED_BY_LEAVE` / `force_replace`.
  - **TODO(notify):** dequeue / notify the shift leader when a reconciled record flips `PENDING → AUTO_APPROVED` — deferred, slots into the Phase-11 schedule-notification TODO. **TODO(multi-shift):** true multi-shift (relax the `(employee_id, work_date)` partial unique; pick the window-containing shift at clock-in) is out of scope — F5.7 is already forward-compatible (window matching).
  - Spec: [attendance-auto-reconcile PRD](epics/E5-attendance/prds/attendance-auto-reconcile.md), [E5 FEATURE §F5.7](epics/E5-attendance/FEATURE.md); description-only notes in `docs/api/E4-shift-scheduling` (schedule-create side-effect) + `E5-attendance` openapi.
**E6 — Leave**
- ✅ **RESOLVED 2026-06-15 — HR-assigned entitlement.**
- ✅ Duration = **working days excluding public holidays** (working day = a day the agent would otherwise be rostered — exact shift-worker nuance to confirm)
- ✅ Period basis = **calendar year** for `ANNUAL_POOL` *(superseded 2026-06-08, **restored 2026-06-12** — the annual quota is keyed to the calendar year and expires year-end; per-type windows vary by `cap_basis`. See the per-type decision below.)*
- ✅ Probation: **pro-rated** annual leave in the first year (also pro-rate mid-year joiners)
- ✅ **Pro-rate for mid-year joiners — ANNUAL_POOL prorated based on `join_at` date** *(resolved 2026-06-19)* — `entitled ≈ entitlement × remaining_months / 12` (half-up). Applies to probation-period employees as well.
- ✅ **Cancel leave — possible only when no one has approved yet** *(resolved 2026-06-19)* — once any line member has approved (zero approval actions on the E11 instance), cancellation is blocked.
- ✅ Non-annual types (sick/event/religious/unpaid): **per-type quotas** → `LeaveQuota` per (employee, leave_type, window) *(superseded 2026-06-08, **reinstated & extended 2026-06-12** — leave_type is again the cap axis, now with an explicit `cap_basis` taxonomy. See the per-type decision below.)*
- ✅ Half-day leave: **not in v1** (full days only)
- Delegate = informational/notified (no enforced coverage); no-leader → HR sole approver; team calendar shows approved + a pending toggle *(default)*
- ✅ **Leave entitlement = per-type ledger (`leave_type` is the cap axis)** *(resolved 2026-06-12 — supersedes the entire 2026-06-08 grant-lot/one-pool decision below; reverts to per-type quotas and extends each type with explicit cap mechanics)*. **Driver:** SWP's `Fitur Ijin` policy defines **18 distinct leave types**, each with its own statutory cap; under Indonesian law (Pasal 93 vs Pasal 79 UU 13/2003 / PP 35/2021) event/sick/religious leave is **separate from** the 12-day annual leave, so it **cannot** draw a single pool.
  1. **`leave_type` carries its own cap mechanics** on the E2 `LeaveType` master: `code, category, cap_basis, cap_value, cap_unit (DAYS|COUNT), paid, gender (ANY|FEMALE|MALE), requires_document, notice_days, min_service_years, lead_days, trail_days`. The active catalog is the **18-code `Fitur Ijin` set** seeded in [E2 operational-master-data §5a](epics/E2-identity/prds/operational-master-data.md).
  2. **`cap_basis` taxonomy** decides metering: `ANNUAL_POOL` (sole accruing pool, calendar-year, **expires year-end, no carryover**, `entitled` from the E2 agreement) · `PER_EVENT` (fixed days **per occurrence**, no standing row) · `PER_MONTH` (resets monthly) · `PER_YEAR_COUNT` (occurrence **count**/year) · `UNCAPPED` (doc-bounded, no standing row) · `LIFETIME_ONCE` (once/employment) · `SERVICE_UNPAID` (eligibility-gated, **unpaid**, once).
  3. **`LeaveQuota` reinstated, generalized per-type** (`leave_quotas`, prefix `SWP-LQ-*`): `id, employee_id, leave_type_id, period_key (`<year>` | `<year-month>` | `EMP`), entitled_days, used_days, pending_days, expires_at (nullable), source (AUTO|ADJUSTMENT|MIGRATION), remark, created_by, created_at/updated_at`. `remaining = entitled − used − pending`. One row per (employee, type, window) for quota-bearing bases; `PER_EVENT`/`UNCAPPED` hold none. **Drops `leave_grants` + `leave_consumptions`** (grant-lots, FIFO, earmark, per-lot expiry).
  4. **Each type meters in its own window** — a request never charges another type's entitlement (the annual pool is never depleted by statutory/sick/religious leave). Reserve `pending_days` at submit, commit to `used_days` on approve, release on reject/cancel.
  5. **No negative balance.** Over-cap → block, then HR adjusts the type's quota. **Eligibility gates** (gender/notice/min-service/lifetime-once) enforced at request time. `paid=false` (e.g. `CLTP`) marks days **unpaid** for payroll (E8).
  - **Invariant remaps:** LQ-1 → annual auto-grant writes one `ANNUAL_POOL` quota from `employment_agreements.annual_leave_entitlement_days`. LQ-2/LQ-3 → reserve/commit/release on `pending_days`/`used_days`. LQ-4 → per-`cap_basis` window expiry/reset (annual year-end, no carryover; monthly/yearly resets). LQ-5/LQ-6 kept (no-negative; HR adjusts a quota, audited). LQ-7 → reinstated (one quota per type/window). New LQ-13..LQ-16 cover `cap_basis` metering, window auto-open, gates, paid flag.
  - **Implementation note:** the backend (`feat/backend-impl`) currently implements the 2026-06-08 grant-lot tables (`leave_grants` migr. 00044, `leave_consumptions`) + handlers/services + E6/E2 openapi. Those need **rework to this per-type model** — tracked as a follow-up coding task, not done in this spec change. Migration 00013 `leave_types` is extended with the cap columns and the 18-code catalog is **seeded**.
  - See [E6 FEATURE §4 + §7](epics/E6-leave/FEATURE.md), [leave-quota-balances PRD](epics/E6-leave/prds/leave-quota-balances.md), [E2 catalog §5a](epics/E2-identity/prds/operational-master-data.md), [E6 openapi](api/E6-leave/openapi.yaml) *(pending regen)*.

<details><summary>✅ <strong>Superseded — Leave balance = per-employee grant-lot ledger</strong> (resolved 2026-06-08; replaced 2026-06-12)</summary>

  1. **One pool per employee.** `leave_type` stays only as a **label + document gate (`requires_document`) + calendar color** — it is **no longer a balance axis**. All ordinary types draw the one pool.
  2. **Grants are lots** (`leave_grants`, prefix `SWP-LG-*`): one row per insert, each with its own `expires_at`. Columns: `id, employee_id, amount_days, granted_at, effective_from, expires_at, source (ANNUAL|ADJUSTMENT|MATERNITY|STATUTORY|MIGRATION|BONUS), earmark (nullable — null = general pool; non-null = purpose code restricting consumption), remark, consumed_days, pending_days, created_by, created_at/updated_at`. Remaining-per-lot = `amount − consumed − pending` (derived).
  3. **Hard per-lot expiry, no carryover.** A lot expires at its own `expires_at` (an expiry sweep zeroes it). No year-end global expiry, no carryover minting.
  4. **Consumption = FIFO by soonest `expires_at`**, across lots, recorded per-lot in `leave_consumptions` (prefix `SWP-LC-*`): `id, leave_request_id (FK), grant_id (FK), days, created_at`. This replaces the single `balance_quota_id` snapshot on leave_requests. Cancel/restore reverses the exact consumption rows.
  5. **No negative balance.** A request consumes only available (unexpired, matching-earmark) lots. Over-quota → HR adds a lot (pre-fund), never a negative balance. (LQ-5 kept, enforced at allocation.)
  6. **Long / statutory leave = HR pre-funds a lot.** e.g. maternity: HR inserts an earmarked lot (`source=MATERNITY, earmark=MATERNITY, remark, expires_at`); the employee then requests against it. No bypass flag, no separate table.
  7. **Optional earmark.** Unearmarked lots = the flat pool, drawn FIFO by ordinary requests. Earmarked lots are consumed **only** by a request of that purpose and are invisible to ordinary FIFO.

</details>

**E7 — Overtime**
- ✅ Public-holiday calendar: **HR-maintained in-app** master (recurring + one-off); shared with E6 duration counting
- ✅ **No auto-detection — all OT must be explicitly requested** *(resolved 2026-06-19)* — removes OC-2 auto-detect from attendance records.
- ✅ **No `min_minutes` enforcement** *(resolved 2026-06-19)* — SL/HR determine OT validity, not a system threshold. `min_minutes` removed from `OvertimeRule`.
- ✅ Minimum OT counted = **30 minutes** *(superseded 2026-06-02 — was 60 min in 2026-05-29 review; PRDs were authoritative)* *(superseded 2026-06-19 — `min_minutes` removed entirely)*
- Pre-approval: worked-without-request OT still approvable after the fact (flagged); Holiday tier beats Rest-day when both apply; cross-midnight OT → start date *(default)*
- ✅ **Holiday & weekly-rest operating model (cross-cutting E4·E6·E7·E10)** *(resolved 2026-06-08)* — grounds the 24/7 outsourced blue-collar reality (client sites keep operating on public holidays; agents work them):
  - **D1 — Holiday calendar is classification-only, never suppresses shifts.** The `/holidays` master (**global only** — per-service-line `HolidayCategory` scoping dropped, §8 2026-06-12) is a **date-level classification** consumed by E7 OT day-type resolution (HOLIDAY tier) and E6 working-day exclusion. It does **not** drive schedule generation — the roster still rosters on holidays. The E4 grid surfaces a holiday tint/badge (visual only).
  - **D2 — worked weekly rest day = RestDay OT premium only.** Under PP 35/2021, working the agent's weekly rest day is compensated as **RestDay-tier OT premium** (reference multiplier in v1, no monetary calc). **No TOIL/substitute-rest-day ledger and no conversion into `cuti tahunan`** — the statutory annual-leave accounting stays untouched. Rest day is **per-agent, derived from the roster** (not a fixed Sunday); HOLIDAY beats RESTDAY when both apply (existing E7 precedence).
  - **D3 — rest-day shortfall is a compliance flag, not a balance.** "No rest day in the week" / **>6 consecutive scheduled workdays** raises a **compliance alert to the shift leader** — never a silent leave credit. Computed rolling-consecutive for the legal flag; rendered as a per-week rest indicator on the (week-scoped) E4 grid.
  - **D4 — holiday calendar seeding = HR-confirmed yearly import, not live-sync.** A yearly **"Import {tahun}"** bootstrap (e.g. Nager.Date / `date-holidays`) prefills candidate holidays; **HR reviews/confirms** and adds **cuti bersama**. The **SKB 3 Menteri decree is authoritative** for cuti bersama (APIs lag; Islamic dates shift by rukyat). HR-maintained master stays the source of truth (no live external sync).
  - See [E4 FEATURE], [E6 FEATURE], [E7 FEATURE §6b holiday calendar], [E10 notifications].

**E8 — Payroll (historical archive + compute-assist runs)**
- Payslips **view-only** for agents (PDF download later); historical payroll **immutable** (HR annotate via audit note); retention indefinite pending compliance input *(default)*
- ✅ **Active compute-assist payroll, flipped from data-only** *(resolved 2026-06-11)* — v1 was "no active runs"; now E8 **also** runs a monthly payroll. The system **assembles** each agent's draft pay from authoritative upstream data (E2 base salary, E5 **verified** attendance, E6 leave, E7 **approved** OT), HR **reviews/adjusts editable component lines**, then **posts immutable payslips** (F8.3). Payment is **manual** — HR transfers in their own bank channel and records the reference + **uploaded bukti transfer** (F8.4, INV-8); **no bank/BPJS/tax API**. Rationale + flow in [E8 FEATURE](epics/E8-payroll/FEATURE.md).
  - ✅ **Agent pay = monthly wage, not billable hours.** Two distinct money flows: **client billing** (revenue, **hours-only**, rates applied outside — E5→E10) vs **agent payroll** (cost, **monthly** `EmploymentAgreement.base_salary` — E8). Payroll does **not** consume billable hours; attendance/OT/leave only **modulate** the monthly base. Consistent with the 2026-06-07 "base salary single-source on E2" lock and the legacy monthly `gaji_pokok` model.
  - ✅ **No statutory auto-engine in v1** — BPJS/PPh21 + allowances are **editable component lines** (optionally config-prefilled), not a calculator. A full statutory/payroll-tax engine is a future epic.
  - ✅ **Immutable posted payslips + carry-forward.** A posted payslip never changes; **late-verified** E5/E7 changes after a run's cutoff accrue as a **`PayrollAdjustment`** consumed by the **next** run (prior-period adjustment line). Protects audit + matches "historical payroll immutable".
  - ✅ **OT pay** = approved hours × `OvertimeRule.multiplier` × hourly base; **hourly base = base_salary / 173** *(default, configurable — Permenaker)*. Finance sub-role **none** (HR runs payroll).
  - **Open:** proration divisor (calendar vs working-day, *default calendar*); re-open posted run before payment (*default no*); statutory config prefill depth.
  - See [E8 FEATURE](epics/E8-payroll/FEATURE.md), [F8.3 payroll-run](epics/E8-payroll/prds/payroll-run.md), [F8.4 payroll-payment](epics/E8-payroll/prds/payroll-payment.md).
- ✅ **Two pay models by `employee_type`: FIELD is attendance-gated, INTERNAL is fixed monthly salary** *(resolved 2026-06-16)* — cross-cutting (E4 scheduling · E5 attendance · E6 leave · E8 payroll). Builds on the FIELD/INTERNAL split (E2, 2026-06-15). Both draw a monthly `EmploymentAgreement.base_salary`, but attendance modulates it **only for FIELD**:
  - **A — FIELD = attendance-gated.** A field agent's take-home is modulated by **payable worked days** — the E5 per-day `is_payable` flag (auto-true on shift days, non-payable on absent/no-shift days) drives proration. This is **why FIELD needs a per-day roster** (the shift produces the payable day).
  - **B — INTERNAL = fixed monthly salary.** Internal staff (white-collar, gaji bulanan) are **not** attendance-gated: daily attendance does not move take-home. Their attendance is a **presence/discipline record, never a pay input**. The per-day **`is_payable` flag is a FIELD-ONLY mechanic** — payroll **ignores it for INTERNAL**.
  - **C — no per-day shift for INTERNAL pay.** Internal employees need **no shift/roster** for payroll; pay is sourced straight from the E2 base salary, not worked-day counting. *(A fixed weekly office-hours pattern may be added later purely for lateness / OT-discipline tracking — open, and pay-independent; per-day rostering is never required for INTERNAL.)*
  - **D — INTERNAL take-home (E8) = base salary − unpaid leave + OT.** The only attendance-independent adjustments: paid leave never deducts; **unpaid** leave (`paid=false` types, e.g. `CLTP`, E6) deducts; approved OT (E7) adds. OT for INTERNAL uses the **manual-request** path (no auto-detect dependency, since INTERNAL may have no shift end).
  - **E — reporting.** INTERNAL is already non-billable (`Company.type = CLIENT` filter, 2026-06-15) and is now also **non-attendance-payable**.
  - **Reconciliation:** E8 INV-6 (monthly base, attendance modulates) holds **for FIELD**; add that INTERNAL proration is **salary-fixed** (only unpaid-leave/OT lines apply). E5 — `is_payable` is FIELD-scoped. E4 — no roster requirement for INTERNAL.
  - See [E8 FEATURE INV-6](epics/E8-payroll/FEATURE.md), [E5 FEATURE], [EPICS §8 E2 "FIELD vs INTERNAL"].
- ✅ **Payroll Period Close — a month-end reconciliation gate (F8.5)** *(resolved 2026-06-17)* — a new **unified `PayrollPeriod`** per `(year, month)` gates all three pay upstreams (E5 attendance + E7 overtime + E6 leave) in one month-end lock, replacing the prior record-by-record reconciliation with no "the month is done" signal. HR reviews a 3-tab cockpit (Attendance · Overtime · Leave), resolves every blocker via the **existing** actions (E5 F5.3 verify / F5.4 correct / F5.6 manual / set-payable / F5.2 auto-close; E7 OT approve; E6 leave approve), then **locks**. State machine `OPEN → LOCKED` (+ super-admin `REOPENED → OPEN`). See [F8.5 payroll-period-close](epics/E8-payroll/prds/payroll-period-close.md), [E8 FEATURE F8.5](epics/E8-payroll/FEATURE.md).
  - ✅ **Global monthly period (v1)** — one period for all companies; **per-company partial close deferred** to v2 (lock each client independently, month flips when all done).
  - ✅ **Lock emits an immutable `PeriodEmployeeSummary`** per FIELD employee (attendance + OT + leave figures) — the authoritative payroll input the F8.3 run reads, **not** live records. Locking stamps `locked_by` / `locked_at`; super-admin **force-lock** with a required reason overrides a non-zero blocker count (audited).
  - ✅ **Post-lock changes never mutate history → `PayrollAdjustment`.** A legal correction (F5.4) or late approval on a locked month leaves the summary untouched and emits a `PayrollAdjustment` (origin = the locked period) carried to the next open run — **reuses the F8.3 PR-13 carry-forward mechanism**, no new ledger.
  - ✅ **FIELD gates the lock; INTERNAL is informational** — blocking applies only to FIELD employees (pay is attendance-gated); INTERNAL staff appear in the completeness view as discipline info and **never block** lock (mirrors the FIELD-only `is_payable`, 2026-06-16).
  - ✅ **Clarification = thin, notification-backed, one round** — HR raises a `ClarificationRequest` on a specific attendance/OT/leave record, auto-targeted to the record's shift leader (else the agent), delivered as an E10 notification. State `OPEN → ANSWERED → RESOLVED` (or `CANCELLED`). **Not** an E11 approval line and **not** a chat thread; blocks nothing on its own but surfaces in the cockpit as "awaiting reply." HR may re-ask.
  - ✅ **F5.7 auto-reconcile is inert on a `LOCKED` period** (PC-11) — a late schedule-create still inserts the shift but must not silently alter locked figures; reconcile runs only against `OPEN`-period records.
  - ✅ **Payroll run requires a `LOCKED` period (PC-10 seam).** The F8.3 run for a month reads its `PeriodEmployeeSummary`, not live records; reopen is super-admin only and **rejected once any run for the month is `Posted`**. *(F8.3 compute is still unbuilt — this feature ships the guard + the summary/adjustment tables it will consume; until then they simply accumulate.)*
  - **Open:** per-company partial close (v2); adjustment **valuation** owner (lean: the F8.3 run values `amount`, v1 writes the event with `amount = null`); OT/leave tab depth (v1 surfaces *pending* as blockers + links to existing approval screens, no inline approve UI).

**E9 — Migration**
- ✅ **Sites are net-new, with a migration shim** *(2026-06-03)* — no geofence/site data is migrated from legacy (`companies.role=2` geo is **not** carried over; `role=4` sub-companies stay ignored). To satisfy the required `Placement.site_id`, the loader **auto-creates one primary "Main Site" per migrated ClientCompany** (geofence empty). Migrated placements attach to it; `leader_scope` defaults to `company`. HR configures geofences and splits multi-site companies **post-cutover**.
- ✅ Migrate **everything**, incl. full attendance history (plan a larger migration + validation window)
- ✅ **v1: one-shot script, no UI** *(resolved 2026-06-02)* — big-bang cutover, no human-in-the-loop review queue. Blocking items (`decrypt_fail`, `orphan_identity`, `unmatched_placement`) handled via pre-built alias tables + hardcoded crosswalks in the migration script, OR logged-and-skipped with a post-run report. Inspection via SQL/CLI on staging DB if needed.
- Placement-string matching = exact + alias list (no fuzzy + manual confirm in v1; if exact+alias miss, log and skip). Keep `lumen_swp` read-only ~6–12 months post-cutover. Maintenance window + validation thresholds sized by dry-runs *(default)*

**E10 — Reporting & Notifications**
- ✅ Billable = **verified-only** (consistent with E5)
- Notifications all-on in v1 (mute non-critical later); billing math = **hours only** (rates applied outside the system) *(default)*
- ✅ **Super Admin dashboard = HR cockpit + admin-only widgets (extends the dashboard D1 "same body, distinct label")** *(resolved 2026-06-11)* — the `super_admin` dashboard now **adds** an admin-only section to the shared `HrDashboard` payload, making it a **superset** rather than a relabel. Four widgets, all `super_admin`-scoped, each deep-linking into its owning epic: **(a) User & access** (active users · accounts pending provisioning · offboarded/disabled ≤30d — E2/F2.7); **(b) Recent audit feed** (last sensitive actions — E1 audit); **(c) Org rollups by position** (headcount + active placements grouped by the free-text position — E3); **(d) Pending grants** (role-change requests awaiting super-admin action; bank-account approval escalations removed 2026-06-14 — profile edits are instant). Carried on `HrDashboard.admin` (present **only** for `super_admin`); the `hr_admin` payload is unchanged. See [F10.2 PRD DB-7], [E10 `openapi` `HrDashboard.admin`].
- ✅ **Shift-leader dashboard is dual-surface (web + mobile)** *(resolved 2026-06-11)* — the existing `LeaderDashboard` payload now also backs a **mobile Beranda** (the shift leader is on-site, phone-first), at parity with the web team dashboard. **No new endpoint** — `GET /dashboards/me` already returns `LeaderDashboard` for `shift_leader`; this adds the mobile surface only. See [F10.2 PRD DB-8], [`.pen` frame `UMzuO`].
- ✅ **Agent Web Calendar (F10.5)** *(resolved 2026-06-19)* — to be implemented. A web-accessible calendar view for agents to see their schedule, attendance, leave, and overtime in one calendar surface. Part of the agent web self-service console (E10).

**E11 — Approvals (configurable multi-line engine, cross-cutting)**
- ✅ **Approval becomes a per-company configurable engine; profile change-requests are removed** *(resolved 2026-06-14 — new epic E11)* — replaces the three hardcoded approval flows with one data-driven engine. Cross-cutting (E2 profile · E6 leave · E7 overtime · E1 RBAC · E10 inbox/notifications).
  - **A — profile change-request approval is removed entirely.** The E2 change-request feature (F2.1 EP-5/EP-5b/EP-5c/EP-5d, `SWP-CHG`, `change_requests.*` incl. `.approve.bank`) is **hard-deleted**. Agent profile edits to **phone, emergency contact, and bank account become instant self-edit** (apply immediately, still audited) — joining photo/address/language in the instant tier. There is no approval queue for profile edits. Supersedes the 2026-06-11 agent-editable-field-tiers decision (the approval tier collapses to instant).
  - **B — one approval template per company.** HR admin + super admin configure a single **ApprovalTemplate** per client company (`approvals.template.manage`). A template has **2 or 3 ordered lines**; **line 3 is optional to configure** (typically a super-admin sign-off), so the minimum is **2 lines**. Each line holds **one or more users** as an **OR-set**: any one member approving satisfies that line; the others need not act. Lines are **sequential** — line 1 must clear before line 2, etc.
  - **C — line membership decides routing, not static roles.** Eligibility to approve a *specific* request is **line membership on that request's chain** (data, server-enforced), **not** a `*.approve` permission. This **supersedes** the 2026-06-12 lead-role point C ("lead is L2/final approver") and the `shift_leader` "L1 approver" model for leave/overtime: those roles still exist for their other duties and may be **placed on lines**, but approval no longer auto-derives from role/placement. Supersedes leave **LA-1/LA-2/LA-3** and overtime **OA-1/OA-2/OA-3**.
  - **D — super-admin bypass.** A super admin may **force-approve an entire request from any state** (`approvals.bypass`), skipping all remaining lines even if not assigned to any line; a **reason is required** and recorded as a `BYPASS` action. (Replaces the prior HR override, LA-8/OA-8.)
  - **E — sequential advance + terminal reject.** Any current-line member **approves** → advance to the next line; clearing the **last configured line** → request `APPROVED`, which fires the request type's **side-effect hook** (leave: commit per-type quota reservation + INV-3 schedule integration, F6.1/F6.4; overtime: count hours by `day_type`, F7.4). Any current-line member **rejects** → request `REJECTED` (terminal, reason required).
  - **F — self-approval is blocked, no auto-skip.** If the requester is a member of a line in their own request's chain, they **cannot** approve it; **another member of that line must act**. If the requester is the line's **sole** member, the line can only be cleared by **super-admin bypass** (D).
  - **G — no template ⇒ super-admin fallback.** If a company has **no** template configured, a submitted request routes to a **single implicit super-admin line** (never blocks the agent, never auto-approves). When a template is later created, pending fallback instances are pulled onto it by the live-reset rule (H).
  - **H — live template, with pending reset.** Editing a template **bumps its version** and **resets all non-terminal instances** for that company to **line 1** on the new chain (prior actions kept as audit, stamped with the old version, no longer counted) and re-notifies the new line-1 members. (No per-instance snapshot.)
  - **I — generic engine, per-type hooks.** The engine is request-type-agnostic; **leave and overtime opt in** in v1, future types register an `OnApproved`/`OnRejected` side-effect hook. Replaces the `leave_approvals` + `overtime_approvals` decision trails with one **`approval_actions`** append-only trail.
  - **J — line members = any active SWP staff user**; template save validates members are active (employment not ended). An offboarded mid-flight member is handled by super-admin bypass (D) + HR re-editing the template (H).
  - See [E11 FEATURE](epics/E11-approvals/FEATURE.md), [E11 PRDs](epics/E11-approvals/), [E11 openapi](api/E11-approvals/openapi.yaml), [E6 leave-approval], [E7 overtime-approval], [E2 employee-profile EP-5], [NAVIGATION-AND-RBAC §4].

---

**Deferred to the build/tech phase (not product):** auth session model (JWT vs server sessions) + token lifetimes + password policy; API pagination style; push provider (FCM/APNs); migration batch sizes/parallelism; audit & export storage/retention infrastructure; dashboard caching/freshness thresholds.
