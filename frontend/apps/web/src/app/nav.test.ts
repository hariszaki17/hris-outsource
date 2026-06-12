import { type Role, permissionsForRole } from '@swp/shared';
import { describe, expect, it } from 'vitest';
import {
  AGENT_NAV_ITEMS,
  NAV_ITEMS,
  SECTION_SUBNAV,
  SETTINGS_ITEM,
  activeSection,
  hasPermission,
  navForRole,
  routeRequirement,
  subnavForSection,
  visibleNav,
} from './nav.ts';

/**
 * Permission-keyed nav gating (capability axis — docs/eng/NAVIGATION-AND-RBAC.md). The shell
 * filters nav against the user's effective permissions, never their role. These tests drive the
 * filters through each role's interim permission bundle. The API remains the real gate — this is
 * the defense-in-depth client view.
 */
const labels = (role: Role) =>
  visibleNav(NAV_ITEMS, permissionsForRole(role)).map((i) => i.labelKey);

describe('visibleNav (permission-keyed)', () => {
  it('the sidebar holds the domain modules in canvas order', () => {
    expect(NAV_ITEMS.map((i) => i.labelKey)).toEqual([
      'nav.dashboard',
      'nav.inbox',
      'nav.employees',
      'nav.placements',
      'nav.clients',
      'nav.scheduleShift',
      'nav.attendance',
      'nav.leave',
      'nav.overtime',
      'nav.payroll',
      'nav.reports',
    ]);
  });

  it('super_admin and hr_admin see every primary module + settings', () => {
    for (const role of ['super_admin', 'hr_admin'] as const) {
      expect(labels(role)).toEqual(NAV_ITEMS.map((i) => i.labelKey));
      expect(hasPermission(permissionsForRole(role), SETTINGS_ITEM.requires)).toBe(true);
    }
  });

  it('shift_leader is scoped: operational modules + inbox, not clients/payroll/reports/settings', () => {
    const sl = labels('shift_leader');
    expect(sl).toEqual([
      'nav.dashboard',
      'nav.inbox',
      'nav.employees',
      'nav.placements',
      'nav.scheduleShift',
      'nav.attendance',
      'nav.leave',
      'nav.overtime',
    ]);
    expect(sl).not.toContain('nav.clients');
    expect(sl).not.toContain('nav.payroll');
    expect(sl).not.toContain('nav.reports');
    expect(hasPermission(permissionsForRole('shift_leader'), SETTINGS_ITEM.requires)).toBe(false);
  });

  it('the inbox is visible to anyone who can approve anything (anyOf)', () => {
    // shift_leader has leave.approve / overtime.approve → inbox shows.
    expect(labels('shift_leader')).toContain('nav.inbox');
    // agent has no approve permission → no inbox (and no web nav at all).
    expect(labels('agent')).toEqual([]);
  });
});

describe('section sub-nav (sub-features live under their parent module)', () => {
  it('agreements live under Karyawan; clients is its own module (no agreements)', () => {
    expect(NAV_ITEMS.map((i) => i.to)).toContain('/client-companies');
    expect(activeSection('/client-companies')).toBe('/client-companies');
    expect(activeSection('/agreements')).toBe('/employees');
  });

  it('master data resolves to Settings (its new home)', () => {
    expect(activeSection('/master-data')).toBe('/settings');
    expect(activeSection('/master-data/overtime-rules')).toBe('/settings');
  });

  it('payroll is its own primary module; corrections under Kehadiran', () => {
    expect(activeSection('/payroll')).toBe('/payroll');
    expect(activeSection('/payroll/SWP-PS-1')).toBe('/payroll');
    expect(activeSection('/corrections')).toBe('/attendance');
  });

  it('leave/quotas resolves to the Cuti section over the bare /leave prefix', () => {
    expect(activeSection('/leave/quotas')).toBe('/leave');
    expect(activeSection('/leave')).toBe('/leave');
  });

  it('shift_leader sub-nav hides admin-only tabs (Kuota under Cuti)', () => {
    const slLeave = subnavForSection('/leave', permissionsForRole('shift_leader')).map(
      (s) => s.labelKey,
    );
    expect(slLeave).toContain('nav.leaveApprovals');
    expect(slLeave).toContain('nav.leaveCalendar');
    expect(slLeave).not.toContain('nav.leaveQuotas');
  });

  it('a section with <2 visible tabs for a role renders no sub-nav', () => {
    // Klien is admin-only entirely → empty for shift_leader.
    expect(subnavForSection('/client-companies', permissionsForRole('shift_leader'))).toEqual([]);
  });

  it('shift_leader sees Karyawan → Perubahan Data tab (change requests route to the SL)', () => {
    // As of the agent-console redesign the SL reviews profile change requests (change_requests.read),
    // so the Karyawan sub-nav now strips to Karyawan + Perubahan Data (not agreements, which is HR).
    const slEmployees = subnavForSection('/employees', permissionsForRole('shift_leader')).map(
      (s) => s.labelKey,
    );
    expect(slEmployees).toEqual(['nav.employees', 'nav.changeRequests']);
    expect(slEmployees).not.toContain('nav.agreements');
  });

  it('every sub-nav route is a real, distinct destination', () => {
    for (const subs of Object.values(SECTION_SUBNAV)) {
      const tos = subs.map((s) => s.to);
      expect(new Set(tos).size).toBe(tos.length);
    }
  });
});

