/**
 * E11 · Bypass modal (super-admin force-approve).
 *
 * .pen frame: KT3Jz (E11 · Overlay — Bypass · Super Admin).
 * Specs: F11.2 · INV-5 (bypass force-approve, skips remaining lines, logged as BYPASS).
 *
 * Reason is REQUIRED (confirm disabled until filled). Accent-purple confirm per the frame.
 * Reuses the shared `comp/Modal` molecule (`@swp/ui`) — does NOT fork a second modal (G3).
 */

import { Button, FormField, Modal, ModalBody, ModalFooter, ModalHeader } from '@swp/ui';
import { ShieldCheck, TriangleAlert } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

interface BypassModalProps {
  open: boolean;
  /** Instance id (`SWP-APV-…`) — shown in the audit warning copy. */
  instanceId: string;
  /** Current line being skipped — for the warning copy. */
  currentLine: number;
  onOpenChange: (open: boolean) => void;
  onConfirm: (reason: string) => void;
  isPending: boolean;
}

export function BypassModal({
  open,
  instanceId,
  currentLine,
  onOpenChange,
  onConfirm,
  isPending,
}: BypassModalProps) {
  const { t } = useTranslation();
  const [reason, setReason] = useState('');
  const [error, setError] = useState('');

  function handleConfirm() {
    const trimmed = reason.trim();
    if (trimmed.length < 10) {
      setError(t('approvals.bypass.reasonMinLength'));
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

  const reasonValid = reason.trim().length >= 10;

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={ShieldCheck} title={t('approvals.bypass.title')} />
      <ModalBody>
        {/* Warn callout */}
        <div className="mb-3.5 flex items-start gap-2.5 rounded-lg border border-warn-bd bg-warn-bg px-3.5 py-3">
          <TriangleAlert aria-hidden className="mt-0.5 size-4 shrink-0 text-warn-tx" />
          <p className="text-[13px] leading-relaxed text-warn-tx">
            {t('approvals.bypass.warning', { id: instanceId, line: currentLine })}
          </p>
        </div>

        <FormField
          htmlFor="bypass-reason"
          label={t('approvals.bypass.reasonLabel')}
          required
          error={error}
        >
          <textarea
            id="bypass-reason"
            data-testid="bypass-reason-input"
            className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
            rows={3}
            placeholder={t('approvals.bypass.reasonPlaceholder')}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={isPending}
          />
        </FormField>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel')}
        </Button>
        {/* Accent-purple confirm per frame KT3Jz. */}
        <button
          type="button"
          data-testid="bypass-confirm"
          onClick={handleConfirm}
          disabled={isPending || !reasonValid}
          className="inline-flex items-center gap-2 rounded-lg bg-[var(--color-accent-purple,#8E0E8E)] px-4 py-2.5 text-sm font-bold text-white transition-opacity hover:opacity-90 disabled:opacity-50"
        >
          <ShieldCheck aria-hidden className="size-4" />
          {isPending ? t('common.processing') : t('approvals.bypass.confirm')}
        </button>
      </ModalFooter>
    </Modal>
  );
}
