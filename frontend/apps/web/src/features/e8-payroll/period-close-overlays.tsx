/**
 * E8 · F8.5 — Payroll Period Close Overlays
 *
 * .pen frames (WEB CONSOLE board, feature group gLZlI, ScreensRow K494zi):
 *   qGPyy  Lock confirm CLEAN — standard confirm, no blockers, Lock enabled
 *   rPFqX  Lock BLOCKED (HR warning) — blocker count + resolve-first callout
 *   TBRoX  Force-lock (super-admin) — required reason textarea, warn header
 *   l0IAi  Reopen (super-admin) — required reason textarea
 *   E3LRpT Clarification raise modal — RHF + Zod question field
 *   ATzJd  Toasts (success / clarif) — useToast()
 *   (no dedicated .pen frame for manage-clarification — minimal DS-consistent
 *    dialog built inline: question + answer display + resolve / cancel actions)
 *
 * DS entry: DESIGN-SYSTEM.md §8.16 (F8.5)
 * Analog: bypass-modal.tsx (required-reason pattern)
 * Zod schemas are local (spec defers Zod codegen for period-close mutations)
 * Invalidation: getGetPayrollPeriodQueryKey + getGetPeriodCompletenessQueryKey + getGetPeriodSummaryQueryKey
 */

import {
  type ClarificationRequest,
  ClarificationStatus,
  type ClarificationSourceType,
  type PayrollPeriod,
  getGetPayrollPeriodQueryKey,
  getGetPeriodCompletenessQueryKey,
  getGetPeriodSummaryQueryKey,
  useCancelClarification,
  useCreateClarification,
  useLockPayrollPeriod,
  useReopenPayrollPeriod,
  useResolveClarification,
} from '@swp/api-client/e8';
import { Button, ConfirmDialog, DateText, FormField, Modal, ModalBody, ModalFooter, ModalHeader, useToast } from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, Lock, LockOpen, MessageSquare, ShieldCheck, TriangleAlert, XCircle } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Local Zod schemas (Zod codegen deferred for period-close mutations — RECON §1)
// ---------------------------------------------------------------------------

const reasonSchema = z
  .string()
  .min(10, 'Alasan minimal 10 karakter')
  .max(500, 'Alasan maksimal 500 karakter');

const questionSchema = z
  .string()
  .min(5, 'Pertanyaan minimal 5 karakter')
  .max(1000, 'Pertanyaan maksimal 1000 karakter');

// ---------------------------------------------------------------------------
// LockDialog — frames qGPyy (clean), rPFqX (HR warning), TBRoX (force super-admin)
// ---------------------------------------------------------------------------

interface LockDialogProps {
  open: boolean;
  year: number;
  month: number;
  period: PayrollPeriod;
  isSuperAdmin: boolean;
  onOpenChange: (open: boolean) => void;
}

