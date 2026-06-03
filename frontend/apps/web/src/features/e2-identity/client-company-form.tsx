/**
 * E2 · Perusahaan Klien — Tambah / Edit
 *
 * .pen frame: ZmJnZ — E2 · Perusahaan Klien — Tambah
 *
 * Exports:
 *   - CreateClientCompanyScreen  — full-page create route (/client-companies/new)
 *   - EditClientCompanyDrawer    — Drawer variant for edit-in-place from list or detail
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
import {
  Button,
  Drawer,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  FormField,
  Input,
  StateView,
  useToast,
} from '@swp/ui';
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
  // Geofence / location (all optional — geo = null if lat/lng absent)
  lat: z
    .string()
    .optional()
    .refine((v) => !v || (!Number.isNaN(Number(v)) && Number(v) >= -90 && Number(v) <= 90), {
      message: 'Latitude harus antara -90 dan 90',
    }),
  lng: z
    .string()
    .optional()
    .refine((v) => !v || (!Number.isNaN(Number(v)) && Number(v) >= -180 && Number(v) <= 180), {
      message: 'Longitude harus antara -180 dan 180',
    }),
  geofence_radius_m: z.coerce
    .number()
    .min(25, 'Minimum 25m')
    .max(1000, 'Maksimum 1000m')
    .default(100),
});

type ClientCompanyFormValues = z.infer<typeof clientCompanySchema>;

// ---------------------------------------------------------------------------
// Build the write request from form values
// ---------------------------------------------------------------------------

function buildWriteRequest(values: ClientCompanyFormValues): ClientCompanyWriteRequest {
  const hasGeo = values.lat && values.lng;
  return {
    name: values.name,
    address: values.address,
    npwp: values.npwp || undefined,
    pic_name: values.pic_name || undefined,
    email: values.email || undefined,
    phone: values.phone || undefined,
    geo: hasGeo ? { lat: Number(values.lat), lng: Number(values.lng) } : null,
    geofence_radius_m: values.geofence_radius_m,
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
  const hasGeo = !!(values.lat && values.lng);
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
        <CheckRow label={t('form.summaryCard.geoOptional')} state={hasGeo ? 'ok' : 'empty'} />
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

            {/* Row 3: phone */}
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
              <div className="flex-1" />
            </div>
          </div>
        </div>

        {/* Section: Lokasi & Geofence */}
        <div className="rounded-xl bg-surface border border-border overflow-hidden">
          <div className="px-5 pt-[18px] pb-3 border-b border-border-soft flex flex-col gap-[2px]">
            <span className="text-[15px] font-semibold text-text">
              {t('form.geofenceSection.title')}
            </span>
            <span className="text-[12px] text-text-2">{t('form.geofenceSection.subtitle')}</span>
          </div>
          <div className="flex flex-col gap-[14px] p-5">
            {/* No geo banner */}
            {!values.lat && !values.lng && (
              <div className="flex items-center gap-[10px] rounded-lg bg-warn-bg border border-warn-bd px-[14px] py-[10px]">
                <span className="text-[13px] text-warn-tx">
                  {t('form.geofenceSection.noGeoHint')}
                </span>
              </div>
            )}

            {/* lat / lng row */}
            <div className="flex gap-[14px]">
              <FormField
                htmlFor="cc-lat"
                label={t('fields.latitude')}
                error={errors.lat?.message}
                className="flex-1"
              >
                <Input
                  id="cc-lat"
                  {...register('lat')}
                  placeholder="−6.2088"
                  className="font-mono"
                />
              </FormField>
              <FormField
                htmlFor="cc-lng"
                label={t('fields.longitude')}
                error={errors.lng?.message}
                className="flex-1"
              >
                <Input
                  id="cc-lng"
                  {...register('lng')}
                  placeholder="106.8456"
                  className="font-mono"
                />
              </FormField>
            </div>

            {/* radius row */}
            <div className="flex gap-[14px]">
              <FormField
                htmlFor="cc-radius"
                label={t('fields.geofenceRadius')}
                error={errors.geofence_radius_m?.message}
                className="flex-1"
              >
                <Input
                  id="cc-radius"
                  {...register('geofence_radius_m')}
                  type="number"
                  min={25}
                  max={1000}
                  className="font-mono"
                />
              </FormField>
              <div className="flex-1" />
            </div>

            {/* hint */}
            <div className="flex items-start gap-2">
              <Info size={13} className="text-text-3 shrink-0 mt-0.5" aria-hidden />
              <p className="text-[11px] text-text-3">{t('form.geofenceSection.hint')}</p>
            </div>

            {/* Map picker placeholder */}
            <div className="flex items-center justify-center rounded-lg bg-surface-2 border border-border-soft h-[280px]">
              <div className="flex flex-col items-center gap-3 text-text-2 p-5">
                <div className="rounded-xl p-3" style={{ background: 'var(--color-surface)' }}>
                  {/* static map pin illustration */}
                  <svg
                    width="32"
                    height="40"
                    viewBox="0 0 32 40"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg"
                    aria-hidden="true"
                  >
                    <path
                      d="M16 0C7.163 0 0 7.163 0 16c0 10.317 14.37 23.05 15.18 23.75a1.25 1.25 0 0 0 1.64 0C17.63 39.05 32 26.317 32 16 32 7.163 24.837 0 16 0Z"
                      fill="#188E4D"
                      fillOpacity="0.15"
                    />
                    <circle cx="16" cy="16" r="6" fill="#188E4D" />
                  </svg>
                </div>
                <span className="text-[12px] italic text-center">
                  {t('form.geofenceSection.mapPlaceholder')}
                </span>
              </div>
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
      lat: '',
      lng: '',
      geofence_radius_m: 100,
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
// Edit drawer
// ---------------------------------------------------------------------------

