import type { Permission } from '@swp/shared';
import {
  Banknote,
  Building2,
  CalendarClock,
  ChartColumn,
  ClipboardCheck,
  Inbox,
  LayoutDashboard,
  type LucideIcon,
  MapPin,
  Plane,
  Settings,
  Timer,
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
 * the aggregated "needs my decision" queue (leave + overtime + attendance + change requests),
 * a *view* over the same data the per-domain approval tabs show (one source of truth). It is
 * visible to anyone who can approve *anything* (`anyOf`).
 */
export const NAV_ITEMS: readonly NavItem[] = [
  { to: '/', labelKey: 'nav.dashboard', icon: LayoutDashboard, requires: 'dashboard.view' },
  {
    to: '/inbox',
    labelKey: 'nav.inbox',
    icon: Inbox,
    requires: {
      anyOf: ['leave.approve', 'overtime.approve', 'attendance.verify', 'change_requests.approve'],
    },
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
    { to: '/change-requests', labelKey: 'nav.changeRequests', requires: 'change_requests.read' },
  ],
  '/client-companies': [
    { to: '/client-companies', labelKey: 'nav.clientCompanies', requires: 'clients.read' },
    { to: '/service-lines', labelKey: 'nav.serviceLines', requires: 'service_lines.read' },
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
