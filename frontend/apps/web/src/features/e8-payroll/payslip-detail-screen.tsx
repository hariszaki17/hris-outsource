/**
 * E8 · Detail Slip Gaji (HR) — payslip detail + decrypt-fail variant.
 *
 * .pen frames implemented:
 *   JaScP   E8 · Detail Slip Gaji (HR)              — normal FINAL state
 *   q8JxjZ  E8 · Detail Slip Gaji (HR) · Decrypt-fail variant — same component, decrypt_fail=true
 *
 * Both states are driven by the same component. The decrypt-fail variant activates when
 * `payslip.decrypt_fail === true` (equivalently `payslip.status === 'DECRYPT_FAIL'`).
 *
 * RBAC: HR admin / Super admin only (PA-2 / INV-4). Shift leader + agent → no-permission gate.
 *
 * Routes (proposed):
 *   /payroll/$payslipId  → PayslipDetailScreen (takes payslipId param)
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type BenefitLine,
  type DeductionLine,
  type EarningLine,
  type Payslip,
  type PayslipAuditNote,
  type PayslipAuditNoteListResponse,
  PayslipStatus,
  useGetPayslip,
  useListPayslipAuditNotes,
} from '@swp/api-client/e8';
import { Banner, Button, DateText, EmptyState, StateView, StatusBadge } from '@swp/ui';
import { ArrowLeft, Lock, TriangleAlert } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  formatMoney,
  formatPeriod,
  payslipStatusKey,
  payslipStatusTone,
} from './payroll-shared.tsx';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface PayslipDetailScreenProps {
  /** `SWP-PS-{n}` id string from the route param. */
  payslipId: string;
  /** Called when the user clicks the back button. */
  onBack?: () => void;
  /** Called when the user clicks "Tambah Catatan" — opens the append-only HR audit-note drawer. */
  onAddNote?: () => void;
}

// ---------------------------------------------------------------------------
// PayslipDetailScreen
// ---------------------------------------------------------------------------

export function PayslipDetailScreen({ payslipId, onBack, onAddNote }: PayslipDetailScreenProps) {
  const { t } = useTranslation('payroll');
  const user = useCurrentUser();

  // RBAC gate
  if (user?.role === 'shift_leader' || user?.role === 'agent') {
    return (
      <EmptyState
        variant="no-permission"
        title={t('common.noPermission')}
        description={t('common.noPermissionBody')}
      />
    );
  }

  return <PayslipDetailInner payslipId={payslipId} onBack={onBack} onAddNote={onAddNote} />;
}

// ---------------------------------------------------------------------------
// Inner component (RBAC cleared)
// ---------------------------------------------------------------------------

