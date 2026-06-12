# Navigation IA & RBAC

**Status:** Adopted 2026-06-03. Authoritative for the web console's information architecture
(sidebar, section sub-nav, Settings) and the client-side RBAC model. Binds `apps/web/src/app/nav.ts`,
the `comp/Sidebar` `.pen` master (`iCqTB`), and `packages/shared/src/rbac.ts`. The Go API and
`docs/api/CONVENTIONS.md` (`x-rbac`) remain authoritative for *enforcement*; this doc governs the
client's *visibility* layer (defense-in-depth, ENGINEERING.md C1).

---

## 1. Why this exists

The web console serves **two opposite users** from one app (the third staff role, super admin,
is HR admin + system config; `agent` is mobile-only):

| | **HR / placement admin** | **Shift leader** |
|---|---|---|
| Scope | All companies, all agents | **One** site, their agents only |
| Pattern | Broad daily, deep periodically | Narrow, deep, **every day** |
| Thinks in | Entities & processes | Tasks ("who's in, what do I approve") |
| Daily question | Run the operation end-to-end | **"What needs my decision right now?"** |

A power user who navigates by **entity** and a frontline operator who navigates by **task**. No
single organizing axis is optimal for both — so the IA uses a **domain backbone** reshaped per
role by **permission-keyed visibility**, plus one cross-cutting **Inbox** for the operator's job.

---

## 2. Organizing principles ✅

1. **Primary axis = business domain**, with the old "Karyawan" junk drawer split apart:
   `Karyawan` is people (agents) only; `Klien & Perjanjian` is its own module; `Penempatan` stays
   first-class (the product differentiator); pure reference/config moves to **Settings → Master Data**.
2. **Cross-cut = `Kotak Masuk` (Inbox)** — the aggregated "needs my decision" queue (leave +
   overtime + attendance + change requests). It is a **view**, not a second queue: it reads the
   same data the per-domain approval tabs show (**single source of truth**). Both surfaces exist
   ("inbox + per-domain", decided 2026-06-03).
3. **Visibility is permission-keyed, never role-keyed.** Nav items declare a capability
   `requires`; the shell filters against the user's effective `permissions`. Role is just a
   bundle of permissions. Adding/editing a role is a **data** change, not code.
4. **Two axes** — *capability* (which pages/actions) drives nav; *scope* (which data rows) is
   separate and **enforced server-side only**. The nav never depends on scope.
5. **Cadence demotes config.** Set-and-forget reference data lives under Settings, not in the
   daily operational modules.

Rejected: **lifecycle-stage grouping** (Setup→People→Ops→Pay headers) — good for onboarding/docs,
but it adds a nesting layer that slows daily items and is pure overhead for the shift leader.

---

## 3. Target IA

### 3.1 Primary sidebar (domain backbone)

Order matches `comp/Sidebar` `iCqTB` and `NAV_ITEMS`:

| # | Module | Route | `requires` |
|---|--------|-------|-----------|
| 1 | Dashboard | `/` | `dashboard.view` |[^db7]
| 2 | **Kotak Masuk** | `/inbox` | *any of* `leave.approve` · `overtime.approve` · `attendance.verify` · `change_requests.approve` |
| 3 | Karyawan | `/employees` | `employees.read` |
| 4 | Penempatan | `/placements` | `placements.read` |
| 5 | **Klien & Perjanjian** | `/client-companies` | *any of* `clients.read` · `agreements.read` |
| 6 | Jadwal Shift | `/schedule` | `schedule.read` |
| 7 | Kehadiran | `/attendance` | `attendance.read` |
| 8 | Cuti | `/leave` | `leave.read` |
| 9 | Lembur | `/overtime` | `overtime.read` |
| 10 | Penggajian | `/payroll` | `payroll.read` |
| 11 | Laporan | `/reports` | `reports.read` |
| footer | Pengaturan | `/settings` | `settings.access` |

The "8 modules" design lock (DESIGN-SYSTEM line 171) is **retired** — sidebar length is a
per-role *outcome* (a shift leader sees ~8; a future finance role would see ~3), not a fixed count.

