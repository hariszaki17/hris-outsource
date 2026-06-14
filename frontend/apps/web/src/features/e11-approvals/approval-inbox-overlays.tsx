/**
 * E11 · Approval inbox — overlays.
 *
 * .pen reusable component referenced:
 *   EnabP   comp/ModalReject  (frame: "Tolak permintaan", reason required, sent to requester)
 *
 * Mirrors the E7 RejectOvertimeModal shape so the reject UX is identical across the per-domain
 * approval tabs and the aggregated inbox (IB-5 single source of truth). Reason is mandatory
 * (the E11 reject endpoint requires a non-empty `reason`).
 */

import { Button, FormField, Modal, ModalBody, ModalFooter, ModalHeader } from '@swp/ui';
import { CornerUpLeft } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

export interface RejectInstanceModalProps {
  open: boolean;
  /** Human label of the request being rejected (requester + summary), shown in the body copy. */
  summary: string;
  onOpenChange: (open: boolean) => void;
  onConfirm: (reason: string) => void;
  isPending: boolean;
}

export function RejectInstanceModal({
  open,
  summary,
  onOpenChange,
  onConfirm,
  isPending,
}: RejectInstanceModalProps) {
  const { t } = useTranslation('approvals');
  const [reason, setReason] = useState('');
  const [error, setError] = useState('');

  function handleConfirm() {
    const trimmed = reason.trim();
    if (trimmed.length < 5) {
      setError(t('inbox.reject.reasonMinLength'));
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

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={CornerUpLeft} title={t('inbox.reject.title')} tone="danger" />
      <ModalBody>
        <p className="mb-3 text-sm text-text-2">{t('inbox.reject.description', { summary })}</p>
        <FormField
          htmlFor="reject-instance-reason"
          label={t('inbox.reject.reasonLabel')}
          required
          error={error}
        >
          <textarea
            id="reject-instance-reason"
            data-testid="reject-reason-input"
            className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
            rows={4}
            placeholder={t('inbox.reject.reasonPlaceholder')}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={isPending}
          />
        </FormField>
        <p className="mt-1.5 text-xs text-text-3">{t('inbox.reject.auditNote')}</p>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel')}
        </Button>
        <Button
          type="button"
          variant="destructive"
          data-testid="reject-confirm"
          onClick={handleConfirm}
          disabled={isPending}
        >
          {isPending ? t('common.processing') : t('inbox.reject.confirm')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
