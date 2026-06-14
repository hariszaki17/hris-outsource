import type { Permission, Role } from '@swp/shared';
import {
  Banknote,
  Building2,
  CalendarClock,
  ChartColumn,
  ClipboardCheck,
  Fingerprint,
  Inbox,
  LayoutDashboard,
  type LucideIcon,
  MapPin,
  Plane,
  Settings,
  Timer,
  UserRound,
  Users,
} from 'lucide-react';

/**
 * App-shell navigation config (DESIGN-SYSTEM §5 · `comp/Sidebar` `iCqTB`; full rationale in
 * docs/eng/NAVIGATION-AND-RBAC.md). The dark sidebar holds the primary domain modules + a
 * Pengaturan footer. Sub-pages live **inside their parent section** as a secondary sub-nav
 * tab strip under the topbar — never one sidebar item per sub-feature.
 *
 * RBAC is **permission-keyed, not role-keyed** (the capability axis): each item declares the
 * permission(s) it `requires`, and visibility is computed against the signed-in user's
 * effective `permissions` (SessionUser.permissions). Adding a role — or a future admin-defined
 * custom role — is a data change to the `ROLE_PERMISSIONS` bundle, with zero nav edits. Client
 * gating is defense-in-depth only; the Go API is the real gate (ENGINEERING.md C1). Data SCOPE
 * (which rows — e.g. a shift leader's one site) is a separate, server-enforced axis and does
 * not appear here.
 */

/** A capability requirement: a single permission, or any-of a set (OR). */
export type Requirement = Permission | { anyOf: readonly Permission[] };

export interface NavItem {
  to: string;
  /** i18n key for the label. */
  labelKey: string;
  icon: LucideIcon;
  requires: Requirement;
}

export interface SubnavItem {
  to: string;
  labelKey: string;
  requires: Requirement;
}

/** True if `permissions` satisfies the requirement. */
export function hasPermission(permissions: readonly Permission[], requires: Requirement): boolean {
  if (typeof requires === 'string') return permissions.includes(requires);
  return requires.anyOf.some((p) => permissions.includes(p));
}

/**
 * Primary sidebar nav (domain backbone). `Kotak Masuk` is a cross-cutting workflow surface —
 * the aggregated "needs my decision" queue (E11 approval lines + E5 attendance verification),
 * a *view* over the same data the per-domain tabs show (one source of truth). It is visible to
 * anyone on an approval line (`approvals.act`) or who verifies attendance (`anyOf`).
 */
export const NAV_ITEMS: readonly NavItem[] = [
  { to: '/', labelKey: 'nav.dashboard', icon: LayoutDashboard, requires: 'dashboard.view' },
  {
    to: '/inbox',
    labelKey: 'nav.inbox',
    icon: Inbox,
    // E11: approval visibility is now `approvals.act` (line-member "needs my decision" queue;
    // acting on a line is membership-gated server-side). `attendance.verify` keeps the E5
    // verification queue in the same inbox.
    requires: { anyOf: ['approvals.act', 'attendance.verify'] },
  },
  { to: '/employees', labelKey: 'nav.employees', icon: Users, requires: 'employees.read' },
  { to: '/placements', labelKey: 'nav.placements', icon: MapPin, requires: 'placements.read' },
  {
    to: '/client-companies',
    labelKey: 'nav.clients',
    icon: Building2,
    requires: 'clients.read',
  },
  {
    to: '/schedule',
    labelKey: 'nav.scheduleShift',
    icon: CalendarClock,
    requires: 'schedule.read',
  },
  {
    to: '/attendance',
    labelKey: 'nav.attendance',
    icon: ClipboardCheck,
    requires: 'attendance.read',
  },
  { to: '/leave', labelKey: 'nav.leave', icon: Plane, requires: 'leave.read' },
  { to: '/overtime', labelKey: 'nav.overtime', icon: Timer, requires: 'overtime.read' },
  { to: '/payroll', labelKey: 'nav.payroll', icon: Banknote, requires: 'payroll.read' },
  { to: '/reports', labelKey: 'nav.reports', icon: ChartColumn, requires: 'reports.read' },
];

/** Footer nav (sidebar bottom). */
export const SETTINGS_ITEM: NavItem = {
  to: '/settings',
  labelKey: 'nav.settings',
  icon: Settings,
  requires: 'settings.access',
};

/**
 * Agent self-service nav backbone (web console under `/me/*`; docs/eng/AGENT-WEB-ACCESS.md).
 * Entirely separate from the staff `NAV_ITEMS` — agents and staff never share a sidebar. Each
 * item is gated on a `self.*` capability key, which only the `agent` bundle carries, so this
 * list filters to empty for any staff role. The shell picks this backbone when the signed-in
 * role is `agent` (AW-4); item visibility within it stays permission-keyed.
 */
