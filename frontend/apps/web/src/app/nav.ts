import type { Role } from '@swp/shared';
import {
  CalendarClock,
  ChartColumn,
  ClipboardCheck,
  LayoutDashboard,
  type LucideIcon,
  MapPin,
  Plane,
  Settings,
  Timer,
  Users,
} from 'lucide-react';

/**
 * App-shell navigation config (DESIGN-SYSTEM §5 · `comp/Sidebar` `iCqTB`). The dark sidebar holds
 * **exactly the 8 primary modules the design specifies** (DESIGN-SYSTEM line 171) + a Pengaturan
 * footer — never one item per sub-feature. Sub-pages (master data, client companies, corrections,
 * leave quotas, payroll, …) live **inside their parent section** as a secondary sub-nav (the design
 * IA: "payroll → under Laporan", "verification/corrections → under Kehadiran", no separate sidebar
 * items). Notifications is the topbar bell, not a sidebar item.
 *
 * Each item declares the roles that may see it — coarse, hand-authored module visibility. This is
 * **interim**: per ENGINEERING.md A2 the per-operation gate is generated from `x-rbac`; this map
 * will be replaced. Client gating is defense-in-depth only — the Go API is the real gate (C1).
 */
export interface NavItem {
  to: string;
  /** i18n key for the label. */
  labelKey: string;
  icon: LucideIcon;
  roles: readonly Role[];
}

export interface SubnavItem {
  to: string;
  labelKey: string;
  roles: readonly Role[];
}

const ALL_WEB: readonly Role[] = ['super_admin', 'hr_admin', 'shift_leader'];
const ADMIN: readonly Role[] = ['super_admin', 'hr_admin'];

/** Primary sidebar nav — the 8 modules from `comp/Sidebar` `iCqTB`, in canvas order. */
export const NAV_ITEMS: readonly NavItem[] = [
  { to: '/', labelKey: 'nav.dashboard', icon: LayoutDashboard, roles: ALL_WEB },
  { to: '/employees', labelKey: 'nav.employees', icon: Users, roles: ALL_WEB },
  { to: '/placements', labelKey: 'nav.placements', icon: MapPin, roles: ALL_WEB },
  { to: '/schedule', labelKey: 'nav.scheduleShift', icon: CalendarClock, roles: ALL_WEB },
  { to: '/attendance', labelKey: 'nav.attendance', icon: ClipboardCheck, roles: ALL_WEB },
  { to: '/leave', labelKey: 'nav.leave', icon: Plane, roles: ALL_WEB },
  { to: '/overtime', labelKey: 'nav.overtime', icon: Timer, roles: ALL_WEB },
  { to: '/reports', labelKey: 'nav.reports', icon: ChartColumn, roles: ADMIN },
];

/** Footer nav (sidebar bottom). */
export const SETTINGS_ITEM: NavItem = {
  to: '/settings',
  labelKey: 'nav.settings',
  icon: Settings,
  roles: ADMIN,
};

/**
 * Secondary section sub-nav — keyed by the parent primary route. Rendered as a tab strip under the
 * topbar when the active route belongs to a section that has >1 visible sub-page. Sub-routes that
 * have no entry here (detail/create pages) inherit their parent section for active-state purposes.
 */
export const SECTION_SUBNAV: Record<string, readonly SubnavItem[]> = {
  '/employees': [
    { to: '/employees', labelKey: 'nav.employees', roles: ALL_WEB },
    { to: '/client-companies', labelKey: 'nav.clientCompanies', roles: ADMIN },
    { to: '/agreements', labelKey: 'nav.agreements', roles: ADMIN },
    { to: '/change-requests', labelKey: 'nav.changeRequests', roles: ADMIN },
    { to: '/service-lines', labelKey: 'nav.serviceLines', roles: ADMIN },
    { to: '/master-data', labelKey: 'nav.masterData', roles: ADMIN },
  ],
  '/schedule': [
    { to: '/schedule', labelKey: 'nav.schedule', roles: ALL_WEB },
    { to: '/shifts', labelKey: 'nav.shifts', roles: ADMIN },
  ],
  '/attendance': [
    { to: '/attendance', labelKey: 'nav.attendance', roles: ALL_WEB },
    { to: '/corrections', labelKey: 'nav.corrections', roles: ALL_WEB },
  ],
  '/leave': [
    { to: '/leave', labelKey: 'nav.leaveApprovals', roles: ALL_WEB },
    { to: '/leave/quotas', labelKey: 'nav.leaveQuotas', roles: ADMIN },
    { to: '/leave/calendar', labelKey: 'nav.leaveCalendar', roles: ALL_WEB },
  ],
  '/overtime': [
    { to: '/overtime', labelKey: 'nav.overtimeApprovals', roles: ALL_WEB },
    { to: '/overtime/rekap', labelKey: 'nav.overtimeRekap', roles: ADMIN },
    { to: '/overtime/aturan', labelKey: 'nav.overtimeRules', roles: ADMIN },
  ],
  '/reports': [
    { to: '/reports', labelKey: 'nav.reports', roles: ADMIN },
    { to: '/payroll', labelKey: 'nav.payroll', roles: ADMIN },
  ],
};

/** Filter a nav list to those visible to `role`. */
export function navForRole<T extends { roles: readonly Role[] }>(
  items: readonly T[],
  role: Role,
): T[] {
  return items.filter((item) => item.roles.includes(role));
}

/** Routes that belong to a section but aren't sub-nav tabs (detail/create), mapped to their parent. */
const SECTION_ALIASES: Record<string, string> = {
  '/company-roster': '/placements',
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
  // Alias routes (rosters, etc.).
  for (const [route, parent] of Object.entries(SECTION_ALIASES)) {
    if (pathname.startsWith(route)) return parent;
  }
  // Primary items without a sub-nav (Dashboard, Karyawan-less, Penempatan).
  const direct = NAV_ITEMS.find((i) =>
    i.to === '/' ? pathname === '/' : pathname.startsWith(i.to),
  );
  return direct ? direct.to : null;
}

/** The sub-nav tabs for a section, filtered by role. Empty when the section has <2 visible tabs. */
export function subnavForSection(sectionTo: string | null, role: Role): SubnavItem[] {
  if (!sectionTo) return [];
  const subs = SECTION_SUBNAV[sectionTo];
  if (!subs) return [];
  const visible = navForRole(subs, role);
  return visible.length > 1 ? visible : [];
}
