/**
 * /me/pengajuan — Agent "Pengajuan Saya" (requests). Tab-based:
 *   • Cuti   — per-type leave-balance grid (every active type: kuota / terpakai / pending / sisa)
 *              + the agent's leave-request history (+ "Ajukan Cuti").
 *   • Lembur — the agent's overtime-request history with confirm/withdraw (+ "Ajukan Lembur").
 *
 * Leave balances come from GET /leave-balances/by-employee/{id}/types (E6 per-type ledger). The
 * legacy E10 dashboard "Sisa Cuti" summary is intentionally NOT used — it ignored the per-type
 * windows and reported 0/0. The server resolves the agent from the JWT principal (scope: self);
 * no employee_id is sent on the lists. All dates go through the Asia/Jakarta TZ layer.
 */
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type LeaveRequest,
  LeaveStatus,
  type LeaveTypeBalance,
  useGetEmployeeTypeBalances,
  useListLeaveRequests,
} from '@swp/api-client/e6';
import {
  type Overtime,
  type OvertimePage,
  OvertimeStatus,
  useConfirmOvertime,
  useListOvertime,
  useWithdrawOvertime,
} from '@swp/api-client/e7';
import type { StatusTone } from '@swp/design-tokens';
import { formatDate } from '@swp/shared';
import { resetCadenceKey } from '@swp/shared/leave';
import {
  Button,
  type Column,
  DataTable,
  EmptyState,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { Plane } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { overtimeStatusTone } from '../e7-overtime/overtime-shared.tsx';
import { AgentPage } from './agent-page.tsx';
import { AgentCorrectionPanel } from './me-correction-panel.tsx';
import { AgentLeaveCreateModal } from './me-leave-create-screen.tsx';
import { AgentLeaveStatusModal } from './me-leave-status-modal.tsx';
import { AgentOvertimeCreateModal } from './me-overtime-create-screen.tsx';

type Tab = 'cuti' | 'lembur' | 'koreksi';

// ---------------------------------------------------------------------------
// Status tone helpers
// ---------------------------------------------------------------------------

function leaveStatusTone(status: LeaveStatus): StatusTone {
  switch (status) {
    case LeaveStatus.APPROVED:
      return 'ok';
    case LeaveStatus.REJECTED:
      return 'bad';
    case LeaveStatus.PENDING:
      return 'warn';
    default:
      return 'neutral';
  }
}

function isOvertimePending(status: OvertimeStatus): boolean {
  return status === OvertimeStatus.PENDING_AGENT_CONFIRM || status === OvertimeStatus.PENDING;
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function AgentPengajuanScreen() {
  const { t } = useTranslation('agent');
  const user = useCurrentUser();
  const [tab, setTab] = useState<Tab>('cuti');

  const [leaveCreateOpen, setLeaveCreateOpen] = useState(false);
  const [overtimeCreateOpen, setOvertimeCreateOpen] = useState(false);

  // ---- Data ----
  const leaveQ = useListLeaveRequests({ limit: 50 });
  const overtimeQ = useListOvertime({ limit: 50 });
  const balancesQ = useGetEmployeeTypeBalances(user?.employeeId ?? '', {
    query: { enabled: !!user?.employeeId },
  });

  const leaveItems = (leaveQ.data?.data as { data?: LeaveRequest[] } | undefined)?.data ?? [];
  const overtimeItems = (overtimeQ.data?.data as OvertimePage | undefined)?.data ?? [];
  const balances = (balancesQ.data?.data as { data?: LeaveTypeBalance[] } | undefined)?.data ?? [];

  // The balance endpoint now returns only the employee's HR-ASSIGNED types (ELE-1 — backend INNER
  // JOINs the entitlement), so the old common/"lihat semua" split + HEAD_OFFICE filter are gone.
  // Just order ANNUAL pool first (CT), then the rest by name — everyday quota up top.
  const eligible = [...balances].sort((a, b) => {
    const ai = a.cap_basis === 'ANNUAL_POOL' ? 0 : 1;
    const bi = b.cap_basis === 'ANNUAL_POOL' ? 0 : 1;
    return ai - bi || a.name.localeCompare(b.name);
  });

  // Koreksi's primary action ("Ajukan Koreksi") lives inside the panel — it drives the
  // searchable-picker flow state — so the header action is omitted on that tab.
  const actions =
    tab === 'cuti' ? (
      <Button variant="primary" size="sm" onClick={() => setLeaveCreateOpen(true)}>
        {t('leaveNewBtn')}
      </Button>
    ) : tab === 'lembur' ? (
      <Button variant="primary" size="sm" onClick={() => setOvertimeCreateOpen(true)}>
        {t('otNewBtn')}
      </Button>
    ) : null;

  return (
    <AgentPage title={t('pengajuanHeading')} actions={actions}>
      <TabBar tab={tab} onTab={setTab} />

      {tab === 'cuti' ? (
        <div className="flex flex-col gap-4 lg:p-6">
          <section className="flex flex-col gap-3">
            <h2 className="text-[15px] font-bold text-text">{t('balSectionTitle')}</h2>
            <LeaveBalancesTable query={balancesQ} items={eligible} />
          </section>
          <section className="flex flex-col gap-3">
            <h2 className="text-[15px] font-bold text-text">{t('leaveHistoryTitle')}</h2>
            <LeavePanel
              query={leaveQ}
              items={leaveItems}
              createOpen={leaveCreateOpen}
              onCreateOpenChange={setLeaveCreateOpen}
            />
          </section>
        </div>
      ) : tab === 'lembur' ? (
        <section className="flex flex-col gap-3">
          <h2 className="text-[15px] font-bold text-text">{t('otHistoryTitle')}</h2>
          <OvertimePanel
            query={overtimeQ}
            items={overtimeItems}
            createOpen={overtimeCreateOpen}
            onCreateOpenChange={setOvertimeCreateOpen}
          />
        </section>
      ) : (
        <AgentCorrectionPanel />
      )}
    </AgentPage>
  );
}

// ---------------------------------------------------------------------------
// Tab bar
// ---------------------------------------------------------------------------

function TabBar({ tab, onTab }: { tab: Tab; onTab: (t: Tab) => void }) {
  const { t } = useTranslation('agent');
  const tabs: { id: Tab; label: string }[] = [
    { id: 'cuti', label: t('tabCuti') },
    { id: 'lembur', label: t('tabLembur') },
    { id: 'koreksi', label: t('tabKoreksi') },
  ];
  return (
    <div
      role="tablist"
      aria-label={t('pengajuanHeading')}
      className="flex gap-1 border-b border-border"
    >
      {tabs.map((tb) => (
        <button
          key={tb.id}
          type="button"
          role="tab"
          aria-selected={tab === tb.id}
          onClick={() => onTab(tb.id)}
          className={[
            '-mb-px border-b-2 px-4 py-2.5 text-[14px] font-medium transition-colors',
            tab === tb.id
              ? 'border-primary text-primary'
              : 'border-transparent text-text-2 hover:text-text',
          ].join(' ')}
        >
          {tb.label}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Per-type leave-balance table (Cuti tab)
//
// Replaces the old card grid (leave-entitlement-assignment §7.1). The balance endpoint now
// returns only the employee's HR-assigned types (ELE-1), so no common/"lihat semua" split.
// Columns: Jenis Cuti · Sisa · Terpakai · Pending · Kuota · Reset / Kedaluwarsa.
// ---------------------------------------------------------------------------

type BalancesQuery = ReturnType<typeof useGetEmployeeTypeBalances>;

/** UNCAPPED / nil-cap types meter no day quota ("Sesuai ketentuan" — sick-with-note, civic duty). */
function isUncappedBalance(b: LeaveTypeBalance): boolean {
  return b.remaining_days == null && b.cap_value == null;
}

function LeaveBalancesTable({ query, items }: { query: BalancesQuery; items: LeaveTypeBalance[] }) {
  const { t } = useTranslation('agent');

  const unit = (b: LeaveTypeBalance, n: number) =>
    b.cap_unit === 'COUNT' ? t('balUnitCount', { count: n }) : t('durationDays', { count: n });

  const columns: Column<LeaveTypeBalance>[] = [
    {
      id: 'type',
      header: t('balColType'),
      priority: 'primary',
      width: 240,
      cell: (b) => (
        <div className="flex items-center gap-2">
          <span
            className="size-2.5 shrink-0 rounded-full"
            style={{ backgroundColor: b.color || 'var(--color-text-3)' }}
            aria-hidden
          />
          <span className="font-medium text-text">{b.name}</span>
          <span className="font-mono text-[11px] text-text-3">{b.code}</span>
        </div>
      ),
    },
    {
      id: 'remaining',
      header: t('balColRemaining'),
      priority: 'secondary',
      width: 140,
      cell: (b) =>
        isUncappedBalance(b) ? (
          <span className="text-[13px] text-text-2">{t('balUncapped')}</span>
        ) : (
          <span className="font-semibold text-text tabular-nums">
            {unit(b, b.remaining_days ?? b.cap_value ?? 0)}
          </span>
        ),
    },
    {
      id: 'used',
      header: t('balColUsed'),
      priority: 'hidden-mobile',
      width: 110,
      cell: (b) => <span className="text-text-2 tabular-nums">{unit(b, b.used_days)}</span>,
    },
    {
      id: 'pending',
      header: t('balColPending'),
      priority: 'hidden-mobile',
      width: 110,
      cell: (b) =>
        b.pending_days > 0 ? (
          <span className="text-warn-tx tabular-nums">{unit(b, b.pending_days)}</span>
        ) : (
          <span className="text-text-3 tabular-nums">—</span>
        ),
    },
    {
      id: 'quota',
      header: t('balColQuota'),
      priority: 'secondary',
      width: 110,
      cell: (b) =>
        b.entitled_days == null ? (
          <span className="text-text-2">{t('balUncapped')}</span>
        ) : (
          <span className="text-text-2 tabular-nums">{unit(b, b.entitled_days)}</span>
        ),
    },
    {
      id: 'reset',
      header: t('balColReset'),
      cell: (b) => <ResetExpiryCell b={b} />,
    },
  ];

  if (query.isError) {
    return (
      <StateView kind="error" title={t('errorGeneric')} onRetry={() => void query.refetch()} />
    );
  }

  return (
    <DataTable
      responsive
      aria-label={t('balSectionTitle')}
      columns={columns}
      data={items}
      getRowId={(b) => b.leave_type_id}
      isLoading={query.isLoading}
      skeletonRows={5}
      empty={<EmptyState variant="fresh" icon={Plane} title={t('balEmpty')} />}
    />
  );
}

/** "Reset / Kedaluwarsa": cap_basis cadence + optional expiry, e.g. "Tahunan · hangus 31 Des 2026". */
function ResetExpiryCell({ b }: { b: LeaveTypeBalance }) {
  const { t } = useTranslation('agent');
  const cadence = t(`balReset.${resetCadenceKey(b.cap_basis)}`);
  // expires_at formatted via the Asia/Jakarta TZ layer (no raw Date formatting).
  const expiry =
    b.expires_at && resetCadenceKey(b.cap_basis) === 'ANNUAL_POOL'
      ? t('balExpiresAt', { date: formatDate(b.expires_at) })
      : null;
  return (
    <span className="text-[13px] text-text-2">
      {cadence}
      {expiry && <span className="text-text-3"> · {expiry}</span>}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Leave history (full DataTable — Cuti tab)
// ---------------------------------------------------------------------------

type LeaveQuery = ReturnType<typeof useListLeaveRequests>;
type OvertimeQuery = ReturnType<typeof useListOvertime>;

function LeavePanel({
  query,
  items,
  createOpen,
  onCreateOpenChange,
}: {
  query: LeaveQuery;
  items: LeaveRequest[];
  createOpen: boolean;
  onCreateOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation('agent');
  const [selected, setSelected] = useState<LeaveRequest | null>(null);

  const columns: Column<LeaveRequest>[] = [
    {
      id: 'type',
      header: t('leaveType'),
      priority: 'primary',
      width: 220,
      cell: (r) => (
        <span className="font-medium text-text">{r.leave_type_name ?? r.leave_type_id}</span>
      ),
    },
    {
      id: 'dateRange',
      header: t('leaveDateRange'),
      priority: 'secondary',
      width: 220,
      cell: (r) => (
        <span className="text-sm text-text-2 tabular-nums">
          {formatDate(r.start_date)} &rarr; {formatDate(r.end_date)}
        </span>
      ),
    },
    {
      id: 'reason',
      header: t('leaveReason'),
      priority: 'secondary',
      cell: (r) =>
        r.reason ? (
          <span className="line-clamp-1 text-sm text-text-2">{r.reason}</span>
        ) : (
          <span className="text-sm italic text-text-3">—</span>
        ),
    },
    {
      id: 'status',
      header: t('leaveStatus'),
      priority: 'secondary',
      width: 200,
      cell: (r) => (
        <StatusBadge dot tone={leaveStatusTone(r.status as LeaveStatus)}>
          {t(`leave.status.${r.status}`, { defaultValue: r.status })}
        </StatusBadge>
      ),
    },
    {
      id: 'detail',
      header: '',
      priority: 'hidden-mobile',
      width: 96,
      align: 'right',
      cell: (row) => (
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            setSelected(row);
          }}
        >
          {t('common.detail', { ns: 'translation' })}
        </Button>
      ),
    },
  ];

  return (
    <>
      {query.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void query.refetch()} />
      ) : (
        <DataTable
          responsive
          aria-label={t('leaveTitle')}
          columns={columns}
          data={items}
          getRowId={(r) => r.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          empty={<EmptyState variant="fresh" title={t('leaveEmpty')} />}
        />
      )}

      {createOpen && (
        <AgentLeaveCreateModal
          open
          onOpenChange={onCreateOpenChange}
          onSuccess={() => void query.refetch()}
        />
      )}
      {selected && (
        <AgentLeaveStatusModal
          request={selected}
          open
          onOpenChange={(o) => {
            if (!o) setSelected(null);
          }}
          onChanged={() => void query.refetch()}
        />
      )}
    </>
  );
}

// ---------------------------------------------------------------------------
// Overtime history (full DataTable — Lembur tab, incl. confirm/withdraw)
// ---------------------------------------------------------------------------

function OvertimePanel({
  query,
  items,
  createOpen,
  onCreateOpenChange,
}: {
  query: OvertimeQuery;
  items: Overtime[];
  createOpen: boolean;
  onCreateOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation('agent');
  const { t: tOt } = useTranslation('overtime');
  const { toast } = useToast();

  const confirm = useConfirmOvertime();
  const withdraw = useWithdrawOvertime();

  async function onConfirm(id: string) {
    try {
      await confirm.mutateAsync({ id, data: {} });
      await query.refetch();
      toast({ tone: 'success', title: t('otConfirmed') });
    } catch {
      toast({ tone: 'error', title: t('otError') });
    }
  }

  async function onWithdraw(id: string) {
    try {
      await withdraw.mutateAsync({ id });
      await query.refetch();
      toast({ tone: 'success', title: t('otWithdrawn') });
    } catch {
      toast({ tone: 'error', title: t('otError') });
    }
  }

  const columns: Column<Overtime>[] = [
    {
      id: 'date',
      header: t('otWorkDate'),
      priority: 'primary',
      width: 140,
      cell: (r) => <span className="font-medium text-text">{formatDate(r.work_date)}</span>,
    },
    {
      id: 'time',
      header: t('otTimeRange'),
      priority: 'secondary',
      width: 160,
      cell: (r) => (
        <span className="text-sm text-text-2 tabular-nums">
          {r.planned_start_time ?? '—'} – {r.planned_end_time ?? '—'}
        </span>
      ),
    },
    {
      id: 'reason',
      header: t('otReason'),
      priority: 'secondary',
      cell: (r) =>
        r.reason ? (
          <span className="line-clamp-2 text-sm text-text-2">{r.reason}</span>
        ) : (
          <span className="text-sm italic text-text-3">—</span>
        ),
    },
    {
      id: 'status',
      header: t('otStatus'),
      priority: 'secondary',
      width: 200,
      cell: (r) => (
        <StatusBadge dot tone={overtimeStatusTone(r.status)}>
          {tOt(`status.${r.status}`, { defaultValue: r.status })}
        </StatusBadge>
      ),
    },
    {
      id: 'actions',
      header: '',
      priority: 'hidden-mobile',
      width: 200,
      align: 'right',
      cell: (r) => {
        const pending = isOvertimePending(r.status);
        const confirmBusy = confirm.isPending && confirm.variables?.id === r.id;
        const withdrawBusy = withdraw.isPending && withdraw.variables?.id === r.id;
        const busy = confirmBusy || withdrawBusy;

        if (!pending) return null;

        return (
          <div className="flex items-center justify-end gap-2">
            {r.status === OvertimeStatus.PENDING_AGENT_CONFIRM && (
              <Button
                size="sm"
                variant="primary"
                disabled={busy}
                onClick={() => void onConfirm(r.id)}
              >
                {t('otConfirm')}
              </Button>
            )}
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => void onWithdraw(r.id)}>
              {t('otWithdraw')}
            </Button>
          </div>
        );
      },
    },
  ];

  return (
    <>
      {query.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void query.refetch()} />
      ) : (
        <DataTable
          responsive
          aria-label={t('otTitle')}
          columns={columns}
          data={items}
          getRowId={(r) => r.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          empty={<EmptyState variant="fresh" title={t('otEmpty')} />}
        />
      )}

      {createOpen && (
        <AgentOvertimeCreateModal
          open
          onOpenChange={onCreateOpenChange}
          onSuccess={() => void query.refetch()}
        />
      )}
    </>
  );
}