export const AGENT_NAV_ITEMS: readonly NavItem[] = [
  // Kehadiran (home /me) merges the old dashboard + attendance + schedule: live clock, clock-in/out,
  // today's shift and recent history. Pengajuan merges leave + overtime (request tabs). Akun merges
  // profile + payslip + tiered Ubah Profil. See docs/eng/AGENT-WEB-ACCESS.md + E2 employee-profile.md.
  { to: '/me', labelKey: 'nav.meKehadiran', icon: Fingerprint, requires: 'self.attendance' },
  {
    to: '/me/pengajuan',
    labelKey: 'nav.mePengajuan',
    icon: ClipboardCheck,
    requires: { anyOf: ['self.leave', 'self.overtime'] },
  },
  { to: '/me/akun', labelKey: 'nav.meAkun', icon: UserRound, requires: 'self.profile' },
];

/** The primary nav backbone for a role: agents get the self-service list, staff get the modules. */
export function navForRole(role: Role): readonly NavItem[] {
  return role === 'agent' ? AGENT_NAV_ITEMS : NAV_ITEMS;
}

/**
 * Secondary section sub-nav — keyed by the parent primary route. Rendered as a tab strip under
 * the topbar when the active section has >1 visible sub-page. Sub-routes with no entry here
 * (detail/create pages) inherit their parent section for active-state.
 *
 * NOTE (migration, see doc §"Target IA"): pure reference/config — shift masters, leave quotas,
 * overtime rules, master data — will consolidate under Pengaturan → Master Data once their
 * routes move under `/settings/*`. They are kept under their domain section here so they stay
 * reachable without a route migration in this pass.
 */
export const SECTION_SUBNAV: Record<string, readonly SubnavItem[]> = {
  '/employees': [
    { to: '/employees', labelKey: 'nav.employees', requires: 'employees.read' },
    // Perjanjian kerja (employment agreement) is a SWP↔employee contract — it lives
    // under Karyawan, not under Klien (the agreement is not client-scoped).
    { to: '/agreements', labelKey: 'nav.agreements', requires: 'agreements.read' },
  ],
  '/client-companies': [
    { to: '/client-companies', labelKey: 'nav.clientCompanies', requires: 'clients.read' },
  ],
  '/schedule': [
    { to: '/schedule', labelKey: 'nav.schedule', requires: 'schedule.read' },
    { to: '/shifts', labelKey: 'nav.shifts', requires: 'shifts.read' },
  ],
  '/attendance': [
    { to: '/attendance', labelKey: 'nav.attendance', requires: 'attendance.read' },
    { to: '/corrections', labelKey: 'nav.corrections', requires: 'attendance.verify' },
  ],
  '/leave': [
    { to: '/leave', labelKey: 'nav.leaveApprovals', requires: 'leave.read' },
    { to: '/leave/quotas', labelKey: 'nav.leaveQuotas', requires: 'leave_quotas.read' },
    { to: '/leave/calendar', labelKey: 'nav.leaveCalendar', requires: 'leave.read' },
  ],
  '/overtime': [
    { to: '/overtime', labelKey: 'nav.overtimeApprovals', requires: 'overtime.read' },
    { to: '/overtime/rekap', labelKey: 'nav.overtimeRekap', requires: 'overtime.read' },
    { to: '/overtime/aturan', labelKey: 'nav.overtimeRules', requires: 'overtime_rules.read' },
  ],
};

/** Filter a nav list to those whose requirement the permissions satisfy. */
export function visibleNav<T extends { requires: Requirement }>(
  items: readonly T[],
  permissions: readonly Permission[],
): T[] {
  return items.filter((item) => hasPermission(permissions, item.requires));
}

/** Routes that belong to a section but aren't sub-nav tabs (detail/create), mapped to parent. */
const SECTION_ALIASES: Record<string, string> = {
  '/company-roster': '/placements',
  '/master-data': '/settings',
};

/**
 * Resolve which primary section a pathname belongs to (for sidebar active-state + sub-nav).
 * Returns the parent primary route, or null for top-level pages with no section.
 */
