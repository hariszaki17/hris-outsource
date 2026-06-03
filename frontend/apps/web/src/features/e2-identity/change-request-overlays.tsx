/**
 * E2 · Antrian Persetujuan Perubahan Data — overlay layer for HR review actions.
 *
 * .pen frames implemented from `Ckteo` (Antrian Persetujuan — HR):
 *   ChangeRequestDetailDrawer  — before→after diff + Approve / Reject CTAs
 *   RejectReasonModal           — modal with required reason textarea
 *
 * EP-5 · F2.x
 */

import { classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type ChangeRequestDetail,
  type ChangeRequestRequestType,
  ChangeRequestStatus,
  type RejectChangeRequestBody,
  useApproveChangeRequest,
  useGetChangeRequest,
  useRejectChangeRequest,
} from '@swp/api-client/e2';
import {
  Avatar,
  Banner,
  Button,
  DateText,
  Drawer,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  FormField,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { Check, CircleX, Clock, RefreshCw } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function initials(name: string): string {
  return name
    .split(' ')
    .slice(0, 2)
    .map((p) => p[0] ?? '')
    .join('')
    .toUpperCase();
}

function formatFieldName(key: string): string {
  const map: Record<string, string> = {
    phone: 'Telepon',
    address: 'Alamat',
    bank_account: 'Rekening Bank',
    bank_name: 'Nama Bank',
    account_number: 'Nomor Rekening',
    account_holder_name: 'Atas Nama',
  };
  return map[key] ?? key;
}

function formatDiffValue(val: unknown): string {
  if (val === null || val === undefined) return '—';
  if (typeof val === 'object') {
    const obj = val as Record<string, unknown>;
    const parts: string[] = [];
    if (obj.bank_name) parts.push(String(obj.bank_name));
    if (obj.account_number) parts.push(String(obj.account_number));
    if (obj.account_holder_name) parts.push(`(${String(obj.account_holder_name)})`);
    return parts.join(' · ') || JSON.stringify(val);
  }
  return String(val);
}

// ---------------------------------------------------------------------------
// DiffRow — single before→after field comparison
// ---------------------------------------------------------------------------

interface DiffRowProps {
  fieldKey: string;
  oldVal: unknown;
  newVal: unknown;
}

function DiffRow({ fieldKey, oldVal, newVal }: DiffRowProps) {
  return (
    <div className="flex flex-col gap-1.5 rounded-md border border-border-soft bg-surface-2 px-3 py-2.5">
      <span className="text-[11px] font-semibold uppercase tracking-wider text-text-3">
        {formatFieldName(fieldKey)}
      </span>
      <div className="flex flex-col gap-1 sm:flex-row sm:items-start sm:gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="text-[10px] uppercase tracking-wider text-text-3">Sebelumnya</span>
          <span className="break-words text-sm text-text-2 line-through decoration-bad-tx">
            {formatDiffValue(oldVal)}
          </span>
        </div>
        <div className="hidden pt-1 text-text-3 sm:block" aria-hidden>
          →
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="text-[10px] uppercase tracking-wider text-text-3">Menjadi</span>
          <span className="break-words text-sm font-medium text-text">
            {formatDiffValue(newVal)}
          </span>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Type helper — exported so screen can narrow request_type labels
// ---------------------------------------------------------------------------

export function requestTypeLabel(type: ChangeRequestRequestType | undefined): string {
  const map: Record<string, string> = {
    PHONE: 'Telepon',
    ADDRESS: 'Alamat',
    BANK_ACCOUNT: 'Rekening Bank',
    MULTIPLE: 'Beberapa Field',
  };
  return map[type ?? ''] ?? type ?? '—';
}

// ---------------------------------------------------------------------------
// RejectReasonModal — wave-1 ModalReject pattern
// ---------------------------------------------------------------------------

const rejectSchema = z.object({
  reason: z.string().min(3, 'Alasan minimal 3 karakter').max(500, 'Alasan maksimal 500 karakter'),
});

type RejectForm = z.infer<typeof rejectSchema>;

export interface RejectReasonModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  changeRequestId: string | null;
  employeeName?: string;
  onDone: () => void;
}

export function RejectReasonModal({
  open,
  onOpenChange,
  changeRequestId,
  employeeName,
  onDone,
}: RejectReasonModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const mutation = useRejectChangeRequest();
  const [serverError, setServerError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<RejectForm>({
    resolver: zodResolver(rejectSchema),
  });

  function handleClose() {
    reset();
    setServerError(null);
    onOpenChange(false);
  }

  function onSubmit(values: RejectForm) {
    if (!changeRequestId) return;
    setServerError(null);

    const body: RejectChangeRequestBody = { reason: values.reason };
    mutation.mutate(
      { changeRequestId, data: body },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('changeRequests.rejectSuccess') });
          handleClose();
          onDone();
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t(message as never) ?? message });
        },
      },
    );
  }

  const saving = isSubmitting || mutation.isPending;

  return (
    <Modal open={open} onOpenChange={handleClose} size="md">
      <ModalHeader
        icon={CircleX}
        tone="danger"
        title={t('changeRequests.rejectModalTitle')}
        closeLabel={t('changeRequests.close')}
      />

      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalBody>
          {employeeName && (
            <p className="text-[13px] text-text-2">
              {t('changeRequests.rejectModalSubtitle', { name: employeeName })}
            </p>
          )}

          {serverError && <Banner tone="bad" title={serverError} />}

          <FormField
            label={t('changeRequests.rejectReasonLabel')}
            htmlFor="rr-reason"
            required
            hint={t('changeRequests.rejectReasonHint')}
            error={errors.reason?.message}
          >
            <textarea
              id="rr-reason"
              rows={4}
              placeholder={t('changeRequests.rejectReasonPlaceholder')}
              aria-invalid={errors.reason ? true : undefined}
              disabled={saving}
              className="flex w-full resize-y rounded-md border border-input bg-background px-3 py-2 text-sm text-text placeholder:text-text-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50 aria-[invalid=true]:border-bad-bd"
              {...register('reason')}
            />
          </FormField>
        </ModalBody>

        <ModalFooter>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={saving}
            onClick={handleClose}
          >
            {t('changeRequests.cancel')}
          </Button>
          <Button
            type="submit"
            variant="destructive"
            size="sm"
            disabled={saving}
            aria-busy={saving}
          >
            <CircleX aria-hidden />
            {t('changeRequests.rejectConfirm')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ChangeRequestDetailDrawer — main HR review surface
// ---------------------------------------------------------------------------

export interface ChangeRequestDetailDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  changeRequestId: string | null;
  onDone: () => void;
}

export function ChangeRequestDetailDrawer({
  open,
  onOpenChange,
  changeRequestId,
  onDone,
}: ChangeRequestDetailDrawerProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const [rejectOpen, setRejectOpen] = useState(false);

  const detailQuery = useGetChangeRequest(changeRequestId ?? '', {
    query: { enabled: open && !!changeRequestId },
  });

  const approveMutation = useApproveChangeRequest();

  function handleApprove() {
    if (!changeRequestId) return;
    approveMutation.mutate(
      { changeRequestId },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('changeRequests.approveSuccess') });
          onOpenChange(false);
          onDone();
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t(message as never) ?? message });
        },
      },
    );
  }

  function handleRejectDone() {
    setRejectOpen(false);
    onOpenChange(false);
    onDone();
  }

  const detail = detailQuery.data?.data as ChangeRequestDetail | undefined;
  const employee = detail?.employee;
  const isPending = detail?.status === ChangeRequestStatus.PENDING;
  const approving = approveMutation.isPending;

  const diffEntries = detail?.diff ? Object.entries(detail.diff) : [];

  return (
    <>
      <Drawer open={open} onOpenChange={onOpenChange} width={560}>
        <DrawerHeader
          title={t('changeRequests.detailDrawerTitle')}
          subtitle={changeRequestId ?? undefined}
          closeLabel={t('changeRequests.close')}
        />

        <DrawerBody>
          {detailQuery.isLoading && (
            <div className="flex items-center justify-center py-12">
              <RefreshCw className="size-5 animate-spin text-text-3" aria-hidden />
              <span className="ml-2 text-sm text-text-3">{t('changeRequests.loading')}</span>
            </div>
          )}

          {detailQuery.isError && (
            <div className="flex flex-col gap-2">
              <Banner tone="bad" title={t('changeRequests.detailLoadError')} />
              <Button type="button" variant="ghost" size="sm" onClick={() => detailQuery.refetch()}>
                {t('changeRequests.retry')}
              </Button>
            </div>
          )}

          {detail && (
            <div className="flex flex-col gap-5">
              {/* Submitter block */}
              <div className="flex items-center gap-3 rounded-lg border border-border-soft bg-surface-2 px-3 py-2.5">
                {employee?.full_name && (
                  <Avatar initials={initials(employee.full_name)} size={40} />
                )}
                <div className="flex min-w-0 flex-col gap-0.5">
                  <span className="font-semibold text-sm text-text">
                    {employee?.full_name ?? '—'}
                  </span>
                  <span className="font-mono text-xs text-text-3">
                    {employee?.nip ?? ''} · {detail.id}
                  </span>
                </div>
                <div className="ml-auto shrink-0">
                  <StatusBadge
                    dot
                    tone={
                      detail.status === ChangeRequestStatus.APPROVED
                        ? 'ok'
                        : detail.status === ChangeRequestStatus.REJECTED
                          ? 'bad'
                          : 'warn'
                    }
                  >
                    {t(`changeRequests.status.${detail.status}`)}
                  </StatusBadge>
                </div>
              </div>

              {/* Meta row */}
              <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[12px] text-text-3">
                <span className="flex items-center gap-1">
                  <Clock className="size-3.5 shrink-0" aria-hidden />
                  {t('changeRequests.submittedAt')}:{' '}
                  <DateText kind="instant" value={detail.submitted_at} className="inline" />
                </span>
                {detail.resolved_at && (
                  <span className="flex items-center gap-1">
                    <Check className="size-3.5 shrink-0" aria-hidden />
                    {t('changeRequests.resolvedAt')}:{' '}
                    <DateText kind="instant" value={detail.resolved_at} className="inline" />
                  </span>
                )}
              </div>

              {/* Agent note */}
              {detail.note && (
                <div className="rounded-md border border-border-soft bg-surface-2 px-3 py-2">
                  <p className="text-[11px] font-semibold uppercase tracking-wider text-text-3">
                    {t('changeRequests.agentNote')}
                  </p>
                  <p className="mt-1 text-sm text-text-2">{detail.note}</p>
                </div>
              )}

              {/* Rejection reason (if rejected) */}
              {detail.status === ChangeRequestStatus.REJECTED && detail.rejection_reason && (
                <div className="rounded-md border border-bad-bd bg-bad-bg px-3 py-2">
                  <p className="text-[11px] font-semibold uppercase tracking-wider text-bad-tx">
                    {t('changeRequests.rejectionReason')}
                  </p>
                  <p className="mt-1 text-sm text-bad-tx">{detail.rejection_reason}</p>
                </div>
              )}

              {/* Diff section */}
              <div className="flex flex-col gap-2">
                <p className="text-[11px] font-bold uppercase tracking-wider text-text-3">
                  {t('changeRequests.diffSectionTitle')}
                </p>
                {diffEntries.length === 0 ? (
                  <p className="text-sm text-text-3">{t('changeRequests.noDiffData')}</p>
                ) : (
                  <div className="flex flex-col gap-2">
                    {diffEntries.map(([key, entry]) => (
                      <DiffRow key={key} fieldKey={key} oldVal={entry.old} newVal={entry.new} />
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
        </DrawerBody>

        {isPending && (
          <DrawerFooter className="justify-between">
            <Button
              type="button"
              variant="destructive"
              size="sm"
              disabled={approving}
              onClick={() => setRejectOpen(true)}
            >
              <CircleX aria-hidden />
              {t('changeRequests.rejectAction')}
            </Button>
            <Button
              type="button"
              variant="primary"
              size="sm"
              disabled={approving}
              aria-busy={approving}
              onClick={handleApprove}
            >
              <Check aria-hidden />
              {t('changeRequests.approveAction')}
            </Button>
          </DrawerFooter>
        )}
      </Drawer>

      <RejectReasonModal
        open={rejectOpen}
        onOpenChange={setRejectOpen}
        changeRequestId={changeRequestId}
        employeeName={detail?.employee?.full_name}
        onDone={handleRejectDone}
      />
    </>
  );
}
