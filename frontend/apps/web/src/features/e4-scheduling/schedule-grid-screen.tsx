/**
 * E4 · Jadwal Mingguan (Shift Leader) — weekly schedule grid per company.
 *
 * .pen frame: Rubba (1440×1024) — "E4 · Jadwal Mingguan (Shift Leader)"
 *   - Sidebar (iCqTB ref, Jadwal Shift nav item highlighted)
 *   - Topbar with company name + "Jadwal Shift" breadcrumb
 *   - Auto-publish banner (ok-bg/ok-tx — INV-4)
 *   - Legend: Pagi / Siang / Malam / Building Day / Cleaning / OFF / Cuti
 *   - WeekNav (prev/next + date range display)
 *   - "Terapkan ke rentang" (BtnSecondary)
 *   - Grid: header row (AGEN col + 7 day columns) + one agent row per placement
 *     - Cell: shift chip (dot + name + HH:MM) or "Libur" or empty "+"
 *     - Cell click → ShiftPickerPopover
 *
 * State variants: default · loading (skeleton) · empty (no placements) ·
 *   error/retry · no-permission · saving · conflict toasts · auto-publish toast
 *
 * F4.2 · F4.3 · F4.4 · INV-1..4 · SA-1..8 · BR-1..6
 * i18n namespace: schedule
 * Route: /schedule  validateSearch: { company_id?: string; week?: string }
 */

import { ClientCompanyPicker } from '@/features/e2-identity/pickers/client-company-picker.tsx';
import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { useCompanyOptions } from '@/lib/use-company-options.ts';
import {
  type ListScheduleParams,
  type ScheduleEntry,
  ScheduleEntryStatus,
  getListScheduleQueryKey,
  useListSchedule,
} from '@swp/api-client/e4';
import { type Holiday, useListHolidays } from '@swp/api-client/e7';
import { LOCALE_ID } from '@swp/shared';
import { Button, EmptyState, SkeletonTableRow, StateView } from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { ChevronLeft, ChevronRight, Copy, Radio, Star, TriangleAlert } from 'lucide-react';
import * as React from 'react';
import { useTranslation } from 'react-i18next';
import {
  type AgentRow,
  type RowCompliance,
  addDays,
  buildAgentRows,
  buildHolidayMaps,
  computeCompliance,
  currentJakartaIso,
  getMondayOfWeek,
  parsePlainDate,
  weekDays,
} from './roster-compliance.ts';
import type { CellTarget } from './schedule-overlays.tsx';
import { BulkApplyModal, ShiftPickerPopover } from './schedule-overlays.tsx';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const DAY_ABBR_ID = ['SEN', 'SEL', 'RAB', 'KAM', 'JUM', 'SAB', 'MIN'];
const AGENT_COL_W = 230; // px — matches .pen width:230 on AgenH/AgentCol

// ---------------------------------------------------------------------------
// Route search params
// ---------------------------------------------------------------------------

export type ScheduleGridSearch = {
  company_id?: string;
  week?: string; // ISO date of the Monday of the target week, e.g. "2026-06-08"
};

// ---------------------------------------------------------------------------
// Display formatters (date math + compliance live in ./roster-compliance.ts)
// ---------------------------------------------------------------------------

function formatDayMonthId(iso: string): string {
  return new Intl.DateTimeFormat(LOCALE_ID, {
    day: 'numeric',
    month: 'short',
    timeZone: 'UTC',
  }).format(parsePlainDate(iso));
}