/**
 * Agent self-service backbone (`/me/*`; docs/eng/AGENT-WEB-ACCESS.md). The agent-console redesign
 * (2026-06-10) merged the old five per-feature items into THREE homes: Kehadiran (/me),
 * Pengajuan (/me/pengajuan), Akun (/me/akun). The old per-feature paths survive as guarded
 * redirect routes, so their route-level requirements must still resolve (a denied deep link goes
 * to /forbidden, never a broken render — defense-in-depth, ENGINEERING.md C1).
 */
describe('AGENT_NAV_ITEMS (agent self-service backbone, 3 merged homes)', () => {
  it('is exactly the three merged homes in canvas order', () => {
    expect(AGENT_NAV_ITEMS).toHaveLength(3);
    expect(AGENT_NAV_ITEMS.map((i) => i.to)).toEqual(['/me', '/me/pengajuan', '/me/akun']);
    expect(AGENT_NAV_ITEMS.map((i) => i.labelKey)).toEqual([
      'nav.meKehadiran',
      'nav.mePengajuan',
      'nav.meAkun',
    ]);
  });

  it('the agent role resolves to the self-service backbone; staff get the modules', () => {
    expect(navForRole('agent')).toBe(AGENT_NAV_ITEMS);
    for (const role of ['super_admin', 'hr_admin', 'shift_leader'] as const) {
      expect(navForRole(role)).toBe(NAV_ITEMS);
    }
  });

  it('agent permissions reveal all three homes; Pengajuan is anyOf(self.leave, self.overtime)', () => {
    const perms = permissionsForRole('agent');
    expect(visibleNav(AGENT_NAV_ITEMS, perms).map((i) => i.to)).toEqual([
      '/me',
      '/me/pengajuan',
      '/me/akun',
    ]);
    const pengajuan = AGENT_NAV_ITEMS.find((i) => i.to === '/me/pengajuan');
    expect(pengajuan?.requires).toEqual({ anyOf: ['self.leave', 'self.overtime'] });
  });

  it('shift_leader holds no self.* keys, so the agent backbone filters empty', () => {
    // shift_leader is the role whose bundle never overlaps the self.* keys. (super_admin/hr_admin
    // hold the whole catalog incl. self.* under the interim bundle, but they never RENDER this
    // backbone — the shell switches by role via navForRole, not by filtering. See next test.)
    expect(visibleNav(AGENT_NAV_ITEMS, permissionsForRole('shift_leader'))).toEqual([]);
  });

  it('staff roles never render the agent backbone — the shell switches by role, not permission', () => {
    // Defense in depth: even though the super-admin bundle technically satisfies self.* keys,
    // navForRole keys the backbone off the ROLE, so only the agent role ever gets AGENT_NAV_ITEMS.
    for (const role of ['super_admin', 'hr_admin', 'shift_leader'] as const) {
      expect(navForRole(role)).not.toBe(AGENT_NAV_ITEMS);
    }
    expect(navForRole('agent')).toBe(AGENT_NAV_ITEMS);
  });
});

describe('routeRequirement (agent /me/* — merged homes + old-path redirects)', () => {
  it('the three merged homes carry their own capability gate', () => {
    expect(routeRequirement('/me')).toBe('self.attendance');
    expect(routeRequirement('/me/akun')).toBe('self.profile');
    expect(routeRequirement('/me/pengajuan')).toEqual({ anyOf: ['self.leave', 'self.overtime'] });
  });

  it('the OLD per-feature paths still resolve a requirement (kept as guarded redirects)', () => {
    // Bookmarks/deep links survive: each redirect route must still gate so a denied link → /forbidden.
    expect(routeRequirement('/me/attendance')).toBe('self.attendance');
    expect(routeRequirement('/me/correction')).toBe('self.attendance');
    expect(routeRequirement('/me/schedule')).toBe('self.schedule');
    expect(routeRequirement('/me/leave')).toBe('self.leave');
    expect(routeRequirement('/me/overtime')).toBe('self.overtime');
    expect(routeRequirement('/me/payslip')).toBe('self.payslip');
    expect(routeRequirement('/me/profile')).toBe('self.profile');
  });

  it('/me (exact) gates on self.attendance but never bleeds into the deeper /me/* paths', () => {
    // The bare-/me rule is anchored (/^\/me$/), so /me/pengajuan must NOT match it.
    expect(routeRequirement('/me')).toBe('self.attendance');
    expect(routeRequirement('/me/pengajuan')).not.toBe('self.attendance');
  });

  it('an agent satisfies every /me requirement; a staff role satisfies none of them', () => {
    const agentPerms = permissionsForRole('agent');
    const slPerms = permissionsForRole('shift_leader');
    for (const path of ['/me', '/me/pengajuan', '/me/akun', '/me/leave', '/me/profile']) {
      const req = routeRequirement(path);
      expect(req).not.toBeNull();
      expect(hasPermission(agentPerms, req as never)).toBe(true);
      expect(hasPermission(slPerms, req as never)).toBe(false);
    }
  });

  it('/me/notifications is auth-only (no capability gate)', () => {
    expect(routeRequirement('/me/notifications')).toBeNull();
  });
});
