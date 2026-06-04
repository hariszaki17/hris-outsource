/**
 * E2 · Perjanjian Kerja — Detail
 *
 * .pen frame: Cu0qg  "E2 · Perjanjian Kerja — Detail (PKT-2026-0042)"
 *
 * Design: BackRow → HeaderCard (nomor+tipe+status, employee sub, renew/close actions) →
 * Two-column layout:
 *   L: Card Detail Perjanjian | Card Riwayat Kompensasi
 *   R: Card Berkas Perjanjian | Card Rantai Perjanjian (successor/predecessor)
 *
 * Actions:
 *   - Perpanjang (Renew) — opens RenewDrawer
 *   - Tutup Perjanjian (Close) — opens CloseModal (ConfirmDialog + inline form)
 *   - Upload lampiran — file input stub → useUploadAgreementAttachment
 *
 * F2.2 EA-3 (renew/successor chain), EA-4 (compensation masking), EA-5 (close).
 * ENGINEERING.md A2 — RBAC defense-in-depth, API is the real gate.
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  AgreementStatus,
  AgreementType,
  type CloseAgreementBody,
  CloseAgreementBodyReason,
  type RenewAgreementBody,
  useCloseAgreement,
  useGetAgreement,
  useRenewAgreement,
  useUploadAgreementAttachment,
} from '@swp/api-client/e2';
import {
  Button,
  DateText,
  Drawer,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  EmptyState,
  FilterSelect,
  FormField,
  FormSection,
  Input,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { Link, useNavigate } from '@tanstack/react-router';
import {
  ArrowLeft,
  Download,
  Eye,
  GitBranch,
  Lock,
  Paperclip,
  RefreshCw,
  XCircle,
} from 'lucide-react';
import { useState } from 'react';
import type React from 'react';
import { useRef } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface AgreementDetailScreenProps {
  agreementId: string;
}

// ---------------------------------------------------------------------------
// KV row helper (matches Cu0qg Y0sYF8 detail cards)
// ---------------------------------------------------------------------------

function KvRow({
  label,
  value,
  mono,
  children,
}: {
  label: string;
  value?: string | null;
  mono?: boolean;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-t border-border-soft px-5 py-[10px]">
      <span className="text-[12px] font-medium text-text-2">{label}</span>
      {children ?? (
        <span
          className={['text-[13px] text-text', mono ? 'font-mono font-medium' : 'font-normal'].join(
            ' ',
          )}
        >
          {value ?? '—'}
        </span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// DetailCard wrapper
// ---------------------------------------------------------------------------

function DetailCard({
  title,
  titleIcon,
  children,
  action,
}: {
  title: string;
  titleIcon?: React.ReactNode;
  children: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-border bg-surface">
      <div className="flex items-center justify-between px-5 py-[16px] pb-[12px]">
        <div className="flex items-center gap-[8px]">
          {titleIcon}
          <span className="text-[14px] font-semibold text-text">{title}</span>
        </div>
        {action}
      </div>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Renew Zod schema + Drawer form
// ---------------------------------------------------------------------------

const renewSchema = z
  .object({
    type: z.nativeEnum(AgreementType),
    agreement_no: z.string().optional(),
    start_date: z.string().min(1, 'Tanggal mulai wajib diisi'),
    end_date: z.string().optional(),
    note: z.string().optional(),
  })
  .superRefine((data, ctx) => {
    if (data.type === AgreementType.PKWT && !data.end_date) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Tanggal akhir wajib untuk PKWT',
        path: ['end_date'],
      });
    }
  });

type RenewFormValues = z.infer<typeof renewSchema>;

interface RenewDrawerProps {
  agreementId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}

function RenewDrawer({ agreementId, open, onOpenChange, onSuccess }: RenewDrawerProps) {
  const { t } = useTranslation('agreements');
  const { toast } = useToast();
  const renewMutation = useRenewAgreement();

  const {
    register,
    handleSubmit,
    watch,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<RenewFormValues>({
    resolver: zodResolver(renewSchema),
    defaultValues: { type: AgreementType.PKWT },
  });

  const selectedType = watch('type');

  async function onSubmit(values: RenewFormValues) {
    const body: RenewAgreementBody = {
      type: values.type,
      start_date: values.start_date,
      end_date: values.type === AgreementType.PKWT ? values.end_date : undefined,
      agreement_no: values.agreement_no || undefined,
      note: values.note || undefined,
    };
    try {
      await renewMutation.mutateAsync({ agreementId, data: body });
      toast({ tone: 'success', title: t('renewSuccess') });
      onSuccess();
      onOpenChange(false);
    } catch (err) {
      const { kind, message } = classifyError(err);
      if (kind === 'validation') {
        applyFieldErrors(err, setError);
      } else {
        toast({ tone: 'error', title: t('renewError'), description: message });
      }
    }
  }

  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerHeader title={t('renewTitle')} />
      <DrawerBody>
        <form id="renew-form" onSubmit={handleSubmit(onSubmit)} noValidate>
          <FormSection>
            <FormField
              label={t('fieldType')}
              htmlFor="renew_type"
              required
              error={errors.type?.message}
            >
              <FilterSelect id="renew_type" aria-label={t('fieldType')} {...register('type')}>
                <option value={AgreementType.PKWT}>PKWT ({t('pkwtLabel')})</option>
                <option value={AgreementType.PKWTT}>PKWTT ({t('pkwttLabel')})</option>
              </FilterSelect>
            </FormField>

            <FormField
              label={t('fieldAgreementNo')}
              htmlFor="renew_agreement_no"
              error={errors.agreement_no?.message}
            >
              <Input
                id="renew_agreement_no"
                placeholder="PKWT/SWP/2027/0142"
                {...register('agreement_no')}
              />
            </FormField>

            <FormField
              label={t('fieldStartDate')}
              htmlFor="renew_start_date"
              required
              error={errors.start_date?.message}
            >
              <Input
                id="renew_start_date"
                type="date"
                {...register('start_date')}
                aria-invalid={!!errors.start_date}
              />
            </FormField>

            {selectedType === AgreementType.PKWT && (
              <FormField
                label={t('fieldEndDate')}
                htmlFor="renew_end_date"
                required
                error={errors.end_date?.message}
              >
                <Input
                  id="renew_end_date"
                  type="date"
                  {...register('end_date')}
                  aria-invalid={!!errors.end_date}
                />
              </FormField>
            )}

            <FormField label={t('fieldNote')} htmlFor="renew_note" error={errors.note?.message}>
              <Input
                id="renew_note"
                placeholder={t('fieldNotePlaceholder')}
                {...register('note')}
              />
            </FormField>
          </FormSection>
        </form>
      </DrawerBody>
      <DrawerFooter>
        <Button
          type="button"
          variant="secondary"
          onClick={() => onOpenChange(false)}
          disabled={isSubmitting}
        >
          {t('common.cancel', { ns: 'translation' })}
        </Button>
        <Button type="submit" form="renew-form" disabled={isSubmitting}>
          {isSubmitting ? t('common.loading', { ns: 'translation' }) : t('renewSubmit')}
        </Button>
      </DrawerFooter>
    </Drawer>
  );
}

// ---------------------------------------------------------------------------
// Close confirmation — uses ConfirmDialog (G4 pattern) with an inline form
// ---------------------------------------------------------------------------

const closeSchema = z.object({
  reason: z.nativeEnum(CloseAgreementBodyReason),
  effective_date: z.string().min(1, 'Tanggal efektif wajib diisi'),
  note: z.string().max(500).optional(),
});

type CloseFormValues = z.infer<typeof closeSchema>;

interface CloseDrawerProps {
  agreementId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}

function CloseDrawer({ agreementId, open, onOpenChange, onSuccess }: CloseDrawerProps) {
  const { t } = useTranslation('agreements');
  const { toast } = useToast();
  const closeMutation = useCloseAgreement();

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<CloseFormValues>({
    resolver: zodResolver(closeSchema),
    defaultValues: { reason: CloseAgreementBodyReason.END_OF_TERM },
  });

  async function onSubmit(values: CloseFormValues) {
    const body: CloseAgreementBody = {
      reason: values.reason,
      effective_date: values.effective_date,
      note: values.note || undefined,
    };
    try {
      await closeMutation.mutateAsync({ agreementId, data: body });
      toast({ tone: 'success', title: t('closeSuccess') });
      onSuccess();
      onOpenChange(false);
    } catch (err) {
      const { kind, message } = classifyError(err);
      if (kind === 'validation') {
        applyFieldErrors(err, setError);
      } else {
        toast({ tone: 'error', title: t('closeError'), description: message });
      }
    }
  }

  return (
    <Drawer open={open} onOpenChange={onOpenChange} width={480}>
      <DrawerHeader title={t('closeTitle')} />
      <DrawerBody>
        <p className="mb-[16px] text-[13px] leading-[1.5] text-text-2">{t('closeDescription')}</p>
        <form id="close-form" onSubmit={handleSubmit(onSubmit)} noValidate>
          <FormSection>
            <FormField
              label={t('fieldCloseReason')}
              htmlFor="close_reason"
              required
              error={errors.reason?.message}
            >
              <FilterSelect
                id="close_reason"
                aria-label={t('fieldCloseReason')}
                {...register('reason')}
              >
                <option value={CloseAgreementBodyReason.END_OF_TERM}>{t('reasonEndOfTerm')}</option>
                <option value={CloseAgreementBodyReason.RESIGNED}>{t('reasonResigned')}</option>
                <option value={CloseAgreementBodyReason.TERMINATED}>{t('reasonTerminated')}</option>
                <option value={CloseAgreementBodyReason.OTHER}>{t('reasonOther')}</option>
              </FilterSelect>
            </FormField>

            <FormField
              label={t('fieldEffectiveDate')}
              htmlFor="close_effective_date"
              required
              error={errors.effective_date?.message}
            >
              <Input
                id="close_effective_date"
                type="date"
                {...register('effective_date')}
                aria-invalid={!!errors.effective_date}
              />
            </FormField>

            <FormField label={t('fieldNote')} htmlFor="close_note" error={errors.note?.message}>
              <Input
                id="close_note"
                placeholder={t('fieldNotePlaceholder')}
                {...register('note')}
              />
            </FormField>
          </FormSection>
        </form>
      </DrawerBody>
      <DrawerFooter>
        <Button
          type="button"
          variant="secondary"
          onClick={() => onOpenChange(false)}
          disabled={isSubmitting}
        >
          {t('common.cancel', { ns: 'translation' })}
        </Button>
        <Button type="submit" form="close-form" variant="destructive" disabled={isSubmitting}>
          {isSubmitting ? t('common.loading', { ns: 'translation' }) : t('closeConfirm')}
        </Button>
      </DrawerFooter>
    </Drawer>
  );
}

// ---------------------------------------------------------------------------
// AgreementDetailScreen
// ---------------------------------------------------------------------------

export function AgreementDetailScreen({ agreementId }: AgreementDetailScreenProps) {
  const { t } = useTranslation('agreements');
  const navigate = useNavigate();
  const { toast } = useToast();
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const query = useGetAgreement(agreementId);
  const uploadMutation = useUploadAgreementAttachment();

  const [renewOpen, setRenewOpen] = useState(false);
  const [closeOpen, setCloseOpen] = useState(false);
  const [uploadedFiles, setUploadedFiles] = useState<Array<{ id: string; name: string }>>([]);

  const agreement = query.data?.data as import('@swp/api-client/e2').Agreement | undefined;

  // ---------------------------------------------------------------------------
  // Upload handler (attachment stub — EA-4)
  // ---------------------------------------------------------------------------

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const result = await uploadMutation.mutateAsync({
        agreementId,
        data: { file, category: 'signed_agreement' },
      });
      // result.data is FileRef on success
      const fileRef = result.data as import('@swp/api-client/e2').FileRef;
      if (fileRef?.id && fileRef?.name) {
        setUploadedFiles((prev) => [...prev, { id: fileRef.id, name: fileRef.name }]);
      }
      toast({ tone: 'success', title: t('uploadSuccess') });
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t('uploadError'), description: message });
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  }

  // ---------------------------------------------------------------------------
  // Loading / error states
  // ---------------------------------------------------------------------------

  if (query.isLoading) {
    return (
      <div className="flex flex-col gap-[16px]">
        <div className="h-[40px] w-[200px] animate-pulse rounded-lg bg-surface-2" />
        <div className="h-[80px] w-full animate-pulse rounded-xl bg-surface-2" />
      </div>
    );
  }

  if (query.isError || !agreement) {
    const { kind } = classifyError(query.error);
    return kind === 'not-found' ? (
      <EmptyState
        variant="no-permission"
        title={t('notFoundTitle')}
        description={t('notFoundBody')}
      />
    ) : (
      <StateView
        kind="error"
        title={t('errorTitle')}
        description={t('errorBody')}
        onRetry={() => query.refetch()}
        retryLabel={t('common.retry', { ns: 'translation' })}
      />
    );
  }

  const statusToneMap = {
    [AgreementStatus.ACTIVE]: 'ok' as const,
    [AgreementStatus.EXPIRING]: 'warn' as const,
    [AgreementStatus.SUPERSEDED]: 'neutral' as const,
    [AgreementStatus.CLOSED]: 'bad' as const,
  };

  const isActionable =
    agreement.status === AgreementStatus.ACTIVE || agreement.status === AgreementStatus.EXPIRING;

  const agreementLabel = agreement.agreement_no ?? agreement.id;

  return (
    <>
      <div className="flex flex-col gap-[16px]">
        {/* Back row — Cu0qg DZ1aY */}
        <Link
          to="/agreements"
          className="flex w-fit items-center gap-[7px] text-[13px] font-medium text-text-2 hover:text-text"
        >
          <ArrowLeft className="size-4" aria-hidden />
          {t('backToList')}
        </Link>

        {/* Header card — Cu0qg jlYCh */}
        <div className="flex items-center justify-between rounded-xl border border-border bg-surface p-5">
          {/* Left: G8Poco */}
          <div className="flex items-center gap-[16px]">
            <div className="flex size-[40px] shrink-0 items-center justify-center rounded-full bg-surface-2 text-[14px] font-semibold text-text-3">
              {agreement.employee_id?.slice(-2).toUpperCase() ?? '??'}
            </div>
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-[10px]">
                <span className="font-mono text-[20px] font-bold text-text">{agreementLabel}</span>
                <span
                  className={[
                    'inline-flex items-center rounded-[6px] border px-[8px] py-[3px] font-mono text-[10px] font-bold tracking-[0.5px]',
                    agreement.type === AgreementType.PKWT
                      ? 'border-info-bd bg-info-bg text-info-tx'
                      : 'border-ok-bd bg-ok-bg text-ok-tx',
                  ].join(' ')}
                >
                  {agreement.type}
                </span>
                <StatusBadge dot tone={statusToneMap[agreement.status]}>
                  {t(`status.${agreement.status}`)}
                </StatusBadge>
              </div>
              <span className="text-[13px] text-text-2">
                {t('employeeId')}: {agreement.employee_id}
              </span>
            </div>
          </div>
          {/* Right: W7OVz — only if ACTIVE/EXPIRING */}
          {isActionable && (
            <div className="flex items-center gap-[10px]">
              <Button type="button" variant="secondary" onClick={() => setRenewOpen(true)}>
                <RefreshCw className="size-4" aria-hidden />
                {t('actionRenew')}
              </Button>
              <Button type="button" variant="destructive" onClick={() => setCloseOpen(true)}>
                <XCircle className="size-4" aria-hidden />
                {t('actionClose')}
              </Button>
            </div>
          )}
        </div>

        {/* Two-column layout — Cu0qg lEb2b */}
        <div className="flex items-start gap-[16px]">
          {/* Left column — Y0sYF8 */}
          <div className="flex min-w-0 flex-1 flex-col gap-[16px]">
            {/* Card Detail Perjanjian — L7UU5 */}
            <DetailCard title={t('cardDetail')}>
              <KvRow label={t('fieldNomor')} value={agreement.agreement_no} mono />
              <KvRow
                label={t('fieldType')}
                value={
                  agreement.type === AgreementType.PKWT
                    ? `PKWT (${t('pkwtLabel')})`
                    : `PKWTT (${t('pkwttLabel')})`
                }
              />
              <KvRow label={t('fieldStartDate')}>
                <DateText
                  kind="date"
                  value={agreement.start_date}
                  className="font-mono text-[13px] font-medium text-text"
                />
              </KvRow>
              {agreement.end_date && (
                <KvRow label={t('fieldEndDate')}>
                  <DateText
                    kind="date"
                    value={agreement.end_date}
                    className="font-mono text-[13px] font-medium text-text"
                  />
                </KvRow>
              )}
              <KvRow label={t('fieldStatus')}>
                <StatusBadge dot tone={statusToneMap[agreement.status]}>
                  {t(`status.${agreement.status}`)}
                </StatusBadge>
              </KvRow>
              {agreement.closed_reason && (
                <KvRow
                  label={t('fieldCloseReason')}
                  value={t(`reason.${agreement.closed_reason}`)}
                />
              )}
              {agreement.closed_at && (
                <KvRow label={t('fieldClosedAt')}>
                  <DateText
                    kind="instant"
                    value={agreement.closed_at}
                    className="font-mono text-[12px] text-text"
                  />
                </KvRow>
              )}
            </DetailCard>

            {/* Card Riwayat Kompensasi — FOebj (EA-4 masked) */}
            <DetailCard
              title={t('cardCompensation')}
              titleIcon={<Lock className="size-[14px] text-warn-tx" aria-hidden />}
            >
              <div className="mx-5 flex items-start gap-[8px] border-t border-border-soft py-[10px]">
                <Lock className="mt-[1px] size-[14px] shrink-0 text-warn-tx" aria-hidden />
                <p className="text-[11px] leading-[1.5] text-warn-tx">
                  {t('compensationMaskedNote')}
                </p>
              </div>
              {agreement.compensation ? (
                <>
                  <KvRow
                    label={t('fieldBaseSalary')}
                    value={
                      typeof agreement.compensation.base_salary_idr === 'number'
                        ? new Intl.NumberFormat('id-ID', {
                            style: 'currency',
                            currency: 'IDR',
                            maximumFractionDigits: 0,
                          }).format(agreement.compensation.base_salary_idr)
                        : '•••••'
                    }
                    mono
                  />
                  {agreement.compensation.effective_date && (
                    <KvRow label={t('fieldCompEffectiveDate')}>
                      <DateText
                        kind="date"
                        value={agreement.compensation.effective_date}
                        className="font-mono text-[13px] text-text"
                      />
                    </KvRow>
                  )}
                </>
              ) : (
                <div className="px-5 pb-[12px]">
                  <p className="text-[12px] text-text-3">{t('compensationHidden')}</p>
                </div>
              )}
            </DetailCard>
          </div>

          {/* Right column — WA5cZ */}
          <div className="flex w-[380px] shrink-0 flex-col gap-[16px]">
            {/* Card Berkas Perjanjian — a7IVS */}
            <DetailCard title={t('cardFile')}>
              {uploadedFiles.length > 0 ? (
                <div className="mx-5 my-[14px] flex flex-col gap-[8px]">
                  {uploadedFiles.map((f) => (
                    <div
                      key={f.id}
                      className="flex items-center gap-[8px] rounded-lg border border-border-soft bg-surface-2 px-[12px] py-[10px]"
                    >
                      <Paperclip className="size-[14px] shrink-0 text-text-2" aria-hidden />
                      <span className="text-[13px] font-medium text-text" data-testid="attachment-name">
                        {f.name}
                      </span>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="mx-5 my-[14px] flex min-h-[160px] flex-col items-center justify-center gap-[10px] rounded-lg border border-border-soft bg-surface-2 px-5 py-[24px]">
                  <Paperclip className="size-[32px] text-text-3" aria-hidden />
                  <p className="text-center text-[12px] text-text-3">{t('noAttachment')}</p>
                </div>
              )}
              <div className="flex items-center gap-[8px] border-t border-border-soft px-[14px] py-[14px]">
                <Button
                  type="button"
                  variant="secondary"
                  className="flex-1"
                  onClick={() => fileInputRef.current?.click()}
                  disabled={uploadMutation.isPending}
                >
                  <Paperclip className="size-4" aria-hidden />
                  {t('actionUpload')}
                </Button>
                <Button type="button" variant="secondary" className="flex-1" disabled>
                  <Eye className="size-4" aria-hidden />
                  {t('actionPreview')}
                </Button>
                <Button type="button" variant="secondary" className="flex-1" disabled>
                  <Download className="size-4" aria-hidden />
                  {t('actionDownload')}
                </Button>
              </div>
              {/* Hidden file input — EA-4 upload stub */}
              <input
                ref={fileInputRef}
                type="file"
                accept="application/pdf,image/jpeg,image/png"
                className="sr-only"
                aria-label={t('actionUpload')}
                data-testid="agreement-attachment-input"
                onChange={handleFileChange}
              />
            </DetailCard>

            {/* Card Rantai Perjanjian — xkQQH */}
            <DetailCard
              title={t('cardChain')}
              titleIcon={<GitBranch className="size-[14px] text-text-2" aria-hidden />}
            >
              <div className="flex flex-col gap-[8px] px-5 pb-5">
                {agreement.predecessor_id && (
                  <Link
                    to="/agreements/$agreementId"
                    params={{ agreementId: String(agreement.predecessor_id) }}
                    className="flex items-center justify-between gap-[10px] rounded-lg border border-border-soft bg-surface-2 px-[12px] py-[10px] text-[12px] text-text hover:bg-surface"
                  >
                    <span className="font-medium">{t('predecessor')}</span>
                    <span className="font-mono text-text-2">{agreement.predecessor_id}</span>
                  </Link>
                )}
                <div className="flex items-center justify-between gap-[10px] rounded-lg border border-primary bg-primary-soft px-[12px] py-[10px] text-[12px]">
                  <span className="font-medium text-text">{t('currentAgreement')}</span>
                  <span className="font-mono text-text">{agreementLabel}</span>
                </div>
                {agreement.successor_id && (
                  <Link
                    to="/agreements/$agreementId"
                    params={{ agreementId: String(agreement.successor_id) }}
                    className="flex items-center justify-between gap-[10px] rounded-lg border border-border-soft bg-surface-2 px-[12px] py-[10px] text-[12px] text-text hover:bg-surface"
                  >
                    <span className="font-medium">{t('successor')}</span>
                    <span className="font-mono text-text-2">{agreement.successor_id}</span>
                  </Link>
                )}
                <p className="text-[11px] leading-[1.5] text-text-3">{t('chainHint')}</p>
              </div>
            </DetailCard>
          </div>
        </div>
      </div>

      {/* Renew drawer */}
      <RenewDrawer
        agreementId={agreementId}
        open={renewOpen}
        onOpenChange={setRenewOpen}
        onSuccess={() => query.refetch()}
      />

      {/* Close drawer */}
      <CloseDrawer
        agreementId={agreementId}
        open={closeOpen}
        onOpenChange={setCloseOpen}
        onSuccess={() => {
          void query.refetch();
          void navigate({ to: '/agreements' as const });
        }}
      />
    </>
  );
}