function PayslipDetailInner({ payslipId, onBack, onAddNote }: PayslipDetailScreenProps) {
  const { t } = useTranslation('payroll');

  const query = useGetPayslip(payslipId);
  const notesQuery = useListPayslipAuditNotes(payslipId);

  // -------------------------------------------------------------------------
  // Loading skeleton
  // -------------------------------------------------------------------------

  if (query.isLoading) {
    return (
      <div className="flex flex-col gap-4 animate-pulse">
        <div className="h-8 w-40 rounded bg-surface-2" />
        <div className="h-[88px] w-full rounded-xl bg-surface-2" />
        <div className="grid grid-cols-[1fr_392px] gap-5">
          <div className="flex flex-col gap-4">
            <div className="h-[220px] rounded-xl bg-surface-2" />
            <div className="h-[160px] rounded-xl bg-surface-2" />
          </div>
          <div className="flex flex-col gap-4">
            <div className="h-[180px] rounded-xl bg-surface-2" />
            <div className="h-[140px] rounded-xl bg-surface-2" />
          </div>
        </div>
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // Error state
  // -------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <EmptyState
          variant="no-permission"
          title={t('common.noPermission')}
          description={t('common.noPermissionBody')}
        />
      );
    }
    if (kind === 'not-found') {
      return (
        <div className="flex flex-col gap-4">
          <BackRow onBack={onBack} />
          <div className="rounded-xl border border-border bg-surface px-5 py-4">
            <p className="text-sm text-text-2">{t('detail.notFound')}</p>
          </div>
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-4">
        <BackRow onBack={onBack} />
        <StateView
          kind="error"
          title={t('errors.loadError')}
          onRetry={() => query.refetch()}
          retryLabel={t('common.retry')}
        />
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // Data
  // -------------------------------------------------------------------------

  // Unwrap: the BE wraps the detail in a `{data: Payslip}` envelope (like every epic),
  // but the E8 openapi declares getPayslip 200 as a BARE `Payslip`, so Orval narrows
  // `query.data.data` to `Payslip`. The actual body is therefore one level deeper. Unwrap
  // the inner `{data}` when present, else fall back to the bare object — matches the
  // Phase-8 leave-detail / Phase-9 overtime-detail {data}-envelope precedent ([08-04]).
  const rawPayslip = query.data?.data as Payslip | { data?: Payslip } | undefined;
  const payslip = (
    rawPayslip && typeof rawPayslip === 'object' && 'data' in rawPayslip && rawPayslip.data
      ? rawPayslip.data
      : rawPayslip
  ) as Payslip | undefined;

  if (!payslip) {
    return (
      <div className="flex flex-col gap-4">
        <BackRow onBack={onBack} />
        <StateView kind="empty" title={t('detail.notFound')} />
      </div>
    );
  }

  const isDecryptFail = payslip.decrypt_fail || payslip.status === PayslipStatus.DECRYPT_FAIL;

  const notes = (notesQuery.data?.data as PayslipAuditNoteListResponse | undefined)?.data ?? [];

  // -------------------------------------------------------------------------
  // Render
  // -------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-4">
      {/* Back row */}
      <BackRow onBack={onBack} />

      {/* Decrypt-fail banner — frame `mNs7a` / `DecryptFailBanner` */}
      {isDecryptFail && (
        <Banner
          tone="bad"
          title={t('detail.decryptFailTitle')}
          description={t('detail.decryptFailDesc')}
          icon={TriangleAlert}
        />
      )}

      {/* Header card — frame `O1gxm` / `c7zEc6` */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface p-5">
        {/* Left: avatar + employee name + meta */}
        <div className="flex items-center gap-3.5">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-full bg-primary-soft text-base font-bold text-primary">
            {avatarInitials(payslip.employee_name ?? payslip.employee_id)}
          </div>
          <div className="flex flex-col gap-0.5">
            <span className="text-base font-semibold text-text">
              {payslip.employee_name ?? payslip.employee_id}
            </span>
            <div className="flex items-center gap-2 text-[12px] text-text-3">
              <span className="font-mono">{payslip.employee_id}</span>
              <span>·</span>
              <span>{formatPeriod(payslip.period)}</span>
              {payslip.working_days != null && (
                <>
                  <span>·</span>
                  <span>
                    {payslip.working_days} {t('detail.workingDaysUnit')}
                  </span>
                </>
              )}
            </div>
          </div>
        </div>

        {/* Right: FINAL · Read-only pill + status badge */}
        <div className="flex items-center gap-3.5">
          {/* "FINAL · Read-only" pill — frame `RNyXt` / `TU6iF` using $bad-bg as per design */}
          <div className="flex items-center gap-1.5 rounded-full border border-bad-bd bg-bad-bg px-[11px] py-[5px]">
            <Lock aria-hidden className="size-3 text-bad-tx" />
            <span className="text-[11px] font-semibold text-bad-tx">
              {isDecryptFail ? t('detail.pillDecryptFail') : t('detail.pillReadOnly')}
            </span>
          </div>
          <div className="flex flex-col items-end gap-1">
            <StatusBadge dot tone={payslipStatusTone(payslip.status)}>
              {t(payslipStatusKey(payslip.status))}
            </StatusBadge>
            {payslip.paid_on && (
              <span className="text-[11px] text-text-3">
                {t('detail.paidOn')}{' '}
                <DateText kind="date" value={payslip.paid_on} className="inline text-[11px]" />
              </span>
            )}
          </div>
        </div>
      </div>

      {/* Two-column layout matching frame `Iw15a` / `k9BcWR` */}
      <div className="grid grid-cols-[1fr_392px] gap-5">
        {/* Left column */}
        <div className="flex flex-col gap-4">
          {/* Earnings — frame `GfCef` / `dKCZ3` */}
          <SectionCard title={t('detail.earningsTitle')}>
            {isDecryptFail || !payslip.earnings?.length ? (
              <DecryptPlaceholder isDecryptFail={isDecryptFail} t={t} />
            ) : (
              <>
                {payslip.earnings.map((line: EarningLine) => (
                  <LineRow key={line.name} name={line.name} value={line.value} />
                ))}
              </>
            )}
          </SectionCard>

          {/* Deductions — frame `YUlmb` / `x3xz3B` */}
          <SectionCard title={t('detail.deductionsTitle')}>
            {isDecryptFail || !payslip.deductions?.length ? (
              <DecryptPlaceholder isDecryptFail={isDecryptFail} t={t} />
            ) : (
              <>
                {payslip.deductions.map((line: DeductionLine) => (
                  <LineRow key={line.name} name={line.name} value={line.value} />
                ))}
              </>
            )}
          </SectionCard>

          {/* Take-home card — frame `F7bXVy` (primary-soft) / `P8MLV` (surface-2 on decrypt-fail) */}
          <div
            className={[
              'flex items-center justify-between rounded-xl border px-[18px] py-[14px]',
              isDecryptFail ? 'border-border bg-surface-2' : 'border-primary bg-primary-soft',
            ].join(' ')}
          >
            <span
              className={[
                'text-sm font-semibold',
                isDecryptFail ? 'text-text-3' : 'text-primary',
              ].join(' ')}
            >
              {t('detail.takeHomeLabel')}
            </span>
            <span
              className={[
                'text-lg font-bold tabular-nums',
                isDecryptFail ? 'text-text-3' : 'text-primary',
              ].join(' ')}
            >
              {formatMoney(payslip.take_home_pay)}
            </span>
          </div>
        </div>

        {/* Right column */}
        <div className="flex flex-col gap-4">
          {/* Benefits — frame `q4ifcm` / `nj42G` */}
          <SectionCard title={t('detail.benefitsTitle')}>
            {isDecryptFail || !payslip.benefits?.length ? (
              <DecryptPlaceholder isDecryptFail={isDecryptFail} t={t} />
            ) : (
              <>
                {payslip.benefits.map((line: BenefitLine) => (
                  <LineRow key={line.name} name={line.name} value={line.value} />
                ))}
              </>
            )}
          </SectionCard>

          {/* Info / summary card — frame `VY1AT` / `xOSXE` */}
          <SectionCard title={t('detail.infoTitle')}>
            <dl className="flex flex-col gap-2">
              <InfoRow label={t('detail.infoId')} value={payslip.id} mono />
              <InfoRow label={t('detail.infoPeriod')} value={formatPeriod(payslip.period)} />
              <InfoRow
                label={t('detail.infoGrossEarnings')}
                value={formatMoney(payslip.gross_earnings)}
                mono
              />
              <InfoRow
                label={t('detail.infoGrossDeductions')}
                value={formatMoney(payslip.gross_deductions)}
                mono
              />
              <InfoRow
                label={t('detail.infoSource')}
                value={`${payslip.source.system} #${payslip.source.source_id}`}
                mono
              />
            </dl>
          </SectionCard>

          {/* Audit notes — frame `oes5m` / `a1irjk` */}
          <SectionCard title={t('detail.auditNotesTitle')}>
            {onAddNote ? (
              <Button type="button" variant="secondary" size="sm" onClick={onAddNote}>
                {t('auditNotes.addSectionTitle')}
              </Button>
            ) : null}
            {notesQuery.isLoading ? (
              <div className="h-10 animate-pulse rounded bg-surface-2" />
            ) : notes.length === 0 ? (
              <p className="text-[12px] text-text-3 italic">{t('detail.auditNotesEmpty')}</p>
            ) : (
              notes.map((note: PayslipAuditNote) => (
                <div
                  key={note.id}
                  className="flex flex-col gap-0.5 border-b border-border-soft pb-2 last:border-0 last:pb-0"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-[12px] font-medium text-text">
                      {note.author_name ?? note.author_id}
                    </span>
                    <DateText
                      kind="instant"
                      value={note.created_at}
                      className="text-[11px] text-text-3"
                    />
                  </div>
                  <p className="text-[12px] text-text-2">{note.text}</p>
                </div>
              ))
            )}
          </SectionCard>

          {/* Security note — frame `iqEjt` / `v1BrD` ($bad-bg, annotation) */}
          <div className="flex items-start gap-2 rounded-lg border border-bad-bd bg-bad-bg px-3 py-2.5">
            <Lock aria-hidden className="mt-0.5 size-3.5 shrink-0 text-bad-tx" />
            <p className="text-[11px] text-bad-tx">{t('detail.securityNote')}</p>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function BackRow({ onBack }: { onBack?: () => void }) {
  const { t } = useTranslation('payroll');
  return (
    <button
      type="button"
      aria-label={t('common.back')}
      className="flex items-center gap-2 text-[13px] font-medium text-text-2 hover:text-text"
      onClick={onBack}
    >
      <ArrowLeft aria-hidden className="size-4" />
      {t('common.back')}
    </button>
  );
}

function SectionCard({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2.5 rounded-xl border border-border bg-surface p-[18px]">
      <h2 className="text-[13px] font-semibold text-text">{title}</h2>
      {children}
    </div>
  );
}

function LineRow({ name, value }: { name: string; value: string | null | undefined }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span className="text-[13px] text-text-2">{name}</span>
      <span className="text-[13px] font-medium tabular-nums text-text">{formatMoney(value)}</span>
    </div>
  );
}

function InfoRow({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <dt className="text-[11px] text-text-3 shrink-0">{label}</dt>
      <dd
        className={['text-[11px] text-text text-right truncate', mono ? 'font-mono' : ''].join(' ')}
      >
        {value}
      </dd>
    </div>
  );
}

function DecryptPlaceholder({
  isDecryptFail,
  t,
}: {
  isDecryptFail: boolean;
  t: (key: string) => string;
}) {
  if (isDecryptFail) {
    return <p className="text-[12px] text-text-3 italic">{t('detail.decryptPlaceholder')}</p>;
  }
  return <p className="text-[12px] text-text-3 italic">{t('detail.emptyLines')}</p>;
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

function avatarInitials(name: string): string {
  return name
    .split(' ')
    .slice(0, 2)
    .map((p) => p[0] ?? '')
    .join('')
    .toUpperCase();
}
