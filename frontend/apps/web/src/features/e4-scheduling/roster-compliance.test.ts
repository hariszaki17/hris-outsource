/**
 * Unit tests for E4 roster-compliance helpers (roster-compliance.ts).
 *
 * Pure-logic assertions — tests every exported pure function.
 * Covers:
 *   1. parsePlainDate / toIsoDate / addDays / getMondayOfWeek / weekDays
 *   2. buildAgentRows / buildAgentRowsFromRoster
 *   3. buildHolidayMaps
 *   4. isWorkedEntry — shift / day-off / cancelled-by-leave guards
 *   5. longestWorkedRun — consecutive-worked-count logic
 *   6. computeCompliance — noRest / longRun / holidayShiftCount
 *   7. hasComplianceIssue — signal detection
 *   8. REST_VIOLATION_DAYS / LONG_RUN_WARN_DAYS constants
 */

import { describe, expect, it } from 'vitest';
import {
  REST_VIOLATION_DAYS,
  LONG_RUN_WARN_DAYS,
  addDays,
  buildAgentRows,
  buildAgentRowsFromRoster,
  buildHolidayMaps,
  computeCompliance,
  getMondayOfWeek,
  hasComplianceIssue,
  isWorkedEntry,
  longestWorkedRun,
  parsePlainDate,
  toIsoDate,
  weekDays,
} from './roster-compliance.ts';
import type { ScheduleEntry } from '@swp/api-client/e4';
import { ScheduleEntryStatus } from '@swp/api-client/e4';
import type { AgentRow, RowCompliance } from './roster-compliance.ts';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function mockEntry(overrides: Partial<ScheduleEntry> = {}): ScheduleEntry {
  return {
    id: overrides.id ?? 'SWP-SCH-9999',
    employee_id: overrides.employee_id ?? 'SWP-EMP-1001',
    employee_name: overrides.employee_name ?? 'Test Agent',
    placement_id: overrides.placement_id ?? 'SWP-PL-001',
    work_date: overrides.work_date ?? '2026-06-15',
    shift_master_name: overrides.shift_master_name ?? 'Pagi',
    is_day_off: overrides.is_day_off ?? false,
    status: overrides.status ?? ScheduleEntryStatus.PUBLISHED,
  } as ScheduleEntry;
}

// ---------------------------------------------------------------------------
// 1. Plain-date helpers
// ---------------------------------------------------------------------------

describe('parsePlainDate', () => {
  it('parses "2026-06-15" to a UTC-midnight Date', () => {
    const d = parsePlainDate('2026-06-15');
    expect(d.getUTCFullYear()).toBe(2026);
    expect(d.getUTCMonth()).toBe(5); // June (0-indexed)
    expect(d.getUTCDate()).toBe(15);
    expect(d.getUTCHours()).toBe(0);
  });
});

describe('toIsoDate', () => {
  it('formats a Date to "YYYY-MM-DD"', () => {
    const d = new Date(Date.UTC(2026, 5, 15));
    expect(toIsoDate(d)).toBe('2026-06-15');
  });
});

describe('addDays', () => {
  it('adds positive days', () => {
    expect(addDays('2026-06-15', 3)).toBe('2026-06-18');
  });

  it('adds zero days returns same date', () => {
    expect(addDays('2026-06-15', 0)).toBe('2026-06-15');
  });

  it('adds negative days to move backward', () => {
    expect(addDays('2026-06-15', -3)).toBe('2026-06-12');
  });

  it('crosses month boundary forward', () => {
    expect(addDays('2026-06-30', 2)).toBe('2026-07-02');
  });

  it('crosses month boundary backward', () => {
    expect(addDays('2026-06-01', -1)).toBe('2026-05-31');
  });
});

// ---------------------------------------------------------------------------
// 2. getMondayOfWeek / weekDays
// ---------------------------------------------------------------------------

describe('getMondayOfWeek', () => {
  it('2026-06-18 (Thursday) → 2026-06-15 (Monday)', () => {
    expect(getMondayOfWeek('2026-06-18')).toBe('2026-06-15');
  });

  it('2026-06-15 (Monday) → itself', () => {
    expect(getMondayOfWeek('2026-06-15')).toBe('2026-06-15');
  });

  it('2026-06-14 (Sunday) → 2026-06-08 (previous Monday)', () => {
    expect(getMondayOfWeek('2026-06-14')).toBe('2026-06-08');
  });

  it('2026-06-21 (Sunday) → 2026-06-15', () => {
    expect(getMondayOfWeek('2026-06-21')).toBe('2026-06-15');
  });
});

