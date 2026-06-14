/**
 * E6 · Detail Pengajuan Cuti — Request detail + E11 approval chain + actions.
 *
 * Approval routing moved to the E11 engine (EPICS §8 E11, 2026-06-14): leave no longer runs
 * its own L1/HR state machine. This screen reads `approval_instance_id` off the request and:
 *   - renders the E11 chain (ApprovalChainTimeline, reused from features/e11-approvals),
 *   - performs approve / reject via useApproveApprovalInstance / useRejectApprovalInstance.
 * Self-approval and line membership are server-enforced (INV-3, ENGINEERING C1); the UI handles
 * 403 SELF_APPROVAL_FORBIDDEN + 409 LINE_ALREADY_CLEARED gracefully (toast + refetch).
 *
 * `LeaveRequest.status` is now DRAFT | PENDING | APPROVED | REJECTED | CANCELLED.
 *
 * Route: /leave/$leaveRequestId
 */

import { classifyError } from '@/lib/api-error.ts';
import { ApiError } from '@swp/api-client';
import { type LeaveRequest, LeaveStatus, useGetLeaveRequest } from '@swp/api-client/e6';
import {
  type ApprovalInstanceDetail,
  useApproveApprovalInstance,
  useGetApprovalInstance,
  useRejectApprovalInstance,
} from '@swp/api-client/e11';
import { Button, DateText, EmptyState, IdChip, StateView, StatusBadge, useToast } from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { Link as RouterLink } from '@tanstack/react-router';
import { ArrowUpRight, FileText, ShieldAlert } from 'lucide-react';
import type * as React from 'react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ApprovalChainTimeline } from '../e11-approvals/approval-chain-timeline.tsx';
import { RejectLeaveModal, leaveStatusTone } from './leave-overlays.tsx';

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
  const { t: ta } = useTranslation('approvals');
  const { toast } = useToast();
  const queryClient = useQueryClient();

  // ---------------------------------------------------------------------------
  // Query — leave request
  // ---------------------------------------------------------------------------

  const query = useGetLeaveRequest(leaveRequestId);
  // The Go BE wraps detail in the standard `{ data: <LeaveRequest> }` envelope; the E6 openapi
  // declares the bare object, so unwrap the extra `data` layer when present.
  const outer = query.data?.data as (LeaveRequest & { data?: LeaveRequest }) | undefined;
  const raw: unknown =
    outer && typeof outer === 'object' && 'data' in outer && outer.data ? outer.data : outer;
  const lr: LeaveRequest | undefined =
    raw && typeof raw === 'object' && 'id' in raw && 'status' in raw
      ? (raw as LeaveRequest)
      : undefined;

  const instanceId = lr?.approval_instance_id ?? undefined;

  // ---------------------------------------------------------------------------
  // Query — E11 approval instance (the chain)
  // ---------------------------------------------------------------------------

  const instanceQuery = useGetApprovalInstance(instanceId ?? '', {
    query: { enabled: Boolean(instanceId) },
  });
  const instance = instanceQuery.data?.data as ApprovalInstanceDetail | undefined;

  // ---------------------------------------------------------------------------
  // Overlay state + mutations (E11 instance hooks)
  // ---------------------------------------------------------------------------

  const [rejectOpen, setRejectOpen] = useState(false);

  const approve = useApproveApprovalInstance();
  const reject = useRejectApprovalInstance();

  /** Invalidate the leave detail + every approval-instance list so cleared rows drop out. */
  function invalidateAll() {
    void query.refetch();
    void instanceQuery.refetch();
    queryClient.invalidateQueries({
      predicate: (q) =>
        q.queryKey.some((k) => typeof k === 'string' && k.includes('/approval-instances')),
    });
  }

  function handleActionError(err: unknown) {
    const code = err instanceof ApiError ? err.code : '';
    if (code === 'SELF_APPROVAL_FORBIDDEN') {
      toast({ tone: 'error', title: ta('detail.toastSelfApprovalTitle') });
      invalidateAll();
      return;
    }
    if (code === 'LINE_ALREADY_CLEARED') {
      toast({ tone: 'info', title: ta('detail.toastAlreadyClearedTitle') });
      invalidateAll();
      return;
    }
    const { kind } = classifyError(err);
    toast({
      tone: 'error',
      title: t('errors.processFailed'),
      description: kind === 'network' ? t('errors.network') : t('errors.tryAgain'),
    });
  }

  function handleApprove() {
    if (!instanceId) return;
    approve.mutate(
      { id: instanceId, data: {} },
      {
        onSuccess: () => {
          invalidateAll();
          toast({ tone: 'success', title: t('detail.toastApprovedTitle') });
        },
        onError: handleActionError,
      },
    );
  }

  function handleRejectConfirm(reason: string) {
    if (!instanceId) return;
    reject.mutate(
      { id: instanceId, data: { reason } },
      {
        onSuccess: () => {
          setRejectOpen(false);
          invalidateAll();
          toast({ tone: 'success', title: t('detail.toastRejectedTitle') });
        },
        onError: (err) => {
          setRejectOpen(false);
          handleActionError(err);
        },
      },
    );
  }

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

  const isPending = lr.status === LeaveStatus.PENDING;
  // Approve/reject are only meaningful while the instance is on its current line. Membership is
  // server-enforced; the UI offers the action whenever the request is PENDING and has an instance.
  const canAct = isPending && Boolean(instanceId);

  const approving = approve.isPending;
  const rejecting = reject.isPending;

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
            {lr.employee_company_name && (
              <span className="text-xs text-text-3">{lr.employee_company_name}</span>
            )}
          </div>
          <StatusBadge dot tone={leaveStatusTone(lr.status)}>
            {t(`status.${lr.status}`)}
          </StatusBadge>
          {lr.backdated && (
            <span className="rounded-full bg-warn-bg px-2.5 py-0.5 text-xs text-warn-tx">
              {t('detail.backdatedBadge')}
            </span>
          )}
        </div>

        {/* Action buttons — approve/reject via the E11 instance */}
        {canAct && (
          <div className="flex items-center gap-2.5">
            <Button
              type="button"
              variant="destructive"
              onClick={() => setRejectOpen(true)}
              disabled={rejecting || approving}
            >
              {rejecting ? t('common.processing') : t('detail.actionReject')}
            </Button>
            <Button
              type="button"
              variant="primary"
              onClick={handleApprove}
              disabled={approving || rejecting}
            >
              {approving ? t('common.processing') : t('detail.actionApprove')}
            </Button>
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

        {/* Right col — quota + approval chain */}
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
                  label={t('detail.balancePool')}
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

          {/* E11 approval chain (reuses the e11-approvals timeline organism) */}
          <Section
            title={t('detail.sectionTimeline')}
            action={
              instanceId ? (
                <RouterLink
                  to="/approval-instances/$instanceId"
                  params={{ instanceId }}
                  className="inline-flex items-center gap-1 text-xs font-semibold text-primary hover:underline"
                >
                  {t('detail.viewChain')}
                  <ArrowUpRight aria-hidden className="size-3.5" />
                </RouterLink>
              ) : undefined
            }
          >
            {!instanceId ? (
              <p className="text-sm text-text-3">{t('detail.chainPending')}</p>
            ) : instanceQuery.isLoading ? (
              <div className="h-24 w-full animate-pulse rounded-lg bg-surface-2" />
            ) : instance ? (
              <ApprovalChainTimeline
                lines={instance.lines ?? []}
                actions={instance.actions ?? []}
                currentLine={instance.current_line}
                status={instance.status}
                requesterName={lr.employee_name ?? lr.employee_id}
                t={ta}
              />
            ) : (
              <p className="text-sm text-text-3">{ta('detail.chainEmpty')}</p>
            )}
          </Section>
        </div>
      </div>

      {/* Reject overlay */}
      {canAct && (
        <RejectLeaveModal
          open={rejectOpen}
          leaveRequest={lr}
          onOpenChange={setRejectOpen}
          onConfirm={handleRejectConfirm}
          isPending={rejecting}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Section wrapper
// ---------------------------------------------------------------------------

function Section({
  title,
  action,
  children,
}: {
  title: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-xl border border-border bg-surface p-5">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="font-semibold text-sm text-text-2 uppercase tracking-wide">{title}</h2>
        {action}
      </div>
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
