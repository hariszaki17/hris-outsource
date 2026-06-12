/**
 * E2 · Karyawan — Detail
 *
 * .pen frames:
 *   JBjBb  HR/Admin detail — full edit actions
 *   rtKzk  SL read-only detail
 *
 * Design: BackRow → HeaderCard (avatar, name, NIP/NIK, status badges, edit btn, kebab) →
 * subrow (position · company) → Tabs (Profil active | Penempatan E3→ | Kehadiran
 * E5→ | Cuti & Lembur E6/7→) → Profil tab: LeftCol (Data Pribadi, Kontak, Statutori & Bank) +
 * RightCol (Akun Login, Ringkasan).
 *
 * Cross-epic tabs show placeholder — no dead-flow (ENGINEERING.md B2).
 * SL variant hides edit actions (RBAC defense-in-depth, A2).
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { type Employee, EmployeeStatus, useGetEmployee } from '@swp/api-client/e2';
import { Avatar, DateText, EmptyState, StateView, StatusBadge } from '@swp/ui';
import { useNavigate, useParams } from '@tanstack/react-router';
import { ArrowLeft, KeyRound, MoreVertical, Pencil, Smartphone } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { EditEmployeeScreen } from './employee-form.tsx';
import { OffboardEmployeeConfirm, ReactivateEmployeeConfirm } from './employee-overlays.tsx';

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

// ---------------------------------------------------------------------------
// KV row component (used in detail cards)
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
    <div className="flex items-center justify-between gap-4 border-b border-border-soft py-[11px] last:border-b-0">
      <span className="text-[13px] text-text-3">{label}</span>
      {children ?? (
        <span className={['text-[13px] font-medium text-text', mono ? 'font-mono' : ''].join(' ')}>
          {value ?? '—'}
        </span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// DetailCard component
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
      <div className="flex items-center justify-between border-b border-border-soft px-[18px] py-[13px]">
        <div className="flex items-center gap-2">
          {titleIcon}
          <span className="text-[14px] font-bold text-text">{title}</span>
        </div>
        {action}
      </div>
      <div className="px-[18px] py-[4px] pb-[8px]">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Cross-epic tab placeholder
// ---------------------------------------------------------------------------

function CrossEpicPlaceholder({ tabName, epic }: { tabName: string; epic: string }) {
  const { t } = useTranslation('employees');
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
      <p className="text-[14px] font-medium text-text-2">{t('crossEpicTitle', { tab: tabName })}</p>
      <p className="max-w-[360px] text-[13px] text-text-3">{t('crossEpicBody', { epic })}</p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Inline mini-tab switch
// ---------------------------------------------------------------------------

type DetailTab = 'profil' | 'penempatan' | 'kehadiran' | 'cuti-lembur';

function DetailTabs({
  active,
  onChange,
}: {
  active: DetailTab;
  onChange: (t: DetailTab) => void;
}) {
  const { t } = useTranslation('employees');

  const tabs: { id: DetailTab; label: string; cross?: boolean }[] = [
    { id: 'profil', label: t('tabDetailProfil') },
    { id: 'penempatan', label: t('tabDetailPenempatan'), cross: true },
    { id: 'kehadiran', label: t('tabDetailKehadiran'), cross: true },
    { id: 'cuti-lembur', label: t('tabDetailCutiLembur'), cross: true },
  ];

  return (
    <div className="flex items-center gap-[28px] border-b border-border">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          onClick={() => onChange(tab.id)}
          className={[
            'flex items-center gap-[6px] pb-[12px] pt-[12px] text-[14px]',
            active === tab.id
              ? 'border-b-2 border-primary font-semibold text-primary'
              : 'border-b-2 border-transparent font-medium text-text-2',
          ].join(' ')}
        >
          {tab.label}
          {tab.cross && <span className="text-[11px] text-text-3">E→</span>}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// EmployeeDetailScreen
// ---------------------------------------------------------------------------

export function EmployeeDetailScreen() {
  const { t } = useTranslation('employees');
  const navigate = useNavigate();
  const { employeeId } = useParams({ from: '/authed/employees/$employeeId' as const });
  const currentUser = useCurrentUser();
  const isShiftLeader = currentUser?.role === 'shift_leader';

  const [activeTab, setActiveTab] = useState<DetailTab>('profil');
  const [showEdit, setShowEdit] = useState(false);
  const [showOffboard, setShowOffboard] = useState(false);
  const [showReactivate, setShowReactivate] = useState(false);

  const query = useGetEmployee(employeeId);

  // ---------------------------------------------------------------------------
  // Loading / error states
  // ---------------------------------------------------------------------------

  if (query.isLoading) {
    return (
      <div className="flex flex-col gap-4 animate-pulse">
        <div className="h-6 w-40 rounded bg-surface-2" />
        <div className="h-24 rounded-xl bg-surface-2" />
      </div>
    );
  }

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'not-found') {
      return (
        <EmptyState
          variant="no-permission"
          title={t('notFoundTitle')}
          description={t('notFoundBody')}
        />
      );
    }
    // A 403 is reachable WITH valid capability: a shift_leader (employees.read) opening an
    // employee outside their server-side company scope. Show a no-permission state, no Retry.
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <EmptyState
          variant="no-permission"
          title={t('noPermissionTitle')}
          description={t('noPermissionBody')}
        />
      );
    }
    return (
      <StateView
        kind="error"
        title={t('errorTitle')}
        description={t('errorBody')}
        onRetry={() => query.refetch()}
        retryLabel={t('common.retry', { ns: 'translation' })}
      />
    );
  }

  const emp = query.data?.data as Employee | undefined;
  if (!emp) return null;

  const isActive = emp.status === EmployeeStatus.ACTIVE;

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <>
      <div className="flex flex-col gap-4">
        {/* Back row */}
        <button
          type="button"
          className="flex items-center gap-[7px] text-[13px] font-medium text-text-2 hover:text-text"
          onClick={() => void navigate({ to: '/employees' as const })}
        >
          <ArrowLeft className="size-4" aria-hidden />
          {t('backToList')}
        </button>

        {/* Header card */}
        <div className="flex items-center justify-between rounded-xl border border-border bg-surface p-5">
          <div className="flex items-center gap-4">
            <Avatar initials={initials(emp.full_name)} size={64} className="rounded-xl" />
            <div className="flex flex-col gap-[6px]">
              <div className="flex items-center gap-[10px]">
                <span className="text-[20px] font-bold text-text">{emp.full_name}</span>
                <StatusBadge dot tone={isActive ? 'ok' : 'bad'}>
                  {isActive ? t('statusActive') : t('statusInactive')}
                </StatusBadge>
                {/* Every employee auto-provisions a login (D1). */}
                <StatusBadge dot tone="info">
                  {t('loginActive')}
                </StatusBadge>
              </div>
              <span className="font-mono text-[12px] text-text-3">
                NIK {emp.nik}
                {emp.nip ? ` · NIP ${emp.nip}` : ''}
              </span>
            </div>
          </div>

          {!isShiftLeader && (
            <div className="flex items-center gap-[10px]">
              <button
                type="button"
                className="flex items-center gap-2 rounded-lg border border-border bg-surface px-[14px] py-[9px] text-[13px] font-semibold text-text-2 hover:bg-surface-2"
                onClick={() => setShowEdit(true)}
              >
                <Pencil className="size-[15px]" aria-hidden />
                {t('editProfile')}
              </button>
              <button
                type="button"
                aria-label={t('moreActions')}
                className="flex size-[38px] items-center justify-center rounded-lg border border-border bg-surface text-text-2 hover:bg-surface-2"
                onClick={() => (isActive ? setShowOffboard(true) : setShowReactivate(true))}
              >
                <MoreVertical className="size-[18px]" aria-hidden />
              </button>
            </div>
          )}
        </div>

        {/* Sub row: position · company */}
        <div className="flex items-center gap-2 px-1 text-[13px]">
          {emp.current_position && (
            <span className="font-medium text-text-2">{emp.current_position}</span>
          )}
          {emp.current_position && emp.current_client_company && (
            <span className="size-[4px] rounded-full bg-text-3" aria-hidden />
          )}
          {emp.current_client_company && (
            <span className="text-text-2">{emp.current_client_company.name}</span>
          )}
        </div>

        {/* Tabs */}
        <DetailTabs active={activeTab} onChange={setActiveTab} />

        {/* Tab content */}
        {activeTab === 'profil' && (
          <div className="flex gap-4">
            {/* Left column */}
            <div className="flex flex-1 flex-col gap-4">
              {/* Data Pribadi */}
              <DetailCard title={t('secDataPribadi')}>
                <KvRow
                  label={t('fieldGender')}
                  value={
                    emp.gender === 'MALE'
                      ? t('genderMale')
                      : emp.gender === 'FEMALE'
                        ? t('genderFemale')
                        : undefined
                  }
                />
                <KvRow
                  label={t('fieldBirthDate')}
                  value={
                    emp.birth_place && emp.birth_date
                      ? `${emp.birth_place}, ${emp.birth_date}`
                      : (emp.birth_date ?? undefined)
                  }
                />
                <KvRow label={t('fieldJoinAt')} value={emp.join_at} />
                <KvRow label={t('fieldTenure')}>
                  <span className="text-[13px] font-medium text-text">
                    {emp.join_at ? <DateText kind="date" value={emp.join_at} /> : '—'}
                  </span>
                </KvRow>
              </DetailCard>

              {/* Kontak */}
              <DetailCard title={t('secKontak')}>
                <KvRow label={t('fieldPhone')} value={emp.phone} mono />
                <KvRow label={t('fieldEmailPersonal')} value={emp.email_personal} />
                <KvRow label={t('fieldAddress')} value={emp.address} />
              </DetailCard>

              {/* Statutori & Bank */}
              {!isShiftLeader && (
                <DetailCard title={t('secStatutori')}>
                  <KvRow label="NIK" value={emp.nik} mono />
                  <KvRow label="NPWP" value={emp.npwp} mono />
                  <KvRow label="BPJS Kesehatan" value={emp.bpjs_kesehatan} mono />
                  <KvRow label="BPJS Ketenagakerjaan" value={emp.bpjs_ketenagakerjaan} mono />
                  {emp.bank_account && (
                    <KvRow
                      label={t('fieldBankAccount')}
                      value={
                        emp.bank_account.bank_name && emp.bank_account.account_number
                          ? `${emp.bank_account.bank_name} · ${emp.bank_account.account_number}`
                          : emp.bank_account.account_number
                      }
                    />
                  )}
                </DetailCard>
              )}
            </div>

            {/* Right column */}
            <div className="flex w-[368px] shrink-0 flex-col gap-4">
              {/* Akun Login */}
              {!isShiftLeader && (
                <DetailCard
                  title={t('secAkunLogin')}
                  titleIcon={<KeyRound className="size-4 text-primary" aria-hidden />}
                >
                  {/* Login auto-provisions on create (D1); the identifier is the phone (D2). */}
                  <KvRow label={t('fieldLoginIdentifier')} value={emp.phone} mono />
                  <KvRow label={t('fieldRole')} value={t('roleAgent')} />
                  <KvRow label={t('fieldLoginStatus')}>
                    <StatusBadge dot tone="ok">
                      {t('statusActive')}
                    </StatusBadge>
                  </KvRow>
                </DetailCard>
              )}

              {/* Ringkasan penempatan */}
              <DetailCard title={t('secRingkasan')}>
                <KvRow label={t('fieldPosition')} value={emp.current_position} />
                <KvRow label={t('fieldClientCompany')} value={emp.current_client_company?.name} />
              </DetailCard>
            </div>
          </div>
        )}

        {activeTab === 'penempatan' && (
          <CrossEpicPlaceholder tabName={t('tabDetailPenempatan')} epic="E3" />
        )}
        {activeTab === 'kehadiran' && (
          <CrossEpicPlaceholder tabName={t('tabDetailKehadiran')} epic="E5" />
        )}
        {activeTab === 'cuti-lembur' && (
          <CrossEpicPlaceholder tabName={t('tabDetailCutiLembur')} epic="E6/E7" />
        )}

        {/* Role note */}
        <div className="flex items-center gap-[9px] rounded-lg border border-l-[3px] border-border bg-surface px-[14px] py-[10px]">
          <Smartphone className="size-[15px] shrink-0 text-text-3" aria-hidden />
          <p className="text-[12px] text-text-2">
            {isShiftLeader ? t('roleNoteDetailSL') : t('roleNoteDetailHR')}
          </p>
        </div>
      </div>

      {/* Edit overlay */}
      {!isShiftLeader && (
        <EditEmployeeScreen
          employeeId={employeeId}
          open={showEdit}
          onOpenChange={setShowEdit}
          onDone={() => {
            setShowEdit(false);
            void query.refetch();
          }}
        />
      )}

      {/* Offboard confirm (F2.7 — employment-end + session revocation) */}
      <OffboardEmployeeConfirm
        open={showOffboard}
        onOpenChange={setShowOffboard}
        employee={emp}
        onDone={() => {
          setShowOffboard(false);
          void query.refetch();
        }}
      />

      {/* Reactivate confirm */}
      <ReactivateEmployeeConfirm
        open={showReactivate}
        onOpenChange={setShowReactivate}
        employee={emp}
        onDone={() => {
          setShowReactivate(false);
          void query.refetch();
        }}
      />
    </>
  );
}
