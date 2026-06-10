import { useCurrentUser } from '@/lib/use-auth.ts';
/**
 * /me — Agent personal dashboard (F10.2 AgentDashboard shape).
 *
 * Web port of apps/mobile/app/(app)/index.tsx. Shows:
 *   - Greeting with the agent's name (useCurrentUser)
 *   - Today's shift (E4 via AgentDashboard.today_shift)
 *   - OT hours this month (E7 via AgentDashboard.ot_this_month_hours)
 *   - Unread notification count (F10.1 via AgentDashboard.recent_notifications_unread)
 *   - Quick-action cards linking to /me/attendance, /me/schedule, /me/leave, /me/overtime
 *
 * Data: GET /dashboards/me → role-shaped Dashboard; agent branch → AgentDashboard.
 * States: loading / error+retry / content (no dead flows — DB C-1, C-3).
 */
import {
  type AgentDashboard,
  AgentDashboardTodayShiftClockInStatus,
  type AgentDashboardTodayShiftClockInStatus as ClockStatus,
  type Dashboard,
  useGetMyDashboard,
} from '@swp/api-client/e10';
import type { StatusTone } from '@swp/design-tokens';
import { Button, StateView, StatusBadge } from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { Bell, Calendar, Clock, Fingerprint, Timer } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { AgentPage } from './agent-page.tsx';

// ─── Helpers ──────────────────────────────────────────────────────────────────

function clockStatusTone(status: ClockStatus): StatusTone {
  switch (status) {
    case AgentDashboardTodayShiftClockInStatus.CLOCKED_IN:
      return 'ok';
    case AgentDashboardTodayShiftClockInStatus.CLOCKED_OUT:
      return 'neutral';
    case AgentDashboardTodayShiftClockInStatus.LATE:
      return 'warn';
    case AgentDashboardTodayShiftClockInStatus.ABSENT:
      return 'bad';
    default:
      return 'neutral';
  }
}

function clockStatusLabel(status: ClockStatus): string {
  switch (status) {
    case AgentDashboardTodayShiftClockInStatus.CLOCKED_IN:
      return 'Sudah Masuk';
    case AgentDashboardTodayShiftClockInStatus.CLOCKED_OUT:
      return 'Sudah Keluar';
    case AgentDashboardTodayShiftClockInStatus.LATE:
      return 'Terlambat';
    case AgentDashboardTodayShiftClockInStatus.ABSENT:
      return 'Tidak Hadir';
    default:
      return 'Belum Absen';
  }
}

// ─── Today-shift card ────────────────────────────────────────────────────────

function TodayShiftCard({ data }: { data: AgentDashboard }) {
  const { t } = useTranslation('agent');
  const shift = data.today_shift;

  return (
    <div className="rounded-xl border border-border bg-surface p-5">
      <div className="flex items-center gap-2 text-text-3">
        <Clock size={15} aria-hidden />
        <span className="text-[12px] font-medium uppercase tracking-wide">
          {t('dashTodayShift')}
        </span>
      </div>
      {shift ? (
        <div className="mt-3 flex flex-col gap-1">
          <p className="text-[18px] font-bold text-text">{shift.shift_name}</p>
          <p className="text-[13px] text-text-2">
            {shift.start_time} – {shift.end_time} · {shift.company_name}
          </p>
          <div className="mt-2">
            <StatusBadge dot tone={clockStatusTone(shift.clock_in_status)}>
              {clockStatusLabel(shift.clock_in_status)}
            </StatusBadge>
          </div>
        </div>
      ) : (
        <p className="mt-3 text-[15px] text-text-2">{t('dashOffToday')}</p>
      )}
    </div>
  );
}

// ─── Stats row ────────────────────────────────────────────────────────────────

function StatsRow({ data }: { data: AgentDashboard }) {
  const { t } = useTranslation('agent');

  return (
    <div className="grid grid-cols-2 gap-3">
      {/* OT this month */}
      <div className="rounded-xl border border-border bg-surface p-5">
        <div className="flex items-center gap-2 text-text-3">
          <Timer size={15} aria-hidden />
          <span className="text-[12px] font-medium uppercase tracking-wide">
            {t('dashOtMonth')}
          </span>
        </div>
        <p className="mt-2 text-[22px] font-bold text-text">
          {t('dashHours', { count: data.ot_this_month_hours })}
        </p>
      </div>

      {/* Unread notifications */}
      <div className="rounded-xl border border-border bg-surface p-5">
        <div className="flex items-center gap-2 text-text-3">
          <Bell size={15} aria-hidden />
          <span className="text-[12px] font-medium uppercase tracking-wide">{t('notifTitle')}</span>
        </div>
        <div className="mt-2">
          {data.recent_notifications_unread > 0 ? (
            <StatusBadge dot tone="warn">
              {t('dashUnread', { count: data.recent_notifications_unread })}
            </StatusBadge>
          ) : (
            <p className="text-[15px] text-text-2">{t('empty')}</p>
          )}
        </div>
      </div>
    </div>
  );
}

// ─── Quick-action grid ───────────────────────────────────────────────────────

function QuickActions() {
  const { t } = useTranslation('agent');

  const links: Array<{
    to: '/me/attendance' | '/me/schedule' | '/me/leave' | '/me/overtime';
    label: string;
    Icon: React.ComponentType<{
      size?: number;
      className?: string;
      'aria-hidden'?: boolean | 'true';
    }>;
  }> = [
    { to: '/me/attendance', label: t('clockTitle'), Icon: Fingerprint },
    { to: '/me/schedule', label: t('scheduleTitle'), Icon: Calendar },
    { to: '/me/leave', label: t('leaveTitle'), Icon: Clock },
    { to: '/me/overtime', label: t('otTitle'), Icon: Timer },
  ];

  return (
    <div className="grid grid-cols-2 gap-3">
      {links.map(({ to, label, Icon }) => (
        <Link key={to} to={to}>
          <div className="flex flex-col items-center gap-2 rounded-xl border border-border bg-surface p-5 text-center transition-colors hover:bg-surface-hover">
            <Icon size={22} className="text-primary" aria-hidden />
            <span className="text-[13px] font-medium text-text">{label}</span>
          </div>
        </Link>
      ))}
    </div>
  );
}

// ─── Screen ───────────────────────────────────────────────────────────────────

import type React from 'react';

export function AgentDashboardScreen() {
  const { t } = useTranslation('agent');
  const user = useCurrentUser();
  const dash = useGetMyDashboard();

  const payload = dash.data?.data as Dashboard | undefined;
  const agentData: AgentDashboard | null =
    payload != null && 'role' in payload && payload.role === 'agent'
      ? (payload as AgentDashboard)
      : null;

  const greeting = t('dashGreeting', { name: user?.name ?? '' });

  return (
    <AgentPage title={greeting}>
      {dash.isLoading ? (
        <StateView kind="loading" title={t('loading')} />
      ) : dash.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void dash.refetch()} />
      ) : agentData !== null ? (
        <div className="flex flex-col gap-4">
          {/* Today's shift */}
          <TodayShiftCard data={agentData} />

          {/* OT hours + unread notifications */}
          <StatsRow data={agentData} />

          {/* Primary CTA — quick clock link (dashQuickClock) */}
          <Link to="/me/attendance">
            <Button variant="primary" className="w-full">
              {t('dashQuickClock')}
            </Button>
          </Link>

          {/* Quick-action grid: attendance / schedule / leave / overtime */}
          <QuickActions />
        </div>
      ) : (
        <StateView kind="empty" title={t('empty')} />
      )}
    </AgentPage>
  );
}
