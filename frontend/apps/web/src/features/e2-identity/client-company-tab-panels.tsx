/**
 * E2 · Perusahaan Klien — Detail tabs (Penempatan Aktif · Pemimpin Shift · Riwayat)
 *
 * These three panels replace the former "coming soon" stubs and make the company
 * detail page the SINGLE home for company-centric placement + shift-leader ops:
 *   - Penempatan Aktif — active agent roster at this company (read).
 *   - Pemimpin Shift    — current leader + Assign / Replace / Revoke (THE entry point
 *                          for shift-leader assignment; consolidated here from the old
 *                          placement-detail / roster screens).
 *   - Riwayat           — historical placements (include_history) at this company.
 *
 * Data: `useGetCompanyRoster` (E3) — returns placements, summary, current_shift_leader,
 * cursor pagination. The SL mutations are the existing modals from placement-overlays.
 *
 * RBAC: write actions are gated to super_admin / hr_admin (client-side is defense in
 * depth; the server's RequireRole on /shift-leader-assignments is the real gate).
 * Shift-leader identity (auth role + company scope) is DERIVED server-side at request
 * time from the active assignment created here — see auth middleware. INV-2/3/4 are
 * enforced by the API and surface as inline banners in the modals.
 */

import {
  ShiftLeaderAssignModal,
  ShiftLeaderEndConfirm,
  ShiftLeaderReplaceModal,
} from '@/features/e3-placement/placement-overlays.tsx';
import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type CompanyRosterResponse,
  type GetCompanyRosterParams,
  type Placement,
  PlacementLifecycleStatus,
  useGetCompanyRoster,
} from '@swp/api-client/e3';
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
} from '@swp/ui';
import { AlertTriangle, RefreshCw, UserMinus, UserPlus } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

const PAGE_SIZE = 25;

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

function initials(name: string): string {
  return name
    .split(' ')
    .slice(0, 2)
    .map((p) => p[0] ?? '')
    .join('')
    .toUpperCase();
}

// ---------------------------------------------------------------------------
// Shared roster table (active or history) with local cursor pagination
// ---------------------------------------------------------------------------

interface RosterTableProps {
  clientCompanyId: string;
  includeHistory: boolean;
}

