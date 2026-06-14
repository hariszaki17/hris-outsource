/**
 * RBAC role model — the four internal roles (EPICS.md §3, CONVENTIONS.md `x-rbac`).
 * The web console is used by the three staff roles; `agent` is the mobile surface only.
 *
 * NOTE (ENGINEERING.md A2): the authoritative *per-operation* permission map is generated
 * from the `x-rbac` OpenAPI extension into `@swp/api-client` (a post-gen step, not yet built).
 * Until that lands, coarse module/nav visibility is hand-authored in the web app
 * (`apps/web/src/app/nav.ts`) against this type, and will be superseded by the generated map.
 */
export const ROLES = ['super_admin', 'hr_admin', 'lead', 'shift_leader', 'agent'] as const;
export type Role = (typeof ROLES)[number];

/**
 * Roles that sign in to the web console. As of 2026-06-10 `agent` is included — agents get a
 * web self-service console under `/me/*` (docs/eng/AGENT-WEB-ACCESS.md, EPICS §8). Their
 * capability keys (`self.*`) never overlap the staff keys, so the surfaces stay separate.
 */
export const WEB_ROLES = [
  'super_admin',
  'hr_admin',
  'lead',
  'shift_leader',
  'agent',
] as const satisfies readonly Role[];
export type WebRole = (typeof WEB_ROLES)[number];

// ---------------------------------------------------------------------------
// Permission model (capability axis) — see docs/eng/NAVIGATION-AND-RBAC.md.
//
// The UI binds to PERMISSIONS, never to roles. A role is just a named bundle of
// permissions (ROLE_PERMISSIONS below). This keeps the system scalable from day one:
// adding a new role — or letting a super-admin define a custom one — is a *data* change
// (extend the bundle), not a code change (no nav/screen edits). Granularity is
// `module.action` so the same vocabulary gates both nav items and in-screen buttons.
//
// SINGLE SOURCE OF TRUTH: these strings are the client mirror of the `x-rbac` permissions
// the Go API enforces (CONVENTIONS.md). The API is the real gate; the client uses them only
// for visibility + defense-in-depth (ENGINEERING.md C1). When the generated `x-rbac` map
// lands in `@swp/api-client`, this catalog is replaced by it with no consumer changes.
//
// SCOPE (which data rows — e.g. a shift leader sees only their site) is a SEPARATE axis,
// enforced entirely server-side (row-level). It deliberately does NOT appear here: the nav
// never depends on scope (you either can approve leave or you can't); scope only filters the
// rows a screen/inbox shows. See the doc, §"Two axes".
// ---------------------------------------------------------------------------

export const PERMISSIONS = [
  'dashboard.view',
  // E2 — Identity & reference
  'employees.read',
  'employees.write',
  'clients.read',
  'clients.write',
  'agreements.read',
  'agreements.write',
  // E3 — Placement
  'placements.read',
  'placements.write',
  // E4 — Scheduling
  'schedule.read',
  'schedule.write',
  'shifts.read',
  'shifts.write',
  // E5 — Attendance
  'attendance.read',
  'attendance.verify',
  // E6 — Leave
  'leave.read',
  'leave.approve',
  'leave_quotas.read',
  'leave_quotas.write',
  // E7 — Overtime
  'overtime.read',
  'overtime.approve',
  'overtime_rules.read',
  'overtime_rules.write',
  // E8 — Payroll
  'payroll.read',
  'payroll.export',
  // E10 — Reporting
  'reports.read',
  // E11 — Approvals (configurable multi-line engine; supersedes change_requests.*).
  // `approvals.template.manage` gates per-company approval-template CRUD (HR/super-admin).
  // `approvals.act` gates only INBOX VISIBILITY — eligibility to act on a specific request
  // line is E11 line-membership, enforced server-side (E11 INV-3), never a permission.
  // `approvals.bypass` is super-admin force-approve (with a reason).
  'approvals.template.manage',
  'approvals.act',
  'approvals.bypass',
  // E1 — System / Settings
  'masterdata.manage',
  'settings.access',
  'settings.users.manage',
  'settings.roles.manage',
  'audit.read',
  // Agent self-service (web console under /me/*; docs/eng/AGENT-WEB-ACCESS.md). These gate the
  // agent's OWN records only — data scope (`scope: self`) is enforced server-side. They never
  // overlap the staff capability keys above, so the agent nav and the admin nav stay disjoint.
  'self.dashboard',
  'self.attendance',
  'self.schedule',
  'self.leave',
  'self.overtime',
  'self.profile',
  'self.payslip',
] as const;
export type Permission = (typeof PERMISSIONS)[number];

