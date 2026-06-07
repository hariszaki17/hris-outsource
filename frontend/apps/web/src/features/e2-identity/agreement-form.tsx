/**
 * E2 · Perjanjian Kerja — Buat (Create Agreement)
 *
 * .pen frame: gxqjg  "E2 · Perjanjian Kerja — Buat (PKWT)"
 *
 * Design: BackRow → HeaderCard (title + subtitle) →
 * Two-column layout:
 *   L: S Tipe Perjanjian | S Detail PKWT (conditional on type=PKWT) |
 *      S Kompensasi & Pajak
 *   R: Summary card | Warn card (existing active agreement)
 * Footer: hint | Batal btn | Aktifkan Perjanjian btn
 *
 * MVP: agreement is always created ACTIVE (backend default). No draft
 * lifecycle and no attachment storage, so the upload section and the
 * "save as draft" path are omitted.
 *
 * RHF + hand-written Zod schema (E2 Zod codegen deferred).
 * Conditional: end_date required when type=PKWT (EA-1).
 * F2.2 EA-1 (create), EA-4 (compensation encryption notice).
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import { AgreementType, type AgreementWriteRequest, useCreateAgreement } from '@swp/api-client/e2';
import { Button, FilterSelect, FormField, FormSection, Input, useToast } from '@swp/ui';
import { Link, useNavigate } from '@tanstack/react-router';
import { ArrowLeft, Check, Info, Lock, TriangleAlert } from 'lucide-react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';
import { EmployeePicker } from './pickers/employee-picker.tsx';

// ---------------------------------------------------------------------------
// Zod schema (hand-written — EA-1/EA-2)
// ---------------------------------------------------------------------------

const agreementSchema = z
  .object({
    employee_id: z.string().min(1, 'Karyawan wajib dipilih'),
    type: z.nativeEnum(AgreementType),
    agreement_no: z.string().optional(),
    start_date: z.string().min(1, 'Tanggal mulai wajib diisi'),
    end_date: z.string().optional(),
    base_salary_idr: z.union([z.number().positive(), z.nan(), z.literal(0)]).optional(),
    effective_date: z.string().optional(),
  })
  .superRefine((data, ctx) => {
    if (data.type === AgreementType.PKWT && !data.end_date) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Tanggal akhir wajib untuk PKWT (EA-1)',
        path: ['end_date'],
      });
    }
  });

type AgreementFormValues = z.infer<typeof agreementSchema>;

// ---------------------------------------------------------------------------
// Section card (matches gxqjg S* sections)
// ---------------------------------------------------------------------------

function SectionCard({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-border bg-surface">
      <div className="border-b border-border-soft px-5 py-[18px] pb-[12px]">
        <p className="text-[15px] font-semibold text-text">{title}</p>
        {subtitle && <p className="mt-[2px] text-[12px] text-text-2">{subtitle}</p>}
      </div>
      <div className="p-5">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Summary KV row (right column preview card)
// ---------------------------------------------------------------------------

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span className="text-[12px] text-text-2">{label}</span>
      <span className="text-[13px] font-medium text-text">{value || '—'}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// CreateAgreementScreen
// ---------------------------------------------------------------------------

export function CreateAgreementScreen() {
  const { t } = useTranslation('agreements');
  const navigate = useNavigate();
  const { toast } = useToast();

  const createMutation = useCreateAgreement();

  const form = useForm<AgreementFormValues>({
    resolver: zodResolver(agreementSchema),
    defaultValues: { type: AgreementType.PKWT },
  });

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    setError,
    formState: { errors, isSubmitting },
  } = form;

  const watchedType = watch('type');
  const watchedEmployee = watch('employee_id');
  const watchedAgreementNo = watch('agreement_no');
  const watchedStartDate = watch('start_date');
  const watchedEndDate = watch('end_date');

  const isPKWT = watchedType === AgreementType.PKWT;

  // ---------------------------------------------------------------------------
  // Submit handler — always creates the agreement ACTIVE (backend default)
  // ---------------------------------------------------------------------------

  async function submitCreate() {
    await handleSubmit(async (values) => {
      const body: AgreementWriteRequest = {
        employee_id: values.employee_id,
        type: values.type,
        agreement_no: values.agreement_no || undefined,
        start_date: values.start_date,
        end_date: values.type === AgreementType.PKWT ? values.end_date : undefined,
        compensation:
          values.base_salary_idr !== undefined && values.base_salary_idr > 0
            ? {
                base_salary_idr: values.base_salary_idr,
                effective_date: values.effective_date || values.start_date || undefined,
              }
            : undefined,
      };
      try {
        const res = await createMutation.mutateAsync({ data: body });
        const created = res.data as import('@swp/api-client/e2').Agreement | undefined;
        toast({
          tone: 'success',
          title: t('createActiveSuccess'),
        });
        if (created?.id) {
          void navigate({
            to: '/agreements/$agreementId' as const,
            params: { agreementId: created.id },
          });
        } else {
          void navigate({ to: '/agreements' as const });
        }
      } catch (err) {
        const { kind, message } = classifyError(err);
        if (kind === 'validation') {
          applyFieldErrors(err, setError);
        } else {
          toast({ tone: 'error', title: t('createError'), description: message });
        }
      }
    })();
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[16px]">
      {/* Back row — gxqjg bEylw */}
      <Link
        to="/agreements"
        className="flex w-fit items-center gap-[7px] text-[13px] font-medium text-text-2 hover:text-text"
      >
        <ArrowLeft className="size-4" aria-hidden />
        {t('backToList')}
      </Link>

      {/* Header card — gxqjg wHM7O */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-[20px] font-semibold text-text">{t('createTitle')}</h1>
          <p className="text-[13px] text-text-2">{t('createSubtitle')}</p>
        </div>
      </div>

      {/* Two-column layout — gxqjg ONQfU */}
      <div className="flex items-start gap-[16px]">
        {/* Left column — UJ5fb */}
        <div className="flex flex-1 flex-col gap-[16px] min-w-0">
          {/* S Tipe Perjanjian — wFQH0 */}
          <SectionCard title={t('secType')} subtitle={t('secTypeHint')}>
            <FormSection>
              <FormField
                label={t('fieldType')}
                htmlFor="type"
                required
                error={errors.type?.message}
              >
                <FilterSelect id="type" aria-label={t('fieldType')} {...register('type')}>
                  <option value={AgreementType.PKWT}>PKWT — {t('pkwtLabel')}</option>
                  <option value={AgreementType.PKWTT}>PKWTT — {t('pkwttLabel')}</option>
                </FilterSelect>
              </FormField>
            </FormSection>
          </SectionCard>

          {/* S Detail PKWT — xH7Lb (always shown; end_date conditional) */}
          <SectionCard
            title={isPKWT ? t('secDetailPKWT') : t('secDetailPKWTT')}
            subtitle={isPKWT ? t('secDetailPKWTHint') : t('secDetailPKWTTHint')}
          >
            <FormSection>
              {/* Employee picker */}
              <FormField
                label={t('fieldEmployee')}
                htmlFor="employee_id"
                required
                error={errors.employee_id?.message}
              >
                <EmployeePicker
                  value={watchedEmployee ?? null}
                  onChange={(val) => setValue('employee_id', val ?? '', { shouldValidate: true })}
                  error={!!errors.employee_id}
                  placeholder={t('fieldEmployeePlaceholder')}
                />
              </FormField>

              <FormField
                label={t('fieldAgreementNo')}
                htmlFor="agreement_no"
                hint={t('fieldAgreementNoHint')}
                error={errors.agreement_no?.message}
              >
                <Input
                  id="agreement_no"
                  placeholder="PKWT/SWP/2026/0143"
                  {...register('agreement_no')}
                />
              </FormField>

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

              {isPKWT && (
                <FormField
                  label={t('fieldEndDate')}
                  htmlFor="end_date"
                  required
                  hint={t('fieldEndDateHint')}
                  error={errors.end_date?.message}
                >
                  <Input
                    id="end_date"
                    type="date"
                    {...register('end_date')}
                    aria-invalid={!!errors.end_date}
                  />
                </FormField>
              )}

              {/* Law info chip — gxqjg M3klw */}
              <div className="flex items-start gap-[8px] rounded-lg border border-info-bd bg-info-bg px-[14px] py-[10px]">
                <Info className="size-4 text-info-tx shrink-0 mt-[1px]" aria-hidden />
                <p className="text-[11px] text-info-tx leading-[1.5]">
                  {isPKWT ? t('lawHintPKWT') : t('lawHintPKWTT')}
                </p>
              </div>
            </FormSection>
          </SectionCard>

          {/* S Kompensasi & Pajak — u5eWg2 */}
          <SectionCard title={t('secCompensation')} subtitle={t('secCompensationHint')}>
            <FormSection>
              <FormField
                label={t('fieldBaseSalary')}
                htmlFor="base_salary_idr"
                hint={t('fieldBaseSalaryHint')}
                error={errors.base_salary_idr?.message}
              >
                <Input
                  id="base_salary_idr"
                  type="number"
                  min={0}
                  placeholder="5000000"
                  {...register('base_salary_idr', { valueAsNumber: true })}
                  aria-invalid={!!errors.base_salary_idr}
                />
              </FormField>

              <FormField
                label={t('fieldCompEffectiveDate')}
                htmlFor="effective_date"
                hint={t('fieldCompEffectiveDateHint')}
                error={errors.effective_date?.message}
              >
                <Input id="effective_date" type="date" {...register('effective_date')} />
              </FormField>

              {/* Encryption notice — gxqjg PvMy9 */}
              <div className="flex items-start gap-[8px] rounded-lg border border-warn-bd bg-warn-bg px-[14px] py-[10px]">
                <Lock className="size-4 text-warn-tx shrink-0 mt-[1px]" aria-hidden />
                <p className="text-[11px] text-warn-tx leading-[1.5]">
                  {t('compensationEncryptNote')}
                </p>
              </div>
            </FormSection>
          </SectionCard>
        </div>

        {/* Right column — dIfiM */}
        <div className="flex w-[380px] shrink-0 flex-col gap-[16px]">
          {/* Summary card — bLCJc */}
          <div className="overflow-hidden rounded-xl border border-border bg-surface">
            <div className="border-b border-border-soft px-5 py-[16px] pb-[12px]">
              <p className="text-[14px] font-semibold text-text">{t('summaryTitle')}</p>
            </div>
            <div className="flex flex-col gap-[10px] p-5">
              <SummaryRow label={t('fieldEmployee')} value={watchedEmployee ?? ''} />
              <SummaryRow label={t('fieldType')} value={watchedType} />
              <SummaryRow label={t('fieldAgreementNo')} value={watchedAgreementNo ?? ''} />
              <SummaryRow label={t('fieldStartDate')} value={watchedStartDate ?? ''} />
              {isPKWT && <SummaryRow label={t('fieldEndDate')} value={watchedEndDate ?? ''} />}
            </div>
          </div>

          {/* Warning: existing active agreement — WLGIN (contextual) */}
          {watchedEmployee && (
            <div className="flex flex-col gap-[8px] rounded-xl border border-warn-bd bg-warn-bg p-[16px]">
              <div className="flex items-center gap-[8px]">
                <TriangleAlert className="size-4 text-warn-tx shrink-0" aria-hidden />
                <span className="text-[13px] font-semibold text-warn-tx">
                  {t('warnActiveTitle')}
                </span>
              </div>
              <p className="text-[11px] text-warn-tx leading-[1.5]">{t('warnActiveBody')}</p>
            </div>
          )}
        </div>
      </div>

      {/* Footer — gxqjg WBvv8 */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[14px]">
        <p className="text-[11px] text-text-3">* {t('footerHint')}</p>
        <div className="flex items-center gap-[10px]">
          <Button
            type="button"
            variant="secondary"
            onClick={() => void navigate({ to: '/agreements' as const })}
            disabled={isSubmitting}
          >
            {t('common.cancel', { ns: 'translation' })}
          </Button>
          <Button type="button" onClick={() => void submitCreate()} disabled={isSubmitting}>
            <Check className="size-4 mr-[6px]" aria-hidden />
            {isSubmitting ? t('common.loading', { ns: 'translation' }) : t('activateSubmit')}
          </Button>
        </div>
      </div>
    </div>
  );
}
