/**
 * E2 · Karyawan — Tambah / Edit form
 *
 * .pen frame h6bDz: full-page create screen. Route `/employees/new` (create) and
 * `/employees/$employeeId/edit` (edit, rendered as Drawer from detail screen).
 *
 * Sections:
 *   1. Data Pribadi  — full_name*, nik*, nip, gender, birth_place+birth_date, join_at*
 *   2. Kontak        — phone, email_personal, address (textarea)
 *   3. Statutori & Bank — npwp, bpjs_kesehatan, bpjs_ketenagakerjaan, bank_name, account_number, account_holder_name
 *   4. Akun Login    — provision_login toggle + login_email (conditional)
 *
 * RHF + hand-written Zod schema (E2 Zod generation deferred).
 * Field errors flow via `applyFieldErrors` (CONVENTIONS §11).
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type Employee,
  type EmployeeWriteRequest,
  Gender,
  useCreateEmployee,
  useGetEmployee,
  useUpdateEmployee,
} from '@swp/api-client/e2';
import {
  Banner,
  Button,
  Drawer,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  FilterSelect,
  FormField,
  FormSection,
  Input,
  Toggle,
  useToast,
} from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { ArrowLeft } from 'lucide-react';
import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Zod schema (hand-written — E2 Zod codegen deferred)
// ---------------------------------------------------------------------------

const employeeSchema = z
  .object({
    full_name: z.string().min(1).max(200),
    nik: z.string().min(16).max(16, 'NIK harus 16 digit'),
    nip: z.string().optional(),
    join_at: z.string().min(1),
    gender: z.nativeEnum(Gender).optional(),
    birth_date: z.string().optional(),
    birth_place: z.string().optional(),
    phone: z.string().optional(),
    email_personal: z.string().email().optional().or(z.literal('')),
    address: z.string().optional(),
    npwp: z.string().optional(),
    bpjs_kesehatan: z.string().optional(),
    bpjs_ketenagakerjaan: z.string().optional(),
    bank_name: z.string().optional(),
    account_number: z.string().optional(),
    account_holder_name: z.string().optional(),
    provision_login: z.boolean().optional().default(false),
    login_email: z.string().optional(),
  })
  .superRefine((data, ctx) => {
    if (data.provision_login && !data.login_email) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Login email wajib diisi jika provisikan login diaktifkan.',
        path: ['login_email'],
      });
    }
  });

type EmployeeFormValues = z.infer<typeof employeeSchema>;

// ---------------------------------------------------------------------------
// Section wrapper
// ---------------------------------------------------------------------------

function FormSectionCard({
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
      <div className="border-b border-border-soft bg-surface-2 px-5 py-[14px]">
        <p className="text-[14px] font-bold text-text">{title}</p>
        {subtitle && <p className="mt-0.5 text-[12px] text-text-3">{subtitle}</p>}
      </div>
      <div className="p-5">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Shared form body (used in both create screen + edit drawer)
// ---------------------------------------------------------------------------

interface EmployeeFormBodyProps {
  form: ReturnType<typeof useForm<EmployeeFormValues>>;
  isEdit?: boolean;
}

function EmployeeFormBody({ form, isEdit }: EmployeeFormBodyProps) {
  const { t } = useTranslation('employees');
  const {
    register,
    formState: { errors },
    watch,
    setValue,
  } = form;

  const provisionLogin = watch('provision_login');

  return (
    <div className="flex flex-col gap-4" style={{ maxWidth: 880 }}>
      {/* Section 1 — Data Pribadi */}
      <FormSectionCard title={t('secDataPribadi')}>
        <FormSection>
          <FormField
            label={t('fieldFullName')}
            htmlFor="full_name"
            required
            error={errors.full_name?.message}
          >
            <Input
              id="full_name"
              placeholder={t('fieldFullNamePlaceholder')}
              {...register('full_name')}
              aria-invalid={!!errors.full_name}
            />
          </FormField>

          <FormField
            label="NIK"
            htmlFor="nik"
            required
            hint={t('fieldNIKHint')}
            error={errors.nik?.message}
          >
            <Input
              id="nik"
              placeholder="16 digit KTP"
              {...register('nik')}
              aria-invalid={!!errors.nik}
            />
          </FormField>

          <FormField label="NIP" htmlFor="nip" error={errors.nip?.message}>
            <Input id="nip" placeholder={t('fieldNIPPlaceholder')} {...register('nip')} />
          </FormField>

          <FormField label={t('fieldGender')} htmlFor="gender" error={errors.gender?.message}>
            <FilterSelect id="gender" aria-label={t('fieldGender')} {...register('gender')}>
              <option value="">{t('fieldGenderPlaceholder')}</option>
              <option value={Gender.MALE}>{t('genderMale')}</option>
              <option value={Gender.FEMALE}>{t('genderFemale')}</option>
            </FilterSelect>
          </FormField>

          <FormField
            label={t('fieldBirthPlace')}
            htmlFor="birth_place"
            error={errors.birth_place?.message}
          >
            <Input id="birth_place" placeholder="Jakarta" {...register('birth_place')} />
          </FormField>

          <FormField
            label={t('fieldBirthDate')}
            htmlFor="birth_date"
            error={errors.birth_date?.message}
          >
            <Input id="birth_date" type="date" {...register('birth_date')} />
          </FormField>

          <FormField
            label={t('fieldJoinAt')}
            htmlFor="join_at"
            required
            error={errors.join_at?.message}
          >
            <Input
              id="join_at"
              type="date"
              {...register('join_at')}
              aria-invalid={!!errors.join_at}
            />
          </FormField>
        </FormSection>
      </FormSectionCard>

      {/* Section 2 — Kontak */}
      <FormSectionCard title={t('secKontak')}>
        <FormSection>
          <FormField label={t('fieldPhone')} htmlFor="phone" error={errors.phone?.message}>
            <Input id="phone" type="tel" placeholder="+62 812-3456-7890" {...register('phone')} />
          </FormField>

          <FormField
            label={t('fieldEmailPersonal')}
            htmlFor="email_personal"
            error={errors.email_personal?.message}
          >
            <Input
              id="email_personal"
              type="email"
              placeholder="nama@gmail.com"
              {...register('email_personal')}
            />
          </FormField>

          <FormField
            label={t('fieldAddress')}
            htmlFor="address"
            span={2}
            error={errors.address?.message}
          >
            <textarea
              id="address"
              rows={3}
              placeholder={t('fieldAddressPlaceholder')}
              className="w-full rounded-lg border border-border bg-surface px-3 py-[10px] text-[13px] text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary"
              {...register('address')}
            />
          </FormField>
        </FormSection>
      </FormSectionCard>

      {/* Section 3 — Statutori & Bank */}
      <FormSectionCard title={t('secStatutori')}>
        <FormSection>
          <FormField label="NPWP" htmlFor="npwp" error={errors.npwp?.message}>
            <Input id="npwp" placeholder="XX.XXX.XXX.X-XXX.XXX" {...register('npwp')} />
          </FormField>

          <FormField
            label="BPJS Kesehatan"
            htmlFor="bpjs_kesehatan"
            error={errors.bpjs_kesehatan?.message}
          >
            <Input id="bpjs_kesehatan" placeholder="13 digit" {...register('bpjs_kesehatan')} />
          </FormField>

          <FormField
            label="BPJS Ketenagakerjaan"
            htmlFor="bpjs_ketenagakerjaan"
            error={errors.bpjs_ketenagakerjaan?.message}
          >
            <Input
              id="bpjs_ketenagakerjaan"
              placeholder="10 digit"
              {...register('bpjs_ketenagakerjaan')}
            />
          </FormField>

          <FormField
            label={t('fieldBankName')}
            htmlFor="bank_name"
            error={errors.bank_name?.message}
          >
            <Input id="bank_name" placeholder="BCA / BRI / Mandiri…" {...register('bank_name')} />
          </FormField>

          <FormField
            label={t('fieldAccountNumber')}
            htmlFor="account_number"
            error={errors.account_number?.message}
          >
            <Input
              id="account_number"
              placeholder="Nomor rekening"
              {...register('account_number')}
            />
          </FormField>

          <FormField
            label={t('fieldAccountHolder')}
            htmlFor="account_holder_name"
            error={errors.account_holder_name?.message}
          >
            <Input
              id="account_holder_name"
              placeholder={t('fieldFullNamePlaceholder')}
              {...register('account_holder_name')}
            />
          </FormField>
        </FormSection>
      </FormSectionCard>

      {/* Section 4 — Akun Login (create only) */}
      {!isEdit && (
        <FormSectionCard title={t('secAkunLogin')} subtitle={t('secAkunLoginSubtitle')}>
          <div className="flex flex-col gap-4">
            {/* Toggle row */}
            <div className="flex items-center justify-between gap-3 rounded-[10px] border border-primary-soft bg-primary-soft px-[14px] py-3">
              <div className="flex flex-col gap-[1px]">
                <span className="text-[13px] font-semibold text-text">
                  {t('fieldProvisionLogin')}
                </span>
                <span className="text-[12px] text-text-2">{t('fieldProvisionLoginSub')}</span>
              </div>
              <Toggle
                checked={provisionLogin ?? false}
                onCheckedChange={(checked) => setValue('provision_login', checked)}
                aria-label={t('fieldProvisionLogin')}
              />
            </div>

            {provisionLogin && (
              <>
                <Banner
                  tone="info"
                  title={t('loginProvisionBannerTitle')}
                  description={t('loginProvisionBanner')}
                />
                <FormSection>
                  <FormField
                    label={t('fieldLoginEmail')}
                    htmlFor="login_email"
                    required
                    error={errors.login_email?.message}
                  >
                    <Input
                      id="login_email"
                      type="email"
                      placeholder="email@contoh.com"
                      {...register('login_email')}
                      aria-invalid={!!errors.login_email}
                    />
                  </FormField>
                </FormSection>
              </>
            )}
          </div>
        </FormSectionCard>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// CreateEmployeeScreen — full-page route `/employees/new`
// ---------------------------------------------------------------------------

export function CreateEmployeeScreen() {
  const { t } = useTranslation('employees');
  const navigate = useNavigate();
  const { toast } = useToast();

  const form = useForm<EmployeeFormValues>({
    resolver: zodResolver(employeeSchema),
    defaultValues: { provision_login: false },
  });

  const mutation = useCreateEmployee();

  async function onSubmit(values: EmployeeFormValues) {
    const body: EmployeeWriteRequest = {
      full_name: values.full_name,
      nik: values.nik,
      nip: values.nip || undefined,
      join_at: values.join_at,
      gender: values.gender,
      birth_date: values.birth_date || undefined,
      birth_place: values.birth_place || undefined,
      phone: values.phone || undefined,
      email_personal: values.email_personal || undefined,
      address: values.address || undefined,
      npwp: values.npwp || undefined,
      bpjs_kesehatan: values.bpjs_kesehatan || undefined,
      bpjs_ketenagakerjaan: values.bpjs_ketenagakerjaan || undefined,
      bank_account:
        values.bank_name || values.account_number
          ? {
              bank_name: values.bank_name,
              account_number: values.account_number,
              account_holder_name: values.account_holder_name,
            }
          : undefined,
      provision_login: values.provision_login,
      login_email: values.provision_login ? values.login_email : undefined,
    };

    try {
      const result = await mutation.mutateAsync({ data: body });
      toast({ tone: 'success', title: t('createSuccess') });
      const created = (result.data as { data?: { id?: string } })?.data;
      if (created?.id) {
        void navigate({
          to: '/employees/$employeeId' as const,
          params: { employeeId: created.id },
        });
      } else {
        void navigate({ to: '/employees' as const });
      }
    } catch (err) {
      if (!applyFieldErrors(err, form.setError)) {
        const { message } = classifyError(err);
        toast({ tone: 'error', title: t('createError'), description: message });
      }
    }
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Back */}
      <button
        type="button"
        className="flex items-center gap-[7px] text-[13px] font-medium text-text-2 hover:text-text"
        onClick={() => void navigate({ to: '/employees' as const })}
      >
        <ArrowLeft className="size-4" aria-hidden />
        {t('backToList')}
      </button>

      {/* Head */}
      <div className="flex flex-col gap-1">
        <h1 className="text-[24px] font-bold text-text">{t('createTitle')}</h1>
        <p className="text-[13px] text-text-2">{t('createSubtitle')}</p>
      </div>

      {/* Form */}
      <form onSubmit={form.handleSubmit(onSubmit)} noValidate>
        <EmployeeFormBody form={form} />

        {/* Footer */}
        <div className="mt-4 flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[14px]">
          <span className="text-[12px] text-text-3">* {t('requiredNote')}</span>
          <div className="flex items-center gap-[10px]">
            <Button
              type="button"
              variant="secondary"
              onClick={() => void navigate({ to: '/employees' as const })}
            >
              {t('cancel')}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t('saving') : t('createSubmit')}
            </Button>
          </div>
        </div>
      </form>
    </div>
  );
}

