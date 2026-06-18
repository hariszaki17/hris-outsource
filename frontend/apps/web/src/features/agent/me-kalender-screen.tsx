/**
 * /me/kalender — Agent "Kalender Saya" (web self-service). A READ-ONLY month calendar that
 * overlays the signed-in employee's own shifts (E4), attendance (E5), leave/cuti (E6),
 * overtime/lembur (E7), and the applicable public holidays (E7 HolidayCalendar) on one grid,
 * color-coded by the design-system status palette. Tap an event → its existing detail surface
 * (Kehadiran `/me` for shift/attendance, Pengajuan `/me/pengajuan` for cuti/lembur). It creates
 * NO records — purely a presentation layer (PRD F10.5; docs/epics/E10-reporting/prds/agent-calendar.md).
 *
 * Scope is server-enforced (scope: self — the API resolves the employee from the JWT principal);
 * no employee_id is sent on the lists. All date math goes through the Asia/Jakarta TZ layer
 * (@swp/shared) — never raw `new Date`. Status colors only via `StatusBadge` (DESIGN-SYSTEM §2).
 *
 * Design follows AGENT-WEB-ACCESS.md AW-6 (documented G0 deviation): `/me/*` web screens are not
 * authored in brainstorm.pen first — built from packages/ui + the mobile frame layouts.
 */
import { useCurrentUser } from '@/lib/use-auth.ts';
import { type ScheduleEntry, ScheduleEntryStatus, useGetScheduleByAgent } from '@swp/api-client/e4';
import { type Attendance, AttendanceStatus, useListAttendance } from '@swp/api-client/e5';
import { type LeaveRequest, LeaveStatus, useListLeaveRequests } from '@swp/api-client/e6';
import { type Holiday, type Overtime, useListHolidays, useListOvertime } from '@swp/api-client/e7';
import type { StatusTone } from '@swp/design-tokens';
import {
  addCalendarDays,
  formatInstant,
  formatLocalTime,
  formatMonthYear,
  jakartaDateOf,
  monthGrid,
  monthRange,
  shiftMonth,
  todayJakartaIso,
} from '@swp/shared';
import { Banner, Button, StateView, StatusBadge } from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { Calendar, ChevronLeft, ChevronRight } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { overtimeStatusTone } from '../e7-overtime/overtime-shared.tsx';
import { AgentPage } from './agent-page.tsx';

// ---------------------------------------------------------------------------
// Pure model + helpers (exported for unit tests — me-kalender-screen.test.ts)
// ---------------------------------------------------------------------------

export type CalEventKind = 'shift' | 'attendance' | 'leave' | 'overtime' | 'holiday';

export interface CalEvent {
  kind: CalEventKind;
  id: string;
  /** "YYYY-MM-DD" Asia/Jakarta calendar day this event is anchored to. */
  date: string;
  /** Data-derived title (holiday/leave-type/shift name). Empty → screen falls back to a kind label. */
  title: string;
  /** Optional time range, e.g. "08:00–17:00". */
  time?: string;
  tone: StatusTone;
  /** Deep-link target — the existing detail surface (CAL-5). `holiday` has none. */
  to?: '/me' | '/me/pengajuan';
}

/** Attendance status → status-palette tone (CAL-3; DESIGN-SYSTEM §2). */
export function attendanceStatusTone(status: AttendanceStatus): StatusTone {
  switch (status) {
    case AttendanceStatus.PRESENT:
      return 'ok';
    case AttendanceStatus.LATE:
      return 'warn';
    case AttendanceStatus.INCOMPLETE:
      return 'onprogress';
    case AttendanceStatus.ABSENT:
      return 'bad';
    case AttendanceStatus.ON_LEAVE:
      return 'info';
    default:
      return 'neutral';
  }
}

/** Leave status → tone (CAL-3). */
export function leaveStatusTone(status: LeaveStatus): StatusTone {
  switch (status) {
    case LeaveStatus.APPROVED:
      return 'ok';
    case LeaveStatus.REJECTED:
      return 'bad';
    case LeaveStatus.PENDING:
      return 'warn';
    default:
      return 'neutral';
  }
}

/**
 * Inclusive list of "YYYY-MM-DD" days in `[start, end]` that fall within `[from, to]` (CAL-8 —
 * a multi-day cuti renders on every covered day; clamped to the visible window per CAL-10).
 */
export function expandLeaveDays(start: string, end: string, from: string, to: string): string[] {
  const lo = start > from ? start : from;
  const hi = end < to ? end : to;
  const days: string[] = [];
  let d = lo;
  // String compare works on zero-padded ISO dates; step via the TZ layer (no raw Date).
  while (d <= hi) {
    days.push(d);
    d = addCalendarDays(d, 1);
  }
  return days;
}

