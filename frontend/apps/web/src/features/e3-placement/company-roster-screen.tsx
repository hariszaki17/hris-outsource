/**
 * E3 · Roster Perusahaan — per-company placement table (HR/Admin + SL read-only)
 *
 * .pen frames:
 *   nLN4d  — E3 · Roster — Plaza Senayan   (HR/Admin: has Export + Buat Penempatan + Reassign)
 *   o5Txgg — E3 SL · Roster (read-only)    (Shift Leader: actions hidden / disabled)
 *
 * Layout: CompanyHeader (name + status badge + address + service-line pills + leader +
 * stat counters) → Filters (search, service line, status, period toggle "Sertakan riwayat")
 * → RosterTable (DataTable with cursor pagination).
 *
 * Role gate: isShiftLeader hides Export, "Buat Penempatan", and "Ganti" (reassign) actions
 * per o5Txgg frame which uses enabled:false on those interactive elements.
 *
 * INV-2 — company with active placements must have shift leader; null leader renders
 * "Tetapkan leader" warn CTA (from .pen h22BP pattern).
 *
 * ENGINEERING.md D1 — typed URL search params + cursor pagination.
 */

import { ServiceLinePicker } from '@/features/e2-identity/pickers/service-line-picker.tsx';
import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type GetCompanyRosterParams,
  PlacementLifecycleStatus,
  useGetCompanyRoster,
} from '@swp/api-client/e3';
import type { CompanyRosterResponse, Placement } from '@swp/api-client/e3';
import type { StatusTone } from '@swp/design-tokens';
import {
  Avatar,
  type Column,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  FilterSelect,
  IdChip,
  SearchField,
  StateView,
  StatusBadge,
  Toggle,
  useToast,
} from '@swp/ui';
import { Link, useNavigate, useSearch } from '@tanstack/react-router';
import { AlertTriangle, Download, MoreVertical, Plus, RefreshCw } from 'lucide-react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

const PAGE_SIZE = 25;

