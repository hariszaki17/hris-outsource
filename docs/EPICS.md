# hris-outsource — Epic Breakdown

> Master decomposition for the from-scratch rebuild of SWP's HRIS, focused on managing outsourced agents.
> Decomposition method: OODA (this brainstorm) + user-story-mapping (epics → features → PRDs/stories).
> Status: brainstorm complete; epics drafted. Next: feature docs per epic, then PRDs per feature.

---

## 1. Context

**Company:** PT Saranawisesa Properindo (SWP) — an outsourcing provider supplying agents across three **service lines**: **Facility Services, Building Management, Parking**. Work is shift-heavy, 24/7, on client sites.

**Why a rebuild (not editing ims-system):** The legacy system (`ims-system`, Laravel Lumen + Next.js, MySQL) is a 200+ table monolith where HRIS is only ~50 tables, the user model is generic multi-tenant, and "placement" is just a string field. The outsource domain (first-class placement, shift leaders, service-line rules) diverges enough that a clean rebuild + data migration beats surgery on the monolith.

**What we keep:** only the HRIS subset. **Migration source:** SWP prod, MySQL DB `lumen_swp`.

---

## 2. Locked decisions

| Area | Decision |
|---|---|
| Tenancy | **Internal only.** Only SWP staff use the system. Client companies are *data*, not tenants/logins. |
| Roles | super admin · HR/placement admin · **shift leader** · agent |
| Placement | An agent placed at a **client company**, in a **service line**, for a **contract period**; tracked with history. |
| Shift leader | **1 per client company/placement.** Verifies attendance, approves OT & leave, manages roster, sees team dashboards. |
| Shifts | A **work-shift master** (working hours + break) is admin-defined; the shift leader **picks from master** to schedule each agent. |
| Service line | **Drives shift & attendance rules** (e.g. Parking = 24/7 rotating, Building Mgmt = office hours). |
| Payroll | **Data-only** in v1 — migrate for history/reporting; no active payroll runs. |
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
   │            │  (service line, period, status, history)
   │            └── ShiftLeaderAssignment (1 leader per company)
   │
Schedule (agent + ShiftMaster + date)  ← shift leader builds from ShiftMaster catalog
   │
Attendance (vs Schedule) · Leave (vs Quota) · Overtime (vs Schedule)
   │
Payroll history (read-only, migrated)
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
**Features:** Employee/Agent profile · Client/Partner Company directory · Service Lines · Positions · Leave types · Attendance codes/policies · Overtime rules.
**Entities:** Employee, ClientCompany, ServiceLine, Position, LeaveType, AttendanceCode, OvertimeRule.
**Depends on:** E1

### E3 — Placement Management ⭐ (differentiator)
**Goal:** place agents at client companies, in a service line, for a period; track history; assign the company's shift leader.
**Features:** Agent placement (contract: agent + company + service line + period + terms) · Placement lifecycle/status · Re-placement & transfer with history · Shift-leader assignment (1 per company) · Company placement roster view.
**Entities:** Placement (≈ legacy `employee_contract`), ShiftLeaderAssignment.
**Depends on:** E2

### E4 — Shift Configuration & Scheduling
**Goal:** define shift master templates and let shift leaders schedule agents per placement; service line drives rules.
**Features:** Work-shift master catalog (hours + breaks) · Service-line shift/attendance policies · Roster/schedule builder (leader picks from master, assigns agents to dates) · Rotation patterns · Schedule calendar & publish/notify.
**Entities:** ShiftMaster, ServiceLineShiftPolicy, Schedule, RotationPattern.
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
**Features:** OT request · OT detection vs schedule · Approval workflow · OT calculation (per service-line rules) · OT status · OT reporting.
**Entities:** Overtime, OvertimeStatus.
**Depends on:** E4, E5, E3

