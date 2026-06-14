/**
 * E6 · Leave overlays — RejectModal (L1 + L2) and BalanceChangedModal (LA-5 override).
 *
 * .pen frames referenced:
 *   yho5i  mCu1Y  — ModalReject HR L2 queue row
 *   qb0S0  V7XCE  — ModalReject SL L1 queue row
 *   DJrBn  MHGOx  — ModalReject HR detail
 *   Hzbbv  g5ZJTf — ModalReject SL detail
 *   ZlnfW  uS8Lb  — Modal Saldo berubah (LA-5)
 *
 * Used by leave-approvals-screen.tsx and leave-detail-screen.tsx.
 */

import type { LeaveRequest } from '@swp/api-client/e6';
import type { LeaveStatus } from '@swp/api-client/e6';
import type { StatusTone } from '@swp/design-tokens';
import { Button, FormField, Modal, ModalBody, ModalFooter, ModalHeader } from '@swp/ui';
import { XOctagon } from 'lucide-react';
import { ShieldAlert } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Status tone helper
// ---------------------------------------------------------------------------

export function leaveStatusTone(status: LeaveStatus): StatusTone {
  switch (status) {
    case 'APPROVED':
      return 'ok';
    case 'REJECTED':
      return 'bad';
    case 'PENDING':
      return 'warn';
    case 'DRAFT':
    case 'CANCELLED':
      return 'neutral';
    default:
      return 'neutral';
  }
}

// ---------------------------------------------------------------------------
// RejectLeaveModal
// ---------------------------------------------------------------------------

interface RejectLeaveModalProps {
  open: boolean;
  leaveRequest: Pick<
    LeaveRequest,
    'id' | 'employee_name' | 'leave_type_name' | 'start_date' | 'end_date' | 'duration_days'
  >;
  onOpenChange: (open: boolean) => void;
  onConfirm: (reason: string) => void;
  isPending: boolean;
}

export function RejectLeaveModal({
  open,
  leaveRequest,
  onOpenChange,
  onConfirm,
  isPending,
}: RejectLeaveModalProps) {
  const { t } = useTranslation('leave');
  const [reason, setReason] = useState('');
  const [error, setError] = useState('');

  function handleConfirm() {
    const trimmed = reason.trim();
    if (trimmed.length < 5) {
      setError(t('reject.reasonMinLength'));
      return;
    }
    setError('');
    onConfirm(trimmed);
  }

  function handleClose() {
    setReason('');
    setError('');
    onOpenChange(false);
  }

  const summary = `${leaveRequest.employee_name ?? leaveRequest.id} · ${leaveRequest.leave_type_name ?? ''} · ${leaveRequest.start_date}–${leaveRequest.end_date} · ${leaveRequest.duration_days} ${t('common.days')}`;

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={XOctagon} title={t('reject.title')} />
      <ModalBody>
        <p className="mb-3 text-sm text-text-2">{t('reject.description', { summary })}</p>
        <FormField htmlFor="reject-reason" label={t('reject.reasonLabel')} required error={error}>
          <textarea
            id="reject-reason"
            className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
            rows={4}
            placeholder={t('reject.reasonPlaceholder')}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={isPending}
          />
        </FormField>
        <p className="mt-1.5 text-xs text-text-3">{t('reject.auditNote')}</p>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel')}
        </Button>
        <Button type="button" variant="destructive" onClick={handleConfirm} disabled={isPending}>
          {isPending ? t('common.processing') : t('reject.confirm')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// BalanceChangedModal (LA-5 override)
// ---------------------------------------------------------------------------

interface BalanceChangedModalProps {
  open: boolean;
  leaveRequest: Pick<
    LeaveRequest,
    'id' | 'employee_name' | 'leave_type_name' | 'duration_days' | 'balance_check'
  >;
  onOpenChange: (open: boolean) => void;
  onConfirm: (overrideReason: string) => void;
  isPending: boolean;
}

export function BalanceChangedModal({
  open,
  leaveRequest,
  onOpenChange,
  onConfirm,
  isPending,
}: BalanceChangedModalProps) {
  const { t } = useTranslation('leave');
  const [overrideReason, setOverrideReason] = useState('');
  const [error, setError] = useState('');

  const bc = leaveRequest.balance_check;

  function handleConfirm() {
    const trimmed = overrideReason.trim();
    if (trimmed.length < 10) {
      setError(t('override.reasonMinLength'));
      return;
    }
    setError('');
    onConfirm(trimmed);
  }

  function handleClose() {
    setOverrideReason('');
    setError('');
    onOpenChange(false);
  }

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={ShieldAlert} title={t('override.title')} tone="warn" />
      <ModalBody>
        <p className="mb-4 text-sm leading-relaxed text-text-2">
          {t('override.description', { name: leaveRequest.employee_name ?? leaveRequest.id })}
        </p>

        {/* Balance stats */}
        <div className="mb-4 grid grid-cols-3 gap-3">
          <div className="rounded-xl bg-surface-2 p-3.5">
            <p className="text-xs text-text-3">{t('override.currentBalance')}</p>
            <p className="mt-1 text-xl font-bold text-text">{bc?.remaining_days_at_check ?? '–'}</p>
            <p className="text-xs text-text-3">{t('common.days')}</p>
          </div>
          <div className="rounded-xl bg-surface-2 p-3.5">
            <p className="text-xs text-text-3">{t('override.requested')}</p>
            <p className="mt-1 text-xl font-bold text-text">{leaveRequest.duration_days}</p>
            <p className="text-xs text-text-3">{t('common.days')}</p>
          </div>
          <div className="rounded-xl bg-surface-2 p-3.5">
            <p className="text-xs text-text-3">{t('override.afterApproval')}</p>
            <p className="mt-1 text-xl font-bold text-negative">
              {bc != null ? (bc.remaining_days_at_check ?? 0) - leaveRequest.duration_days : '–'}
            </p>
            <p className="text-xs text-text-3">{t('common.days')}</p>
          </div>
        </div>

        {/* Audit notice */}
        <div className="mb-4 flex items-start gap-2 rounded-lg border border-warn-bd bg-warn-bg px-3 py-2.5">
          <ShieldAlert aria-hidden className="mt-0.5 size-3.5 shrink-0 text-warn-tx" />
          <p className="text-xs leading-relaxed text-warn-tx">{t('override.auditNote')}</p>
        </div>

        <FormField
          htmlFor="override-reason"
          label={t('override.reasonLabel')}
          required
          error={error}
        >
          <textarea
            id="override-reason"
            className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
            rows={3}
            placeholder={t('override.reasonPlaceholder')}
            value={overrideReason}
            onChange={(e) => setOverrideReason(e.target.value)}
            disabled={isPending}
          />
        </FormField>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel')}
        </Button>
        <Button type="button" variant="destructive" onClick={handleConfirm} disabled={isPending}>
          <ShieldAlert aria-hidden className="size-3.5" />
          {isPending ? t('common.processing') : t('override.confirm')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
