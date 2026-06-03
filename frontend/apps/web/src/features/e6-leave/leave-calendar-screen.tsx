/**
 * E6 · Kalender Cuti — monthly leave calendar.
 *
 * .pen frames implemented:
 *   s5niW  E6 · Kalender Cuti (HR)         — cross-company, all clients visible
 *   YvYcr  E6 SL · Kalender Cuti Tim       — SL own-company, locked company filter
 *
 * Route:  /leave/calendar
 * Feature: F6.5  (LC-1, LV-3, LV-4)
 *
 * validateSearch fields: company_id, leave_type_id, period, month, show_pending
 */

import { ClientCompanyPicker } from '@/features/e2-identity/pickers/client-company-picker.tsx';
import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type GetLeaveCalendarParams,
  type LeaveCalendarClash,
  type LeaveCalendarEntry,
  LeaveStatus,
  useGetLeaveCalendar,
} from '@swp/api-client/e6';
import type { StatusTone } from '@swp/design-tokens';
import { LOCALE_ID, TZ } from '@swp/shared';
import {
  Avatar,
  Button,
  EmptyState,
  FilterSelect,
  Skeleton,
  StateView,
  StatusBadge,
  Toggle,
} from '@swp/ui';
import { CalendarOff, ChevronLeft, ChevronRight, Download, Lock } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type LeaveCalendarSearch = {
  company_id?: string;
  leave_type_id?: string;
  period?: number;
  month?: number;
  show_pending?: boolean;
};

// ---------------------------------------------------------------------------
// Local date helpers — plain Date, no new npm deps, Asia/Jakarta aware
// ---------------------------------------------------------------------------

function daysInMonth(year: number, month: number): number {
  return new Date(Date.UTC(year, month, 0)).getUTCDate();
}

/** weekday Mon=0 … Sun=6 of the 1st of the given month */
function firstWeekdayMonFirst(year: number, month: number): number {
  // getUTCDay: 0=Sun,1=Mon…6=Sat → convert to Mon=0
  const sunFirst = new Date(Date.UTC(year, month - 1, 1)).getUTCDay();
  return (sunFirst + 6) % 7;
}

function todayInJakarta(): { year: number; month: number; day: number } {
  const fmt = new Intl.DateTimeFormat(LOCALE_ID, {
    timeZone: TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  });
  const parts = fmt.formatToParts(new Date());
  const get = (type: string) => Number(parts.find((p) => p.type === type)?.value ?? '0');
  return { year: get('year'), month: get('month'), day: get('day') };
}

function isoDate(year: number, month: number, day: number): string {
  return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
}

function monthLabel(year: number, month: number): string {
  return new Intl.DateTimeFormat(LOCALE_ID, {
    month: 'long',
    year: 'numeric',
    timeZone: TZ,
  }).format(new Date(Date.UTC(year, month - 1, 1)));
}

// ---------------------------------------------------------------------------
// Leave-type code → StatusTone (design: Tahunan=info/blue, Sakit=bad, Lainnya=ok)
// pending entries rendered as neutral (dimmed per design pending legend)
// ---------------------------------------------------------------------------

