import type { Role } from '@swp/shared';
import { describe, expect, it } from 'vitest';
import {
  NAV_ITEMS,
  SECTION_SUBNAV,
  SETTINGS_ITEM,
  activeSection,
  navForRole,
  subnavForSection,
} from './nav.ts';

/**
 * Role-aware nav gating (interim x-rbac map, ENGINEERING.md A2/C1). Asserts the coarse
 * module visibility the shell relies on. The API remains the real gate — this is the
 * defense-in-depth client view.
 */
const labels = (role: Role) => navForRole(NAV_ITEMS, role).map((i) => i.labelKey);

describe('navForRole', () => {
  it('the sidebar holds exactly the 8 design modules (comp/Sidebar iCqTB)', () => {
    expect(NAV_ITEMS.map((i) => i.labelKey)).toEqual([
      'nav.dashboard',
      'nav.employees',
      'nav.placements',
      'nav.scheduleShift',
      'nav.attendance',
      'nav.leave',
      'nav.overtime',
      'nav.reports',
    ]);
  });

  it('super_admin and hr_admin see every primary module + settings', () => {
    for (const role of ['super_admin', 'hr_admin'] as const) {
      expect(labels(role)).toEqual(NAV_ITEMS.map((i) => i.labelKey));
      expect(SETTINGS_ITEM.roles.includes(role)).toBe(true);
    }
  });

  it('shift_leader is scoped: sees operational modules, not reports/settings', () => {
    const sl = labels('shift_leader');
    expect(sl).toEqual([
      'nav.dashboard',
      'nav.employees',
      'nav.placements',
      'nav.scheduleShift',
      'nav.attendance',
      'nav.leave',
      'nav.overtime',
    ]);
    expect(sl).not.toContain('nav.reports');
    expect(SETTINGS_ITEM.roles.includes('shift_leader')).toBe(false);
  });

  it('agent (mobile-only) sees no web nav', () => {
    expect(labels('agent')).toEqual([]);
  });
});

describe('section sub-nav (sub-features live under their parent module)', () => {
  it('client companies / master data resolve to the Karyawan section, not the sidebar', () => {
    expect(NAV_ITEMS.map((i) => i.to)).not.toContain('/client-companies');
    expect(NAV_ITEMS.map((i) => i.to)).not.toContain('/payroll');
    expect(activeSection('/client-companies')).toBe('/employees');
    expect(activeSection('/master-data')).toBe('/employees');
    expect(activeSection('/master-data/overtime-rules')).toBe('/employees');
  });

  it('payroll lives under Laporan; corrections under Kehadiran (DESIGN-SYSTEM IA)', () => {
    expect(activeSection('/payroll')).toBe('/reports');
    expect(activeSection('/payroll/SWP-PS-1')).toBe('/reports');
    expect(activeSection('/corrections')).toBe('/attendance');
  });

  it('leave/quotas resolves to the Cuti section over the bare /leave prefix', () => {
    expect(activeSection('/leave/quotas')).toBe('/leave');
    expect(activeSection('/leave')).toBe('/leave');
  });

  it('shift_leader sub-nav hides admin-only tabs (Kuota under Cuti)', () => {
    const slLeave = subnavForSection('/leave', 'shift_leader').map((s) => s.labelKey);
    expect(slLeave).toContain('nav.leaveApprovals');
    expect(slLeave).toContain('nav.leaveCalendar');
    expect(slLeave).not.toContain('nav.leaveQuotas');
  });

  it('a section with <2 visible tabs for a role renders no sub-nav', () => {
    // Karyawan: shift_leader only sees /employees → no sub-nav strip.
    expect(subnavForSection('/employees', 'shift_leader')).toEqual([]);
    // Reports section is admin-only entirely → empty for shift_leader.
    expect(subnavForSection('/reports', 'shift_leader')).toEqual([]);
  });

  it('every sub-nav route is a real, distinct destination', () => {
    for (const subs of Object.values(SECTION_SUBNAV)) {
      const tos = subs.map((s) => s.to);
      expect(new Set(tos).size).toBe(tos.length);
    }
  });
});
