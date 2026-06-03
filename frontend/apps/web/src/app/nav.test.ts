import { type Role, permissionsForRole } from '@swp/shared';
import { describe, expect, it } from 'vitest';
import {
  NAV_ITEMS,
  SECTION_SUBNAV,
  SETTINGS_ITEM,
  activeSection,
  hasPermission,
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
      'nav.clientsAgreements',
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
    expect(sl).not.toContain('nav.clientsAgreements');
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
  it('clients & agreements is now its own primary module, not under Karyawan', () => {
    expect(NAV_ITEMS.map((i) => i.to)).toContain('/client-companies');
    expect(activeSection('/client-companies')).toBe('/client-companies');
    expect(activeSection('/agreements')).toBe('/client-companies');
    expect(activeSection('/service-lines')).toBe('/client-companies');
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
    // Karyawan: shift_leader only sees /employees (not change-requests) → no sub-nav strip.
    expect(subnavForSection('/employees', permissionsForRole('shift_leader'))).toEqual([]);
    // Clients & Agreements is admin-only entirely → empty for shift_leader.
    expect(subnavForSection('/client-companies', permissionsForRole('shift_leader'))).toEqual([]);
  });

  it('every sub-nav route is a real, distinct destination', () => {
    for (const subs of Object.values(SECTION_SUBNAV)) {
      const tos = subs.map((s) => s.to);
      expect(new Set(tos).size).toBe(tos.length);
    }
  });
});
