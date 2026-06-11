/**
 * E2 · Antrian Persetujuan Perubahan Data — overlay layer for review actions.
 *
 * .pen frames implemented from `Ckteo` / `MdhFZ` / `CATOn` / `jg8zl`
 * (Antrian Persetujuan — HR + Shift Leader):
 *   ChangeRequestDetailDrawer  — before→after per-field diff + bank-split gating
 *                                 + Approve / Reject CTAs
 *   RejectChangeRequestModal   — modal with required reason textarea (mirrors RejectLeaveModal)
 *
 * Bank-split (employee-profile.md): a shift leader (company-scoped) may approve the
 * non-bank fields, but the `bank_account` field needs `change_requests.approve.bank`
 * (HR/super-admin only). Approving a mixed request as an SL applies the non-bank fields
 * and escalates the bank field to HR — the request lands on PARTIALLY_APPROVED.
 *
 * EP-5 · F2.x
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type ChangeRequest,
  type ChangeRequestDetail,
  type ChangeRequestRequestType,
  ChangeRequestStatus,
  FieldResolution,
  type RejectChangeRequestBody,
  useApproveChangeRequest,
  useGetChangeRequest,
  useRejectChangeRequest,
} from '@swp/api-client/e2';
import type { StatusTone } from '@swp/design-tokens';
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
import { Check, CircleX, Clock, Landmark, RefreshCw } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** The bank-account field is the only field gated by `change_requests.approve.bank`. */
const BANK_FIELD_KEY = 'bank_account';

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
    emergency_contact: 'Kontak Darurat',
    bank_account: 'Rekening Bank',
    bank_name: 'Nama Bank',
    account_number: 'Nomor Rekening',
    account_holder_name: 'Atas Nama',
    name: 'Nama',
  };
  return map[key] ?? key;
}

function formatDiffValue(val: unknown): string {
  if (val === null || val === undefined) return '—';
  if (typeof val === 'object') {
    const obj = val as Record<string, unknown>;
    const parts: string[] = [];
    // Bank account object
    if (obj.bank_name) parts.push(String(obj.bank_name));
    if (obj.account_number) parts.push(String(obj.account_number));
    if (obj.account_holder_name) parts.push(`(${String(obj.account_holder_name)})`);
    // Emergency contact object {name, phone}
    if (obj.name) parts.push(String(obj.name));
    if (obj.phone) parts.push(String(obj.phone));
    return parts.join(' · ') || JSON.stringify(val);
  }
  return String(val);
}

/** Map a change-request status to a status-badge tone. */
function changeRequestTone(status: ChangeRequestStatus): StatusTone {
  switch (status) {
    case ChangeRequestStatus.APPROVED:
      return 'ok';
    case ChangeRequestStatus.REJECTED:
      return 'bad';
    case ChangeRequestStatus.PARTIALLY_APPROVED:
      return 'onprogress';
    default:
      return 'warn';
  }
}

// ---------------------------------------------------------------------------
// DiffRow — single before→after field comparison (bank-split aware)
// ---------------------------------------------------------------------------

interface DiffRowProps {
  fieldKey: string;
  oldVal: unknown;
  newVal: unknown;
  /** Per-field resolution (APPLIED / ESCALATED_TO_HR / PENDING) once acted on. */
  resolution?: FieldResolution;
  /**
   * True when this row is gated behind `change_requests.approve.bank` and the current
   * reviewer lacks it — the row is shown read-only with a "Perlu HR" badge + escalate hint.
   */
  bankGated: boolean;
}

