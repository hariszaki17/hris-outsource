/**
 * Unit tests for E3 placements-screen (Penempatan — Daftar per Perusahaan Klien).
 *
 * Pure-logic assertions (no DOM render) mirroring the e5 attendance / e6 leave test patterns.
 * Covers:
 *   1. PlacementsSearch type shape.
 *   2. initials helper (avatar abbreviation).
 *   3. lifecycleTone mapping (lifecycle status → StatusBadge tone).
 *   4. SL company-lock logic (isShiftLeader pins company_id).
 */

import { describe, expect, it } from 'vitest';
import type { PlacementsSearch } from './placements-screen.ts';

// ---------------------------------------------------------------------------
// 1. PlacementsSearch type
// ---------------------------------------------------------------------------

describe('PlacementsSearch type', () => {
  it('accepts all filter fields', () => {
    const s: PlacementsSearch = {
      q: 'Budi',
      company_id: 'SWP-CMP-0021',
      position: 'Petugas Parkir',
      status: 'ACTIVE',
      expiring_soon: true,
      awaiting_agreement: false,
      cursor: 'abc123',
    };
    expect(s.q).toBe('Budi');
    expect(s.company_id).toBe('SWP-CMP-0021');
    expect(s.position).toBe('Petugas Parkir');
    expect(s.status).toBe('ACTIVE');
    expect(s.expiring_soon).toBe(true);
    expect(s.awaiting_agreement).toBe(false);
  });

  it('all fields are optional', () => {
    const s: PlacementsSearch = {};
    expect(s.q).toBeUndefined();
    expect(s.company_id).toBeUndefined();
    expect(s.position).toBeUndefined();
    expect(s.status).toBeUndefined();
    expect(s.expiring_soon).toBeUndefined();
    expect(s.cursor).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// 2. initials helper (inline in placements-screen.tsx)
// ---------------------------------------------------------------------------

function initials(name: string): string {
  return name
    .split(' ')
    .slice(0, 2)
    .map((p) => p[0] ?? '')
    .join('')
    .toUpperCase();
}

describe('initials', () => {
  it('"Budi Santoso" → "BS"', () => {
    expect(initials('Budi Santoso')).toBe('BS');
  });

  it('"Dewi Lestari" → "DL"', () => {
    expect(initials('Dewi Lestari')).toBe('DL');
  });

  it('single name "Sari" → "S"', () => {
    expect(initials('Sari')).toBe('S');
  });

  it('three-part name "Rudi Wijaya Putra" → "RW" (first 2 parts only)', () => {
    expect(initials('Rudi Wijaya Putra')).toBe('RW');
  });

  it('empty string → ""', () => {
    expect(initials('')).toBe('');
  });

  it('lowercase input → uppercase output', () => {
    expect(initials('budi santoso')).toBe('BS');
  });
});

// ---------------------------------------------------------------------------
// 3. lifecycleTone mapping (inline in placements-screen.tsx)
// ---------------------------------------------------------------------------

import type { StatusTone } from '@swp/design-tokens';
import type { PlacementLifecycleStatus } from '@swp/api-client/e3';

const lifecycleTone: Record<PlacementLifecycleStatus, StatusTone> = {
  ACTIVE: 'ok',
  EXTENDED: 'ok',
  PENDING_START: 'info',
  EXPIRING: 'warn',
  ENDED: 'neutral',
  TRANSFERRED: 'neutral',
  SUPERSEDED: 'neutral',
  TERMINATED: 'bad',
  RESIGNED: 'bad',
};

describe('lifecycleTone', () => {
  it('ACTIVE → ok', () => {
    expect(lifecycleTone['ACTIVE']).toBe('ok');
  });

  it('EXTENDED → ok', () => {
    expect(lifecycleTone['EXTENDED']).toBe('ok');
  });

  it('PENDING_START → info', () => {
    expect(lifecycleTone['PENDING_START']).toBe('info');
  });

  it('EXPIRING → warn', () => {
    expect(lifecycleTone['EXPIRING']).toBe('warn');
  });

  it('ENDED → neutral', () => {
    expect(lifecycleTone['ENDED']).toBe('neutral');
  });

  it('TRANSFERRED → neutral', () => {
    expect(lifecycleTone['TRANSFERRED']).toBe('neutral');
  });

  it('SUPERSEDED → neutral', () => {
    expect(lifecycleTone['SUPERSEDED']).toBe('neutral');
  });

  it('TERMINATED → bad', () => {
    expect(lifecycleTone['TERMINATED']).toBe('bad');
  });

  it('RESIGNED → bad', () => {
    expect(lifecycleTone['RESIGNED']).toBe('bad');
  });

  it('covers all known lifecycle statuses', () => {
    const expectedStatuses: PlacementLifecycleStatus[] = [
      'ACTIVE',
      'EXTENDED',
      'PENDING_START',
      'EXPIRING',
      'ENDED',
      'TRANSFERRED',
      'SUPERSEDED',
      'TERMINATED',
      'RESIGNED',
    ];
    for (const s of expectedStatuses) {
      expect(lifecycleTone[s]).toBeDefined();
    }
  });
});

// ---------------------------------------------------------------------------
// 4. SL company-lock logic (mirrors the query-param derivation)
// ---------------------------------------------------------------------------

function resolveCompanyId(
  isShiftLeader: boolean,
  slCompanyId: string | undefined,
  searchCompanyId: string | undefined,
): string | undefined {
  return isShiftLeader ? slCompanyId : searchCompanyId || undefined;
}

describe('SL company-lock (queryParams derivation)', () => {
  it('for shift_leader: always uses slCompanyId, ignoring search.company_id', () => {
    expect(resolveCompanyId(true, 'SWP-CMP-0021', 'SWP-CMP-999')).toBe('SWP-CMP-0021');
  });

  it('for shift_leader: undefined slCompanyId produces undefined (graceful)', () => {
    expect(resolveCompanyId(true, undefined, 'SWP-CMP-999')).toBeUndefined();
  });

  it('for HR: uses search.company_id freely', () => {
    expect(resolveCompanyId(false, undefined, 'SWP-CMP-007')).toBe('SWP-CMP-007');
  });

  it('for HR with no filter: returns undefined (shows all companies)', () => {
    expect(resolveCompanyId(false, undefined, undefined)).toBeUndefined();
  });
});
