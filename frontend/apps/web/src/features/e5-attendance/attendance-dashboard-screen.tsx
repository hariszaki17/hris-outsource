/**
 * E5 · Kehadiran — Dashboard (HR + Shift Leader scoped)
 *
 * .pen frames:
 *   sZCLW  HR/Admin dashboard — Kehadiran Dashboard
 *   V2QL7  SL scoped dashboard — E5 SL · Team Attendance — Plaza Senayan
 *
 * Design: TitleBand → 4× StatCards → TableCard (Tabs, FilterRow, DataTable, Pagination).
 * HR: cross-company. SL: own-company scope, locked company filter + ScopeBanner.
 * Columns: Karyawan (name+ID) | Perusahaan | Lini Layanan | Masuk | Status | Verifikasi | Aksi.
 *
 * ENGINEERING.md D1 — typed URL search params + cursor pagination.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type Attendance,
  type AttendancePage,
  AttendanceStatus,
  type ListAttendanceParams,
  VerificationStatus,
  useListAttendance,
} from '@swp/api-client/e5';
import type { StatusTone } from '@swp/design-tokens';
import {
  type Column,
  CursorPagination,
  DataTable,
  EmptyState,
  FilterSelect,
  SearchField,
  StatCard,
  StatusBadge,
} from '@swp/ui';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { CircleCheck, ClockAlert, TriangleAlert, Users } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

export type AttendanceDashboardSearch = {
  q?: string;
  status?: AttendanceStatus;
  tab?: 'all' | 'present' | 'late' | 'absent';
  cursor?: string;
};

function attendanceStatusTone(status: AttendanceStatus): StatusTone {
  switch (status) {
    case AttendanceStatus.PRESENT:
      return 'ok';
    case AttendanceStatus.LATE:
      return 'warn';
    case AttendanceStatus.ABSENT:
      return 'bad';
    case AttendanceStatus.INCOMPLETE:
      return 'warn';
    case AttendanceStatus.ON_LEAVE:
      return 'info';
    default:
      return 'neutral';
  }
}

function verificationStatusTone(vs: VerificationStatus): StatusTone {
  switch (vs) {
    case VerificationStatus.VERIFIED:
    case VerificationStatus.AUTO_APPROVED:
      return 'ok';
    case VerificationStatus.PENDING:
    case VerificationStatus.ESCALATED:
      return 'warn';
    case VerificationStatus.REJECTED:
      return 'bad';
    default:
      return 'neutral';
  }
}

// ---------------------------------------------------------------------------
// Tab strip
// ---------------------------------------------------------------------------

interface TabItem {
  id: string;
  label: string;
  active: boolean;
  onClick: () => void;
}

function StatusTabs({ tabs }: { tabs: TabItem[] }) {
  return (
    <div className="flex items-center gap-[26px] px-[18px] pt-[6px] border-b border-border">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          onClick={tab.onClick}
          className={[
            'flex items-center gap-[7px] pb-[14px] pt-[14px] text-[14px] font-medium',
            tab.active
              ? 'border-b-2 border-primary text-primary font-semibold'
              : 'border-b-2 border-transparent text-text-2',
          ].join(' ')}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// AttendanceDashboardScreen
// ---------------------------------------------------------------------------

export function AttendanceDashboardScreen() {
  const { t } = useTranslation('attendance');
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as AttendanceDashboardSearch;
  const currentUser = useCurrentUser();
  const isShiftLeader = currentUser?.role === 'shift_leader';

  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  const activeTab = search.tab ?? 'all';

  const tabStatusMap: Record<string, AttendanceStatus | undefined> = {
    all: undefined,
    present: AttendanceStatus.PRESENT,
    late: AttendanceStatus.LATE,
    absent: AttendanceStatus.ABSENT,
  };

  const queryParams: ListAttendanceParams = {
    limit: PAGE_SIZE,
    cursor: search.cursor,
    q: search.q,
    status: search.status
      ? [search.status]
      : tabStatusMap[activeTab]
        ? [tabStatusMap[activeTab] as AttendanceStatus]
        : undefined,
  };

  const query = useListAttendance(queryParams);
  const page = query.data?.data as AttendancePage | undefined;
  const rows: Attendance[] = page?.data ?? [];

  const hasFilters = Boolean(search.q || search.status);

  function setSearch(partial: Partial<AttendanceDashboardSearch>) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    void (
      navigate as (o: {
        to: string;
        search?: Record<string, unknown>;
        params?: Record<string, unknown>;
      }) => void
    )({
      to: '/attendance',
      search: { ...search, ...partial, cursor: undefined },
    });
  }

  // Stat counts
  const presentCount = rows.filter((r) => r.status === AttendanceStatus.PRESENT).length;
  const lateCount = rows.filter((r) => r.status === AttendanceStatus.LATE).length;
  const pendingCount = rows.filter(
    (r) =>
      r.verification_status === VerificationStatus.PENDING ||
      r.verification_status === VerificationStatus.ESCALATED,
  ).length;

  // Tabs
  const tabs: TabItem[] = [
    {
      id: 'all',
      label: t('tabAll'),
      active: activeTab === 'all',
      onClick: () => setSearch({ tab: 'all' }),
    },
    {
      id: 'present',
      label: t('tabPresent'),
      active: activeTab === 'present',
      onClick: () => setSearch({ tab: 'present' }),
    },
    {
      id: 'late',
      label: t('tabLate'),
      active: activeTab === 'late',
      onClick: () => setSearch({ tab: 'late' }),
    },
    {
      id: 'absent',
      label: t('tabAbsent'),
      active: activeTab === 'absent',
      onClick: () => setSearch({ tab: 'absent' }),
    },
  ];

  // Columns
  const columns: Column<Attendance>[] = [
    {
      id: 'employee',
      header: t('colEmployee'),
      width: 220,
      cell: (row) => (
        <div className="flex flex-col gap-[2px]">
          <span className="text-[13px] font-medium text-text">
            {row.employee_name ?? row.employee_id}
          </span>
          <span className="font-mono text-[11px] text-text-3">{row.employee_id}</span>
        </div>
      ),
    },
    {
      id: 'company',
      header: t('colCompany'),
      width: 180,
      cell: (row) => (
        <span className="text-[13px] text-text">{row.company_name ?? row.company_id}</span>
      ),
    },
    {
      id: 'service_line',
      header: t('colServiceLine'),
      width: 160,
      cell: (row) => <span className="text-[13px] text-text">{row.service_line}</span>,
    },
    {
      id: 'check_in_at',
      header: t('colCheckIn'),
      width: 130,
      cell: (row) => (
        <span className="text-[13px] text-text">
          {new Date(row.check_in_at).toLocaleTimeString('id-ID', {
            timeZone: 'Asia/Jakarta',
            hour: '2-digit',
            minute: '2-digit',
          })}
        </span>
      ),
    },
    {
      id: 'status',
      header: t('colStatus'),
      width: 120,
      cell: (row) => (
        <StatusBadge dot tone={attendanceStatusTone(row.status)}>
          {t(`status.${row.status}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'verification_status',
      header: t('colVerification'),
      width: 140,
      cell: (row) => (
        <StatusBadge dot tone={verificationStatusTone(row.verification_status)}>
          {t(`verificationStatus.${row.verification_status}`)}
        </StatusBadge>
      ),
    },
  ];

  // Error state
  if (query.isError) {
    const err = classifyError(query.error);
    if (err.kind === 'forbidden') {
      return (
        <div className="flex flex-col gap-[18px]">
          <div className="flex items-center gap-2 rounded-xl border border-border bg-surface px-5 py-[18px]">
            <span className="text-[14px] text-text-2">{t('noPermission')}</span>
          </div>
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-[18px]">
        <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
          <span className="text-[14px] text-bad">{t('loadError')}</span>
          <button
            type="button"
            className="rounded-lg border border-border bg-surface px-4 py-2 text-[13px] font-medium text-text-2 hover:bg-surface-2"
            onClick={() => query.refetch()}
          >
            {t('retry')}
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[18px]">
      {/* SL scope banner */}
      {isShiftLeader && (
        <div className="flex items-center gap-2 bg-warn-bg px-6 py-[10px] border-b border-warn-bd">
          <span className="text-warn-tx" aria-hidden>
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              aria-hidden="true"
            >
              <title>lock</title>
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
          </span>
          <p className="text-[12px] font-semibold text-warn-tx">{t('scopeBanner')}</p>
        </div>
      )}

      {/* Title band */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl font-bold text-text">{t('dashTitle')}</h1>
          <p className="text-[13px] text-text-2">{t('dashSubtitle')}</p>
        </div>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard
          label={t('statTotalAgents')}
          value={query.isLoading ? '—' : String(rows.length)}
          sub={t('statTotalAgentsSub')}
          icon={Users}
          tone="brand"
        />
        <StatCard
          label={t('statPresent')}
          value={query.isLoading ? '—' : String(presentCount)}
          sub={t('statPresentSub')}
          icon={CircleCheck}
          tone="ok"
        />
        <StatCard
          label={t('statLate')}
          value={query.isLoading ? '—' : String(lateCount)}
          sub={t('statLateSub')}
          icon={ClockAlert}
          tone="warn"
        />
        <StatCard
          label={t('statPending')}
          value={query.isLoading ? '—' : String(pendingCount)}
          sub={t('statPendingSub')}
          icon={TriangleAlert}
          tone="bad"
        />
      </div>

      {/* Table card */}
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        <StatusTabs tabs={tabs} />

        {/* Filter row */}
        <div className="flex items-center gap-[10px] border-b border-border-soft px-[18px] py-[14px]">
          <SearchField
            placeholder={t('searchPlaceholder')}
            defaultValue={search.q ?? ''}
            containerClassName="w-[260px]"
            onChange={(e) => setSearch({ q: e.target.value || undefined })}
          />
          <FilterSelect
            aria-label={t('filterStatus')}
            value={search.status ?? ''}
            onChange={(e) =>
              setSearch({ status: (e.target.value as AttendanceStatus) || undefined })
            }
          >
            <option value="">{t('filterStatusAll')}</option>
            <option value={AttendanceStatus.PRESENT}>
              {t(`status.${AttendanceStatus.PRESENT}`)}
            </option>
            <option value={AttendanceStatus.LATE}>{t(`status.${AttendanceStatus.LATE}`)}</option>
            <option value={AttendanceStatus.ABSENT}>
              {t(`status.${AttendanceStatus.ABSENT}`)}
            </option>
            <option value={AttendanceStatus.INCOMPLETE}>
              {t(`status.${AttendanceStatus.INCOMPLETE}`)}
            </option>
            <option value={AttendanceStatus.ON_LEAVE}>
              {t(`status.${AttendanceStatus.ON_LEAVE}`)}
            </option>
          </FilterSelect>
          <div className="flex-1" />
          {hasFilters && (
            <button
              type="button"
              className="flex items-center gap-2 rounded-lg border border-border bg-surface px-[14px] py-[9px] text-[13px] font-medium text-text-2 hover:bg-surface-2"
              onClick={() => setSearch({ q: undefined, status: undefined })}
            >
              {t('resetFilters')}
            </button>
          )}
        </div>

        {/* Data table */}
        <DataTable
          aria-label={t('dashTitle')}
          columns={columns}
          data={rows}
          getRowId={(r) => r.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          onRowClick={(rec) =>
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            void (
              navigate as (o: {
                to: string;
                search?: Record<string, unknown>;
                params?: Record<string, unknown>;
              }) => void
            )({
              to: '/attendance/$attendanceId',
              params: { attendanceId: rec.id },
            })
          }
          empty={
            hasFilters ? (
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
                  // eslint-disable-next-line @typescript-eslint/no-explicit-any
                  void (
                    navigate as (o: {
                      to: string;
                      search?: Record<string, unknown>;
                      params?: Record<string, unknown>;
                    }) => void
                  )({
                    to: '/attendance',
                    search: { ...search, cursor: cursor || undefined },
                  });
                }}
                onNext={() => {
                  const nextCursor = page?.next_cursor;
                  if (!nextCursor) return;
                  setPrevCursors((s) => [...s, search.cursor ?? '']);
                  // eslint-disable-next-line @typescript-eslint/no-explicit-any
                  void (
                    navigate as (o: {
                      to: string;
                      search?: Record<string, unknown>;
                      params?: Record<string, unknown>;
                    }) => void
                  )({
                    to: '/attendance',
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
