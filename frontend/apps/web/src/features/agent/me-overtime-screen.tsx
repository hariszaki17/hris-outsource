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
import type { StatusTone } from '@swp/design-tokens';
import { formatDate } from '@swp/shared';
import { Button, StateView, StatusBadge, useToast } from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
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
  const { toast } = useToast();
  const navigate = useNavigate();

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

  return (
    <AgentPage
      title={t('otTitle')}
      actions={
        <Button variant="primary" onClick={() => void navigate({ to: '/me/overtime/new' })}>
          {t('otNewBtn')}
        </Button>
      }
    >
      <div className="flex flex-col gap-3">
        {list.isLoading ? (
          <StateView kind="loading" title={t('loading')} />
        ) : list.isError ? (
          <StateView kind="error" title={t('errorGeneric')} onRetry={() => void list.refetch()} />
        ) : items.length === 0 ? (
          <StateView kind="empty" title={t('otEmpty')} />
        ) : (
          items.map((it) => (
            <OvertimeCard
              key={it.id}
              item={it}
              tone={overtimeStatusTone(it.status)}
              pending={isPending(it.status)}
              onConfirm={onConfirm}
              onWithdraw={onWithdraw}
              confirmBusy={confirm.isPending && confirm.variables?.id === it.id}
              withdrawBusy={withdraw.isPending && withdraw.variables?.id === it.id}
            />
          ))
        )}
      </div>
    </AgentPage>
  );
}

// ---------------------------------------------------------------------------
// Card
// ---------------------------------------------------------------------------

interface OvertimeCardProps {
  item: Overtime;
  tone: StatusTone;
  pending: boolean;
  onConfirm: (id: string) => Promise<void>;
  onWithdraw: (id: string) => Promise<void>;
  confirmBusy: boolean;
  withdrawBusy: boolean;
}

function OvertimeCard({
  item,
  tone,
  pending,
  onConfirm,
  onWithdraw,
  confirmBusy,
  withdrawBusy,
}: OvertimeCardProps) {
  const { t } = useTranslation('agent');
  const { t: tOt } = useTranslation('overtime');
  const busy = confirmBusy || withdrawBusy;

  return (
    <div className="rounded-xl border border-border bg-surface p-4">
      <div className="flex items-center justify-between">
        <span className="text-[14px] font-semibold text-text">{formatDate(item.work_date)}</span>
        <StatusBadge dot tone={tone}>
          {tOt(`status.${item.status}`, { defaultValue: item.status })}
        </StatusBadge>
      </div>

      {(item.planned_start_time || item.planned_end_time) && (
        <p className="mt-1 text-[12px] text-text-2">
          {item.planned_start_time ?? '—'} – {item.planned_end_time ?? '—'}
        </p>
      )}

      {item.reason ? (
        <p className="mt-1 text-[12px] text-text-3 line-clamp-2">{item.reason}</p>
      ) : null}

      {(item.status === OvertimeStatus.PENDING_AGENT_CONFIRM || pending) && (
        <div className="mt-3 flex items-center gap-2">
          {item.status === OvertimeStatus.PENDING_AGENT_CONFIRM && (
            <Button variant="primary" disabled={busy} onClick={() => void onConfirm(item.id)}>
              {t('otConfirm')}
            </Button>
          )}
          {pending && (
            <Button variant="secondary" disabled={busy} onClick={() => void onWithdraw(item.id)}>
              {t('otWithdraw')}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
