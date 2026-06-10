/**
 * E2 · Perusahaan Klien — Daftar (Client Companies list)
 *
 * .pen frame: qIpsj — E2 · Perusahaan Klien — Daftar
 *
 * Layout: TitleBand → 4× StatCards → TableCard (FilterRow, THead, rows, pagination).
 * Columns: Perusahaan (icon+name+alamat) | Lini Layanan | Pemimpin Shift | Penempatan |
 * Geofence | Status | inline action.
 * Row action: inline Deactivate / Reactivate (opens ConfirmDialog). Edit is on the detail page.
 *
 * ENGINEERING.md D1 — typed URL search params + cursor pagination.
 * F2.3 — Client Company directory. CC-5 — active-placement guard on deactivate.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type ClientCompany,
  ClientCompanyStatus,
  type ListClientCompaniesParams,
  useDeactivateClientCompany,
  useListClientCompanies,
  useReactivateClientCompany,
} from '@swp/api-client/e2';
import type { StatusTone } from '@swp/design-tokens';
import {
  type Column,
  ConfirmDialog,
  CursorPagination,
  DataTable,
  EmptyState,
  StatCard,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { Link, useNavigate, useSearch } from '@tanstack/react-router';
import { Ban, Building2, CircleCheck, MapPin, PowerOff, RotateCcw } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

export type ClientCompaniesSearch = {
  q?: string;
  status?: ClientCompanyStatus;
  /**
   * Reserved: parsed by the route's validateSearch but NOT applied — the directory PRD
   * (client-company-directory.md) specifies only search + active/inactive status filtering,
   * and there is no service-line options source on this screen. Kept for URL/route-type
   * compatibility; intentionally not wired into the query or `hasFilters`.
   */
  service_line?: string;
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Main screen
// ---------------------------------------------------------------------------

