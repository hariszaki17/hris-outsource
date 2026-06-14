/**
 * E5 · Detail Verifikasi (HR + Shift Leader scoped)
 *
 * .pen frames:
 *   VY894  HR detail — Screen 3 · Detail Verifikasi
 *   RZPQz  SL detail — E5 SL · Detail Verifikasi — Plaza Senayan
 *
 * Design: BackRow → HeaderCard (employee info + status) → two-column layout:
 *   LeftCol: clock-in/out times, photos, geo coordinates, flags.
 *   RightCol: verification history, verify/reject actions.
 *
 * F5.3: single verify (ConfirmDialog) + reject (ConfirmDialog w/ reason).
 * SL: ScopeBanner; own-record is auto-escalated to HR — action buttons hidden.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type Attendance,
  AttendanceStatus,
  type GetAttendance200,
  VerificationStatus,
  useGetAttendance,
  useRejectAttendance,
  useVerifyAttendance,
} from '@swp/api-client/e5';
import type { StatusTone } from '@swp/design-tokens';
import { ConfirmDialog, DateText, Input, StatusBadge, useToast } from '@swp/ui';
import {
  ArrowLeft,
  CheckCheck,
  CheckSquare,
  Clock,
  MapPin,
  TriangleAlert,
  XCircle,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface AttendanceDetailScreenProps {
  attendanceId: string;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function attendanceStatusTone(s: AttendanceStatus): StatusTone {
  switch (s) {
    case AttendanceStatus.PRESENT:
      return 'ok';
    case AttendanceStatus.LATE:
      return 'warn';
    case AttendanceStatus.ABSENT:
      return 'bad';
    case AttendanceStatus.INCOMPLETE:
      return 'warn';
    case AttendanceStatus.ON_LEAVE:
      return 'info';
    default:
      return 'neutral';
  }
}

function verificationStatusTone(vs: VerificationStatus): StatusTone {
  switch (vs) {
    case VerificationStatus.VERIFIED:
    case VerificationStatus.AUTO_APPROVED:
      return 'ok';
    case VerificationStatus.PENDING:
    case VerificationStatus.ESCALATED:
      return 'warn';
    case VerificationStatus.REJECTED:
      return 'bad';
    default:
      return 'neutral';
  }
}

const FLAG_LABELS: Record<string, string> = {
  LATE: 'flagLabel.LATE',
  EARLY: 'flagLabel.EARLY',
  OUTSIDE_GEOFENCE: 'flagLabel.OUTSIDE_GEOFENCE',
  UNSCHEDULED: 'flagLabel.UNSCHEDULED',
  ESCALATED: 'flagLabel.ESCALATED',
  CORRECTED: 'flagLabel.CORRECTED',
  AUTO_CLOSED: 'flagLabel.AUTO_CLOSED',
  ABSENT: 'flagLabel.ABSENT',
  NEEDS_CODE_VERIFICATION: 'flagLabel.NEEDS_CODE_VERIFICATION',
};

// ---------------------------------------------------------------------------
// AttendanceDetailScreen
// ---------------------------------------------------------------------------

export function AttendanceDetailScreen({ attendanceId }: AttendanceDetailScreenProps) {
  const { t } = useTranslation('attendance');
  const currentUser = useCurrentUser();
  const { toast } = useToast();
  const isShiftLeader = currentUser?.role === 'shift_leader';

  const [verifyOpen, setVerifyOpen] = useState(false);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState('');
  const [verifyCheckInAt, setVerifyCheckInAt] = useState('');
  const [verifyCheckOutAt, setVerifyCheckOutAt] = useState('');

  const query = useGetAttendance(attendanceId);
  // getAttendanceResponse.data is GetAttendance200 | error unions; GetAttendance200 has { data?: Attendance }
  const record = (query.data?.data as GetAttendance200 | undefined)?.data as Attendance | undefined;

  const verifySingle = useVerifyAttendance();
  const rejectSingle = useRejectAttendance();

  const isSaving = verifySingle.isPending || rejectSingle.isPending;

  function handleVerifyConfirm() {
    const data: Record<string, string> = {};
    const needsTimes =
      record?.status === AttendanceStatus.ABSENT || record?.status === AttendanceStatus.INCOMPLETE;
    if (needsTimes && verifyCheckInAt) {
      // datetime-local gives YYYY-MM-DDTHH:mm in local time; append WIB offset explicitly.
      data.check_in_at = `${verifyCheckInAt}+07:00`;
      if (verifyCheckOutAt) {
        data.check_out_at = `${verifyCheckOutAt}+07:00`;
      }
    }
    verifySingle.mutate(
      { id: attendanceId, data },
      {
        onSuccess: () => {
          setVerifyOpen(false);
          toast({ tone: 'success', title: t('verifySuccess') });
        },
        onError: (err) => {
          const e = classifyError(err);
          toast({ tone: 'error', title: t('verifyError'), description: e.message });
        },
      },
    );
  }

  function handleRejectConfirm() {
    rejectSingle.mutate(
      { id: attendanceId, data: { reason: rejectReason } },
      {
        onSuccess: () => {
          setRejectOpen(false);
          setRejectReason('');
          toast({ tone: 'success', title: t('rejectSuccess') });
        },
        onError: (err) => {
          const e = classifyError(err);
          toast({ tone: 'error', title: t('rejectError'), description: e.message });
        },
      },
    );
  }

  // Error state
  if (query.isError) {
    const err = classifyError(query.error);
    if (err.kind === 'not-found') {
      return (
        <div className="flex flex-col gap-4">
          <button
            type="button"
            aria-label={t('back')}
            className="flex items-center gap-2 text-[13px] font-medium text-text-2 hover:text-text"
            onClick={() => window.history.back()}
          >
            <ArrowLeft className="size-4" aria-hidden />
            {t('back')}
          </button>
          <div className="rounded-xl border border-border bg-surface px-5 py-[18px]">
            <p className="text-[14px] text-text-2">{t('notFound')}</p>
          </div>
        </div>
      );
    }
    if (err.kind === 'forbidden') {
      return (
        <div className="rounded-xl border border-border bg-surface px-5 py-[18px]">
          <p className="text-[14px] text-text-2">{t('noPermission')}</p>
        </div>
      );
    }
    return (
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <span className="text-[14px] text-bad">{t('loadError')}</span>
        <button
          type="button"
          className="rounded-lg border border-border bg-surface px-4 py-2 text-[13px] font-medium text-text-2 hover:bg-surface-2"
          onClick={() => query.refetch()}
        >
          {t('retry')}
        </button>
      </div>
    );
  }

  // Loading skeleton
  if (query.isLoading || !record) {
    return (
      <div className="flex flex-col gap-4 animate-pulse">
        <div className="h-8 w-40 rounded bg-surface-2" />
        <div className="h-[100px] w-full rounded-xl bg-surface-2" />
        <div className="grid grid-cols-2 gap-4">
          <div className="h-[300px] rounded-xl bg-surface-2" />
          <div className="h-[300px] rounded-xl bg-surface-2" />
        </div>
      </div>
    );
  }

  const isPending =
    record.verification_status === VerificationStatus.PENDING ||
    record.verification_status === VerificationStatus.ESCALATED;

  // SL cannot verify/reject their own escalated records
  const isOwnEscalated =
    isShiftLeader && record.verification_status === VerificationStatus.ESCALATED;

  const canActOnRecord = isPending && !isOwnEscalated;

  return (
    <div className="flex flex-col gap-4">
      {/* SL scope banner */}
      {isShiftLeader && (
        <div className="flex items-center gap-2 bg-warn-bg px-6 py-[10px] border-b border-warn-bd">
          <span className="text-warn-tx" aria-hidden>
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              aria-hidden="true"
            >
              <title>lock</title>
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
          </span>
          <p className="text-[12px] font-semibold text-warn-tx">{t('scopeBanner')}</p>
        </div>
      )}

      {/* Back row */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <button
            type="button"
            aria-label={t('back')}
            className="flex size-8 items-center justify-center rounded-lg border border-border bg-surface hover:bg-surface-2"
            onClick={() => window.history.back()}
          >
            <ArrowLeft className="size-4 text-text-2" aria-hidden />
          </button>
          <span className="text-[13px] font-medium text-text-2">{t('back')}</span>
        </div>
        {canActOnRecord && (
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="flex items-center gap-2 rounded-lg border border-border bg-surface px-4 py-[8px] text-[13px] font-medium text-text-2 hover:bg-surface-2 disabled:opacity-50"
              disabled={isSaving}
              onClick={() => setVerifyOpen(true)}
            >
              <CheckSquare className="size-4" aria-hidden />
              {t('verifyBtn')}
            </button>
            <button
              type="button"
              className="flex items-center gap-2 rounded-lg border border-bad/40 bg-surface px-4 py-[8px] text-[13px] font-medium text-bad hover:bg-bad-bg disabled:opacity-50"
              disabled={isSaving}
              onClick={() => {
                setRejectReason('');
                setRejectOpen(true);
              }}
            >
              <XCircle className="size-4" aria-hidden />
              {t('rejectBtn')}
            </button>
          </div>
        )}
        {isOwnEscalated && (
          <div className="flex items-center gap-2 rounded-lg bg-bad-bg px-3 py-[7px]">
            <TriangleAlert className="size-4 text-bad" aria-hidden />
            <span className="text-[12px] font-medium text-bad">{t('escalatedToHR')}</span>
          </div>
        )}
      </div>

      {/* Header card */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface p-5">
        <div className="flex items-center gap-4">
          <div className="flex size-11 items-center justify-center rounded-full bg-primary-soft text-[16px] font-bold text-primary">
            {(record.employee_name ?? record.employee_id)
              .split(' ')
              .slice(0, 2)
              .map((p: string) => p[0] ?? '')
              .join('')
              .toUpperCase()}
          </div>
          <div className="flex flex-col gap-1">
            <span className="text-[16px] font-semibold text-text">
              {record.employee_name ?? record.employee_id}
            </span>
            <div className="flex items-center gap-2">
              <span className="font-mono text-[11px] text-text-3">{record.employee_id}</span>
              <span className="text-text-3">·</span>
              <span className="text-[12px] text-text-2">
                {record.company_name ?? record.company_id}
              </span>
              {record.position && (
                <>
                  <span className="text-text-3">·</span>
                  <span className="text-[12px] text-text-2">{record.position}</span>
                </>
              )}
            </div>
          </div>
        </div>
        <div className="flex flex-col items-end gap-2">
          <StatusBadge dot tone={attendanceStatusTone(record.status)}>
            {t(`status.${record.status}`)}
          </StatusBadge>
          <StatusBadge dot tone={verificationStatusTone(record.verification_status)}>
            {t(`verificationStatus.${record.verification_status}`)}
          </StatusBadge>
        </div>
      </div>

      {/* Two-column layout */}
      <div className="grid grid-cols-[1fr_392px] gap-4">
        {/* Left col: times, geo, flags */}
        <div className="flex flex-col gap-4">
          {/* Clock times */}
          <div className="flex flex-col gap-3 rounded-xl border border-border bg-surface p-5">
            <h2 className="text-[13px] font-semibold text-text">{t('clockSection')}</h2>
            <div className="grid grid-cols-2 gap-4">
              <div className="flex flex-col gap-1">
                <span className="text-[11px] font-medium uppercase tracking-wider text-text-3">
                  {t('checkInLabel')}
                </span>
                <div className="flex items-center gap-2">
                  <Clock className="size-4 text-text-3" aria-hidden />
                  {record.check_in_at ? (
                    <DateText
                      value={record.check_in_at}
                      kind="instant"
                      className="text-[14px] font-medium text-text"
                    />
                  ) : (
                    <span className="text-[14px] font-medium text-text-3">
                      {t('colCheckInEmpty')}
                    </span>
                  )}
                </div>
                {record.shift_start_at && (
                  <span className="text-[11px] text-text-3">
                    {t('scheduled')}:{' '}
                    <DateText
                      value={record.shift_start_at}
                      kind="instant"
                      className="inline text-[11px]"
                    />
                  </span>
                )}
                {(record.late_minutes ?? 0) > 0 ? (
                  <span className="text-[11px] text-warn-tx font-medium">
                    {t('lateBy', { minutes: record.late_minutes })}
                  </span>
                ) : record.status === AttendanceStatus.PRESENT ||
                  record.status === AttendanceStatus.LATE ? (
                  <span className="text-[11px] text-ok-tx font-medium">{t('onTime')}</span>
                ) : null}
              </div>
              <div className="flex flex-col gap-1">
                <span className="text-[11px] font-medium uppercase tracking-wider text-text-3">
                  {t('checkOutLabel')}
                </span>
                {record.check_out_at ? (
                  <div className="flex items-center gap-2">
                    <Clock className="size-4 text-text-3" aria-hidden />
                    <DateText
                      value={record.check_out_at}
                      kind="instant"
                      className="text-[14px] font-medium text-text"
                    />
                  </div>
                ) : (
                  <span className="text-[13px] text-text-3 italic">{t('noCheckOut')}</span>
                )}
                {record.shift_end_at && (
                  <span className="text-[11px] text-text-3">
                    {t('scheduled')}:{' '}
                    <DateText
                      value={record.shift_end_at}
                      kind="instant"
                      className="inline text-[11px]"
                    />
                  </span>
                )}
                {record.worked_minutes != null && (
                  <span className="text-[11px] text-text-2">
                    {t('workedMinutes', { minutes: record.worked_minutes })}
                  </span>
                )}
              </div>
            </div>
          </div>

          {/* Geolocation */}
          <div className="flex flex-col gap-3 rounded-xl border border-border bg-surface p-5">
            <h2 className="text-[13px] font-semibold text-text">{t('geoSection')}</h2>
            <div className="grid grid-cols-2 gap-4">
              <div className="flex flex-col gap-1">
                <span className="text-[11px] font-medium uppercase tracking-wider text-text-3">
                  {t('checkInGeo')}
                </span>
                <div className="flex items-center gap-2">
                  <MapPin className="size-4 text-text-3" aria-hidden />
                  <span className="font-mono text-[12px] text-text">
                    {record.lat_in != null && record.lng_in != null
                      ? `${record.lat_in.toFixed(6)}, ${record.lng_in.toFixed(6)}`
                      : '—'}
                  </span>
                </div>
                {record.geofence_in && (
                  <StatusBadge dot tone={record.geofence_in.inside ? 'ok' : 'bad'}>
                    {record.geofence_in.inside ? t('geofence.INSIDE') : t('geofence.OUTSIDE')}
                  </StatusBadge>
                )}
              </div>
              {record.lat_out != null && record.lng_out != null && (
                <div className="flex flex-col gap-1">
                  <span className="text-[11px] font-medium uppercase tracking-wider text-text-3">
                    {t('checkOutGeo')}
                  </span>
                  <div className="flex items-center gap-2">
                    <MapPin className="size-4 text-text-3" aria-hidden />
                    <span className="font-mono text-[12px] text-text">
                      {(record.lat_out as number).toFixed(6)},{' '}
                      {(record.lng_out as number).toFixed(6)}
                    </span>
                  </div>
                  {record.geofence_out && (
                    <StatusBadge dot tone={record.geofence_out.inside ? 'ok' : 'bad'}>
                      {record.geofence_out.inside ? t('geofence.INSIDE') : t('geofence.OUTSIDE')}
                    </StatusBadge>
                  )}
                </div>
              )}
            </div>
          </div>

          {/* Flags */}
          {record.flags.length > 0 && (
            <div className="flex flex-col gap-3 rounded-xl border border-border bg-surface p-5">
              <h2 className="text-[13px] font-semibold text-text">{t('flagsSection')}</h2>
              <div className="flex flex-wrap gap-2">
                {record.flags.map((flag) => (
                  <span
                    key={flag}
                    className="rounded-full bg-warn-bg px-[10px] py-[4px] text-[11px] font-semibold text-warn-tx"
                  >
                    {t(FLAG_LABELS[flag] ?? flag)}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Right col: verification info */}
        <div className="flex flex-col gap-4">
          {/* Verification history */}
          <div className="flex flex-col gap-3 rounded-xl border border-border bg-surface p-5">
            <h2 className="text-[13px] font-semibold text-text">{t('verificationSection')}</h2>

            {record.verification_status === VerificationStatus.VERIFIED && record.verified_at && (
              <div className="flex flex-col gap-1">
                <span className="text-[11px] text-text-3">{t('verifiedBy')}</span>
                <span className="text-[13px] font-medium text-text">{record.verified_by}</span>
                <DateText
                  value={record.verified_at}
                  kind="instant"
                  className="text-[11px] text-text-3"
                />
              </div>
            )}

            {record.verification_status === VerificationStatus.REJECTED && (
              <div className="flex flex-col gap-2">
                <div className="flex flex-col gap-1">
                  <span className="text-[11px] text-text-3">{t('rejectedBy')}</span>
                  <span className="text-[13px] font-medium text-text">{record.rejected_by}</span>
                  {record.rejected_at && (
                    <DateText
                      value={record.rejected_at}
                      kind="instant"
                      className="text-[11px] text-text-3"
                    />
                  )}
                </div>
                {record.reject_reason && (
                  <div className="rounded-lg bg-bad-bg p-3">
                    <p className="text-[12px] text-bad">{record.reject_reason}</p>
                  </div>
                )}
              </div>
            )}

            {record.verification_status === VerificationStatus.PENDING && (
              <p className="text-[12px] text-text-3 italic">{t('pendingNoAction')}</p>
            )}

            {record.verification_status === VerificationStatus.ESCALATED && (
              <div className="flex flex-col gap-2">
                <p className="text-[12px] text-warn-tx">{t('escalatedNote')}</p>
              </div>
            )}

            {record.verification_status === VerificationStatus.AUTO_APPROVED && (
              <p className="text-[12px] text-ok-tx">{t('autoApprovedNote')}</p>
            )}
          </div>

          {/* Record metadata */}
          <div className="flex flex-col gap-3 rounded-xl border border-border bg-surface p-5">
            <h2 className="text-[13px] font-semibold text-text">{t('metaSection')}</h2>
            <dl className="flex flex-col gap-2">
              {[
                { label: t('metaId'), value: record.id },
                { label: t('metaPlacement'), value: record.placement_id },
                { label: t('metaAutoClosed'), value: record.auto_closed ? t('yes') : t('no') },
                {
                  label: t('metaCreated'),
                  node: (
                    <DateText
                      value={record.created_at}
                      kind="instant"
                      className="text-[12px] text-text"
                    />
                  ),
                },
                {
                  label: t('metaUpdated'),
                  node: (
                    <DateText
                      value={record.updated_at}
                      kind="instant"
                      className="text-[12px] text-text"
                    />
                  ),
                },
              ].map(({ label, value, node }) => (
                <div key={label} className="flex justify-between gap-4">
                  <dt className="text-[11px] text-text-3 shrink-0">{label}</dt>
                  <dd className="font-mono text-[11px] text-text text-right truncate">
                    {node ?? value}
                  </dd>
                </div>
              ))}
            </dl>
          </div>
        </div>
      </div>

      {/* Verify confirm */}
      <ConfirmDialog
        open={verifyOpen}
        onOpenChange={(open) => {
          setVerifyOpen(open);
          if (!open) {
            setVerifyCheckInAt('');
            setVerifyCheckOutAt('');
          }
        }}
        icon={CheckCheck}
        tone="neutral"
        title={t('verifyDialogTitle')}
        description={t('verifyDialogDesc')}
        cancelLabel={t('cancel')}
        confirmLabel={verifySingle.isPending ? t('saving') : t('verifyConfirm')}
        confirmTone="primary"
        confirmDisabled={
          verifySingle.isPending ||
          ((record?.status === AttendanceStatus.ABSENT ||
            record?.status === AttendanceStatus.INCOMPLETE) &&
            !verifyCheckInAt)
        }
        loading={verifySingle.isPending}
        onConfirm={handleVerifyConfirm}
      >
        {(record?.status === AttendanceStatus.ABSENT ||
          record?.status === AttendanceStatus.INCOMPLETE) && (
          <div className="mt-3 flex flex-col gap-3">
            <div className="flex flex-col gap-1">
              <label htmlFor="verify-checkin" className="text-[12px] font-medium text-text-2">
                {t('checkInLabel')}
              </label>
              <Input
                id="verify-checkin"
                type="datetime-local"
                value={verifyCheckInAt}
                onChange={(e) => setVerifyCheckInAt(e.target.value)}
                required
              />
            </div>
            <div className="flex flex-col gap-1">
              <label htmlFor="verify-checkout" className="text-[12px] font-medium text-text-2">
                {t('checkOutLabel')}
              </label>
              <Input
                id="verify-checkout"
                type="datetime-local"
                value={verifyCheckOutAt}
                onChange={(e) => setVerifyCheckOutAt(e.target.value)}
              />
            </div>
            <p className="text-[11px] text-text-3">{t('verifyTimeNote')}</p>
          </div>
        )}
      </ConfirmDialog>

      {/* Reject confirm */}
      <ConfirmDialog
        open={rejectOpen}
        onOpenChange={(open) => {
          setRejectOpen(open);
          if (!open) setRejectReason('');
        }}
        icon={XCircle}
        tone="warn"
        title={t('rejectDialogTitle')}
        description={t('rejectDialogDesc')}
        cancelLabel={t('cancel')}
        confirmLabel={rejectSingle.isPending ? t('saving') : t('rejectConfirm')}
        confirmTone="danger"
        loading={rejectSingle.isPending}
        confirmDisabled={rejectReason.trim().length < 5}
        onConfirm={handleRejectConfirm}
      >
        <div className="mt-3 flex flex-col gap-1">
          <label htmlFor="detail-reject-reason" className="text-[12px] font-medium text-text-2">
            {t('rejectReasonLabel')}
          </label>
          <Input
            id="detail-reject-reason"
            placeholder={t('rejectReasonPlaceholder')}
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
          />
        </div>
      </ConfirmDialog>
    </div>
  );
}
