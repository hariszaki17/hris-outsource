/**
 * Status Koreksi — correction detail modal (opened from a row in the Pengajuan · Koreksi tab).
 *
 * Mirrors AgentLeaveStatusModal: shows the proposed change (before → after where available),
 * reason, and a collapsed approval lifecycle (Diajukan → Persetujuan → Diterapkan). The
 * per-line approval chain lives in the E11 engine (an approver surface); the agent sees the
 * collapsed outcome. The correction carries `approval_instance_id` — we follow it via
 * useGetApprovalInstance only to surface live chain progress (line N/M) when still pending.
 * A still-PENDING correction can be withdrawn (useCancelCorrection).
 *
 * Payable state: NEW_ENTRY / no-shift days carry `is_payable` once applied — surfaced as a badge.
 *
 * F5.6 refs: CR-5 (original values preserved), approval_instance follow-through.
 */
import { type Correction, CorrectionStatus, useCancelCorrection } from '@swp/api-client/e5';
import { type ApprovalInstance, useGetApprovalInstance } from '@swp/api-client/e11';
import type { StatusTone } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared';
import { Button, Modal, ModalBody, ModalFooter, ModalHeader, StatusBadge, useToast } from '@swp/ui';
import { Check, Loader, PencilLine, X } from 'lucide-react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

function correctionStatusTone(status: CorrectionStatus): StatusTone {
  switch (status) {
    case CorrectionStatus.APPROVED:
    case CorrectionStatus.APPLIED:
      return 'ok';
    case CorrectionStatus.REJECTED:
      return 'bad';
    case CorrectionStatus.PENDING:
      return 'warn';
    default:
      return 'neutral';
  }
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

function timeOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { timeStyle: 'short', dateStyle: 'medium' }) : '—';
}

export interface AgentCorrectionStatusModalProps {
  correction: Correction;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after a successful withdraw so the list can refetch. */
  onChanged?: () => void;
}

export function AgentCorrectionStatusModal({
  correction,
  open,
  onOpenChange,
  onChanged,
}: AgentCorrectionStatusModalProps) {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const cancel = useCancelCorrection();

  const status = correction.status as CorrectionStatus;
  const isPending = status === CorrectionStatus.PENDING;

  // Follow the E11 approval_instance only while pending — to surface live "Baris N/M" progress.
  const instanceQ = useGetApprovalInstance(correction.approval_instance_id ?? '', {
    query: { enabled: open && isPending && Boolean(correction.approval_instance_id) },
  });
  const instance = (instanceQ.data?.data as { data?: ApprovalInstance } | undefined)?.data;

  async function onWithdraw() {
    try {
      await cancel.mutateAsync({ id: correction.id });
      toast({ tone: 'success', title: t('corrWithdrawn') });
      onOpenChange(false);
      onChanged?.();
    } catch {
      toast({ tone: 'error', title: t('corrWithdrawError') });
    }
  }

  // Lifecycle steps: Diajukan → Persetujuan (→ Diterapkan / Ditolak / Ditarik).
  const steps: Step[] = [
    {
      title: t('statusSubmitted'),
      sub: formatInstant(correction.created_at),
      tone: 'ok',
      icon: Check,
    },
  ];
  if (status === CorrectionStatus.PENDING) {
    const lineSub =
      instance?.line_count && instance.line_count > 1
        ? t('corrLineProgress', {
            current: instance.current_line,
            total: instance.line_count,
          })
        : t('statusPending');
    steps.push({ title: t('statusStageApproval'), sub: lineSub, tone: 'warn', icon: Loader });
  } else if (status === CorrectionStatus.APPROVED || status === CorrectionStatus.APPLIED) {
    steps.push({
      title: t('statusStageApproval'),
      sub: formatInstant(correction.decided_at ?? correction.updated_at ?? correction.created_at),
      tone: 'ok',
      icon: Check,
    });
    if (status === CorrectionStatus.APPLIED) {
      steps.push({
        title: t('corrStageApplied'),
        sub: t('corrAppliedNote'),
        tone: 'ok',
        icon: Check,
      });
    }
  } else if (status === CorrectionStatus.REJECTED) {
    steps.push({
      title: t('statusStageApproval'),
      sub:
        correction.reject_reason ?? formatInstant(correction.decided_at ?? correction.created_at),
      tone: 'bad',
      icon: X,
    });
  } else if (status === CorrectionStatus.CANCELLED) {
    steps.push({
      title: t('statusStageWithdrawn'),
      sub: formatInstant(correction.updated_at ?? correction.created_at),
      tone: 'neutral',
      icon: X,
    });
  }

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg" className="w-[540px]">
      <ModalHeader
        icon={PencilLine}
        tone="brand"
        title={t('corrStatusTitle')}
        closeLabel={t('statusClose')}
      />

      <ModalBody className="gap-5">
        {/* Summary grid */}
        <div className="flex flex-col gap-4">
          <div className="grid grid-cols-2 gap-4">
            <Detail label={t('corrColType')} value={t(`corrType.${correction.type}`)} />
            <Detail
              label={t('leaveStatus')}
              value={
                <StatusBadge dot tone={correctionStatusTone(status)}>
                  {t(`corrStatus.${status}`, { defaultValue: status })}
                </StatusBadge>
              }
            />
          </div>
          {correction.work_date && (
            <Detail
              label={t('corrWorkDate')}
              value={formatInstant(correction.work_date, { dateStyle: 'medium' })}
            />
          )}
          {/* Proposed change (before → after where the original snapshot is available) */}
          {correction.proposed_check_in_at && (
            <Detail label={t('corrCheckInTime')} value={timeOf(correction.proposed_check_in_at)} />
          )}
          {correction.proposed_check_out_at && (
            <Detail
              label={t('corrCheckOutTime')}
              value={timeOf(correction.proposed_check_out_at)}
            />
          )}
          <Detail label={t('corrReason')} value={correction.reason} />
        </div>

        <div className="h-px w-full bg-border-soft" />

        {/* Approval timeline */}
        <div className="flex flex-col gap-3">
          <h3 className="text-[13px] font-bold text-text">{t('statusApprovalTitle')}</h3>
          <ol className="flex flex-col">
            {steps.map((s, i) => (
              <li key={`${s.title}-${i}`} className="flex gap-3">
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
        {isPending && (
          <Button
            variant="destructive"
            size="sm"
            disabled={cancel.isPending}
            onClick={() => void onWithdraw()}
          >
            {t('corrWithdraw')}
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}

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
