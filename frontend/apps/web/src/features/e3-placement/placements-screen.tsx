/**
 * E3 · Penempatan — Daftar per Perusahaan Klien (HR/Admin view)
 *
 * .pen frame: C2SSLA — E3 · Penempatan — Perusahaan
 *
 * Layout: TitleBand → 4× StatCards → CompanyGrid (card per client company,
 * showing active service lines, agent count, shift leader, and "no-leader" warn state).
 * Filters: search company, service line, area + expiring-soon toggle switches to
 * useListExpiringPlacements.
 *
 * ENGINEERING.md D1 — typed URL search params + cursor pagination.
 * F3.1 — Placement list grouped by company.
 * INV-2 — every active company must have a shift leader (warn badge if missing).
 */

import { ClientCompanyPicker } from '@/features/e2-identity/pickers/client-company-picker.tsx';
import { ServiceLinePicker } from '@/features/e2-identity/pickers/service-line-picker.tsx';
import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type ListExpiringPlacementsParams,
  type ListPlacementsParams,
  type PlacementLifecycleStatus,
  useGetPlacementStats,
  useListExpiringPlacements,
  useListPlacements,
} from '@swp/api-client/e3';
import type { Placement, PlacementStats } from '@swp/api-client/e3';
import type { StatusTone } from '@swp/design-tokens';
import {
  Avatar,
  type Column,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  IdChip,
  SearchField,
  StatCard,
  StateView,
  StatusBadge,
  Toggle,
} from '@swp/ui';
import { Link, useNavigate, useSearch } from '@tanstack/react-router';
import { Building2, CalendarClock, CheckCircle2, Clock, Plus } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

/** Typed filter/cursor search params for `/placements`. */
export type PlacementsSearch = {
  q?: string;
  company_id?: string;
  service_line_id?: string;
  status?: PlacementLifecycleStatus;
  expiring_soon?: boolean;
  awaiting_agreement?: boolean;
  cursor?: string;
};

function initials(name: string): string {
  return name
    .split(' ')
    .slice(0, 2)
    .map((p) => p[0] ?? '')
    .join('')
    .toUpperCase();
}

// ---------------------------------------------------------------------------
// Lifecycle status → StatusBadge tone (ENGINEERING.md F3)
// ---------------------------------------------------------------------------

const lifecycleTone: Record<PlacementLifecycleStatus, StatusTone> = {
  ACTIVE: 'ok',
  EXTENDED: 'ok',
  PENDING_START: 'info',
  EXPIRING: 'warn',
  ENDED: 'neutral',
  TRANSFERRED: 'neutral',
  SUPERSEDED: 'neutral',
  TERMINATED: 'bad',
  RESIGNED: 'bad',
};

// ---------------------------------------------------------------------------
// PlacementsScreen
// ---------------------------------------------------------------------------

