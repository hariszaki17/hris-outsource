/**
 * E2 · Perusahaan Klien — Tambah / Edit
 *
 * .pen frame: ZmJnZ — E2 · Perusahaan Klien — Tambah
 *
 * Exports:
 *   - CreateClientCompanyScreen  — full-page create route (/client-companies/new)
 *   - EditClientCompanyScreen    — full-page edit route (/client-companies/$id/edit)
 *
 * Sections (two-column layout matching .pen L + R):
 *   Left col:
 *     S1. Profil Perusahaan — name*, address*, npwp, pic_name, email, phone
 *     S2. Lokasi & Geofence — geo.lat, geo.lng, geofence_radius_m
 *   Right col:
 *     Ringkasan & Validasi card (live field-status checklist)
 *     Pedoman card (guidelines)
 *
 * RHF + hand-written Zod (E2 Zod codegen deferred).
 * Field errors from server → applyFieldErrors (CONVENTIONS §11).
 *
 * F2.3 — CC-1 (create), CC-2 (NPWP unique). INV: name unique.
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type ClientCompany,
  type ClientCompanyWriteRequest,
  useCreateClientCompany,
  useGetClientCompany,
  useUpdateClientCompany,
} from '@swp/api-client/e2';
import { Button, FormField, Input, StateView, useToast } from '@swp/ui';
import { Link, useNavigate } from '@tanstack/react-router';
import { ArrowLeft, CheckCircle2, Circle, CircleAlert, Info, ShieldCheck } from 'lucide-react';
import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Zod schema (hand-written — E2 Zod codegen deferred)
// ---------------------------------------------------------------------------

const clientCompanySchema = z.object({
  name: z.string().min(1, 'Nama perusahaan wajib diisi').max(200),
  address: z.string().min(1, 'Alamat wajib diisi'),
  npwp: z.string().optional(),
  pic_name: z.string().optional(),
  email: z.string().email('Format email tidak valid').optional().or(z.literal('')),
  phone: z.string().optional(),
  // Shift-leadership granularity (E3 F3.4). Geofence/location now lives on Sites (F2.6).
  leader_scope: z.enum(['company', 'site']).default('company'),
});

type ClientCompanyFormValues = z.infer<typeof clientCompanySchema>;

// ---------------------------------------------------------------------------
// Build the write request from form values
// ---------------------------------------------------------------------------

function buildWriteRequest(values: ClientCompanyFormValues): ClientCompanyWriteRequest {
  return {
    name: values.name,
    address: values.address,
    npwp: values.npwp || undefined,
    pic_name: values.pic_name || undefined,
    email: values.email || undefined,
    phone: values.phone || undefined,
    leader_scope: values.leader_scope,
  };
}

// ---------------------------------------------------------------------------
// Validation summary (right col Ringkasan card)
// ---------------------------------------------------------------------------

interface ValidationSummaryProps {
  values: Partial<ClientCompanyFormValues>;
}

type CheckState = 'ok' | 'warn' | 'empty';

function CheckRow({ label, state }: { label: string; state: CheckState }) {
  return (
    <div className="flex items-center gap-[10px]">
      {state === 'ok' ? (
        <CheckCircle2 size={16} className="text-ok-tx shrink-0" aria-hidden />
      ) : state === 'warn' ? (
        <CircleAlert size={16} className="text-warn-tx shrink-0" aria-hidden />
      ) : (
        <Circle size={16} className="text-text-3 shrink-0" aria-hidden />
      )}
      <span className="text-[12px] text-text-2">{label}</span>
    </div>
  );
}

function ValidationSummary({ values }: ValidationSummaryProps) {
  const { t } = useTranslation('clientCompanies');
  return (
    <div className="rounded-xl bg-surface border border-border overflow-hidden">
      <div className="px-5 pt-4 pb-3 border-b border-border-soft">
        <span className="text-[14px] font-semibold text-text">{t('form.summaryCard.title')}</span>
      </div>
      <div className="flex flex-col gap-[10px] p-5">
        <CheckRow label={t('fields.name')} state={values.name ? 'ok' : 'warn'} />
        <CheckRow label={t('fields.address')} state={values.address ? 'ok' : 'warn'} />
        <CheckRow
          label={`${t('fields.npwp')} (${t('form.optional')})`}
          state={values.npwp ? 'ok' : 'empty'}
        />
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Guideline card (right col)
// ---------------------------------------------------------------------------

function GuidelineCard() {
  const { t } = useTranslation('clientCompanies');
  return (
    <div className="rounded-xl bg-info-bg border border-info-bd overflow-hidden p-4 flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <ShieldCheck size={16} className="text-info-tx shrink-0" aria-hidden />
        <span className="text-[13px] font-semibold text-info-tx">
          {t('form.guidelineCard.title')}
        </span>
      </div>
      <p className="text-[11px] text-info-tx leading-relaxed whitespace-pre-line">
        {t('form.guidelineCard.body')}
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Shared form body
// ---------------------------------------------------------------------------

interface ClientCompanyFormBodyProps {
  form: ReturnType<typeof useForm<ClientCompanyFormValues>>;
}

function ClientCompanyFormBody({ form }: ClientCompanyFormBodyProps) {
  const { t } = useTranslation('clientCompanies');
  const {
    register,
    watch,
    formState: { errors },
  } = form;

  const values = watch();

  return (
    <div className="flex gap-4">
      {/* Left col */}
      <div className="flex-1 flex flex-col gap-4">
        {/* Section: Profil Perusahaan */}
        <div className="rounded-xl bg-surface border border-border overflow-hidden">
          <div className="px-5 pt-[18px] pb-3 border-b border-border-soft flex flex-col gap-[2px]">
            <span className="text-[15px] font-semibold text-text">
              {t('form.profileSection.title')}
            </span>
            <span className="text-[12px] text-text-2">{t('form.profileSection.subtitle')}</span>
          </div>
          <div className="flex flex-col gap-[14px] p-5">
            {/* Row 1: name + npwp */}
            <div className="flex gap-[14px]">
              <FormField
                htmlFor="cc-name"
                label={`${t('fields.name')} *`}
                error={errors.name?.message}
                className="flex-1"
              >
                <Input
                  id="cc-name"
                  {...register('name')}
                  placeholder={t('fields.namePlaceholder')}
                  aria-required
                />
              </FormField>
              <FormField
                htmlFor="cc-npwp"
                label={t('fields.npwp')}
                error={errors.npwp?.message}
                className="flex-1"
              >
                <Input
                  id="cc-npwp"
                  {...register('npwp')}
                  placeholder={t('fields.npwpPlaceholder')}
                  className="font-mono"
                />
              </FormField>
            </div>

            {/* Row: address */}
            <FormField
              htmlFor="cc-address"
              label={`${t('fields.address')} *`}
              error={errors.address?.message}
            >
              <Input
                id="cc-address"
                {...register('address')}
                placeholder={t('fields.addressPlaceholder')}
                aria-required
              />
            </FormField>

            {/* Row 2: pic + email */}
            <div className="flex gap-[14px]">
              <FormField
                htmlFor="cc-pic-name"
                label={t('fields.pic')}
                error={errors.pic_name?.message}
                className="flex-1"
              >
                <Input
                  id="cc-pic-name"
                  {...register('pic_name')}
                  placeholder={t('fields.picPlaceholder')}
                />
              </FormField>
              <FormField
                htmlFor="cc-email"
                label={t('fields.email')}
                error={errors.email?.message}
                className="flex-1"
              >
                <Input
                  id="cc-email"
                  {...register('email')}
                  type="email"
                  placeholder={t('fields.emailPlaceholder')}
                />
              </FormField>
            </div>

            {/* Row 3: phone + leader scope */}
            <div className="flex gap-[14px]">
              <FormField
                htmlFor="cc-phone"
                label={t('fields.phone')}
                error={errors.phone?.message}
                className="flex-1"
              >
                <Input
                  id="cc-phone"
                  {...register('phone')}
                  placeholder={t('fields.phonePlaceholder')}
                  className="font-mono"
                />
              </FormField>
              <FormField
                htmlFor="cc-leader-scope"
                label={t('fields.leaderScope')}
                error={errors.leader_scope?.message}
                className="flex-1"
              >
                <select
                  id="cc-leader-scope"
                  {...register('leader_scope')}
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 text-[13px] text-text outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <option value="company">{t('fields.leaderScopeCompany')}</option>
                  <option value="site">{t('fields.leaderScopeSite')}</option>
                </select>
              </FormField>
            </div>

            {/* Geofence moved to Sites (F2.6) */}
            <div className="flex items-start gap-2 rounded-lg bg-info-bg border border-info-bd px-3 py-[10px]">
              <Info size={14} className="text-info-tx shrink-0 mt-0.5" aria-hidden />
              <p className="text-[11px] text-info-tx leading-relaxed">
                {t('form.geofenceMovedHint')}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Right col: summary + guidelines */}
      <div className="w-[380px] flex flex-col gap-4">
        <ValidationSummary values={values} />
        <GuidelineCard />
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Create screen (full page)
// ---------------------------------------------------------------------------

