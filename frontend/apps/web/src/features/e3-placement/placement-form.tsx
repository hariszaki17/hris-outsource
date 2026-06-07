/**
 * E3 · Penempatan — Buat Penempatan
 *
 * Design source: .pen frame `g3OzZz` "E3 · Buat Penempatan"
 *
 * Layout: Sidebar + Main column (vertical)
 *   Title band (title + status-preview pill)
 *   Card: Agen & Perjanjian Kerja
 *     Row: employee_id | agreement_id (filtered to chosen employee)
 *     AgentNote (ok-bg when no active placement; bad-bg on INV-1)
 *   Card: Penempatan
 *     Row: client_company_id | service_line_id
 *     Row: position_id (half-width, depends on service_line_id)
 *   Card: Periode & Ketentuan
 *     Row: start_date | end_date
 *     CapNote (info-bg)
 *     Row: notes (full-width)
 *     Row: backdate_reason (conditional — start_date < today)
 *   Footer: hint | Batal | Simpan Penempatan
 *
 * State variants:
 *   default · validation errors · saving · INV-1 conflict (Banner)
 *   outside-contract warning (warn Banner) · success toast → navigate
 *
 * INV-1 path: ApiError.isInvariantViolation + code.startsWith('INV_1') →
 *   parse `error.details` as INVViolationDetails → render conflict Banner
 *   showing current_placement info + suggestedActions CTAs.
 *
 * i18n: namespace `placementForm`
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import { ApiError } from '@swp/api-client';
import type {
  INVViolationDetails,
  INVViolationDetailsSuggestedActionsItem,
  PlacementCreateResponse,
} from '@swp/api-client/e3';
import {
  INVViolationDetailsSuggestedActionsItem as SuggestedAction,
  useCreatePlacement,
} from '@swp/api-client/e3';
import { Banner, Button, FormField, FormSection, Input, useToast } from '@swp/ui';
import { Link, useNavigate } from '@tanstack/react-router';
import { ArrowLeft, CalendarClock, Check, CircleCheck, Info, X } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';
import { ClientCompanyPicker } from '../e2-identity/pickers/client-company-picker.tsx';
import { EmployeePicker } from '../e2-identity/pickers/employee-picker.tsx';
import { PositionPicker } from '../e2-identity/pickers/position-picker.tsx';
import { ServiceLinePicker } from '../e2-identity/pickers/service-line-picker.tsx';
import { SitePicker } from '../e2-identity/pickers/site-picker.tsx';
import { AgreementPicker } from './agreement-picker.tsx';

// ---------------------------------------------------------------------------
// Zod schema (hand-written — F3.1 BR-1b, BR-4, BR-5, BR-6)
// ---------------------------------------------------------------------------

const placementSchema = z
  .object({
    employee_id: z.string().min(1, 'Agen wajib dipilih'),
    agreement_id: z.string().min(1, 'Perjanjian kerja wajib dipilih'),
    client_company_id: z.string().min(1, 'Perusahaan klien wajib dipilih'),
    site_id: z.string().min(1, 'Site wajib dipilih'),
    service_line_id: z.string().min(1, 'Lini layanan wajib dipilih'),
    position_id: z.string().min(1, 'Posisi wajib dipilih'),
    start_date: z.string().min(1, 'Tanggal mulai wajib diisi'),
    end_date: z.string().nullable().optional(),
    backdate_reason: z.string().max(1000).nullable().optional(),
    notes: z.string().max(2000).nullable().optional(),
  })
  .superRefine((data, ctx) => {
    if (data.end_date && data.start_date && data.end_date <= data.start_date) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Tanggal berakhir harus setelah tanggal mulai (BR-4)',
        path: ['end_date'],
      });
    }
    const today = new Date().toISOString().slice(0, 10);
    if (data.start_date && data.start_date < today && !data.backdate_reason) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Alasan backdating wajib diisi (BR-6)',
        path: ['backdate_reason'],
      });
    }
  });

type PlacementFormValues = z.infer<typeof placementSchema>;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function isBackdate(startDate: string | undefined): boolean {
  if (!startDate) return false;
  const today = new Date().toISOString().slice(0, 10);
  return startDate < today;
}

function isFuture(startDate: string | undefined): boolean {
  if (!startDate) return false;
  const today = new Date().toISOString().slice(0, 10);
  return startDate > today;
}

/** Extract INVViolationDetails from an ApiError's raw error details payload. */
function extractINVDetails(error: unknown): INVViolationDetails | null {
  if (!(error instanceof ApiError)) return null;
  if (!error.isInvariantViolation) return null;
  // The error envelope's `details` is stored on the raw JSON — we cast carefully.
  const raw = error as ApiError & { details?: INVViolationDetails };
  return raw.details ?? null;
}