export function ClientCompaniesScreen() {
  const { t } = useTranslation('clientCompanies');
  const navigate = useNavigate();
  // useSearch needs the route registered; cast to avoid TS path errors since routes will be added
  const search = useSearch({ strict: false }) as ClientCompaniesSearch;
  const { toast } = useToast();
  const currentUser = useCurrentUser();
  // Defense-in-depth (ENGINEERING.md A2): the directory + its write actions are HR/super_admin
  // only (clients.read/clients.write). A shift_leader has neither, so hide write affordances —
  // the API is the real gate, but we never paint a button that would 403 (no dead-flow).
  const canWrite = currentUser?.role === 'hr_admin' || currentUser?.role === 'super_admin';

  const [deactivateTarget, setDeactivateTarget] = useState<ClientCompany | null>(null);
  const [reactivateTarget, setReactivateTarget] = useState<ClientCompany | null>(null);

  const params: ListClientCompaniesParams = {
    limit: PAGE_SIZE,
    ...(search.q ? { q: search.q } : {}),
    ...(search.status ? { status: search.status } : {}),
    ...(search.cursor ? { cursor: search.cursor } : {}),
  };

  const query = useListClientCompanies(params);
  const responseData = query.data?.data;
  // responseData is ListClientCompanies200 | UnauthenticatedResponse | ForbiddenResponse
  const listData = responseData && 'data' in responseData ? responseData : null;
  const companies: ClientCompany[] = listData?.data ?? [];
  const nextCursor = listData?.next_cursor ?? null;
  const hasMore = listData?.has_more ?? false;

  const deactivateMut = useDeactivateClientCompany();
  const reactivateMut = useReactivateClientCompany();

  const activeCount = companies.filter((c) => c.status === ClientCompanyStatus.ACTIVE).length;
  const totalSites = companies.reduce((n, c) => n + (c.site_count ?? 0), 0);
  const inactiveCount = companies.filter((c) => c.status === ClientCompanyStatus.INACTIVE).length;

  function setFilter(patch: Partial<ClientCompaniesSearch>) {
    navigate({
      to: '.',
      search: (prev: Record<string, unknown>) => ({ ...prev, ...patch, cursor: undefined }),
      replace: true,
    } as never);
  }

  function handleDeactivateConfirm() {
    if (!deactivateTarget) return;
    deactivateMut.mutate(
      { clientCompanyId: deactivateTarget.id, data: {} },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('toast.deactivated') });
          void query.refetch();
          setDeactivateTarget(null);
        },
        onError: (err) => {
          const { kind, message } = classifyError(err);
          if (kind === 'conflict') {
            toast({
              tone: 'warn',
              title: t('toast.deactivateConflict'),
              description: t('toast.deactivateConflictDesc'),
            });
          } else {
            toast({ tone: 'error', title: t('toast.deactivateFailed'), description: message });
          }
          setDeactivateTarget(null);
        },
      },
    );
  }

  function handleReactivateConfirm() {
    if (!reactivateTarget) return;
    reactivateMut.mutate(
      { clientCompanyId: reactivateTarget.id },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('toast.reactivated') });
          void query.refetch();
          setReactivateTarget(null);
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t('toast.reactivateFailed'), description: message });
          setReactivateTarget(null);
        },
      },
    );
  }

  // Table columns (matches .pen THead: Perusahaan | Lini Layanan | Pemimpin Shift | Penempatan | Geofence | Status | kebab)
  const columns: Column<ClientCompany>[] = [
    {
      id: 'name',
      header: t('table.company'),
      flex: 2,
      cell: (row) => (
        <div className="flex items-center gap-3">
          <div className="flex items-center justify-center w-9 h-9 rounded-lg bg-surface-2 shrink-0">
            <Building2 size={18} className="text-text-2" aria-hidden />
          </div>
          <div className="flex flex-col gap-0.5 min-w-0">
            <Link
              to={'/client-companies/$clientCompanyId' as never}
              params={{ clientCompanyId: row.id } as never}
              className="text-[14px] font-semibold text-text hover:text-primary truncate"
            >
              {row.name}
            </Link>
            <span className="text-[11px] text-text-2 truncate">{row.address}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'has_leader',
      header: t('table.shiftLeader'),
      flex: 1,
      cell: (row) => (
        <span className="text-[13px] text-text">
          {row.has_leader ? (
            t('table.assigned')
          ) : (
            <span className="text-text-3 italic">{t('table.unassigned')}</span>
          )}
        </span>
      ),
    },
    {
      id: 'active_placement_count',
      header: t('table.placements'),
      flex: 0.8,
      cell: (row) => (
        <span className="text-[13px] text-text">
          {row.active_placement_count != null
            ? t('table.agentCount', { count: row.active_placement_count })
            : '—'}
        </span>
      ),
    },
    {
      id: 'sites',
      header: t('table.sites'),
      flex: 0.8,
      cell: (row) => (
        <div className="flex items-center gap-[6px]">
          <MapPin size={14} className="text-text-3 shrink-0" aria-hidden />
          <span className="text-[12px] text-text">
            {t('table.siteCount', { count: row.site_count ?? 0 })}
          </span>
        </div>
      ),
    },
    {
      id: 'status',
      header: t('table.status'),
      flex: 0.6,
      cell: (row) => {
        const tone: StatusTone = row.status === ClientCompanyStatus.ACTIVE ? 'ok' : 'bad';
        return (
          <StatusBadge tone={tone} dot>
            {row.status === ClientCompanyStatus.ACTIVE ? t('status.active') : t('status.inactive')}
          </StatusBadge>
        );
      },
    },
  ];

  // Deactivate/Reactivate are hr_admin/super_admin only — append the actions column only when
  // the role can write, so read-only roles never get a row action that would 403.
  if (canWrite) {
    columns.push({
      id: 'actions',
      header: '',
      align: 'center',
      flex: 0.6,
      cell: (row) =>
        row.status === ClientCompanyStatus.ACTIVE ? (
          <button
            type="button"
            className="rounded-md px-3 py-1.5 text-[13px] font-medium bg-destructive text-destructive-foreground hover:opacity-90"
            onClick={() => setDeactivateTarget(row)}
          >
            {t('actions.deactivate')}
          </button>
        ) : (
          <button
            type="button"
            className="rounded-md px-3 py-1.5 text-[13px] font-medium text-ok-tx hover:bg-ok-bg"
            onClick={() => setReactivateTarget(row)}
          >
            {t('actions.reactivate')}
          </button>
        ),
    });
  }

  const hasFilters = !!(search.q || search.status);

  return (
    <div className="flex flex-col gap-[18px] p-6 bg-app-bg min-h-full overflow-y-auto">
      {/* Title band */}
      <div className="flex items-center justify-between rounded-xl bg-surface border border-border px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-[18px] font-semibold text-text">{t('title')}</h1>
          <p className="text-[13px] text-text-2">{t('subtitle')}</p>
        </div>
        {canWrite && (
          <Link to={'/client-companies/new' as never}>
            <button
              type="button"
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-white text-[14px] font-medium hover:bg-primary-strong"
            >
              {t('actions.addCompany')}
            </button>
          </Link>
        )}
      </div>

      {/* Stat cards */}
      <div className="flex gap-4">
        <StatCard
          className="flex-1"
          icon={Building2}
          tone="neutral"
          label={t('stats.total')}
          value={companies.length}
          sub={t('stats.totalDesc')}
        />
        <StatCard
          className="flex-1"
          icon={CircleCheck}
          tone="ok"
          label={t('stats.active')}
          value={activeCount}
          sub={t('stats.activeDesc')}
        />
        <StatCard
          className="flex-1"
          icon={MapPin}
          tone="info"
          label={t('stats.sites')}
          value={`${totalSites}`}
          sub={t('stats.sitesDesc')}
        />
        <StatCard
          className="flex-1"
          icon={Ban}
          tone="bad"
          label={t('stats.inactive')}
          value={inactiveCount}
          sub={t('stats.inactiveDesc')}
        />
      </div>

      {/* Table card */}
      <div className="flex flex-col rounded-xl bg-surface border border-border overflow-hidden">
        {/* Filter row */}
        <div className="flex items-center gap-[10px] px-[18px] py-[14px] border-b border-border-soft">
          <div className="flex w-[280px] items-center gap-2 rounded-md border border-border bg-surface px-3 py-2">
            <input
              type="search"
              aria-label={t('filter.searchLabel')}
              className="flex-1 border-0 bg-transparent p-0 text-[13px] text-text outline-none placeholder:text-text-3"
              placeholder={t('filter.searchPlaceholder')}
              value={search.q ?? ''}
              onChange={(e) => setFilter({ q: e.target.value || undefined })}
            />
          </div>
          <div className="relative w-44 rounded-md border border-border bg-surface px-3 py-2">
            <select
              aria-label={t('filter.statusLabel')}
              className="w-full appearance-none border-0 bg-transparent pr-6 text-[13px] font-medium text-text-2 outline-none"
              value={search.status ?? ''}
              onChange={(e) =>
                setFilter({ status: (e.target.value as ClientCompanyStatus) || undefined })
              }
            >
              <option value="">{t('filter.allStatus')}</option>
              <option value={ClientCompanyStatus.ACTIVE}>{t('status.active')}</option>
              <option value={ClientCompanyStatus.INACTIVE}>{t('status.inactive')}</option>
            </select>
          </div>
        </div>

        {/* Data table */}
        {query.isPending ? (
          <DataTable<ClientCompany>
            aria-label={t('title')}
            columns={columns}
            data={[]}
            getRowId={(row) => row.id}
            isLoading
          />
        ) : query.isError ? (
          (() => {
            const { kind, message } = classifyError(query.error);
            // A role lacking clients.read (e.g. shift_leader deep-linking) gets a 403 here —
            // render a no-permission state with no Retry, mirroring employees-screen.tsx.
            return kind === 'forbidden' || kind === 'unauthenticated' ? (
              <div className="p-8">
                <EmptyState
                  variant="no-permission"
                  title={t('state.noPermissionTitle')}
                  description={t('state.noPermissionBody')}
                />
              </div>
            ) : (
              <div className="p-8">
                <StateView
                  kind="error"
                  title={t('state.errorTitle')}
                  description={message}
                  onRetry={() => void query.refetch()}
                />
              </div>
            );
          })()
        ) : companies.length === 0 ? (
          <div className="p-8">
            <EmptyState
              variant={hasFilters ? 'filtered' : 'fresh'}
              title={hasFilters ? t('empty.filteredTitle') : t('empty.freshTitle')}
              description={hasFilters ? t('empty.filteredDesc') : t('empty.freshDesc')}
            />
          </div>
        ) : (
          <DataTable<ClientCompany>
            aria-label={t('title')}
            columns={columns}
            data={companies}
            getRowId={(row) => row.id}
          />
        )}

        {/* Pagination */}
        {(hasMore || search.cursor) && (
          <div className="px-4 py-3 border-t border-border-soft">
            <CursorPagination
              rangeLabel=""
              hasNext={hasMore}
              hasPrev={!!search.cursor}
              onNext={() => nextCursor && setFilter({ cursor: nextCursor })}
              onPrev={() => setFilter({ cursor: undefined })}
            />
          </div>
        )}
      </div>

      {/* Deactivate confirm */}
      <ConfirmDialog
        open={!!deactivateTarget}
        onOpenChange={(o) => {
          if (!o) setDeactivateTarget(null);
        }}
        icon={PowerOff}
        tone="danger"
        title={t('confirm.deactivateTitle')}
        description={
          deactivateTarget?.active_placement_count
            ? t('confirm.deactivateWithPlacements', {
                count: deactivateTarget.active_placement_count,
              })
            : t('confirm.deactivateDesc')
        }
        confirmLabel={t('confirm.deactivateBtn')}
        cancelLabel={t('confirm.cancel')}
        onConfirm={handleDeactivateConfirm}
        loading={deactivateMut.isPending}
      />

      {/* Reactivate confirm */}
      <ConfirmDialog
        open={!!reactivateTarget}
        onOpenChange={(o) => {
          if (!o) setReactivateTarget(null);
        }}
        icon={RotateCcw}
        tone="brand"
        title={t('confirm.reactivateTitle')}
        description={t('confirm.reactivateDesc')}
        confirmLabel={t('confirm.reactivateBtn')}
        cancelLabel={t('confirm.cancel')}
        onConfirm={handleReactivateConfirm}
        loading={reactivateMut.isPending}
      />
    </div>
  );
}
