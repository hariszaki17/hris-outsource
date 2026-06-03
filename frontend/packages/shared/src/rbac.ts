/**
 * RBAC role model — the four internal roles (EPICS.md §3, CONVENTIONS.md `x-rbac`).
 * The web console is used by the three staff roles; `agent` is the mobile surface only.
 *
 * NOTE (ENGINEERING.md A2): the authoritative *per-operation* permission map is generated
 * from the `x-rbac` OpenAPI extension into `@swp/api-client` (a post-gen step, not yet built).
 * Until that lands, coarse module/nav visibility is hand-authored in the web app
 * (`apps/web/src/app/nav.ts`) against this type, and will be superseded by the generated map.
 */
export const ROLES = ['super_admin', 'hr_admin', 'shift_leader', 'agent'] as const;
export type Role = (typeof ROLES)[number];

/** Roles that sign in to the web console (`agent` is mobile-only). */
export const WEB_ROLES = [
  'super_admin',
  'hr_admin',
  'shift_leader',
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
  'change_requests.read',
  'change_requests.approve',
  'clients.read',
  'clients.write',
  'agreements.read',
  'agreements.write',
  'service_lines.read',
  'service_lines.write',
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
  // E1 — System / Settings
  'masterdata.manage',
  'settings.access',
  'settings.users.manage',
  'settings.roles.manage',
  'audit.read',
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
  // Full access, including defining custom roles.
  super_admin: [...PERMISSIONS],
  // Everything except authoring roles/permissions (super-admin owns the access model).
  hr_admin: PERMISSIONS.filter((p) => p !== 'settings.roles.manage'),
  // On-site supervisor: their site's daily operation only. No clients, contracts, payroll,
  // reports, master data, or settings. Scope (their one company) is enforced server-side.
  shift_leader: [
    'dashboard.view',
    'employees.read',
    'placements.read',
    'schedule.read',
    'schedule.write',
    'attendance.read',
    'attendance.verify',
    'leave.read',
    'leave.approve',
    'overtime.read',
    'overtime.approve',
  ],
  // Mobile-only — no web console access.
  agent: [],
};

/** Effective permissions for a role (interim: from the static bundle; later: from the API). */
export function permissionsForRole(role: Role): readonly Permission[] {
  return ROLE_PERMISSIONS[role] ?? [];
}
