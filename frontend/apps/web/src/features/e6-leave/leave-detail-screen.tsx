/**
 * E6 · Detail Pengajuan Cuti — Request detail + approval actions.
 *
 * .pen frames implemented:
 *   DJrBn  E6 · Detail Pengajuan Cuti                        (HR final approve, standard 2-level)
 *   eHXWF  E6 · Detail Pengajuan Cuti — No-leader            (HR sole approver, routing.no_leader)
 *   ZlnfW  E6 · Detail Pengajuan Cuti — Saldo berubah (LA-5) (balance_check.requires_override)
 *   Hzbbv  E6 SL · Detail Pengajuan Cuti (L1)                (SL L1 approve/reject)
 *
 * Route: /leave/$leaveRequestId
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { ApiError } from '@swp/api-client';
import {
  LeaveDecision,
  type LeaveRequest,
  LeaveStatus,
  type LeaveTimelineEntry,
  useApproveLeaveRequestFinal,
  useApproveLeaveRequestL1,
  useApproveLeaveRequestOverride,
  useGetLeaveRequest,
  useRejectLeaveRequest,
} from '@swp/api-client/e6';
import { Button, DateText, EmptyState, IdChip, StateView, StatusBadge, useToast } from '@swp/ui';
import { CheckCircle2, Clock, FileText, ShieldAlert, XCircle } from 'lucide-react';
import type * as React from 'react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { BalanceChangedModal, RejectLeaveModal, leaveStatusTone } from './leave-overlays.tsx';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface LeaveDetailScreenProps {
  leaveRequestId: string;
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function LeaveDetailScreen({ leaveRequestId }: LeaveDetailScreenProps) {
  const { t } = useTranslation('leave');
  const { toast } = useToast();
  const user = useCurrentUser();

  const isHR = user?.role === 'hr_admin' || user?.role === 'super_admin';
  const isSL = user?.role === 'shift_leader';

  // ---------------------------------------------------------------------------
  // Query
  // ---------------------------------------------------------------------------

  const query = useGetLeaveRequest(leaveRequestId);
  // The real Go BE wraps the detail (and the approve/reject action results) in the
  // standard `{ data: <LeaveRequest> }` envelope — the same envelope every other
  // epic's detail screen unwraps (e.g. attendance-detail reads query.data.data.data).
  // The E6 openapi declares the bare LeaveRequest (no envelope), so the generated
  // type narrows `query.data.data` to LeaveRequest; we unwrap the BE's extra `data`
  // layer when present and fall back to the bare shape so both contracts work.
  const outer = query.data?.data as
    | (LeaveRequest & { data?: LeaveRequest })
    | undefined;
  const raw: unknown =
    outer && typeof outer === 'object' && 'data' in outer && outer.data ? outer.data : outer;
  // The Orval response union includes error types; narrow to LeaveRequest.
  const lr: LeaveRequest | undefined =
    raw && typeof raw === 'object' && 'id' in raw && 'status' in raw
      ? (raw as LeaveRequest)
      : undefined;

  // ---------------------------------------------------------------------------
  // Overlay state
  // ---------------------------------------------------------------------------

  const [rejectOpen, setRejectOpen] = useState(false);
  const [overrideOpen, setOverrideOpen] = useState(false);

  // ---------------------------------------------------------------------------
  // Mutations
  // ---------------------------------------------------------------------------

  const approveL1 = useApproveLeaveRequestL1({
    mutation: {
      onSuccess: () => {
        toast({
          tone: 'success',
          title: t('detail.toastForwardedTitle'),
          description: t('detail.toastForwardedBody'),
        });
        void query.refetch();
      },
      onError: (err) => {
        const { kind } = classifyError(err);
        toast({
          tone: 'error',
          title: t('errors.processFailed'),
          description: kind === 'network' ? t('errors.network') : t('errors.tryAgain'),
        });
      },
    },
  });

  const approveFinal = useApproveLeaveRequestFinal({
    mutation: {
      onSuccess: () => {
        toast({
          tone: 'success',
          title: t('detail.toastApprovedTitle'),
          description: t('detail.toastApprovedBody'),
        });
        void query.refetch();
      },
      onError: (err) => {
        const { kind, message } = classifyError(err);
        // Balance re-check failed at final approval: BE returns 422
        // BALANCE_RECHECK_FAILED (code) — its Bahasa message does NOT contain
        // "BALANCE" and the presence of error.fields classifies it as 'validation'
        // (not 'rule'). Test the actual error CODE (mirrors the Phase-6 error.details
        // precedent) so the override modal opens against the real BE.
        const code = err instanceof ApiError ? err.code : '';
        const isBalanceRecheck =
          code === 'BALANCE_RECHECK_FAILED' ||
          code.toUpperCase().includes('BALANCE') ||
          (kind === 'rule' && message.toUpperCase().includes('BALANCE'));
        if (isBalanceRecheck) {
          setOverrideOpen(true);
          return;
        }
        toast({
          tone: 'error',
          title: t('errors.processFailed'),
          description: kind === 'network' ? t('errors.network') : t('errors.tryAgain'),
        });
      },
    },
  });

  const approveOverride = useApproveLeaveRequestOverride({
    mutation: {
      onSuccess: () => {
        setOverrideOpen(false);
        toast({
          tone: 'success',
          title: t('detail.toastApprovedTitle'),
          description: t('detail.toastApprovedBody'),
        });
        void query.refetch();
      },
      onError: (err) => {
        const { kind } = classifyError(err);
        toast({
          tone: 'error',
          title: t('errors.processFailed'),
          description: kind === 'network' ? t('errors.network') : t('errors.tryAgain'),
        });
      },
    },
  });

  const rejectMutation = useRejectLeaveRequest({
    mutation: {
      onSuccess: () => {
        setRejectOpen(false);
        toast({
          tone: 'success',
          title: t('detail.toastRejectedTitle'),
          description: t('detail.toastRejectedBody'),
        });
        void query.refetch();
      },
      onError: (err) => {
        const { kind } = classifyError(err);
        toast({
          tone: 'error',
          title: t('errors.processFailed'),
          description: kind === 'network' ? t('errors.network') : t('errors.tryAgain'),
        });
      },
    },
  });

  // ---------------------------------------------------------------------------
  // Loading / error / not-found states
  // ---------------------------------------------------------------------------

  if (query.isLoading) {
    return (
      <div className="flex flex-col gap-4 p-6">
        <div className="h-8 w-64 animate-pulse rounded-lg bg-surface-2" />
        <div className="h-48 w-full animate-pulse rounded-xl bg-surface-2" />
        <div className="h-64 w-full animate-pulse rounded-xl bg-surface-2" />
      </div>
    );
  }

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'not-found') {
      return (
        <EmptyState
          variant="filtered"
          title={t('detail.notFoundTitle')}
          description={t('detail.notFoundBody')}
        />
      );
    }
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <EmptyState
          variant="no-permission"
          title={t('errors.forbidden')}
          description={t('detail.noPermissionBody')}
        />
      );
    }
    return (
      <StateView
        kind="error"
        title={t('detail.errorTitle')}
        description={t('errors.network')}
        onRetry={() => query.refetch()}
        retryLabel={t('common.retry')}
      />
    );
  }

  if (!lr) return null;

  // ---------------------------------------------------------------------------
  // Derived state
  // ---------------------------------------------------------------------------

  const noLeader = Boolean(lr.routing?.no_leader);
  const balanceRequiresOverride = Boolean(lr.balance_check?.requires_override);

  const canApproveL1 = isSL && lr.status === LeaveStatus.PENDING_L1;
  const canApproveFinal = isHR && lr.status === LeaveStatus.PENDING_HR;
  const canReject =
    (isSL && lr.status === LeaveStatus.PENDING_L1) ||
    (isHR && lr.status === LeaveStatus.PENDING_HR);

  const isTerminal =
    lr.status === LeaveStatus.APPROVED ||
    lr.status === LeaveStatus.REJECTED ||
    lr.status === LeaveStatus.CANCELLED;

  const approvingL1 = approveL1.isPending;
  const approvingFinal = approveFinal.isPending || approveOverride.isPending;
  const rejecting = rejectMutation.isPending;

  // ---------------------------------------------------------------------------
  // Action handlers
  // ---------------------------------------------------------------------------

  function handleApproveL1() {
    approveL1.mutate({ id: leaveRequestId, data: {} });
  }

  function handleApproveFinal() {
    if (balanceRequiresOverride) {
      setOverrideOpen(true);
      return;
    }
    approveFinal.mutate({ id: leaveRequestId, data: {} });
  }

  function handleOverrideConfirm(overrideReason: string) {
    approveOverride.mutate({ id: leaveRequestId, data: { override_reason: overrideReason } });
  }

  function handleRejectConfirm(reason: string) {
    rejectMutation.mutate({ id: leaveRequestId, data: { reason } });
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-4">
      {/* Header card */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface p-5">
        <div className="flex items-center gap-3.5">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-surface-2 text-lg font-bold text-text">
            {(lr.employee_name ?? '?').charAt(0).toUpperCase()}
          </div>
          <div className="flex flex-col">
            <span className="font-semibold text-text">{lr.employee_name ?? lr.employee_id}</span>
            <span className="text-xs text-text-3">
              {lr.employee_company_name} · {lr.employee_service_line}
            </span>
          </div>
          <StatusBadge dot tone={leaveStatusTone(lr.status)}>
            {t(`status.${lr.status}`)}
          </StatusBadge>
          {noLeader && (
            <span className="rounded-full bg-surface-2 px-2.5 py-0.5 text-xs text-text-3">
              {t('detail.noLeaderBadge')}
            </span>
          )}
          {lr.backdated && (
            <span className="rounded-full bg-warn-bg px-2.5 py-0.5 text-xs text-warn-tx">
              {t('detail.backdatedBadge')}
            </span>
          )}
        </div>

        {/* Action buttons */}
        {!isTerminal && (
          <div className="flex items-center gap-2.5">
            {canReject && (
              <Button
                type="button"
                variant="destructive"
                onClick={() => setRejectOpen(true)}
                disabled={rejecting || approvingL1 || approvingFinal}
              >
                {rejecting ? t('common.processing') : t('detail.actionReject')}
              </Button>
            )}
            {canApproveL1 && (
              <Button
                type="button"
                variant="primary"
                onClick={handleApproveL1}
                disabled={approvingL1 || rejecting}
              >
                {approvingL1 ? t('common.processing') : t('detail.actionForward')}
              </Button>
            )}
            {canApproveFinal && (
              <Button
                type="button"
                variant={balanceRequiresOverride ? 'destructive' : 'primary'}
                onClick={handleApproveFinal}
                disabled={approvingFinal || rejecting}
              >
                {balanceRequiresOverride && <ShieldAlert aria-hidden className="size-3.5" />}
                {approvingFinal
                  ? t('common.processing')
                  : balanceRequiresOverride
                    ? t('detail.actionOverride')
                    : t('detail.actionApprove')}
              </Button>
            )}
          </div>
        )}
      </div>

      {/* Two-column layout */}
      <div className="flex gap-5">
        {/* Left col — request info */}
        <div className="flex w-[760px] flex-col gap-4">
          {/* Request details */}
          <Section title={t('detail.sectionRequest')}>
            <InfoRow label={t('detail.fieldId')}>
              <IdChip id={lr.id} />
            </InfoRow>
            <InfoRow label={t('detail.fieldType')}>
              <span className="text-sm text-text">{lr.leave_type_name ?? lr.leave_type_id}</span>
            </InfoRow>
            <InfoRow label={t('detail.fieldDates')}>
              <span className="text-sm text-text">
                <DateText kind="date" value={lr.start_date} /> –{' '}
                <DateText kind="date" value={lr.end_date} />
                <span className="ml-1.5 text-text-3">
                  ({lr.duration_days} {t('common.days')})
                </span>
              </span>
            </InfoRow>
            {lr.reason && (
              <InfoRow label={t('detail.fieldReason')}>
                <p className="text-sm leading-relaxed text-text-2">{lr.reason}</p>
              </InfoRow>
            )}
            {lr.notes && (
              <InfoRow label={t('detail.fieldNotes')}>
                <p className="text-sm leading-relaxed text-text-2">{lr.notes}</p>
              </InfoRow>
            )}
            {lr.delegate_name && (
              <InfoRow label={t('detail.fieldDelegate')}>
                <span className="text-sm text-text-2">{lr.delegate_name}</span>
              </InfoRow>
            )}
          </Section>

          {/* Attachments */}
          {lr.document_url && (
            <Section title={t('detail.sectionAttachment')}>
              <a
                href={lr.document_url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-sm font-medium text-primary hover:underline"
              >
                <FileText aria-hidden className="size-4" />
                {t('detail.viewDocument')}
              </a>
            </Section>
          )}

          {/* Clock-in conflict warning */}
          {lr.clock_in_conflict && (
            <div className="flex items-start gap-2 rounded-lg border border-warn-bd bg-warn-bg px-4 py-3">
              <ShieldAlert aria-hidden className="mt-0.5 size-4 shrink-0 text-warn-tx" />
              <p className="text-sm leading-relaxed text-warn-tx">{t('detail.clockInConflict')}</p>
            </div>
          )}
        </div>

        {/* Right col — quota + timeline */}
        <div className="flex flex-1 flex-col gap-4">
          {/* Balance check */}
          {lr.balance_check && (
            <Section title={t('detail.sectionBalance')}>
              <div className="flex flex-col gap-1.5">
                <BalanceStat
                  label={t('detail.balanceRequested')}
                  value={lr.balance_check.requested_days ?? 0}
                  t={t}
                />
                <BalanceStat
                  label={t('detail.balanceRemaining')}
                  value={lr.balance_check.remaining_days_at_check ?? 0}
                  t={t}
                  negative={Boolean(lr.balance_check.requires_override)}
                />
                {lr.balance_check.requires_override && (
                  <div className="mt-1.5 flex items-center gap-1.5 text-xs text-warn-tx">
                    <ShieldAlert aria-hidden className="size-3.5" />
                    {t('detail.balanceExceeded')}
                  </div>
                )}
              </div>
            </Section>
          )}

          {/* Schedule impact (post-approval) */}
          {lr.schedule_impact && lr.schedule_impact.length > 0 && (
            <Section title={t('detail.sectionScheduleImpact')}>
              <ul className="flex flex-col gap-1">
                {lr.schedule_impact.map((s) => (
                  <li key={s.schedule_id} className="flex items-center justify-between text-sm">
                    {s.date && <DateText kind="date" value={s.date} className="text-text-2" />}
                    <span className="text-xs text-text-3">{s.new_status}</span>
                  </li>
                ))}
              </ul>
            </Section>
          )}

          {/* Approval timeline */}
          <Section title={t('detail.sectionTimeline')}>
            <ApprovalTimeline timeline={lr.timeline} noLeader={noLeader} t={t} />
          </Section>
        </div>
      </div>

      {/* Overlays */}
      {canReject && (
        <RejectLeaveModal
          open={rejectOpen}
          leaveRequest={lr}
          onOpenChange={setRejectOpen}
          onConfirm={handleRejectConfirm}
          isPending={rejecting}
        />
      )}

      {canApproveFinal && (
        <BalanceChangedModal
          open={overrideOpen}
          leaveRequest={lr}
          onOpenChange={setOverrideOpen}
          onConfirm={handleOverrideConfirm}
          isPending={approveOverride.isPending}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Section wrapper
// ---------------------------------------------------------------------------

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-border bg-surface p-5">
      <h2 className="mb-4 font-semibold text-sm text-text-2 uppercase tracking-wide">{title}</h2>
      <div className="flex flex-col gap-3">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// InfoRow
// ---------------------------------------------------------------------------

function InfoRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-3">
      <span className="w-36 shrink-0 text-sm text-text-3">{label}</span>
      <div className="flex-1">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// BalanceStat
// ---------------------------------------------------------------------------

function BalanceStat({
  label,
  value,
  negative,
  t,
}: {
  label: string;
  value: number;
  negative?: boolean;
  t: (key: string) => string;
}) {
  return (
    <div className="flex items-center justify-between rounded-lg bg-surface-2 px-3.5 py-2.5">
      <span className="text-sm text-text-3">{label}</span>
      <span className={`font-semibold text-sm ${negative ? 'text-negative' : 'text-text'}`}>
        {value} {t('common.days')}
      </span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// ApprovalTimeline
// ---------------------------------------------------------------------------

function ApprovalTimeline({
  timeline,
  noLeader,
  t,
}: {
  timeline: LeaveTimelineEntry[];
  noLeader: boolean;
  t: (key: string, opts?: Record<string, unknown>) => string;
}) {
  return (
    <ol className="flex flex-col gap-4">
      {timeline.map((entry, idx) => {
        const isApproved =
          entry.decision === LeaveDecision.APPROVED ||
          entry.decision === LeaveDecision.OVERRIDE_APPROVED;
        const isRejected = entry.decision === LeaveDecision.REJECTED;
        const isPending = entry.status === 'PENDING';

        return (
          <li key={`${entry.stage}-${idx}`} className="flex gap-3">
            <div className="flex flex-col items-center">
              <div
                className={`flex h-7 w-7 items-center justify-center rounded-full text-sm font-medium ${
                  isApproved
                    ? 'bg-teal-50 text-teal-700'
                    : isRejected
                      ? 'bg-red-50 text-red-700'
                      : 'bg-surface-2 text-text-3'
                }`}
              >
                {isApproved ? (
                  <CheckCircle2 aria-hidden className="size-4" />
                ) : isRejected ? (
                  <XCircle aria-hidden className="size-4" />
                ) : (
                  <Clock aria-hidden className="size-4" />
                )}
              </div>
              {idx < timeline.length - 1 && (
                <div className="mt-1 h-full min-h-[16px] w-px bg-border" />
              )}
            </div>
            <div className="flex flex-1 flex-col gap-0.5 pb-2">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-text">
                  {noLeader && entry.stage === 'HR'
                    ? t('detail.stageSoleApprover')
                    : t(`detail.stage${entry.stage}`)}
                </span>
                {!isPending && entry.occurred_at && (
                  <DateText
                    kind="instant"
                    value={entry.occurred_at}
                    className="text-xs text-text-3"
                  />
                )}
              </div>
              {entry.actor_name && <span className="text-xs text-text-3">{entry.actor_name}</span>}
              {entry.reject_reason && (
                <p className="mt-1 rounded-lg bg-red-50 px-2.5 py-2 text-xs leading-relaxed text-red-800">
                  {t('detail.rejectionReason')}: {entry.reject_reason}
                </p>
              )}
              {entry.override && entry.override_reason && (
                <p className="mt-1 rounded-lg bg-amber-50 px-2.5 py-2 text-xs leading-relaxed text-amber-800">
                  <ShieldAlert aria-hidden className="mr-1 inline size-3" />
                  {t('detail.overrideReason')}: {entry.override_reason}
                </p>
              )}
              {entry.decision_note && (
                <p className="mt-1 text-xs italic text-text-3">{entry.decision_note}</p>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