interface EditClientCompanyDrawerProps {
  clientCompanyId: string;
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
}

export function EditClientCompanyDrawer({
  clientCompanyId,
  open,
  onClose,
  onSaved,
}: EditClientCompanyDrawerProps) {
  const { t } = useTranslation('clientCompanies');
  const { toast } = useToast();

  const query = useGetClientCompany(clientCompanyId, { query: { enabled: open } });
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
      lat: '',
      lng: '',
      geofence_radius_m: 100,
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
        lat: company.geo?.lat != null ? String(company.geo.lat) : '',
        lng: company.geo?.lng != null ? String(company.geo.lng) : '',
        geofence_radius_m: company.geofence_radius_m ?? 100,
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
          onSaved();
        },
        onError: (err) => {
          if (applyFieldErrors(err, form.setError as never)) return;
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t('toast.updateFailed'), description: message });
        },
      },
    );
  }

  return (
    <Drawer
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
      width={640}
    >
      <DrawerHeader title={t('form.editTitle')} onClose={onClose} />
      <DrawerBody>
        {query.isPending ? (
          <StateView kind="loading" title={t('state.loading')} />
        ) : query.isError ? (
          <StateView
            kind="error"
            title={t('state.errorTitle')}
            description={classifyError(query.error).message}
            onRetry={() => void query.refetch()}
          />
        ) : (
          <form id="edit-client-company-form" onSubmit={form.handleSubmit(onSubmit)} noValidate>
            <ClientCompanyFormBody form={form} />
          </form>
        )}
      </DrawerBody>
      <DrawerFooter>
        <span className="text-[11px] text-text-3 flex-1">{t('form.footerHint')}</span>
        <Button type="button" variant="secondary" onClick={onClose}>
          {t('form.cancel')}
        </Button>
        <Button
          type="submit"
          form="edit-client-company-form"
          disabled={updateMut.isPending || query.isPending}
        >
          {updateMut.isPending ? t('form.saving') : t('form.save')}
        </Button>
      </DrawerFooter>
    </Drawer>
  );
}
