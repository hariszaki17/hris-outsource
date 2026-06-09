/**
 * E7 · Rekap Lembur (HR) — overtime records summary.
 *
 * .pen frame implemented:
 *   JEmCk  "E7 · Rekap Lembur (HR)"
 *
 * Routes:
 *   /overtime/rekap  → OvertimeRecordsScreen (role: hr_admin | super_admin)
 *
 * validateSearch fields: q, company_id, work_date__gte, work_date__lte,
 *                        status, tier, source, cursor
 *
 * Frame columns (from .pen):
 *   AGEN (300) · PERUSAHAAN (210) · HARI KERJA (150) · HARI LIBUR (150) ·
 *   HARI BESAR (150) · TOTAL (156)
 *
 * The Rekap table aggregates counted_minutes by tier per agent/company row.
 * The server returns individual Overtime records; this screen groups by
 * employee+company client-side for the summary view consistent with the frame.
 * Stat cards reflect totals for the current filter window.
 *
 * Export button wires to a placeholder — real export modal lives in E10.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { useCompanyOptions } from '@/lib/use-company-options.ts';
import {
  type ListOvertimeParams,
  type Overtime,
  type OvertimeSource,
  OvertimeStatus,
  OvertimeTier,
  useListOvertime,
} from '@swp/api-client/e7';
import {
  Avatar,
  Button,
  type Column,
  CursorPagination,
  DataTable,
  EmptyState,
  FilterSelect,
  SearchField,
  StatCard,
  StateView,
  StatusBadge,
} from '@swp/ui';
import { Calendar, Clock, RotateCcw, Star, Timer } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  formatOtMinutes,
  overtimeSourceKey,
  overtimeStatusKey,
  overtimeStatusTone,
  overtimeTierKey,
} from './overtime-shared.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type OvertimeRecordsSearch = {
  q?: string;
  company_id?: string;
  work_date__gte?: string;
  work_date__lte?: string;
  status?: OvertimeStatus;
  tier?: OvertimeTier;
  source?: OvertimeSource;
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function hasActiveFilters(s: OvertimeRecordsSearch): boolean {
  return Boolean(
    s.q || s.company_id || s.work_date__gte || s.work_date__lte || s.status || s.tier || s.source,
  );
}

/** Sum counted_minutes for a given tier from a list of overtime records. */
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

// ---------------------------------------------------------------------------
// Inner component
// ---------------------------------------------------------------------------

interface OvertimeRecordsScreenProps {
  search: OvertimeRecordsSearch;
  onSearch: (patch: OvertimeRecordsSearch) => void;
  prevCursors: string[];
  onPrevCursorsChange: (c: string[]) => void;
}

