import { useCurrentUser } from '@/lib/use-auth.ts';
/**
 * /me/akun — Agent "Akun" (account). Merges the old /me/profile and /me/payslip surfaces into
 * one screen with two in-page tabs (Plan §E · brainstorm.pen frames i0zzG / l2BKa):
 *   - Profil   → read-only profile card + photo + "Ubah Profil" modal
 *   - Slip Gaji→ payslip history (cursor pagination)
 *
 * "Ubah Profil" modal — ALL agent self-edits are now INSTANT (no approval queue) since the E2
 * profile change-request surface was removed (EPICS §8 E11, 2026-06-14). Every editable field —
 * alamat, bahasa aplikasi, foto, telepon, kontak darurat, rekening bank — is applied immediately
 * via a single `PATCH /me/profile` (useUpdateMyProfile). Photo flow: useInitProfilePhotoUpload →
 * PUT the bytes to the returned presigned URL → pass `photo_object_key` to PATCH. Phone is a
 * unique login identifier (E2 D2): a 409 conflict surfaces inline on the phone field.
 */
import { ApiError } from '@swp/api-client';
import {
  type BankAccount,
  type Employee,
  InitProfilePhotoUploadBodyContentType,
  type SelfProfileUpdate,
  type UploadTicket,
  useGetEmployee,
  useInitProfilePhotoUpload,
  useUpdateMyProfile,
} from '@swp/api-client/e2';
import { type Payslip, type PayslipListResponse, useListPayslips } from '@swp/api-client/e8';
import { formatDate } from '@swp/shared';
import {
  Avatar,
  Button,
  type Column,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  FilterSelect,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { ImagePlus, Info, Pencil } from 'lucide-react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  formatMoney,
  formatPeriod,
  payslipStatusKey,
  payslipStatusTone,
} from '../e8-payroll/payroll-shared.tsx';
import { AgentPage } from './agent-page.tsx';

type AkunTab = 'profile' | 'payslip';

const PAGE_SIZE = 20;

const CONTENT_TYPE_BY_EXT: Record<string, InitProfilePhotoUploadBodyContentType> = {
  jpg: InitProfilePhotoUploadBodyContentType['image/jpeg'],
  jpeg: InitProfilePhotoUploadBodyContentType['image/jpeg'],
  png: InitProfilePhotoUploadBodyContentType['image/png'],
  webp: InitProfilePhotoUploadBodyContentType['image/webp'],
};

function contentTypeForFile(file: File): InitProfilePhotoUploadBodyContentType | undefined {
  const direct = (Object.values(InitProfilePhotoUploadBodyContentType) as string[]).includes(
    file.type,
  )
    ? (file.type as InitProfilePhotoUploadBodyContentType)
    : undefined;
  if (direct) return direct;
  const ext = file.name.split('.').pop()?.toLowerCase() ?? '';
  return CONTENT_TYPE_BY_EXT[ext];
}

