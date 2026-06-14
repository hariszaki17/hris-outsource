/**
 * E11 · Detail Permintaan — request detail + approval-chain timeline + actions.
 *
 * .pen frames implemented:
 *   OHseV  E11 · Detail Permintaan (rantai persetujuan)   — detail + chain timeline + bypass card
 *   KT3Jz  E11 · Overlay — Bypass (Super Admin)           — via ./bypass-modal.tsx
 *   EnabP  comp/ModalReject                               — reject overlay (frame-faithful inline)
 * Specs: F11.2 (execution) · F11.3 (instance detail) · IB-4 (chain timeline) ·
 *        INV-3 (no self-approval) · INV-5 (bypass).
 *
 * Route: /approval-instances/$instanceId  (param name `instanceId`).
 *
 * Loads `useGetApprovalInstance(id)` → ApprovalInstanceDetail (lines[] + actions[]) and renders:
 *   - request summary card, status pill ("Menunggu · Baris N dari M"),
 *   - the chain-progress timeline + action trail (./approval-chain-timeline.tsx),
 *   - Approve / Reject (ModalReject) on PENDING, super-admin Bypass card (KT3Jz).
 * Self-approval is disabled client-side as defense-in-depth (INV-3) but the server is the gate:
 * 403 SELF_APPROVAL_FORBIDDEN + 409 LINE_ALREADY_CLEARED are toasted and trigger a refetch.
 * All async surfaces have loading / empty / error / no-permission / terminal states (no dead flow).
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { ApiError } from '@swp/api-client';
import {
  type ApprovalInstanceDetail,
  InstanceStatus,
  RequestType,
  useApproveApprovalInstance,
  useBypassApprovalInstance,
  useGetApprovalInstance,
  useRejectApprovalInstance,
} from '@swp/api-client/e11';
import type { StatusTone } from '@swp/design-tokens';
import {
  Button,
  DateText,
  EmptyState,
  IdChip,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { ArrowLeft, Check, CornerUpLeft, Info, ShieldCheck, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ApprovalChainTimeline } from './approval-chain-timeline.tsx';
import { BypassModal } from './bypass-modal.tsx';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface ApprovalDetailScreenProps {
  instanceId: string;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function instanceStatusTone(status: InstanceStatus): StatusTone {
  switch (status) {
    case InstanceStatus.APPROVED:
      return 'ok';
    case InstanceStatus.REJECTED:
      return 'bad';
    case InstanceStatus.PENDING:
      return 'warn';
    default:
      return 'neutral';
  }
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export default function ApprovalDetailScreen({ instanceId }: ApprovalDetailScreenProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const user = useCurrentUser();

  // Bypass is super-admin only (api-facts: approvals.bypass). Server is the real gate (C1).
  const canBypass = user?.role === 'super_admin';

  // ---------------------------------------------------------------------------
  // Query — unwrap the standard `{ data: <detail> }` envelope (and the BE's extra
  // wrapper, mirroring leave-detail-screen) and narrow off the Orval error union.
  // ---------------------------------------------------------------------------

  const query = useGetApprovalInstance(instanceId);
  const outer = query.data?.data as
    | (ApprovalInstanceDetail & { data?: ApprovalInstanceDetail })
    | undefined;
  const raw: unknown =
    outer && typeof outer === 'object' && 'data' in outer && outer.data ? outer.data : outer;
  const instance: ApprovalInstanceDetail | undefined =
    raw && typeof raw === 'object' && 'id' in raw && 'status' in raw
      ? (raw as ApprovalInstanceDetail)
      : undefined;

  // ---------------------------------------------------------------------------
  // Overlay state
  // ---------------------------------------------------------------------------

  const [rejectOpen, setRejectOpen] = useState(false);
  const [bypassOpen, setBypassOpen] = useState(false);

  // ---------------------------------------------------------------------------
  // Shared error handler — surfaces server-enforced gates (INV-3 / OR clearance).
  // ---------------------------------------------------------------------------

  function handleActionError(err: unknown) {
    const { kind } = classifyError(err);
    const code = err instanceof ApiError ? err.code : '';
    if (code === 'SELF_APPROVAL_FORBIDDEN') {
      toast({
        tone: 'error',
        title: t('approvals.detail.toastSelfApprovalTitle'),
        description: t('approvals.detail.toastSelfApprovalBody'),
      });
      void query.refetch();
      return;
    }
    if (code === 'LINE_ALREADY_CLEARED' || kind === 'conflict') {
      toast({
        tone: 'error',
        title: t('approvals.detail.toastAlreadyClearedTitle'),
        description: t('approvals.detail.toastAlreadyClearedBody'),
      });
      void query.refetch();
      return;
    }
    toast({
      tone: 'error',
      title: t('approvals.detail.toastActionFailedTitle'),
      description: kind === 'network' ? t('common.errorNetwork') : t('common.errorTryAgain'),
    });
  }

  // ---------------------------------------------------------------------------
  // Mutations
  // ---------------------------------------------------------------------------

  const approve = useApproveApprovalInstance({
    mutation: {
      onSuccess: () => {
        toast({
          tone: 'success',
          title: t('approvals.detail.toastApprovedTitle'),
          description: t('approvals.detail.toastApprovedBody'),
        });
        void query.refetch();
      },
      onError: handleActionError,
    },
  });

  const reject = useRejectApprovalInstance({
    mutation: {
      onSuccess: () => {
        setRejectOpen(false);
        toast({
          tone: 'success',
          title: t('approvals.detail.toastRejectedTitle'),
          description: t('approvals.detail.toastRejectedBody'),
        });
        void query.refetch();
      },
      onError: handleActionError,
    },
  });

  const bypass = useBypassApprovalInstance({
    mutation: {
      onSuccess: () => {
        setBypassOpen(false);
        toast({
          tone: 'success',
          title: t('approvals.detail.toastBypassedTitle'),
          description: t('approvals.detail.toastBypassedBody'),
        });
        void query.refetch();
      },
      onError: handleActionError,
    },
  });

  // ---------------------------------------------------------------------------
  // Loading / error / not-found states (no dead-flow — ENGINEERING.md B2)
  // ---------------------------------------------------------------------------

  if (query.isLoading) {
    return (
      <div className="flex flex-col gap-4 p-6">
        <div className="h-8 w-72 animate-pulse rounded-lg bg-surface-2" />
        <div className="flex gap-5">
          <div className="h-80 w-[440px] animate-pulse rounded-xl bg-surface-2" />
          <div className="h-80 flex-1 animate-pulse rounded-xl bg-surface-2" />
        </div>
      </div>
    );
  }

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'not-found') {
      return (
        <EmptyState
          variant="filtered"
          title={t('approvals.detail.notFoundTitle')}
          description={t('approvals.detail.notFoundBody')}
        />
      );
    }
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <EmptyState
          variant="no-permission"
          title={t('approvals.detail.noPermissionTitle')}
          description={t('approvals.detail.noPermissionBody')}
        />
      );
    }
    return (
      <StateView
        kind="error"
        title={t('approvals.detail.errorTitle')}
        description={t('common.errorNetwork')}
        onRetry={() => query.refetch()}
        retryLabel={t('common.retry')}
      />
    );
  }

  if (!instance) return null;

  // ---------------------------------------------------------------------------
  // Derived state
  // ---------------------------------------------------------------------------

  const lineCount = instance.line_count ?? 1;
  const currentLine = instance.current_line || 1;
  const isPending = instance.status === InstanceStatus.PENDING;
  const isTerminal =
    instance.status === InstanceStatus.APPROVED || instance.status === InstanceStatus.REJECTED;

  const lines = instance.lines ?? [];
  const actions = instance.actions ?? [];

  // Self-approval defense-in-depth (INV-3): the requester cannot clear a line they're on.
  // `requester_id` is a `SWP-USR-…` id. We have no current-user id in the session today, so
  // this is best-effort — the SERVER is the gate (403 SELF_APPROVAL_FORBIDDEN is handled).
  const currentLineMembers = lines.find((l) => l.line_no === currentLine)?.members ?? [];
  const isRequester =
    Boolean(instance.requester_id) &&
    currentLineMembers.some((m) => m.user_id === instance.requester_id);
  const selfApprovalBlocked = isRequester;

  const requestTypeLabel =
    instance.request_type === RequestType.LEAVE
      ? t('approvals.detail.typeLeave')
      : t('approvals.detail.typeOvertime');

  const acting = approve.isPending || reject.isPending || bypass.isPending;

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-4">
      {/* TitleBand */}
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => window.history.back()}
            aria-label={t('common.back')}
            className="flex size-9 items-center justify-center rounded-lg border border-border bg-surface text-text-2 hover:bg-surface-2"
          >
            <ArrowLeft aria-hidden className="size-4" />
          </button>
          <div className="flex flex-col gap-0.5">
            <h1 className="font-bold text-[26px] leading-tight text-text">
              {requestTypeLabel} · {instance.summary?.trim() || instance.request_id}
            </h1>
            <div className="flex items-center gap-2 text-xs text-text-3">
              <IdChip id={instance.request_id} />
              <span>·</span>
              <IdChip id={instance.id} />
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2.5" data-testid="detail-status" data-status={instance.status}>
          <StatusBadge dot tone={instanceStatusTone(instance.status)}>
            {isPending
              ? t('approvals.detail.statusPendingLine', { current: currentLine, total: lineCount })
              : t(`approvals.status.${instance.status}`)}
          </StatusBadge>
          {isPending && (
            <>
              <Button
                type="button"
                variant="destructive"
                data-testid="detail-reject"
                onClick={() => setRejectOpen(true)}
                disabled={acting}
              >
                <X aria-hidden className="size-3.5" />
                {t('approvals.detail.actionReject')}
              </Button>
              <Button
                type="button"
                variant="primary"
                data-testid="detail-approve"
                onClick={() => approve.mutate({ id: instanceId, data: {} })}
                disabled={acting || selfApprovalBlocked}
                title={selfApprovalBlocked ? t('approvals.detail.selfApprovalHint') : undefined}
              >
                <Check aria-hidden className="size-3.5" />
                {approve.isPending ? t('common.processing') : t('approvals.detail.actionApprove')}
              </Button>
            </>
          )}
        </div>
      </div>

      {/* Self-approval defense-in-depth notice */}
      {isPending && selfApprovalBlocked && (
        <div className="flex items-start gap-2 rounded-lg border border-warn-bd bg-warn-bg px-3.5 py-2.5">
          <Info aria-hidden className="mt-0.5 size-4 shrink-0 text-warn-tx" />
          <p className="text-xs leading-relaxed text-warn-tx">
            {t('approvals.detail.selfApprovalHint')}
          </p>
        </div>
      )}

      {/* Two columns */}
      <div className="flex gap-5">
        {/* LeftCol — request summary */}
        <div className="flex w-[440px] flex-col gap-4">
          <div className="rounded-xl border border-border bg-surface p-5">
            <h2 className="mb-4 font-bold text-lg text-text">
              {t('approvals.detail.sectionRequest')}
            </h2>
            <div className="flex flex-col gap-2.5">
              <InfoRow label={t('approvals.detail.fieldType')}>
                <span className="font-semibold text-sm text-text">{requestTypeLabel}</span>
              </InfoRow>
              <InfoRow label={t('approvals.detail.fieldSummary')}>
                <span className="font-semibold text-sm text-text">
                  {instance.summary?.trim() || '—'}
                </span>
              </InfoRow>
              <InfoRow label={t('approvals.detail.fieldRequestId')}>
                <IdChip id={instance.request_id} />
              </InfoRow>
              {instance.requester_id && (
                <InfoRow label={t('approvals.detail.fieldRequester')}>
                  <IdChip id={instance.requester_id} />
                </InfoRow>
              )}
              <InfoRow label={t('approvals.detail.fieldCompany')}>
                <IdChip id={instance.company_id} />
              </InfoRow>
              {instance.created_at && (
                <InfoRow label={t('approvals.detail.fieldSubmitted')}>
                  <DateText
                    kind="instant"
                    value={instance.created_at}
                    className="text-sm text-text"
                  />
                </InfoRow>
              )}
            </div>
          </div>
        </div>

        {/* RightCol — chain timeline */}
        <div className="flex flex-1 flex-col gap-4">
          <div className="flex flex-col rounded-xl border border-border bg-surface p-5">
            <div className="flex flex-col gap-0.5">
              <h2 className="font-bold text-lg text-text">{t('approvals.detail.sectionChain')}</h2>
              <p className="text-xs text-text-3">
                {instance.template_version != null
                  ? t('approvals.detail.chainSubtitle', { version: instance.template_version })
                  : t('approvals.detail.chainSubtitleFallback')}
              </p>
            </div>

            {lines.length === 0 ? (
              <p className="pt-4 text-sm text-text-3">{t('approvals.detail.chainEmpty')}</p>
            ) : (
              <ApprovalChainTimeline
                lines={lines}
                actions={actions}
                currentLine={currentLine}
                status={instance.status}
                currentUserId={undefined}
                requesterName={instance.requester_id}
                t={t}
              />
            )}
          </div>

          {/* Terminal banner — make the closed state explicit (no dead flow). */}
          {isTerminal && (
            <div
              data-testid="terminal-banner"
              className={`flex items-center gap-2 rounded-lg border px-3.5 py-3 text-sm ${
                instance.status === InstanceStatus.APPROVED
                  ? 'border-ok-bd bg-ok-bg text-ok-tx'
                  : 'border-bad-bd bg-bad-bg text-bad-tx'
              }`}
            >
              {instance.status === InstanceStatus.APPROVED ? (
                <Check aria-hidden className="size-4 shrink-0" />
              ) : (
                <X aria-hidden className="size-4 shrink-0" />
              )}
              <span>
                {instance.status === InstanceStatus.APPROVED
                  ? t('approvals.detail.terminalApproved')
                  : t('approvals.detail.terminalRejected')}
              </span>
            </div>
          )}
        </div>
      </div>

      {/* BypassCard — super-admin only, on a still-pending instance (INV-5). */}
      {canBypass && isPending && (
        <div
          data-testid="bypass-card"
          className="flex items-center gap-3 rounded-xl border border-[var(--color-accent-purple,#8E0E8E)] bg-surface p-4"
        >
          <span className="flex size-[34px] shrink-0 items-center justify-center rounded-full bg-[#8E0E8E1A] text-[var(--color-accent-purple,#8E0E8E)]">
            <ShieldCheck aria-hidden className="size-[18px]" />
          </span>
          <div className="flex flex-1 flex-col gap-0.5">
            <p className="font-bold text-sm text-text">{t('approvals.detail.bypassCardTitle')}</p>
            <p className="text-xs leading-relaxed text-text-2">
              {t('approvals.detail.bypassCardBody')}
            </p>
          </div>
          <button
            type="button"
            data-testid="detail-bypass"
            onClick={() => setBypassOpen(true)}
            disabled={acting}
            className="rounded-lg border border-[var(--color-accent-purple,#8E0E8E)] bg-surface px-3.5 py-2 text-[13px] font-bold text-[var(--color-accent-purple,#8E0E8E)] hover:bg-[#8E0E8E0D] disabled:opacity-50"
          >
            {t('approvals.detail.bypassCardButton')}
          </button>
        </div>
      )}

      {/* Reject overlay (frame EnabP — comp/ModalReject) */}
      {isPending && (
        <RejectModal
          open={rejectOpen}
          onOpenChange={setRejectOpen}
          onConfirm={(reason) => reject.mutate({ id: instanceId, data: { reason } })}
          isPending={reject.isPending}
        />
      )}

      {/* Bypass overlay (frame KT3Jz) */}
      {canBypass && isPending && (
        <BypassModal
          open={bypassOpen}
          instanceId={instance.id}
          currentLine={currentLine}
          onOpenChange={setBypassOpen}
          onConfirm={(reason) => bypass.mutate({ id: instanceId, data: { reason } })}
          isPending={bypass.isPending}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// InfoRow
// ---------------------------------------------------------------------------

function InfoRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="shrink-0 text-sm text-text-3">{label}</span>
      <div className="text-right">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// RejectModal — frame EnabP (comp/ModalReject). Reason REQUIRED, sent to requester.
// Domain-agnostic to E11; mirrors the shared ModalReject recipe.
// ---------------------------------------------------------------------------

function RejectModal({
  open,
  onOpenChange,
  onConfirm,
  isPending,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (reason: string) => void;
  isPending: boolean;
}) {
  const { t } = useTranslation();
  const [reason, setReason] = useState('');
  const [error, setError] = useState('');

  function handleConfirm() {
    const trimmed = reason.trim();
    if (trimmed.length < 5) {
      setError(t('approvals.reject.reasonMinLength'));
      return;
    }
    setError('');
    onConfirm(trimmed);
  }

  function handleClose() {
    setReason('');
    setError('');
    onOpenChange(false);
  }

  return (
    <Modal open={open} onOpenChange={handleClose} size="sm">
      <ModalHeader icon={CornerUpLeft} title={t('approvals.reject.title')} tone="danger" />
      <ModalBody>
        <p className="mb-3 text-sm text-text-2">{t('approvals.reject.description')}</p>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="reject-reason" className="text-xs font-semibold text-text-2">
            {t('approvals.reject.reasonLabel')} *
          </label>
          <textarea
            id="reject-reason"
            data-testid="reject-reason-input"
            className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50"
            rows={4}
            placeholder={t('approvals.reject.reasonPlaceholder')}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={isPending}
          />
          {error && <p className="text-xs text-bad-tx">{error}</p>}
          <div className="flex items-center gap-1.5 text-xs text-text-3">
            <Info aria-hidden className="size-3.5" />
            {t('approvals.reject.auditNote')}
          </div>
        </div>
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={isPending}>
          {t('common.cancel')}
        </Button>
        <Button
          type="button"
          variant="destructive"
          data-testid="reject-confirm"
          onClick={handleConfirm}
          disabled={isPending}
        >
          {isPending ? t('common.processing') : t('approvals.reject.confirm')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
