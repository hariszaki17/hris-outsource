/**
 * /me/overtime — Agent's own overtime requests: list, confirm (PENDING_AGENT_CONFIRM),
 * and withdraw (any pending state). Web port of apps/mobile/app/overtime.tsx.
 *
 * F7.1 / OA — agent self-service surface.
 */
import {
  type Overtime,
  type OvertimePage,
  OvertimeStatus,
  useConfirmOvertime,
  useListOvertime,
  useWithdrawOvertime,
} from '@swp/api-client/e7';
import { formatDate } from '@swp/shared';
import {
  Button,
  type Column,
  DataTable,
  EmptyState,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { overtimeStatusTone } from '../e7-overtime/overtime-shared.tsx';
import { AgentPage } from './agent-page.tsx';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function isPending(status: OvertimeStatus): boolean {
  return (
    status === OvertimeStatus.PENDING_AGENT_CONFIRM ||
    status === OvertimeStatus.PENDING_L1 ||
    status === OvertimeStatus.PENDING_HR
  );
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function AgentOvertimeScreen() {
  const { t } = useTranslation('agent');
  const { t: tOt } = useTranslation('overtime');
  const { toast } = useToast();

  const list = useListOvertime({ limit: 20 });
  const confirm = useConfirmOvertime();
  const withdraw = useWithdrawOvertime();

  const page = list.data?.data as OvertimePage | undefined;
  const items: Overtime[] = page?.data ?? [];

  async function onConfirm(id: string) {
    try {
      await confirm.mutateAsync({ id, data: {} });
      await list.refetch();
      toast({ tone: 'success', title: t('otConfirmed') });
    } catch {
      toast({ tone: 'error', title: t('otError') });
    }
  }

  async function onWithdraw(id: string) {
    try {
      await withdraw.mutateAsync({ id });
      await list.refetch();
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
        const pending = isPending(r.status);
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
    <AgentPage
      title={t('otTitle')}
      actions={
        <Button variant="primary" asChild size="sm">
          <Link to="/me/overtime/new">{t('otNewBtn')}</Link>
        </Button>
      }
    >
      {list.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void list.refetch()} />
      ) : (
        <DataTable
          aria-label={t('otTitle')}
          columns={columns}
          data={items}
          getRowId={(r) => r.id}
          isLoading={list.isLoading}
          skeletonRows={6}
          empty={<EmptyState variant="fresh" title={t('otEmpty')} />}
        />
      )}
    </AgentPage>
  );
}
