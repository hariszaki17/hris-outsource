/**
 * Unit tests for the agent /me/kalender calendar helpers (E10 F10.5). Targets the pure data
 * layer — event bucketing, multi-day leave expansion, Jakarta-day anchoring, and status→tone
 * mapping — which is where the calendar's correctness lives (CAL-2/3/7/8). Component render is
 * exercised via the route + MSW elsewhere.
 */
import { ScheduleEntryStatus } from '@swp/api-client/e4';
import { AttendanceStatus } from '@swp/api-client/e5';
import { LeaveStatus } from '@swp/api-client/e6';
import { describe, expect, it } from 'vitest';
import {
  type CalEvent,
  attendanceStatusTone,
  bucketByDate,
  buildEvents,
  expandLeaveDays,
  leaveStatusTone,
} from './me-kalender-screen.tsx';

// Minimal row factories — only the fields buildEvents reads (cast through unknown to satisfy
// the generated row types without restating every column).
const schedule = (o: Record<string, unknown>) => o as never;
const attendance = (o: Record<string, unknown>) => o as never;
const leave = (o: Record<string, unknown>) => o as never;
const overtime = (o: Record<string, unknown>) => o as never;
const holiday = (o: Record<string, unknown>) => o as never;

const JUNE = { from: '2026-06-01', to: '2026-06-30' };

describe('attendanceStatusTone / leaveStatusTone (CAL-3)', () => {
  it('maps attendance statuses to the design-system tones', () => {
    expect(attendanceStatusTone(AttendanceStatus.PRESENT)).toBe('ok');
    expect(attendanceStatusTone(AttendanceStatus.LATE)).toBe('warn');
    expect(attendanceStatusTone(AttendanceStatus.INCOMPLETE)).toBe('onprogress');
    expect(attendanceStatusTone(AttendanceStatus.ABSENT)).toBe('bad');
    expect(attendanceStatusTone(AttendanceStatus.ON_LEAVE)).toBe('info');
  });

  it('maps leave statuses: approved=ok, pending=warn, rejected=bad', () => {
    expect(leaveStatusTone(LeaveStatus.APPROVED)).toBe('ok');
    expect(leaveStatusTone(LeaveStatus.PENDING)).toBe('warn');
    expect(leaveStatusTone(LeaveStatus.REJECTED)).toBe('bad'); // C-6: rejected still shown
  });
});

describe('expandLeaveDays (CAL-8 multi-day span)', () => {
  it('returns every covered day within the window, inclusive', () => {
    expect(expandLeaveDays('2026-06-10', '2026-06-12', JUNE.from, JUNE.to)).toEqual([
      '2026-06-10',
      '2026-06-11',
      '2026-06-12',
    ]);
  });

  it('clamps a span that starts before the visible month (C-3)', () => {
    expect(expandLeaveDays('2026-05-30', '2026-06-02', JUNE.from, JUNE.to)).toEqual([
      '2026-06-01',
      '2026-06-02',
    ]);
  });

  it('is a single day when start === end', () => {
    expect(expandLeaveDays('2026-06-15', '2026-06-15', JUNE.from, JUNE.to)).toEqual(['2026-06-15']);
  });
});