// ---------------------------------------------------------------------------
// EditEmployeeScreen — Drawer, used from detail page
// ---------------------------------------------------------------------------

export interface EditEmployeeScreenProps {
  employeeId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDone: () => void;
}

export function EditEmployeeScreen({
  employeeId,
  open,
  onOpenChange,
  onDone,
}: EditEmployeeScreenProps) {
  const { t } = useTranslation('employees');
  const { toast } = useToast();

  const getQuery = useGetEmployee(employeeId, { query: { enabled: open } });
  const emp = getQuery.data?.data as Employee | undefined;

  const form = useForm<EmployeeFormValues>({
    resolver: zodResolver(employeeSchema),
    defaultValues: { provision_login: false },
  });

  // Populate form when employee data loads
  useEffect(() => {
    if (!emp) return;
    form.reset({
      full_name: emp.full_name,
      nik: emp.nik,
      nip: emp.nip ?? '',
      join_at: emp.join_at,
      gender: emp.gender,
      birth_date: emp.birth_date ?? '',
      birth_place: emp.birth_place ?? '',
      phone: emp.phone ?? '',
      email_personal: emp.email_personal ?? '',
      address: emp.address ?? '',
      npwp: emp.npwp ?? '',
      bpjs_kesehatan: emp.bpjs_kesehatan ?? '',
      bpjs_ketenagakerjaan: emp.bpjs_ketenagakerjaan ?? '',
      bank_name: emp.bank_account?.bank_name ?? '',
      account_number: emp.bank_account?.account_number ?? '',
      account_holder_name: emp.bank_account?.account_holder_name ?? '',
    });
  }, [emp, form]);

  const mutation = useUpdateEmployee();

  async function onSubmit(values: EmployeeFormValues) {
    const body: EmployeeWriteRequest = {
      full_name: values.full_name,
      nik: values.nik,
      nip: values.nip || undefined,
      join_at: values.join_at,
      gender: values.gender,
      birth_date: values.birth_date || undefined,
      birth_place: values.birth_place || undefined,
      phone: values.phone || undefined,
      email_personal: values.email_personal || undefined,
      address: values.address || undefined,
      npwp: values.npwp || undefined,
      bpjs_kesehatan: values.bpjs_kesehatan || undefined,
      bpjs_ketenagakerjaan: values.bpjs_ketenagakerjaan || undefined,
      bank_account:
        values.bank_name || values.account_number
          ? {
              bank_name: values.bank_name,
              account_number: values.account_number,
              account_holder_name: values.account_holder_name,
            }
          : undefined,
    };

    try {
      await mutation.mutateAsync({ employeeId, data: body });
      toast({ tone: 'success', title: t('editSuccess') });
      onDone();
    } catch (err) {
      if (!applyFieldErrors(err, form.setError)) {
        const { message } = classifyError(err);
        toast({ tone: 'error', title: t('editError'), description: message });
      }
    }
  }

  return (
    <Drawer open={open} onOpenChange={onOpenChange} width={640}>
      <DrawerHeader
        title={t('editTitle')}
        subtitle={emp ? emp.full_name : undefined}
        onClose={() => onOpenChange(false)}
        closeLabel={t('cancel')}
      />
      <DrawerBody>
        {getQuery.isLoading ? (
          <div className="flex flex-col gap-3 animate-pulse p-4">
            <div className="h-4 w-48 rounded bg-surface-2" />
            <div className="h-4 w-full rounded bg-surface-2" />
          </div>
        ) : (
          <form id="edit-employee-form" onSubmit={form.handleSubmit(onSubmit)} noValidate>
            <EmployeeFormBody form={form} isEdit />
          </form>
        )}
      </DrawerBody>
      <DrawerFooter>
        <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
          {t('cancel')}
        </Button>
        <Button
          type="submit"
          form="edit-employee-form"
          disabled={mutation.isPending || getQuery.isLoading}
        >
          {mutation.isPending ? t('saving') : t('editSubmit')}
        </Button>
      </DrawerFooter>
    </Drawer>
  );
}
