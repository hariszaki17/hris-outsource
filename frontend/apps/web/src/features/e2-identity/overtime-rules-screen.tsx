import { classifyError } from '@/lib/api-error.ts';
import {
  type ListOvertimeRules200,
  type ListOvertimeRulesParams,
  type OvertimeRule,
  OvertimeRuleStatus,
  useListOvertimeRules,
} from '@swp/api-client/e2';
import { type Column, DataTable, EmptyState, FilterSelect, StateView, StatusBadge } from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { ArrowLeft, Info, Timer } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * E2 · Aturan Lembur — read-only list.
 * Built from .pen frame `SnXpE` (list). Seeded rules rarely change, so this screen
 * is view-only: no create/edit/deactivate entry-points (decided 2026-06-09).
 * Overtime rules are GLOBAL ONLY (service-line scope dropped 2026-06-12). OR-3: rates.
 * Routes: /master-data/overtime-rules — consumers must register in the router.
 * Refs: F7.1 (E7 OT calc consumes this master).
 */

const PAGE_SIZE = 200;

// ---------------------------------------------------------------------------
// OvertimeRulesScreen
// ---------------------------------------------------------------------------

export function OvertimeRulesScreen() {
  const { t } = useTranslation();

  const [statusFilter, setStatusFilter] = useState<OvertimeRuleStatus | undefined>(undefined);

  const params: ListOvertimeRulesParams = {
    limit: PAGE_SIZE,
    status: statusFilter,
  };

  const query = useListOvertimeRules(params);

  const hasFilters = Boolean(statusFilter);

  const columns: Column<OvertimeRule>[] = [
    {
      id: 'name',
      header: t('masterData.overtimeRules.colName'),
      width: 260,
      cell: (row) => (
        <div className="flex items-center gap-3">
          <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-warn-bg">
            <Timer className="size-4 text-warn-tx" aria-hidden />
          </div>
          <div className="flex flex-col gap-[2px]">
            <span className="text-[14px] font-medium text-text">{row.name}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'weekday_rate',
      header: t('masterData.overtimeRules.colWeekday'),
      width: 110,
      cell: (row) => (
        <span className="font-mono text-[13px] font-medium text-text">{row.weekday_rate}×</span>
      ),
    },
    {
      id: 'restday_rate',
      header: t('masterData.overtimeRules.colRestday'),
      width: 110,
      cell: (row) => (
        <span className="font-mono text-[13px] font-medium text-text">{row.restday_rate}×</span>
      ),
    },
    {
      id: 'holiday_rate',
      header: t('masterData.overtimeRules.colHoliday'),
      width: 110,
      cell: (row) => (
        <span className="font-mono text-[13px] font-medium text-text">{row.holiday_rate}×</span>
      ),
    },
    {
      id: 'min_minutes',
      header: t('masterData.overtimeRules.colMinMinutes'),
      width: 110,
      cell: (row) => (
        <span className="font-mono text-[13px] font-medium text-text">
          {row.min_minutes} {t('masterData.overtimeRules.minutes')}
        </span>
      ),
    },
    {
      id: 'max_minutes_per_day',
      header: t('masterData.overtimeRules.colMaxMinutes'),
      width: 140,
      cell: (row) =>
        row.max_minutes_per_day != null ? (
          <span className="font-mono text-[13px] font-medium text-text">
            {row.max_minutes_per_day} {t('masterData.overtimeRules.minutes')}
          </span>
        ) : (
          <span className="text-text-3">—</span>
        ),
    },
    {
      id: 'status',
      header: t('masterData.overtimeRules.colStatus'),
      width: 110,
      cell: (row) => (
        <StatusBadge dot tone={row.status === OvertimeRuleStatus.ACTIVE ? 'ok' : 'bad'}>
          {row.status === OvertimeRuleStatus.ACTIVE
            ? t('masterData.statusActive')
            : t('masterData.statusInactive')}
        </StatusBadge>
      ),
    },
  ];

  if (query.isError) {
    const { kind } = classifyError(query.error);
    return (
      <div className="flex flex-col gap-4">
        {kind === 'forbidden' || kind === 'unauthenticated' ? (
          <EmptyState
            variant="no-permission"
            title={t('errors.forbidden')}
            description={t('masterData.noPermission')}
          />
        ) : (
          <StateView
            kind="error"
            title={t('masterData.overtimeRules.errorTitle')}
            description={t('errors.network')}
            onRetry={() => query.refetch()}
            retryLabel={t('common.retry')}
          />
        )}
      </div>
    );
  }

  const page = query.data?.data as ListOvertimeRules200 | undefined;
  const rows = (page?.data ?? []) as OvertimeRule[];

  return (
    <div className="flex flex-col gap-4">
      {/* Back link — /master-data route registered when router is wired (RETURN §1) */}
      <Link
        to="/master-data"
        className="flex w-fit items-center gap-[7px] text-[13px] font-medium text-text-2 hover:text-text"
      >
        <ArrowLeft className="size-4" aria-hidden />
        {t('masterData.backToHub')}
      </Link>

      {/* Title band */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-[20px] font-semibold text-text">
            {t('masterData.overtimeRules.title')}
          </h1>
          <p className="text-[13px] text-text-2">{t('masterData.overtimeRules.subtitle')}</p>
        </div>
      </div>

      {/* Shared-with-E7 note */}
      <div className="flex items-center gap-[9px] rounded-lg border border-info-bd bg-info-bg px-[14px] py-[10px]">
        <Info className="size-[15px] shrink-0 text-info-tx" aria-hidden />
        <p className="text-[12px] text-info-tx">{t('masterData.overtimeRules.sharedNote')}</p>
      </div>

      {/* Table card */}
      <div className="overflow-hidden rounded-xl border border-border bg-surface">
        <div className="border-b border-border-soft px-[18px] py-[14px]">
          <FilterSelect
            aria-label={t('masterData.filterStatus')}
            value={statusFilter ?? ''}
            onChange={(e) => setStatusFilter((e.target.value as OvertimeRuleStatus) || undefined)}
          >
            <option value="">{t('masterData.filterAllStatus')}</option>
            <option value={OvertimeRuleStatus.ACTIVE}>{t('masterData.statusActive')}</option>
            <option value={OvertimeRuleStatus.INACTIVE}>{t('masterData.statusInactive')}</option>
          </FilterSelect>
        </div>

        <DataTable
          aria-label={t('masterData.overtimeRules.title')}
          columns={columns}
          data={rows}
          getRowId={(r) => r.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          empty={
            hasFilters ? (
              <EmptyState
                variant="filtered"
                title={t('masterData.filteredTitle')}
                description={t('masterData.filteredBody')}
              />
            ) : (
              <EmptyState
                variant="fresh"
                title={t('masterData.overtimeRules.emptyTitle')}
                description={t('masterData.overtimeRules.seedHint')}
              />
            )
          }
        />
      </div>
    </div>
  );
}
