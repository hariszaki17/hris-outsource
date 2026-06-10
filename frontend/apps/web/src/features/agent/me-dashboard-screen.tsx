/**
 * /me — Agent personal dashboard (web). Built to the console layout (cf.
 * features/e10-reporting/dashboard-screen.tsx): title band + 4 StatCards + a today's-shift
 * panel + quick-action links. Data: GET /dashboards/me (AgentDashboard shape).
 * docs/eng/AGENT-WEB-ACCESS.md.
 */
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type AgentDashboard,
  AgentDashboardTodayShiftClockInStatus as ClockStatus,
  type Dashboard,
  useGetMyDashboard,
} from '@swp/api-client/e10';
import type { StatusTone } from '@swp/design-tokens';
import { Button, SkeletonCard, StatCard, StateView, StatusBadge } from '@swp/ui';
import { Link } from '@tanstack/react-router';
import {
  Bell,
  CalendarClock,
  CheckCircle2,
  ClockAlert,
  Fingerprint,
  Plane,
  Timer,
} from 'lucide-react';
import type { ComponentType } from 'react';
import { useTranslation } from 'react-i18next';
import { AgentPage } from './agent-page.tsx';

function clockStatusTone(s: ClockStatus): StatusTone {
  switch (s) {
    case ClockStatus.CLOCKED_IN:
    case ClockStatus.CLOCKED_OUT:
      return 'ok';
    case ClockStatus.LATE:
      return 'warn';
    case ClockStatus.ABSENT:
      return 'bad';
    default:
      return 'neutral';
  }
}

export function AgentDashboardScreen() {
  const { t } = useTranslation('agent');
  const user = useCurrentUser();
  const query = useGetMyDashboard();
  const greeting = t('dashGreeting', { name: user?.name ?? '' });

  if (query.isLoading) {
    return (
      <AgentPage title={greeting}>
        <div className="grid grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      </AgentPage>
    );
  }
  if (query.isError) {
    return (
      <AgentPage title={greeting}>
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void query.refetch()} />
      </AgentPage>
    );
  }

  // Unwrap both envelopes: customFetch wraps the body in { data, ... } and the BE wraps the
  // payload in { data: <Dashboard> } (cf. dashboard-screen.tsx).
  const outer = (query.data as { data?: { data?: Dashboard } | Dashboard } | undefined)?.data;
  const d = ((outer as { data?: Dashboard })?.data ?? outer) as AgentDashboard | undefined;

  if (!d) {
    return (
      <AgentPage title={greeting}>
        <StateView kind="empty" title={t('empty')} />
      </AgentPage>
    );
  }

  const shift = d.today_shift;
  const hasShift = Boolean(shift?.schedule_id);
  const ra = d.recent_attendance;

  return (
    <AgentPage
      title={greeting}
      actions={
        <Button asChild variant="primary">
          <Link to="/me/attendance">
            <Fingerprint className="size-4" aria-hidden />
            {t('dashQuickClock')}
          </Link>
        </Button>
      }
    >
      {/* KPI stat cards */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard
          label={t('dashTodayShift')}
          value={hasShift ? `${shift?.start_time}–${shift?.end_time}` : t('dashOffToday')}
          sub={hasShift ? shift?.company_name : t('dashNoShift')}
          icon={CalendarClock}
          tone="brand"
        />
        <StatCard
          label={t('dashOtMonth')}
          value={t('dashHours', { count: d.ot_this_month_hours })}
          icon={Timer}
          tone="warn"
        />
        <StatCard
          label={t('leaveTitle')}
          value={`${d.leave_balance.annual_remaining_days}/${d.leave_balance.annual_quota_days}`}
          sub={d.leave_balance.period_label}
          icon={Plane}
          tone="info"
        />
        <StatCard
          label={t('notifTitle')}
          value={String(d.recent_notifications_unread)}
          sub={t('dashUnread', { count: d.recent_notifications_unread })}
          icon={Bell}
          tone={d.recent_notifications_unread > 0 ? 'warn' : 'neutral'}
        />
      </div>

      {/* Today's shift + recent attendance */}
      <div className="flex gap-4">
        <div className="flex min-w-0 flex-1 flex-col gap-4 rounded-xl border border-border bg-surface p-[18px]">
          <span className="text-[15px] font-bold text-text">{t('dashTodayShift')}</span>
          {hasShift && shift ? (
            <div className="flex items-center justify-between gap-4">
              <div className="flex flex-col gap-1">
                <span className="text-[18px] font-semibold text-text">{shift.shift_name}</span>
                <span className="text-[13px] text-text-2">
                  {shift.start_time}–{shift.end_time} · {shift.company_name}
                </span>
              </div>
              <StatusBadge dot tone={clockStatusTone(shift.clock_in_status)}>
                {t(`clockStatus.${shift.clock_in_status}`, { defaultValue: shift.clock_in_status })}
              </StatusBadge>
            </div>
          ) : (
            <p className="py-2 text-[13px] text-text-3">{t('dashOffToday')}</p>
          )}
        </div>

        <div className="flex w-[320px] shrink-0 flex-col gap-3 rounded-xl border border-border bg-surface p-[18px]">
          <span className="text-[15px] font-bold text-text">{t('historyTitle')}</span>
          <div className="flex flex-col gap-2.5">
            <RecentRow
              icon={CheckCircle2}
              tone="text-ok-tx"
              label={t('recent7dPresent')}
              value={ra.last_7d_present}
            />
            <RecentRow
              icon={ClockAlert}
              tone="text-warn-tx"
              label={t('recent7dLate')}
              value={ra.last_7d_late}
            />
            <RecentRow
              icon={Bell}
              tone="text-bad-tx"
              label={t('recent7dAbsent')}
              value={ra.last_7d_absent}
            />
          </div>
        </div>
      </div>
    </AgentPage>
  );
}

function RecentRow({
  icon: Icon,
  tone,
  label,
  value,
}: {
  icon: ComponentType<{ className?: string }>;
  tone: string;
  label: string;
  value: number;
}) {
  return (
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-2">
        <Icon className={`size-4 ${tone}`} />
        <span className="text-[13px] text-text-2">{label}</span>
      </div>
      <span className="text-[15px] font-semibold tabular-nums text-text">{value}</span>
    </div>
  );
}