function leaveTypeTone(leaveTypeCode: string | undefined, status: LeaveStatus): StatusTone {
  const isPending = status === LeaveStatus.PENDING_L1 || status === LeaveStatus.PENDING_HR;
  if (isPending) return 'neutral';
  switch ((leaveTypeCode ?? '').toLowerCase()) {
    case 'tahunan':
    case 'annual':
      return 'info';
    case 'sakit':
    case 'sick':
      return 'bad';
    default:
      return 'ok';
  }
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

interface DayCellProps {
  day: number;
  isToday: boolean;
  isOutsideMonth: boolean;
  isClash: boolean;
  entries: LeaveCalendarEntry[];
  showPending: boolean;
}

function DayCell({ day, isToday, isOutsideMonth, isClash, entries, showPending }: DayCellProps) {
  const { t } = useTranslation('leaveCalendar');

  const visibleEntries = entries.filter((e) => {
    if (isOutsideMonth) return false;
    if (!showPending) return e.status === LeaveStatus.APPROVED;
    return (
      e.status === LeaveStatus.APPROVED ||
      e.status === LeaveStatus.PENDING_L1 ||
      e.status === LeaveStatus.PENDING_HR
    );
  });

  const isPending = (e: LeaveCalendarEntry) =>
    e.status === LeaveStatus.PENDING_L1 || e.status === LeaveStatus.PENDING_HR;

  return (
    <div
      className={[
        'flex flex-col gap-1 p-[7px] min-h-[98px] w-full border-r border-border-soft last:border-r-0',
        isOutsideMonth ? 'bg-surface-2' : isClash ? 'bg-warn-bg' : 'bg-surface',
        isToday ? 'ring-1 ring-inset ring-primary' : '',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <span
        className={[
          'text-xs font-medium leading-none',
          isOutsideMonth ? 'text-text-3' : 'text-text-2',
          isToday ? 'text-primary font-bold' : '',
        ]
          .filter(Boolean)
          .join(' ')}
      >
        {isOutsideMonth ? '' : day}
      </span>
      {visibleEntries.slice(0, 3).map((e) => (
        <div
          key={e.leave_request_id}
          className={[
            'flex items-center gap-1 rounded px-1 py-0.5',
            isPending(e) ? 'opacity-60 border border-dashed border-accent-blue' : '',
          ]
            .filter(Boolean)
            .join(' ')}
          title={e.employee_name ?? e.employee_id}
        >
          <Avatar
            initials={(e.employee_name ?? e.employee_id).slice(0, 2).toUpperCase()}
            size={18}
          />
          <span className="text-[11px] leading-tight text-text truncate max-w-[64px]">
            {e.employee_name ?? e.employee_id}
          </span>
          <StatusBadge tone={leaveTypeTone(e.leave_type_code, e.status)}>
            {e.leave_type_code ?? t('leave')}
          </StatusBadge>
        </div>
      ))}
      {visibleEntries.length > 3 && (
        <span className="text-[10px] text-text-3">
          {t('moreEntries', { count: visibleEntries.length - 3 })}
        </span>
      )}
      {isClash && !isOutsideMonth && (
        <span
          className="mt-auto text-[10px] font-medium text-warn-tx border border-warn-bd rounded px-1"
          title={t('clashTitle')}
        >
          {t('clash')}
        </span>
      )}
    </div>
  );
}

// Day-of-week header labels matching design frame: SEN SEL RAB KAM JUM SAB MIN (Mon–Sun)
const DAY_HEADER_KEYS = ['sen', 'sel', 'rab', 'kam', 'jum', 'sab', 'min'] as const;

interface CalendarGridProps {
  year: number;
  month: number;
  entries: LeaveCalendarEntry[];
  clashes: LeaveCalendarClash[];
  showPending: boolean;
}

function CalendarGrid({ year, month, entries, clashes, showPending }: CalendarGridProps) {
  const { t } = useTranslation('leaveCalendar');
  const today = todayInJakarta();
  const total = daysInMonth(year, month);
  const offset = firstWeekdayMonFirst(year, month);

  // Build entry index: iso-date → entries[]
  const entryMap = new Map<string, LeaveCalendarEntry[]>();
  for (const e of entries) {
    const start = new Date(`${e.start_date}T00:00:00Z`);
    const end = new Date(`${e.end_date}T00:00:00Z`);
    const cur = new Date(start);
    while (cur <= end) {
      const key = cur.toISOString().slice(0, 10);
      const arr = entryMap.get(key) ?? [];
      arr.push(e);
      entryMap.set(key, arr);
      cur.setUTCDate(cur.getUTCDate() + 1);
    }
  }

  const clashDates = new Set(clashes.map((c) => c.date));

  // Build grid: leading empty cells, days, trailing empty cells to fill last week
  const cells: Array<{ day: number; isOutsideMonth: boolean }> = [];
  for (let i = 0; i < offset; i++) cells.push({ day: 0, isOutsideMonth: true });
  for (let d = 1; d <= total; d++) cells.push({ day: d, isOutsideMonth: false });
  while (cells.length % 7 !== 0) cells.push({ day: 0, isOutsideMonth: true });

  const weeks: Array<typeof cells> = [];
  for (let i = 0; i < cells.length; i += 7) weeks.push(cells.slice(i, i + 7));

  return (
    <div className="rounded-xl border border-border overflow-hidden bg-surface w-full">
      {/* Day-of-week header */}
      <div className="flex w-full bg-surface-2 border-b border-border">
        {DAY_HEADER_KEYS.map((h) => (
          <div key={h} className="flex-1 px-[10px] py-2 text-xs font-medium text-text-2 text-left">
            {t(`day.${h}`)}
          </div>
        ))}
      </div>
      {/* Weeks */}
      {weeks.map((week) => (
        <div
          key={week.map((c) => `${c.isOutsideMonth ? 'o' : ''}${c.day}`).join('-')}
          className="flex w-full border-b border-border-soft last:border-b-0"
        >
          {week.map((cell, ci) => {
            const cellKey = cell.isOutsideMonth
              ? `outside-${cell.day}-${ci}`
              : isoDate(year, month, cell.day);
            const cellEntries = cell.isOutsideMonth ? [] : (entryMap.get(cellKey) ?? []);
            const isClash = !cell.isOutsideMonth && clashDates.has(cellKey);
            const isToday =
              !cell.isOutsideMonth &&
              today.year === year &&
              today.month === month &&
              today.day === cell.day;
            return (
              <DayCell
                key={cellKey}
                day={cell.day}
                isToday={isToday}
                isOutsideMonth={cell.isOutsideMonth}
                isClash={isClash}
                entries={cellEntries}
                showPending={showPending}
              />
            );
          })}
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// SkeletonGrid
// ---------------------------------------------------------------------------

function SkeletonGrid() {
  return (
    <div className="rounded-xl border border-border overflow-hidden w-full">
      <div className="flex w-full bg-surface-2 border-b border-border">
        {DAY_HEADER_KEYS.map((h) => (
          <div key={h} className="flex-1 px-[10px] py-2">
            <Skeleton className="h-3 w-8 rounded" />
          </div>
        ))}
      </div>
      {['w0', 'w1', 'w2', 'w3', 'w4'].map((wk) => (
        <div key={wk} className="flex w-full border-b border-border-soft last:border-b-0">
          {['d0', 'd1', 'd2', 'd3', 'd4', 'd5', 'd6'].map((dk) => (
            <div
              key={`${wk}-${dk}`}
              className="flex-1 p-[7px] min-h-[98px] flex flex-col gap-2 bg-surface border-r border-border-soft last:border-r-0"
            >
              <Skeleton className="h-3 w-4 rounded" />
              <Skeleton className="h-5 w-full rounded" />
              <Skeleton className="h-5 w-4/5 rounded" />
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Legend helpers (no raw hex — Tailwind token class names only)
// ---------------------------------------------------------------------------

function LegendDot({ colorClass, label }: { colorClass: string; label: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <span aria-hidden className={`h-2 w-2 rounded-full ${colorClass}`} />
      <span className="text-xs text-text-2">{label}</span>
    </div>
  );
}

function LegendBox({
  bgClass,
  borderClass,
  opacityClass = '',
  label,
}: {
  bgClass: string;
  borderClass: string;
  opacityClass?: string;
  label: string;
}) {
  return (
    <div className="flex items-center gap-1.5">
      <span
        aria-hidden
        className={`h-2.5 w-2.5 rounded-sm border ${bgClass} ${borderClass} ${opacityClass}`}
      />
      <span className="text-xs text-text-2">{label}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main screen
// ---------------------------------------------------------------------------

export function LeaveCalendarScreen() {
  const { t } = useTranslation('leaveCalendar');
  const user = useCurrentUser();

  const isShiftLeader = user?.role === 'shift_leader';
  const isHrAdmin = user?.role === 'hr_admin' || user?.role === 'super_admin';

  const today = todayInJakarta();
  const [year, setYear] = useState(today.year);
  const [month, setMonth] = useState(today.month);
  const [companyId, setCompanyId] = useState<string | null>(null);
  const [showPending, setShowPending] = useState(false);

  // SL is locked to their own company — pass no company_id, server enforces own-company
  const params: GetLeaveCalendarParams = {
    period: year,
    month,
    show_pending: showPending,
    ...(isHrAdmin && companyId ? { company_id: companyId } : {}),
  };

  const query = useGetLeaveCalendar(params);
  const calendarData = query.data?.data;

  function prevMonth() {
    if (month === 1) {
      setYear((y) => y - 1);
      setMonth(12);
    } else {
      setMonth((m) => m - 1);
    }
  }

  function nextMonth() {
    if (month === 12) {
      setYear((y) => y + 1);
      setMonth(1);
    } else {
      setMonth((m) => m + 1);
    }
  }

  // No permission — neither role can see this
  if (!isShiftLeader && !isHrAdmin) {
    return (
      <div className="flex flex-1 items-center justify-center p-6">
        <EmptyState
          icon={Lock}
          title={t('noPermission.title')}
          description={t('noPermission.description')}
        />
      </div>
    );
  }

  const isLoading = query.isLoading;
  const isError = query.isError;
  const errorInfo = isError ? classifyError(query.error) : null;

  const entries: LeaveCalendarEntry[] =
    calendarData && 'entries' in calendarData ? calendarData.entries : [];
  const clashes: LeaveCalendarClash[] =
    calendarData && 'clashes' in calendarData ? (calendarData.clashes ?? []) : [];

  const isEmpty = !isLoading && !isError && entries.length === 0;

  const titleKey = isShiftLeader ? 'titleSL' : 'title';
  const subtitleKey = isShiftLeader ? 'subtitleSL' : 'subtitle';

  return (
    <div className="flex flex-col flex-1 gap-4 p-6 bg-app-bg w-full">
      {/* Header band */}
      <div className="flex items-center justify-between w-full">
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-bold text-text">{t(titleKey)}</h1>
          <p className="text-[13px] text-text-3">{t(subtitleKey)}</p>
        </div>
        <div className="flex items-center gap-[10px]">
          {/* Month navigation */}
          <div className="flex items-center gap-3 rounded-lg border border-border bg-surface px-3 py-2">
            <button
              type="button"
              aria-label={t('prevMonth')}
              onClick={prevMonth}
              className="text-text-2 hover:text-text transition-colors"
            >
              <ChevronLeft aria-hidden className="h-4 w-4" />
            </button>
            <span className="text-sm font-semibold text-text min-w-[130px] text-center">
              {monthLabel(year, month)}
            </span>
            <button
              type="button"
              aria-label={t('nextMonth')}
              onClick={nextMonth}
              className="text-text-2 hover:text-text transition-colors"
            >
              <ChevronRight aria-hidden className="h-4 w-4" />
            </button>
          </div>
          {/* Company filter — HR only; SL sees locked label from design */}
          {isHrAdmin ? (
            <ClientCompanyPicker
              value={companyId}
              onChange={setCompanyId}
              placeholder={t('allCompanies')}
            />
          ) : (
            <FilterSelect disabled>
              <option value="">{user?.companyName ?? t('ownCompany')}</option>
            </FilterSelect>
          )}
          {/* Export */}
          <Button type="button" variant="secondary" size="sm">
            <Download aria-hidden className="h-4 w-4 mr-1.5" />
            {t('export')}
          </Button>
        </div>
      </div>

      {/* Legend — matches design frame legend row */}
      <div className="flex items-center gap-4 w-full flex-wrap">
        <LegendDot colorClass="bg-accent-blue" label={t('legend.annual')} />
        <LegendDot colorClass="bg-bad-tx" label={t('legend.sick')} />
        <LegendDot colorClass="bg-accent-green" label={t('legend.other')} />
        <LegendBox bgClass="bg-warn-bg" borderClass="border-warn-bd" label={t('legend.clash')} />
        <LegendBox
          bgClass="bg-accent-blue"
          borderClass="border-accent-blue"
          opacityClass="opacity-40"
          label={t('legend.pending')}
        />
        {/* Pending toggle — rightmost, justifyContent:end per design */}
        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs font-semibold text-text-2">{t('showPending')}</span>
          <Toggle
            checked={showPending}
            onCheckedChange={setShowPending}
            aria-label={t('showPendingAria')}
          />
        </div>
      </div>

      {/* Calendar body — state variants */}
      {isLoading && <SkeletonGrid />}

      {isError && errorInfo && (
        <StateView
          kind={errorInfo.kind === 'forbidden' ? 'no-permission' : 'error'}
          title={t(errorInfo.message, { defaultValue: errorInfo.message })}
          onRetry={() => void query.refetch()}
        />
      )}

      {isEmpty && !isError && (
        <EmptyState
          icon={CalendarOff}
          title={t('empty.title')}
          description={t('empty.description')}
        />
      )}

      {!isLoading && !isError && !isEmpty && (
        <CalendarGrid
          year={year}
          month={month}
          entries={entries}
          clashes={clashes}
          showPending={showPending}
        />
      )}
    </div>
  );
}