/**
 * Static role → permission bundles (the INTERIM map; ENGINEERING.md A2). This is the one
 * place that knows what each role can do. When the backend serves effective permissions
 * (a `/me` response or the generated `x-rbac` map), delete this table — nav declarations and
 * screen guards never change.
 *
 * `agent` has no web permissions (mobile-only). A future `client_viewer` role (external
 * client/partner access — see the doc's "Client portal" section) would be added here as a
 * read-only, hard-scoped bundle, and would surface in `apps/client-portal`, not this console.
 */
export const ROLE_PERMISSIONS: Record<Role, readonly Permission[]> = {
  // Full access, including defining custom roles. Via the all-permissions spread this includes
  // every E11 key: approvals.template.manage, approvals.act AND approvals.bypass (super-admin only).
  super_admin: [...PERMISSIONS],
  // Everything except authoring roles/permissions (super-admin owns the access model) AND
  // except approvals.bypass (force-approve is super-admin only). hr_admin keeps
  // approvals.template.manage + approvals.act.
  hr_admin: PERMISSIONS.filter(
    (p) => p !== 'settings.roles.manage' && p !== 'approvals.bypass',
  ),
  // Company-scoped operational approver over a set of assigned client companies (2026-06-12).
  // Placement lifecycle (create/transfer/end/renew — not SLA, not master edits), schedule, and
  // attendance verification; sees the E11 inbox via approvals.act (acting is line-membership-gated
  // server-side). No clients, contracts, payroll, reports, master data, or settings. Its company
  // set is resolved read-time from lead_assignments into Principal.CompanyIDs.
  lead: [
    'dashboard.view',
    'employees.read',
    'placements.read',
    'placements.write',
    'schedule.read',
    'schedule.write',
    'shifts.read',
    'attendance.read',
    'attendance.verify',
    'leave.read',
    'overtime.read',
    'approvals.act',
  ],
  // On-site supervisor: their site's daily operation only. No clients, contracts, payroll,
  // reports, master data, or settings. Scope (their one company) is enforced server-side.
  shift_leader: [
    'dashboard.view',
    'employees.read',
    'placements.read',
    'schedule.read',
    'schedule.write',
    // Read-only access to the Master Shift catalog: the leader needs it to render the
    // weekly schedule grid's shift picker AND to open the /shifts catalog screen. The
    // backend allows shift_leader GET /shift-masters; writes stay super_admin/hr_admin
    // only (no `shifts.write` here), so the catalog is read-only for a leader (SM-rbac).
    'shifts.read',
    'attendance.read',
    'attendance.verify',
    'leave.read',
    'overtime.read',
    // E11 inbox visibility. The SL approves only requests where they're on the current E11
    // line (membership-gated server-side) — not a *.approve permission. (Replaces the removed
    // change_requests.read/.approve; profile edits are now instant self-edit, E11 removes them.)
    'approvals.act',
  ],
  // Agent self-service (mobile + web console under /me/*). These `self.*` keys gate the agent's
  // OWN records; data scope is server-enforced (scope: self). See docs/eng/AGENT-WEB-ACCESS.md.
  agent: [
    'self.dashboard',
    'self.attendance',
    'self.schedule',
    'self.leave',
    'self.overtime',
    'self.profile',
    'self.payslip',
  ],
};

/** Effective permissions for a role (interim: from the static bundle; later: from the API). */
export function permissionsForRole(role: Role): readonly Permission[] {
  return ROLE_PERMISSIONS[role] ?? [];
}
