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
import { useCompanyOptions } from '@/lib/use-company-options.ts';
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
  Button,
  type Column,
  CursorPagination,
  DataTable,
  EmptyState,
  SearchField,
  StatCard,
  StateView,
  StatusBadge,
} from '@swp/ui';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { CircleCheck, UserCheck, UserMinus, UserPlus, UserX, Users } from 'lucide-react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { OffboardEmployeeConfirm, ReactivateEmployeeConfirm } from './employee-overlays.tsx';

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
  /** Status-tab shortcut: 'all' | 'active' | 'inactive' */
  tab?: 'all' | 'active' | 'inactive';
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

  // Company filter options — HR/super_admin pick freely; SL is server-pinned to one company
  // (NAVIGATION-AND-RBAC §4.2), so skip the fetch and lock the control below.
  const { options: companyOptions } = useCompanyOptions({ enabled: !isShiftLeader });

  const [prevCursors, setPrevCursors] = useState<string[]>([]);
  // Debounce the free-text search so we navigate once the user pauses, not per keystroke.
  const searchDebounce = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Inline lifecycle action (offboard / reactivate) target + modal visibility.
  const [lifecycleTarget, setLifecycleTarget] = useState<Employee | null>(null);
  const [showOffboard, setShowOffboard] = useState(false);
  const [showReactivate, setShowReactivate] = useState(false);

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

  // SCOPING: a shift_leader is pinned to their own company server-side; pin the param too so the
  // client query mirrors the server scope (mirror attendance-dashboard-screen.tsx). HR/super_admin
  // pick a company freely via the filter dropdown.
  const slCompanyId = isShiftLeader ? (currentUser?.companyId ?? undefined) : undefined;

  const params: ListEmployeesParams = {
    limit: PAGE_SIZE,
    q: search.q || undefined,
    status: tabStatus ?? search.status,
    client_company: isShiftLeader ? slCompanyId : search.client_company || undefined,
    cursor: search.cursor,
  };

  const query = useListEmployees(params);

  // SL company pin is implicit scope, not a user-set filter → exclude it from hasFilters.
  const hasFilters = Boolean(
    search.q || search.status || (!isShiftLeader && search.client_company),
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
      flex: 2,
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
    // POSISI / LINI LAYANAN / PENEMPATAN come from the employee's current (non-terminal)
    // placement, resolved by the list query's LATERAL join. "—" = genuinely unplaced.
    {
      id: 'posisi',
      header: t('colPosisi'),
      flex: 1.5,
      cell: (emp) => (
        <span className="text-[13px] text-text">{emp.current_position?.name ?? '—'}</span>
      ),
    },
    {
      id: 'liniLayanan',
      header: t('colLiniLayanan'),
      flex: 1.5,
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
      flex: 1.5,
      cell: (emp) => (
        <span className="text-[13px] text-text">{emp.current_client_company?.name ?? '—'}</span>
      ),
    },
    {
      id: 'status',
      header: t('colStatus'),
      flex: 0.6,
      cell: (emp) => (
        <StatusBadge dot tone={statusTone[emp.status]}>
          {emp.status === EmployeeStatus.ACTIVE ? t('statusActive') : t('statusInactive')}
        </StatusBadge>
      ),
    },
  ];

  // Row click opens the detail screen (all roles); fuller management lives there.
  // The kebab was removed — its View/Edit both just navigated to detail (redundant).
  // A single inline lifecycle action remains (offboard active / reactivate inactive),
  // the one action not reachable by row-click. Non-SL only (SL is read-only).
  if (!isShiftLeader) {
    columns.push({
      id: 'actions',
      header: '',
      flex: 0.6,
      cell: (emp) => {
        const active = emp.status === EmployeeStatus.ACTIVE;
        const label = active ? t('menuOffboard') : t('menuReactivate');
        return (
          <div className="flex justify-end">
            <Button
              variant={active ? 'destructive' : 'secondary'}
              size="sm"
              onClick={(e) => {
                e.stopPropagation();
                setLifecycleTarget(emp);
                if (active) setShowOffboard(true);
                else setShowReactivate(true);
              }}
            >
              {active ? (
                <UserMinus className="size-3.5" aria-hidden />
              ) : (
                <UserCheck className="size-3.5" aria-hidden />
              )}
              {label}
            </Button>
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
      <div className="grid grid-cols-3 gap-4">
        {/* Counts are derived from the current cursor page only (no server total field on the
            page envelope) — labelled "on this page" so they are never read as org-wide totals. */}
        <StatCard
          label={t('statTotal')}
          value={query.isLoading ? '—' : String(rows.length)}
          sub={t('statThisPage')}
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
          sub={t('statThisPage')}
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
          sub={t('statThisPage')}
          icon={UserX}
          tone="bad"
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
            containerClassName="w-[300px]"
            onChange={(e) => {
              const v = e.target.value;
              if (searchDebounce.current) clearTimeout(searchDebounce.current);
              searchDebounce.current = setTimeout(() => setSearch({ q: v || undefined }), 300);
            }}
          />
          {/* Company filter — locked (disabled) for shift_leader; populated dropdown for HR. */}
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
                <title>lock</title>
                <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                <path d="M7 11V7a5 5 0 0 1 10 0v4" />
              </svg>
              {currentUser?.companyName ?? currentUser?.companyId ?? t('filterCompany')}
            </span>
          ) : (
            <select
              aria-label={t('filterCompany')}
              className="rounded-lg border border-border bg-surface px-[10px] py-[9px] text-[13px] text-text-2 outline-none"
              value={search.client_company ?? ''}
              onChange={(e) => setSearch({ client_company: e.target.value || undefined })}
            >
              <option value="">{t('filterCompanyAll')}</option>
              {companyOptions.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          )}
          {/* Status filtering is the tabs above (Semua / Aktif / Nonaktif) — no separate dropdown. */}
        </div>

        {/* Data table */}
        <DataTable
          aria-label={t('title')}
          columns={columns}
          data={rows}
          getRowId={(e) => e.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          onRowClick={(emp) =>
            void navigate({
              to: '/employees/$employeeId' as const,
              params: { employeeId: emp.id },
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

      {/* Inline lifecycle modals (offboard / reactivate) — target set by the row icon */}
      <OffboardEmployeeConfirm
        open={showOffboard}
        onOpenChange={setShowOffboard}
        employee={lifecycleTarget}
        onDone={() => {
          setShowOffboard(false);
          void query.refetch();
        }}
      />
      <ReactivateEmployeeConfirm
        open={showReactivate}
        onOpenChange={setShowReactivate}
        employee={lifecycleTarget}
        onDone={() => {
          setShowReactivate(false);
          void query.refetch();
        }}
      />
    </div>
  );
}
