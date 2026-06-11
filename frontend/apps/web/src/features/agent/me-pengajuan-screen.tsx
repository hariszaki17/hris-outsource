/**
 * /me/pengajuan — Agent "Pengajuan Saya" (requests). A self-service DASHBOARD that summarizes
 * the agent's leave + overtime activity (brainstorm.pen frame HcTQb · "Agen Web · Pengajuan").
 *
 * Layout (matches HcTQb):
 *   1. Title band: h1 "Pengajuan Saya" + BOTH CTAs always visible
 *        ("Ajukan Cuti" secondary · "Ajukan Lembur" primary).
 *   2. Row of three StatCards: Sisa Cuti · Lembur bulan ini · Menunggu Persetujuan.
 *   3. Two side-by-side panels (Cuti | Lembur): each a compact list of the 5 most-recent
 *      items with a "Lihat semua →" link that switches to the full-list view.
 *
 * The "Lihat semua" target reuses the legacy DataTable surfaces as secondary views:
 *   - view='leave'    → full leave list (type · range · reason · status), row → status modal.
 *   - view='overtime' → full OT list WITH confirm/withdraw row actions.
 * A small back button returns to the dashboard (AgentPage.backTo expects a route string, so
 * the back affordance is rendered as an in-page ghost Button instead).
 *
 * The server resolves the agent from the JWT principal (`scope: self`) — no employee_id is sent
 * on the lists. Dashboard stats come from GET /me/dashboard (E10). All dates go through the
 * Asia/Jakarta TZ layer (@swp/shared · currentJakartaIso).
 */