function RosterTable({ clientCompanyId, includeHistory }: RosterTableProps) {
  const { t } = useTranslation('placements');
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<PlacementLifecycleStatus | ''>('');
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  const params: GetCompanyRosterParams = {
    limit: PAGE_SIZE,
    q: q || undefined,
    status: status || undefined,
    include_history: includeHistory || undefined,
    cursor,
  };

  const query = useGetCompanyRoster(clientCompanyId, params);
  const rosterData = query.data?.data as CompanyRosterResponse | undefined;
  const rows = (rosterData?.placements ?? []) as Placement[];
  const leaderEmployeeId = rosterData?.current_shift_leader?.employee_id;
  const hasFilters = Boolean(q || status);

  function resetPaging() {
    setCursor(undefined);
    setPrevCursors([]);
  }

  if (query.isError) {
    const { kind, message } = classifyError(query.error);
    return (
      <StateView
        kind={kind === 'forbidden' || kind === 'unauthenticated' ? 'empty' : 'error'}
        title={t('errorTitle')}
        description={message}
        onRetry={() => void query.refetch()}
      />
    );
  }

  const columns: Column<Placement>[] = [
    {
      id: 'agen',
      header: t('rosterColAgen'),
      width: 320,
      cell: (pl) => (
        <div className="flex items-center gap-[8px]">
          <Avatar initials={initials(pl.employee_name ?? pl.employee_id)} size={32} />
          <div className="flex flex-col gap-[2px]">
            <div className="flex items-center gap-[7px]">
              <span className="text-[14px] font-semibold text-text">{pl.employee_name ?? '—'}</span>
              {leaderEmployeeId === pl.employee_id && (
                <span className="flex items-center gap-[4px] rounded-full bg-primary-soft px-[7px] py-[2px] text-[11px] font-medium text-primary">
                  {t('leaderBadge')}
                </span>
              )}
            </div>
            <IdChip id={pl.employee_id} />
          </div>
        </div>
      ),
    },
    {
      id: 'liniLayanan',
      header: t('rosterColLiniLayanan'),
      width: 165,
      cell: (pl) =>
        pl.service_line_name ? (
          <span className="text-[13px] text-text-2">{pl.service_line_name}</span>
        ) : (
          <span className="text-[13px] text-text-3">—</span>
        ),
    },
    {
      id: 'posisi',
      header: t('rosterColPosisi'),
      width: 190,
      cell: (pl) => <span className="text-[13px] text-text-2">{pl.position_name ?? '—'}</span>,
    },
    {
      id: 'periode',
      header: t('rosterColPeriode'),
      width: 220,
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

  return (
    <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
      <div className="flex items-center gap-[10px] border-b border-border-soft px-[18px] py-[14px]">
        <SearchField
          placeholder={t('rosterSearchPlaceholder')}
          defaultValue={q}
          containerClassName="w-[220px]"
          onChange={(e) => {
            setQ(e.target.value);
            resetPaging();
          }}
        />
        <FilterSelect
          aria-label={t('filterStatus')}
          value={status}
          onChange={(e) => {
            setStatus((e.target.value as PlacementLifecycleStatus) || '');
            resetPaging();
          }}
        >
          <option value="">{t('filterStatusAll')}</option>
          <option value={PlacementLifecycleStatus.ACTIVE}>{t('lifecycle.ACTIVE')}</option>
          <option value={PlacementLifecycleStatus.PENDING_START}>
            {t('lifecycle.PENDING_START')}
          </option>
          <option value={PlacementLifecycleStatus.EXPIRING}>{t('lifecycle.EXPIRING')}</option>
          <option value={PlacementLifecycleStatus.ENDED}>{t('lifecycle.ENDED')}</option>
          <option value={PlacementLifecycleStatus.TERMINATED}>{t('lifecycle.TERMINATED')}</option>
          <option value={PlacementLifecycleStatus.RESIGNED}>{t('lifecycle.RESIGNED')}</option>
        </FilterSelect>
      </div>

      <DataTable
        aria-label={t('rosterTitle')}
        columns={columns}
        data={rows}
        getRowId={(p) => p.id}
        isLoading={query.isLoading}
        skeletonRows={5}
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
                include_history: includeHistory ? t('countAll') : t('countActive'),
              })}
              hasPrev={prevCursors.length > 0}
              hasNext={Boolean(rosterData?.has_more)}
              prevLabel={t('common.prev', { ns: 'translation' })}
              nextLabel={t('common.next', { ns: 'translation' })}
              onPrev={() => {
                const next = [...prevCursors];
                const c = next.pop();
                setPrevCursors(next);
                setCursor(c || undefined);
              }}
              onNext={() => {
                const nextCursor = rosterData?.next_cursor;
                if (!nextCursor) return;
                setPrevCursors((s) => [...s, cursor ?? '']);
                setCursor(nextCursor);
              }}
            />
          ) : undefined
        }
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Penempatan Aktif — active roster
// ---------------------------------------------------------------------------

export function PenempatanAktifPanel({ clientCompanyId }: { clientCompanyId: string }) {
  return <RosterTable clientCompanyId={clientCompanyId} includeHistory={false} />;
}

// ---------------------------------------------------------------------------
// Riwayat — historical placements (include_history)
// ---------------------------------------------------------------------------

export function RiwayatPanel({ clientCompanyId }: { clientCompanyId: string }) {
  return <RosterTable clientCompanyId={clientCompanyId} includeHistory={true} />;
}

// ---------------------------------------------------------------------------
// Pemimpin Shift — current leader + assign / replace / revoke (single entry point)
// ---------------------------------------------------------------------------