function DiffRow({ fieldKey, oldVal, newVal, resolution, bankGated }: DiffRowProps) {
  const { t } = useTranslation();

  const resolutionBadge =
    resolution === FieldResolution.APPLIED ? (
      <StatusBadge dot tone="ok">
        {t('changeRequests.fieldApplied')}
      </StatusBadge>
    ) : resolution === FieldResolution.ESCALATED_TO_HR ? (
      <StatusBadge dot tone="onprogress">
        {t('changeRequests.fieldEscalated')}
      </StatusBadge>
    ) : null;

  return (
    <div
      className={[
        'flex flex-col gap-1.5 rounded-md border px-3 py-2.5',
        bankGated ? 'border-warn-bd bg-warn-bg/40' : 'border-border-soft bg-surface-2',
      ].join(' ')}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wider text-text-3">
          {fieldKey === BANK_FIELD_KEY && <Landmark className="size-3.5 text-text-3" aria-hidden />}
          {formatFieldName(fieldKey)}
        </span>
        {/* Resolution state wins; otherwise the bank-gate "Perlu HR" badge. */}
        {resolutionBadge ??
          (bankGated ? (
            <StatusBadge dot tone="warn">
              {t('changeRequests.bankNeedsHrBadge')}
            </StatusBadge>
          ) : null)}
      </div>
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
      {bankGated && !resolution && (
        <p className="text-[11px] text-warn-tx">{t('changeRequests.bankEscalatedNote')}</p>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Type helper — exported so screen can narrow request_type labels
// ---------------------------------------------------------------------------

export function requestTypeLabel(type: ChangeRequestRequestType | undefined): string {
  const map: Record<string, string> = {
    PHONE: 'Telepon',
    EMERGENCY_CONTACT: 'Kontak Darurat',
    BANK_ACCOUNT: 'Rekening Bank',
    MULTIPLE: 'Beberapa Field',
  };
  return map[type ?? ''] ?? type ?? '—';
}

// ---------------------------------------------------------------------------
// RejectChangeRequestModal — mirrors RejectLeaveModal (jg8zl)
// ---------------------------------------------------------------------------

const rejectSchema = z.object({
  reason: z.string().min(3, 'Alasan minimal 3 karakter').max(500, 'Alasan maksimal 500 karakter'),
});

type RejectForm = z.infer<typeof rejectSchema>;

export interface RejectChangeRequestModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  changeRequestId: string | null;
  employeeName?: string;
  onDone: () => void;
}

export function RejectChangeRequestModal({
  open,
  onOpenChange,
  changeRequestId,
  employeeName,
  onDone,
}: RejectChangeRequestModalProps) {
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
            htmlFor="reject-reason"
            required
            hint={t('changeRequests.rejectReasonHint')}
            error={errors.reason?.message}
          >
            <textarea
              id="reject-reason"
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
// ChangeRequestDetailDrawer — main review surface (HR + Shift Leader)
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
  const user = useCurrentUser();
  const [rejectOpen, setRejectOpen] = useState(false);

  // Bank-split gate (defense-in-depth; the API is the real gate). Reviewers without
  // `change_requests.approve.bank` (i.e. shift leaders) can apply non-bank fields only —
  // the bank field escalates to HR.
  const canApproveBank = user?.permissions.includes('change_requests.approve.bank') ?? false;

  const detailQuery = useGetChangeRequest(changeRequestId ?? '', {
    query: { enabled: open && !!changeRequestId },
  });

  const approveMutation = useApproveChangeRequest();

  const detail = detailQuery.data?.data as ChangeRequestDetail | undefined;
  const employee = detail?.employee;
  // Both PENDING and PARTIALLY_APPROVED still need a decision: a partially-approved
  // request has its non-bank fields applied but the bank field escalated to HR, who
  // finalizes it from the same review footer.
  const isPending =
    detail?.status === ChangeRequestStatus.PENDING ||
    detail?.status === ChangeRequestStatus.PARTIALLY_APPROVED;
  const approving = approveMutation.isPending;

  const diffEntries = detail?.diff ? Object.entries(detail.diff) : [];
  const fieldResolutions = detail?.field_resolutions ?? {};

  // Does this request touch the bank field at all?
  const hasBankField = diffEntries.some(([key]) => key === BANK_FIELD_KEY);
  // Are there any non-bank fields the reviewer CAN apply right now?
  const hasNonBankField = diffEntries.some(([key]) => key !== BANK_FIELD_KEY);

  function handleApprove() {
    if (!changeRequestId) return;
    approveMutation.mutate(
      { changeRequestId },
      {
        onSuccess: (res) => {
          // The approve response carries the resulting status: a shift leader approving a mixed
          // request lands on PARTIALLY_APPROVED (non-bank applied, bank escalated to HR).
          const status = (res?.data as ChangeRequest | undefined)?.status;
          const partial = status === ChangeRequestStatus.PARTIALLY_APPROVED;
          toast({
            tone: 'success',
            title: partial
              ? t('changeRequests.partialApproveToast')
              : t('changeRequests.approveSuccess'),
          });
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

  // Approve CTA copy: when the reviewer lacks the bank permission AND the request mixes
  // a bank field with non-bank fields, approving only applies the non-bank fields.
  const approveLabel =
    !canApproveBank && hasBankField && hasNonBankField
      ? t('changeRequests.approveNonBankAction')
      : t('changeRequests.approveAction');

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
                  <StatusBadge dot tone={changeRequestTone(detail.status)}>
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

              {/* Bank-escalation banner: this reviewer can't finalize the bank field. */}
              {isPending && hasBankField && !canApproveBank && (
                <Banner
                  tone="warn"
                  title={t('changeRequests.bankGateBannerTitle')}
                  description={t('changeRequests.bankGateBannerBody')}
                />
              )}

              {/* Awaiting-HR banner: a leader already applied non-bank; bank still pending. */}
              {detail.bank_pending && (
                <Banner
                  tone="info"
                  title={t('changeRequests.bankPendingBannerTitle')}
                  description={t('changeRequests.bankPendingBannerBody')}
                />
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
                      <DiffRow
                        key={key}
                        fieldKey={key}
                        oldVal={entry.old}
                        newVal={entry.new}
                        resolution={fieldResolutions[key]?.resolution}
                        bankGated={key === BANK_FIELD_KEY && !canApproveBank}
                      />
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
              {/* Bank-only request a leader can't finalize → approving escalates it to HR. */}
              {!canApproveBank && hasBankField && !hasNonBankField
                ? t('changeRequests.escalateToHrAction')
                : approveLabel}
            </Button>
          </DrawerFooter>
        )}
      </Drawer>

      <RejectChangeRequestModal
        open={rejectOpen}
        onOpenChange={setRejectOpen}
        changeRequestId={changeRequestId}
        employeeName={detail?.employee?.full_name}
        onDone={handleRejectDone}
      />
    </>
  );
}
