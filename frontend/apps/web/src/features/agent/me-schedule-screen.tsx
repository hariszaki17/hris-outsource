import { useCurrentUser } from '@/lib/use-auth.ts';
/**
 * /me/schedule — Agent's own weekly schedule (read-only).
 *
 * Web port of apps/mobile/app/(app)/schedule.tsx. Fetches the current agent's
 * weekly schedule via GET /schedule/by-agent/{employee_id} (useGetScheduleByAgent),
 * renders day-off / shift times / company per day card, with Prev/Next week nav.
 *
 * F4.3 · SV-1 · SV-3 · INV-1
 * i18n namespace: agent
 * Route: /me/schedule
 */
import {
  type GetScheduleByAgent200,
  type ScheduleEntry,
  ScheduleEntryStatus,
  useGetScheduleByAgent,
} from '@swp/api-client/e4';
import { LOCALE_ID } from '@swp/shared/datetime';
import { Button, StateView, StatusBadge } from '@swp/ui';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  addDays,
  currentJakartaIso,
  getMondayOfWeek,
  parsePlainDate,
  weekDays,
} from '../e4-scheduling/roster-compliance.ts';
import { AgentPage } from './agent-page.tsx';

// ---------------------------------------------------------------------------
// Display helpers (Asia/Jakarta-safe, UTC-midnight dates)
// ---------------------------------------------------------------------------

function formatDayAbbr(iso: string): string {
  return new Intl.DateTimeFormat(LOCALE_ID, {
    weekday: 'short',
    timeZone: 'UTC',
  }).format(parsePlainDate(iso));
}

function formatDayNum(iso: string): string {
  return new Intl.DateTimeFormat(LOCALE_ID, {
    day: 'numeric',
    timeZone: 'UTC',
  }).format(parsePlainDate(iso));
}

function formatWeekRange(monday: string): string {
  const sunday = addDays(monday, 6);
  const start = new Intl.DateTimeFormat(LOCALE_ID, {
    day: 'numeric',
    month: 'short',
    timeZone: 'UTC',
  }).format(parsePlainDate(monday));
  const end = new Intl.DateTimeFormat(LOCALE_ID, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(parsePlainDate(sunday));
  return `${start} – ${end}`;
}

// ---------------------------------------------------------------------------
// DayRow — one divided row per day inside the weekly panel
// ---------------------------------------------------------------------------

interface DayRowProps {
  iso: string; // YYYY-MM-DD
  entry?: ScheduleEntry;
}

function DayRow({ iso, entry }: DayRowProps) {
  const { t } = useTranslation('agent');

  let statusLabel: string;
  let tone: 'neutral' | 'info' | 'ok' | 'warn' | 'bad' = 'neutral';
  let shiftTime: string | null = null;

  if (!entry) {
    statusLabel = t('scheduleNoShift');
    tone = 'neutral';
  } else if (entry.is_day_off) {
    statusLabel = t('scheduleDayOff');
    tone = 'neutral';
  } else if (entry.status === ScheduleEntryStatus.CANCELLED_BY_LEAVE) {
    statusLabel = t('scheduleCancelledLeave');
    tone = 'info';
  } else {
    shiftTime = `${entry.start_time ?? '—'}–${entry.end_time ?? '—'}`;
    statusLabel = shiftTime;
    tone = 'ok';
  }

  return (
    <div className="flex items-center gap-4 px-5 py-4">
      {/* Left: weekday abbr + date number */}
      <div className="flex w-12 shrink-0 flex-col gap-0.5">
        <span className="text-[11px] uppercase tracking-wide text-text-3">
          {formatDayAbbr(iso)}
        </span>
        <span className="text-[17px] font-semibold leading-tight text-text">
          {formatDayNum(iso)}
        </span>
      </div>

      {/* Middle: company name (if any) */}
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        {entry?.company_name ? (
          <span className="truncate text-[13px] font-medium text-text">{entry.company_name}</span>
        ) : null}
        {shiftTime ? (
          <span className="text-[12px] tabular-nums text-text-3">{shiftTime}</span>
        ) : null}
      </div>

      {/* Right: status badge */}
      <div className="shrink-0">
        <StatusBadge dot tone={tone}>
          {statusLabel}
        </StatusBadge>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function AgentScheduleScreen() {
  const { t } = useTranslation('agent');
  const user = useCurrentUser();
  const employeeId = user?.employeeId ?? '';

  // ---- Week state (local — no URL sync needed for self-service view) ----
  const [monday, setMonday] = useState<string>(() => getMondayOfWeek(currentJakartaIso()));
  const sunday = addDays(monday, 6);
  const days: string[] = weekDays(monday);

  function prevWeek() {
    setMonday((m) => addDays(m, -7));
  }
  function nextWeek() {
    setMonday((m) => addDays(m, 7));
  }

  // ---- Query ----
  const q = useGetScheduleByAgent(
    employeeId,
    { start_date: monday, end_date: sunday, include_company: true },
    { query: { enabled: !!employeeId } },
  );

  const body = q.data?.data as GetScheduleByAgent200 | undefined;
  const entries: ScheduleEntry[] = (body as { data?: ScheduleEntry[] } | undefined)?.data ?? [];
  const byDate = (iso: string) => entries.find((e) => e.work_date === iso);

  // ---- Week nav — goes in the AgentPage `actions` slot ----
  const weekNav = (
    <>
      <Button variant="ghost" size="sm" onClick={prevWeek} aria-label={t('schedulePrevWeek')}>
        <ChevronLeft size={16} aria-hidden />
      </Button>
      <span className="min-w-[160px] text-center text-[13px] font-medium text-text">
        {formatWeekRange(monday)}
      </span>
      <Button variant="ghost" size="sm" onClick={nextWeek} aria-label={t('scheduleNextWeek')}>
        <ChevronRight size={16} aria-hidden />
      </Button>
    </>
  );

  return (
    <AgentPage title={t('scheduleTitle')} actions={weekNav}>
      {/* Weekly schedule panel — one panel, rows divided by a hairline */}
      <div className="rounded-xl border border-border bg-surface">
        {q.isLoading ? (
          <div className="p-6">
            <StateView kind="loading" title={t('loading')} />
          </div>
        ) : q.isError ? (
          <div className="p-6">
            <StateView kind="error" title={t('errorGeneric')} onRetry={() => void q.refetch()} />
          </div>
        ) : entries.length === 0 && days.every((d) => !byDate(d)) ? (
          <div className="p-6">
            <StateView kind="empty" title={t('scheduleEmpty')} />
          </div>
        ) : (
          <div className="divide-y divide-border-soft">
            {days.map((iso) => (
              <DayRow key={iso} iso={iso} entry={byDate(iso)} />
            ))}
          </div>
        )}
      </div>
    </AgentPage>
  );
}
