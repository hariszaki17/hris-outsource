/**
 * E6 · Persetujuan Cuti — Approval queue list.
 *
 * .pen frames implemented:
 *   yho5i  E6 · Persetujuan Cuti (HR L2)  — HR sees cross-company, PENDING_HR rows
 *   qb0S0  E6 SL · Persetujuan Cuti (L1)  — Shift Leader sees own-company, PENDING_L1 rows
 *
 * Routes:
 *   /leave             → HR approval queue (role = hr_admin)
 *   /leave             → SL approval queue (role = shift_leader, scoped company)
 *
 * validateSearch fields: q, status, company_id, leave_type_id, cursor
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  LeaveTypeStatus,
  type ListLeaveTypes200,
  useListClientCompanies,
  useListLeaveTypes,
} from '@swp/api-client/e2';
import {
  type LeaveRequest,
  LeaveStatus,
  type ListLeaveRequestsParams,
  useListLeaveRequests,
} from '@swp/api-client/e6';
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
import { Link } from '@tanstack/react-router';
// Link not used until route is registered; use plain anchor for now to avoid route type mismatch
import { Download, RotateCcw } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { leaveStatusTone } from './leave-overlays.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type LeaveApprovalsSearch = {
  q?: string;
  status?: LeaveStatus;
  company_id?: string;
  leave_type_id?: string;
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function hasActiveFilters(s: LeaveApprovalsSearch): boolean {
  return Boolean(s.q || s.status || s.company_id || s.leave_type_id);
}

// ---------------------------------------------------------------------------
// Inner component
// ---------------------------------------------------------------------------

interface LeaveApprovalsScreenProps {
  search: LeaveApprovalsSearch;
  onSearch: (patch: LeaveApprovalsSearch) => void;
  prevCursors: string[];
  onPrevCursorsChange: (c: string[]) => void;
}

function LeaveApprovalsScreenInner({
  search,
  onSearch,
  prevCursors,
  onPrevCursorsChange,
}: LeaveApprovalsScreenProps) {
  const { t } = useTranslation('leave');
  const user = useCurrentUser();

  const isHR = user?.role === 'hr_admin' || user?.role === 'super_admin';
  const isSL = user?.role === 'shift_leader';
  void isSL; // scoping used in column render (employee_company_name)

  // Default status filter: HR sees PENDING_HR, SL sees PENDING_L1
  const defaultStatus = isHR ? LeaveStatus.PENDING_HR : LeaveStatus.PENDING_L1;

  const params: ListLeaveRequestsParams = {
    limit: PAGE_SIZE,
    cursor: search.cursor,
    q: search.q,
    status: search.status ?? defaultStatus,
    company_id: search.company_id || undefined,
    leave_type_id: search.leave_type_id || undefined,
  };

  const query = useListLeaveRequests(params);

  // Company filter options — HR/admin only (SL is locked to their own company).
  const companiesQuery = useListClientCompanies(
    { limit: 200 },
    { query: { enabled: isHR, staleTime: 60_000 } },
  );
  const companyOptions = useMemo(() => {
    if (!isHR) return [];
    const cc =
      (companiesQuery.data?.data as { data?: { id: string; name: string }[] } | undefined)?.data ??
      [];
    return cc.map((c) => ({ value: c.id, label: c.name }));
  }, [isHR, companiesQuery.data]);

  // Leave type filter options — active types only.
  const leaveTypesQuery = useListLeaveTypes(
    { limit: 200, status: LeaveTypeStatus.ACTIVE },
    { query: { staleTime: 5 * 60_000 } },
  );
  const leaveTypeOptions = useMemo(() => {
    const lt =
      (leaveTypesQuery.data?.data as ListLeaveTypes200 | undefined)?.data ??
      ([] as { id: string; name: string }[]);
    return lt.map((l) => ({ value: l.id, label: l.name }));
  }, [leaveTypesQuery.data]);

  const hasFilters = hasActiveFilters(search);

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  function setSearch(patch: LeaveApprovalsSearch) {
    onSearch({ ...search, cursor: undefined, ...patch });
    onPrevCursorsChange([]);
  }

  // ---------------------------------------------------------------------------
  // Error state
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <div className="flex flex-col gap-[18px]">
          <TitleBand isHR={isHR} />
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
        <TitleBand isHR={isHR} />
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

  const page = query.data?.data;
  const rows: LeaveRequest[] = (page as { data?: LeaveRequest[] })?.data ?? [];

  // ---------------------------------------------------------------------------
  // Columns
  // ---------------------------------------------------------------------------

  const columns: Column<LeaveRequest>[] = [
    {
      id: 'agent',
      header: t('approvals.colAgent'),
      width: 200,
      cell: (r) => (
        <div className="flex flex-col">
          <span className="font-medium text-text">{r.employee_name ?? r.employee_id}</span>
          {isHR && r.employee_company_name && (
            <span className="text-xs text-text-3">{r.employee_company_name}</span>
          )}
        </div>
      ),
    },
    {
      id: 'leaveType',
      header: t('approvals.colType'),
      width: 160,
      cell: (r) => (
        <span className="text-sm text-text-2">{r.leave_type_name ?? r.leave_type_id}</span>
      ),
    },
    {
      id: 'dates',
      header: t('approvals.colDates'),
      width: 200,
      cell: (r) => (
        <div className="flex flex-col text-sm text-text-2">
          <span>
            <DateText kind="date" value={r.start_date} /> –{' '}
            <DateText kind="date" value={r.end_date} />
          </span>
          <span className="text-xs text-text-3">
            {r.duration_days} {t('common.days')}
          </span>
        </div>
      ),
    },
    {
      id: 'submitted',
      header: t('approvals.colSubmitted'),
      width: 170,
      cell: (r) => <DateText kind="instant" value={r.created_at} className="text-sm text-text-2" />,
    },
    {
      id: 'status',
      header: t('approvals.colStatus'),
      width: 150,
      cell: (r) => (
        <StatusBadge dot tone={leaveStatusTone(r.status)}>
          {t(`status.${r.status}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'action',
      header: '',
      width: 100,
      cell: (r) => (
        <Link
          to="/leave/$leaveRequestId"
          params={{ leaveRequestId: r.id }}
          className="text-sm font-medium text-primary hover:underline"
        >
          {t('approvals.review')}
        </Link>
      ),
    },
  ];

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      <TitleBand isHR={isHR} />

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-2.5">
        <SearchField
          placeholder={t('approvals.searchPlaceholder')}
          defaultValue={search.q ?? ''}
          containerClassName="w-64"
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
        />
        <FilterSelect
          aria-label={t('approvals.filterStatus')}
          value={search.status ?? ''}
          onChange={(e) => setSearch({ status: (e.target.value as LeaveStatus) || undefined })}
        >
          <option value="">{t('approvals.filterStatus')}</option>
          {Object.values(LeaveStatus).map((s) => (
            <option key={s} value={s}>
              {t(`status.${s}`)}
            </option>
          ))}
        </FilterSelect>
        {isHR && (
          <FilterSelect
            aria-label={t('approvals.filterCompany')}
            value={search.company_id ?? ''}
            onChange={(e) => setSearch({ company_id: e.target.value || undefined })}
          >
            <option value="">{t('approvals.filterCompany')}</option>
            {companyOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </FilterSelect>
        )}
        <FilterSelect
          aria-label={t('approvals.filterType')}
          value={search.leave_type_id ?? ''}
          onChange={(e) => setSearch({ leave_type_id: e.target.value || undefined })}
        >
          <option value="">{t('approvals.filterType')}</option>
          {leaveTypeOptions.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
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
                  status: undefined,
                  company_id: undefined,
                  leave_type_id: undefined,
                  cursor: undefined,
                })
              }
            >
              <RotateCcw aria-hidden className="size-3.5" />
              {t('approvals.resetFilters')}
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
            <CursorPagination
              rangeLabel={t('approvals.resultRange', { count: rows.length })}
              hasPrev={prevCursors.length > 0}
              hasNext={Boolean((page as { has_more?: boolean })?.has_more)}
              prevLabel={t('common.prev')}
              nextLabel={t('common.next')}
              onPrev={() => {
                const next = [...prevCursors];
                const cursor = next.pop();
                onPrevCursorsChange(next);
                onSearch({ ...search, cursor: cursor || undefined });
              }}
              onNext={() => {
                const nextCursor = (page as { next_cursor?: string })?.next_cursor;
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

export function LeaveApprovalsScreen() {
  const [search, setSearch] = useState<LeaveApprovalsSearch>({});
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  return (
    <LeaveApprovalsScreenInner
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

function TitleBand({ isHR }: { isHR: boolean }) {
  const { t } = useTranslation('leave');
  return (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="font-bold text-3xl text-text">{t('approvals.title')}</h1>
        <p className="text-sm text-text-3">
          {isHR ? t('approvals.subtitleHR') : t('approvals.subtitleSL')}
        </p>
      </div>
      <Button
        type="button"
        variant="secondary"
        disabled
        title={t('approvals.exportComingSoon')}
        aria-label={t('approvals.exportComingSoon')}
      >
        <Download aria-hidden className="size-4" />
        {t('approvals.export')}
      </Button>
    </div>
  );
}