[^db7]: `dashboard.view` gates the route for all staff roles that hold it (`super_admin`, `hr_admin`, `shift_leader`, `lead`; agents use `self.dashboard`); the **Super Admin admin-widget block** (DB-7: users & access · audit feed · org rollups · pending grants) needs **no new permission key** — the server fills `HrDashboard.admin` only when the principal's effective role is `super_admin`, and the client renders those widgets conditionally on `role === 'super_admin'`. Defense-in-depth: the API is the gate (ENGINEERING C1). The shift-leader dashboard is dual-surface (web + mobile Beranda, DB-8) on the same `LeaderDashboard` payload.

### 3.2 Section sub-nav (tabs under the topbar)

Rendered only when a section has **>1 permitted tab** for the user. Detail/create routes inherit
their section.

- **Karyawan** → Employees · Change requests
- **Klien & Perjanjian** → Client companies · Agreements · Service lines
- **Jadwal Shift** → Schedule · Shift masters *(→ Settings, see §3.4)*
- **Kehadiran** → Attendance · Corrections
- **Cuti** → Approvals · Leave quotas *(→ Settings)* · Calendar
- **Lembur** → Approvals · Summary (rekap) · Rules *(→ Settings)*
- **Penggajian / Laporan** → single pages (no sub-nav yet)

### 3.3 Settings (Pengaturan)

Hub cards + nested routes: **Users & Roles** · **Audit Log** · **General** · **Master Data**
(service lines, leave types, attendance codes, overtime rules). A future **Roles & Permissions**
card (`settings.roles.manage`) is where a super admin defines custom roles — see §5.

### 3.4 Planned migration (not in this pass)

Pure reference/config is *homed* in Settings conceptually but its **routes have not moved yet**:
shift masters, leave quotas, overtime rules stay under their domain sections; `/master-data` is a
top-level route reachable from the Settings hub card (aliased to the `/settings` section for
sidebar active-state). Follow-up: move these under `/settings/*` and relocate overtime "rekap"
(a report) under Laporan. Tracked here so the menu doesn't imply a structure the routes don't have.

---

## 4. RBAC model

### 4.1 Capability axis — permissions ✅

The catalog lives in `packages/shared/src/rbac.ts` (`PERMISSIONS`, type `Permission`). Granularity
is **`module.action`** (decided 2026-06-03) so one vocabulary gates both nav items and in-screen
buttons: `employees.read`/`.write`, `leave.read`/`.approve`, `payroll.read`/`.export`,
`change_requests.read`/`.approve` (+ HR-only `change_requests.approve.bank`),
`settings.roles.manage`, `masterdata.manage`, etc.

A **role is a named bundle** of permissions (`ROLE_PERMISSIONS`). The UI checks
`hasPermission(user.permissions, requires)` — never `role === '…'`. Requirements may be a single
permission or `{ anyOf: [...] }` (used by the Inbox and Klien & Perjanjian).

**Single source of truth:** these strings mirror the `x-rbac` permissions the Go API enforces
(CONVENTIONS.md). When the generated `x-rbac` map lands in `@swp/api-client`, the static catalog
is replaced by it with **no consumer changes**.

Interim role bundles:

- **super_admin** — all permissions (incl. `settings.roles.manage`).
- **hr_admin** — all except `settings.roles.manage` (super admin owns the access model).
- **shift_leader** — `dashboard.view`, `employees.read`, `placements.read`, `schedule.read/.write`,
  `attendance.read/.verify`, `leave.read/.approve`, `overtime.read/.approve`,
  `change_requests.read/.approve` (**not** `.approve.bank`). No clients,
  contracts, payroll, reports, master data, or settings.