### E8 — Payroll Data (historical, read-only)
**Goal:** preserve migrated payroll history for continuity & reporting. **Not** active payroll.
**Features:** Payslip history view · Salary components (read) · Benefits (read) · Payroll reports/exports.
**Entities:** Payslip, EmployeeSalary, SalaryColumn, EmployeeBenefit — read-only.
**Out of scope (v1):** payroll runs, BPJS/PPh21 calculation.
**Depends on:** E2, E9

### E9 — Data Migration (SWP prod → hris-outsource)
**Goal:** transform-and-load everything from MySQL `lumen_swp` into Postgres under the new model; validate & reconcile.
**Features:** Source extraction (MySQL) · Field/entity mapping (legacy role + `company_id` → new model; `placement` string → entity) · Load into Postgres · Identity/role remap · Reconciliation & validation reports · Idempotent re-runs · Cutover runbook.
**v1 mode (resolved 2026-06-02):** **One-shot script — no UI.** Big-bang cutover via full script execution. Blocking items (`decrypt_fail`, `orphan_identity`, `unmatched_placement`) are pre-resolved in code (alias tables + hardcoded crosswalks) or logged + skipped; no human-in-the-loop review queue. Manual inspection if needed via SQL/CLI on the staging DB.
**Depends on:** target schemas from E1–E8 (runs continuously; lands at cutover).

### E10 — Reporting, Exports & Notifications (cross-cutting)
**Goal:** operational reports + notifications across modules.
**Features:** Role-based dashboards · Exports (Excel/PDF) · Notifications (in-app/email: schedule published, approvals pending, attendance anomalies) · Approval inboxes.
**Depends on:** spans E2–E8.

---

## 6. Build sequencing (one release, dependency-ordered)

```
E1 Foundations
  └─ E2 Identity/Org/Master
       └─ E3 Placement ⭐
            └─ E4 Shift & Scheduling
                 └─ E5 Attendance
                      ├─ E6 Leave        (parallel after E3/E2)
                      └─ E7 Overtime     (after E5)
E8 Payroll-data  ─┐
E9 Migration     ─┼─ continuous; E9 lands at cutover, E8/E10 follow data
E10 Reporting    ─┘
```

## 7. Documentation index — all 10 epics drafted ✅

Each epic has a `FEATURE.md` (features + BPMN-style Mermaid workflows) and per-feature `prds/*.md` (user story + Gherkin AC + cases). E2–E8 also have a `DATA-MAPPING.md` (legacy `lumen_swp` → new model); E9 orchestrates those mappings; E10 & E1 read across modules.

| Epic | Feature doc | PRDs | Data map |
|------|-------------|------|----------|
| **E1** Foundations & Platform | [FEATURE](epics/E1-foundations/FEATURE.md) | authentication · rbac-roles · audit-log · platform-conventions | — (uses E2) |
| **E2** Identity, Org & Master | [FEATURE](epics/E2-identity/FEATURE.md) | employee-profile · employment-agreement · client-company-directory · client-sites-geofence · service-lines-positions · operational-master-data | [✓](epics/E2-identity/DATA-MAPPING.md) |
| **E3** Placement ⭐ | [FEATURE](epics/E3-placement/FEATURE.md) | agent-placement · placement-lifecycle · replacement-transfer · shift-leader-assignment · company-roster | [✓](epics/E3-placement/DATA-MAPPING.md) |
| **E4** Shift & Scheduling | [FEATURE](epics/E4-shift-scheduling/FEATURE.md) | shift-master-catalog · daily-schedule-assignment · schedule-views · schedule-changes-swaps | [✓](epics/E4-shift-scheduling/DATA-MAPPING.md) |
| **E5** Attendance | [FEATURE](epics/E5-attendance/FEATURE.md) | clock-in-out · attendance-evaluation · attendance-verification · attendance-corrections · attendance-records | [✓](epics/E5-attendance/DATA-MAPPING.md) |
| **E6** Leave | [FEATURE](epics/E6-leave/FEATURE.md) | leave-quota-balances · leave-request · leave-approval · leave-schedule-integration · leave-calendar-views | [✓](epics/E6-leave/DATA-MAPPING.md) |
| **E7** Overtime | [FEATURE](epics/E7-overtime/FEATURE.md) | overtime-rules · overtime-capture · overtime-approval · overtime-records | [✓](epics/E7-overtime/DATA-MAPPING.md) |
| **E8** Payroll (read-only) | [FEATURE](epics/E8-payroll/FEATURE.md) | payslip-history · payroll-archive | [✓](epics/E8-payroll/DATA-MAPPING.md) |
| **E9** Data Migration *(script-only, no UI v1)* | [FEATURE](epics/E9-migration/FEATURE.md) | extraction-staging · transform-crosswalks · reconciliation-review · load-idempotent · cutover-validation | (orchestrates E2–E8) |
| **E10** Reporting & Notifications | [FEATURE](epics/E10-reporting/FEATURE.md) | notifications · dashboards · attendance-billable-report · export-framework | — (reads modules) |