function suggestedActionLabel(
  action: INVViolationDetailsSuggestedActionsItem,
  t: (k: string) => string,
): string {
  switch (action) {
    case SuggestedAction.end:
      return t('inv1.actionEnd');
    case SuggestedAction.transfer:
      return t('inv1.actionTransfer');
    case SuggestedAction.replace:
      return t('inv1.actionReplace');
    case SuggestedAction.end_existing_first:
      return t('inv1.actionEndFirst');
    case SuggestedAction.assign_after_placement:
      return t('inv1.actionAssignAfter');
    default:
      return action;
  }
}

// ---------------------------------------------------------------------------
// Section card (matches g3OzZz Ipnoy / ZDjKt / q4vBQ)
// ---------------------------------------------------------------------------

function SectionCard({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-xl border border-border bg-surface">
      <div className="rounded-t-xl border-b border-border-soft px-5 py-[14px]">
        <p className="text-[14px] font-bold text-text">{title}</p>
      </div>
      <div className="flex flex-col gap-4 p-5">{children}</div>
    </div>
  );
}

// Two-column field row (matches g3OzZz XUXTQ / w0p9lP etc.)
function FieldRow({ children }: { children: React.ReactNode }) {
  return <div className="flex gap-[14px]">{children}</div>;
}

// ---------------------------------------------------------------------------
// CreatePlacementScreen
// ---------------------------------------------------------------------------

