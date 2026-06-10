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
import { Button, type Column, DataTable, EmptyState, StateView, StatusBadge } from '@swp/ui';
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

  // ---------------------------------------------------------------------------
  // Columns
  // ---------------------------------------------------------------------------

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
          <span className="text-sm text-text-2 line-clamp-1">{r.reason}</span>
        ) : (
          <span className="text-sm text-text-3 italic">—</span>
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

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <AgentPage
      title={t('leaveTitle')}
      actions={
        <>
          {/* Balance chip — shown in the actions area alongside the CTA button */}
          {balance !== undefined && (
            <div className="flex items-center gap-1.5 rounded-full border border-border bg-surface px-3 py-1.5">
              <CalendarDays size={14} className="text-primary" aria-hidden />
              <span className="text-[13px] text-text-2">
                {t('leaveBalance', { days: balance })}
              </span>
            </div>
          )}
          <Button variant="primary" asChild size="sm">
            <Link to="/me/leave/new">{t('leaveNewBtn')}</Link>
          </Button>
        </>
      }
    >
      {listQ.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void listQ.refetch()} />
      ) : (
        <DataTable
          aria-label={t('leaveTitle')}
          columns={columns}
          data={items}
          getRowId={(r) => r.id}
          isLoading={listQ.isLoading}
          skeletonRows={6}
          empty={<EmptyState variant="fresh" title={t('leaveEmpty')} />}
        />
      )}
    </AgentPage>
  );
}
