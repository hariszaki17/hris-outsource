/**
 * E2 · Perusahaan Klien — Daftar (Client Companies list)
 *
 * .pen frame: qIpsj — E2 · Perusahaan Klien — Daftar
 *
 * Layout: TitleBand → 4× StatCards → TableCard (FilterRow, THead, rows, pagination).
 * Columns: Perusahaan (icon+name+alamat) | Lini Layanan | Pemimpin Shift | Penempatan |
 * Geofence | Status | kebab actions.
 * Row actions: Edit (opens EditClientCompanyDrawer), Deactivate / Reactivate.
 *
 * ENGINEERING.md D1 — typed URL search params + cursor pagination.
 * F2.3 — Client Company directory. CC-5 — active-placement guard on deactivate.
 */

import { classifyError } from '@/lib/api-error.ts';
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
import {
  Ban,
  Building2,
  CircleCheck,
  MapPin,
  MoreVertical,
  PowerOff,
  RotateCcw,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { EditClientCompanyDrawer } from './client-company-form.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

export type ClientCompaniesSearch = {
  q?: string;
  status?: ClientCompanyStatus;
  service_line?: string;
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Row actions menu
// ---------------------------------------------------------------------------

interface RowActionsMenuProps {
  company: ClientCompany;
  onEdit: (company: ClientCompany) => void;
  onDeactivate: (company: ClientCompany) => void;
  onReactivate: (company: ClientCompany) => void;
}

function RowActionsMenu({ company, onEdit, onDeactivate, onReactivate }: RowActionsMenuProps) {
  const [open, setOpen] = useState(false);
  const { t } = useTranslation('clientCompanies');

  return (
    <div className="relative">
      <button
        type="button"
        aria-label={t('actions.openMenu')}
        onClick={() => setOpen((p) => !p)}
        className="flex items-center justify-center w-8 h-8 rounded-md hover:bg-surface-2 text-text-2"
      >
        <MoreVertical size={18} aria-hidden />
      </button>
      {open && (
        <div className="absolute right-0 top-9 z-20 min-w-[160px] rounded-lg border border-border bg-surface shadow-md py-1">
          <button
            type="button"
            className="w-full text-left px-4 py-2 text-sm text-text hover:bg-surface-2"
            onClick={() => {
              setOpen(false);
              onEdit(company);
            }}
          >
            {t('actions.edit')}
          </button>
          {company.status === ClientCompanyStatus.ACTIVE ? (
            <button
              type="button"
              className="w-full text-left px-4 py-2 text-sm text-bad-tx hover:bg-bad-bg"
              onClick={() => {
                setOpen(false);
                onDeactivate(company);
              }}
            >
              {t('actions.deactivate')}
            </button>
          ) : (
            <button
              type="button"
              className="w-full text-left px-4 py-2 text-sm text-ok-tx hover:bg-ok-bg"
              onClick={() => {
                setOpen(false);
                onReactivate(company);
              }}
            >
              {t('actions.reactivate')}
            </button>
          )}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main screen
// ---------------------------------------------------------------------------

export function ClientCompaniesScreen() {
  const { t } = useTranslation('clientCompanies');
  const navigate = useNavigate();
  // useSearch needs the route registered; cast to avoid TS path errors since routes will be added
  const search = useSearch({ strict: false }) as ClientCompaniesSearch;
  const { toast } = useToast();

  const [editCompany, setEditCompany] = useState<ClientCompany | null>(null);
  const [deactivateTarget, setDeactivateTarget] = useState<ClientCompany | null>(null);
  const [reactivateTarget, setReactivateTarget] = useState<ClientCompany | null>(null);

  const params: ListClientCompaniesParams = {
    limit: PAGE_SIZE,
    ...(search.q ? { q: search.q } : {}),
    ...(search.status ? { status: search.status } : {}),
    ...(search.service_line ? { service_line: search.service_line } : {}),
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
  const geofenceCount = companies.filter((c) => c.geofence_active).length;
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
      width: 320,
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
      id: 'service_line',
      header: t('table.serviceLine'),
      width: 170,
      cell: () => <span className="text-[13px] text-text-3">—</span>,
    },
    {
      id: 'has_leader',
      header: t('table.shiftLeader'),
      width: 200,
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
      width: 130,
      cell: (row) => (
        <span className="text-[13px] text-text">
          {row.active_placement_count != null
            ? t('table.agentCount', { count: row.active_placement_count })
            : '—'}
        </span>
      ),
    },
    {
      id: 'geofence_radius_m',
      header: t('table.geofence'),
      width: 130,
      cell: (row) =>
        row.geofence_active ? (
          <div className="flex items-center gap-[6px]">
            <MapPin size={14} className="text-ok-tx shrink-0" aria-hidden />
            <span className="text-[12px] font-mono text-text">{row.geofence_radius_m}m</span>
          </div>
        ) : (
          <span className="text-[12px] text-text-3 italic">{t('table.noGeo')}</span>
        ),
    },
    {
      id: 'status',
      header: t('table.status'),
      width: 120,
      cell: (row) => {
        const tone: StatusTone = row.status === ClientCompanyStatus.ACTIVE ? 'ok' : 'bad';
        return (
          <StatusBadge tone={tone} dot>
            {row.status === ClientCompanyStatus.ACTIVE ? t('status.active') : t('status.inactive')}
          </StatusBadge>
        );
      },
    },
    {
      id: 'actions',
      header: '',
      width: 52,
      align: 'center',
      cell: (row) => (
        <RowActionsMenu
          company={row}
          onEdit={setEditCompany}
          onDeactivate={setDeactivateTarget}
          onReactivate={setReactivateTarget}
        />
      ),
    },
  ];

  const hasFilters = !!(search.q || search.status || search.service_line);

  return (
    <div className="flex flex-col gap-[18px] p-6 bg-app-bg min-h-full overflow-y-auto">
      {/* Title band */}
      <div className="flex items-center justify-between rounded-xl bg-surface border border-border px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-[18px] font-semibold text-text">{t('title')}</h1>
          <p className="text-[13px] text-text-2">{t('subtitle')}</p>
        </div>
        <Link to={'/client-companies/new' as never}>
          <button
            type="button"
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-white text-[14px] font-medium hover:bg-primary-strong"
          >
            {t('actions.addCompany')}
          </button>
        </Link>
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
          label={t('stats.geofence')}
          value={`${geofenceCount} / ${activeCount}`}
          sub={t('stats.geofenceDesc')}
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
              className="flex-1 border-0 bg-transparent p-0 text-[13px] text-text outline-none placeholder:text-text-3"
              placeholder={t('filter.searchPlaceholder')}
              value={search.q ?? ''}
              onChange={(e) => setFilter({ q: e.target.value || undefined })}
            />
          </div>
          <div className="relative w-44 rounded-md border border-border bg-surface px-3 py-2">
            <select
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
            columns={columns}
            data={[]}
            getRowId={(row) => row.id}
            isLoading
          />
        ) : query.isError ? (
          <div className="p-8">
            <StateView
              kind="error"
              title={t('state.errorTitle')}
              description={classifyError(query.error).message}
              onRetry={() => void query.refetch()}
            />
          </div>
        ) : companies.length === 0 ? (
          <div className="p-8">
            <EmptyState
              variant={hasFilters ? 'filtered' : 'fresh'}
              title={hasFilters ? t('empty.filteredTitle') : t('empty.freshTitle')}
              description={hasFilters ? t('empty.filteredDesc') : t('empty.freshDesc')}
            />
          </div>
        ) : (
          <DataTable<ClientCompany> columns={columns} data={companies} getRowId={(row) => row.id} />
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

      {/* Edit drawer */}
      {editCompany && (
        <EditClientCompanyDrawer
          clientCompanyId={editCompany.id}
          open
          onClose={() => setEditCompany(null)}
          onSaved={() => {
            setEditCompany(null);
            void query.refetch();
          }}
        />
      )}

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
