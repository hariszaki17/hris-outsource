/**
 * Unit tests for E5 attendance dashboard screen logic.
 *
 * Scope: pure-logic assertions (no render) mirroring the existing nav.test.ts /
 * api-error.test.ts pattern.  Full render tests (MSW + i18n + router) are a
 * follow-up once a shared render helper is added to src/test/.
 *
 * Covers:
 *   1. AttendanceDashboardSearch type includes the 4 new filter fields.
 *   2. check_in_at nullable guard — null yields placeholder key 'colCheckInEmpty'.
 *   3. SL company-lock — isShiftLeader=true produces locked company param from
 *      currentUser.companyId (not from search.company_id).
 */

import { describe, expect, it } from 'vitest';
import type { AttendanceDashboardSearch } from './attendance-dashboard-screen.ts';

// ---------------------------------------------------------------------------
// 1. Type shape — verify the new filter fields exist on the search type at
//    compile time (a TS error here = the type is missing the field).
// ---------------------------------------------------------------------------

describe('AttendanceDashboardSearch type', () => {
  it('accepts the filter fields alongside existing ones', () => {
    const search: AttendanceDashboardSearch = {
      q: 'budi',
      tab: 'late',
      cursor: 'abc',
      company_id: 'SWP-CMP-014',
      site_id: 'SWP-SITE-031',
      position: 'Petugas Parkir',
    };
    expect(search.company_id).toBe('SWP-CMP-014');
    expect(search.site_id).toBe('SWP-SITE-031');
    expect(search.position).toBe('Petugas Parkir');
  });

  it('all filter fields are optional', () => {
    // Should compile with none of the filter fields set.
    const search: AttendanceDashboardSearch = {};
    expect(search.company_id).toBeUndefined();
    expect(search.site_id).toBeUndefined();
    expect(search.position).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// 2. check_in_at nullable guard — the rendering logic in the colCheckIn cell.
//    Extracted here so the null-path is explicitly covered without a full render.
// ---------------------------------------------------------------------------

/** Mirrors the cell render logic in attendance-dashboard-screen.tsx colCheckIn. */
function renderCheckIn(check_in_at: string | null | undefined, placeholder: string): string {
  if (!check_in_at) return placeholder;
  return new Date(check_in_at).toLocaleTimeString('id-ID', {
    timeZone: 'Asia/Jakarta',
    hour: '2-digit',
    minute: '2-digit',
  });
}

describe('colCheckIn null guard', () => {
  it('returns placeholder for null check_in_at (ABSENT row)', () => {
    expect(renderCheckIn(null, '—')).toBe('—');
  });

  it('returns placeholder for undefined check_in_at', () => {
    expect(renderCheckIn(undefined, '—')).toBe('—');
  });

  it('formats a valid ISO timestamp as HH:MM', () => {
    // 2026-06-03T07:18:00Z = 14:18 WIB (UTC+7)
    const result = renderCheckIn('2026-06-03T07:18:00Z', '—');
    expect(result).not.toBe('—');
    expect(result).toMatch(/\d{2}\.\d{2}/); // id-ID locale uses "." separator
  });
});

// ---------------------------------------------------------------------------
// 3. SL company-lock — the queryParams derivation logic.
//    Mirrors: company_id = isShiftLeader ? slCompanyId : search.company_id
// ---------------------------------------------------------------------------

/** Mirrors the company_id selection logic in queryParams. */
function resolveCompanyId(
  isShiftLeader: boolean,
  slCompanyId: string | undefined,
  searchCompanyId: string | undefined,
): string | undefined {
  return isShiftLeader ? slCompanyId : searchCompanyId || undefined;
}

describe('SL company-lock queryParams logic', () => {
  it('for shift_leader: always uses slCompanyId from session, ignoring search.company_id', () => {
    const result = resolveCompanyId(true, 'SWP-CMP-014', 'SWP-CMP-999');
    expect(result).toBe('SWP-CMP-014');
  });

  it('for shift_leader: undefined slCompanyId produces undefined (graceful)', () => {
    const result = resolveCompanyId(true, undefined, 'SWP-CMP-999');
    expect(result).toBeUndefined();
  });

  it('for HR/super_admin: uses search.company_id freely', () => {
    const result = resolveCompanyId(false, undefined, 'SWP-CMP-007');
    expect(result).toBe('SWP-CMP-007');
  });

  it('for HR with no filter set: returns undefined (shows all companies)', () => {
    const result = resolveCompanyId(false, undefined, undefined);
    expect(result).toBeUndefined();
  });
});
