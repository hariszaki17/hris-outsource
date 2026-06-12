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
| Placement | An agent placed at a **client company**, in a **service line**, for a **contract period** (the employment agreement may be backfilled — optional at create, §8 2026-06-11); tracked with history. |
| Shift leader | **1 per client company/placement.** Verifies attendance, approves OT & leave, manages roster, sees team dashboards. |
| Shifts | A **work-shift master** (working hours + break) is admin-defined; the shift leader **picks from master** to schedule each agent. |
| Service line | **Drives shift & attendance rules** (e.g. Parking = 24/7 rotating, Building Mgmt = office hours). |
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
   │            │  (service line, period, status, history)
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
**Features:** Employee/Agent profile · Client/Partner Company directory · Service Lines · Positions · Leave types · Attendance codes/policies · Overtime rules.
**Entities:** Employee, ClientCompany, ServiceLine, Position, LeaveType, AttendanceCode, OvertimeRule.
**Depends on:** E1

### E3 — Placement Management ⭐ (differentiator)
**Goal:** place agents at client companies, in a service line, for a period; track history; assign the company's shift leader.
**Features:** Agent placement (agent + company + service line + period; employment agreement may be backfilled — optional at create, see §8 2026-06-11) · Placement lifecycle/status · Re-placement & transfer with history · Shift-leader assignment (1 per company) · Company placement roster view.
**Entities:** Placement (≈ legacy `employee_contract`), ShiftLeaderAssignment.
**Depends on:** E2

### E4 — Shift Configuration & Scheduling
**Goal:** define shift master templates and let shift leaders schedule agents per placement; service line drives rules.
**Features:** Work-shift master catalog (hours + breaks) · Service-line shift/attendance policies · Roster/schedule builder (leader picks from master, assigns agents to dates) · Rotation patterns · Schedule calendar & publish/notify · **Roster-compliance indicators** (holiday-shift badge + holiday day-column tint; missing-weekly-rest flag; >6-consecutive-workday cap warning) — derives from roster + `/holidays`, surfaces to the shift leader (§8 D1/D3).
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
**Features:** OT request · OT detection vs schedule · Approval workflow · OT calculation (per service-line rules) · OT status · OT reporting · **Holiday calendar bootstrap** ("Import {tahun}" prefill → HR confirm + cuti bersama, §8 D4) · **statutory OT multiplier seeding** (PP 35/2021 defaults: workday 1.5×→2×; rest-day/holiday progressive 2×/3×/4×, stored as reference).
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
**Features:** Role-based dashboards · Exports (Excel/PDF) · Notifications (in-app/email: schedule published, approvals pending, attendance anomalies, **profile change-request approved/rejected** — reject carries the reason, F2.1 EP-5c) · **shift-leader compliance notifications** (agent assigned a holiday shift; agent with no weekly rest / >6 consecutive workdays — §8 D3) · Approval inboxes.
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
| **E8** Payroll (history + compute-assist) | [FEATURE](epics/E8-payroll/FEATURE.md) | payslip-history · payroll-archive · payroll-run · payroll-payment | [✓](epics/E8-payroll/DATA-MAPPING.md) |
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
- ✅ **Agents get a web self-service console (reverses "agent is mobile-only")** *(resolved 2026-06-10)* — the `agent` role may now sign in to the **web console** (`apps/web`) and reach a self-service surface at parity with the mobile app (dashboard, **web clock-in/out**, own schedule, own attendance + corrections, leave, overtime, profile, payslip, notifications). This **supersedes** NAVIGATION-AND-RBAC.md §4's "`agent` — none (mobile-only)". It is **frontend-only**: every clock + self endpoint already declares `x-rbac: { roles:[agent], scope:self }` and resolves the agent from the JWT principal. Mechanics: agent routes under the **`/me/*`** prefix; new **`self.*` capability keys** form the `agent` bundle; `agent` joins `WEB_ROLES`; the shell selects the nav backbone by role. **Does not change internal-only tenancy** (agents are SWP staff, not clients — the client portal stays a separate, unratified concern). G0 exception (no agent web `.pen` frames; built pragmatically from `packages/ui` + mobile frames) is recorded in the spec. See [docs/eng/AGENT-WEB-ACCESS.md](eng/AGENT-WEB-ACCESS.md).

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
- ✅ **Service-line create may seed initial positions atomically** *(resolved 2026-06-08)* — `POST /service-lines` accepts an **optional `positions` array**; when provided, the line **and all its positions** are created in a **single all-or-nothing transaction** — if any position is invalid or duplicates another within the line, **nothing is persisted** (no line, no positions). Per-line name uniqueness (F2.4 SP-3) is enforced **across the batch**, not just against already-stored rows. The web "Tambah Lini Layanan" modal supports adding initial positions inline; the dedicated `POST /service-lines/{id}/positions` endpoint still exists for adding positions later from the detail page. No domain/invariant or soft-delete change. See [E2 F2.4 PRD], [E2 FEATURE §F2.4].

