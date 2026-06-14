/**
 * E10 · Dashboard — role-branched (HR Admin / Super Admin / Shift Leader)
 *
 * .pen frames:
 *   ETi5H  E10 · Dashboard (HR)
 *   DhzyL  E10 · Dashboard (Super Admin) — same shape as HR + admin widgets below (F10.2 DB-7)
 *   RiSPW  E10 SL · Dashboard Tim
 *   biFs5  State · Approval inbox empty
 *   elJj3  State · Approval inbox filtered-zero
 *
 * Design: TitleBand → 4× StatCards → [Chart | ApprovalInboxPanel] → BillableTrend/7-day bar.
 * HR/Super-Admin: cross-company KPIs + billable bar chart.
 * Super Admin only: admin widgets section (DB-7) rendered below the HR cockpit when
 *   `role === 'super_admin'` AND `data.admin` is present; hr_admin never sees it (C-6).
 * SL: scoped team summary (today clock-in/late/absent/pending) + schedule alerts sidebar.
 * Agent: mobile-only — renders a minimal fallback.
 *
 * ENGINEERING.md D1 — role label comes from data.role_label; no client-hardcoding.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type Dashboard,
  type HrDashboard,
  HrDashboardRole,
  type LeaderDashboard,
  LeaderDashboardRole,
  LeaderDashboardScheduleAlertsItemKind,
  type SuperAdminWidgets,
  useGetMyDashboard,
} from '@swp/api-client/e10';
import { EmptyState, SkeletonCard, StatCard, StateView } from '@swp/ui';
import { Link } from '@tanstack/react-router';
import {
  AlertTriangle,
  BarChart3,
  Building2,
  CalendarClock,
  CheckCircle2,
  Clock,
  ClockAlert,
  MapPin,
  ShieldAlert,
  UserCheck,
  UserMinus,
  UserPlus,
  Users,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ApprovalInboxPanel } from './approval-inbox-panel.tsx';
import { LeaderCompliancePanel } from './leader-compliance-panel.tsx';

// ---------------------------------------------------------------------------
// Narrow helpers
// ---------------------------------------------------------------------------

function isHrDashboard(d: Dashboard): d is HrDashboard {
  return d.role === HrDashboardRole.hr_admin || d.role === HrDashboardRole.super_admin;
}

function isLeaderDashboard(d: Dashboard): d is LeaderDashboard {
  return d.role === LeaderDashboardRole.shift_leader;
}

// ---------------------------------------------------------------------------
// Schedule alert kind → icon/tone map
// ---------------------------------------------------------------------------

function scheduleAlertIcon(kind: LeaderDashboardScheduleAlertsItemKind) {
  switch (kind) {
    case LeaderDashboardScheduleAlertsItemKind.COVERAGE_GAP:
      return { Icon: Users, className: 'text-bad-tx' };
    case LeaderDashboardScheduleAlertsItemKind.UNASSIGNED_SHIFT:
      return { Icon: CalendarClock, className: 'text-warn-tx' };
    case LeaderDashboardScheduleAlertsItemKind.PLACEMENT_EXPIRING:
      return { Icon: MapPin, className: 'text-warn-tx' };
    default:
      return { Icon: AlertTriangle, className: 'text-text-3' };
  }
}

// ---------------------------------------------------------------------------
// BillableTrendBar — 7-day bar chart (HR/Super-Admin only, frame "Trend")
// ---------------------------------------------------------------------------

interface TrendBarProps {
  label: string;
  value: number;
  maxValue: number;
}

function TrendBar({ label, value, maxValue }: TrendBarProps) {
  const heightPct = maxValue > 0 ? Math.round((value / maxValue) * 100) : 0;
  // Map 0-100% pct → 0-126px (inner plot area ≈130px, leave headroom for label)
  const barHeightPx = Math.max(4, Math.round((heightPct / 100) * 126));

  return (
    <div className="flex h-full flex-1 flex-col items-center justify-end gap-[6px]">
      <span className="text-[11px] font-semibold text-text">{value > 0 ? String(value) : ''}</span>
      <div
        className="w-full rounded-t-lg bg-primary"
        style={{ height: barHeightPx }}
        role="presentation"
      />
      <span className="text-[11px] font-medium text-text-3">{label}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Admin widgets section (F10.2 DB-7, frame DhzyL)
// Gated: super_admin role + data.admin present. hr_admin payload omits `admin` (C-6).
// ---------------------------------------------------------------------------

/** Format an ISO timestamp into a short Jakarta-tz absolute label (e.g. "17 Jun 14:32").
 *  Mirrors the same pattern used in notifications-screen.tsx. */