export function PemimpinShiftPanel({
  clientCompanyId,
  companyName,
}: { clientCompanyId: string; companyName: string }) {
  const { t } = useTranslation('clientCompanies');
  const user = useCurrentUser();
  const canManage = user?.role === 'super_admin' || user?.role === 'hr_admin';

  const [assignOpen, setAssignOpen] = useState(false);
  const [replaceOpen, setReplaceOpen] = useState(false);
  const [endOpen, setEndOpen] = useState(false);

  // current_shift_leader comes from the roster response (single source of truth).
  const query = useGetCompanyRoster(clientCompanyId, { limit: 1 });
  const rosterData = query.data?.data as CompanyRosterResponse | undefined;
  const leader = rosterData?.current_shift_leader;

  if (query.isPending) {
    return <StateView kind="loading" title={t('state.loading')} />;
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-xl border border-border bg-surface p-5">
        {leader ? (
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Avatar initials={initials(leader.employee_name ?? leader.employee_id)} size={44} />
              <div className="flex flex-col gap-1">
                <span className="text-[11px] font-bold uppercase tracking-[0.4px] text-text-3">
                  {t('detail.sl.currentLeaderLabel')}
                </span>
                <span className="text-[15px] font-semibold text-text">
                  {leader.employee_name ?? leader.employee_id}
                </span>
                <div className="flex items-center gap-2 pt-[2px]">
                  <IdChip id={leader.employee_id} />
                  <span className="text-[12px] text-text-3">
                    {t('detail.sl.since')} <DateText kind="date" value={leader.assigned_at} />
                  </span>
                </div>
              </div>
            </div>
            {canManage && (
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  className="flex items-center gap-2 rounded-lg border border-border bg-surface px-3 py-2 text-[13px] font-medium text-text-2 hover:bg-surface-2"
                  onClick={() => setReplaceOpen(true)}
                >
                  <RefreshCw className="size-4" aria-hidden />
                  {t('detail.sl.replaceBtn')}
                </button>
                <button
                  type="button"
                  className="flex items-center gap-2 rounded-lg border border-bad-bd bg-surface px-3 py-2 text-[13px] font-medium text-bad-tx hover:bg-bad-bg"
                  onClick={() => setEndOpen(true)}
                >
                  <UserMinus className="size-4" aria-hidden />
                  {t('detail.sl.revokeBtn')}
                </button>
              </div>
            )}
          </div>
        ) : (
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-[7px]">
              <span className="flex items-center gap-[5px] rounded-full border border-warn-bd bg-warn-bg px-[10px] py-[5px]">
                <AlertTriangle className="size-3.5 text-warn-tx" aria-hidden />
                <span className="text-[12px] font-semibold text-warn-tx">
                  {t('detail.sl.noLeader')}
                </span>
              </span>
              <span className="text-[12px] text-text-2">{t('detail.sl.noLeaderHint')}</span>
            </div>
            {canManage && (
              <button
                type="button"
                className="flex items-center gap-2 rounded-lg bg-primary px-[14px] py-[9px] text-[13px] font-semibold text-white hover:bg-primary/90"
                onClick={() => setAssignOpen(true)}
              >
                <UserPlus className="size-4" aria-hidden />
                {t('detail.sl.assignBtn')}
              </button>
            )}
          </div>
        )}
      </div>

      {/* Modals — reused from placement-overlays (the canonical SL mutation flows) */}
      <ShiftLeaderAssignModal
        open={assignOpen}
        onClose={() => setAssignOpen(false)}
        companyId={clientCompanyId}
        companyName={companyName}
      />
      {leader && (
        <>
          <ShiftLeaderReplaceModal
            open={replaceOpen}
            onClose={() => setReplaceOpen(false)}
            assignmentId={leader.id}
            companyName={companyName}
            currentLeaderName={leader.employee_name ?? leader.employee_id}
          />
          <ShiftLeaderEndConfirm
            open={endOpen}
            onClose={() => setEndOpen(false)}
            assignmentId={leader.id}
            companyName={companyName}
            leaderName={leader.employee_name ?? leader.employee_id}
          />
        </>
      )}
    </div>
  );
}