function formatWeekRangeId(monday: string, sunday: string): string {
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
// Shift-dot color by shift name heuristic (matches .pen legend)
// ---------------------------------------------------------------------------

function shiftDotClass(entry: ScheduleEntry): string {
  const name = (entry.shift_master_name ?? '').toLowerCase();
  if (name.includes('malam')) return 'bg-text-3';
  if (name.includes('siang')) return 'bg-accent-blue';
  return 'bg-accent-gold'; // Pagi / Cleaning / Building / default
}

// ---------------------------------------------------------------------------
// Main screen
// ---------------------------------------------------------------------------

export function ScheduleGridScreen() {
  const { t } = useTranslation('schedule');
  const navigate = useNavigate({ from: '/schedule' });
  const qc = useQueryClient();

  const search = useSearch({ strict: false }) as ScheduleGridSearch;

  // ---- Week state ----
  const todayIso = currentJakartaIso();
  const initialMonday = search.week ? getMondayOfWeek(search.week) : getMondayOfWeek(todayIso);
  const [monday, setMonday] = React.useState<string>(initialMonday);
  const sunday = addDays(monday, 6);
  const days: string[] = weekDays(monday);

  // ---- Company state ----
  // A shift leader is scoped to ONE company (INV-3 / SV-1): pin the grid to their
  // own company and hide the picker. The server auto-scopes too (defense-in-depth);
  // this just removes the friction + the cross-company company list from the UI.
  const user = useCurrentUser();
  const isShiftLeader = user?.role === 'shift_leader';
  const slCompanyId = isShiftLeader ? (user?.companyId ?? null) : null;
  const [companyId, setCompanyId] = React.useState<string | null>(
    slCompanyId ?? search.company_id ?? null,
  );

  // Resolve the SELECTED client company's display name for the page title — independent of
  // whether any placements/rows exist (a company can be selected with zero placed agents).
  // Shift leaders are pinned to their own company (companyName on the session); HR/super-admin
  // look the label up from the shared company options by the selected id.
  const { options: companyOptions } = useCompanyOptions({ enabled: !isShiftLeader });
  const selectedCompanyName = isShiftLeader
    ? (user?.companyName ?? null)
    : (companyOptions.find((o) => o.value === companyId)?.label ?? null);

  // ---- Popover state ----
  const [popoverTarget, setPopoverTarget] = React.useState<CellTarget | null>(null);
  const [popoverAnchor] = React.useState<React.RefObject<HTMLElement | null>>(
    React.createRef<HTMLElement>(),
  );
  const popoverContainerRef = React.useRef<HTMLDivElement>(null);

  // ---- Bulk modal ----
  const [bulkOpen, setBulkOpen] = React.useState(false);

  // ---- Schedule query ----
  const queryParams: ListScheduleParams | null = companyId
    ? {
        company_id: companyId,
        start_date: monday,
        end_date: sunday,
      }
    : null;

  const scheduleQuery = useListSchedule(queryParams!, {
    query: { enabled: !!queryParams, staleTime: 0 },
  });

  const scheduleQueryKey = queryParams
    ? getListScheduleQueryKey(queryParams)
    : ['schedule-disabled'];

  const entries: ScheduleEntry[] =
    (scheduleQuery.data?.data as { data?: ScheduleEntry[] } | undefined)?.data ?? [];

  // Fallback: some shapes return flat array
  const flatEntries: ScheduleEntry[] = Array.isArray(scheduleQuery.data?.data)
    ? (scheduleQuery.data?.data as unknown as ScheduleEntry[])
    : entries;

  const agentRows = React.useMemo(() => buildAgentRows(flatEntries), [flatEntries]);

  // ---- Holiday calendar (EPICS §8 D1) — classification only, never suppresses shifts ----
  // The week may straddle a year boundary; fetch both years (react-query dedupes equal keys).
  const mondayYear = Number(monday.slice(0, 4));
  const sundayYear = Number(sunday.slice(0, 4));
  const holidaysA = useListHolidays({ year: mondayYear }, { query: { staleTime: 5 * 60 * 1000 } });
  const holidaysB = useListHolidays(
    { year: sundayYear },
    { query: { enabled: sundayYear !== mondayYear, staleTime: 5 * 60 * 1000 } },
  );

  // date(iso) → holiday name, scoped to the company's service lines (global holidays always apply).
  const { holidaySet, holidayNameByDate } = React.useMemo(() => {
    const extract = (q: typeof holidaysA): Holiday[] => {
      const body = q.data?.data as { data?: Holiday[] } | Holiday[] | undefined;
      if (Array.isArray(body)) return body;
      return body?.data ?? [];
    };
    const all = [...extract(holidaysA), ...(sundayYear !== mondayYear ? extract(holidaysB) : [])];
    const slIds = new Set(agentRows.map((r) => r.serviceLineId).filter((x): x is string => !!x));
    return buildHolidayMaps(all, days, slIds);
  }, [holidaysA, holidaysB, agentRows, days, mondayYear, sundayYear]);

  // ---- Per-agent roster compliance (EPICS §8 D3) ----
  const complianceByRow = React.useMemo(() => {
    const m = new Map<string, RowCompliance>();
    for (const row of agentRows) {
      m.set(
        `${row.employeeId}::${row.placementId}`,
        computeCompliance(row.cells, days, holidaySet),
      );
    }
    return m;
  }, [agentRows, days, holidaySet]);

  const complianceSummary = React.useMemo(() => {
    let agentsNoRest = 0;
    let holidayShifts = 0;
    for (const c of complianceByRow.values()) {
      if (c.noRest) agentsNoRest += 1;
      holidayShifts += c.holidayShiftCount;
    }
    return { agentsNoRest, holidayShifts };
  }, [complianceByRow]);

  // ---- Sync URL search params ----
  const syncSearch = React.useCallback(
    (cid: string | null, mon: string) => {
      const s: ScheduleGridSearch = {};
      if (cid) s.company_id = cid;
      s.week = mon;
      navigate({ search: s, replace: true }).catch(() => null);
    },
    [navigate],
  );

  // ---- Week navigation ----
  const goPrevWeek = () => {
    const prev = addDays(monday, -7);
    setMonday(prev);
    syncSearch(companyId, prev);
  };
  const goNextWeek = () => {
    const next = addDays(monday, 7);
    setMonday(next);
    syncSearch(companyId, next);
  };

  // ---- Company change ----
  const handleCompanyChange = (id: string | null) => {
    setCompanyId(id);
    syncSearch(id, monday);
    setPopoverTarget(null);
  };

  // ---- Cell click ----
  const handleCellClick = (row: AgentRow, dateIso: string, cellEl: HTMLElement) => {
    const entry = row.cells[dateIso];
    // Replace popoverAnchor current (ENGINEERING.md pattern: ref points to anchor)
    (popoverAnchor as React.MutableRefObject<HTMLElement | null>).current = cellEl;
    setPopoverTarget({
      employeeId: row.employeeId,
      employeeName: row.employeeName,
      placementId: row.placementId,
      serviceLineId: row.serviceLineId,
      serviceLineName: row.serviceLineName,
      date: dateIso,
      existingEntryId: entry?.id,
      existingShiftName: entry?.shift_master_name ?? undefined,
      isDayOff: entry?.is_day_off,
    });
  };

  // ---- Error handling ----
  const queryError = scheduleQuery.error;
  const errorKind = queryError ? classifyError(queryError).kind : null;

  // ---------------------------------------------------------------------------
  // Render helpers
  // ---------------------------------------------------------------------------

  const renderCellContent = (row: AgentRow, dateIso: string) => {
    const entry = row.cells[dateIso];
    if (!entry) {
      return (
        <span
          aria-hidden
          className="flex size-6 items-center justify-center rounded-full border border-dashed border-border text-sm text-text-3 opacity-0 transition-opacity group-hover:opacity-100"
        >
          +
        </span>
      );
    }
    if (entry.status === ScheduleEntryStatus.CANCELLED_BY_LEAVE || entry.is_day_off) {
      return (
        <span className="text-xs font-semibold text-text-3">
          {entry.is_day_off ? t('cell.dayOff') : t('cell.cancelledLeave')}
        </span>
      );
    }
    if (!entry.shift_master_name) return null;

    const dotClass = shiftDotClass(entry);
    const isHolidayShift = holidaySet.has(dateIso);
    const timeShort =
      entry.start_time && entry.end_time
        ? `${entry.start_time.slice(0, 5).replace(':', '.')}–${entry.end_time.slice(0, 5).replace(':', '.')}`
        : '';

    return (
      <div className="w-full rounded-md bg-surface-2 px-2 py-1.5">
        <div className="flex items-center gap-1.5">
          <span aria-hidden className={`size-1.5 shrink-0 rounded-full ${dotClass}`} />
          <span className="truncate text-[11px] font-bold text-text leading-tight">
            {entry.shift_master_name}
          </span>
          <span className="ml-auto flex shrink-0 items-center gap-1">
            {isHolidayShift && (
              <span
                title={t('compliance.holidayShiftTip')}
                className="flex items-center gap-0.5 rounded bg-bad-bg px-1 text-[9px] font-bold text-bad-tx"
              >
                <Star aria-hidden className="size-2.5 fill-current" />
                {t('holiday.shiftBadge')}
              </span>
            )}
            {entry.cross_midnight && (
              <span className="rounded bg-warn-bg px-1 text-[9px] font-bold text-warn-tx">+1</span>
            )}
          </span>
        </div>
        {timeShort && <p className="mt-0.5 font-mono text-[10px] text-text-3">{timeShort}</p>}
      </div>
    );
  };

  // ---------------------------------------------------------------------------
  // Legend items
  // ---------------------------------------------------------------------------

  const legendItems = [
    { dot: 'bg-accent-gold', label: t('legend.pagi') },
    { dot: 'bg-accent-blue', label: t('legend.siang') },
    { dot: 'bg-text-3', label: t('legend.malam') },
    { dot: 'bg-accent-gold', label: t('legend.buildingDay') },
    { dot: 'bg-accent-gold', label: t('legend.cleaning') },
    { dot: 'bg-border', label: t('legend.off') },
    { dot: 'bg-warn-bg border border-warn-bd', label: t('legend.cuti') },
  ];

  // ---------------------------------------------------------------------------
  // JSX
  // ---------------------------------------------------------------------------

  return (
    <div className="flex min-h-full flex-col gap-4 p-6">
      {/* Page header */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-bold text-text">
            {t('screen.title', {
              company: selectedCompanyName || t('screen.titleNoCompany'),
            })}
          </h1>
          <p className="text-sm text-text-3">{t('screen.subtitle', { count: agentRows.length })}</p>
        </div>
        <div className="flex items-center gap-2.5">
          {/* Week picker */}
          <div className="flex items-center gap-2 rounded-lg border border-border bg-surface px-3 py-2">
            <button
              type="button"
              aria-label={t('weekNav.prev')}
              onClick={goPrevWeek}
              className="text-text-2 hover:text-text"
            >
              <ChevronLeft aria-hidden className="size-4" />
            </button>
            <span className="min-w-[150px] text-center text-sm font-semibold text-text">
              {formatWeekRangeId(monday, sunday)}
            </span>
            <button
              type="button"
              aria-label={t('weekNav.next')}
              onClick={goNextWeek}
              className="text-text-2 hover:text-text"
            >
              <ChevronRight aria-hidden className="size-4" />
            </button>
          </div>

          {/* Bulk apply */}
          <Button
            variant="secondary"
            onClick={() => setBulkOpen(true)}
            disabled={!companyId || agentRows.length === 0}
          >
            <Copy aria-hidden className="size-4" />
            {t('action.bulkApply')}
          </Button>
        </div>
      </div>

      {/* Company selector — hidden for shift leaders (pinned to their own company) */}
      {!isShiftLeader && (
        <div className="w-72">
          <ClientCompanyPicker
            value={companyId}
            onChange={(id) => {
              handleCompanyChange(id);
            }}
            placeholder={t('screen.companyPlaceholder')}
          />
        </div>
      )}

      {/* Auto-publish banner (INV-4) */}
      <div className="flex items-center gap-2 rounded-lg border border-ok-bd bg-ok-bg px-3 py-2.5">
        <Radio aria-hidden className="size-3.5 text-ok-tx" />
        <p className="text-xs font-medium text-ok-tx">{t('banner.autoPublish')}</p>
      </div>

      {/* Legend */}
      <div className="flex flex-wrap items-center gap-4">
        {legendItems.map((item) => (
          <div key={item.label} className="flex items-center gap-1.5">
            <span aria-hidden className={`size-2.5 rounded-full ${item.dot}`} />
            <span className="text-xs text-text-2">{item.label}</span>
          </div>
        ))}
      </div>

      {/* No company selected */}
      {!companyId && (
        <EmptyState
          variant="fresh"
          title={t('empty.noCompanyTitle')}
          description={t('empty.noCompanyDesc')}
        />
      )}

      {/* Loading skeleton */}
      {companyId && scheduleQuery.isLoading && (
        <div className="flex flex-col gap-0 overflow-hidden rounded-xl border border-border bg-surface">
          <div className="flex border-b border-border bg-surface-2 px-4 py-3">
            <span className="text-xs font-semibold uppercase tracking-wide text-text-3">
              {t('grid.agentCol')}
            </span>
          </div>
          {Array.from({ length: 5 }).map((_, i) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: skeleton row, no stable key
            <SkeletonTableRow key={i} columns={8} />
          ))}
        </div>
      )}

      {/* Error */}
      {companyId && !scheduleQuery.isLoading && scheduleQuery.isError && (
        <StateView
          kind={errorKind === 'forbidden' ? 'no-permission' : 'error'}
          title={
            errorKind === 'forbidden' ? t('error.noPermissionTitle') : t('error.loadFailedTitle')
          }
          description={
            errorKind === 'forbidden' ? t('error.noPermissionDesc') : t('error.loadFailedDesc')
          }
          onRetry={errorKind !== 'forbidden' ? () => scheduleQuery.refetch() : undefined}
          retryLabel={t('error.retry')}
        />
      )}

      {/* Empty — company selected but no placements */}
      {companyId &&
        !scheduleQuery.isLoading &&
        !scheduleQuery.isError &&
        agentRows.length === 0 && (
          <EmptyState
            variant="fresh"
            title={t('empty.noAgentsTitle')}
            description={t('empty.noAgentsDesc')}
          />
        )}

      {/* Roster-compliance summary (EPICS §8 D3) — only when something needs attention */}
      {companyId &&
        !scheduleQuery.isLoading &&
        !scheduleQuery.isError &&
        agentRows.length > 0 &&
        (complianceSummary.agentsNoRest > 0 || complianceSummary.holidayShifts > 0) && (
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 rounded-lg border border-warn-bd bg-warn-bg px-3 py-2.5">
            <div className="flex items-center gap-1.5">
              <TriangleAlert aria-hidden className="size-3.5 text-warn-tx" />
              <span className="text-xs font-semibold text-warn-tx">
                {t('compliance.bannerTitle')}
              </span>
            </div>
            {complianceSummary.agentsNoRest > 0 && (
              <span className="text-xs font-medium text-warn-tx">
                {t('compliance.bannerNoRest', { count: complianceSummary.agentsNoRest })}
              </span>
            )}
            {complianceSummary.holidayShifts > 0 && (
              <span className="text-xs font-medium text-warn-tx">
                {t('compliance.bannerHolidayShift', { count: complianceSummary.holidayShifts })}
              </span>
            )}
          </div>
        )}

      {/* Grid */}
      {companyId && !scheduleQuery.isLoading && !scheduleQuery.isError && agentRows.length > 0 && (
        <div
          ref={popoverContainerRef}
          className="relative overflow-hidden rounded-xl border border-border bg-surface"
        >
          {/* Grid table — flex-based to match .pen grid layout */}
          <div className="w-full overflow-x-auto">
            {/* Header row */}
            <div
              className="flex border-b border-border bg-surface-2"
              style={{ minWidth: `${AGENT_COL_W + 7 * 120}px` }}
            >
              {/* AGEN column header */}
              <div
                className="shrink-0 border-r border-border-soft px-4 py-2.5"
                style={{ width: `${AGENT_COL_W}px` }}
              >
                <span className="text-[11px] font-semibold uppercase tracking-[0.5px] text-text-3">
                  {t('grid.agentCol')}
                </span>
              </div>
              {/* Day headers */}
              {days.map((d, i) => {
                const isToday = d === todayIso;
                const holidayName = holidayNameByDate.get(d);
                const isHoliday = !!holidayName;
                return (
                  <div
                    key={d}
                    title={holidayName}
                    className={`flex flex-1 flex-col items-center justify-center border-r border-border-soft px-1 py-2 last:border-r-0 ${
                      isToday ? 'bg-primary-soft' : isHoliday ? 'bg-bad-bg/40' : ''
                    }`}
                  >
                    <span
                      className={`text-[10px] font-semibold tracking-[0.3px] ${
                        isToday ? 'text-primary' : isHoliday ? 'text-bad-tx' : 'text-text-3'
                      }`}
                    >
                      {DAY_ABBR_ID[i]}
                    </span>
                    <span
                      className={`text-[13px] font-bold ${
                        isToday ? 'text-primary-strong' : isHoliday ? 'text-bad-tx' : 'text-text'
                      }`}
                    >
                      {formatDayMonthId(d)}
                    </span>
                    {isHoliday && (
                      <span className="mt-0.5 max-w-full truncate text-[9px] font-medium text-bad-tx">
                        {holidayName}
                      </span>
                    )}
                  </div>
                );
              })}
            </div>

            {/* Agent rows */}
            {agentRows.map((row, rowIdx) => (
              <div
                key={`${row.employeeId}::${row.placementId}`}
                className={`flex border-b border-border-soft last:border-b-0 ${
                  rowIdx % 2 === 1 ? 'bg-surface' : 'bg-surface'
                }`}
                style={{ minWidth: `${AGENT_COL_W + 7 * 120}px` }}
              >
                {/* Agent name column */}
                {(() => {
                  const c = complianceByRow.get(`${row.employeeId}::${row.placementId}`);
                  return (
                    <div
                      className="shrink-0 border-r border-border-soft px-4 py-2.5"
                      style={{ width: `${AGENT_COL_W}px` }}
                    >
                      <p className="text-[13px] font-semibold text-text leading-tight">
                        {row.employeeName}
                      </p>
                      {row.serviceLineName && (
                        <p className="mt-0.5 text-[11px] text-text-3">{row.serviceLineName}</p>
                      )}
                      {c && (c.noRest || c.longRun || c.holidayShiftCount > 0) && (
                        <div className="mt-1 flex flex-wrap items-center gap-1">
                          {c.noRest ? (
                            <span
                              title={t('compliance.noRestTip')}
                              className="inline-flex items-center gap-0.5 rounded bg-bad-bg px-1 py-0.5 text-[9px] font-semibold text-bad-tx"
                            >
                              <TriangleAlert aria-hidden className="size-2.5" />
                              {t('compliance.noRest')}
                            </span>
                          ) : (
                            c.longRun && (
                              <span
                                title={t('compliance.longRunTip', { count: c.longestRun })}
                                className="inline-flex items-center gap-0.5 rounded bg-warn-bg px-1 py-0.5 text-[9px] font-semibold text-warn-tx"
                              >
                                <TriangleAlert aria-hidden className="size-2.5" />
                                {t('compliance.longRun', { count: c.longestRun })}
                              </span>
                            )
                          )}
                          {c.holidayShiftCount > 0 && (
                            <span
                              title={t('compliance.holidayShiftTip')}
                              className="inline-flex items-center gap-0.5 rounded bg-bad-bg px-1 py-0.5 text-[9px] font-semibold text-bad-tx"
                            >
                              <Star aria-hidden className="size-2.5 fill-current" />
                              {t('compliance.holidayShift', { count: c.holidayShiftCount })}
                            </span>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })()}

                {/* Day cells */}
                {days.map((d) => {
                  const entry = row.cells[d];
                  const isToday = d === todayIso;
                  const isCancelled = entry?.status === ScheduleEntryStatus.CANCELLED_BY_LEAVE;
                  const isHoliday = holidaySet.has(d);

                  return (
                    <button
                      key={d}
                      type="button"
                      aria-label={t('cell.ariaLabel', {
                        agent: row.employeeName,
                        date: formatDayMonthId(d),
                      })}
                      onClick={(e) => handleCellClick(row, d, e.currentTarget)}
                      className={[
                        'group relative flex flex-1 cursor-pointer items-center justify-center border-r border-border-soft p-1.5 text-left transition-colors last:border-r-0',
                        isToday && !isCancelled
                          ? 'bg-primary-soft/40'
                          : isHoliday && !isCancelled
                            ? 'bg-bad-bg/20'
                            : '',
                        isCancelled ? 'opacity-50' : '',
                        'hover:bg-surface-2',
                      ]
                        .filter(Boolean)
                        .join(' ')}
                      style={{ minHeight: '64px' }}
                    >
                      {renderCellContent(row, d)}
                    </button>
                  );
                })}
              </div>
            ))}
          </div>

          {/* Shift picker popover — absolutely positioned relative to grid container */}
          {popoverTarget && (
            <ShiftPickerPopover
              target={popoverTarget}
              anchorRef={popoverAnchor}
              onClose={() => setPopoverTarget(null)}
              onMutated={() => {
                /* grid refetched via invalidate inside popover */
              }}
              scheduleQueryKey={scheduleQueryKey}
            />
          )}
        </div>
      )}

      {/* Bulk apply modal */}
      {bulkOpen && companyId && (
        <BulkApplyModal
          open={bulkOpen}
          onClose={() => setBulkOpen(false)}
          companyId={companyId}
          employeeIds={agentRows.map((r) => r.employeeId)}
          onMutated={() => {
            qc.invalidateQueries({ queryKey: scheduleQueryKey }).catch(() => null);
          }}
          scheduleQueryKey={scheduleQueryKey}
        />
      )}
    </div>
  );
}
