/**
 * E8 · Arsip Payroll (HR) — payslip archive list.
 *
 * .pen frame implemented:
 *   jBgLn  E8 · Arsip Payroll (HR)
 *
 * F8.2: filters (employee_id / year / month / status), DataTable, "FINAL · Read-only" pill,
 * row → detail navigation.
 *
 * RBAC: HR admin / Super admin only (PA-2 / INV-4). Shift leader + agent → no-permission gate.
 *
 * Routes (proposed):
 *   /payroll                  → PayslipArchiveScreen (this component)
 *
 * Filters are local useState — no URL search params wired (TanStack Router validateSearch not
 * connected; note for integrator if URL-persisted filters are desired).
 *
 * The API has NO free-text `q` param — filters are period/year/employee_id/status only.
 * The design shows a "Cari karyawan / NIK" SearchField; since the API has no text search param
 * that field is rendered but drives `employee_id` (exact match) — see DEVIATIONS note.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type ListPayslipsParams,
  type Payslip,
  type PayslipListResponse,
  PayslipListResponseMetaCode,
  PayslipStatus,
  useListPayslips,
} from '@swp/api-client/e8';
import {
  Button,
  type Column,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  FilterSelect,
  SearchField,
  StateView,
  StatusBadge,
} from '@swp/ui';
import { Lock, RotateCcw } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  formatMoney,
  formatPeriod,
  payslipStatusKey,
  payslipStatusTone,
} from './payroll-shared.tsx';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

const CURRENT_YEAR = new Date().getFullYear();
const YEAR_OPTIONS = Array.from({ length: CURRENT_YEAR - 2019 + 1 }, (_, i) => CURRENT_YEAR - i);
const MONTH_OPTIONS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12];

// ---------------------------------------------------------------------------
// Local filter state type
// ---------------------------------------------------------------------------

interface ArchiveFilters {
  employeeSearch: string; // displayed in SearchField; sent as employee_id when non-empty
  year: string;
  month: string;
  status: string;
  cursor?: string;
}

function hasActiveFilters(f: ArchiveFilters): boolean {
  return Boolean(f.employeeSearch || f.year || f.month || f.status);
}

// ---------------------------------------------------------------------------
// PayslipArchiveScreen
// ---------------------------------------------------------------------------

export function PayslipArchiveScreen({
  onRowClick,
}: {
  onRowClick?: (payslipId: string) => void;
}) {
  const { t } = useTranslation('payroll');
  const user = useCurrentUser();

  // RBAC gate (client-side defense-in-depth — API is the real gate per INV-4 / PA-2)
  if (user?.role === 'shift_leader' || user?.role === 'agent') {
    return (
      <EmptyState
        variant="no-permission"
        title={t('common.noPermission')}
        description={t('common.noPermissionBody')}
      />
    );
  }

  return <PayslipArchiveInner onRowClick={onRowClick} />;
}

// ---------------------------------------------------------------------------
// Inner component (RBAC already cleared above)
// ---------------------------------------------------------------------------

function PayslipArchiveInner({
  onRowClick,
}: {
  onRowClick?: (payslipId: string) => void;
}) {
  const { t } = useTranslation('payroll');

  const [filters, setFilters] = useState<ArchiveFilters>({
    employeeSearch: '',
    year: String(CURRENT_YEAR),
    month: '',
    status: '',
    cursor: undefined,
  });
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  function patchFilters(patch: Partial<ArchiveFilters>) {
    setFilters((prev) => ({ ...prev, cursor: undefined, ...patch }));
    setPrevCursors([]);
  }

  // Build API params
  const params: ListPayslipsParams = {
    limit: PAGE_SIZE,
    cursor: filters.cursor,
    employee_id: filters.employeeSearch || undefined,
    year: filters.year ? Number(filters.year) : undefined,
    period:
      filters.year && filters.month
        ? `${filters.year}-${filters.month.padStart(2, '0')}`
        : undefined,
    status: (filters.status as PayslipStatus) || undefined,
  };
  // When period is set, year alone is redundant — clear to avoid dual filter ambiguity
  if (params.period) {
    params.year = undefined;
  }

  const query = useListPayslips(params);

  // -------------------------------------------------------------------------
  // Error state
  // -------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <div className="flex flex-col gap-4">
          <ArchiveTitleBand />
          <EmptyState
            variant="no-permission"
            title={t('common.noPermission')}
            description={t('common.noPermissionBody')}
          />
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-4">
        <ArchiveTitleBand />
        <StateView
          kind="error"
          title={t('errors.loadError')}
          onRetry={() => query.refetch()}
          retryLabel={t('common.retry')}
        />
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // Data
  // -------------------------------------------------------------------------

  const body = query.data?.data as PayslipListResponse | undefined;
  const rows: Payslip[] = body?.data ?? [];
  const isMissingHistory = body?.meta?.code === PayslipListResponseMetaCode.MISSING_PAYROLL_HISTORY;

  const activeFilters = hasActiveFilters(filters);

  // -------------------------------------------------------------------------
  // Columns
  // -------------------------------------------------------------------------

  const columns: Column<Payslip>[] = [
    {
      id: 'employee',
      header: t('archive.colEmployee'),
      width: 250,
      cell: (r) => (
        <div className="flex flex-col gap-0.5">
          <span className="font-medium text-text">{r.employee_name ?? r.employee_id}</span>
          <span className="font-mono text-[11px] text-text-3">{r.employee_id}</span>
        </div>
      ),
    },
    {
      id: 'period',
      header: t('archive.colPeriod'),
      width: 120,
      cell: (r) => <span className="text-sm text-text-2">{formatPeriod(r.period)}</span>,
    },
    {
      id: 'paidOn',
      header: t('archive.colPaidOn'),
      width: 120,
      cell: (r) =>
        r.paid_on ? (
          <DateText kind="date" value={r.paid_on} className="text-sm text-text-2" />
        ) : (
          <span className="text-sm text-text-3 italic">{t('common.notAvailable')}</span>
        ),
    },
    {
      id: 'workingDays',
      header: t('archive.colWorkingDays'),
      width: 100,
      cell: (r) => (
        <span className="text-sm text-text-2">{r.working_days != null ? r.working_days : '—'}</span>
      ),
    },
    {
      id: 'grossEarnings',
      header: t('archive.colGrossEarnings'),
      width: 160,
      cell: (r) => (
        <span className="text-sm text-text tabular-nums">{formatMoney(r.gross_earnings)}</span>
      ),
    },
    {
      id: 'takeHome',
      header: t('archive.colTakeHome'),
      width: 150,
      cell: (r) => (
        <span className="text-sm font-medium text-text tabular-nums">
          {formatMoney(r.take_home_pay)}
        </span>
      ),
    },
    {
      id: 'status',
      header: t('archive.colStatus'),
      width: 160,
      cell: (r) => (
        <StatusBadge dot tone={payslipStatusTone(r.status)}>
          {t(payslipStatusKey(r.status))}
        </StatusBadge>
      ),
    },
    {
      id: 'action',
      header: '',
      width: 56,
      cell: (r) => (
        <button
          type="button"
          className="text-sm font-medium text-primary hover:underline"
          onClick={() => onRowClick?.(r.id)}
        >
          {t('archive.viewDetail')}
        </button>
      ),
    },
  ];

  // -------------------------------------------------------------------------
  // Render
  // -------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      <ArchiveTitleBand />

      {/* Confidentiality banner — matches frame `glGDo` ($bad-bg, lock icon) */}
      <div className="flex items-center gap-2 rounded-lg border border-bad-bd bg-bad-bg px-3 py-[9px]">
        <Lock aria-hidden className="size-3.5 shrink-0 text-bad-tx" />
        <p className="text-[12px] font-semibold text-bad-tx">{t('archive.confBanner')}</p>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-2.5">
        {/*
          DEVIATION: API has no free-text search param. The design SearchField
          ("Cari karyawan / NIK") is wired to `employee_id` (exact-match).
          See DEVIATIONS in the module-level JSDoc.
        */}
        <SearchField
          aria-label={t('archive.searchExactAriaLabel')}
          placeholder={t('archive.searchExactPlaceholder')}
          defaultValue={filters.employeeSearch}
          containerClassName="w-64"
          onChange={(e) => patchFilters({ employeeSearch: e.target.value })}
        />
        <FilterSelect
          aria-label={t('archive.filterYear')}
          value={filters.year}
          onChange={(e) => patchFilters({ year: e.target.value, month: '' })}
        >
          <option value="">{t('archive.filterYear')}</option>
          {YEAR_OPTIONS.map((y) => (
            <option key={y} value={String(y)}>
              {y}
            </option>
          ))}
        </FilterSelect>
        <FilterSelect
          aria-label={t('archive.filterMonth')}
          value={filters.month}
          onChange={(e) => patchFilters({ month: e.target.value })}
        >
          <option value="">{t('archive.filterMonth')}</option>
          {MONTH_OPTIONS.map((m) => (
            <option key={m} value={String(m)}>
              {t(`month.${m}`)}
            </option>
          ))}
        </FilterSelect>
        <FilterSelect
          aria-label={t('archive.filterStatus')}
          value={filters.status}
          onChange={(e) => patchFilters({ status: e.target.value })}
        >
          <option value="">{t('archive.filterStatus')}</option>
          {Object.values(PayslipStatus).map((s) => (
            <option key={s} value={s}>
              {t(payslipStatusKey(s))}
            </option>
          ))}
        </FilterSelect>

        {activeFilters && (
          <>
            <div className="h-6 w-px bg-border" />
            <Button
              type="button"
              variant="ghost"
              onClick={() =>
                patchFilters({
                  employeeSearch: '',
                  year: String(CURRENT_YEAR),
                  month: '',
                  status: '',
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
        aria-label={t('archive.tableAriaLabel')}
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        isLoading={query.isLoading}
        skeletonRows={8}
        empty={
          isMissingHistory ? (
            <EmptyState
              variant="fresh"
              title={t('archive.noHistoryTitle')}
              description={t('archive.noHistoryBody')}
            />
          ) : activeFilters ? (
            <EmptyState
              variant="filtered"
              title={t('archive.filteredTitle')}
              description={t('archive.filteredBody')}
            />
          ) : (
            <EmptyState
              variant="fresh"
              title={t('archive.emptyTitle')}
              description={t('archive.emptyBody')}
            />
          )
        }
        footer={
          rows.length > 0 ? (
            <CursorPagination
              rangeLabel={t('common.resultRange', { count: rows.length })}
              hasPrev={prevCursors.length > 0}
              hasNext={Boolean(body?.has_more)}
              prevLabel={t('common.prev')}
              nextLabel={t('common.next')}
              onPrev={() => {
                const next = [...prevCursors];
                const cursor = next.pop();
                setPrevCursors(next);
                setFilters((prev) => ({ ...prev, cursor: cursor || undefined }));
              }}
              onNext={() => {
                const nextCursor = body?.next_cursor;
                if (!nextCursor) return;
                setPrevCursors((prev) => [...prev, filters.cursor ?? '']);
                setFilters((prev) => ({ ...prev, cursor: nextCursor }));
              }}
            />
          ) : undefined
        }
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// ArchiveTitleBand
// ---------------------------------------------------------------------------

function ArchiveTitleBand() {
  const { t } = useTranslation('payroll');
  return (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="font-bold text-3xl text-text">{t('archive.title')}</h1>
        <p className="text-sm text-text-3">{t('archive.subtitle')}</p>
      </div>
    </div>
  );
}
