/**
 * E2 · Karyawan — Daftar (HR/Admin + Shift Leader scoped list)
 *
 * .pen frames:
 *   WElYh  HR/Admin list — Karyawan Daftar
 *   n3wi1w SL scoped list — same screen, read-only, scope note different
 *
 * Design: TitleBand → 4× StatCards → RoleNote banner → TableCard (Tabs, FilterRow, DataTable,
 * Pagination). Columns: Karyawan (avatar+name+NIK) | Posisi | Lini Layanan | Penempatan |
 * Login | Status | kebab.
 *
 * ENGINEERING.md D1 — typed URL search params + cursor pagination.
 * ENGINEERING.md A2 — role gate is defense-in-depth; API is the real gate.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type Employee,
  EmployeeStatus,
  type ListEmployees200,
  type ListEmployeesParams,
  useListEmployees,
} from '@swp/api-client/e2';
import type { StatusTone } from '@swp/design-tokens';
import {
  Avatar,
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
import { useNavigate, useSearch } from '@tanstack/react-router';
import { CircleCheck, KeyRound, MoreVertical, UserPlus, UserX, Users } from 'lucide-react';
import type * as React from 'react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { EmployeeRowActionsMenu } from './employee-overlays.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

/** Typed filter/cursor search params for `/employees`. */
export type EmployeesSearch = {
  q?: string;
  status?: EmployeeStatus;
  service_line?: string;
  client_company?: string;
  has_login?: boolean;
  /** Status-tab shortcut: 'all' | 'active' | 'inactive' | 'no-login' */
  tab?: 'all' | 'active' | 'inactive' | 'no-login';
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
// Inline tab component (no @swp/ui Tabs primitive yet)
// ---------------------------------------------------------------------------

interface TabItem {
  id: string;
  label: string;
  count?: number;
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
          <span>{tab.label}</span>
          {tab.count !== undefined && (
            <span
              className={[
                'rounded-full px-[7px] py-[2px] text-[11px] font-semibold',
                tab.active ? 'bg-primary-soft text-primary' : 'bg-app-bg text-text-2',
              ].join(' ')}
            >
              {tab.count}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// EmployeesScreen
// ---------------------------------------------------------------------------

export function EmployeesScreen() {
  const { t } = useTranslation('employees');
  const navigate = useNavigate();
  const search = useSearch({ from: '/authed/employees' as const });
  const currentUser = useCurrentUser();
  const isShiftLeader = currentUser?.role === 'shift_leader';

  const [prevCursors, setPrevCursors] = useState<string[]>([]);
  const [openMenuEmployeeId, setOpenMenuEmployeeId] = useState<string | null>(null);
  const kebabRefs = useRef<Map<string, HTMLButtonElement>>(new Map());

  // ---------------------------------------------------------------------------
  // Search params → API params
  // ---------------------------------------------------------------------------

  const activeTab = search.tab ?? 'all';

  const tabStatus: EmployeeStatus | undefined =
    activeTab === 'active'
      ? EmployeeStatus.ACTIVE
      : activeTab === 'inactive'
        ? EmployeeStatus.INACTIVE
        : undefined;

  const tabHasLogin: boolean | undefined = activeTab === 'no-login' ? false : undefined;

  const params: ListEmployeesParams = {
    limit: PAGE_SIZE,
    q: search.q || undefined,
    status: tabStatus ?? search.status,
    service_line: search.service_line || undefined,
    client_company: search.client_company || undefined,
    has_login: tabHasLogin ?? search.has_login,
    cursor: search.cursor,
  };

  const query = useListEmployees(params);

  const hasFilters = Boolean(
    search.q || search.status || search.service_line || search.client_company || search.has_login,
  );

  // Totals for stat cards (from page envelope, not real aggregates — use count from loaded page)
  const page = query.data?.data as ListEmployees200 | undefined;
  const rows = (page?.data ?? []) as Employee[];

  // ---------------------------------------------------------------------------
  // Navigation helpers
  // ---------------------------------------------------------------------------

  function setSearch(patch: EmployeesSearch) {
    const next: EmployeesSearch = { ...search, cursor: undefined, ...patch };
    void navigate({
      to: '/employees' as const,
      search: next,
    });
    setPrevCursors([]);
  }

  // ---------------------------------------------------------------------------
  // Columns
  // ---------------------------------------------------------------------------

  const statusTone: Record<EmployeeStatus, StatusTone> = {
    ACTIVE: 'ok',
    INACTIVE: 'bad',
  };

  const columns: Column<Employee>[] = [
    {
      id: 'karyawan',
      header: t('colKaryawan'),
      width: 280,
      cell: (emp) => (
        <div className="flex items-center gap-[11px]">
          <Avatar initials={initials(emp.full_name)} size={34} />
          <div className="flex flex-col gap-[2px]">
            <span className="text-[14px] font-semibold text-text">{emp.full_name}</span>
            <span className="font-mono text-[11px] text-text-3">NIK {emp.nik}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'posisi',
      header: t('colPosisi'),
      width: 180,
      cell: (emp) => (
        <span className="text-[13px] text-text">{emp.current_position?.name ?? '—'}</span>
      ),
    },
    {
      id: 'liniLayanan',
      header: t('colLiniLayanan'),
      width: 170,
      cell: (emp) =>
        emp.current_service_line ? (
          <div className="flex items-center gap-[7px]">
            <span className="size-[8px] rounded-full bg-info-tx shrink-0" aria-hidden />
            <span className="text-[13px] text-text-2">{emp.current_service_line.name}</span>
          </div>
        ) : (
          <span className="text-[13px] text-text-3">—</span>
        ),
    },
    {
      id: 'penempatan',
      header: t('colPenempatan'),
      width: 220,
      cell: (emp) => (
        <span className="text-[13px] text-text">{emp.current_client_company?.name ?? '—'}</span>
      ),
    },
    {
      id: 'login',
      header: t('colLogin'),
      width: 130,
      cell: (emp) =>
        emp.has_login ? (
          <StatusBadge dot tone="info">
            {t('loginActive')}
          </StatusBadge>
        ) : (
          <StatusBadge dot tone="neutral">
            {t('loginNone')}
          </StatusBadge>
        ),
    },
    {
      id: 'status',
      header: t('colStatus'),
      width: 120,
      cell: (emp) => (
        <StatusBadge dot tone={statusTone[emp.status]}>
          {emp.status === EmployeeStatus.ACTIVE ? t('statusActive') : t('statusInactive')}
        </StatusBadge>
      ),
    },
  ];

  // Kebab column only for non-SL roles
  if (!isShiftLeader) {
    columns.push({
      id: 'actions',
      header: '',
      width: 52,
      cell: (emp) => {
        const isOpen = openMenuEmployeeId === emp.id;
        const setRef = (el: HTMLButtonElement | null) => {
          if (el) kebabRefs.current.set(emp.id, el);
          else kebabRefs.current.delete(emp.id);
        };
        const anchorRef: React.RefObject<HTMLElement | null> = {
          get current() {
            return kebabRefs.current.get(emp.id) ?? null;
          },
        };
        return (
          <div className="relative flex justify-center">
            <button
              ref={setRef}
              type="button"
              aria-label={t('rowActions')}
              aria-expanded={isOpen}
              aria-haspopup="menu"
              className="flex size-[30px] items-center justify-center rounded-[7px] text-text-3 hover:bg-surface-2"
              onClick={() => setOpenMenuEmployeeId(isOpen ? null : emp.id)}
            >
              <MoreVertical className="size-4" aria-hidden />
            </button>
            <EmployeeRowActionsMenu
              employee={emp}
              open={isOpen}
              anchorRef={anchorRef}
              onClose={() => setOpenMenuEmployeeId(null)}
              onView={() => {
                void navigate({
                  to: '/employees/$employeeId' as const,
                  params: { employeeId: emp.id },
                });
              }}
              onEdit={() => {
                // edit opens detail screen which opens the drawer
                void navigate({
                  to: '/employees/$employeeId' as const,
                  params: { employeeId: emp.id },
                });
              }}
              onToggleStatus={() => setOpenMenuEmployeeId(null)}
            />
          </div>
        );
      },
    });
  }

  // ---------------------------------------------------------------------------
  // Role note text
  // ---------------------------------------------------------------------------

  const roleNoteText = isShiftLeader ? t('roleNoteSL') : t('roleNoteHR');

  // ---------------------------------------------------------------------------
  // Error state
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    return (
      <div className="flex flex-col gap-[18px]">
        <div className="flex items-start justify-between">
          <h1 className="text-3xl font-bold text-text">{t('title')}</h1>
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
  // Tab definitions
  // ---------------------------------------------------------------------------

  const tabs: TabItem[] = [
    {
      id: 'all',
      label: t('tabAll'),
      active: activeTab === 'all',
      onClick: () => setSearch({ tab: 'all' }),
    },
    {
      id: 'active',
      label: t('tabActive'),
      active: activeTab === 'active',
      onClick: () => setSearch({ tab: 'active' }),
    },
    {
      id: 'inactive',
      label: t('tabInactive'),
      active: activeTab === 'inactive',
      onClick: () => setSearch({ tab: 'inactive' }),
    },
    {
      id: 'no-login',
      label: t('tabNoLogin'),
      active: activeTab === 'no-login',
      onClick: () => setSearch({ tab: 'no-login' }),
    },
  ];

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl font-bold text-text">{t('title')}</h1>
          <p className="text-[13px] text-text-2">{t('subtitle')}</p>
        </div>
        {!isShiftLeader && (
          <button
            type="button"
            className="flex items-center gap-2 rounded-lg bg-primary px-4 py-[10px] text-[14px] font-semibold text-white hover:bg-primary/90"
            onClick={() => void navigate({ to: '/employees/new' as const })}
          >
            <UserPlus className="size-4" aria-hidden />
            {t('add')}
          </button>
        )}
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard
          label={t('statTotal')}
          value={query.isLoading ? '—' : String(rows.length)}
          sub={t('statTotalSub')}
          icon={Users}
          tone="brand"
        />
        <StatCard
          label={t('statActive')}
          value={
            query.isLoading
              ? '—'
              : String(rows.filter((e) => e.status === EmployeeStatus.ACTIVE).length)
          }
          sub={t('statActiveSub')}
          icon={CircleCheck}
          tone="ok"
        />
        <StatCard
          label={t('statInactive')}
          value={
            query.isLoading
              ? '—'
              : String(rows.filter((e) => e.status === EmployeeStatus.INACTIVE).length)
          }
          sub={t('statInactiveSub')}
          icon={UserX}
          tone="bad"
        />
        <StatCard
          label={t('statNoLogin')}
          value={query.isLoading ? '—' : String(rows.filter((e) => !e.has_login).length)}
          sub={t('statNoLoginSub')}
          icon={KeyRound}
          tone="warn"
        />
      </div>

      {/* Role note */}
      <div className="flex items-center gap-[9px] rounded-lg border border-l-[3px] border-border bg-surface px-[14px] py-[10px]">
        <span className="shrink-0 text-text-3" aria-hidden>
          <svg width="15" height="15" viewBox="0 0 15 15" fill="none" aria-hidden="true">
            <title>info</title>
            <circle cx="7.5" cy="7.5" r="7" stroke="currentColor" strokeWidth="1" />
            <text x="7.5" y="11" textAnchor="middle" fontSize="9" fill="currentColor">
              i
            </text>
          </svg>
        </span>
        <p className="text-[12px] text-text-2">{roleNoteText}</p>
      </div>

      {/* Table card */}
      <div
        className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface"
        style={{ height: 'fill_container' }}
      >
        {/* Tabs */}
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
            onChange={(e) => setSearch({ status: (e.target.value as EmployeeStatus) || undefined })}
          >
            <option value="">{t('filterStatusAll')}</option>
            <option value={EmployeeStatus.ACTIVE}>{t('statusActive')}</option>
            <option value={EmployeeStatus.INACTIVE}>{t('statusInactive')}</option>
          </FilterSelect>
          <div className="flex-1" />
          <button
            type="button"
            className="flex items-center gap-2 rounded-lg border border-border bg-surface px-[14px] py-[9px] text-[13px] font-medium text-text-2 hover:bg-surface-2"
            onClick={() =>
              setSearch({
                q: undefined,
                status: undefined,
                service_line: undefined,
                client_company: undefined,
                has_login: undefined,
              })
            }
          >
            {t('resetFilters')}
          </button>
        </div>

        {/* Data table */}
        <DataTable
          aria-label={t('title')}
          columns={columns}
          data={rows}
          getRowId={(e) => e.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          onRowClick={
            isShiftLeader
              ? (emp) =>
                  void navigate({
                    to: '/employees/$employeeId' as const,
                    params: { employeeId: emp.id },
                  })
              : undefined
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
                  void navigate({
                    to: '/employees' as const,
                    search: { ...search, cursor: cursor || undefined },
                  });
                }}
                onNext={() => {
                  const nextCursor = page?.next_cursor;
                  if (!nextCursor) return;
                  setPrevCursors((s) => [...s, search.cursor ?? '']);
                  void navigate({
                    to: '/employees' as const,
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