export function activeSection(pathname: string): string | null {
  // Direct sub-nav membership (longest route prefix wins, e.g. /leave/quotas over /leave).
  let best: string | null = null;
  let bestLen = -1;
  for (const [parent, subs] of Object.entries(SECTION_SUBNAV)) {
    for (const sub of subs) {
      const match = sub.to === '/' ? pathname === '/' : pathname.startsWith(sub.to);
      if (match && sub.to.length > bestLen) {
        best = parent;
        bestLen = sub.to.length;
      }
    }
  }
  if (best) return best;
  // Agent self-service backbone (/me/*) — longest matching item wins so /me/attendance beats /me.
  if (pathname === '/me' || pathname.startsWith('/me/')) {
    const agent = AGENT_NAV_ITEMS.reduce<string | null>((acc, i) => {
      const match = i.to === '/me' ? pathname === '/me' : pathname.startsWith(i.to);
      if (!match) return acc;
      return acc === null || i.to.length > acc.length ? i.to : acc;
    }, null);
    if (agent) return agent;
  }
  // Alias routes (rosters, payroll, master data).
  for (const [route, parent] of Object.entries(SECTION_ALIASES)) {
    if (pathname.startsWith(route)) return parent;
  }
  // Primary items without a sub-nav (Dashboard, Inbox, Penempatan).
  const direct = NAV_ITEMS.find((i) =>
    i.to === '/' ? pathname === '/' : pathname.startsWith(i.to),
  );
  return direct ? direct.to : null;
}

/**
 * Capability requirement for a concrete pathname (route-level guard, NAVIGATION-AND-RBAC §4.4).
 * The router's `beforeLoad` uses this to redirect a permitted-section-but-denied deep link to
 * `/forbidden`, instead of rendering a broken screen and relying on the API 403 (defense-in-depth,
 * ENGINEERING.md C1 — the Go API is still the real gate). Rules are ordered most-specific first;
 * the first match wins. A `null` result means the route is not capability-gated (auth-only, e.g.
 * `/`, `/notifications`, `/forbidden`). SCOPE (which rows) stays server-only and never appears here.
 */
const ROUTE_REQUIREMENTS: readonly [RegExp, Requirement][] = [
  // Agent self-service (/me/*) — most-specific first; /me/notifications is auth-only (no entry).
  // The merged homes: Kehadiran (/me), Pengajuan (/me/pengajuan), Akun (/me/akun). The old
  // per-feature paths are kept as guarded redirect routes (router.tsx) so bookmarks survive —
  // their requirements stay listed here so a denied deep link still resolves to /forbidden.
  [/^\/me\/pengajuan/, { anyOf: ['self.leave', 'self.overtime'] }],
  [/^\/me\/akun/, 'self.profile'],
  [/^\/me\/attendance/, 'self.attendance'],
  [/^\/me\/correction/, 'self.attendance'],
  [/^\/me\/schedule/, 'self.schedule'],
  [/^\/me\/leave/, 'self.leave'],
  [/^\/me\/overtime/, 'self.overtime'],
  [/^\/me\/payslip/, 'self.payslip'],
  [/^\/me\/profile/, 'self.profile'],
  // Kehadiran is the agent home — gated on self.attendance (the live clock-in/out surface).
  [/^\/me$/, 'self.attendance'],
  // Most-specific first.
  [/^\/client-companies\/[^/]+\/roster/, 'placements.read'],
  [/^\/client-companies/, 'clients.read'],
  [/^\/employees/, 'employees.read'],
  [/^\/agreements/, 'agreements.read'],
  [/^\/master-data/, 'masterdata.manage'],
  [/^\/placements/, 'placements.read'],
  [/^\/schedule/, 'schedule.read'],
  [/^\/shifts/, 'shifts.read'],
  [/^\/attendance\/verification/, 'attendance.verify'],
  [/^\/attendance/, 'attendance.read'],
  [/^\/corrections/, 'attendance.verify'],
  [/^\/leave\/quotas/, 'leave_quotas.read'],
  [/^\/leave/, 'leave.read'],
  [/^\/overtime\/aturan/, 'overtime_rules.read'],
  [/^\/overtime/, 'overtime.read'],
  [/^\/payroll/, 'payroll.read'],
  [/^\/reports/, 'reports.read'],
  [/^\/inbox/, { anyOf: ['approvals.act', 'attendance.verify'] }],
  [/^\/settings/, 'settings.access'],
];

/**
 * Returns the capability required to view `pathname`, or `null` for auth-only routes.
 * `/` (dashboard) is gated on `dashboard.view`; unmatched routes are ungated.
 */
export function routeRequirement(pathname: string): Requirement | null {
  if (pathname === '/') return 'dashboard.view';
  for (const [pattern, requires] of ROUTE_REQUIREMENTS) {
    if (pattern.test(pathname)) return requires;
  }
  return null;
}

/** The sub-nav tabs for a section, filtered by permission. Empty when <2 tabs are visible. */
export function subnavForSection(
  sectionTo: string | null,
  permissions: readonly Permission[],
): SubnavItem[] {
  if (!sectionTo) return [];
  const subs = SECTION_SUBNAV[sectionTo];
  if (!subs) return [];
  const visible = visibleNav(subs, permissions);
  return visible.length > 1 ? visible : [];
}