import { type LeaveRequest, LeaveStatus, useListLeaveRequests } from '@swp/api-client/e6';
import {
  type Overtime,
  type OvertimePage,
  OvertimeStatus,
  useConfirmOvertime,
  useListOvertime,
  useWithdrawOvertime,
} from '@swp/api-client/e7';
import { type AgentDashboard, type Dashboard, useGetMyDashboard } from '@swp/api-client/e10';
import type { StatusTone } from '@swp/design-tokens';
import { formatDate } from '@swp/shared';
import {
  Button,
  type Column,
  DataTable,
  EmptyState,
  StatCard,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { ArrowLeft, ArrowRight, Clock, Plane, Timer } from 'lucide-react';
import type * as React from 'react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { currentJakartaIso } from '../e4-scheduling/roster-compliance.ts';
import { overtimeStatusTone } from '../e7-overtime/overtime-shared.tsx';
import { AgentPage } from './agent-page.tsx';
import { AgentLeaveCreateModal } from './me-leave-create-screen.tsx';
import { AgentLeaveStatusModal } from './me-leave-status-modal.tsx';
import { AgentOvertimeCreateModal } from './me-overtime-create-screen.tsx';

type PengajuanView = 'dashboard' | 'leave' | 'overtime';

// ---------------------------------------------------------------------------
// Status tone helpers
// ---------------------------------------------------------------------------

function leaveStatusTone(status: LeaveStatus): StatusTone {
  switch (status) {
    case LeaveStatus.APPROVED:
      return 'ok';
    case LeaveStatus.REJECTED:
      return 'bad';
    case LeaveStatus.PENDING_L1:
    case LeaveStatus.PENDING_HR:
      return 'warn';
    default:
      return 'neutral';
  }
}

function isOvertimePending(status: OvertimeStatus): boolean {
  return (
    status === OvertimeStatus.PENDING_AGENT_CONFIRM ||
    status === OvertimeStatus.PENDING_L1 ||
    status === OvertimeStatus.PENDING_HR
  );
}

// Inclusive day count between two plain dates, used as a fallback when the API omits duration_days.
function inclusiveDays(start: string, end: string): number {
  const s = Date.parse(`${start.slice(0, 10)}T00:00:00Z`);
  const e = Date.parse(`${end.slice(0, 10)}T00:00:00Z`);
  if (Number.isNaN(s) || Number.isNaN(e) || e < s) return 1;
  return Math.round((e - s) / 86_400_000) + 1;
}

// Hours between two "HH:MM" planned times (fallback when no hours field is present).
function hoursBetween(start?: string | null, end?: string | null): number {
  if (!start || !end) return 0;
  const [sh = Number.NaN, sm = Number.NaN] = start.split(':').map(Number);
  const [eh = Number.NaN, em = Number.NaN] = end.split(':').map(Number);
  if ([sh, sm, eh, em].some(Number.isNaN)) return 0;
  let mins = eh * 60 + em - (sh * 60 + sm);
  if (mins < 0) mins += 24 * 60; // cross-midnight
  return Math.round((mins / 60) * 10) / 10;
}

// Current-month label in Asia/Jakarta, e.g. "Juni 2026" — via the TZ-safe @swp/shared layer.
function currentMonthLabel(): string {
  return formatDate(currentJakartaIso(), { month: 'long', year: 'numeric' });
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function AgentPengajuanScreen() {
  const { t } = useTranslation('agent');
  const [view, setView] = useState<PengajuanView>('dashboard');

  // Modal open state is hoisted so the page-level CTAs (in the actions slot) can trigger them.
  const [leaveCreateOpen, setLeaveCreateOpen] = useState(false);
  const [overtimeCreateOpen, setOvertimeCreateOpen] = useState(false);

  // ---- Data ----
  const dashQ = useGetMyDashboard();
  const leaveQ = useListLeaveRequests({ limit: 20 });
  const overtimeQ = useListOvertime({ limit: 20 });

  // Dashboard unwrap (double envelope — cf. me-kehadiran-screen.tsx).
  const outer = (dashQ.data as { data?: { data?: Dashboard } | Dashboard } | undefined)?.data;
  const dash = ((outer as { data?: Dashboard })?.data ?? outer) as AgentDashboard | undefined;

  const leaveBody = leaveQ.data?.data as { data?: LeaveRequest[] } | undefined;
  const leaveItems: LeaveRequest[] = leaveBody?.data ?? [];
  const overtimePage = overtimeQ.data?.data as OvertimePage | undefined;
  const overtimeItems: Overtime[] = overtimePage?.data ?? [];

  const newButtons = (
    <>
      <Button variant="secondary" size="sm" onClick={() => setLeaveCreateOpen(true)}>
        {t('leaveNewBtn')}
      </Button>
      <Button variant="primary" size="sm" onClick={() => setOvertimeCreateOpen(true)}>
        {t('otNewBtn')}
      </Button>
    </>
  );

  // ---- Full-list views (the "Lihat semua" targets) ----
  if (view === 'leave') {
    return (
      <AgentPage
        title={t('leaveTitle')}
        actions={
          <Button variant="ghost" size="sm" onClick={() => setView('dashboard')}>
            <ArrowLeft size={16} aria-hidden />
            {t('back')}
          </Button>
        }
      >
        <LeavePanel
          query={leaveQ}
          items={leaveItems}
          createOpen={leaveCreateOpen}
          onCreateOpenChange={setLeaveCreateOpen}
        />
      </AgentPage>
    );
  }

  if (view === 'overtime') {
    return (
      <AgentPage
        title={t('otTitle')}
        actions={
          <Button variant="ghost" size="sm" onClick={() => setView('dashboard')}>
            <ArrowLeft size={16} aria-hidden />
            {t('back')}
          </Button>
        }
      >
        <OvertimePanel
          query={overtimeQ}
          items={overtimeItems}
          createOpen={overtimeCreateOpen}
          onCreateOpenChange={setOvertimeCreateOpen}
        />
      </AgentPage>
    );
  }

  // ---- Dashboard view ----
  const pendingTotal = dash ? dash.pending_requests.leave + dash.pending_requests.ot : 0;

  return (
    <AgentPage title={t('pengajuanHeading')} actions={newButtons}>
      {/* 1 · Stat cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard
          label={t('statSisaCuti')}
          value={
            dash
              ? `${dash.leave_balance.annual_remaining_days}/${dash.leave_balance.annual_quota_days}`
              : '—'
          }
          sub={dash?.leave_balance.period_label}
          icon={Plane}
          tone="info"
        />
        <StatCard
          label={t('dashOtMonth')}
          value={dash ? t('dashHours', { count: dash.ot_this_month_hours }) : '—'}
          sub={currentMonthLabel()}
          icon={Timer}
          tone="warn"
        />
        <StatCard
          label={t('statPending')}
          value={dash ? String(pendingTotal) : '—'}
          sub={t('statPendingSub')}
          icon={Clock}
          tone={pendingTotal > 0 ? 'warn' : 'neutral'}
        />
      </div>

      {/* 2 · Two side-by-side recent-activity panels */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <RecentLeavePanel query={leaveQ} items={leaveItems} onViewAll={() => setView('leave')} />
        <RecentOvertimePanel
          query={overtimeQ}
          items={overtimeItems}
          onViewAll={() => setView('overtime')}
        />
      </div>

      {leaveCreateOpen && (
        <AgentLeaveCreateModal
          open
          onOpenChange={setLeaveCreateOpen}
          onSuccess={() => void leaveQ.refetch()}
        />
      )}
      {overtimeCreateOpen && (
        <AgentOvertimeCreateModal
          open
          onOpenChange={setOvertimeCreateOpen}
          onSuccess={() => void overtimeQ.refetch()}
        />
      )}
    </AgentPage>
  );
}

// ---------------------------------------------------------------------------
// Shared panel chrome (dashboard cards)
// ---------------------------------------------------------------------------

function PanelShell({
  title,
  onViewAll,
  children,
}: {
  title: string;
  onViewAll: () => void;
  children: React.ReactNode;
}) {
  const { t } = useTranslation('agent');
  return (
    <section className="rounded-lg border border-border bg-surface p-5">
      <div className="flex items-center justify-between">
        <h2 className="text-[15px] font-bold text-text">{title}</h2>
        <button
          type="button"
          onClick={onViewAll}
          className="flex items-center gap-1 text-[13px] font-medium text-primary"
        >
          {t('viewAll')}
          <ArrowRight size={14} aria-hidden />
        </button>
      </div>
      <div className="mt-2">{children}</div>
    </section>
  );
}

function SkeletonRows() {
  return (
    <div className="flex flex-col">
      {[0, 1, 2].map((i) => (
        <div
          key={i}
          className="flex items-center justify-between gap-3 border-b border-border py-3 last:border-0"
        >
          <div className="flex flex-1 flex-col gap-1.5">
            <span className="h-3.5 w-1/3 animate-pulse rounded bg-surface-2" />
            <span className="h-3 w-1/2 animate-pulse rounded bg-surface-2" />
          </div>
          <span className="h-5 w-16 animate-pulse rounded-full bg-surface-2" />
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Recent leave panel (dashboard)
// ---------------------------------------------------------------------------

type LeaveQuery = ReturnType<typeof useListLeaveRequests>;
type OvertimeQuery = ReturnType<typeof useListOvertime>;

function RecentLeavePanel({
  query,
  items,
  onViewAll,
}: {
  query: LeaveQuery;
  items: LeaveRequest[];
  onViewAll: () => void;
}) {
  const { t } = useTranslation('agent');
  const [selected, setSelected] = useState<LeaveRequest | null>(null);
  const recent = items.slice(0, 5);

  return (
    <PanelShell title={t('panelCuti')} onViewAll={onViewAll}>
      {query.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void query.refetch()} />
      ) : query.isLoading ? (
        <SkeletonRows />
      ) : recent.length === 0 ? (
        <EmptyState
          variant="fresh"
          icon={Plane}
          title={t('leaveEmpty')}
          description={t('leaveEmptyHint')}
        />
      ) : (
        <div className="flex flex-col">
          {recent.map((r) => {
            const days = r.duration_days ?? inclusiveDays(r.start_date, r.end_date);
            return (
              <button
                key={r.id}
                type="button"
                onClick={() => setSelected(r)}
                className="flex items-center justify-between gap-3 border-b border-border py-3 text-left last:border-0 hover:bg-surface-2"
              >
                <div className="flex min-w-0 flex-col">
                  <span className="truncate text-[14px] font-medium text-text">
                    {r.leave_type_name ?? r.leave_type_id}
                  </span>
                  <span className="text-[13px] tabular-nums text-text-2">
                    {formatDate(r.start_date)} – {formatDate(r.end_date)} ·{' '}
                    {t('durationDays', { count: days })}
                  </span>
                </div>
                <StatusBadge dot tone={leaveStatusTone(r.status as LeaveStatus)}>
                  {t(`leave.status.${r.status}`, { defaultValue: r.status })}
                </StatusBadge>
              </button>
            );
          })}
        </div>
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
    </PanelShell>
  );
}

// ---------------------------------------------------------------------------
// Recent overtime panel (dashboard)
// ---------------------------------------------------------------------------

function RecentOvertimePanel({
  query,
  items,
  onViewAll,
}: {
  query: OvertimeQuery;
  items: Overtime[];
  onViewAll: () => void;
}) {
  const { t } = useTranslation('agent');
  const { t: tOt } = useTranslation('overtime');
  const recent = items.slice(0, 5);

  return (
    <PanelShell title={t('panelLembur')} onViewAll={onViewAll}>
      {query.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void query.refetch()} />
      ) : query.isLoading ? (
        <SkeletonRows />
      ) : recent.length === 0 ? (
        <EmptyState
          variant="fresh"
          icon={Timer}
          title={t('otEmpty')}
          description={t('otEmptyHint')}
        />
      ) : (
        <div className="flex flex-col">
          {recent.map((r) => {
            const hours = hoursBetween(r.planned_start_time, r.planned_end_time);
            return (
              <div
                key={r.id}
                className="flex items-center justify-between gap-3 border-b border-border py-3 last:border-0"
              >
                <div className="flex min-w-0 flex-col">
                  <span className="truncate text-[14px] font-medium text-text">{t('otTitle')}</span>
                  <span className="text-[13px] tabular-nums text-text-2">
                    {formatDate(r.work_date)} · {t('dashHours', { count: hours })}
                  </span>
                </div>
                <StatusBadge dot tone={overtimeStatusTone(r.status)}>
                  {tOt(`status.${r.status}`, { defaultValue: r.status })}
                </StatusBadge>
              </div>
            );
          })}
        </div>
      )}
    </PanelShell>
  );
}

// ---------------------------------------------------------------------------
// Leave panel (full DataTable — "Lihat semua" view)
// ---------------------------------------------------------------------------

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
      width: 200,
      cell: (r) => (
        <span className="font-medium text-text">{r.leave_type_name ?? r.leave_type_id}</span>
      ),
    },
    {
      id: 'dateRange',
      header: t('leaveDateRange'),
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
      width: 160,
      cell: (r) => (
        <StatusBadge dot tone={leaveStatusTone(r.status as LeaveStatus)}>
          {t(`leave.status.${r.status}`, { defaultValue: r.status })}
        </StatusBadge>
      ),
    },
  ];

  return (
    <>
      {query.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void query.refetch()} />
      ) : (
        <DataTable
          aria-label={t('leaveTitle')}
          columns={columns}
          data={items}
          getRowId={(r) => r.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          onRowClick={(r) => setSelected(r)}
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
// Overtime panel (full DataTable — "Lihat semua" view, incl. confirm/withdraw)
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
      width: 140,
      cell: (r) => <span className="font-medium text-text">{formatDate(r.work_date)}</span>,
    },
    {
      id: 'time',
      header: t('otTimeRange'),
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