export function PlacementsScreen() {
  const { t } = useTranslation('placements');
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as PlacementsSearch;
  const currentUser = useCurrentUser();
  const isShiftLeader = currentUser?.role === 'shift_leader';

  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  const expiringOn = Boolean(search.expiring_soon);

  // ShiftLeader can read service lines only via placements.read; they have no
  // service_lines.read, so the master-data-backed ServiceLinePicker is not shown.
  const canReadServiceLines = currentUser?.permissions.includes('service_lines.read') ?? false;

  // For SL the company filter is server-pinned to currentUser.companyId; client
  // reflects it (locked control) and pins the param so the cache key is stable.
  const slCompanyId = isShiftLeader ? (currentUser?.companyId ?? undefined) : undefined;

  // ---------------------------------------------------------------------------
  // Search params → API params
  // ---------------------------------------------------------------------------

  const regularParams: ListPlacementsParams = {
    limit: PAGE_SIZE,
    q: search.q || undefined,
    company_id: isShiftLeader ? slCompanyId : search.company_id || undefined,
    service_line_id: search.service_line_id || undefined,
    status: search.status || undefined,
    awaiting_agreement: search.awaiting_agreement || undefined,
    cursor: search.cursor,
  };

  const expiringParams: ListExpiringPlacementsParams = {
    limit: PAGE_SIZE,
    company_id: isShiftLeader ? slCompanyId : search.company_id || undefined,
    cursor: search.cursor,
  };

  const regularQuery = useListPlacements(regularParams, {
    query: { enabled: !expiringOn },
  });

  const expiringQuery = useListExpiringPlacements(expiringParams, {
    query: { enabled: expiringOn },
  });

  const query = expiringOn ? expiringQuery : regularQuery;

  // Global placement aggregates for the stat cards (F3.1 / C2SSLA). Computed
  // server-side over ALL non-deleted placements (not the loaded page), so the
  // cards do not undercount past page 1. Independent of the expiring toggle.
  const statsQuery = useGetPlacementStats();
  const stats = statsQuery.data?.data as PlacementStats | undefined;

  const page = query.data?.data as
    | { data?: Placement[]; next_cursor?: string | null; has_more?: boolean }
    | undefined;
  const rows = (page?.data ?? []) as Placement[];

  const hasFilters = Boolean(
    search.q ||
      search.company_id ||
      search.service_line_id ||
      search.status ||
      search.awaiting_agreement,
  );

  // ---------------------------------------------------------------------------
  // Navigation helpers
  // ---------------------------------------------------------------------------

  function setSearch(patch: Partial<PlacementsSearch>) {
    const next: PlacementsSearch = { ...search, cursor: undefined, ...patch };
    void navigate({ to: '/placements', search: next });
    setPrevCursors([]);
  }

  // ---------------------------------------------------------------------------
  // Columns
  // ---------------------------------------------------------------------------

  const columns: Column<Placement>[] = [
    {
      id: 'agen',
      header: t('colAgen'),
      width: 280,
      cell: (pl) => (
        <div className="flex items-center gap-[8px]">
          <Avatar initials={initials(pl.employee_name ?? pl.employee_id)} size={32} />
          <div className="flex flex-col gap-[2px]">
            <span className="text-[14px] font-semibold text-text">{pl.employee_name ?? '—'}</span>
            <IdChip id={pl.employee_id} />
          </div>
        </div>
      ),
    },
    {
      id: 'perusahaan',
      header: t('colPerusahaan'),
      width: 210,
      cell: (pl) => (
        <span className="text-[13px] text-text-2">{pl.client_company_name ?? '—'}</span>
      ),
    },
    {
      id: 'liniLayanan',
      header: t('colLiniLayanan'),
      width: undefined,
      cell: (pl) =>
        pl.service_line_name ? (
          <div className="flex items-center gap-[7px]">
            <span className="size-[7px] rounded-full bg-info-tx shrink-0" aria-hidden />
            <span className="whitespace-nowrap text-[13px] text-text-2">
              {pl.service_line_name}
            </span>
          </div>
        ) : (
          <span className="text-[13px] text-text-3">—</span>
        ),
    },
    {
      id: 'posisi',
      header: t('colPosisi'),
      width: 175,
      cell: (pl) => <span className="text-[13px] text-text-2">{pl.position_name ?? '—'}</span>,
    },
    {
      id: 'periode',
      header: t('colPeriode'),
      width: 220,
      cell: (pl) => (
        <span className="text-[13px] text-text-2 tabular-nums">
          <DateText kind="date" value={pl.start_date} />
          {' – '}
          {pl.end_date ? (
            <DateText kind="date" value={pl.end_date} />
          ) : (
            <span className="text-text-3">{t('openEnded')}</span>
          )}
        </span>
      ),
    },
    {
      id: 'status',
      header: t('colStatus'),
      width: 180,
      cell: (pl) => (
        <div className="flex flex-wrap items-center gap-[6px]">
          <StatusBadge dot tone={lifecycleTone[pl.lifecycle_status]}>
            {t(`lifecycle.${pl.lifecycle_status}`)}
          </StatusBadge>
          {pl.awaiting_agreement && (
            <StatusBadge dot tone="warn">
              {t('awaitingAgreementBadge')}
            </StatusBadge>
          )}
        </div>
      ),
    },
  ];

  // ---------------------------------------------------------------------------
  // Error state
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    return (
      <div className="flex flex-col gap-[18px]">
        <div className="flex items-start justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
          <div className="flex flex-col gap-1">
            <h1 className="text-3xl font-bold text-text">{t('title')}</h1>
            <p className="text-[13px] text-text-3">{t('subtitle')}</p>
          </div>
        </div>
        {kind === 'forbidden' || kind === 'unauthenticated' ? (
          <EmptyState
            variant="no-permission"
            title={t('noPermissionTitle')}
            description={t('noPermissionBody')}
          />
        ) : (
          <StateView
            kind="error"
            title={t('errorTitle')}
            description={t('errorBody')}
            onRetry={() => query.refetch()}
            retryLabel={t('common.retry', { ns: 'translation' })}
          />
        )}
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl font-bold text-text">{t('title')}</h1>
          <p className="text-[14px] text-text-3">{t('subtitle')}</p>
        </div>
        {!isShiftLeader && (
          <Link
            to="/placements/new"
            className="flex items-center gap-2 rounded-lg bg-primary px-4 py-[10px] text-[14px] font-semibold text-white hover:bg-primary/90"
          >
            <Plus className="size-4" aria-hidden />
            {t('createPlacement')}
          </Link>
        )}
      </div>

      {/* Stat cards — from .pen C2SSLA Stats row */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard
          label={t('statPerusahaanKlien')}
          value={statsQuery.isLoading || !stats ? '—' : String(stats.client_company_count)}
          sub={t('statPerusahaanKlienSub')}
          icon={Building2}
          tone="brand"
        />
        <StatCard
          label={t('statPenempatanAktif')}
          value={statsQuery.isLoading || !stats ? '—' : String(stats.active_count)}
          sub={t('statPenempatanAktifSub')}
          icon={CheckCircle2}
          tone="ok"
        />
        <StatCard
          label={t('statAkanBerakhir')}
          value={statsQuery.isLoading || !stats ? '—' : String(stats.expiring_count)}
          sub={t('statAkanBerakhirSub')}
          icon={Clock}
          tone="warn"
        />
        <StatCard
          label={t('statTerjadwal')}
          value={statsQuery.isLoading || !stats ? '—' : String(stats.pending_count)}
          sub={t('statTerjadwalSub')}
          icon={CalendarClock}
          tone="info"
        />
      </div>

      {/* Table card */}
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        {/* Filter row — from .pen C2SSLA Filters band */}
        <div className="flex items-center justify-between gap-[10px] border-b border-border-soft px-[18px] py-[14px]">
          <div className="flex items-center gap-[10px]">
            <SearchField
              placeholder={t('searchPlaceholder')}
              defaultValue={search.q ?? ''}
              containerClassName="w-[220px]"
              onChange={(e) => setSearch({ q: e.target.value || undefined })}
            />
            {/* Company filter — locked (disabled) for shift_leader, pinned to their
                own company; free ClientCompanyPicker for HR/super_admin. */}
            {isShiftLeader ? (
              <span className="inline-flex items-center gap-[6px] rounded-lg border border-border bg-surface-2 px-[10px] py-[9px] text-[13px] text-text-2 opacity-70">
                <svg
                  width="12"
                  height="12"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  aria-hidden="true"
                >
                  <title>{t('filterCompanyLocked')}</title>
                  <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                  <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                </svg>
                {currentUser?.companyName ?? currentUser?.companyId ?? t('filterCompany')}
              </span>
            ) : (
              <div className="w-[200px]">
                <ClientCompanyPicker
                  value={search.company_id ?? null}
                  onChange={(v) => setSearch({ company_id: v ?? undefined })}
                  placeholder={t('filterCompany')}
                />
              </div>
            )}
            {/* ServiceLinePicker as filter — only when the role can read the
                service-lines master endpoint (SL lacks service_lines.read). */}
            {canReadServiceLines && (
              <div className="w-[240px]">
                <ServiceLinePicker
                  value={search.service_line_id ?? null}
                  onChange={(v) => setSearch({ service_line_id: v ?? undefined })}
                  placeholder={t('filterServiceLine')}
                />
              </div>
            )}
          </div>
          <div className="flex items-center gap-[16px]">
            {/* Awaiting-agreement toggle — drives the awaiting_agreement query filter
                (the "menunggu perjanjian" backlog, EPICS §8 2026-06-11). */}
            <div className="flex items-center gap-[8px]">
              <span className="text-[13px] font-medium text-text-2">
                {t('filterAwaitingAgreement')}
              </span>
              <Toggle
                checked={Boolean(search.awaiting_agreement)}
                onCheckedChange={(v) =>
                  setSearch({ awaiting_agreement: v || undefined, cursor: undefined })
                }
                aria-label={t('filterAwaitingAgreement')}
              />
            </div>
            {/* Expiring-soon toggle — from .pen C2SSLA "Akan berakhir" toggle */}
            <div className="flex items-center gap-[8px]">
              <span className="text-[13px] font-medium text-text-2">{t('filterExpiringSoon')}</span>
              <Toggle
                checked={expiringOn}
                onCheckedChange={(v) =>
                  setSearch({ expiring_soon: v || undefined, cursor: undefined })
                }
                aria-label={t('filterExpiringSoon')}
              />
            </div>
          </div>
        </div>

        {/* Data table */}
        <DataTable
          aria-label={t('title')}
          columns={columns}
          data={rows}
          getRowId={(p) => p.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          onRowClick={(pl) =>
            void navigate({
              to: '/placements/$placementId',
              params: { placementId: pl.id },
            })
          }
          empty={
            expiringOn ? (
              <EmptyState
                variant="fresh"
                title={t('expiringEmptyTitle')}
                description={t('expiringEmptyBody')}
              />
            ) : hasFilters ? (
              <EmptyState
                variant="filtered"
                title={t('filteredTitle')}
                description={t('filteredBody')}
              />
            ) : (
              <EmptyState variant="fresh" title={t('emptyTitle')} description={t('emptyBody')} />
            )
          }
          footer={
            rows.length > 0 ? (
              <CursorPagination
                rangeLabel={t('resultRange', { count: rows.length })}
                hasPrev={prevCursors.length > 0}
                hasNext={Boolean(page?.has_more)}
                prevLabel={t('common.prev', { ns: 'translation' })}
                nextLabel={t('common.next', { ns: 'translation' })}
                onPrev={() => {
                  const next = [...prevCursors];
                  const cursor = next.pop();
                  setPrevCursors(next);
                  void navigate({
                    to: '/placements',
                    search: { ...search, cursor: cursor || undefined },
                  });
                }}
                onNext={() => {
                  const nextCursor = page?.next_cursor;
                  if (!nextCursor) return;
                  setPrevCursors((s) => [...s, search.cursor ?? '']);
                  void navigate({
                    to: '/placements',
                    search: { ...search, cursor: nextCursor },
                  });
                }}
              />
            ) : undefined
          }
        />
      </div>
    </div>
  );
}