describe('buildEvents (CAL-2 overlay, CAL-7 anchoring)', () => {
  it('overlays all five sources onto the right days', () => {
    const events = buildEvents(
      {
        schedule: [
          schedule({
            id: 'SWP-SCH-1',
            work_date: '2026-06-05',
            start_time: '08:00',
            end_time: '17:00',
            status: ScheduleEntryStatus.SCHEDULED,
            is_day_off: false,
            shift_master_name: 'Pagi',
          }),
        ],
        attendance: [
          attendance({
            id: 'SWP-ATT-1',
            shift_start_at: '2026-06-05T01:00:00Z', // 08:00 WIB
            check_in_at: '2026-06-05T01:05:00Z',
            status: AttendanceStatus.PRESENT,
          }),
        ],
        leave: [
          leave({
            id: 'SWP-LR-1',
            start_date: '2026-06-10',
            end_date: '2026-06-11',
            status: LeaveStatus.APPROVED,
            leave_type_name: 'Cuti Tahunan',
          }),
        ],
        overtime: [
          overtime({
            id: 'SWP-OT-1',
            work_date: '2026-06-05',
            planned_start_time: '17:00',
            planned_end_time: '19:00',
            status: 'APPROVED',
          }),
        ],
        holidays: [holiday({ id: 'SWP-HOL-1', date: '2026-06-01', name: 'Hari Lahir Pancasila' })],
      },
      JUNE.from,
      JUNE.to,
    );

    const byDate = bucketByDate(events);
    // 5 June: shift + attendance + overtime
    expect((byDate.get('2026-06-05') ?? []).map((e) => e.kind).sort()).toEqual([
      'attendance',
      'overtime',
      'shift',
    ]);
    // multi-day leave on both days
    expect(byDate.get('2026-06-10')?.some((e) => e.kind === 'leave')).toBe(true);
    expect(byDate.get('2026-06-11')?.some((e) => e.kind === 'leave')).toBe(true);
    // holiday on the 1st, named, with no deep-link
    const hol = byDate.get('2026-06-01')?.find((e) => e.kind === 'holiday') as CalEvent;
    expect(hol.title).toBe('Hari Lahir Pancasila');
    expect(hol.to).toBeUndefined();
  });

  it('skips day-off schedule entries (no shift marker)', () => {
    const events = buildEvents(
      {
        schedule: [
          schedule({
            id: 'SWP-SCH-2',
            work_date: '2026-06-07',
            is_day_off: true,
            status: ScheduleEntryStatus.SCHEDULED,
          }),
        ],
        attendance: [],
        leave: [],
        overtime: [],
        holidays: [],
      },
      JUNE.from,
      JUNE.to,
    );
    expect(events).toHaveLength(0);
  });

  it('excludes events outside the visible window (CAL-10)', () => {
    const events = buildEvents(
      {
        schedule: [],
        attendance: [],
        leave: [],
        overtime: [
          overtime({
            id: 'SWP-OT-9',
            work_date: '2026-07-01',
            planned_start_time: '17:00',
            planned_end_time: '19:00',
            status: 'PENDING',
          }),
        ],
        holidays: [],
      },
      JUNE.from,
      JUNE.to,
    );
    expect(events).toHaveLength(0);
  });

  it('drops draft/cancelled leave from the grid (CAL-3 noise filter)', () => {
    const events = buildEvents(
      {
        schedule: [],
        attendance: [],
        leave: [
          leave({
            id: 'SWP-LR-9',
            start_date: '2026-06-03',
            end_date: '2026-06-03',
            status: LeaveStatus.CANCELLED,
          }),
        ],
        overtime: [],
        holidays: [],
      },
      JUNE.from,
      JUNE.to,
    );
    expect(events.filter((e) => e.kind === 'leave')).toHaveLength(0);
  });

  it('deep-links shift/attendance to /me and leave/overtime to /me/pengajuan (CAL-5)', () => {
    const events = buildEvents(
      {
        schedule: [
          schedule({
            id: 's',
            work_date: '2026-06-05',
            is_day_off: false,
            status: ScheduleEntryStatus.SCHEDULED,
            start_time: '08:00',
            end_time: '17:00',
          }),
        ],
        attendance: [
          attendance({
            id: 'a',
            shift_start_at: '2026-06-05T01:00:00Z',
            status: AttendanceStatus.PRESENT,
          }),
        ],
        leave: [
          leave({
            id: 'l',
            start_date: '2026-06-05',
            end_date: '2026-06-05',
            status: LeaveStatus.PENDING,
          }),
        ],
        overtime: [overtime({ id: 'o', work_date: '2026-06-05', status: 'PENDING' })],
        holidays: [],
      },
      JUNE.from,
      JUNE.to,
    );
    const to = (k: string) => events.find((e) => e.kind === k)?.to;
    expect(to('shift')).toBe('/me');
    expect(to('attendance')).toBe('/me');
    expect(to('leave')).toBe('/me/pengajuan');
    expect(to('overtime')).toBe('/me/pengajuan');
  });
});