function formatJakartaShort(iso: string): string {
  try {
    return new Intl.DateTimeFormat('id-ID', {
      timeZone: 'Asia/Jakarta',
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

function AdminWidgetsSection({ admin }: { admin: SuperAdminWidgets }) {
  const { t } = useTranslation('dashboard');

  return (
    <div className="flex flex-col gap-4">
      {/* Section header */}
      <div className="flex items-center gap-2">
        <ShieldAlert aria-hidden className="size-4 text-text-3" />
        <span className="text-[13px] font-semibold uppercase tracking-wide text-text-3">
          {t('admin.sectionTitle')}
        </span>
      </div>

      {/* Widget row: 2-up on top, 2-up on bottom */}
      <div className="grid grid-cols-2 gap-4">
        {/* Widget 1 — Pengguna & Akses (user_access) */}
        <div className="flex flex-col gap-[14px] rounded-xl border border-border bg-surface p-[18px]">
          <div className="flex items-center justify-between">
            <span className="text-[15px] font-bold text-text">{t('admin.userAccess.title')}</span>
            <Link
              to="/settings/users"
              className="text-[12px] font-semibold text-primary hover:underline"
            >
              {t('admin.userAccess.cta')}
            </Link>
          </div>
          <div className="grid grid-cols-3 gap-3">
            <StatCard
              label={t('admin.userAccess.activeUsers')}
              value={String(admin.user_access.active_users)}
              sub={t('admin.userAccess.activeUsersSub')}
              icon={UserCheck}
              tone="ok"
            />
            <StatCard
              label={t('admin.userAccess.pendingProvisioning')}
              value={String(admin.user_access.pending_provisioning)}
              sub={t('admin.userAccess.pendingProvisioningSub')}
              icon={UserPlus}
              tone={admin.user_access.pending_provisioning > 0 ? 'warn' : 'neutral'}
            />
            <StatCard
              label={t('admin.userAccess.offboarded30d')}
              value={String(admin.user_access.offboarded_30d)}
              sub={t('admin.userAccess.offboarded30dSub')}
              icon={UserMinus}
              tone="neutral"
            />
          </div>
        </div>

        {/* Widget 2 — Aktivitas Audit Terbaru (recent_audit) */}
        <div className="flex flex-col gap-[14px] rounded-xl border border-border bg-surface p-[18px]">
          <div className="flex items-center justify-between">
            <span className="text-[15px] font-bold text-text">{t('admin.recentAudit.title')}</span>
            {/* TODO: link to /settings/audit-log once a dedicated audit route is confirmed */}
            <Link
              to="/settings/audit-log"
              className="text-[12px] font-semibold text-primary hover:underline"
            >
              {t('admin.recentAudit.cta')}
            </Link>
          </div>
          {admin.recent_audit.length === 0 ? (
            <EmptyState
              variant="fresh"
              title={t('admin.recentAudit.emptyTitle')}
              description={t('admin.recentAudit.emptyBody')}
            />
          ) : (
            <div className="flex flex-col divide-y divide-border-soft">
              {admin.recent_audit.map((entry) => (
                <div key={entry.id} className="flex items-start justify-between gap-3 py-[10px]">
                  <div className="flex min-w-0 flex-col gap-[2px]">
                    <span className="text-[13px] font-medium text-text truncate">
                      {entry.actor_label}
                      <span className="mx-1 font-normal text-text-3">·</span>
                      <span className="font-mono text-[12px] text-text-2">{entry.action}</span>
                      <span className="mx-1 font-normal text-text-3">·</span>
                      {entry.target_label}
                    </span>
                  </div>
                  <span className="shrink-0 text-[11px] text-text-3 tabular-nums">
                    {formatJakartaShort(entry.at)}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Widget 3 — Rekap per Posisi (org_rollups, grouped by free-text position) */}
        <div className="flex flex-col gap-[14px] rounded-xl border border-border bg-surface p-[18px]">
          <div className="flex items-center gap-2">
            <Building2 aria-hidden className="size-4 text-text-3" />
            <span className="text-[15px] font-bold text-text">{t('admin.orgRollups.title')}</span>
          </div>
          {admin.org_rollups.length === 0 ? (
            <EmptyState variant="default" title={t('admin.orgRollups.emptyTitle')} />
          ) : (
            <table className="w-full text-[13px]">
              <thead>
                <tr className="border-b border-border-soft">
                  <th className="pb-[8px] text-left text-[11px] font-semibold uppercase tracking-wide text-text-3">
                    {t('admin.orgRollups.colPosition')}
                  </th>
                  <th className="pb-[8px] text-right text-[11px] font-semibold uppercase tracking-wide text-text-3">
                    {t('admin.orgRollups.colHeadcount')}
                  </th>
                  <th className="pb-[8px] text-right text-[11px] font-semibold uppercase tracking-wide text-text-3">
                    {t('admin.orgRollups.colPlacements')}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-soft">
                {admin.org_rollups.map((row) => (
                  <tr key={row.position}>
                    <td className="py-[10px] font-medium text-text">{row.position}</td>
                    <td className="py-[10px] text-right tabular-nums text-text-2">
                      {row.headcount}
                    </td>
                    <td className="py-[10px] text-right tabular-nums text-text-2">
                      {row.active_placements}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Widget 4 — Persetujuan Tertunda (pending_grants).
            Bank/profile change-request approvals removed (E11, 2026-06-14) — agent self-edits are
            now instant. Only role-change requests remain; leave/overtime approvals live in /inbox. */}
        <div className="flex flex-col gap-[14px] rounded-xl border border-border bg-surface p-[18px]">
          <span className="text-[15px] font-bold text-text">{t('admin.pendingGrants.title')}</span>
          <div className="grid grid-cols-1 gap-3">
            <div className="flex flex-col gap-3">
              <StatCard
                label={t('admin.pendingGrants.roleRequests')}
                value={String(admin.pending_grants.role_requests)}
                sub={t('admin.pendingGrants.roleRequestsSub')}
                icon={Users}
                tone={admin.pending_grants.role_requests > 0 ? 'info' : 'neutral'}
              />
              <Link
                to="/settings/users"
                className="text-[12px] font-semibold text-primary hover:underline"
              >
                {t('admin.pendingGrants.ctaRoles')} →
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// HR / Super-Admin dashboard
// ---------------------------------------------------------------------------

function HrDashboardView({ data }: { data: HrDashboard }) {
  const { t } = useTranslation('dashboard');
  const user = useCurrentUser();
  // DB widget deep-link (no dead-flow): only show the report CTA to roles with reports.read.
  const canReadReports = user?.permissions.includes('reports.read') ?? false;

  const kpis = data.kpis;

  // Billable trend bar chart
  const trendPoints = data.billable_trend.points;
  const maxTrend = trendPoints.reduce((m, p) => Math.max(m, p.value), 0);

  // Approvals panel: no kind-filter on this screen (filtering is in the full queue)
  const panelRows = data.pending_approvals_panel;

  // Format large numbers: 8420 → "8.420"
  function fmt(n: number): string {
    return n.toLocaleString('id-ID');
  }

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band */}
      <div className="flex items-center justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-[30px] font-bold text-text">{t('title')}</h1>
          <p className="text-[14px] text-text-3">
            {t('hrSubtitle', { period: data.period_label })}
          </p>
        </div>
        <div className="flex items-center gap-[10px]">
          <span className="text-[12px] text-text-3">{data.role_label}</span>
          {canReadReports && (
            <Link
              to="/reports"
              className="flex items-center gap-2 rounded-lg bg-primary px-[14px] py-[9px] text-[13px] font-semibold text-white hover:bg-primary-strong"
            >
              <BarChart3 aria-hidden className="size-4" />
              {t('makeReport')}
            </Link>
          )}
        </div>
      </div>

      {/* KPI stat cards */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard
          label={t('kpi.attendanceRate')}
          value={`${kpis.attendance_rate_pct}%`}
          sub={t('kpi.attendanceRateSub')}
          icon={CheckCircle2}
          tone="ok"
        />
        <StatCard
          label={t('kpi.billableHours')}
          value={`${fmt(kpis.billable_hours_mtd)} j`}
          sub={t('kpi.billableHoursSub')}
          icon={Clock}
          tone="brand"
        />
        <StatCard
          label={t('kpi.otHours')}
          value={`${fmt(kpis.ot_hours_mtd)} j`}
          sub={t('kpi.otHoursSub')}
          icon={ClockAlert}
          tone="warn"
        />
        <StatCard
          label={t('kpi.activePlacements')}
          value={String(kpis.active_placements)}
          sub={t('kpi.activePlacementsSub', { count: kpis.active_companies })}
          icon={Users}
          tone="info"
        />
      </div>

      {/* Alert chips: expiring placements / agreements / attendance anomalies */}
      {(data.expiring_placements_30d > 0 ||
        data.expiring_agreements_30d > 0 ||
        data.attendance_anomalies_today > 0) && (
        <div className="flex flex-wrap gap-2">
          {data.expiring_placements_30d > 0 && (
            <div className="flex items-center gap-[6px] rounded-full border border-warn-bd bg-warn-bg px-[10px] py-[4px]">
              <MapPin aria-hidden className="size-[12px] text-warn-tx" />
              <span className="text-[12px] font-semibold text-warn-tx">
                {t('alert.expiringPlacements', { count: data.expiring_placements_30d })}
              </span>
            </div>
          )}
          {data.expiring_agreements_30d > 0 && (
            <div className="flex items-center gap-[6px] rounded-full border border-warn-bd bg-warn-bg px-[10px] py-[4px]">
              <CalendarClock aria-hidden className="size-[12px] text-warn-tx" />
              <span className="text-[12px] font-semibold text-warn-tx">
                {t('alert.expiringAgreements', { count: data.expiring_agreements_30d })}
              </span>
            </div>
          )}
          {data.attendance_anomalies_today > 0 && (
            <div className="flex items-center gap-[6px] rounded-full border border-bad-bd bg-bad-bg px-[10px] py-[4px]">
              <AlertTriangle aria-hidden className="size-[12px] text-bad-tx" />
              <span className="text-[12px] font-semibold text-bad-tx">
                {t('alert.attendanceAnomalies', { count: data.attendance_anomalies_today })}
              </span>
            </div>
          )}
        </div>
      )}

      {/* Chart row: billable-by-line chart + approval inbox panel */}
      <div className="flex gap-4">
        {/* Billable bar chart (left, fill) */}
        <div className="flex min-w-0 flex-1 flex-col gap-4 overflow-hidden rounded-xl border border-border bg-surface p-[18px]">
          <div className="flex items-center justify-between">
            <span className="text-[15px] font-bold text-text">{t('chart.billableTitle')}</span>
            <span className="text-[12px] text-text-3">
              {data.period_label} · {t('chart.billableUnit')}
            </span>
          </div>
          {trendPoints.length === 0 ? (
            <div className="flex h-[200px] items-center justify-center">
              <EmptyState
                variant="default"
                title={t('chart.emptyTitle')}
                description={t('chart.emptyBody')}
              />
            </div>
          ) : (
            <div
              className="flex h-[200px] items-end justify-around gap-6 px-[10px] pb-0 pt-[10px]"
              role="img"
              aria-label={t('chart.billableTitle')}
            >
              {trendPoints.map((pt) => (
                <TrendBar
                  key={pt.date}
                  label={new Date(pt.date).toLocaleDateString('id-ID', {
                    month: 'short',
                    day: 'numeric',
                    timeZone: 'Asia/Jakarta',
                  })}
                  value={pt.value}
                  maxValue={maxTrend}
                />
              ))}
            </div>
          )}
        </div>

        {/* Approval inbox panel (right, fixed width 392) */}
        <div className="w-[392px] shrink-0">
          <ApprovalInboxPanel rows={panelRows} />
        </div>
      </div>

      {/* Attendance trend — 7-day bar */}
      <div className="flex flex-col gap-4 rounded-xl border border-border bg-surface p-[18px]">
        <span className="text-[15px] font-bold text-text">{t('trend.title7d')}</span>
        {/* This is a static visual hint only; live data requires a separate E5 query */}
        <div className="flex h-[130px] items-end justify-around gap-[18px] px-[6px]">
          {['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min'].map((day) => (
            <div
              key={day}
              className="flex flex-1 flex-col items-center justify-end gap-[6px] h-full"
            >
              <div className="w-full rounded-t-lg bg-primary opacity-80" style={{ height: 90 }} />
              <span className="text-[11px] font-medium text-text-3">{day}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Admin widgets (DB-7, frame DhzyL) — super_admin only; hr_admin payload omits admin (C-6) */}
      {data.role === HrDashboardRole.super_admin && data.admin && (
        <AdminWidgetsSection admin={data.admin} />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Shift-Leader dashboard
// ---------------------------------------------------------------------------

function LeaderDashboardView({ data }: { data: LeaderDashboard }) {
  const { t } = useTranslation('dashboard');
  const today = data.today;
  const pending = data.pending_counts;
  const alerts = data.schedule_alerts;
  const panelRows = data.pending_approvals_panel;

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band */}
      <div className="flex items-center justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-[30px] font-bold text-text">{t('title')}</h1>
          <p className="text-[14px] text-text-3">
            {data.company.name} · {t('slSubtitle')}
          </p>
        </div>
        <span className="rounded-lg border border-border bg-surface px-[12px] py-[6px] text-[13px] font-medium text-text-2">
          {data.role_label}
        </span>
      </div>

      {/* Today summary stat cards */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard
          label={t('sl.todayTotal')}
          value={String(today.shifts_total)}
          sub={t('sl.todayTotalSub')}
          icon={Users}
          tone="brand"
        />
        <StatCard
          label={t('sl.clockedIn')}
          value={String(today.clocked_in)}
          sub={t('sl.clockedInSub')}
          icon={CheckCircle2}
          tone="ok"
        />
        <StatCard
          label={t('sl.lateCount')}
          value={String(today.late_count)}
          sub={t('sl.lateCountSub')}
          icon={ClockAlert}
          tone="warn"
        />
        <StatCard
          label={t('sl.absent')}
          value={String(today.absent_count)}
          sub={t('sl.absentSub')}
          icon={AlertTriangle}
          tone="bad"
        />
      </div>

      {/* Pending counts chips */}
      {(pending.attendance_verify > 0 || pending.leave_approve > 0 || pending.ot_approve > 0) && (
        <div className="flex flex-wrap gap-2">
          {pending.attendance_verify > 0 && (
            <div className="flex items-center gap-[6px] rounded-full border border-bad-bd bg-bad-bg px-[10px] py-[4px]">
              <span className="text-[12px] font-semibold text-bad-tx">
                {t('sl.pendingVerify', { count: pending.attendance_verify })}
              </span>
            </div>
          )}
          {pending.leave_approve > 0 && (
            <div className="flex items-center gap-[6px] rounded-full border border-warn-bd bg-warn-bg px-[10px] py-[4px]">
              <span className="text-[12px] font-semibold text-warn-tx">
                {t('sl.pendingLeave', { count: pending.leave_approve })}
              </span>
            </div>
          )}
          {pending.ot_approve > 0 && (
            <div className="flex items-center gap-[6px] rounded-full border border-warn-bd bg-warn-bg px-[10px] py-[4px]">
              <span className="text-[12px] font-semibold text-warn-tx">
                {t('sl.pendingOt', { count: pending.ot_approve })}
              </span>
            </div>
          )}
        </div>
      )}

      {/* Main row: schedule alerts + approval inbox */}
      <div className="flex gap-4">
        {/* Schedule alerts (left, fill) */}
        <div className="flex min-w-0 flex-1 flex-col gap-4 overflow-hidden rounded-xl border border-border bg-surface p-[18px]">
          <span className="text-[15px] font-bold text-text">{t('sl.scheduleAlerts')}</span>
          {alerts.length === 0 ? (
            <EmptyState
              variant="fresh"
              title={t('sl.noAlerts')}
              description={t('sl.noAlertsSub')}
            />
          ) : (
            <div className="flex flex-col divide-y divide-border-soft">
              {alerts.map((alert) => {
                const { Icon, className: iconClass } = scheduleAlertIcon(alert.kind);
                return (
                  <div
                    key={`${alert.kind}-${alert.date ?? alert.label}`}
                    className="flex items-start gap-[10px] py-[12px]"
                  >
                    <Icon aria-hidden className={['size-4 mt-0.5 shrink-0', iconClass].join(' ')} />
                    <div className="flex flex-col gap-[2px]">
                      <span className="text-[13px] font-medium text-text">{alert.label}</span>
                      {alert.date && (
                        <span className="text-[11px] text-text-3">
                          {new Date(alert.date).toLocaleDateString('id-ID', {
                            timeZone: 'Asia/Jakarta',
                            weekday: 'short',
                            month: 'short',
                            day: 'numeric',
                          })}
                        </span>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Approval inbox panel (right, fixed width 392) */}
        <div className="w-[392px] shrink-0">
          <ApprovalInboxPanel rows={panelRows} />
        </div>
      </div>

      {/* Roster-compliance (EPICS §8 D3) — this week's over-scheduling + holiday shifts */}
      <LeaderCompliancePanel companyId={data.company.id} />

      {/* Attendance trend 7d */}
      <div className="flex flex-col gap-4 rounded-xl border border-border bg-surface p-[18px]">
        <span className="text-[15px] font-bold text-text">{t('sl.trend7d')}</span>
        <div className="flex h-[130px] items-end justify-around gap-[18px] px-[6px]">
          {['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min'].map((day) => (
            <div
              key={day}
              className="flex flex-1 flex-col items-center justify-end gap-[6px] h-full"
            >
              <div className="w-full rounded-t-lg bg-primary opacity-80" style={{ height: 90 }} />
              <span className="text-[11px] font-medium text-text-3">{day}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Agent fallback (mobile-only; web renders a simple redirect hint)
// ---------------------------------------------------------------------------

function AgentFallback() {
  const { t } = useTranslation('dashboard');
  return (
    <div className="flex items-center justify-center py-24">
      <EmptyState variant="no-permission" title={t('agent.title')} description={t('agent.body')} />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Loading skeleton
// ---------------------------------------------------------------------------

function DashboardSkeleton() {
  return (
    <div className="flex flex-col gap-[18px]">
      <div className="h-[72px] animate-pulse rounded-xl bg-surface" />
      <div className="grid grid-cols-4 gap-4">
        {[1, 2, 3, 4].map((i) => (
          <SkeletonCard key={i} />
        ))}
      </div>
      <div className="flex gap-4">
        <div className="flex-1 h-[320px] animate-pulse rounded-xl bg-surface" />
        <div className="w-[392px] h-[320px] animate-pulse rounded-xl bg-surface" />
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// DashboardScreen — public export
// ---------------------------------------------------------------------------

export function DashboardScreen() {
  const { t } = useTranslation('dashboard');
  // State for any future filter (currently no kind-filter UI on the dashboard itself;
  // included to keep the panel's filterActive prop satisfied)
  const [_activeFilter] = useState(null);

  const query = useGetMyDashboard();

  // ----- Loading -----
  if (query.isLoading) {
    return <DashboardSkeleton />;
  }

  // ----- Error -----
  if (query.isError) {
    const err = classifyError(query.error);
    if (err.kind === 'forbidden' || err.kind === 'unauthenticated') {
      return (
        <StateView
          kind="no-permission"
          title={t('errors.noPermission')}
          description={t('errors.noPermissionBody')}
        />
      );
    }
    return (
      <StateView
        kind="error"
        title={t('errors.loadError')}
        description={t('errors.network')}
        onRetry={() => void query.refetch()}
        retryLabel={t('errors.retry')}
      />
    );
  }

  // ----- No data -----
  if (!query.data) {
    return <StateView kind="empty" title={t('errors.noData')} />;
  }

  // Unwrap BOTH envelopes: Orval's customFetch wraps the HTTP body in { data, status,
  // headers }, and the BE handler wraps the Dashboard in { data: <Dashboard> } (dataResponse,
  // like every epic) even though the E10 openapi declares the bare Dashboard. So the real
  // payload lives at query.data.data.data — fixed toward what the BE returns (the recurring
  // {data}-unwrap finding; cf. [08-04]/[10-04] precedents). A bare fallback keeps it robust
  // if the envelope ever flattens.
  const outer = (query.data as { data?: { data?: Dashboard } | Dashboard }).data;
  const body = ((outer as { data?: Dashboard })?.data ?? outer) as Dashboard;

  // ----- Role branching -----
  if (isHrDashboard(body)) {
    return <HrDashboardView data={body} />;
  }

  if (isLeaderDashboard(body)) {
    return <LeaderDashboardView data={body} />;
  }

  // Agent role — mobile-only
  return <AgentFallback />;
}
