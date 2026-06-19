/**
 * Unit tests for E7 overtime-records-screen (Rekap Lembur).
 *
 * Pure-logic assertions (no DOM render) mirroring the e5 attendance / e6 leave test patterns.
 * Covers:
 *   1. OvertimeRecordsSearch type shape.
 *   2. formatOtMinutes helper (overtime-shared.tsx).
 *   3. overtimeStatusTone mapping.
 *   4. overtimeTierTone / overtimeTierKey (day-type ↔ tone/label).
 *   5. overtimeSourceTone / overtimeSourceKey (capture path).
 *   6. hasActiveFilters guard.
 *   7. sumTierMinutes / totalApprovedMinutes aggregation helpers.
 */

import { describe, expect, it } from 'vitest';
import type { OvertimeRecordsSearch } from './overtime-records-screen.ts';
import {
  formatOtMinutes,
  overtimeSourceKey,
  overtimeSourceTone,
  overtimeStatusKey,
  overtimeStatusTone,
  overtimeTierKey,
  overtimeTierTone,
} from './overtime-shared.tsx';
import { OvertimeSource, OvertimeStatus, OvertimeTier } from '@swp/api-client/e7';
import type { Overtime } from '@swp/api-client/e7';

// ---------------------------------------------------------------------------
// 1. OvertimeRecordsSearch type
// ---------------------------------------------------------------------------

