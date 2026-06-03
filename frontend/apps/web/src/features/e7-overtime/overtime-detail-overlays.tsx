/**
 * E7 · Lembur — Detail screen overlays.
 *
 * .pen frames referenced:
 *   uG6mQ   qTCEX/z736oS   — ActionCard footer (Tolak / Setujui buttons)
 *   YGLK3   Q4PRIx         — E7 · ModalReject (EnabP comp instance)
 *   STI8j   t9YPt          — Sheet · Tarik kembali OT (agent, destructive, no reason)
 *
 * Three overlays:
 *   RejectOvertimeModal  — reason-required rejection (L1 or HR-final)
 *   WithdrawOvertimeModal — destructive no-reason withdraw (agent, PENDING_L1 only)
 *   ConfirmOvertimeModal  — agent confirms auto-detected OT (note optional)
 */

import type { Overtime } from '@swp/api-client/e7';
import { Button, FormField, Modal, ModalBody, ModalFooter, ModalHeader } from '@swp/ui';
import { CornerUpLeft, Info, ShieldCheck, Sparkles } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { formatOtMinutes } from './overtime-shared.tsx';

// ---------------------------------------------------------------------------
// RejectOvertimeModal
// ---------------------------------------------------------------------------

interface RejectOvertimeModalProps {
  open: boolean;
  overtime: Pick<Overtime, 'id' | 'employee' | 'work_date' | 'calculation'>;
  onOpenChange: (open: boolean) => void;
  onConfirm: (reason: string) => void;
  isPending: boolean;
}

export function RejectOvertimeModal({
  open,
  overtime,
  onOpenChange,
  onConfirm,
  isPending,
}: RejectOvertimeModalProps) {
  const { t } = useTranslation('overtime');
  const [reason, setReason] = useState('');
  const [error, setError] = useState('');

  function handleConfirm() {
    const trimmed = reason.trim();
    if (trimmed.length < 5) {
      setError(t('detail.rejectReasonMinLength'));
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

  const summary = `${overtime.employee.name ?? overtime.employee.id} · ${overtime.work_date} · ${formatOtMinutes(overtime.calculation.counted_minutes)}`;

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={ShieldCheck} title={t('detail.rejectModalTitle')} tone="danger" />
      <ModalBody>
        <p className="mb-3 text-sm leading-relaxed text-text-2">
          {t('detail.rejectModalDesc', { summary })}
        </p>
        <FormField
          htmlFor="ot-reject-reason"
          label={t('detail.rejectReasonLabel')}
          required
          error={error}
        >
          <textarea
            id="ot-reject-reason"
            className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
            rows={4}
            placeholder={t('detail.rejectReasonPlaceholder')}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={isPending}
          />
        </FormField>
        <p className="mt-1.5 flex items-center gap-1.5 text-xs text-text-3">
          <ShieldCheck aria-hidden className="size-3 shrink-0" />
          {t('detail.rejectAuditNote')}
        </p>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel')}
        </Button>
        <Button
          type="button"
          variant="destructive"
          onClick={handleConfirm}
          disabled={isPending || reason.trim().length < 5}
        >
          {isPending ? t('common.processing') : t('detail.rejectConfirmBtn')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// WithdrawOvertimeModal
// Destructive, no reason required (Wave-3.4 / STI8j pattern — bottom sheet
// on mobile; here we keep it as a compact Modal since this is the web console).
// ---------------------------------------------------------------------------

interface WithdrawOvertimeModalProps {
  open: boolean;
  overtime: Pick<Overtime, 'id' | 'work_date' | 'calculation' | 'tier_indicator'>;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  isPending: boolean;
}

export function WithdrawOvertimeModal({
  open,
  overtime,
  onOpenChange,
  onConfirm,
  isPending,
}: WithdrawOvertimeModalProps) {
  const { t } = useTranslation('overtime');

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="sm">
      <ModalHeader icon={CornerUpLeft} title={t('detail.withdrawModalTitle')} tone="danger" />
      <ModalBody>
        {/* Detail mini-card matches STI8j → DetailRow */}
        <div className="mb-4 flex flex-col gap-1.5 rounded-xl bg-surface-2 px-4 py-3">
          <div className="flex items-center justify-between text-sm">
            <span className="text-text-3">{t('detail.fieldDate')}</span>
            <span className="font-medium text-text">{overtime.work_date}</span>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-text-3">{t('detail.fieldCounted')}</span>
            <span className="font-mono font-semibold text-text">
              {formatOtMinutes(overtime.calculation.counted_minutes)}
            </span>
          </div>
        </div>

        <p className="mb-3 text-sm leading-relaxed text-text-2">{t('detail.withdrawModalDesc')}</p>

        {/* Hint row from STI8j → HintRow */}
        <div className="flex items-center gap-2 text-xs text-text-3">
          <Info aria-hidden className="size-3.5 shrink-0" />
          {t('detail.withdrawHint')}
        </div>
      </ModalBody>
      <ModalFooter>
        <Button
          type="button"
          variant="secondary"
          onClick={() => onOpenChange(false)}
          disabled={isPending}
        >
          {t('common.back')}
        </Button>
        <Button type="button" variant="destructive" onClick={onConfirm} disabled={isPending}>
          {isPending ? t('common.processing') : t('detail.withdrawConfirmBtn')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ConfirmOvertimeModal
// Agent confirms an AUTO_DETECTED OT. Optional note (ConfirmOvertimeBody.note).
// Tone = neutral (positive act, just confirming).
// ---------------------------------------------------------------------------

interface ConfirmOvertimeModalProps {
  open: boolean;
  overtime: Pick<Overtime, 'id' | 'work_date' | 'calculation' | 'tier_indicator'>;
  onOpenChange: (open: boolean) => void;
  onConfirm: (note?: string) => void;
  isPending: boolean;
}

export function ConfirmOvertimeModal({
  open,
  overtime,
  onOpenChange,
  onConfirm,
  isPending,
}: ConfirmOvertimeModalProps) {
  const { t } = useTranslation('overtime');
  const [note, setNote] = useState('');

  function handleClose() {
    setNote('');
    onOpenChange(false);
  }

  function handleConfirm() {
    onConfirm(note.trim() || undefined);
  }

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={Sparkles} title={t('detail.confirmModalTitle')} tone="info" />
      <ModalBody>
        {/* Calc summary */}
        <div className="mb-4 flex flex-col gap-1.5 rounded-xl bg-surface-2 px-4 py-3">
          <div className="flex items-center justify-between text-sm">
            <span className="text-text-3">{t('detail.fieldDate')}</span>
            <span className="font-medium text-text">{overtime.work_date}</span>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-text-3">{t('detail.fieldCounted')}</span>
            <span className="font-mono font-semibold text-primary">
              {formatOtMinutes(overtime.calculation.counted_minutes)}
            </span>
          </div>
        </div>

        <p className="mb-3 text-sm leading-relaxed text-text-2">{t('detail.confirmModalDesc')}</p>

        <FormField htmlFor="ot-confirm-note" label={t('detail.confirmNoteLabel')}>
          <textarea
            id="ot-confirm-note"
            className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
            rows={3}
            placeholder={t('detail.confirmNotePlaceholder')}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            disabled={isPending}
          />
        </FormField>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel')}
        </Button>
        <Button type="button" variant="primary" onClick={handleConfirm} disabled={isPending}>
          {isPending ? t('common.processing') : t('detail.confirmBtn')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
