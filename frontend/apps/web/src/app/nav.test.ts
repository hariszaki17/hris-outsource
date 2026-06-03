import type { Role } from '@swp/shared';
import { describe, expect, it } from 'vitest';
import { NAV_ITEMS, SETTINGS_ITEM, navForRole } from './nav.ts';

/**
 * Role-aware nav gating (interim x-rbac map, ENGINEERING.md A2/C1). Asserts the coarse
 * module visibility the shell relies on. The API remains the real gate — this is the
 * defense-in-depth client view.
 */
const labels = (role: Role) => navForRole(NAV_ITEMS, role).map((i) => i.labelKey);

describe('navForRole', () => {
  it('super_admin and hr_admin see every primary module + settings', () => {
    for (const role of ['super_admin', 'hr_admin'] as const) {
      expect(labels(role)).toEqual(NAV_ITEMS.map((i) => i.labelKey));
      expect(SETTINGS_ITEM.roles.includes(role)).toBe(true);
    }
  });

  it('shift_leader is scoped: no employee master, no reports, no settings', () => {
    const sl = labels('shift_leader');
    expect(sl).not.toContain('nav.employees');
    expect(sl).not.toContain('nav.reports');
    expect(SETTINGS_ITEM.roles.includes('shift_leader')).toBe(false);
    // but keeps the operational modules they supervise
    expect(sl).toEqual(
      expect.arrayContaining([
        'nav.dashboard',
        'nav.placements',
        'nav.schedule',
        'nav.attendance',
        'nav.leave',
        'nav.overtime',
      ]),
    );
  });

  it('agent (mobile-only) sees no web nav', () => {
    expect(labels('agent')).toEqual([]);
  });
});