describe('OvertimeRecordsSearch type', () => {
  it('accepts all filter fields + cursor', () => {
    const s: OvertimeRecordsSearch = {
      q: 'Budi',
      company_id: 'SWP-CMP-0021',
      work_date__gte: '2026-06-01',
      work_date__lte: '2026-06-30',
      status: OvertimeStatus.APPROVED,
      tier: OvertimeTier.WORKDAY,
      source: OvertimeSource.REQUESTED,
      cursor: 'abc123',
    };
    expect(s.q).toBe('Budi');
    expect(s.company_id).toBe('SWP-CMP-0021');
    expect(s.work_date__gte).toBe('2026-06-01');
    expect(s.status).toBe(OvertimeStatus.APPROVED);
    expect(s.tier).toBe(OvertimeTier.WORKDAY);
    expect(s.source).toBe(OvertimeSource.REQUESTED);
  });

  it('all fields are optional', () => {
    const s: OvertimeRecordsSearch = {};
    expect(s.q).toBeUndefined();
    expect(s.company_id).toBeUndefined();
    expect(s.cursor).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// 2. formatOtMinutes (overtime-shared.tsx)
// ---------------------------------------------------------------------------

describe('formatOtMinutes', () => {
  it('converts 1380 minutes to "23j" (zero minutes omitted)', () => {
    expect(formatOtMinutes(1380)).toBe('23j');
  });

  it('converts 30 minutes to "30m" (zero hours omitted)', () => {
    expect(formatOtMinutes(30)).toBe('30m');
  });

  it('converts 0 minutes to "0m"', () => {
    expect(formatOtMinutes(0)).toBe('0m');
  });

  it('converts 60 minutes to "1j" (zero minutes omitted)', () => {
    expect(formatOtMinutes(60)).toBe('1j');
  });

  it('converts 150 minutes to "2j 30m"', () => {
    expect(formatOtMinutes(150)).toBe('2j 30m');
  });

  it('converts 90 minutes to "1j 30m"', () => {
    expect(formatOtMinutes(90)).toBe('1j 30m');
  });
});

// ---------------------------------------------------------------------------
// 3. overtimeStatusTone — lifecycle status → StatusBadge tone
// ---------------------------------------------------------------------------

describe('overtimeStatusTone', () => {
  it('APPROVED → ok', () => {
    expect(overtimeStatusTone(OvertimeStatus.APPROVED)).toBe('ok');
  });

  it('PENDING → onprogress', () => {
    expect(overtimeStatusTone(OvertimeStatus.PENDING)).toBe('onprogress');
  });

  it('PENDING_AGENT_CONFIRM → onprogress', () => {
    expect(overtimeStatusTone(OvertimeStatus.PENDING_AGENT_CONFIRM)).toBe('onprogress');
  });

  it('REJECTED → bad', () => {
    expect(overtimeStatusTone(OvertimeStatus.REJECTED)).toBe('bad');
  });

  it('CANCELLED → neutral', () => {
    expect(overtimeStatusTone(OvertimeStatus.CANCELLED)).toBe('neutral');
  });
});

// ---------------------------------------------------------------------------
// 4. overtimeStatusKey — i18n key generation
// ---------------------------------------------------------------------------

describe('overtimeStatusKey', () => {
  it('generates "status.APPROVED" for APPROVED', () => {
    expect(overtimeStatusKey(OvertimeStatus.APPROVED)).toBe('status.APPROVED');
  });

  it('generates "status.REJECTED" for REJECTED', () => {
    expect(overtimeStatusKey(OvertimeStatus.REJECTED)).toBe('status.REJECTED');
  });
});

// ---------------------------------------------------------------------------
// 5. overtimeTierTone / overtimeTierKey — day-type tier
// ---------------------------------------------------------------------------

describe('overtimeTierTone', () => {
  it('HOLIDAY → info', () => {
    expect(overtimeTierTone(OvertimeTier.HOLIDAY)).toBe('info');
  });

  it('RESTDAY → warn', () => {
    expect(overtimeTierTone(OvertimeTier.RESTDAY)).toBe('warn');
  });

  it('WORKDAY → neutral', () => {
    expect(overtimeTierTone(OvertimeTier.WORKDAY)).toBe('neutral');
  });
});

describe('overtimeTierKey', () => {
  it('generates "tier.WORKDAY" for WORKDAY', () => {
    expect(overtimeTierKey(OvertimeTier.WORKDAY)).toBe('tier.WORKDAY');
  });

  it('generates "tier.RESTDAY" for RESTDAY', () => {
    expect(overtimeTierKey(OvertimeTier.RESTDAY)).toBe('tier.RESTDAY');
  });

  it('generates "tier.HOLIDAY" for HOLIDAY', () => {
    expect(overtimeTierKey(OvertimeTier.HOLIDAY)).toBe('tier.HOLIDAY');
  });
});

// ---------------------------------------------------------------------------
// 6. overtimeSourceTone / overtimeSourceKey — capture path
// ---------------------------------------------------------------------------

describe('overtimeSourceTone', () => {
  it('REQUESTED → neutral', () => {
    expect(overtimeSourceTone(OvertimeSource.REQUESTED)).toBe('neutral');
  });

  it('AUTO_DETECTED → info', () => {
    expect(overtimeSourceTone(OvertimeSource.AUTO_DETECTED)).toBe('info');
  });

  it('WORKED_WITHOUT_REQUEST → warn', () => {
    expect(overtimeSourceTone(OvertimeSource.WORKED_WITHOUT_REQUEST)).toBe('warn');
  });
});

describe('overtimeSourceKey', () => {
  it('generates "source.REQUESTED.label" for REQUESTED', () => {
    expect(overtimeSourceKey(OvertimeSource.REQUESTED)).toBe('source.REQUESTED.label');
  });

  it('generates "source.AUTO_DETECTED.label" for AUTO_DETECTED', () => {
    expect(overtimeSourceKey(OvertimeSource.AUTO_DETECTED)).toBe('source.AUTO_DETECTED.label');
  });
});

// ---------------------------------------------------------------------------
// 7. hasActiveFilters (inline in overtime-records-screen.tsx)
// ---------------------------------------------------------------------------

function hasActiveFilters(s: OvertimeRecordsSearch): boolean {
  return Boolean(
    s.q || s.company_id || s.work_date__gte || s.work_date__lte || s.status || s.tier || s.source,
  );
}

describe('hasActiveFilters', () => {
  it('returns false for an empty search', () => {
    expect(hasActiveFilters({})).toBe(false);
  });

  it('returns true when q is set', () => {
    expect(hasActiveFilters({ q: 'Budi' })).toBe(true);
  });

  it('returns true when company_id is set', () => {
    expect(hasActiveFilters({ company_id: 'SWP-CMP-0021' })).toBe(true);
  });

  it('returns true when status filter is set', () => {
    expect(hasActiveFilters({ status: OvertimeStatus.APPROVED })).toBe(true);
  });

  it('returns true when tier filter is set', () => {
    expect(hasActiveFilters({ tier: OvertimeTier.HOLIDAY })).toBe(true);
  });

  it('returns false when only cursor is set (cursor is not a filter)', () => {
    expect(hasActiveFilters({ cursor: 'abc' })).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 8. sumTierMinutes / totalApprovedMinutes aggregation helpers
// ---------------------------------------------------------------------------

function sumTierMinutes(rows: Overtime[], tier: OvertimeTier): number {
  return rows
    .filter((r) => r.tier_indicator === tier && r.status === OvertimeStatus.APPROVED)
    .reduce((acc, r) => acc + r.calculation.counted_minutes, 0);
}

function totalApprovedMinutes(rows: Overtime[]): number {
  return rows
    .filter((r) => r.status === OvertimeStatus.APPROVED)
    .reduce((acc, r) => acc + r.calculation.counted_minutes, 0);
}

const mockRow = (tier: OvertimeTier, status: OvertimeStatus, minutes: number): Overtime =>
  ({
    id: `SWP-OT-${tier}-${status}`,
    tier_indicator: tier,
    status,
    calculation: { counted_minutes: minutes },
    employee: { id: 'SWP-EMP-1001', name: 'Test Agent' },
    company: { id: 'SWP-CMP-001', name: 'Test Co' },
  }) as unknown as Overtime;

describe('sumTierMinutes', () => {
  const rows: Overtime[] = [
    mockRow(OvertimeTier.WORKDAY, OvertimeStatus.APPROVED, 120),
    mockRow(OvertimeTier.WORKDAY, OvertimeStatus.APPROVED, 90),
    mockRow(OvertimeTier.WORKDAY, OvertimeStatus.REJECTED, 60),
    mockRow(OvertimeTier.RESTDAY, OvertimeStatus.APPROVED, 180),
    mockRow(OvertimeTier.HOLIDAY, OvertimeStatus.APPROVED, 240),
    mockRow(OvertimeTier.HOLIDAY, OvertimeStatus.PENDING, 120),
  ];

  it('sums only approved WORKDAY minutes', () => {
    expect(sumTierMinutes(rows, OvertimeTier.WORKDAY)).toBe(210); // 120+90
  });

  it('sums only approved RESTDAY minutes', () => {
    expect(sumTierMinutes(rows, OvertimeTier.RESTDAY)).toBe(180);
  });

  it('sums only approved HOLIDAY minutes', () => {
    expect(sumTierMinutes(rows, OvertimeTier.HOLIDAY)).toBe(240);
  });

  it('returns 0 for tier with no approved records', () => {
    expect(sumTierMinutes([], OvertimeTier.WORKDAY)).toBe(0);
  });

  it('ignores PENDING rows even when tier matches', () => {
    expect(sumTierMinutes(rows, OvertimeTier.HOLIDAY)).toBe(240);
  });
});

describe('totalApprovedMinutes', () => {
  const rows: Overtime[] = [
    mockRow(OvertimeTier.WORKDAY, OvertimeStatus.APPROVED, 60),
    mockRow(OvertimeTier.RESTDAY, OvertimeStatus.APPROVED, 120),
    mockRow(OvertimeTier.HOLIDAY, OvertimeStatus.APPROVED, 90),
    mockRow(OvertimeTier.WORKDAY, OvertimeStatus.REJECTED, 200),
    mockRow(OvertimeTier.HOLIDAY, OvertimeStatus.PENDING, 50),
  ];

  it('sums all approved minutes across tiers', () => {
    expect(totalApprovedMinutes(rows)).toBe(270); // 60+120+90
  });

  it('excludes REJECTED and PENDING rows', () => {
    expect(totalApprovedMinutes(rows)).toBe(270);
  });

  it('returns 0 for empty list', () => {
    expect(totalApprovedMinutes([])).toBe(0);
  });
});