function initialsOf(name?: string): string {
  if (!name) return '?';
  const parts = name.trim().split(/\s+/);
  return (parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '');
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function AgentAkunScreen() {
  const { t } = useTranslation('agent');
  const [tab, setTab] = useState<AkunTab>('profile');
  const [editOpen, setEditOpen] = useState(false);

  const user = useCurrentUser();
  const employeeId = user?.employeeId ?? '';
  const q = useGetEmployee(employeeId, { query: { enabled: !!employeeId } });
  const emp = q.data?.data as Employee | undefined;

  const tabs: { id: AkunTab; label: string }[] = [
    { id: 'profile', label: t('profileTitle') },
    { id: 'payslip', label: t('payslipTitle') },
  ];

  const actions =
    tab === 'profile' ? (
      <Button variant="primary" size="sm" disabled={!emp} onClick={() => setEditOpen(true)}>
        <Pencil className="size-4" aria-hidden />
        {t('profileEditBtn')}
      </Button>
    ) : undefined;

  return (
    <AgentPage title={t('akunTitle')} actions={actions}>
      {/* In-page tabs */}
      <div className="flex items-center gap-7 border-b border-border">
        {tabs.map((tb) => (
          <button
            key={tb.id}
            type="button"
            onClick={() => setTab(tb.id)}
            className={[
              '-mb-px border-b-2 py-3 text-[14px] font-medium transition-colors',
              tab === tb.id
                ? 'border-primary font-semibold text-primary'
                : 'border-transparent text-text-2 hover:text-text',
            ].join(' ')}
          >
            {tb.label}
          </button>
        ))}
      </div>

      {tab === 'profile' ? (
        <ProfilePanel employeeId={employeeId} emp={emp} query={q} />
      ) : (
        <PayslipPanel />
      )}

      {editOpen && emp && (
        <UbahProfilModal
          emp={emp}
          open
          onOpenChange={setEditOpen}
          onChanged={() => void q.refetch()}
        />
      )}
    </AgentPage>
  );
}

// ---------------------------------------------------------------------------
// Profile panel
// ---------------------------------------------------------------------------

function ProfilePanel({
  emp,
  query,
}: {
  employeeId: string;
  emp: Employee | undefined;
  query: ReturnType<typeof useGetEmployee>;
}) {
  const { t } = useTranslation('agent');

  if (query.isLoading) {
    return <StateView kind="loading" title={t('loading')} />;
  }
  if (query.isError) {
    return (
      <StateView kind="error" title={t('errorGeneric')} onRetry={() => void query.refetch()} />
    );
  }

  const status = emp?.status;
  const isActive = status === 'ACTIVE';
  const statusLabel = isActive ? t('statusActive') : t('statusInactive');

  const hasPlacement = !!(emp?.current_position || emp?.current_client_company?.name);

  return (
    <div className="flex flex-col gap-4">
      {/* Header card */}
      <div className="rounded-xl border border-border bg-surface p-6">
        <div className="flex items-center gap-4">
          <Avatar
            size={64}
            shape="circle"
            initials={initialsOf(emp?.full_name)}
            src={emp?.photo_url ?? undefined}
          />
          <div className="flex min-w-0 flex-col gap-0.5">
            <span className="text-[18px] font-bold text-text">{emp?.full_name ?? '—'}</span>
            {emp?.nip ? <span className="font-mono text-[12px] text-text-3">{emp.nip}</span> : null}
          </div>
          {status ? (
            <div className="ml-auto">
              <StatusBadge dot tone={isActive ? 'ok' : 'neutral'}>
                {statusLabel}
              </StatusBadge>
            </div>
          ) : null}
        </div>
      </div>

      {/* Section grid — fills horizontal space first (2 cols on wide), stacks + scrolls when narrow */}
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        {/* Penempatan */}
        <Section title={t('sectionPlacement')}>
          {hasPlacement ? (
            <Grid>
              <ReadOnly label={t('profilePosition')} value={emp?.current_position ?? '—'} />
              <ReadOnly
                label={t('profileClientCompany')}
                value={emp?.current_client_company?.name ?? '—'}
              />
            </Grid>
          ) : (
            <p className="text-sm text-text-3">{t('profileNotPlaced')}</p>
          )}
        </Section>

        {/* Identitas */}
        <Section title={t('sectionIdentity')}>
          <Grid>
            <ReadOnly label={t('profileNik')} value={emp?.nik ?? '—'} />
            <ReadOnly label={t('profileGender')} value={genderLabel(t, emp?.gender)} />
            <ReadOnly label={t('profileBirthPlace')} value={emp?.birth_place ?? '—'} />
            <ReadOnly
              label={t('profileBirthDate')}
              value={emp?.birth_date ? formatDate(emp.birth_date) : '—'}
            />
          </Grid>
        </Section>

        {/* Kepegawaian */}
        <Section title={t('sectionEmployment')}>
          <Grid>
            <ReadOnly label={t('profileNip')} value={emp?.nip ?? '—'} />
            <ReadOnly
              label={t('profileJoinDate')}
              value={emp?.join_at ? formatDate(emp.join_at) : '—'}
            />
            <ReadOnly label={t('profileStatus')} value={status ? statusLabel : '—'} />
            <ReadOnly label={t('profileAppLanguage')} value={languageLabel(t, emp?.app_language)} />
          </Grid>
        </Section>

        {/* Kontak */}
        <Section title={t('sectionContact')}>
          <Grid>
            <ReadOnly label={t('profilePhone')} value={emp?.phone ?? '—'} />
            <ReadOnly label={t('profileEmail')} value={emp?.email_personal ?? '—'} />
            <ReadOnly
              className="sm:col-span-2"
              label={t('profileAddress')}
              value={emp?.address ?? '—'}
            />
            <ReadOnly
              label={t('profileEmergencyContact')}
              value={
                emp?.emergency_contact?.name
                  ? `${emp.emergency_contact.name}${emp.emergency_contact.phone ? ` · ${emp.emergency_contact.phone}` : ''}`
                  : '—'
              }
            />
          </Grid>
        </Section>

        {/* Keuangan & Pajak */}
        <Section title={t('sectionFinance')}>
          <Grid>
            <ReadOnly label={t('profileBank')} value={bankLabel(emp?.bank_account)} />
            <ReadOnly label={t('profileNpwp')} value={emp?.npwp ?? '—'} />
            <ReadOnly label={t('profileBpjsHealth')} value={emp?.bpjs_kesehatan ?? '—'} />
            <ReadOnly label={t('profileBpjsEmployment')} value={emp?.bpjs_ketenagakerjaan ?? '—'} />
          </Grid>
        </Section>
      </div>

      {/* Tier note */}
      <div className="flex items-center gap-2 rounded-lg border border-info-bd bg-info-bg px-3.5 py-3 text-[13px] font-medium text-info-tx">
        <Info className="size-4 shrink-0" aria-hidden />
        {t('profileTierNote')}
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-xl border border-border bg-surface p-6">
      <h2 className="mb-4 text-[15px] font-bold text-text">{title}</h2>
      {children}
    </section>
  );
}

function Grid({ children }: { children: React.ReactNode }) {
  return <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">{children}</div>;
}

function languageLabel(t: (k: string) => string, lang?: string): string {
  if (lang === 'en') return t('languageEn');
  if (lang === 'id') return t('languageId');
  return '—';
}

function genderLabel(t: (k: string) => string, gender?: string): string {
  if (gender === 'MALE') return t('genderMale');
  if (gender === 'FEMALE') return t('genderFemale');
  return '—';
}

function bankLabel(bank?: BankAccount): string {
  if (!bank?.account_number) return '—';
  const holder = bank.account_holder_name ? ` · ${bank.account_holder_name}` : '';
  return `${`${bank.bank_name ?? ''} ${bank.account_number}`.trim()}${holder}`;
}

function ReadOnly({
  label,
  value,
  className,
}: {
  label: string;
  value: string;
  className?: string;
}) {
  return (
    <div className={['flex flex-col gap-1', className].filter(Boolean).join(' ')}>
      <span className="text-xs font-medium text-text-3">{label}</span>
      <span className="text-sm font-semibold text-text">{value}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Payslip panel
// ---------------------------------------------------------------------------

function PayslipPanel() {
  const { t } = useTranslation('agent');
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  const query = useListPayslips({ limit: PAGE_SIZE, cursor });
  const body = query.data?.data as PayslipListResponse | undefined;
  const items: Payslip[] = body?.data ?? [];
  const hasMore = body?.has_more ?? false;

  const columns: Column<Payslip>[] = [
    {
      id: 'period',
      header: t('payslipPeriodCol', { defaultValue: 'Periode' }),
      width: 160,
      cell: (r) => <span className="font-medium text-text">{formatPeriod(r.period)}</span>,
    },
    {
      id: 'takeHome',
      header: t('payslipTakeHome'),
      width: 180,
      cell: (r) => (
        <span className="font-semibold tabular-nums text-text">{formatMoney(r.take_home_pay)}</span>
      ),
    },
    {
      id: 'gross',
      header: t('payslipGross'),
      width: 180,
      cell: (r) => (
        <span className="text-sm tabular-nums text-text-2">{formatMoney(r.gross_earnings)}</span>
      ),
    },
    {
      id: 'status',
      header: t('payslipStatusCol', { defaultValue: 'Status' }),
      width: 140,
      cell: (r) => (
        <StatusBadge dot tone={payslipStatusTone(r.status)}>
          {t(payslipStatusKey(r.status), { ns: 'payroll', defaultValue: r.status })}
        </StatusBadge>
      ),
    },
    {
      id: 'paidOn',
      header: t('payslipPaidOnCol', { defaultValue: 'Tanggal Bayar' }),
      width: 140,
      cell: (r) =>
        r.paid_on ? (
          <DateText kind="date" value={r.paid_on} className="text-sm text-text-2" />
        ) : (
          <span className="text-sm italic text-text-3">—</span>
        ),
    },
  ];

  if (query.isError) {
    return (
      <StateView
        kind="error"
        title={t('errorGeneric')}
        onRetry={() => void query.refetch()}
        retryLabel={t('retry')}
      />
    );
  }

  return (
    <DataTable
      aria-label={t('payslipTitle')}
      columns={columns}
      data={items}
      getRowId={(r) => r.id}
      isLoading={query.isLoading}
      skeletonRows={8}
      empty={<EmptyState variant="fresh" title={t('payslipEmpty')} />}
      footer={
        items.length > 0 && (hasMore || prevCursors.length > 0) ? (
          <CursorPagination
            rangeLabel={t('resultRange', { defaultValue: '{{count}} entri', count: items.length })}
            hasPrev={prevCursors.length > 0}
            hasNext={hasMore}
            prevLabel={t('prev', { defaultValue: 'Sebelumnya' })}
            nextLabel={t('next', { defaultValue: 'Berikutnya' })}
            onPrev={() => {
              const next = [...prevCursors];
              const prev = next.pop();
              setPrevCursors(next);
              setCursor(prev || undefined);
            }}
            onNext={() => {
              const nextCursor = body?.next_cursor;
              if (!nextCursor) return;
              setPrevCursors((prev) => [...prev, cursor ?? '']);
              setCursor(nextCursor);
            }}
          />
        ) : undefined
      }
    />
  );
}

// ---------------------------------------------------------------------------
// Tiered "Ubah Profil" modal
// ---------------------------------------------------------------------------

interface UbahProfilModalProps {
  emp: Employee;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onChanged?: () => void;
}

function UbahProfilModal({ emp, open, onOpenChange, onChanged }: UbahProfilModalProps) {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const updateProfile = useUpdateMyProfile();
  const initUpload = useInitProfilePhotoUpload();
  const fileInput = useRef<HTMLInputElement>(null);

  // All fields are now instant (single PATCH /me/profile).
  const [address, setAddress] = useState(emp.address ?? '');
  const [language, setLanguage] = useState<string>(emp.app_language ?? 'id');
  const [photoFile, setPhotoFile] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [phone, setPhone] = useState(emp.phone ?? '');
  const [phoneError, setPhoneError] = useState('');
  const [ecName, setEcName] = useState(emp.emergency_contact?.name ?? '');
  const [ecPhone, setEcPhone] = useState(emp.emergency_contact?.phone ?? '');
  const [bankName, setBankName] = useState(emp.bank_account?.bank_name ?? '');
  const [bankNumber, setBankNumber] = useState(emp.bank_account?.account_number ?? '');
  const [bankHolder, setBankHolder] = useState(emp.bank_account?.account_holder_name ?? '');

  const busy = updateProfile.isPending || initUpload.isPending;

  function onPickPhoto(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0] ?? null;
    if (!file) return;
    if (!contentTypeForFile(file)) {
      toast({ tone: 'error', title: t('profilePhotoBadType') });
      return;
    }
    setPhotoFile(file);
    setPhotoPreview(URL.createObjectURL(file));
  }

  // Upload photo to presigned PUT, returns the object_key to apply via PATCH.
  async function uploadPhoto(file: File): Promise<string | null> {
    const contentType = contentTypeForFile(file);
    if (!contentType) {
      toast({ tone: 'error', title: t('profilePhotoBadType') });
      return null;
    }
    const res = await initUpload.mutateAsync({
      data: { content_type: contentType, content_length: file.size },
    });
    const ticket = res.data as UploadTicket;
    if (file.size > ticket.max_bytes) {
      toast({ tone: 'error', title: t('profilePhotoTooBig') });
      return null;
    }
    const put = await fetch(ticket.upload_url, {
      method: 'PUT',
      headers: { 'Content-Type': contentType },
      body: file,
    });
    if (!put.ok) {
      toast({ tone: 'error', title: t('profilePhotoError') });
      return null;
    }
    return ticket.object_key;
  }

  async function onSave() {
    setPhoneError('');

    // Build a single instant payload (PATCH /me/profile) from every changed field.
    const patch: SelfProfileUpdate = {};
    if (address !== (emp.address ?? '')) patch.address = address;
    if (language !== (emp.app_language ?? 'id')) patch.app_language = language as 'id' | 'en';
    if (phone !== (emp.phone ?? '')) patch.phone = phone;
    if (
      ecName !== (emp.emergency_contact?.name ?? '') ||
      ecPhone !== (emp.emergency_contact?.phone ?? '')
    ) {
      patch.emergency_contact = { name: ecName, phone: ecPhone };
    }
    if (
      bankName !== (emp.bank_account?.bank_name ?? '') ||
      bankNumber !== (emp.bank_account?.account_number ?? '') ||
      bankHolder !== (emp.bank_account?.account_holder_name ?? '')
    ) {
      patch.bank_account = {
        bank_name: bankName,
        account_number: bankNumber,
        account_holder_name: bankHolder,
      };
    }

    try {
      if (photoFile) {
        const key = await uploadPhoto(photoFile);
        if (key === null) return; // upload failed — abort the whole save
        patch.photo_object_key = key;
      }
    } catch {
      toast({ tone: 'error', title: t('profileError') });
      return;
    }

    if (Object.keys(patch).length === 0) {
      toast({ tone: 'info', title: t('profileNoChange') });
      return;
    }

    try {
      await updateProfile.mutateAsync({ data: patch });
    } catch (err) {
      // Phone is a unique login identifier (E2 D2): a 409 surfaces inline on the field.
      const code = err instanceof ApiError ? err.code : '';
      const conflict =
        err instanceof ApiError && (err.status === 409 || code.toUpperCase().includes('PHONE'));
      if (conflict && patch.phone !== undefined) {
        setPhoneError(t('profilePhoneTaken'));
        return;
      }
      toast({ tone: 'error', title: t('profileError') });
      return;
    }

    toast({ tone: 'success', title: t('profileSuccess') });
    onOpenChange(false);
    onChanged?.();
  }

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg" className="w-[600px]">
      <ModalHeader
        icon={Pencil}
        tone="brand"
        title={t('profileEditBtn')}
        closeLabel={t('cancel')}
      />

      <ModalBody className="gap-5">
        <section className="flex flex-col gap-4">
          {/* Photo */}
          <FormField label={t('profilePhoto')} htmlFor="profile-photo-btn">
            <div className="flex items-center gap-4">
              <Avatar
                size={56}
                shape="circle"
                initials={initialsOf(emp.full_name)}
                src={photoPreview ?? emp.photo_url ?? undefined}
              />
              <input
                ref={fileInput}
                type="file"
                accept="image/jpeg,image/png,image/webp"
                className="hidden"
                onChange={onPickPhoto}
              />
              <Button
                id="profile-photo-btn"
                variant="secondary"
                size="sm"
                disabled={busy}
                onClick={() => fileInput.current?.click()}
              >
                <ImagePlus className="size-4" aria-hidden />
                {t('profilePhotoChange')}
              </Button>
            </div>
          </FormField>

          {/* Address */}
          <FormField label={t('profileAddress')} htmlFor="profile-address">
            <textarea
              id="profile-address"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              rows={3}
              className="w-full resize-y rounded-md border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
            />
          </FormField>

          {/* App language */}
          <FormField label={t('profileAppLanguage')} htmlFor="profile-language">
            <FilterSelect
              id="profile-language"
              containerClassName="w-full"
              value={language}
              onChange={(e) => setLanguage(e.target.value)}
            >
              <option value="id">{t('languageId')}</option>
              <option value="en">{t('languageEn')}</option>
            </FilterSelect>
          </FormField>

          <FormField label={t('profileName')} htmlFor="profile-name">
            <Input id="profile-name" value={emp.full_name ?? '—'} disabled />
          </FormField>

          <FormField label={t('profilePhone')} htmlFor="profile-phone" error={phoneError}>
            <Input
              id="profile-phone"
              type="tel"
              value={phone}
              onChange={(e) => {
                setPhone(e.target.value);
                if (phoneError) setPhoneError('');
              }}
            />
          </FormField>

          <div className="grid grid-cols-2 gap-4">
            <FormField label={t('profileEmergencyName')} htmlFor="ec-name">
              <Input id="ec-name" value={ecName} onChange={(e) => setEcName(e.target.value)} />
            </FormField>
            <FormField label={t('profileEmergencyPhone')} htmlFor="ec-phone">
              <Input
                id="ec-phone"
                type="tel"
                value={ecPhone}
                onChange={(e) => setEcPhone(e.target.value)}
              />
            </FormField>
          </div>

          <div className="grid grid-cols-3 gap-4">
            <FormField label={t('profileBankName')} htmlFor="bank-name">
              <Input
                id="bank-name"
                value={bankName}
                onChange={(e) => setBankName(e.target.value)}
              />
            </FormField>
            <FormField label={t('profileBankNumber')} htmlFor="bank-number">
              <Input
                id="bank-number"
                value={bankNumber}
                onChange={(e) => setBankNumber(e.target.value)}
              />
            </FormField>
            <FormField label={t('profileBankHolder')} htmlFor="bank-holder">
              <Input
                id="bank-holder"
                value={bankHolder}
                onChange={(e) => setBankHolder(e.target.value)}
              />
            </FormField>
          </div>
        </section>
      </ModalBody>

      <ModalFooter>
        <Button variant="secondary" size="sm" disabled={busy} onClick={() => onOpenChange(false)}>
          {t('cancel')}
        </Button>
        <Button variant="primary" size="sm" disabled={busy} onClick={() => void onSave()}>
          {t('profileSave')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