- **lead** *(2026-06-12)* — service-line operational approver ("parking lead", "facility lead", …):
  `dashboard.view`, `employees.read`, `placements.read/.write` (placement lifecycle:
  create/transfer/end/renew — **not** shift-leader-assignment, **not** placement master edits),
  `schedule.read/.write`, `attendance.read/.verify`, `leave.read/.approve` (final/L2),
  `overtime.read/.approve` (final/L2), `change_requests.read/.approve` (**not** `.approve.bank` —
  bank-account changes escalate to HR). No clients, contracts, payroll, reports, master data, or
  settings. Note: lead does **not** do L1 leave/overtime approval (that stays `shift_leader`; lead is
  the L2/final approver scoped to the agent's company) and **cannot** add employees (tambah karyawan),
  run payroll, write master data, or assign shift-leaders (SLA).
- **agent** — the `self.*` self-service bundle: `self.dashboard`, `self.attendance`, `self.schedule`,
  `self.leave`, `self.overtime`, `self.profile`, `self.payslip`. *(Updated 2026-06-10: agents now
  have a **web self-service console** under `/me/*` — reverses the prior "none (mobile-only)". The
  shell picks the nav backbone by role; `self.*` keys never overlap the staff capability keys, so
  the two surfaces stay cleanly separated. Ratified EPICS §8; full spec:
  [AGENT-WEB-ACCESS.md](./AGENT-WEB-ACCESS.md).)*

**Change-request approval is split by sensitivity** *(2026-06-11).* `change_requests.approve` covers
non-sensitive fields (phone, emergency contact); **bank-account** changes need the HR-only
`change_requests.approve.bank` (fraud/payroll risk). A request mixing both is **partially actionable**:
a shift leader approves the non-bank fields, and the **bank field escalates to HR** (stays pending
until an `change_requests.approve.bank` holder acts) — it never silently applies. The Inbox review UI
disables the bank-field action and shows "Perlu HR" for approvers without the sub-permission. (F2.1
EP-5c/EP-5d.)

### 4.2 Scope axis — data rows (server-only) ✅

*Which records* a user may see (e.g. a shift leader's single site) is a **separate axis enforced
entirely server-side** (row-level), decided 2026-06-03 as "purely backend". It deliberately does
**not** appear in the permission catalog or the nav: you either can approve leave or you can't —
the menu is identical; scope only filters the rows a list/inbox returns. The client renders
whatever scoped rows the API returns.

**Shift-leader role + company scope are derived read-time (2026-06-08).** ✅ For a field employee,
the **effective `shift_leader` role and its company scope are not stored on `users`** — the auth
middleware derives both **per request** from the active E3 `shift_leader_assignments` row, which is
the **single source of truth** for leadership. An employee with an active assignment ⇒ role
`shift_leader`, scope = that one company (INV-3); without one ⇒ role `agent`, no company scope.
Staff roles (`super_admin`, `hr_admin`) are global and never derived. Stored `users.role` /
`users.company_id` and the JWT `cmp` claim are **advisory only**; `/auth/me` reports the
request-time derived role + scope. **Fail-safe:** on resolver error or no assignment the scope is
stripped and the role falls back to `agent` — deny, never escalate. **No re-login required:**
assigning or revoking leadership in E3 takes effect on the next request, since nothing leader-related
is baked into the token.

**Lead role + multi-company scope (2026-06-12).** ✅ Unlike `shift_leader` (derived-from-placement),
`lead` is **SWP staff with a stored `users.role = 'lead'`** — but its **company set is still resolved
read-time, not stored on the user**. A `shift_leader` has `company` scope over **exactly one** company
(its derived assignment); a `lead` has `company` scope over a **set of many** client companies,
assigned via the `lead_assignments` table (mirrors `shift_leader_assignments`). The auth middleware
resolves that set per request into **`Principal.CompanyIDs []string`** (in contrast to the single
`Principal.CompanyID` used by `shift_leader`). `GuardCompany` passes for a `lead` when the **resource's
company is a member of the lead's assigned set** (membership test over `CompanyIDs`), just as it passes
for a `shift_leader` when the resource company equals its single `CompanyID`; `super_admin`/`hr_admin`
stay global (no company guard). Lead is the L2/final approver for leave + overtime **scoped to the
agent's company** via this same guard, and arranges placements only within its assigned set; HR keeps
global oversight + override. **Fail-safe:** on resolver error or an empty assigned set the scope is
stripped — deny, never escalate. `service_line` remains a **data label, not an RBAC axis** — scope is
keyed on company membership only.

### 4.3 Day-one vs deferred

Built now (cheap): nav/buttons declare permissions; `SessionUser.permissions` carries the
effective set (today derived from `role` via `permissionsForRole()` at login). Deferred
(expensive): swap the static `ROLE_PERMISSIONS` table for an API-delivered map (`/me`), and add
the Roles & Permissions admin screen. **Nav declarations and screen guards never change** across
that swap — that is the whole point of getting the abstraction boundary right today.

### 4.4 Graceful degradation (no dead flows)

- A section with **zero** permitted tabs is hidden entirely; `subnavForSection` returns `[]` when
  fewer than 2 tabs are visible (single-tab sections render as a plain page).
- A permitted-section-but-denied deep link → `/forbidden` + `comp/EmptyNoPermission` (`MRbzz`).
- Client gating is defense-in-depth; a missing client check is a UX bug, not a security hole — the
  API is the gate.

---

## 5. Client / partner access (forward-looking)

Per supervisor input (2026-06-03), client/partner companies **may** be given access "to know
what's going on." This **flips a foundational decision**: CLAUDE.md and EPICS §8 state the system
is *internal-only; client companies are data, not tenants.* Granting clients login makes a client
an **external actor**. ⚠ **This change must be ratified in EPICS §8** (dated) before client work
begins — it is not yet adopted; this section records the decided architecture so the RBAC model is
ready.

**Decided 2026-06-03:** a **separate client web SPA** — `apps/client-portal` (browser, Vite SPA,
own URL/subdomain), sharing `packages/{api-client, ui, design-tokens, shared}` but shipping only
curated read-only screens. Internal screens are **not in that bundle**, so they cannot leak;
hardened and deployed separately. (Clients use the **web**, not the React Native mobile app.)

Why this is now low-risk: a client is just another permission bundle + a hard scope —
`client_viewer = { dashboard.view, attendance.read, reports.read }`, scope = their own company.
The permission model above serves it with no special-casing. New escalations to plan for:

- **Scope becomes a security boundary** (Client A must never see Client B's agents): mandatory,
  audited row-level enforcement; the portal is **scoped by construction** (only calls scoped
  endpoints).
- **Per-field governance**: clients see presence, not salary/payroll/margins/PII/other clients —
  a "client-visible" classification, not just per-page gating.
- **Client identity sub-domain** (E2 expansion): client users, possibly client-admin vs
  client-viewer, invitations, separate auth.

Likely client surface ("know what's going on"): coverage/SLA dashboard · their placed-agent roster
· attendance/presence at their site · leave calendar · billable/SLA reports · shared documents.
**Not**: payroll, margins, other clients, master data, internal config. (Scope to confirm with the
supervisor.)

---

## 6. Implementation map

| Concern | Location |
|---|---|
| Permission catalog, types, role bundles | `packages/shared/src/rbac.ts` (`PERMISSIONS`, `Permission`, `ROLE_PERMISSIONS`, `permissionsForRole`) |
| Effective permissions on the session | `apps/web/src/lib/auth.ts` (`SessionUser.permissions`); derived at login in `login-screen.tsx` |
| Nav config + permission filters | `apps/web/src/app/nav.ts` (`NAV_ITEMS`, `SECTION_SUBNAV`, `Requirement`, `hasPermission`, `visibleNav`, `subnavForSection`) |
| Shell wiring | `apps/web/src/app/shell.tsx` |
| Inbox screen | `apps/web/src/features/e10-reporting/inbox-screen.tsx` (reuses `ApprovalInboxPanel` + `useGetMyDashboard`) |
| Master Data home | Settings hub card → `/master-data` |
| Sidebar design master | `docs/design/brainstorm.pen` · `comp/Sidebar` `iCqTB` |
| Tests | `apps/web/src/app/nav.test.ts` |

---

## 7. Open decisions

- **EPICS §8 ratification** of the internal-only → client-accessible change (blocks client work).
- **Config route migration** (§3.4): move master-data/quotas/shifts/rules under `/settings/*`,
  rekap under Laporan.
- **Inbox live counts**: the v1 screen reuses the dashboard's `pending_approvals_panel`; a
  dedicated paginated full-queue endpoint is a follow-up.
- **Client surface scope**: confirm the exact read-only slice with the supervisor before building
  `apps/client-portal`.