**Totals:** 10 epics · 44 PRDs · 10 feature docs · 7 data-mapping docs.

> Folder layout: `docs/epics/<E#-name>/FEATURE.md` + `prds/<feature>.md` (+ `DATA-MAPPING.md` for E2–E8).

---

## 8. Resolved product decisions — open-items review (2026-05-29)

> Authoritative decision log from the open-items review. Where this conflicts with a per-epic `FEATURE.md` §7 "Still open" list, **this section wins** (per-epic docs reconciled progressively). ✅ = explicitly chosen; *(default)* = sensible default applied — override anytime.

**E1 — Foundations**
- MFA: not in v1 (admin/HR hardening later) *(default)*
- ✅ **Every employee auto-provisions a login at create; phone is the primary login identifier** *(resolved 2026-06-07)* — supersedes the prior "email-less agents: assign an email at provisioning (login stays email+password)" default. Login no longer requires email; the login identifier is the agent's **phone number** (required) or **email** (optional). See the cross-cutting auto-provision decision below.
- Role assignment: super_admin + hr_admin may assign roles *(default)*
- Finance sub-role: none in v1 (HR sees payroll); IDR-only, no multi-currency; bulk actions audited per affected row *(default)*

**E2 — Identity, Org & Master**
- ✅ **Client sites = first-class entity (reverses "flat, no sub-sites")** *(resolved 2026-06-03)* — a ClientCompany has **one or more `Site`s** (placement locations). Geofence config (**address, lat/lng, `geofence_radius_m` default 100m**) moves **off ClientCompany onto `Site`**. This supersedes the 2026-05-29 "flat (no sub-sites)" decision and DATA-MAPPING G-6's "ignore sub-companies". New feature **F2.6 Client Sites & Geofence**.
- ✅ **Placement targets a Site** *(2026-06-03)* — `Placement.site_id` is **required**; every company has **≥1 Site** (single-location companies get one auto **primary "Main Site"**). E5 geofence always resolves from `placement.site` (no company fallback).
- ✅ **Geofence model = single circle per site** *(2026-06-03)* — one center (lat/lng) + radius; out-of-geofence stays **allowed + flagged** (E5). Multi-circle / polygon are post-v1.
- ✅ **Shift-leader scope = configurable per company** *(2026-06-03)* — `ClientCompany.leader_scope ∈ {company, site}` (default `company`). `company` → one leader for the whole company (today's behavior); `site` → one leader **per site**. Rewrites E3 INV-2/3/4 to a "leadership unit" (company **or** site). See [E3 FEATURE §4].
- Agent mobile-editable fields: phone, address, bank (HR-approved) *(default)*
- Service lines: the 3 are seeded but **admin-extendable** *(default)*
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
  - **D — service-line maintenance consolidated on the detail page.** Renaming a service line **and** adding/updating/removing its **positions** all happen on the service-line **detail page**; the service-line list's "Edit" action now **navigates to that detail page** instead of opening a rename-only modal.
  - See [E2 F2.3 PRD], [E2 F2.4 PRD], [E2 FEATURE §F2.3/F2.4].

**E4 — Shift & Scheduling**
- ✅ Agent shift-swap / day-off requests: **deferred to post-v1** (v1 = leader-driven schedule edits only; F4.4 swaps drop from v1 scope)
- ✅ Scheduling over approved leave: **blocked**
- ✅ **One shift per agent per day** (no split shifts)
- Shift reminder = evening-before + ~1h prior; bulk "apply shift to date range" helper included; schedule horizon unbounded *(default)*

**E5 — Attendance**
- ✅ Per-site geofence (100m default) · ✅ late grace **15 min** · ✅ unscheduled clock-in **allowed + flagged** · ✅ **online-only** for v1
- ✅ Billable counts **verified records only**
- ✅ Leaders' own exception records **escalate to HR** (no self-verify)
- ✅ Agent self-correction window **7 days** (older = HR only)
- ✅ Anti-spoofing (mock GPS): **post-v1**
- Early clock-out flagged if >15 min early; no auto-verify SLA (pending stays + reminder) *(default)*

**E6 — Leave**
- ✅ Duration = **working days excluding public holidays** (working day = a day the agent would otherwise be rostered — exact shift-worker nuance to confirm)
- ✅ Period basis = **calendar year**
- ✅ Probation: **pro-rated** annual leave in the first year (also pro-rate mid-year joiners)
- ✅ Non-annual types (sick/maternity/unpaid): **per-type quotas** → `LeaveQuota` generalizes to one per (employee, leave_type, period)
- ✅ Half-day leave: **not in v1** (full days only)
- Delegate = informational/notified (no enforced coverage); no-leader → HR sole approver; team calendar shows approved + a pending toggle *(default)*

**E7 — Overtime**
- ✅ Public-holiday calendar: **HR-maintained in-app** master (recurring + one-off); shared with E6 duration counting
- ✅ Auto-detected OT: **agent confirms, then leader approves**
- ✅ Minimum OT counted = **30 minutes** *(superseded 2026-06-02 — was 60 min in 2026-05-29 review; PRDs were authoritative)*
- Pre-approval: worked-without-request OT still approvable after the fact (flagged); Holiday tier beats Rest-day when both apply; cross-midnight OT → start date *(default)*

**E8 — Payroll (read-only)**
- Payslips **view-only** in v1 (PDF download later); historical payroll **immutable** (HR annotate via audit note); retention indefinite pending compliance input *(default)*

**E9 — Migration**
- ✅ **Sites are net-new, with a migration shim** *(2026-06-03)* — no geofence/site data is migrated from legacy (`companies.role=2` geo is **not** carried over; `role=4` sub-companies stay ignored). To satisfy the required `Placement.site_id`, the loader **auto-creates one primary "Main Site" per migrated ClientCompany** (geofence empty). Migrated placements attach to it; `leader_scope` defaults to `company`. HR configures geofences and splits multi-site companies **post-cutover**.
- ✅ Migrate **everything**, incl. full attendance history (plan a larger migration + validation window)
- ✅ **v1: one-shot script, no UI** *(resolved 2026-06-02)* — big-bang cutover, no human-in-the-loop review queue. Blocking items (`decrypt_fail`, `orphan_identity`, `unmatched_placement`) handled via pre-built alias tables + hardcoded crosswalks in the migration script, OR logged-and-skipped with a post-run report. Inspection via SQL/CLI on staging DB if needed.
- Placement-string matching = exact + alias list (no fuzzy + manual confirm in v1; if exact+alias miss, log and skip). Keep `lumen_swp` read-only ~6–12 months post-cutover. Maintenance window + validation thresholds sized by dry-runs *(default)*

**E10 — Reporting & Notifications**
- ✅ Billable = **verified-only** (consistent with E5)
- Notifications all-on in v1 (mute non-critical later); billing math = **hours only** (rates applied outside the system) *(default)*

**Deferred to the build/tech phase (not product):** auth session model (JWT vs server sessions) + token lifetimes + password policy; API pagination style; push provider (FCM/APNs); migration batch sizes/parallelism; audit & export storage/retention infrastructure; dashboard caching/freshness thresholds.