interface SourceData {
  schedule: ScheduleEntry[];
  attendance: Attendance[];
  leave: LeaveRequest[];
  overtime: Overtime[];
  holidays: Holiday[];
}

/**
 * Flatten the five sources into per-day calendar events for the visible month window
 * `[from, to]`. Pure — no i18n, no React — so the test suite can assert bucketing/tones
 * directly (CAL-2, CAL-3, CAL-7, CAL-8). Generic titles (shift/attendance/overtime) are left
 * empty and localized at render; data-named events (holiday, leave type, shift master) carry
 * their title here.
 */
export function buildEvents(src: SourceData, from: string, to: string): CalEvent[] {
  const within = (iso: string) => iso >= from && iso <= to;
  const events: CalEvent[] = [];

  for (const h of src.holidays) {
    if (within(h.date)) {
      events.push({ kind: 'holiday', id: h.id, date: h.date, title: h.name, tone: 'bad' });
    }
  }

  for (const e of src.schedule) {
    if (e.is_day_off || !within(e.work_date)) continue;
    const tone: StatusTone =
      e.status === ScheduleEntryStatus.CANCELLED_BY_LEAVE ? 'neutral' : 'info';
    const time =
      e.start_time && e.end_time
        ? `${formatLocalTime(e.start_time)}–${formatLocalTime(e.end_time)}`
        : undefined;
    events.push({
      kind: 'shift',
      id: e.id,
      date: e.work_date,
      title: e.shift_master_name ?? '',
      time,
      tone,
      to: '/me',
    });
  }

  for (const a of src.attendance) {
    // No work_date on Attendance — anchor to the WIB day of the scheduled shift, else clock-in.
    const basis = a.shift_start_at ?? a.check_in_at;
    if (!basis) continue;
    const date = jakartaDateOf(basis);
    if (!within(date)) continue;
    const time = a.check_in_at
      ? `${formatInstant(a.check_in_at, { timeStyle: 'short' })}${
          a.check_out_at ? `–${formatInstant(a.check_out_at, { timeStyle: 'short' })}` : ''
        }`
      : undefined;
    events.push({
      kind: 'attendance',
      id: a.id,
      date,
      title: '',
      time,
      tone: attendanceStatusTone(a.status),
      to: '/me',
    });
  }

  for (const l of src.leave) {
    // Only outcomes worth seeing on the grid (drafts/cancelled are noise) — CAL-3/C-6.
    if (
      l.status !== LeaveStatus.PENDING &&
      l.status !== LeaveStatus.APPROVED &&
      l.status !== LeaveStatus.REJECTED
    ) {
      continue;
    }
    const tone = leaveStatusTone(l.status);
    for (const date of expandLeaveDays(l.start_date, l.end_date, from, to)) {
      events.push({
        kind: 'leave',
        id: `${l.id}:${date}`,
        date,
        title: l.leave_type_name ?? '',
        tone,
        to: '/me/pengajuan',
      });
    }
  }

  for (const o of src.overtime) {
    if (!within(o.work_date)) continue;
    const start = o.planned_start_time ?? o.actual_start_time;
    const end = o.planned_end_time ?? o.actual_end_time;
    const time = start && end ? `${formatLocalTime(start)}–${formatLocalTime(end)}` : undefined;
    events.push({
      kind: 'overtime',
      id: o.id,
      date: o.work_date,
      title: '',
      time,
      tone: overtimeStatusTone(o.status),
      to: '/me/pengajuan',
    });
  }

  return events;
}

/** Group events by their "YYYY-MM-DD" date. */
export function bucketByDate(events: CalEvent[]): Map<string, CalEvent[]> {
  const map = new Map<string, CalEvent[]>();
  for (const ev of events) {
    const arr = map.get(ev.date);
    if (arr) arr.push(ev);
    else map.set(ev.date, [ev]);
  }
  return map;
}

