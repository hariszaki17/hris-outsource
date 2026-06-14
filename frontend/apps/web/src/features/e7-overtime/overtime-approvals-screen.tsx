/**
 * E7 · Persetujuan Lembur — approval queue (view over the E11 engine).
 *
 * Approval routing moved to the E11 configurable engine (EPICS §8 E11, 2026-06-14). This queue
 * is now a VIEW over the same pending records the E11 inbox surfaces (IB-5): it lists PENDING
 * overtime and routes each row to overtime-detail, where the E11 approval chain + approve/reject
 * actions live. The deleted per-domain hooks (useApproveOvertimeL1/Final, useRejectOvertime,
 * useBulkApproveOvertime/useBulkRejectOvertime) are gone.
 *
 * NOTE: bulk approve/reject is DROPPED in E11 v1 — there is no bulk approval endpoint on the
 * engine. Re-introduce the multi-select UI once an E11 bulk op exists.
 *
 * validateSearch fields: q, company_id, source, cursor
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type ListOvertimeParams,
  type Overtime,
  OvertimeSource,
  OvertimeStatus,
  useListOvertime,
} from '@swp/api-client/e7';
import {
  Avatar,
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
import { Eye, RotateCcw } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  formatOtMinutes,
  overtimeSourceKey,
  overtimeSourceTone,
  overtimeTierKey,
} from './overtime-shared.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type OvertimeApprovalsSearch = {
  q?: string;
  company_id?: string;
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

function hasActiveFilters(s: OvertimeApprovalsSearch): boolean {
  return Boolean(s.q || s.company_id || s.source);
}

// ---------------------------------------------------------------------------
// Inner component
// ---------------------------------------------------------------------------

interface OvertimeApprovalsScreenProps {
  search: OvertimeApprovalsSearch;
  onSearch: (patch: OvertimeApprovalsSearch) => void;
  prevCursors: string[];
  onPrevCursorsChange: (c: string[]) => void;
}

function OvertimeApprovalsScreenInner({
  search,
  onSearch,
  prevCursors,
  onPrevCursorsChange,
}: OvertimeApprovalsScreenProps) {
  const { t } = useTranslation('overtime');
  const user = useCurrentUser();

  const isHR = user?.role === 'hr_admin' || user?.role === 'super_admin';

  // E11: approval sub-states collapsed into a single PENDING.
  const params: ListOvertimeParams = {
    limit: PAGE_SIZE,
    cursor: search.cursor,
    status: OvertimeStatus.PENDING,
    company_id: search.company_id || undefined,
    source: search.source || undefined,
  };

  const query = useListOvertime(params);
  const hasFilters = hasActiveFilters(search);

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  function setSearch(patch: OvertimeApprovalsSearch) {
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
          <ApprovalsTitle isHR={isHR} />
          <EmptyState
            variant="no-permission"
            title={t('errors.forbidden')}
            description={t('approvals.noPermissionBody')}
          />
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-[18px]">
        <ApprovalsTitle isHR={isHR} />
        <StateView
          kind="error"
          title={t('approvals.errorTitle')}
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

  // Total counted minutes in the visible queue
  const queueTotalMinutes = rows.reduce((acc, r) => acc + r.calculation.counted_minutes, 0);

  // ---------------------------------------------------------------------------
  // Columns
  // ---------------------------------------------------------------------------

  const columns: Column<Overtime>[] = [
    {
      id: 'agent',
      header: t('approvals.colAgent'),
      width: 260,
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
            size={34}
          />
          <div className="flex flex-col gap-0.5">
            <span className="text-sm font-semibold text-text">
              {r.employee.name ?? r.employee.id}
            </span>
            <span className="text-[12px] text-text-3">{r.company.name ?? r.company.id}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'overtime',
      header: t('approvals.colOvertime'),
      width: 280,
      cell: (r) => (
        <div className="flex flex-col gap-0.5">
          <div className="flex items-center gap-1.5">
            <span className="font-mono text-sm font-semibold text-text">
              <DateText kind="date" value={r.work_date} />
              {' · '}
              {formatOtMinutes(r.calculation.counted_minutes)}
            </span>
          </div>
          <div className="flex items-center gap-1.5">
            <span className="inline-flex items-center rounded-full bg-surface-2 px-2 py-0.5 text-[11px] font-semibold text-text-2">
              {t(overtimeTierKey(r.tier_indicator))}
            </span>
            {r.flagged_no_preapproval && (
              <StatusBadge dot tone="warn">
                {t('approvals.flagNoPreapproval')}
              </StatusBadge>
            )}
          </div>
        </div>
      ),
    },
    {
      id: 'source',
      header: t('approvals.colSource'),
      width: 160,
      cell: (r) => (
        <StatusBadge dot tone={overtimeSourceTone(r.source)}>
          {t(overtimeSourceKey(r.source))}
        </StatusBadge>
      ),
    },
    {
      id: 'actions',
      header: '',
      width: 160,
      cell: (r) => (
        <div className="flex items-center justify-end">
          {/* Approve/reject happen on the detail screen (E11 chain). */}
          <a
            href={`/overtime/${r.id}`}
            className="inline-flex items-center gap-1.5 rounded-[7px] border border-border bg-surface px-2.5 py-1.5 text-[12px] font-semibold text-text-2 hover:bg-surface-2"
          >
            <Eye aria-hidden className="size-[13px]" />
            {t('approvals.review')}
          </a>
        </div>
      ),
    },
  ];

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      <ApprovalsTitle isHR={isHR} />

      {/* Filters — frame: Search · Semua perusahaan (HR only) · Semua sumber */}
      <div className="flex flex-wrap items-center gap-2.5">
        <SearchField
          placeholder={t('approvals.searchPlaceholder')}
          defaultValue={search.q ?? ''}
          containerClassName="w-64"
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
        />

        {isHR && (
          <FilterSelect
            aria-label={t('approvals.filterCompany')}
            value={search.company_id ?? ''}
            onChange={(e) => setSearch({ company_id: e.target.value || undefined })}
          >
            <option value="">{t('approvals.filterCompany')}</option>
          </FilterSelect>
        )}

        <FilterSelect
          aria-label={t('approvals.filterSource')}
          value={search.source ?? ''}
          onChange={(e) => setSearch({ source: (e.target.value as OvertimeSource) || undefined })}
        >
          <option value="">{t('approvals.filterSource')}</option>
          {Object.values(OvertimeSource).map((src) => (
            <option key={src} value={src}>
              {t(overtimeSourceKey(src))}
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
        aria-label={t('approvals.tableAriaLabel')}
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        isLoading={query.isLoading}
        skeletonRows={8}
        empty={
          hasFilters ? (
            <EmptyState
              variant="filtered"
              title={t('approvals.filteredTitle')}
              description={t('approvals.filteredBody')}
            />
          ) : (
            <EmptyState
              variant="fresh"
              title={t('approvals.emptyTitle')}
              description={t('approvals.emptyBody')}
            />
          )
        }
        footer={
          rows.length > 0 ? (
            <div className="flex items-center justify-between px-[18px] py-3">
              <span className="text-[13px] text-text-3">
                {isHR
                  ? t('approvals.queueSummaryHR', {
                      count: rows.length,
                      total: formatOtMinutes(queueTotalMinutes),
                    })
                  : t('approvals.queueSummarySL', {
                      count: rows.length,
                      total: formatOtMinutes(queueTotalMinutes),
                    })}
              </span>
              <CursorPagination
                rangeLabel={t('approvals.resultRange', { count: rows.length })}
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
            </div>
          ) : undefined
        }
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public export
// ---------------------------------------------------------------------------

export function OvertimeApprovalsScreen() {
  const [search, setSearch] = useState<OvertimeApprovalsSearch>({});
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  return (
    <OvertimeApprovalsScreenInner
      search={search}
      onSearch={(patch) => setSearch(patch)}
      prevCursors={prevCursors}
      onPrevCursorsChange={setPrevCursors}
    />
  );
}

// ---------------------------------------------------------------------------
// TitleBand
// ---------------------------------------------------------------------------

function ApprovalsTitle({ isHR }: { isHR: boolean }) {
  const { t } = useTranslation('overtime');
  return (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-bold text-text">{t('approvals.title')}</h1>
        <p className="text-sm text-text-3">
          {isHR ? t('approvals.subtitleHR') : t('approvals.subtitleSL')}
        </p>
      </div>
    </div>
  );
}
