/**
 * E5 · Koreksi Kehadiran — overlay layer for correction detail drawer + reject modal.
 *
 * .pen frames implemented:
 *   sSKtK  HR · Koreksi · Detail  (CorrectionDetailDrawer)
 *   EnabP  comp/ModalReject        (RejectCorrectionModal)
 *
 * ENGINEERING.md F5.4 · BR-1..BR-5 · INV-1.
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type Correction,
  CorrectionStatus,
  CorrectionType,
  type GetCorrection200,
  useApproveCorrection,
  useGetCorrection,
  useRejectCorrection,
} from '@swp/api-client/e5';
import type { StatusTone } from '@swp/design-tokens';
import {
  Button,
  DateText,
  Drawer,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { AlertTriangle, CheckCircle2, FileText, XCircle } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function cx(...classes: (string | undefined | false | null)[]): string {
  return classes.filter(Boolean).join(' ');
}

export function correctionStatusTone(status: CorrectionStatus): StatusTone {
  switch (status) {
    case CorrectionStatus.PENDING:
      return 'warn';
    case CorrectionStatus.APPROVED:
    case CorrectionStatus.APPLIED:
      return 'ok';
    case CorrectionStatus.REJECTED:
      return 'bad';
    case CorrectionStatus.CANCELLED:
      return 'neutral';
    default:
      return 'neutral';
  }
}

export function correctionTypeLabel(type: CorrectionType, t: (key: string) => string): string {
  switch (type) {
    case CorrectionType.CHECK_IN:
      return t('corrections.typeCheckIn');
    case CorrectionType.CHECK_OUT:
      return t('corrections.typeCheckOut');
    case CorrectionType.CODE:
      return t('corrections.typeCode');
    case CorrectionType.OTHER:
      return t('corrections.typeOther');
    default:
      return type;
  }
}

function formatDiffValue(value: unknown): string {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'string') {
    if (/^\d{4}-\d{2}-\d{2}T/.test(value)) {
      try {
        return new Date(value).toLocaleString('id-ID', { timeZone: 'Asia/Jakarta' });
      } catch {
        return value;
      }
    }
    return value;
  }
  return String(value);
}

// ---------------------------------------------------------------------------
// 1) DiffTable — before/after field diff from correction.diff[] or proposed fields
// ---------------------------------------------------------------------------

interface DiffRow {
  field: string;
  before: unknown;
  after: unknown;
}

function buildDiffRows(correction: Correction, t: (key: string) => string): DiffRow[] {
  if (correction.diff && correction.diff.length > 0) {
    return correction.diff.map((item) => ({
      field: item.field,
      before: item.before,
      after: item.after,
    }));
  }
  const rows: DiffRow[] = [];
  const snap = correction.original_snapshot as Record<string, unknown>;
  if (correction.proposed_check_in_at !== undefined) {
    rows.push({
      field: t('corrections.fieldCheckIn'),
      before: snap?.check_in_at,
      after: correction.proposed_check_in_at,
    });
  }
  if (correction.proposed_check_out_at !== undefined) {
    rows.push({
      field: t('corrections.fieldCheckOut'),
      before: snap?.check_out_at,
      after: correction.proposed_check_out_at,
    });
  }
  if (correction.proposed_attendance_code_id !== undefined) {
    rows.push({
      field: t('corrections.fieldAttendanceCode'),
      before: snap?.attendance_code_id,
      after: correction.proposed_attendance_code_id,
    });
  }
  return rows;
}

interface DiffTableProps {
  correction: Correction;
}

function DiffTable({ correction }: DiffTableProps) {
  const { t } = useTranslation('corrections');
  const rows = buildDiffRows(correction, t);

  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="border-b border-border-soft">
          <th className="py-2 pr-4 text-left font-medium text-text-3">
            {t('corrections.diffField')}
          </th>
          <th className="py-2 pr-4 text-left font-medium text-text-3">
            {t('corrections.diffBefore')}
          </th>
          <th className="py-2 text-left font-medium text-text-3">{t('corrections.diffAfter')}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.field} className="border-b border-border-soft last:border-0">
            <td className="py-2.5 pr-4 font-medium text-text">{row.field}</td>
            <td className="py-2.5 pr-4 text-text-2 line-through opacity-60">
              {formatDiffValue(row.before)}
            </td>
            <td className="py-2.5 font-medium text-ok">{formatDiffValue(row.after)}</td>
          </tr>
        ))}
        {rows.length === 0 && (
          <tr>
            <td colSpan={3} className="py-4 text-center text-text-3">
              —
            </td>
          </tr>
        )}
      </tbody>
    </table>
  );
}

// ---------------------------------------------------------------------------
// 2) RejectCorrectionModal  (.pen comp/ModalReject — EnabP)
// ---------------------------------------------------------------------------

const rejectSchema = z.object({
  reason: z.string().min(5, 'corrections.rejectReasonMin').max(500, 'corrections.rejectReasonMax'),
});
type RejectFormValues = z.infer<typeof rejectSchema>;

export interface RejectCorrectionModalProps {
  open: boolean;
  correctionId: string | null;
  onOpenChange: (open: boolean) => void;
  onDone: () => void;
}

export function RejectCorrectionModal({
  open,
  correctionId,
  onOpenChange,
  onDone,
}: RejectCorrectionModalProps) {
  const { t } = useTranslation('corrections');
  const { toast } = useToast();

  const rejectMutation = useRejectCorrection();

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<RejectFormValues>({ resolver: zodResolver(rejectSchema) });

  useEffect(() => {
    if (!open) reset();
  }, [open, reset]);

  async function onSubmit(values: RejectFormValues) {
    if (!correctionId) return;
    try {
      await rejectMutation.mutateAsync({ id: correctionId, data: { reason: values.reason } });
      toast({ tone: 'success', title: t('corrections.rejectSuccess') });
      onDone();
    } catch (err) {
      if (!applyFieldErrors(err, setError)) {
        const { message } = classifyError(err);
        toast({ tone: 'error', title: t(message) });
      }
    }
  }

  const reasonId = 'reject-reason';

  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalHeader icon={XCircle} tone="danger" title={t('corrections.rejectTitle')} />
      <form onSubmit={handleSubmit(onSubmit)}>
        <ModalBody>
          <p className="mb-4 text-sm text-text-2">{t('corrections.rejectBody')}</p>
          <div className="flex flex-col gap-1.5">
            <label htmlFor={reasonId} className="text-sm font-semibold text-text">
              {t('corrections.rejectReasonLabel')}
              <span aria-hidden className="ml-0.5 text-error">
                *
              </span>
            </label>
            <textarea
              id={reasonId}
              {...register('reason')}
              rows={4}
              aria-describedby={errors.reason ? `${reasonId}-error` : undefined}
              aria-invalid={Boolean(errors.reason)}
              className={cx(
                'w-full resize-none rounded-lg border px-3 py-2 text-sm text-text outline-none',
                'border-border bg-surface placeholder:text-text-3',
                'focus:border-primary focus:ring-1 focus:ring-primary',
                errors.reason && 'border-error focus:border-error focus:ring-error',
              )}
              placeholder={t('corrections.rejectReasonPlaceholder')}
            />
            {errors.reason?.message && (
              <p id={`${reasonId}-error`} role="alert" className="text-xs text-error">
                {t(errors.reason.message)}
              </p>
            )}
          </div>
        </ModalBody>
        <ModalFooter>
          <Button
            type="button"
            variant="ghost"
            onClick={() => onOpenChange(false)}
            disabled={isSubmitting}
          >
            {t('corrections.cancel')}
          </Button>
          <Button type="submit" variant="destructive" disabled={isSubmitting}>
            <XCircle aria-hidden className="size-4" />
            {isSubmitting ? t('corrections.rejecting') : t('corrections.rejectConfirm')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// 3) CorrectionDetailDrawer  (.pen frame sSKtK — HR · Koreksi · Detail)
//    Approve is inline; Reject opens RejectCorrectionModal.
// ---------------------------------------------------------------------------

export interface CorrectionDetailDrawerProps {
  open: boolean;
  correctionId: string | null;
  onOpenChange: (open: boolean) => void;
  onDone: () => void;
}

export function CorrectionDetailDrawer({
  open,
  correctionId,
  onOpenChange,
  onDone,
}: CorrectionDetailDrawerProps) {
  const { t } = useTranslation('corrections');
  const { toast } = useToast();

  const [rejectOpen, setRejectOpen] = useState(false);

  const query = useGetCorrection(correctionId ?? '', {
    query: { enabled: open && Boolean(correctionId) },
  });

  const approveMutation = useApproveCorrection();

  async function handleApprove(id: string) {
    try {
      await approveMutation.mutateAsync({ id, data: {} });
      toast({ tone: 'success', title: t('corrections.approveSuccess') });
      onDone();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t(message) });
    }
  }

  function handleRejectDone() {
    setRejectOpen(false);
    onDone();
  }

  const page = query.data?.data as GetCorrection200 | undefined;
  const correction = page?.data;
  const isPending = correction?.status === CorrectionStatus.PENDING;
  const isApproving = approveMutation.isPending;

  return (
    <>
      <Drawer open={open} onOpenChange={onOpenChange} width={560}>
        <DrawerHeader
          title={
            correction
              ? `${t('corrections.detailTitle')} · ${correction.id}`
              : t('corrections.detailTitle')
          }
        />

        <DrawerBody>
          {query.isLoading && <StateView kind="loading" title={t('corrections.loading')} />}

          {query.isError &&
            (() => {
              const { kind } = classifyError(query.error);
              if (kind === 'forbidden' || kind === 'unauthenticated') {
                return (
                  <StateView
                    kind="empty"
                    title={t('errors.forbidden')}
                    description={t('corrections.noPermissionBody')}
                  />
                );
              }
              return (
                <StateView
                  kind="error"
                  title={t('corrections.errorTitle')}
                  description={t('errors.network')}
                  onRetry={() => query.refetch()}
                  retryLabel={t('common.retry')}
                />
              );
            })()}

          {correction && (
            <div className="flex flex-col gap-6">
              {/* Header — requester + status */}
              <div className="flex items-center justify-between">
                <div className="flex flex-col gap-0.5">
                  <span className="font-semibold text-text">{correction.requester_id}</span>
                  <span className="font-mono text-xs text-text-3">{correction.attendance_id}</span>
                </div>
                <StatusBadge dot tone={correctionStatusTone(correction.status)}>
                  {t(`corrections.status.${correction.status}`)}
                </StatusBadge>
              </div>

              {/* Meta grid */}
              <div className="grid grid-cols-2 gap-4 rounded-xl border border-border bg-surface p-4 text-sm">
                <div className="flex flex-col gap-1">
                  <span className="text-xs text-text-3">{t('corrections.metaType')}</span>
                  <span className="font-medium text-text">
                    {correctionTypeLabel(correction.type, t)}
                  </span>
                </div>
                <div className="flex flex-col gap-1">
                  <span className="text-xs text-text-3">{t('corrections.metaSubmitted')}</span>
                  <DateText
                    kind="instant"
                    value={correction.created_at}
                    className="font-medium text-text"
                  />
                </div>
                {correction.decided_at && (
                  <div className="flex flex-col gap-1">
                    <span className="text-xs text-text-3">{t('corrections.metaDecided')}</span>
                    <DateText
                      kind="instant"
                      value={correction.decided_at as string}
                      className="font-medium text-text"
                    />
                  </div>
                )}
                {correction.decided_by && (
                  <div className="flex flex-col gap-1">
                    <span className="text-xs text-text-3">{t('corrections.metaDecidedBy')}</span>
                    <span className="font-medium text-text">{correction.decided_by as string}</span>
                  </div>
                )}
              </div>

              {/* Requester reason */}
              <div className="flex flex-col gap-2">
                <span className="text-sm font-medium text-text-2">
                  {t('corrections.requesterReason')}
                </span>
                <p className="rounded-lg border border-border-soft bg-surface p-3 text-sm text-text">
                  {correction.reason}
                </p>
              </div>

              {/* Before → After diff */}
              <div className="flex flex-col gap-2">
                <span className="text-sm font-medium text-text-2">
                  {t('corrections.diffLabel')}
                </span>
                <div className="overflow-x-auto rounded-xl border border-border bg-surface p-4">
                  <DiffTable correction={correction} />
                </div>
              </div>

              {/* Evidence file */}
              {correction.evidence_file_id && (
                <div className="flex items-center gap-2 rounded-lg border border-border-soft bg-surface p-3">
                  <FileText aria-hidden className="size-4 text-text-3" />
                  <span className="text-sm text-text-2">{t('corrections.evidenceFile')}</span>
                  <span className="font-mono text-xs text-text-3">
                    {correction.evidence_file_id as string}
                  </span>
                </div>
              )}

              {/* Reject reason */}
              {correction.reject_reason && (
                <div className="flex items-start gap-2 rounded-lg border border-error/20 bg-error/5 p-3">
                  <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0 text-error" />
                  <div className="flex flex-col gap-0.5">
                    <span className="text-sm font-medium text-error">
                      {t('corrections.rejectReasonLabel')}
                    </span>
                    <span className="text-sm text-text-2">{correction.reject_reason}</span>
                  </div>
                </div>
              )}

              {/* Applied notice */}
              {correction.status === CorrectionStatus.APPLIED && (
                <div className="flex items-center gap-2 rounded-lg border border-ok/20 bg-ok/5 p-3">
                  <CheckCircle2 aria-hidden className="size-4 text-ok" />
                  <span className="text-sm font-medium text-ok">
                    {t('corrections.appliedNote')}
                  </span>
                </div>
              )}
            </div>
          )}
        </DrawerBody>

        {correction && isPending && (
          <DrawerFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
              disabled={isApproving}
            >
              {t('corrections.cancel')}
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setRejectOpen(true)}
              disabled={isApproving}
            >
              <XCircle aria-hidden className="size-4" />
              {t('corrections.reject')}
            </Button>
            <Button
              type="button"
              disabled={isApproving}
              onClick={() => handleApprove(correction.id)}
            >
              <CheckCircle2 aria-hidden className="size-4" />
              {isApproving ? t('corrections.approving') : t('corrections.approve')}
            </Button>
          </DrawerFooter>
        )}
      </Drawer>

      <RejectCorrectionModal
        open={rejectOpen}
        correctionId={correctionId}
        onOpenChange={setRejectOpen}
        onDone={handleRejectDone}
      />
    </>
  );
}
