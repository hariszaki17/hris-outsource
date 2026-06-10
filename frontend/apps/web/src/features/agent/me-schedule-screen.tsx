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
// DayCard — one row per day of the week
// ---------------------------------------------------------------------------

interface DayCardProps {
  iso: string; // YYYY-MM-DD
  entry?: ScheduleEntry;
}

function DayCard({ iso, entry }: DayCardProps) {
  const { t } = useTranslation('agent');

  let detail: string;
  let tone: 'neutral' | 'info' | 'ok' | 'warn' | 'bad' = 'neutral';

  if (!entry) {
    detail = t('scheduleNoShift');
    tone = 'neutral';
  } else if (entry.is_day_off) {
    detail = t('scheduleDayOff');
    tone = 'neutral';
  } else if (entry.status === ScheduleEntryStatus.CANCELLED_BY_LEAVE) {
    detail = t('scheduleCancelledLeave');
    tone = 'info';
  } else {
    detail = `${entry.start_time ?? '—'}–${entry.end_time ?? '—'}`;
    tone = 'ok';
  }

  return (
    <div className="rounded-xl border border-border bg-surface p-4">
      <div className="flex items-center justify-between gap-3">
        {/* Left: weekday + date */}
        <div className="flex min-w-[48px] flex-col gap-0.5">
          <span className="text-[12px] text-text-3">{formatDayAbbr(iso)}</span>
          <span className="text-[16px] font-semibold text-text">{formatDayNum(iso)}</span>
        </div>

        {/* Right: shift details + company */}
        <div className="flex flex-1 items-center justify-end gap-3">
          {entry?.company_name ? (
            <span className="truncate text-[12px] text-text-3">{entry.company_name}</span>
          ) : null}
          <StatusBadge dot tone={tone}>
            {detail}
          </StatusBadge>
        </div>
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

  return (
    <AgentPage title={t('scheduleTitle')}>
      {/* Week navigation */}
      <div className="flex items-center justify-between gap-2">
        <Button variant="ghost" onClick={prevWeek} aria-label={t('schedulePrevWeek')}>
          <ChevronLeft size={16} aria-hidden />
          <span className="sr-only">{t('schedulePrevWeek')}</span>
        </Button>

        <span className="text-[14px] font-medium text-text">{formatWeekRange(monday)}</span>

        <Button variant="ghost" onClick={nextWeek} aria-label={t('scheduleNextWeek')}>
          <ChevronRight size={16} aria-hidden />
          <span className="sr-only">{t('scheduleNextWeek')}</span>
        </Button>
      </div>

      {/* States */}
      {q.isLoading ? (
        <StateView kind="loading" title={t('loading')} />
      ) : q.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void q.refetch()} />
      ) : entries.length === 0 && days.every((d) => !byDate(d)) ? (
        <StateView kind="empty" title={t('scheduleEmpty')} />
      ) : (
        <div className="flex flex-col gap-3">
          {days.map((iso) => (
            <DayCard key={iso} iso={iso} entry={byDate(iso)} />
          ))}
        </div>
      )}
    </AgentPage>
  );
}
