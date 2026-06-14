/**
 * Status Pengajuan — leave-request detail modal (opened by clicking a row in /me/leave).
 *
 * Shows the request summary (type, dates, duration, reason, status). Approval routing now lives
 * in the E11 engine (the agent doesn't see the line-by-line chain — that's an approver surface);
 * the modal shows the collapsed lifecycle status + a "Diajukan" step. A pending request can be
 * withdrawn (useCancelLeaveRequest). Matches docs/design/brainstorm.pen
 * "Agen Web · Status Pengajuan (modal)".
 *
 * Status is now DRAFT | PENDING | APPROVED | REJECTED | CANCELLED (E11, 2026-06-14).
 */
import { type LeaveRequest, LeaveStatus, useCancelLeaveRequest } from '@swp/api-client/e6';
import type { StatusTone } from '@swp/design-tokens';
import { formatDate, formatInstant } from '@swp/shared';
import { Button, Modal, ModalBody, ModalFooter, ModalHeader, StatusBadge, useToast } from '@swp/ui';
import { Check, Loader, Plane, X } from 'lucide-react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Helpers
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

function isPending(status: LeaveStatus): boolean {
  return status === LeaveStatus.PENDING;
}

type StepTone = 'ok' | 'warn' | 'bad' | 'neutral';

interface Step {
  title: string;
  sub: string;
  tone: StepTone;
  icon: typeof Check;
}

// Static class map — interpolated Tailwind classes would be purged at build.
const STEP_DOT_CLASS: Record<StepTone, string> = {
  ok: 'border-ok-bd bg-ok-bg text-ok-tx',
  warn: 'border-warn-bd bg-warn-bg text-warn-tx',
  bad: 'border-bad-bd bg-bad-bg text-bad-tx',
  neutral: 'border-border bg-surface-2 text-text-3',
};

// ---------------------------------------------------------------------------
// Modal
// ---------------------------------------------------------------------------

export interface AgentLeaveStatusModalProps {
  request: LeaveRequest;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after a successful withdraw so the list can refetch. */
  onChanged?: () => void;
}

export function AgentLeaveStatusModal({
  request,
  open,
  onOpenChange,
  onChanged,
}: AgentLeaveStatusModalProps) {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const cancel = useCancelLeaveRequest();

  const pending = isPending(request.status as LeaveStatus);

  async function onWithdraw() {
    try {
      await cancel.mutateAsync({ id: request.id, data: {} });
      toast({ tone: 'success', title: t('statusWithdrawn') });
      onOpenChange(false);
      onChanged?.();
    } catch {
      toast({ tone: 'error', title: t('statusWithdrawError') });
    }
  }

  // Build the lifecycle steps: implicit "Diajukan" + the collapsed approval outcome.
  // The per-line chain lives in E11 and is shown only on the approver surfaces.
  const steps: Step[] = [
    {
      title: t('statusSubmitted'),
      sub: formatInstant(request.created_at),
      tone: 'ok',
      icon: Check,
    },
  ];
  const status = request.status as LeaveStatus;
  if (status === LeaveStatus.PENDING) {
    steps.push({
      title: t('statusStageApproval'),
      sub: t('statusPending'),
      tone: 'warn',
      icon: Loader,
    });
  } else if (status === LeaveStatus.APPROVED) {
    steps.push({
      title: t('statusStageApproval'),
      sub: formatInstant(request.updated_at ?? request.created_at),
      tone: 'ok',
      icon: Check,
    });
  } else if (status === LeaveStatus.REJECTED) {
    steps.push({
      title: t('statusStageApproval'),
      sub: formatInstant(request.updated_at ?? request.created_at),
      tone: 'bad',
      icon: X,
    });
  } else if (status === LeaveStatus.CANCELLED) {
    steps.push({
      title: t('statusStageWithdrawn'),
      sub: formatInstant(request.updated_at ?? request.created_at),
      tone: 'neutral',
      icon: X,
    });
  }

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg" className="w-[540px]">
      <ModalHeader
        icon={Plane}
        tone="brand"
        title={t('statusTitle')}
        closeLabel={t('statusClose')}
      />

      <ModalBody className="gap-5">
        {/* Summary grid */}
        <div className="flex flex-col gap-4">
          <div className="grid grid-cols-2 gap-4">
            <Detail
              label={t('leaveType')}
              value={request.leave_type_name ?? request.leave_type_id}
            />
            <Detail
              label={t('leaveStatus')}
              value={
                <StatusBadge dot tone={leaveStatusTone(request.status as LeaveStatus)}>
                  {t(`leave.status.${request.status}`, { defaultValue: request.status })}
                </StatusBadge>
              }
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <Detail
              label={t('leaveDateRange')}
              value={`${formatDate(request.start_date)} → ${formatDate(request.end_date)}`}
            />
            <Detail
              label={t('statusDuration')}
              value={t('statusDurationDays', { count: request.duration_days })}
            />
          </div>
          {request.reason && <Detail label={t('leaveReason')} value={request.reason} />}
        </div>

        <div className="h-px w-full bg-border-soft" />

        {/* Approval timeline */}
        <div className="flex flex-col gap-3">
          <h3 className="text-[13px] font-bold text-text">{t('statusApprovalTitle')}</h3>
          <ol className="flex flex-col">
            {steps.map((s, i) => (
              <li key={`${s.title}-${i}`} className="flex gap-3">
                {/* Rail: dot + connector */}
                <div className="flex flex-col items-center">
                  <span
                    className={[
                      'inline-flex size-[22px] shrink-0 items-center justify-center rounded-full border',
                      STEP_DOT_CLASS[s.tone],
                    ].join(' ')}
                  >
                    <s.icon className="size-3" aria-hidden />
                  </span>
                  {i < steps.length - 1 && <span className="my-0.5 w-0.5 flex-1 bg-border" />}
                </div>
                {/* Content */}
                <div className="flex flex-col gap-0.5 pb-4">
                  <span className="text-sm font-semibold text-text">{s.title}</span>
                  {s.sub && <span className="text-xs text-text-3">{s.sub}</span>}
                </div>
              </li>
            ))}
          </ol>
        </div>
      </ModalBody>

      <ModalFooter>
        <Button variant="secondary" size="sm" onClick={() => onOpenChange(false)}>
          {t('statusClose')}
        </Button>
        {pending && (
          <Button
            variant="destructive"
            size="sm"
            disabled={cancel.isPending}
            onClick={() => void onWithdraw()}
          >
            {t('statusWithdraw')}
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// Read-only detail cell
// ---------------------------------------------------------------------------

function Detail({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-medium text-text-3">{label}</span>
      {typeof value === 'string' ? (
        <span className="text-sm font-semibold text-text">{value}</span>
      ) : (
        value
      )}
    </div>
  );
}