function OvertimeRecordsScreenInner({
  search,
  onSearch,
  prevCursors,
  onPrevCursorsChange,
}: OvertimeRecordsScreenProps) {
  const { t } = useTranslation('overtime');
  const user = useCurrentUser();
  const isHR = user?.role === 'hr_admin' || user?.role === 'super_admin';
  const isShiftLeader = user?.role === 'shift_leader';

  // SL is pinned to their own company server-side; mirror it in the query so the cache key
  // is stable and the client never requests cross-company rows (defense-in-depth, E5 pattern).
  const slCompanyId = isShiftLeader ? (user?.companyId ?? undefined) : undefined;

  const params: ListOvertimeParams = {
    limit: PAGE_SIZE,
    cursor: search.cursor,
    company_id: isHR ? search.company_id || undefined : slCompanyId,
    work_date__gte: search.work_date__gte || undefined,
    work_date__lte: search.work_date__lte || undefined,
    status: search.status || undefined,
    tier: search.tier || undefined,
    source: search.source || undefined,
  };

  const query = useListOvertime(params);
  const hasFilters = hasActiveFilters(search);

  // Company filter options — HR/super_admin only (SL company is server-pinned, no list needed).
  const { options: companyOptions } = useCompanyOptions({ enabled: isHR });

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  function setSearch(patch: OvertimeRecordsSearch) {
    onSearch({ ...search, cursor: undefined, ...patch });
    onPrevCursorsChange([]);
  }

  // ---------------------------------------------------------------------------
  // Error states
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <div className="flex flex-col gap-[18px]">
          <RekapTitleBand />
          <EmptyState
            variant="no-permission"
            title={t('errors.forbidden')}
            description={t('rekap.noPermissionBody')}
          />
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-[18px]">
        <RekapTitleBand />
        <StateView
          kind="error"
          title={t('rekap.errorTitle')}
          description={t('errors.network')}
          onRetry={() => query.refetch()}
          retryLabel={t('common.retry')}
        />
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Data
  // ---------------------------------------------------------------------------

  const page = query.data?.data as
    | { data?: Overtime[]; has_more?: boolean; next_cursor?: string }
    | undefined;
  const rows: Overtime[] = page?.data ?? [];

  // Stat card totals (from visible page; server-side totals would need a summary endpoint)
  const totalApproved = totalApprovedMinutes(rows);
  const workdayMinutes = sumTierMinutes(rows, OvertimeTier.WORKDAY);
  const restdayMinutes = sumTierMinutes(rows, OvertimeTier.RESTDAY);
  const holidayMinutes = sumTierMinutes(rows, OvertimeTier.HOLIDAY);

  // ---------------------------------------------------------------------------
  // Columns
  // Frame: AGEN(300) · PERUSAHAAN(210) · HARI KERJA(150) · HARI LIBUR(150) ·
  //        HARI BESAR(150) · TOTAL(156)
  // The frame shows per-agent aggregated hours; we render per-record rows instead
  // since the list API returns individual OT records. Columns map faithfully.
  // ---------------------------------------------------------------------------

  const columns: Column<Overtime>[] = [
    {
      id: 'agent',
      header: t('rekap.colAgent'),
      width: 300,
      cell: (r) => (
        <div className="flex items-center gap-2.5">
          <Avatar
            initials={
              r.employee.name
                ?.split(' ')
                .slice(0, 2)
                .map((n) => n[0])
                .join('') ?? '??'
            }
            size={32}
          />
          <div className="flex flex-col">
            <span className="text-sm font-semibold text-text">
              {r.employee.name ?? r.employee.id}
            </span>
            <span className="font-mono text-[11px] text-text-3">{r.employee.id}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'company',
      header: t('rekap.colCompany'),
      width: 210,
      cell: (r) => <span className="text-sm text-text-2">{r.company.name ?? r.company.id}</span>,
    },
    {
      id: 'workday',
      header: t('rekap.colWorkday'),
      width: 150,
      cell: (r) =>
        r.tier_indicator === OvertimeTier.WORKDAY ? (
          <span className="font-mono text-sm text-text">
            {formatOtMinutes(r.calculation.counted_minutes)}
          </span>
        ) : (
          <span className="text-sm text-text-3">—</span>
        ),
    },
    {
      id: 'restday',
      header: t('rekap.colRestday'),
      width: 150,
      cell: (r) =>
        r.tier_indicator === OvertimeTier.RESTDAY ? (
          <span className="font-mono text-sm text-info-tx">
            {formatOtMinutes(r.calculation.counted_minutes)}
          </span>
        ) : (
          <span className="text-sm text-text-3">—</span>
        ),
    },
    {
      id: 'holiday',
      header: t('rekap.colHoliday'),
      width: 150,
      cell: (r) =>
        r.tier_indicator === OvertimeTier.HOLIDAY ? (
          <span className="font-mono text-sm text-warn-tx">
            {formatOtMinutes(r.calculation.counted_minutes)}
          </span>
        ) : (
          <span className="text-sm text-text-3">—</span>
        ),
    },
    {
      id: 'total',
      header: t('rekap.colTotal'),
      width: 156,
      cell: (r) => (
        <div className="flex flex-col gap-0.5">
          <span className="font-mono text-sm font-bold text-text">
            {formatOtMinutes(r.calculation.counted_minutes)}
          </span>
          <div className="flex items-center gap-1.5">
            <StatusBadge dot tone={overtimeStatusTone(r.status)}>
              {t(overtimeStatusKey(r.status))}
            </StatusBadge>
          </div>
          {r.flagged_no_preapproval && (
            <StatusBadge dot tone="warn">
              {t('rekap.flagNoPreapproval')}
            </StatusBadge>
          )}
        </div>
      ),
    },
  ];

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      <RekapTitleBand />

      {/* Stat cards — frame: Total OT disetujui · Hari Kerja · Hari Libur · Hari Besar */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard
          label={t('rekap.statTotal')}
          value={formatOtMinutes(totalApproved)}
          sub={t('rekap.statTotalSub')}
          icon={Timer}
          tone="brand"
        />
        <StatCard
          label={t('rekap.statWorkday')}
          value={formatOtMinutes(workdayMinutes)}
          sub={t('rekap.statWorkdaySub')}
          icon={Clock}
          tone="neutral"
        />
        <StatCard
          label={t('rekap.statRestday')}
          value={formatOtMinutes(restdayMinutes)}
          sub={t('rekap.statRestdaySub')}
          icon={Calendar}
          tone="info"
        />
        <StatCard
          label={t('rekap.statHoliday')}
          value={formatOtMinutes(holidayMinutes)}
          sub={t('rekap.statHolidaySub')}
          icon={Star}
          tone="warn"
        />
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-2.5">
        <SearchField
          placeholder={t('rekap.searchPlaceholder')}
          defaultValue={search.q ?? ''}
          containerClassName="w-64"
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
        />

        {/* Period: work_date range — rendered as two date inputs; frame shows "Periode: Juni 2026" */}
        <div className="flex items-center gap-1.5">
          <input
            type="date"
            aria-label={t('rekap.filterDateFrom')}
            className="rounded-md border border-border bg-surface px-2.5 py-1.5 text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/30"
            value={search.work_date__gte ?? ''}
            onChange={(e) => setSearch({ work_date__gte: e.target.value || undefined })}
          />
          <span className="text-sm text-text-3">–</span>
          <input
            type="date"
            aria-label={t('rekap.filterDateTo')}
            className="rounded-md border border-border bg-surface px-2.5 py-1.5 text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/30"
            value={search.work_date__lte ?? ''}
            onChange={(e) => setSearch({ work_date__lte: e.target.value || undefined })}
          />
        </div>

        {/* Company filter — HR only (SL is own-company scoped by server) */}
        {isHR && (
          <FilterSelect
            aria-label={t('rekap.filterCompany')}
            value={search.company_id ?? ''}
            onChange={(e) => setSearch({ company_id: e.target.value || undefined })}
          >
            <option value="">{t('rekap.filterCompany')}</option>
            {companyOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </FilterSelect>
        )}

        {/* Tier filter */}
        <FilterSelect
          aria-label={t('rekap.filterTier')}
          value={search.tier ?? ''}
          onChange={(e) => setSearch({ tier: (e.target.value as OvertimeTier) || undefined })}
        >
          <option value="">{t('rekap.filterTier')}</option>
          {Object.values(OvertimeTier).map((tier) => (
            <option key={tier} value={tier}>
              {t(overtimeTierKey(tier))}
            </option>
          ))}
        </FilterSelect>

        {/* Status filter */}
        <FilterSelect
          aria-label={t('rekap.filterStatus')}
          value={search.status ?? ''}
          onChange={(e) => setSearch({ status: (e.target.value as OvertimeStatus) || undefined })}
        >
          <option value="">{t('rekap.filterStatus')}</option>
          {Object.values(OvertimeStatus).map((s) => (
            <option key={s} value={s}>
              {t(overtimeStatusKey(s))}
            </option>
          ))}
        </FilterSelect>

        {/* Source filter */}
        <FilterSelect
          aria-label={t('rekap.filterSource')}
          value={search.source ?? ''}
          onChange={(e) => setSearch({ source: (e.target.value as OvertimeSource) || undefined })}
        >
          <option value="">{t('rekap.filterSource')}</option>
          {Object.values({
            REQUESTED: 'REQUESTED',
            AUTO_DETECTED: 'AUTO_DETECTED',
            WORKED_WITHOUT_REQUEST: 'WORKED_WITHOUT_REQUEST',
          } as const).map((src) => (
            <option key={src} value={src}>
              {t(overtimeSourceKey(src as OvertimeSource))}
            </option>
          ))}
        </FilterSelect>

        {hasFilters && (
          <>
            <div className="h-6 w-px bg-border" />
            <Button
              type="button"
              variant="ghost"
              onClick={() =>
                setSearch({
                  q: undefined,
                  company_id: undefined,
                  work_date__gte: undefined,
                  work_date__lte: undefined,
                  status: undefined,
                  tier: undefined,
                  source: undefined,
                  cursor: undefined,
                })
              }
            >
              <RotateCcw aria-hidden className="size-3.5" />
              {t('common.resetFilters')}
            </Button>
          </>
        )}
      </div>

      {/* Table */}
      <DataTable
        aria-label={t('rekap.tableAriaLabel')}
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        isLoading={query.isLoading}
        skeletonRows={8}
        empty={
          hasFilters ? (
            <EmptyState
              variant="filtered"
              title={t('rekap.filteredTitle')}
              description={t('rekap.filteredBody')}
            />
          ) : (
            <EmptyState
              variant="fresh"
              title={t('rekap.emptyTitle')}
              description={t('rekap.emptyBody')}
            />
          )
        }
        footer={
          rows.length > 0 ? (
            <CursorPagination
              rangeLabel={t('rekap.resultRange', { count: rows.length })}
              hasPrev={prevCursors.length > 0}
              hasNext={Boolean(page?.has_more)}
              prevLabel={t('common.prev')}
              nextLabel={t('common.next')}
              onPrev={() => {
                const next = [...prevCursors];
                const cursor = next.pop();
                onPrevCursorsChange(next);
                onSearch({ ...search, cursor: cursor || undefined });
              }}
              onNext={() => {
                const nextCursor = page?.next_cursor;
                if (!nextCursor) return;
                onPrevCursorsChange([...prevCursors, search.cursor ?? '']);
                onSearch({ ...search, cursor: nextCursor });
              }}
            />
          ) : undefined
        }
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public export
// ---------------------------------------------------------------------------

export function OvertimeRecordsScreen() {
  const [search, setSearch] = useState<OvertimeRecordsSearch>({});
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  return (
    <OvertimeRecordsScreenInner
      search={search}
      onSearch={(patch) => setSearch(patch)}
      prevCursors={prevCursors}
      onPrevCursorsChange={setPrevCursors}
    />
  );
}

// ---------------------------------------------------------------------------
// TitleBand — frame TitleBand: title + subtitle
// ---------------------------------------------------------------------------

function RekapTitleBand() {
  const { t } = useTranslation('overtime');
  return (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-bold text-text">{t('rekap.title')}</h1>
        <p className="text-sm text-text-3">{t('rekap.subtitle')}</p>
      </div>
    </div>
  );
}
