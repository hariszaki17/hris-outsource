import { useCurrentUser } from '@/lib/use-auth.ts';
/**
 * /me/leave — Agent's own leave requests (list). Web port of apps/mobile/app/leave.tsx.
 *
 * F6.1 agent self-service: lists the authenticated agent's leave history. The server
 * enforces `scope: self` — no employee_id is sent. Shows pool balance from the grant-lot
 * ledger (GET /leave-balances/by-employee/{employeeId}).
 */
import {
  type LeaveRequest,
  LeaveStatus,
  useGetLeaveBalanceByEmployee,
  useListLeaveRequests,
} from '@swp/api-client/e6';
import type { StatusTone } from '@swp/design-tokens';
import { formatDate } from '@swp/shared';
import { Button, StateView, StatusBadge } from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { CalendarDays } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { AgentPage } from './agent-page.tsx';

// ---------------------------------------------------------------------------
// Helpers
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

// ---------------------------------------------------------------------------
// Row
// ---------------------------------------------------------------------------

function LeaveRow({ item }: { item: LeaveRequest }) {
  const { t } = useTranslation('agent');
  const tone = leaveStatusTone(item.status as LeaveStatus);
  return (
    <div className="rounded-xl border border-border bg-surface p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-0.5">
          <span className="text-[14px] font-semibold text-text">
            {item.leave_type_name ?? item.leave_type_id}
          </span>
          <span className="text-[12px] text-text-2">
            {formatDate(item.start_date)} &rarr; {formatDate(item.end_date)}
          </span>
          {item.reason && (
            <span className="mt-1 text-[12px] text-text-3 line-clamp-1">{item.reason}</span>
          )}
        </div>
        <StatusBadge dot tone={tone}>
          {t(`leave.status.${item.status}`, { defaultValue: item.status })}
        </StatusBadge>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function AgentLeaveScreen() {
  const { t } = useTranslation('agent');
  const user = useCurrentUser();
  const employeeId = user?.employeeId ?? '';

  const listQ = useListLeaveRequests({ limit: 20 });
  const balanceQ = useGetLeaveBalanceByEmployee(employeeId, undefined, {
    query: { enabled: Boolean(employeeId), staleTime: 60_000 },
  });

  const body = listQ.data?.data as { data?: LeaveRequest[] } | undefined;
  const items: LeaveRequest[] = body?.data ?? [];

  const balance = (balanceQ.data?.data as { pool_remaining?: number } | undefined)?.pool_remaining;

  return (
    <AgentPage
      title={t('leaveTitle')}
      actions={
        <Button variant="primary" asChild size="sm">
          <Link to="/me/leave/new">{t('leaveNewBtn')}</Link>
        </Button>
      }
    >
      {/* Balance chip */}
      {balance !== undefined && (
        <div className="flex items-center gap-2 rounded-xl border border-border bg-surface px-4 py-3">
          <CalendarDays size={16} className="text-primary" aria-hidden />
          <span className="text-[13px] text-text-2">{t('leaveBalance', { days: balance })}</span>
        </div>
      )}

      {/* List */}
      <div className="flex flex-col gap-3">
        {listQ.isLoading ? (
          <StateView kind="loading" title={t('loading')} />
        ) : listQ.isError ? (
          <StateView kind="error" title={t('errorGeneric')} onRetry={() => void listQ.refetch()} />
        ) : items.length === 0 ? (
          <StateView kind="empty" title={t('leaveEmpty')} />
        ) : (
          items.map((item) => <LeaveRow key={item.id} item={item} />)
        )}
      </div>
    </AgentPage>
  );
}