export function CreatePlacementScreen() {
  const { t } = useTranslation('placementForm');
  const navigate = useNavigate();
  const { toast } = useToast();

  // INV-1 conflict state — set on a 409 INV_1_VIOLATION response.
  const [invConflict, setInvConflict] = useState<INVViolationDetails | null>(null);
  // Outside-contract warning — set when the server returns a 422 PLACEMENT_OUTSIDE_CONTRACT rule.
  const [outsideContractWarn, setOutsideContractWarn] = useState(false);

  const createMutation = useCreatePlacement();

  const form = useForm<PlacementFormValues>({
    resolver: zodResolver(placementSchema),
    defaultValues: {
      employee_id: '',
      agreement_id: '',
      client_company_id: '',
      site_id: '',
      service_line_id: '',
      position_id: '',
      start_date: '',
      end_date: null,
      backdate_reason: null,
      notes: null,
    },
  });

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    setError,
    formState: { errors, isSubmitting },
  } = form;

  const watchedEmployeeId = watch('employee_id');
  const watchedAgreementId = watch('agreement_id');
  const watchedServiceLineId = watch('service_line_id');
  const watchedPositionId = watch('position_id');
  const watchedStartDate = watch('start_date');

  const showBackdateReason = isBackdate(watchedStartDate);
  const previewStatus = isFuture(watchedStartDate) ? t('statusScheduled') : t('statusActive');

  // ---------------------------------------------------------------------------
  // Submit
  // ---------------------------------------------------------------------------

  async function onSubmit(values: PlacementFormValues) {
    // Clear previous conflict / warning state on each attempt.
    setInvConflict(null);
    setOutsideContractWarn(false);

    try {
      const res = await createMutation.mutateAsync({
        data: {
          employee_id: values.employee_id,
          agreement_id: values.agreement_id,
          client_company_id: values.client_company_id,
          site_id: values.site_id,
          service_line_id: values.service_line_id,
          position_id: values.position_id,
          start_date: values.start_date,
          end_date: values.end_date ?? null,
          backdate_reason: values.backdate_reason ?? null,
          notes: values.notes ?? null,
        },
      });

      const created = res.data as PlacementCreateResponse | undefined;
      toast({ tone: 'success', title: t('successToast') });

      const placementId = created?.id;

      if (placementId) {
        void navigate({ to: '/placements/$placementId' as const, params: { placementId } });
      } else {
        void navigate({ to: '/placements' as const });
      }
    } catch (err) {
      const { kind } = classifyError(err);

      if (kind === 'invariant' && err instanceof ApiError && err.code.startsWith('INV_1')) {
        // INV-1: agent already has an active placement.
        const details = extractINVDetails(err);
        setInvConflict(details);
        return;
      }

      if (kind === 'rule' && err instanceof ApiError && err.code === 'PLACEMENT_OUTSIDE_CONTRACT') {
        setOutsideContractWarn(true);
        return;
      }

      if (kind === 'validation') {
        applyFieldErrors(err, setError);
        return;
      }

      toast({
        tone: 'error',
        title: t('errorToast'),
        description: err instanceof Error ? err.message : undefined,
      });
    }
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[16px]">
      {/* Back row */}
      <Link
        to="/placements"
        className="flex w-fit items-center gap-[7px] text-[13px] font-medium text-text-2 hover:text-text"
      >
        <ArrowLeft className="size-4" aria-hidden />
        {t('backToList')}
      </Link>

      {/* Title band — g3OzZz rNmMX TitleBand */}
      <div className="flex w-full items-center justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-[30px] font-bold text-text">{t('title')}</h1>
          <p className="text-[14px] text-text-3">{t('subtitle')}</p>
        </div>
        {watchedStartDate && (
          <div className="flex items-center gap-[7px] rounded-full border border-info-bd bg-info-bg px-[14px] py-[7px]">
            <CalendarClock className="size-[15px] text-info-tx" aria-hidden />
            <span className="text-[13px] font-semibold text-info-tx">
              {t('previewStatus', { status: previewStatus, date: watchedStartDate })}
            </span>
          </div>
        )}
      </div>

      {/* INV-1 conflict Banner — shown when 409 INV_1_VIOLATION */}
      {invConflict && (
        <Banner
          tone="bad"
          title={t('inv1.title')}
          description={
            [
              invConflict.current_placement
                ? t('inv1.existingAt', {
                    company:
                      invConflict.current_placement.client_company_name ??
                      invConflict.current_placement.client_company_id,
                    status: invConflict.current_placement.lifecycle_status,
                  })
                : null,
              invConflict.suggested_actions?.length
                ? `${t('inv1.suggestedLabel')} ${invConflict.suggested_actions
                    .map((a) => suggestedActionLabel(a, t))
                    .join(', ')}`
                : null,
            ]
              .filter(Boolean)
              .join(' — ') || t('inv1.description')
          }
        />
      )}

      {/* Outside-contract warning Banner */}
      {outsideContractWarn && (
        <Banner
          tone="warn"
          title={t('outsideContractTitle')}
          description={t('outsideContractDescription')}
        />
      )}

      {/* Form */}
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void handleSubmit(onSubmit)(e);
        }}
        noValidate
      >
        <div className="flex flex-col gap-[16px]">
          {/* Card: Agen & Perjanjian Kerja — g3OzZz Ipnoy */}
          <SectionCard title={t('sectionAgent')}>
            <FormSection>
              {/* Row: employee + agreement — g3OzZz XUXTQ */}
              <FieldRow>
                <div className="flex-1 min-w-0">
                  <FormField
                    label={t('fieldEmployee')}
                    htmlFor="employee_id"
                    required
                    error={errors.employee_id?.message}
                  >
                    <EmployeePicker
                      value={watchedEmployeeId || null}
                      onChange={(val) => {
                        setValue('employee_id', val ?? '', { shouldValidate: true });
                        // Reset agreement when employee changes.
                        setValue('agreement_id', '', { shouldValidate: false });
                      }}
                      error={!!errors.employee_id}
                      placeholder={t('fieldEmployeePlaceholder')}
                    />
                  </FormField>
                </div>
                <div className="flex-1 min-w-0">
                  <FormField
                    label={t('fieldAgreement')}
                    htmlFor="agreement_id"
                    required
                    error={errors.agreement_id?.message}
                  >
                    <AgreementPicker
                      value={watchedAgreementId || null}
                      onChange={(val) =>
                        setValue('agreement_id', val ?? '', { shouldValidate: true })
                      }
                      employeeId={watchedEmployeeId || null}
                      disabled={!watchedEmployeeId}
                      error={!!errors.agreement_id}
                    />
                  </FormField>
                </div>
              </FieldRow>

              {/* Agent validity note — g3OzZz ZU0v2 */}
              {watchedEmployeeId && !invConflict && (
                <div className="flex items-center gap-2 rounded-lg border border-ok-bd bg-ok-bg px-3 py-[9px]">
                  <CircleCheck className="size-[15px] shrink-0 text-ok-tx" aria-hidden />
                  <p className="text-[12px] font-medium text-ok-tx">{t('agentValidNote')}</p>
                </div>
              )}
              {watchedEmployeeId && invConflict && (
                <div className="flex items-center gap-2 rounded-lg border border-bad-bd bg-bad-bg px-3 py-[9px]">
                  <X className="size-[15px] shrink-0 text-bad-tx" aria-hidden />
                  <p className="text-[12px] font-medium text-bad-tx">{t('agentConflictNote')}</p>
                </div>
              )}
            </FormSection>
          </SectionCard>

          {/* Card: Penempatan — g3OzZz ZDjKt */}
          <SectionCard title={t('sectionPlacement')}>
            <FormSection>
              {/* Row: client_company + service_line — g3OzZz w0p9lP */}
              <FieldRow>
                <div className="flex-1 min-w-0">
                  <FormField
                    label={t('fieldClientCompany')}
                    htmlFor="client_company_id"
                    required
                    error={errors.client_company_id?.message}
                  >
                    <ClientCompanyPicker
                      value={watch('client_company_id') || null}
                      onChange={(val) => {
                        setValue('client_company_id', val ?? '', { shouldValidate: true });
                        // Reset site when the company changes (sites belong to a company, BR-3b).
                        setValue('site_id', '', { shouldValidate: false });
                      }}
                      error={!!errors.client_company_id}
                      placeholder={t('fieldClientCompanyPlaceholder')}
                    />
                  </FormField>
                </div>
                <div className="flex-1 min-w-0">
                  <FormField
                    label={t('fieldSite')}
                    htmlFor="site_id"
                    required
                    error={errors.site_id?.message}
                  >
                    <SitePicker
                      clientCompanyId={watch('client_company_id') || null}
                      value={watch('site_id') || null}
                      onChange={(val) => setValue('site_id', val ?? '', { shouldValidate: true })}
                      error={!!errors.site_id}
                    />
                  </FormField>
                </div>
              </FieldRow>

              {/* Row: service_line */}
              <FieldRow>
                <div className="flex-1 min-w-0" style={{ maxWidth: '50%' }}>
                  <FormField
                    label={t('fieldServiceLine')}
                    htmlFor="service_line_id"
                    required
                    error={errors.service_line_id?.message}
                  >
                    <ServiceLinePicker
                      value={watchedServiceLineId || null}
                      onChange={(val) => {
                        setValue('service_line_id', val ?? '', { shouldValidate: true });
                        // Reset position when service line changes.
                        setValue('position_id', '', { shouldValidate: false });
                      }}
                      error={!!errors.service_line_id}
                      placeholder={t('fieldServiceLinePlaceholder')}
                    />
                  </FormField>
                </div>
              </FieldRow>

              {/* Row: position (half-width per design — g3OzZz TcBbR Bhezi width:549) */}
              <FieldRow>
                <div className="flex-1 min-w-0" style={{ maxWidth: '50%' }}>
                  <FormField
                    label={t('fieldPosition')}
                    htmlFor="position_id"
                    required
                    error={errors.position_id?.message}
                  >
                    <PositionPicker
                      value={watchedPositionId || null}
                      onChange={(val) =>
                        setValue('position_id', val ?? '', { shouldValidate: true })
                      }
                      serviceLineId={watchedServiceLineId || null}
                      error={!!errors.position_id}
                      placeholder={t('fieldPositionPlaceholder')}
                    />
                  </FormField>
                </div>
              </FieldRow>
            </FormSection>
          </SectionCard>

          {/* Card: Periode & Ketentuan — g3OzZz q4vBQ */}
          <SectionCard title={t('sectionPeriod')}>
            <FormSection>
              {/* Row: start_date + end_date — g3OzZz LVdWl */}
              <FieldRow>
                <div className="flex-1 min-w-0">
                  <FormField
                    label={t('fieldStartDate')}
                    htmlFor="start_date"
                    required
                    error={errors.start_date?.message}
                  >
                    <Input
                      id="start_date"
                      type="date"
                      {...register('start_date')}
                      aria-invalid={!!errors.start_date}
                    />
                  </FormField>
                </div>
                <div className="flex-1 min-w-0">
                  <FormField
                    label={t('fieldEndDate')}
                    htmlFor="end_date"
                    error={errors.end_date?.message}
                  >
                    <Input
                      id="end_date"
                      type="date"
                      {...register('end_date')}
                      aria-invalid={!!errors.end_date}
                    />
                  </FormField>
                </div>
              </FieldRow>

              {/* CapNote (info) — g3OzZz KkMG9 */}
              <div className="flex items-start gap-2 rounded-lg border border-info-bd bg-info-bg px-[14px] py-[9px]">
                <Info className="mt-[1px] size-[15px] shrink-0 text-info-tx" aria-hidden />
                <p className="text-[12px] leading-[1.4] text-info-tx">{t('capNote')}</p>
              </div>

              {/* Leave-quota info — annual leave is an employment-agreement term; the quota
                  is auto-prorated from the agent's onboarding date and adjustable later in Cuti. */}
              <div className="flex items-start gap-2 rounded-lg border border-info-bd bg-info-bg px-[14px] py-[9px]">
                <Info className="mt-[1px] size-[15px] shrink-0 text-info-tx" aria-hidden />
                <p className="text-[12px] leading-[1.4] text-info-tx">
                  {t('leaveQuotaNote')}{' '}
                  <Link
                    to="/leave/quotas"
                    className="font-semibold underline underline-offset-2 hover:no-underline"
                  >
                    {t('leaveQuotaNoteLink')}
                  </Link>
                </p>
              </div>

              {/* Notes — g3OzZz XFXPx H8XsW */}
              <FormField label={t('fieldNotes')} htmlFor="notes" error={errors.notes?.message}>
                <Input
                  id="notes"
                  placeholder={t('fieldNotesPlaceholder')}
                  {...register('notes')}
                  aria-invalid={!!errors.notes}
                />
              </FormField>

              {/* Backdate reason — conditional BR-6 */}
              {showBackdateReason && (
                <FormField
                  label={t('fieldBackdateReason')}
                  htmlFor="backdate_reason"
                  required
                  hint={t('fieldBackdateReasonHint')}
                  error={errors.backdate_reason?.message}
                >
                  <Input
                    id="backdate_reason"
                    placeholder={t('fieldBackdateReasonPlaceholder')}
                    {...register('backdate_reason')}
                    aria-invalid={!!errors.backdate_reason}
                  />
                </FormField>
              )}
            </FormSection>
          </SectionCard>

          {/* Footer — g3OzZz KfwmS */}
          <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-6 py-[16px]">
            <p className="text-[13px] text-text-3">{t('footerHint')}</p>
            <div className="flex items-center gap-[10px]">
              <Button
                type="button"
                variant="secondary"
                onClick={() => void navigate({ to: '/placements' as const })}
                disabled={isSubmitting}
              >
                <X className="mr-[6px] size-4" aria-hidden />
                {t('actionCancel')}
              </Button>
              <Button type="submit" disabled={isSubmitting}>
                <Check className="mr-[6px] size-4" aria-hidden />
                {isSubmitting ? t('actionSaving') : t('actionSave')}
              </Button>
            </div>
          </div>
        </div>
      </form>
    </div>
  );
}
