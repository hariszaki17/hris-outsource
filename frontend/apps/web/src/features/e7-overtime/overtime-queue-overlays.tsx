/**
 * E7 Lembur — queue overlays.
 *
 * .pen reusable components referenced:
 *   r4KZl5  comp/ModalBulkApprove
 *   EnabP   comp/ModalReject
 *
 * Exports:
 *   RejectOvertimeModal   — single-row reject (queue or detail); requires reason ≥5 chars.
 *   BulkApproveModal      — bulk-approve with optional note; shows selected count.
 *   BulkRejectModal       — bulk-reject with required reason ≥5 chars.
 */

import type { Overtime } from '@swp/api-client/e7';
import { Button, FormField, Modal, ModalBody, ModalFooter, ModalHeader } from '@swp/ui';
import { CheckCheck, XOctagon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { formatOtMinutes } from './overtime-shared.tsx';

// ---------------------------------------------------------------------------
// RejectOvertimeModal
// ---------------------------------------------------------------------------

export interface RejectOvertimeModalProps {
  open: boolean;
  overtime: Pick<Overtime, 'id' | 'work_date' | 'calculation'> & {
    employeeName: string;
  };
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

  const counted = overtime.calculation.counted_minutes;
  const summary = `${overtime.employeeName} · ${overtime.work_date} · ${formatOtMinutes(counted)}`;

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={XOctagon} title={t('reject.title')} tone="danger" />
      <ModalBody>
        <p className="mb-3 text-sm text-text-2">{t('reject.description', { summary })}</p>
        <FormField
          htmlFor="reject-ot-reason"
          label={t('reject.reasonLabel')}
          required
          error={error}
        >
          <textarea
            id="reject-ot-reason"
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
// BulkApproveModal
// ---------------------------------------------------------------------------

export interface BulkApproveModalProps {
  open: boolean;
  /** How many rows are selected. */
  count: number;
  onOpenChange: (open: boolean) => void;
  onConfirm: (note: string) => void;
  isPending: boolean;
}

export function BulkApproveModal({
  open,
  count,
  onOpenChange,
  onConfirm,
  isPending,
}: BulkApproveModalProps) {
  const { t } = useTranslation('overtime');
  const [note, setNote] = useState('');

  function handleConfirm() {
    onConfirm(note.trim());
  }

  function handleClose() {
    setNote('');
    onOpenChange(false);
  }

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={CheckCheck} title={t('bulkApprove.title')} tone="brand" />
      <ModalBody>
        <p className="mb-3 text-sm text-text-2">{t('bulkApprove.description', { count })}</p>
        <FormField htmlFor="bulk-approve-note" label={t('bulkApprove.noteLabel')}>
          <textarea
            id="bulk-approve-note"
            className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
            rows={3}
            placeholder={t('bulkApprove.notePlaceholder')}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            disabled={isPending}
          />
        </FormField>
        <p className="mt-1.5 text-xs text-text-3">{t('bulkApprove.auditNote')}</p>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel')}
        </Button>
        <Button type="button" variant="primary" onClick={handleConfirm} disabled={isPending}>
          {isPending ? t('common.processing') : t('bulkApprove.confirm', { count })}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// BulkRejectModal
// ---------------------------------------------------------------------------

export interface BulkRejectModalProps {
  open: boolean;
  count: number;
  onOpenChange: (open: boolean) => void;
  onConfirm: (reason: string) => void;
  isPending: boolean;
}

export function BulkRejectModal({
  open,
  count,
  onOpenChange,
  onConfirm,
  isPending,
}: BulkRejectModalProps) {
  const { t } = useTranslation('overtime');
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

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={XOctagon} title={t('bulkReject.title')} tone="danger" />
      <ModalBody>
        <p className="mb-3 text-sm text-text-2">{t('bulkReject.description', { count })}</p>
        <FormField
          htmlFor="bulk-reject-reason"
          label={t('reject.reasonLabel')}
          required
          error={error}
        >
          <textarea
            id="bulk-reject-reason"
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
          {isPending ? t('common.processing') : t('bulkReject.confirm', { count })}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