**E3 — Placement & Shift-Leader Assignment**
- ✅ **Shift-leader identity is fully assignment-driven; role + scope are derived, never stored** *(resolved 2026-06-08)* — cross-cutting (E1 auth · E2 client-companies · E3 leader assignment · E4 schedule). A "shift leader" is **not** a separately-managed auth role:
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

**E4 — Shift & Scheduling**
- ✅ Agent shift-swap / day-off requests: **deferred to post-v1** (v1 = leader-driven schedule edits only; F4.4 swaps drop from v1 scope)
- ✅ Scheduling over approved leave: **blocked**
- ✅ **One shift per agent per day** (no split shifts)
- Shift reminder = evening-before + ~1h prior; bulk "apply shift to date range" helper included; schedule horizon unbounded *(default)*
- ✅ **Shift-master time edits propagate to unrealized schedule entries; frozen by attendance (start@check-in, end@check-out)** *(resolved 2026-06-09)* — when a shift master's `start_at`/`end_at` is edited, the change propagates to all `Schedule` entries with `work_date >= today`, `status != Off`, and not leave-cancelled — but only the unrealized portion: `start_time` freezes when the agent checks in; `end_time`/`cross_midnight` freezes when the agent checks out. An entry with no attendance follows the master live. An entry checked-in-but-not-out has `start_time` frozen while `end_time` still tracks master edits (the open attendance record's shift-end window also updates so lateness/early/auto-close use the live end). An entry with both check-in and check-out is fully frozen. Break times are master-only and not stored on entries. Rationale: each time value freezes exactly when it becomes operationally real (start is judged at check-in, end at check-out); fixes the UI mismatch where a schedule cell showed a stale snapshot while the shift picker showed the current master time. See [E4 FEATURE §4 INV-5], [F4.2 SA-10 + C-7/C-8], [F5.1 CI-9], [E5 FEATURE INV-6].

**E5 — Attendance**
- ✅ Per-site geofence (100m default) · ✅ late grace **15 min** · ✅ unscheduled clock-in **allowed + flagged** · ✅ **online-only** for v1
- ✅ Billable counts **verified records only**
- ✅ Leaders' own exception records **escalate to HR** (no self-verify)
- ✅ Agent self-correction window **7 days** (older = HR only)
- ✅ Anti-spoofing (mock GPS): **post-v1**
- Early clock-out flagged if >15 min early; no auto-verify SLA (pending stays + reminder) *(default)*