/** Typed filter/cursor search params for the roster route. */
export type CompanyRosterSearch = {
  q?: string;
  service_line_id?: string;
  status?: PlacementLifecycleStatus;
  include_history?: boolean;
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
// Lifecycle status → StatusBadge tone
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
// Props
// ---------------------------------------------------------------------------

export interface CompanyRosterScreenProps {
  clientCompanyId: string;
}

// ---------------------------------------------------------------------------
// CompanyRosterScreen
// ---------------------------------------------------------------------------

export function CompanyRosterScreen({ clientCompanyId }: CompanyRosterScreenProps) {
  const { t } = useTranslation('placements');
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as CompanyRosterSearch;
  const currentUser = useCurrentUser();
  const isShiftLeader = currentUser?.role === 'shift_leader';
  const { toast } = useToast();

  const [prevCursors, setPrevCursors] = useState<string[]>([]);
  const [openMenuPlacementId, setOpenMenuPlacementId] = useState<string | null>(null);
  const kebabRefs = useRef<Map<string, HTMLButtonElement>>(new Map());

  // ---------------------------------------------------------------------------
  // Search params → API params
  // ---------------------------------------------------------------------------

  const params: GetCompanyRosterParams = {
    limit: PAGE_SIZE,
    q: search.q || undefined,
    service_line_id: search.service_line_id || undefined,
    status: search.status || undefined,
    include_history: search.include_history || undefined,
    cursor: search.cursor,
  };

  const query = useGetCompanyRoster(clientCompanyId, params);

  const rosterData = query.data?.data as CompanyRosterResponse | undefined;
  const rows = (rosterData?.placements ?? []) as Placement[];
  const summary = rosterData?.summary;
  const shiftLeader = rosterData?.current_shift_leader;

  const hasFilters = Boolean(search.q || search.service_line_id || search.status);

  // ---------------------------------------------------------------------------
  // Navigation helpers
  // ---------------------------------------------------------------------------

  function setSearch(patch: Partial<CompanyRosterSearch>) {
    const next: CompanyRosterSearch = { ...search, cursor: undefined, ...patch };
    void navigate({
      to: '/client-companies/$clientCompanyId',
      params: { clientCompanyId },
      search: next,
    });
    setPrevCursors([]);
  }

  // ---------------------------------------------------------------------------
  // Error state
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    return (
      <div className="flex flex-col gap-[18px]">
        {kind === 'forbidden' || kind === 'unauthenticated' ? (
          <EmptyState
            variant="no-permission"
            title={t('noPermissionTitle')}
            description={t('noPermissionBody')}
          />
        ) : kind === 'not-found' ? (
          <EmptyState
            variant="fresh"
            title={t('rosterNotFoundTitle')}
            description={t('rosterNotFoundBody')}
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
  // Columns
  // ---------------------------------------------------------------------------

  const columns: Column<Placement>[] = [
    {
      id: 'agen',
      header: t('rosterColAgen'),
      width: 330,
      cell: (pl) => {
        const isLeader = shiftLeader?.employee_id === pl.employee_id;
        return (
          <div className="flex items-center gap-[8px]">
            <Avatar initials={initials(pl.employee_name ?? pl.employee_id)} size={32} />
            <div className="flex flex-col gap-[2px]">
              <div className="flex items-center gap-[7px]">
                <span className="text-[14px] font-semibold text-text">
                  {pl.employee_name ?? '—'}
                </span>
                {isLeader && (
                  <span className="flex items-center gap-[4px] rounded-full bg-primary-soft px-[7px] py-[2px] text-[11px] font-medium text-primary">
                    {t('leaderBadge')}
                  </span>
                )}
              </div>
              <IdChip id={pl.employee_id} />
            </div>
          </div>
        );
      },
    },
    {
      id: 'liniLayanan',
      header: t('rosterColLiniLayanan'),
      width: 165,
      cell: (pl) =>
        pl.service_line_name ? (
          <div className="flex items-center gap-[8px]">
            <span className="size-[7px] rounded-full bg-info-tx shrink-0" aria-hidden />
            <span className="text-[13px] text-text-2">{pl.service_line_name}</span>
          </div>
        ) : (
          <span className="text-[13px] text-text-3">—</span>
        ),
    },
    {
      id: 'posisi',
      header: t('rosterColPosisi'),
      width: 195,
      cell: (pl) => <span className="text-[13px] text-text-2">{pl.position_name ?? '—'}</span>,
    },
    {
      id: 'periode',
      header: t('rosterColPeriode'),
      width: 230,
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
      header: t('rosterColStatus'),
      width: 140,
      cell: (pl) => (
        <StatusBadge dot tone={lifecycleTone[pl.lifecycle_status]}>
          {t(`lifecycle.${pl.lifecycle_status}`)}
        </StatusBadge>
      ),
    },
  ];

  // Kebab column — shown to both roles (links to detail), but HR gets extra actions
  columns.push({
    id: 'actions',
    header: '',
    width: 56,
    cell: (pl) => {
      const isOpen = openMenuPlacementId === pl.id;
      const setRef = (el: HTMLButtonElement | null) => {
        if (el) kebabRefs.current.set(pl.id, el);
        else kebabRefs.current.delete(pl.id);
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
            onClick={() => setOpenMenuPlacementId(isOpen ? null : pl.id)}
          >
            <MoreVertical className="size-4" aria-hidden />
          </button>
        </div>
      );
    },
  });

  // ---------------------------------------------------------------------------
  // Render — CompanyHeader (from .pen nLN4d fc2qf)
  // ---------------------------------------------------------------------------

  const companyName = rosterData?.company_name ?? t('rosterCompanyFallback');

  return (
    <div className="flex flex-col gap-[18px]">
      {/* CompanyHeader card */}
      <div className="flex flex-col gap-[16px] rounded-xl border border-border bg-surface p-[20px]">
        {/* Row 1: name + status + actions */}
        <div className="flex items-start justify-between">
          <div className="flex flex-col gap-[8px]">
            {/* Name row */}
            <div className="flex items-center gap-[10px]">
              <h1 className="text-[22px] font-bold text-text">
                {query.isLoading ? '—' : companyName}
              </h1>
              <StatusBadge dot tone="ok">
                {t('companyStatusActive')}
              </StatusBadge>
            </div>
            {/* Service line pills row */}
            {rosterData?.summary?.by_service_line && (
              <div className="flex flex-wrap gap-[8px] pt-[2px]">
                {rosterData.summary.by_service_line.map((sl) => (
                  <span
                    key={sl.service_line_id}
                    className="flex items-center gap-[6px] rounded-full border border-border bg-surface-2 px-[10px] py-[4px] text-[11px] font-semibold text-text-2"
                  >
                    <span className="size-[6px] rounded-full bg-info-tx shrink-0" aria-hidden />
                    {sl.service_line_name ?? sl.service_line_id}
                  </span>
                ))}
              </div>
            )}
          </div>

          {/* Action buttons — hidden for shift leader (o5Txgg: enabled:false pattern) */}
          {!isShiftLeader && (
            <div className="flex items-center gap-[10px]">
              <button
                type="button"
                className="flex items-center gap-2 rounded-lg border border-border bg-surface px-[14px] py-[9px] text-[13px] font-medium text-text-2 hover:bg-surface-2"
                onClick={() => {
                  toast({ tone: 'info', title: t('exportQueued') });
                }}
              >
                <Download className="size-4" aria-hidden />
                {t('export')}
              </button>
              <Link
                to="/placements/new"
                className="flex items-center gap-2 rounded-lg bg-primary px-[14px] py-[9px] text-[13px] font-semibold text-white hover:bg-primary/90"
              >
                <Plus className="size-4" aria-hidden />
                {t('createPlacement')}
              </Link>
            </div>
          )}
        </div>

        {/* Divider */}
        <div className="h-px w-full bg-border-soft" />

        {/* Row 2: shift leader + stat counters */}
        <div className="flex items-center justify-between">
          {/* Shift leader info */}
          <div className="flex items-center gap-[10px]">
            {shiftLeader ? (
              <>
                <Avatar
                  initials={initials(shiftLeader.employee_name ?? shiftLeader.employee_id)}
                  size={36}
                />
                <div className="flex flex-col gap-[2px]">
                  <span className="text-[11px] font-bold uppercase tracking-[0.4px] text-text-3">
                    {t('shiftLeaderLabel')}
                  </span>
                  <span className="text-[14px] font-semibold text-text">
                    {shiftLeader.employee_name ?? shiftLeader.employee_id}
                  </span>
                </div>
                {/* Reassign — hidden for SL (o5Txgg enabled:false pattern) */}
                {!isShiftLeader && (
                  <button
                    type="button"
                    className="ml-[4px] flex items-center gap-[6px] rounded-md px-[10px] py-[6px] text-[12px] font-medium text-text-2 hover:bg-surface-2"
                    onClick={() => {
                      toast({ tone: 'info', title: t('reassignNotImplemented') });
                    }}
                  >
                    <RefreshCw className="size-3" aria-hidden />
                    {t('reassign')}
                  </button>
                )}
              </>
            ) : (
              /* No-leader warn state — from .pen h22BP */
              <div className="flex items-center gap-[5px] rounded-full border border-warn-bd bg-warn-bg px-[9px] py-[4px]">
                <AlertTriangle className="size-3 text-warn-tx" aria-hidden />
                <span className="text-[11px] font-semibold text-warn-tx">{t('noLeaderCta')}</span>
              </div>
            )}
          </div>

          {/* Summary counters — from .pen d4tcP4 */}
          <div className="flex items-center gap-[8px]">
            <div className="flex items-center gap-[6px] rounded-lg bg-surface-2 px-[12px] py-[6px]">
              <span className="text-[15px] font-bold text-ok-tx">
                {summary?.total_active ?? '—'}
              </span>
              <span className="text-[12px] font-medium text-text-2">{t('countAktif')}</span>
            </div>
            <div className="flex items-center gap-[6px] rounded-lg bg-surface-2 px-[12px] py-[6px]">
              <span className="text-[15px] font-bold text-info-tx">
                {summary?.total_scheduled ?? '—'}
              </span>
              <span className="text-[12px] font-medium text-text-2">{t('countTerjadwal')}</span>
            </div>
            <div className="flex items-center gap-[6px] rounded-lg bg-surface-2 px-[12px] py-[6px]">
              <span className="text-[15px] font-bold text-warn-tx">
                {summary?.total_expiring ?? '—'}
              </span>
              <span className="text-[12px] font-medium text-text-2">{t('countAkanBerakhir')}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Roster table card */}
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        {/* Filter row — from .pen KEdDw / DCrjS */}
        <div className="flex items-center justify-between gap-[10px] border-b border-border-soft px-[18px] py-[14px]">
          <div className="flex items-center gap-[10px]">
            <SearchField
              placeholder={t('rosterSearchPlaceholder')}
              defaultValue={search.q ?? ''}
              containerClassName="w-[220px]"
              onChange={(e) => setSearch({ q: e.target.value || undefined })}
            />
            {/* Service line filter */}
            <div className="w-[180px]">
              <ServiceLinePicker
                value={search.service_line_id ?? null}
                onChange={(v) => setSearch({ service_line_id: v ?? undefined })}
                placeholder={t('filterServiceLine')}
              />
            </div>
            {/* Status filter */}
            <FilterSelect
              aria-label={t('filterStatus')}
              value={search.status ?? ''}
              onChange={(e) =>
                setSearch({ status: (e.target.value as PlacementLifecycleStatus) || undefined })
              }
            >
              <option value="">{t('filterStatusAll')}</option>
              <option value={PlacementLifecycleStatus.ACTIVE}>{t('lifecycle.ACTIVE')}</option>
              <option value={PlacementLifecycleStatus.PENDING_START}>
                {t('lifecycle.PENDING_START')}
              </option>
              <option value={PlacementLifecycleStatus.EXPIRING}>{t('lifecycle.EXPIRING')}</option>
              <option value={PlacementLifecycleStatus.ENDED}>{t('lifecycle.ENDED')}</option>
              <option value={PlacementLifecycleStatus.TERMINATED}>
                {t('lifecycle.TERMINATED')}
              </option>
              <option value={PlacementLifecycleStatus.RESIGNED}>{t('lifecycle.RESIGNED')}</option>
            </FilterSelect>
          </div>

          {/* Include history toggle — from .pen "Sertakan riwayat" */}
          <div className="flex items-center gap-[8px]">
            <span className="text-[13px] font-medium text-text-2">{t('filterIncludeHistory')}</span>
            <Toggle
              checked={Boolean(search.include_history)}
              onCheckedChange={(v) =>
                setSearch({ include_history: v || undefined, cursor: undefined })
              }
              aria-label={t('filterIncludeHistory')}
            />
          </div>
        </div>

        {/* Data table */}
        <DataTable
          aria-label={t('rosterTitle')}
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
            hasFilters ? (
              <EmptyState
                variant="filtered"
                title={t('filteredTitle')}
                description={t('filteredBody')}
              />
            ) : (
              <EmptyState
                variant="fresh"
                title={t('rosterEmptyTitle')}
                description={t('rosterEmptyBody')}
              />
            )
          }
          footer={
            rows.length > 0 ? (
              <CursorPagination
                rangeLabel={t('rosterResultRange', {
                  count: rows.length,
                  include_history: search.include_history ? t('countAll') : t('countActive'),
                })}
                hasPrev={prevCursors.length > 0}
                hasNext={Boolean(rosterData?.has_more)}
                prevLabel={t('common.prev', { ns: 'translation' })}
                nextLabel={t('common.next', { ns: 'translation' })}
                onPrev={() => {
                  const next = [...prevCursors];
                  const cursor = next.pop();
                  setPrevCursors(next);
                  void navigate({
                    to: '/client-companies/$clientCompanyId',
                    params: { clientCompanyId },
                    search: { ...search, cursor: cursor || undefined },
                  });
                }}
                onNext={() => {
                  const nextCursor = rosterData?.next_cursor;
                  if (!nextCursor) return;
                  setPrevCursors((s) => [...s, search.cursor ?? '']);
                  void navigate({
                    to: '/client-companies/$clientCompanyId',
                    params: { clientCompanyId },
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