export function LockDialog({
  open,
  year,
  month,
  period,
  isSuperAdmin,
  onOpenChange,
}: LockDialogProps) {
  const { t } = useTranslation('payroll');
  const { toast } = useToast();
  const qc = useQueryClient();
  const [reason, setReason] = useState('');
  const [reasonError, setReasonError] = useState('');

  const hasBlockers =
    period.blockers.attendance > 0 || period.blockers.overtime > 0 || period.blockers.leave > 0;

  // Is this a force-lock path? (super_admin + blockers > 0)
  const isForcePath = isSuperAdmin && hasBlockers;
  // HR sees blocker warning if there are blockers and they're not super-admin
  const isHrBlockedPath = !isSuperAdmin && hasBlockers;

  const { mutate: lock, isPending } = useLockPayrollPeriod({
    mutation: {
      onSuccess: () => {
        // frame ATzJd — success toast
        toast({
          tone: 'success',
          title: t('periodClose.lockSuccessTitle'),
          description: t('periodClose.lockSuccessDesc'),
        });
        // Invalidate: period + completeness + summary (RECON §1)
        void qc.invalidateQueries({ queryKey: getGetPayrollPeriodQueryKey(year, month) });
        void qc.invalidateQueries({
          queryKey: getGetPeriodCompletenessQueryKey(year, month),
        });
        void qc.invalidateQueries({
          queryKey: getGetPeriodSummaryQueryKey(year, month),
        });
        handleClose();
      },
      onError: () => {
        toast({
          tone: 'error',
          title: t('periodClose.lockErrorTitle'),
          description: t('periodClose.lockErrorDesc'),
        });
      },
    },
  });

  function handleClose() {
    setReason('');
    setReasonError('');
    onOpenChange(false);
  }

  function handleConfirm() {
    if (isForcePath) {
      // TBRoX — reason required for force-lock
      const parsed = reasonSchema.safeParse(reason.trim());
      if (!parsed.success) {
        setReasonError(parsed.error.issues[0]?.message ?? t('periodClose.reasonRequired'));
        return;
      }
      lock({ year, month, data: { force: true, reason: reason.trim() } });
    } else {
      // qGPyy — clean lock
      lock({ year, month, data: {} });
    }
  }

  // frame rPFqX — HR blocked: cannot lock, show warning only
  if (isHrBlockedPath) {
    return (
      <Modal open={open} onOpenChange={handleClose} size="sm">
        {/* rPFqX — warn header */}
        <ModalHeader icon={TriangleAlert} title={t('periodClose.lockBlockedTitle')} />
        <ModalBody>
          <div className="mb-3.5 flex items-start gap-2.5 rounded-lg border border-warn-bd bg-warn-bg px-3.5 py-3">
            <TriangleAlert aria-hidden className="mt-0.5 size-4 shrink-0 text-warn-tx" />
            <div className="flex flex-col gap-1.5">
              <p className="text-[13px] font-semibold text-warn-tx">
                {t('periodClose.lockBlockedWarning')}
              </p>
              <ul className="list-disc ml-4 text-[12px] text-warn-tx space-y-0.5">
                {period.blockers.attendance > 0 && (
                  <li>
                    {t('periodClose.blockerAttendance', {
                      count: period.blockers.attendance,
                    })}
                  </li>
                )}
                {period.blockers.overtime > 0 && (
                  <li>
                    {t('periodClose.blockerOvertime', {
                      count: period.blockers.overtime,
                    })}
                  </li>
                )}
                {period.blockers.leave > 0 && (
                  <li>{t('periodClose.blockerLeave', { count: period.blockers.leave })}</li>
                )}
              </ul>
            </div>
          </div>
          <p className="text-[13px] text-text-2">{t('periodClose.lockBlockedInstruct')}</p>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="primary" onClick={handleClose}>
            {t('periodClose.lockBlockedOk')}
          </Button>
        </ModalFooter>
      </Modal>
    );
  }

  // frame TBRoX — super-admin force-lock (has blockers)
  if (isForcePath) {
    return (
      <Modal open={open} onOpenChange={handleClose} size="sm">
        <ModalHeader icon={ShieldCheck} title={t('periodClose.forceLockTitle')} />
        <ModalBody>
          {/* Warn callout — frame TBRoX warn header */}
          <div className="mb-3.5 flex items-start gap-2.5 rounded-lg border border-warn-bd bg-warn-bg px-3.5 py-3">
            <TriangleAlert aria-hidden className="mt-0.5 size-4 shrink-0 text-warn-tx" />
            <p className="text-[13px] leading-relaxed text-warn-tx">
              {t('periodClose.forceLockWarning', {
                attendance: period.blockers.attendance,
                overtime: period.blockers.overtime,
                leave: period.blockers.leave,
              })}
            </p>
          </div>

          {/* Required reason — frame TBRoX */}
          <FormField
            htmlFor="force-lock-reason"
            label={t('periodClose.forceLockReasonLabel')}
            required
            error={reasonError}
          >
            <textarea
              id="force-lock-reason"
              data-testid="force-lock-reason-input"
              className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
              rows={3}
              placeholder={t('periodClose.forceLockReasonPlaceholder')}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              disabled={isPending}
            />
          </FormField>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
            {t('common.cancel', { ns: 'translation' })}
          </Button>
          {/* Warn-tone confirm for force-lock — frame TBRoX */}
          <button
            type="button"
            data-testid="force-lock-confirm"
            onClick={handleConfirm}
            disabled={isPending || reason.trim().length < 10}
            className="inline-flex items-center gap-2 rounded-lg bg-[var(--color-warn,#B45309)] px-4 py-2.5 text-sm font-bold text-white transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            <ShieldCheck aria-hidden className="size-4" />
            {isPending
              ? t('common.processing', { ns: 'translation' })
              : t('periodClose.forceLockConfirm')}
          </button>
        </ModalFooter>
      </Modal>
    );
  }

  // frame qGPyy — clean lock confirm (isCleanPath = true)
  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={Lock} title={t('periodClose.lockCleanTitle')} />
      <ModalBody>
        <p className="text-[14px] text-text">{t('periodClose.lockCleanBody')}</p>
        <div className="mt-3 flex items-start gap-2.5 rounded-lg border border-ok-bd bg-ok-bg px-3.5 py-3">
          <Lock aria-hidden className="mt-0.5 size-4 shrink-0 text-ok-tx" />
          <p className="text-[13px] text-ok-tx">{t('periodClose.lockCleanNote')}</p>
        </div>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel', { ns: 'translation' })}
        </Button>
        <Button
          type="button"
          variant="primary"
          data-testid="lock-confirm"
          onClick={handleConfirm}
          disabled={isPending}
        >
          <Lock aria-hidden className="size-4" />
          {isPending ? t('common.processing', { ns: 'translation' }) : t('periodClose.lockConfirm')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ReopenDialog — frame l0IAi (super-admin reopen, required reason)
// ---------------------------------------------------------------------------

interface ReopenDialogProps {
  open: boolean;
  year: number;
  month: number;
  onOpenChange: (open: boolean) => void;
}

export function ReopenDialog({ open, year, month, onOpenChange }: ReopenDialogProps) {
  const { t } = useTranslation('payroll');
  const { toast } = useToast();
  const qc = useQueryClient();
  const [reason, setReason] = useState('');
  const [reasonError, setReasonError] = useState('');

  const { mutate: reopen, isPending } = useReopenPayrollPeriod({
    mutation: {
      onSuccess: () => {
        // frame ATzJd — success toast
        toast({
          tone: 'success',
          title: t('periodClose.reopenSuccessTitle'),
          description: t('periodClose.reopenSuccessDesc'),
        });
        void qc.invalidateQueries({ queryKey: getGetPayrollPeriodQueryKey(year, month) });
        void qc.invalidateQueries({
          queryKey: getGetPeriodCompletenessQueryKey(year, month),
        });
        void qc.invalidateQueries({
          queryKey: getGetPeriodSummaryQueryKey(year, month),
        });
        handleClose();
      },
      onError: (err: unknown) => {
        // PC-12: 409 PERIOD_RUN_POSTED
        const code = (err as { data?: { error?: { code?: string } } })?.data?.error?.code;
        toast({
          tone: 'error',
          title: t('periodClose.reopenErrorTitle'),
          description:
            code === 'PERIOD_RUN_POSTED'
              ? t('periodClose.reopenPostedError')
              : t('periodClose.reopenGenericError'),
        });
      },
    },
  });

  function handleClose() {
    setReason('');
    setReasonError('');
    onOpenChange(false);
  }

  function handleConfirm() {
    const parsed = reasonSchema.safeParse(reason.trim());
    if (!parsed.success) {
      setReasonError(parsed.error.issues[0]?.message ?? t('periodClose.reasonRequired'));
      return;
    }
    setReasonError('');
    reopen({ year, month, data: { reason: reason.trim() } });
  }

  const reasonValid = reason.trim().length >= 10;

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      {/* frame l0IAi */}
      <ModalHeader icon={LockOpen} title={t('periodClose.reopenTitle')} />
      <ModalBody>
        {/* Warn callout — frame l0IAi */}
        <div className="mb-3.5 flex items-start gap-2.5 rounded-lg border border-warn-bd bg-warn-bg px-3.5 py-3">
          <TriangleAlert aria-hidden className="mt-0.5 size-4 shrink-0 text-warn-tx" />
          <p className="text-[13px] leading-relaxed text-warn-tx">
            {t('periodClose.reopenWarning')}
          </p>
        </div>

        {/* Required reason — frame l0IAi */}
        <FormField
          htmlFor="reopen-reason"
          label={t('periodClose.reopenReasonLabel')}
          required
          error={reasonError}
        >
          <textarea
            id="reopen-reason"
            data-testid="reopen-reason-input"
            className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
            rows={3}
            placeholder={t('periodClose.reopenReasonPlaceholder')}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={isPending}
          />
        </FormField>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel', { ns: 'translation' })}
        </Button>
        <button
          type="button"
          data-testid="reopen-confirm"
          onClick={handleConfirm}
          disabled={isPending || !reasonValid}
          className="inline-flex items-center gap-2 rounded-lg bg-[var(--color-accent-purple,#8E0E8E)] px-4 py-2.5 text-sm font-bold text-white transition-opacity hover:opacity-90 disabled:opacity-50"
        >
          <LockOpen aria-hidden className="size-4" />
          {isPending
            ? t('common.processing', { ns: 'translation' })
            : t('periodClose.reopenConfirm')}
        </button>
      </ModalFooter>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ClarificationDialog — frame E3LRpT (raise clarification, PC-13)
// ---------------------------------------------------------------------------

interface ClarificationDialogProps {
  open: boolean;
  /** Which upstream the clarification targets (ATTENDANCE / OVERTIME / LEAVE). */
  sourceType: ClarificationSourceType;
  /** The upstream record ID (e.g. employee_id for attendance tab). */
  sourceId: string;
  /** Display label shown in the modal header. */
  label: string;
  onOpenChange: (open: boolean) => void;
  /** Called with the created ClarificationRequest on success so the caller can track it. */
  onCreated?: (clarification: ClarificationRequest) => void;
}

export function ClarificationDialog({
  open,
  sourceType,
  sourceId,
  label,
  onOpenChange,
  onCreated,
}: ClarificationDialogProps) {
  const { t } = useTranslation('payroll');
  const { toast } = useToast();
  const [question, setQuestion] = useState('');
  const [questionError, setQuestionError] = useState('');

  const { mutate: createClarification, isPending } = useCreateClarification({
    mutation: {
      onSuccess: (response) => {
        // frame ATzJd — clarification sent toast
        toast({
          tone: 'info',
          title: t('periodClose.clarifSentTitle'),
          description: t('periodClose.clarifSentDesc'),
        });
        // Surface the created ClarificationRequest to the parent so it can
        // show the "Menunggu klarifikasi" badge + manage button on the row.
        // The backend wraps the response in { data: ClarificationRequest }.
        const created = (response.data as { data?: ClarificationRequest })?.data ?? (response.data as ClarificationRequest);
        onCreated?.(created);
        handleClose();
      },
      onError: () => {
        toast({
          tone: 'error',
          title: t('periodClose.clarifErrorTitle'),
          description: t('periodClose.clarifErrorDesc'),
        });
      },
    },
  });

  function handleClose() {
    setQuestion('');
    setQuestionError('');
    onOpenChange(false);
  }

  function handleSend() {
    const parsed = questionSchema.safeParse(question.trim());
    if (!parsed.success) {
      setQuestionError(parsed.error.issues[0]?.message ?? t('periodClose.questionRequired'));
      return;
    }
    setQuestionError('');
    createClarification({
      data: {
        source_type: sourceType,
        source_id: sourceId,
        question: question.trim(),
      },
    });
  }

  const questionValid = question.trim().length >= 5;

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      {/* frame E3LRpT */}
      <ModalHeader icon={MessageSquare} title={t('periodClose.clarifTitle')} />
      <ModalBody>
        {/* Target label */}
        <div className="mb-3 rounded-lg border border-border bg-surface-2 px-3.5 py-2.5">
          <p className="text-[11px] font-medium text-text-3 uppercase tracking-wide mb-0.5">
            {t('periodClose.clarifTarget')}
          </p>
          <p className="text-[13px] font-semibold text-text">{label}</p>
          <p className="text-[11px] text-text-3">{sourceId}</p>
        </div>

        {/* Advisory note — PC-13: one round, not an approval line */}
        <div className="mb-3.5 flex items-start gap-2 rounded-lg border border-info-bd bg-info-bg px-3 py-2.5">
          <MessageSquare aria-hidden className="mt-0.5 size-3.5 shrink-0 text-info-tx" />
          <p className="text-[12px] text-info-tx">{t('periodClose.clarifAdvisory')}</p>
        </div>

        {/* Question field — frame E3LRpT */}
        <FormField
          htmlFor="clarif-question"
          label={t('periodClose.clarifQuestionLabel')}
          required
          error={questionError}
        >
          <textarea
            id="clarif-question"
            data-testid="clarif-question-input"
            className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
            rows={4}
            placeholder={t('periodClose.clarifQuestionPlaceholder')}
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            disabled={isPending}
          />
        </FormField>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel', { ns: 'translation' })}
        </Button>
        <Button
          type="button"
          variant="primary"
          data-testid="clarif-send"
          onClick={handleSend}
          disabled={isPending || !questionValid}
        >
          <MessageSquare aria-hidden className="size-4" />
          {isPending
            ? t('common.processing', { ns: 'translation' })
            : t('periodClose.clarifSendBtn')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ManageClarificationDialog — no dedicated .pen frame; DS-consistent minimal
// dialog for HR to view question + answer and resolve or cancel (PC-13).
// RBAC: visible to payroll.read holders (HR + super-admin) — gated in screen.
// ---------------------------------------------------------------------------

interface ManageClarificationDialogProps {
  open: boolean;
  clarification: ClarificationRequest;
  /** Employee display name shown in the header. */
  employeeName: string;
  year: number;
  month: number;
  onOpenChange: (open: boolean) => void;
  /** Called when the clarification is resolved or cancelled so the parent
   *  can clear its local state. */
  onSettled: (id: string) => void;
}

export function ManageClarificationDialog({
  open,
  clarification,
  employeeName,
  year,
  month,
  onOpenChange,
  onSettled,
}: ManageClarificationDialogProps) {
  const { t } = useTranslation('payroll');
  const { toast } = useToast();
  const qc = useQueryClient();
  const [cancelConfirmOpen, setCancelConfirmOpen] = useState(false);

  const isOpen =
    clarification.status === ClarificationStatus.OPEN ||
    clarification.status === ClarificationStatus.ANSWERED;

  // Shared invalidation helper — refresh period badge + completeness table
  function invalidate() {
    void qc.invalidateQueries({ queryKey: getGetPayrollPeriodQueryKey(year, month) });
    void qc.invalidateQueries({ queryKey: getGetPeriodCompletenessQueryKey(year, month) });
  }

  const { mutate: resolve, isPending: isResolving } = useResolveClarification({
    mutation: {
      onSuccess: () => {
        toast({
          tone: 'success',
          title: t('periodClose.clarifResolveSuccessTitle'),
          description: t('periodClose.clarifResolveSuccessDesc'),
        });
        invalidate();
        onSettled(clarification.id);
        onOpenChange(false);
      },
      onError: () => {
        toast({
          tone: 'error',
          title: t('periodClose.clarifManageErrorTitle'),
          description: t('periodClose.clarifManageErrorDesc'),
        });
      },
    },
  });

  const { mutate: cancel, isPending: isCancelling } = useCancelClarification({
    mutation: {
      onSuccess: () => {
        toast({
          tone: 'success',
          title: t('periodClose.clarifCancelSuccessTitle'),
          description: t('periodClose.clarifCancelSuccessDesc'),
        });
        invalidate();
        onSettled(clarification.id);
        setCancelConfirmOpen(false);
        onOpenChange(false);
      },
      onError: () => {
        toast({
          tone: 'error',
          title: t('periodClose.clarifManageErrorTitle'),
          description: t('periodClose.clarifManageErrorDesc'),
        });
        setCancelConfirmOpen(false);
      },
    },
  });

  const isPending = isResolving || isCancelling;

  return (
    <>
      <Modal open={open && !cancelConfirmOpen} onOpenChange={onOpenChange} size="sm">
        <ModalHeader icon={MessageSquare} title={t('periodClose.clarifManageTitle')} />

        <ModalBody>
          {/* Employee context chip */}
          <div className="mb-3 rounded-lg border border-border bg-surface-2 px-3.5 py-2.5">
            <p className="text-[11px] font-medium text-text-3 uppercase tracking-wide mb-0.5">
              {t('periodClose.clarifTarget')}
            </p>
            <p className="text-[13px] font-semibold text-text">{employeeName}</p>
            <p className="text-[11px] text-text-3 font-mono">{clarification.source_id}</p>
          </div>

          {/* Question */}
          <div className="mb-3">
            <p className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-text-3">
              {t('periodClose.clarifViewQuestion')}
            </p>
            <p className="rounded-lg border border-border bg-surface-2 px-3.5 py-3 text-[13px] leading-relaxed text-text">
              {clarification.question}
            </p>
          </div>

          {/* Answer — or "waiting" empty state */}
          <div className="mb-1">
            <p className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-text-3">
              {t('periodClose.clarifViewAnswer')}
            </p>
            {clarification.answer ? (
              <div className="rounded-lg border border-ok-bd bg-ok-bg px-3.5 py-3">
                <p className="text-[13px] leading-relaxed text-ok-tx">{clarification.answer}</p>
                {clarification.answered_at && (
                  <p className="mt-1.5 text-[11px] text-ok-tx/70">
                    {t('periodClose.clarifAnsweredAt')}{' '}
                    <DateText
                      kind="instant"
                      value={clarification.answered_at}
                      className="text-[11px]"
                      options={{ dateStyle: 'short', timeStyle: 'short' }}
                    />
                  </p>
                )}
              </div>
            ) : (
              <div className="flex items-center gap-2 rounded-lg border border-warn-bd bg-warn-bg px-3.5 py-3">
                <MessageSquare aria-hidden className="size-3.5 shrink-0 text-warn-tx" />
                <p className="text-[12px] text-warn-tx">{t('periodClose.clarifNoAnswer')}</p>
              </div>
            )}
          </div>
        </ModalBody>

        <ModalFooter>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => onOpenChange(false)}
            disabled={isPending}
          >
            {t('common.close', { ns: 'translation' })}
          </Button>

          {/* Only show resolve/cancel when the clarification is still actionable */}
          {isOpen && (
            <>
              {/* Cancel — needs a confirm step (no dead-flow) */}
              <Button
                type="button"
                variant="secondary"
                size="sm"
                data-testid="clarif-manage-cancel"
                onClick={() => setCancelConfirmOpen(true)}
                disabled={isPending}
              >
                <XCircle aria-hidden className="size-3.5" />
                {t('periodClose.clarifCancelBtn')}
              </Button>

              {/* Resolve — closes the loop directly */}
              <Button
                type="button"
                variant="primary"
                size="sm"
                data-testid="clarif-manage-resolve"
                onClick={() => resolve({ id: clarification.id })}
                disabled={isPending}
              >
                <CheckCircle2 aria-hidden className="size-3.5" />
                {isResolving
                  ? t('common.processing', { ns: 'translation' })
                  : t('periodClose.clarifResolveBtn')}
              </Button>
            </>
          )}
        </ModalFooter>
      </Modal>

      {/* Cancel confirm — ConfirmDialog molecule (no dead-flow: cancel needs confirm) */}
      <ConfirmDialog
        open={cancelConfirmOpen}
        onOpenChange={setCancelConfirmOpen}
        icon={XCircle}
        tone="warn"
        title={t('periodClose.clarifCancelConfirmTitle')}
        description={t('periodClose.clarifCancelConfirmBody')}
        cancelLabel={t('common.cancel', { ns: 'translation' })}
        confirmLabel={
          isCancelling
            ? t('common.processing', { ns: 'translation' })
            : t('periodClose.clarifCancelBtn')
        }
        confirmTone="danger"
        onConfirm={() => cancel({ id: clarification.id })}
        loading={isCancelling}
      />
    </>
  );
}