export function CreateClientCompanyScreen() {
  const { t } = useTranslation('clientCompanies');
  const navigate = useNavigate();
  const { toast } = useToast();

  const form = useForm<ClientCompanyFormValues>({
    resolver: zodResolver(clientCompanySchema),
    defaultValues: {
      name: '',
      address: '',
      npwp: '',
      pic_name: '',
      email: '',
      phone: '',
      leader_scope: 'company',
    },
  });

  const createMut = useCreateClientCompany();

  function onSubmit(values: ClientCompanyFormValues) {
    createMut.mutate(
      { data: buildWriteRequest(values) },
      {
        onSuccess: (res) => {
          const created = res.data as ClientCompany;
          toast({ tone: 'success', title: t('toast.created') });
          navigate({
            to: '/client-companies/$clientCompanyId' as never,
            params: { clientCompanyId: created.id } as never,
          });
        },
        onError: (err) => {
          if (applyFieldErrors(err, form.setError as never)) return;
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t('toast.createFailed'), description: message });
        },
      },
    );
  }

  return (
    <div className="flex flex-col gap-4 p-6 bg-app-bg h-full overflow-y-auto">
      {/* Back row */}
      <div className="flex items-center gap-[7px]">
        <Link
          to={'/client-companies' as never}
          className="flex items-center gap-[7px] text-text-2 hover:text-text"
        >
          <ArrowLeft size={16} aria-hidden />
          <span className="text-[13px] font-medium">{t('form.backLink')}</span>
        </Link>
      </div>

      {/* Header */}
      <div className="rounded-xl bg-surface border border-border px-5 py-[18px] flex items-center justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-[18px] font-semibold text-text">{t('form.createTitle')}</h1>
          <p className="text-[12px] text-text-2">{t('form.createSubtitle')}</p>
        </div>
      </div>

      {/* Form body */}
      <form onSubmit={form.handleSubmit(onSubmit)} noValidate>
        <ClientCompanyFormBody form={form} />

        {/* Footer */}
        <div className="mt-4 rounded-xl bg-surface border border-border px-5 py-[14px] flex items-center justify-between">
          <span className="text-[11px] text-text-3">{t('form.footerHint')}</span>
          <div className="flex items-center gap-[10px]">
            <Link to={'/client-companies' as never}>
              <Button type="button" variant="secondary">
                {t('form.cancel')}
              </Button>
            </Link>
            <Button type="submit" disabled={createMut.isPending}>
              {createMut.isPending ? t('form.saving') : t('form.save')}
            </Button>
          </div>
        </div>
      </form>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Edit screen (full page)
// ---------------------------------------------------------------------------

export function EditClientCompanyScreen({ clientCompanyId }: { clientCompanyId: string }) {
  const { t } = useTranslation('clientCompanies');
  const navigate = useNavigate();
  const { toast } = useToast();

  const query = useGetClientCompany(clientCompanyId);
  const company = query.data?.data as ClientCompany | undefined;

  const form = useForm<ClientCompanyFormValues>({
    resolver: zodResolver(clientCompanySchema),
    defaultValues: {
      name: '',
      address: '',
      npwp: '',
      pic_name: '',
      email: '',
      phone: '',
      leader_scope: 'company',
    },
  });

  // Populate form when data loads
  useEffect(() => {
    if (company) {
      form.reset({
        name: company.name ?? '',
        address: company.address ?? '',
        npwp: company.npwp ?? '',
        pic_name: company.pic_name ?? '',
        email: company.email ?? '',
        phone: company.phone ?? '',
        leader_scope: company.leader_scope ?? 'company',
      });
    }
  }, [company, form]);

  const updateMut = useUpdateClientCompany();

  function onSubmit(values: ClientCompanyFormValues) {
    updateMut.mutate(
      { clientCompanyId, data: buildWriteRequest(values) },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('toast.updated') });
          navigate({
            to: '/client-companies/$clientCompanyId' as never,
            params: { clientCompanyId } as never,
          });
        },
        onError: (err) => {
          if (applyFieldErrors(err, form.setError as never)) return;
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t('toast.updateFailed'), description: message });
        },
      },
    );
  }

  if (query.isPending) {
    return (
      <div className="p-6 bg-app-bg h-full">
        <StateView kind="loading" title={t('state.loading')} />
      </div>
    );
  }

  if (query.isError || !company) {
    return (
      <div className="p-6 bg-app-bg h-full">
        <StateView
          kind="error"
          title={t('state.errorTitle')}
          description={classifyError(query.error).message}
          onRetry={() => void query.refetch()}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 p-6 bg-app-bg h-full overflow-y-auto">
      {/* Back row */}
      <div className="flex items-center gap-[7px]">
        <Link
          to={'/client-companies/$clientCompanyId' as never}
          params={{ clientCompanyId } as never}
          className="flex items-center gap-[7px] text-text-2 hover:text-text"
        >
          <ArrowLeft size={16} aria-hidden />
          <span className="text-[13px] font-medium">{t('detail.backLink')}</span>
        </Link>
      </div>

      {/* Header */}
      <div className="rounded-xl bg-surface border border-border px-5 py-[18px] flex items-center justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-[18px] font-semibold text-text">{t('form.editTitle')}</h1>
          <p className="text-[12px] text-text-2">{t('form.editSubtitle')}</p>
        </div>
      </div>

      {/* Form body */}
      <form onSubmit={form.handleSubmit(onSubmit)} noValidate>
        <ClientCompanyFormBody form={form} />

        {/* Footer */}
        <div className="mt-4 rounded-xl bg-surface border border-border px-5 py-[14px] flex items-center justify-between">
          <span className="text-[11px] text-text-3">{t('form.footerHint')}</span>
          <div className="flex items-center gap-[10px]">
            <Link
              to={'/client-companies/$clientCompanyId' as never}
              params={{ clientCompanyId } as never}
            >
              <Button type="button" variant="secondary">
                {t('form.cancel')}
              </Button>
            </Link>
            <Button type="submit" disabled={updateMut.isPending}>
              {updateMut.isPending ? t('form.saving') : t('form.save')}
            </Button>
          </div>
        </div>
      </form>
    </div>
  );
}
