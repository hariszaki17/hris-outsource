import {
  addDays,
  buildAgentRows,
  buildHolidayMaps,
  computeCompliance,
  currentJakartaIso,
  getMondayOfWeek,
  hasComplianceIssue,
  weekDays,
} from '@/features/e4-scheduling/roster-compliance.ts';
/**
 * E10 · Shift-leader compliance panel (EPICS §8 D3).
 *
 * Surfaces this week's roster-compliance issues to the shift leader on their
 * dashboard — agents with no weekly rest, ≥6 consecutive workdays, or assigned
 * to a public-holiday shift. Derived client-side from the leader's company
 * roster + the /holidays calendar, reusing the same `computeCompliance` logic
 * as the schedule grid (single source of truth). Read-only awareness; the rows
 * deep-link to /schedule for the leader to act.
 */
import { type ScheduleEntry, useListSchedule } from '@swp/api-client/e4';
import { type Holiday, useListHolidays } from '@swp/api-client/e7';
import { EmptyState, StateView } from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { ArrowRight, CalendarCheck, Star, TriangleAlert } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';

interface LeaderCompliancePanelProps {
  companyId: string;
}

export function LeaderCompliancePanel({ companyId }: LeaderCompliancePanelProps) {
  const { t } = useTranslation(['dashboard', 'schedule']);

  // Current week (Mon–Sun) in Asia/Jakarta.
  const monday = useMemo(() => getMondayOfWeek(currentJakartaIso()), []);
  const days = useMemo(() => weekDays(monday), [monday]);
  const sunday = addDays(monday, 6);

  const scheduleQuery = useListSchedule(
    { company_id: companyId, start_date: monday, end_date: sunday },
    { query: { enabled: !!companyId, staleTime: 60 * 1000 } },
  );

  const mondayYear = Number(monday.slice(0, 4));
  const sundayYear = Number(sunday.slice(0, 4));
  const holidaysA = useListHolidays({ year: mondayYear }, { query: { staleTime: 5 * 60 * 1000 } });
  const holidaysB = useListHolidays(
    { year: sundayYear },
    { query: { enabled: sundayYear !== mondayYear, staleTime: 5 * 60 * 1000 } },
  );

  const issues = useMemo(() => {
    const body = scheduleQuery.data?.data as
      | { data?: ScheduleEntry[] }
      | ScheduleEntry[]
      | undefined;
    const entries: ScheduleEntry[] = Array.isArray(body) ? body : (body?.data ?? []);
    const rows = buildAgentRows(entries);

    const extract = (q: typeof holidaysA): Holiday[] => {
      const hb = q.data?.data as { data?: Holiday[] } | Holiday[] | undefined;
      return Array.isArray(hb) ? hb : (hb?.data ?? []);
    };
    const allHolidays = [
      ...extract(holidaysA),
      ...(sundayYear !== mondayYear ? extract(holidaysB) : []),
    ];
    const slIds = new Set(rows.map((r) => r.serviceLineId).filter((x): x is string => !!x));
    const { holidaySet } = buildHolidayMaps(allHolidays, days, slIds);

    return rows
      .map((row) => ({ row, c: computeCompliance(row.cells, days, holidaySet) }))
      .filter((x) => hasComplianceIssue(x.c))
      .sort((a, b) => Number(b.c.noRest) - Number(a.c.noRest)); // violations first
  }, [scheduleQuery.data, holidaysA, holidaysB, days, mondayYear, sundayYear]);

  const loading = scheduleQuery.isLoading;

  return (
    <div className="flex flex-col gap-3 overflow-hidden rounded-xl border border-border bg-surface p-[18px]">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <CalendarCheck aria-hidden className="size-4 text-text-2" />
          <span className="text-[15px] font-bold text-text">{t('sl.complianceTitle')}</span>
        </div>
        <Link
          to="/schedule"
          search={{ company_id: companyId, week: monday }}
          className="flex items-center gap-1 text-[12px] font-medium text-primary hover:underline"
        >
          {t('sl.viewSchedule')}
          <ArrowRight aria-hidden className="size-3.5" />
        </Link>
      </div>

      {loading ? (
        <StateView kind="loading" title={t('schedule:common.loading', { defaultValue: '' })} />
      ) : issues.length === 0 ? (
        <EmptyState
          variant="fresh"
          title={t('sl.complianceAllClear')}
          description={t('sl.complianceAllClearSub')}
        />
      ) : (
        <div className="flex flex-col divide-y divide-border-soft">
          {issues.map(({ row, c }) => (
            <div
              key={`${row.employeeId}::${row.placementId}`}
              className="flex items-start justify-between gap-3 py-[10px]"
            >
              <div className="flex min-w-0 flex-col gap-0.5">
                <span className="truncate text-[13px] font-semibold text-text">
                  {row.employeeName}
                </span>
                {row.serviceLineName && (
                  <span className="truncate text-[11px] text-text-3">{row.serviceLineName}</span>
                )}
              </div>
              <div className="flex flex-wrap items-center justify-end gap-1">
                {c.noRest ? (
                  <span
                    title={t('schedule:compliance.noRestTip')}
                    className="inline-flex items-center gap-0.5 rounded bg-bad-bg px-1.5 py-0.5 text-[10px] font-semibold text-bad-tx"
                  >
                    <TriangleAlert aria-hidden className="size-2.5" />
                    {t('schedule:compliance.noRest')}
                  </span>
                ) : (
                  c.longRun && (
                    <span
                      title={t('schedule:compliance.longRunTip', { count: c.longestRun })}
                      className="inline-flex items-center gap-0.5 rounded bg-warn-bg px-1.5 py-0.5 text-[10px] font-semibold text-warn-tx"
                    >
                      <TriangleAlert aria-hidden className="size-2.5" />
                      {t('schedule:compliance.longRun', { count: c.longestRun })}
                    </span>
                  )
                )}
                {c.holidayShiftCount > 0 && (
                  <span
                    title={t('schedule:compliance.holidayShiftTip')}
                    className="inline-flex items-center gap-0.5 rounded bg-bad-bg px-1.5 py-0.5 text-[10px] font-semibold text-bad-tx"
                  >
                    <Star aria-hidden className="size-2.5 fill-current" />
                    {t('schedule:compliance.holidayShift', { count: c.holidayShiftCount })}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