const MAX_CELL_CHIPS = 2;

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function AgentKalenderScreen() {
  const { t } = useTranslation('agent');
  const navigate = useNavigate();
  const user = useCurrentUser();
  const employeeId = user?.employeeId ?? '';

  const today = todayJakartaIso();
  // Anchor is any day inside the visible month; nav clamps to day 1 via shiftMonth.
  const [anchor, setAnchor] = useState<string>(today);
  const [selected, setSelected] = useState<string>(today);

  const range = useMemo(() => monthRange(anchor), [anchor]);
  const weeks = useMemo(() => monthGrid(anchor), [anchor]);
  const year = Number(range.from.slice(0, 4));

  // ---- Data (scope: self, month-windowed — CAL-1, CAL-10) ----
  const scheduleQ = useGetScheduleByAgent(
    employeeId,
    { start_date: range.from, end_date: range.to, include_company: true },
    { query: { enabled: !!employeeId } },
  );
  const attendanceQ = useListAttendance({ date_from: range.from, date_to: range.to, limit: 200 });
  // Leave is bucketed client-side so multi-day spans starting in a prior month still render.
  const leaveQ = useListLeaveRequests({ limit: 100 });
  const overtimeQ = useListOvertime({
    work_date__gte: range.from,
    work_date__lte: range.to,
    limit: 200,
  });
  const holidaysQ = useListHolidays({ year }, { query: { enabled: true } });

  const schedule = (scheduleQ.data?.data as { data?: ScheduleEntry[] } | undefined)?.data ?? [];
  const attendance = (attendanceQ.data?.data as { data?: Attendance[] } | undefined)?.data ?? [];
  const leave = (leaveQ.data?.data as { data?: LeaveRequest[] } | undefined)?.data ?? [];
  const overtime = (overtimeQ.data?.data as { data?: Overtime[] } | undefined)?.data ?? [];
  const holidays = (holidaysQ.data?.data as { data?: Holiday[] } | undefined)?.data ?? [];

  const events = useMemo(
    () => buildEvents({ schedule, attendance, leave, overtime, holidays }, range.from, range.to),
    [schedule, attendance, leave, overtime, holidays, range.from, range.to],
  );
  const byDate = useMemo(() => bucketByDate(events), [events]);
  const holidayDates = useMemo(
    () => new Set(events.filter((e) => e.kind === 'holiday').map((e) => e.date)),
    [events],
  );

  // ---- States (B2 no dead-flow; CAL-12 partial resilience) ----
  const allLoading =
    scheduleQ.isLoading &&
    attendanceQ.isLoading &&
    leaveQ.isLoading &&
    overtimeQ.isLoading &&
    holidaysQ.isLoading;
  const allError =
    scheduleQ.isError &&
    attendanceQ.isError &&
    leaveQ.isError &&
    overtimeQ.isError &&
    holidaysQ.isError;

  const failedSources = [
    scheduleQ.isError && t('calendar.kind.shift'),
    attendanceQ.isError && t('calendar.kind.attendance'),
    leaveQ.isError && t('calendar.kind.leave'),
    overtimeQ.isError && t('calendar.kind.overtime'),
    holidaysQ.isError && t('calendar.kind.holiday'),
  ].filter((s): s is string => Boolean(s));

  const refetchAll = () => {
    void scheduleQ.refetch();
    void attendanceQ.refetch();
    void leaveQ.refetch();
    void overtimeQ.refetch();
    void holidaysQ.refetch();
  };

  const kindLabel = (ev: CalEvent) => ev.title || t(`calendar.kind.${ev.kind}`);
  const weekdayKeys = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const;
  const selectedEvents = byDate.get(selected) ?? [];

  return (
    <AgentPage title={t('calendar.title')} subtitle={t('calendar.subtitle')}>
      {/* Month toolbar */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            aria-label={t('calendar.navPrev')}
            onClick={() => setAnchor(shiftMonth(anchor, -1))}
          >
            <ChevronLeft className="size-4" />
          </Button>
          <span className="min-w-[140px] text-center font-bold text-[15px] text-text">
            {formatMonthYear(anchor)}
          </span>
          <Button
            variant="ghost"
            size="sm"
            aria-label={t('calendar.navNext')}
            onClick={() => setAnchor(shiftMonth(anchor, 1))}
          >
            <ChevronRight className="size-4" />
          </Button>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            setAnchor(today);
            setSelected(today);
          }}
        >
          <Calendar className="mr-1.5 size-4" />
          {t('calendar.today')}
        </Button>
      </div>

      {/* Per-source failure notice (partial render — CAL-12) */}
      {!allError && failedSources.length > 0 ? (
        <Banner
          tone="warn"
          title={t('calendar.partialErrorTitle')}
          description={t('calendar.partialErrorDesc', { sources: failedSources.join(', ') })}
        />
      ) : null}

      {allLoading ? (
        <StateView kind="loading" title={t('calendar.loading')} />
      ) : allError ? (
        <StateView kind="error" title={t('calendar.errorGeneric')} onRetry={refetchAll} />
      ) : (
        <div className="flex flex-col gap-5 lg:flex-row lg:items-start">
          {/* Month grid */}
          <div className="min-w-0 flex-1 overflow-hidden rounded-xl border border-border bg-surface">
            <div className="grid grid-cols-7 border-border border-b">
              {weekdayKeys.map((d) => (
                <div
                  key={d}
                  className="px-2 py-2 text-center font-semibold text-[12px] text-text-muted"
                >
                  {t(`calendar.weekday.${d}`)}
                </div>
              ))}
            </div>
            {weeks.map((week) => (
              <div key={week.map((c) => c ?? 'x').join('')} className="grid grid-cols-7">
                {week.map((iso, i) => {
                  if (!iso) {
                    // Positional blank — keyed by its weekday column (stable within the week,
                    // the parent row key makes it globally unique); never reorders.
                    return (
                      <div
                        key={`blank-${weekdayKeys[i]}`}
                        className="min-h-[84px] border-border border-t"
                      />
                    );
                  }
                  const dayEvents = byDate.get(iso) ?? [];
                  const isToday = iso === today;
                  const isSelected = iso === selected;
                  const isHoliday = holidayDates.has(iso);
                  const dayNum = Number(iso.slice(8, 10));
                  return (
                    <button
                      type="button"
                      key={iso}
                      onClick={() => setSelected(iso)}
                      className={[
                        'flex min-h-[84px] flex-col gap-1 border-border border-t p-1.5 text-left transition-colors',
                        isHoliday ? 'bg-bad-bg/40' : 'hover:bg-surface-muted',
                        isSelected ? 'ring-2 ring-brand ring-inset' : '',
                      ].join(' ')}
                    >
                      <span
                        className={[
                          'inline-flex size-6 items-center justify-center rounded-full text-[12px]',
                          isToday ? 'bg-brand font-bold text-white' : 'text-text',
                          isHoliday && !isToday ? 'font-semibold text-bad-tx' : '',
                        ].join(' ')}
                      >
                        {dayNum}
                      </span>
                      <div className="flex flex-col gap-0.5">
                        {dayEvents.slice(0, MAX_CELL_CHIPS).map((ev) => (
                          <StatusBadge key={ev.id} dot tone={ev.tone} className="max-w-full">
                            <span className="truncate">{kindLabel(ev)}</span>
                          </StatusBadge>
                        ))}
                        {dayEvents.length > MAX_CELL_CHIPS ? (
                          <span className="pl-1 text-[11px] text-text-muted">
                            {t('calendar.moreCount', { count: dayEvents.length - MAX_CELL_CHIPS })}
                          </span>
                        ) : null}
                      </div>
                    </button>
                  );
                })}
              </div>
            ))}
          </div>

          {/* Selected-day agenda + legend */}
          <div className="flex w-full shrink-0 flex-col gap-4 lg:w-[320px]">
            <section className="rounded-xl border border-border bg-surface p-4">
              <h2 className="mb-3 font-bold text-[15px] text-text">{t('calendar.agendaTitle')}</h2>
              <p className="mb-3 text-[13px] text-text-muted">
                {new Intl.DateTimeFormat('id-ID', {
                  weekday: 'long',
                  day: 'numeric',
                  month: 'long',
                  timeZone: 'UTC',
                }).format(
                  new Date(
                    Date.UTC(
                      Number(selected.slice(0, 4)),
                      Number(selected.slice(5, 7)) - 1,
                      Number(selected.slice(8, 10)),
                    ),
                  ),
                )}
              </p>
              {selectedEvents.length === 0 ? (
                <StateView kind="empty" title={t('calendar.dayEmpty')} />
              ) : (
                <ul className="flex flex-col gap-2">
                  {selectedEvents.map((ev) => {
                    const row = (
                      <div className="flex items-start justify-between gap-2 rounded-lg border border-border px-3 py-2">
                        <div className="flex min-w-0 flex-col gap-1">
                          <StatusBadge dot tone={ev.tone}>
                            {kindLabel(ev)}
                          </StatusBadge>
                          {ev.time ? (
                            <span className="text-[12px] text-text-muted">{ev.time}</span>
                          ) : null}
                        </div>
                      </div>
                    );
                    const dest = ev.to;
                    return (
                      <li key={ev.id}>
                        {dest ? (
                          <button
                            type="button"
                            className="w-full text-left"
                            onClick={() => navigate({ to: dest })}
                          >
                            {row}
                          </button>
                        ) : (
                          row
                        )}
                      </li>
                    );
                  })}
                </ul>
              )}
            </section>

            {/* Legend (CAL-11) */}
            <section className="rounded-xl border border-border bg-surface p-4">
              <h2 className="mb-3 font-bold text-[13px] text-text">{t('calendar.legendTitle')}</h2>
              <ul className="flex flex-col gap-2">
                {(['shift', 'attendance', 'leave', 'overtime', 'holiday'] as const).map((kind) => (
                  <li key={kind} className="flex items-center gap-2 text-[12px] text-text-muted">
                    <StatusBadge dot tone={LEGEND_TONE[kind]}>
                      {t(`calendar.kind.${kind}`)}
                    </StatusBadge>
                  </li>
                ))}
              </ul>
            </section>
          </div>
        </div>
      )}
    </AgentPage>
  );
}

/** Representative tone per kind for the legend (leave/attendance vary by status at render). */
const LEGEND_TONE: Record<CalEventKind, StatusTone> = {
  shift: 'info',
  attendance: 'ok',
  leave: 'warn',
  overtime: 'onprogress',
  holiday: 'bad',
};
