import type { Role } from '@swp/shared';
import {
  Building2,
  CalendarCheck,
  CalendarClock,
  CalendarDays,
  ChartColumn,
  ClipboardCheck,
  Clock,
  Database,
  FilePenLine,
  FileText,
  Inbox,
  Layers,
  LayoutDashboard,
  type LucideIcon,
  MapPin,
  Plane,
  Settings,
  Timer,
  Users,
} from 'lucide-react';

/**
 * App-shell navigation config (DESIGN-SYSTEM §5 · comp/Sidebar `iCqTB`). Each item declares the
 * roles that may see it — coarse, hand-authored module visibility. This is **interim**: per
 * ENGINEERING.md A2 the per-operation gate is generated from `x-rbac` into `@swp/api-client`;
 * this map will be replaced by that derived permission map. Client gating is defense-in-depth
 * only — the Go API is the real gate (C1).
 */
export interface NavItem {
  to: string;
  /** i18n key for the label. */
  labelKey: string;
  icon: LucideIcon;
  roles: readonly Role[];
}

const ALL_WEB: readonly Role[] = ['super_admin', 'hr_admin', 'shift_leader'];
const ADMIN: readonly Role[] = ['super_admin', 'hr_admin'];

/** Primary nav (sidebar MENU section), in canvas order. */
export const NAV_ITEMS: readonly NavItem[] = [
  { to: '/', labelKey: 'nav.dashboard', icon: LayoutDashboard, roles: ALL_WEB },
  { to: '/employees', labelKey: 'nav.employees', icon: Users, roles: ALL_WEB },
  { to: '/client-companies', labelKey: 'nav.clientCompanies', icon: Building2, roles: ADMIN },
  { to: '/agreements', labelKey: 'nav.agreements', icon: FileText, roles: ADMIN },
  { to: '/change-requests', labelKey: 'nav.changeRequests', icon: Inbox, roles: ADMIN },
  { to: '/service-lines', labelKey: 'nav.serviceLines', icon: Layers, roles: ADMIN },
  { to: '/master-data', labelKey: 'nav.masterData', icon: Database, roles: ADMIN },
  { to: '/placements', labelKey: 'nav.placements', icon: MapPin, roles: ALL_WEB },
  { to: '/schedule', labelKey: 'nav.schedule', icon: CalendarClock, roles: ALL_WEB },
  { to: '/shifts', labelKey: 'nav.shifts', icon: Clock, roles: ADMIN },
  { to: '/attendance', labelKey: 'nav.attendance', icon: ClipboardCheck, roles: ALL_WEB },
  { to: '/corrections', labelKey: 'nav.corrections', icon: FilePenLine, roles: ALL_WEB },
  { to: '/leave', labelKey: 'nav.leave', icon: Plane, roles: ALL_WEB },
  { to: '/leave/quotas', labelKey: 'nav.leaveQuotas', icon: CalendarCheck, roles: ADMIN },
  { to: '/leave/calendar', labelKey: 'nav.leaveCalendar', icon: CalendarDays, roles: ALL_WEB },
  { to: '/overtime', labelKey: 'nav.overtime', icon: Timer, roles: ALL_WEB },
  { to: '/reports', labelKey: 'nav.reports', icon: ChartColumn, roles: ADMIN },
];

/** Footer nav (sidebar bottom) — settings/master data. */
export const SETTINGS_ITEM: NavItem = {
  to: '/settings',
  labelKey: 'nav.settings',
  icon: Settings,
  roles: ADMIN,
};

/** Filter a nav list to those visible to `role`. */
export function navForRole(items: readonly NavItem[], role: Role): NavItem[] {
  return items.filter((item) => item.roles.includes(role));
}