**E6 — Leave**
- ✅ Duration = **working days excluding public holidays** (working day = a day the agent would otherwise be rostered — exact shift-worker nuance to confirm)
- ✅ Period basis = **calendar year** *(superseded 2026-06-08 — period is now per-lot; an ANNUAL lot's `expires_at` = the entitlement period end, but balance is no longer keyed on a global calendar-year period. See the grant-lot decision below.)*
- ✅ Probation: **pro-rated** annual leave in the first year (also pro-rate mid-year joiners)
- ✅ Non-annual types (sick/maternity/unpaid): **per-type quotas** → `LeaveQuota` generalizes to one per (employee, leave_type, period) *(superseded 2026-06-08 — leave_type is no longer a balance axis; there is ONE pool per employee held as grant-lots. Long/statutory types = HR pre-funds an earmarked lot. See the grant-lot decision below.)*
- ✅ Half-day leave: **not in v1** (full days only)
- Delegate = informational/notified (no enforced coverage); no-leader → HR sole approver; team calendar shows approved + a pending toggle *(default)*
- ✅ **Leave balance = per-employee grant-lot ledger** *(resolved 2026-06-08 — supersedes the per-type-quota / calendar-year-expiry model above and E6 FEATURE INV-1/INV-4)* — replaces "one `LeaveQuota` row per (employee, leave_type, calendar-year) expiring at year-end" with a single **per-employee pool held as grant-lots**:
  1. **One pool per employee.** `leave_type` stays only as a **label + document gate (`requires_document`) + calendar color** — it is **no longer a balance axis**. All ordinary types draw the one pool.
  2. **Grants are lots** (`leave_grants`, prefix `SWP-LG-*`): one row per insert, each with its own `expires_at`. Columns: `id, employee_id, amount_days, granted_at, effective_from, expires_at, source (ANNUAL|ADJUSTMENT|MATERNITY|STATUTORY|MIGRATION|BONUS), earmark (nullable — null = general pool; non-null = purpose code restricting consumption), remark, consumed_days, pending_days, created_by, created_at/updated_at`. Remaining-per-lot = `amount − consumed − pending` (derived).
  3. **Hard per-lot expiry, no carryover.** A lot expires at its own `expires_at` (an expiry sweep zeroes it). No year-end global expiry, no carryover minting.
  4. **Consumption = FIFO by soonest `expires_at`**, across lots, recorded per-lot in `leave_consumptions` (prefix `SWP-LC-*`): `id, leave_request_id (FK), grant_id (FK), days, created_at`. This replaces the single `balance_quota_id` snapshot on leave_requests. Cancel/restore reverses the exact consumption rows.
  5. **No negative balance.** A request consumes only available (unexpired, matching-earmark) lots. Over-quota → HR adds a lot (pre-fund), never a negative balance. (LQ-5 kept, enforced at allocation.)
  6. **Long / statutory leave = HR pre-funds a lot.** e.g. maternity: HR inserts an earmarked lot (`source=MATERNITY, earmark=MATERNITY, remark, expires_at`); the employee then requests against it. No bypass flag, no separate table.
  7. **Optional earmark.** Unearmarked lots = the flat pool, drawn FIFO by ordinary requests. Earmarked lots are consumed **only** by a request of that purpose and are invisible to ordinary FIFO. Balance UI shows: total pool (unearmarked) + a separate line per earmarked lot with its expiry. Balance = Σ(`amount − consumed − pending`) over lots where `now < expires_at`, split unearmarked-vs-earmarked.
  - **Invariant changes:** LQ-1 (per-type yearly grant) **replaced** — entitlement is granted as lots; the annual auto-grant still sources `employment_agreements.annual_leave_entitlement_days` but writes a single `ANNUAL` lot with `expires_at` = period end. LQ-4 (year-end expire / no carryover) **replaced** by per-lot hard expiry. LQ-5 (never negative) **kept**, enforced at allocation. LQ-7 (one quota per type/period) **dropped** (lots, not per-type rows). LQ-2/LQ-3 (deduct/restore on approve/cancel) **restated** as FIFO consumption rows (reserve `pending_days` at submit, commit to `consumed_days` on approve, release on reject/cancel). LQ-6 (HR manual adjust w/ reason, audited) **becomes** "HR grants/adjusts a lot (amount, `expires_at`, earmark, remark), audited."
  - See [E6 FEATURE §4 + §7](epics/E6-leave/FEATURE.md), [leave-quota-balances PRD](epics/E6-leave/prds/leave-quota-balances.md), [E6 openapi](api/E6-leave/openapi.yaml).

**E7 — Overtime**
- ✅ Public-holiday calendar: **HR-maintained in-app** master (recurring + one-off); shared with E6 duration counting
- ✅ Auto-detected OT: **agent confirms, then leader approves**
- ✅ Minimum OT counted = **30 minutes** *(superseded 2026-06-02 — was 60 min in 2026-05-29 review; PRDs were authoritative)*
- Pre-approval: worked-without-request OT still approvable after the fact (flagged); Holiday tier beats Rest-day when both apply; cross-midnight OT → start date *(default)*
- ✅ **Holiday & weekly-rest operating model (cross-cutting E4·E6·E7·E10)** *(resolved 2026-06-08)* — grounds the 24/7 outsourced blue-collar reality (client sites keep operating on public holidays; agents work them):
  - **D1 — Holiday calendar is classification-only, never suppresses shifts.** The `/holidays` master (global, with optional per-service-line `HolidayCategory` scoping already in the E7 spec) is a **date-level classification** consumed by E7 OT day-type resolution (HOLIDAY tier) and E6 working-day exclusion. It does **not** drive schedule generation — the roster still rosters on holidays. The E4 grid surfaces a holiday tint/badge (visual only).
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

**E9 — Migration**
- ✅ **Sites are net-new, with a migration shim** *(2026-06-03)* — no geofence/site data is migrated from legacy (`companies.role=2` geo is **not** carried over; `role=4` sub-companies stay ignored). To satisfy the required `Placement.site_id`, the loader **auto-creates one primary "Main Site" per migrated ClientCompany** (geofence empty). Migrated placements attach to it; `leader_scope` defaults to `company`. HR configures geofences and splits multi-site companies **post-cutover**.
- ✅ Migrate **everything**, incl. full attendance history (plan a larger migration + validation window)
- ✅ **v1: one-shot script, no UI** *(resolved 2026-06-02)* — big-bang cutover, no human-in-the-loop review queue. Blocking items (`decrypt_fail`, `orphan_identity`, `unmatched_placement`) handled via pre-built alias tables + hardcoded crosswalks in the migration script, OR logged-and-skipped with a post-run report. Inspection via SQL/CLI on staging DB if needed.
- Placement-string matching = exact + alias list (no fuzzy + manual confirm in v1; if exact+alias miss, log and skip). Keep `lumen_swp` read-only ~6–12 months post-cutover. Maintenance window + validation thresholds sized by dry-runs *(default)*

**E10 — Reporting & Notifications**
- ✅ Billable = **verified-only** (consistent with E5)
- Notifications all-on in v1 (mute non-critical later); billing math = **hours only** (rates applied outside the system) *(default)*
- ✅ **Super Admin dashboard = HR cockpit + admin-only widgets (extends the dashboard D1 "same body, distinct label")** *(resolved 2026-06-11)* — the `super_admin` dashboard now **adds** an admin-only section to the shared `HrDashboard` payload, making it a **superset** rather than a relabel. Four widgets, all `super_admin`-scoped, each deep-linking into its owning epic: **(a) User & access** (active users · accounts pending provisioning · offboarded/disabled ≤30d — E2/F2.7); **(b) Recent audit feed** (last sensitive actions — E1 audit); **(c) Org rollups by service line** (headcount + active placements across Facility / Building / Parking — E3); **(d) Pending grants** (bank-account approval escalations + role-change requests awaiting super-admin action). Carried on `HrDashboard.admin` (present **only** for `super_admin`); the `hr_admin` payload is unchanged. See [F10.2 PRD DB-7], [E10 `openapi` `HrDashboard.admin`].
- ✅ **Shift-leader dashboard is dual-surface (web + mobile)** *(resolved 2026-06-11)* — the existing `LeaderDashboard` payload now also backs a **mobile Beranda** (the shift leader is on-site, phone-first), at parity with the web team dashboard. **No new endpoint** — `GET /dashboards/me` already returns `LeaderDashboard` for `shift_leader`; this adds the mobile surface only. See [F10.2 PRD DB-8], [`.pen` frame `UMzuO`].

**Deferred to the build/tech phase (not product):** auth session model (JWT vs server sessions) + token lifetimes + password policy; API pagination style; push provider (FCM/APNs); migration batch sizes/parallelism; audit & export storage/retention infrastructure; dashboard caching/freshness thresholds.