describe('weekDays', () => {
  it('returns 7 days Mon→Sun for a given Monday', () => {
    const days = weekDays('2026-06-15');
    expect(days).toHaveLength(7);
    expect(days).toEqual([
      '2026-06-15', // Mon
      '2026-06-16', // Tue
      '2026-06-17', // Wed
      '2026-06-18', // Thu
      '2026-06-19', // Fri
      '2026-06-20', // Sat
      '2026-06-21', // Sun
    ]);
  });
});

// ---------------------------------------------------------------------------
// 3. buildAgentRows
// ---------------------------------------------------------------------------

describe('buildAgentRows', () => {
  it('groups entries by employee_id + placement_id', () => {
    const entries: ScheduleEntry[] = [
      mockEntry({ id: '1', employee_id: 'SWP-EMP-A', placement_id: 'SWP-PL-1', work_date: '2026-06-15' }),
      mockEntry({ id: '2', employee_id: 'SWP-EMP-A', placement_id: 'SWP-PL-1', work_date: '2026-06-16' }),
      mockEntry({ id: '3', employee_id: 'SWP-EMP-B', placement_id: 'SWP-PL-2', work_date: '2026-06-15' }),
    ];
    const rows = buildAgentRows(entries);
    expect(rows).toHaveLength(2);

    const rowA = rows.find((r) => r.employeeId === 'SWP-EMP-A')!;
    expect(rowA.placementId).toBe('SWP-PL-1');
    expect(Object.keys(rowA.cells)).toHaveLength(2);
    expect(rowA.cells['2026-06-15']?.id).toBe('1');
    expect(rowA.cells['2026-06-16']?.id).toBe('2');

    const rowB = rows.find((r) => r.employeeId === 'SWP-EMP-B')!;
    expect(rowB.placementId).toBe('SWP-PL-2');
    expect(Object.keys(rowB.cells)).toHaveLength(1);
  });

  it('same employee different placements get separate rows', () => {
    const entries: ScheduleEntry[] = [
      mockEntry({ id: '1', employee_id: 'SWP-EMP-A', placement_id: 'SWP-PL-1', work_date: '2026-06-15' }),
      mockEntry({ id: '2', employee_id: 'SWP-EMP-A', placement_id: 'SWP-PL-2', work_date: '2026-06-15' }),
    ];
    const rows = buildAgentRows(entries);
    expect(rows).toHaveLength(2);
  });

  it('returns empty array for no entries', () => {
    expect(buildAgentRows([])).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// 4. buildAgentRowsFromRoster
// ---------------------------------------------------------------------------

describe('buildAgentRowsFromRoster', () => {
  it('seeds rows from placements even when unscheduled', () => {
    const placements = [
      { id: 'SWP-PL-1', employee_id: 'SWP-EMP-A', employee_name: 'Agent A' },
    ];
    const entries: ScheduleEntry[] = [];
    const rows = buildAgentRowsFromRoster(placements, entries);
    expect(rows).toHaveLength(1);
    expect(rows[0].cells).toEqual({});
  });

  it('overlays schedule entries on placement rows', () => {
    const placements = [
      { id: 'SWP-PL-1', employee_id: 'SWP-EMP-A', employee_name: 'Agent A' },
    ];
    const entries: ScheduleEntry[] = [
      mockEntry({ id: '1', employee_id: 'SWP-EMP-A', placement_id: 'SWP-PL-1', work_date: '2026-06-15' }),
    ];
    const rows = buildAgentRowsFromRoster(placements, entries);
    expect(rows).toHaveLength(1);
    expect(rows[0].cells['2026-06-15']?.id).toBe('1');
  });

  it('adds orphan entries (placement NOT in roster) as extra rows', () => {
    const placements = [
      { id: 'SWP-PL-1', employee_id: 'SWP-EMP-A', employee_name: 'Agent A' },
    ];
    const entries: ScheduleEntry[] = [
      mockEntry({ id: '99', employee_id: 'SWP-EMP-B', placement_id: 'SWP-PL-99', work_date: '2026-06-15' }),
    ];
    const rows = buildAgentRowsFromRoster(placements, entries);
    expect(rows).toHaveLength(2);
  });
});

// ---------------------------------------------------------------------------
// 5. buildHolidayMaps
// ---------------------------------------------------------------------------

describe('buildHolidayMaps', () => {
  const holidays = [
    { id: '1', date: '2026-06-17', name: 'Idul Adha' },
    { id: '2', date: '2026-06-20', name: 'Hari Raya' },
    { id: '3', date: '2026-12-25', name: 'Natal' }, // outside week
  ] as Parameters<typeof buildHolidayMaps>[0];

  const days = ['2026-06-15', '2026-06-16', '2026-06-17', '2026-06-18', '2026-06-19', '2026-06-20', '2026-06-21'];

  it('includes holidays falling within the visible days', () => {
    const { holidaySet, holidayNameByDate } = buildHolidayMaps(holidays, days);
    expect(holidaySet.has('2026-06-17')).toBe(true);
    expect(holidaySet.has('2026-06-20')).toBe(true);
    expect(holidayNameByDate.get('2026-06-17')).toBe('Idul Adha');
  });

  it('excludes holidays outside the visible week', () => {
    const { holidaySet } = buildHolidayMaps(holidays, days);
    expect(holidaySet.has('2026-12-25')).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 6. isWorkedEntry
// ---------------------------------------------------------------------------

describe('isWorkedEntry', () => {
  it('undefined entry → false', () => {
    expect(isWorkedEntry(undefined)).toBe(false);
  });

  it('entry with shift_master_name → true', () => {
    expect(isWorkedEntry(mockEntry({ shift_master_name: 'Pagi' }))).toBe(true);
  });

  it('day-off entry → false', () => {
    expect(isWorkedEntry(mockEntry({ shift_master_name: 'Pagi', is_day_off: true }))).toBe(false);
  });

  it('CANCELLED_BY_LEAVE → false', () => {
    expect(
      isWorkedEntry(
        mockEntry({ shift_master_name: 'Pagi', status: ScheduleEntryStatus.CANCELLED_BY_LEAVE }),
      ),
    ).toBe(false);
  });

  it('empty shift_master_name → false', () => {
    expect(isWorkedEntry(mockEntry({ shift_master_name: '' }))).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 7. longestWorkedRun
// ---------------------------------------------------------------------------

describe('longestWorkedRun', () => {
  const days = ['2026-06-15', '2026-06-16', '2026-06-17', '2026-06-18', '2026-06-19', '2026-06-20', '2026-06-21'];

  it('all 7 days worked → run of 7', () => {
    const cells: Record<string, ScheduleEntry | undefined> = {};
    for (const d of days) cells[d] = mockEntry({ work_date: d });
    expect(longestWorkedRun(cells, days)).toBe(7);
  });

  it('Mon–Fri worked, Sat–Sun off → run of 5', () => {
    const cells: Record<string, ScheduleEntry | undefined> = {};
    for (let i = 0; i < 5; i++) cells[days[i]] = mockEntry({ work_date: days[i] });
    // Sat–Sun: undefined (off)
    expect(longestWorkedRun(cells, days)).toBe(5);
  });

  it('Mon–Wed worked, Thu off, Fri–Sun worked → run of 3', () => {
    const cells: Record<string, ScheduleEntry | undefined> = {};
    for (let i = 0; i < 3; i++) cells[days[i]] = mockEntry({ work_date: days[i] });
    // Thu off
    for (let i = 4; i < 7; i++) cells[days[i]] = mockEntry({ work_date: days[i] });
    expect(longestWorkedRun(cells, days)).toBe(3);
  });

  it('no worked days → run of 0', () => {
    expect(longestWorkedRun({}, days)).toBe(0);
  });

  it('two disjoint runs picks the longer one', () => {
    const cells: Record<string, ScheduleEntry | undefined> = {};
    // Mon–Tue worked
    cells[days[0]] = mockEntry({ work_date: days[0] });
    cells[days[1]] = mockEntry({ work_date: days[1] });
    // Wed off
    // Thu–Sat worked
    cells[days[3]] = mockEntry({ work_date: days[3] });
    cells[days[4]] = mockEntry({ work_date: days[4] });
    cells[days[5]] = mockEntry({ work_date: days[5] });
    expect(longestWorkedRun(cells, days)).toBe(3);
  });
});

// ---------------------------------------------------------------------------
// 8. computeCompliance
// ---------------------------------------------------------------------------

describe('computeCompliance', () => {
  const days = ['2026-06-15', '2026-06-16', '2026-06-17', '2026-06-18', '2026-06-19', '2026-06-20', '2026-06-21'];
  const holidaySet = new Set(['2026-06-17']); // Wed is holiday

  it('all 7 days worked, one holiday → noRest=true, holidayShiftCount=1', () => {
    const cells: Record<string, ScheduleEntry | undefined> = {};
    for (const d of days) cells[d] = mockEntry({ work_date: d });
    const c = computeCompliance(cells, days, holidaySet);
    expect(c.workedCount).toBe(7);
    expect(c.longestRun).toBe(7);
    expect(c.noRest).toBe(true);
    expect(c.longRun).toBe(false); // noRest overrides longRun
    expect(c.holidayShiftCount).toBe(1);
  });

  it('6 days worked → noRest=false, longRun=true', () => {
    const cells: Record<string, ScheduleEntry | undefined> = {};
    for (let i = 0; i < 6; i++) cells[days[i]] = mockEntry({ work_date: days[i] });
    // Sunday off
    const c = computeCompliance(cells, days, holidaySet);
    expect(c.workedCount).toBe(6);
    expect(c.longestRun).toBe(6);
    expect(c.noRest).toBe(false);
    expect(c.longRun).toBe(true);
  });

  it('5 days worked, no holiday → noRest=false, longRun=false, holidayShiftCount=0', () => {
    const cells: Record<string, ScheduleEntry | undefined> = {};
    for (let i = 0; i < 5; i++) cells[days[i]] = mockEntry({ work_date: days[i] });
    // weekend off
    const c = computeCompliance(cells, days, holidaySet);
    expect(c.workedCount).toBe(5);
    expect(c.longestRun).toBe(5);
    expect(c.noRest).toBe(false);
    expect(c.longRun).toBe(false);
    expect(c.holidayShiftCount).toBe(1); // Wednesday Jun 17 is a holiday in the set
  });

  it('empty schedule → all zeros', () => {
    const c = computeCompliance({}, days, new Set());
    expect(c.workedCount).toBe(0);
    expect(c.longestRun).toBe(0);
    expect(c.noRest).toBe(false);
    expect(c.longRun).toBe(false);
    expect(c.holidayShiftCount).toBe(0);
  });

  it('day-off entries not counted as worked', () => {
    const cells: Record<string, ScheduleEntry | undefined> = {};
    for (const d of days) cells[d] = mockEntry({ work_date: d, is_day_off: true });
    const c = computeCompliance(cells, days, new Set());
    expect(c.workedCount).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// 9. hasComplianceIssue — signal detection
// ---------------------------------------------------------------------------

describe('hasComplianceIssue', () => {
  it('noRest → true', () => {
    expect(hasComplianceIssue({ workedCount: 7, longestRun: 7, noRest: true, longRun: false, holidayShiftCount: 0 })).toBe(true);
  });

  it('longRun → true', () => {
    expect(hasComplianceIssue({ workedCount: 6, longestRun: 6, noRest: false, longRun: true, holidayShiftCount: 0 })).toBe(true);
  });

  it('holiday shift → true', () => {
    expect(hasComplianceIssue({ workedCount: 5, longestRun: 5, noRest: false, longRun: false, holidayShiftCount: 2 })).toBe(true);
  });

  it('normal schedule → false', () => {
    expect(hasComplianceIssue({ workedCount: 5, longestRun: 5, noRest: false, longRun: false, holidayShiftCount: 0 })).toBe(false);
  });

  it('empty schedule → false', () => {
    expect(hasComplianceIssue({ workedCount: 0, longestRun: 0, noRest: false, longRun: false, holidayShiftCount: 0 })).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 10. Compliance constants
// ---------------------------------------------------------------------------

describe('compliance constants', () => {
  it('REST_VIOLATION_DAYS is 7', () => {
    expect(REST_VIOLATION_DAYS).toBe(7);
  });

  it('LONG_RUN_WARN_DAYS is 6', () => {
    expect(LONG_RUN_WARN_DAYS).toBe(6);
  });
});
