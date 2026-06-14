/**
 * E7 · Detail Lembur (HR Review) — full OT detail + decision UI (E11 engine).
 *
 * Approval routing moved to the E11 configurable engine (EPICS §8 E11, 2026-06-14). This screen
 * reads `approval_instance_id` off the overtime record and:
 *   - renders the E11 approval chain (ApprovalChainTimeline, reused from features/e11-approvals),
 *   - performs approve / reject via useApproveApprovalInstance / useRejectApprovalInstance.
 * Self-approval + line membership are server-enforced (INV-3, ENGINEERING C1); the UI handles
 * 403 SELF_APPROVAL_FORBIDDEN + 409 LINE_ALREADY_CLEARED gracefully.
 *
 * Agent lifecycle actions (confirm / withdraw) are KEPT — those E7 hooks still exist.
 *
 * Status machine (E11): PENDING_AGENT_CONFIRM | PENDING | APPROVED | REJECTED | CANCELLED.
 *
 * Route: /overtime/$overtimeId
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { ApiError } from '@swp/api-client';
import {
  type Overtime,
  OvertimeSource,
  OvertimeStatus,
  OvertimeTier,
  useConfirmOvertime,
  useGetOvertime,
  useWithdrawOvertime,
} from '@swp/api-client/e7';
import {
  type ApprovalInstanceDetail,
  useApproveApprovalInstance,
  useGetApprovalInstance,
  useRejectApprovalInstance,
} from '@swp/api-client/e11';
import { Button, DateText, EmptyState, IdChip, StateView, StatusBadge, useToast } from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { Link as RouterLink } from '@tanstack/react-router';
import {
  AlertTriangle,
  ArrowUpRight,
  CheckCircle2,
  Clock,
  CornerUpLeft,
  Flag,
  Info,
  Link,
  ShieldCheck,
  Sparkles,
  Timer,
  XCircle,
} from 'lucide-react';
import type * as React from 'react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ApprovalChainTimeline } from '../e11-approvals/approval-chain-timeline.tsx';
import {
  ConfirmOvertimeModal,
  RejectOvertimeModal,
  WithdrawOvertimeModal,
} from './overtime-detail-overlays.tsx';
import {
  formatOtMinutes,
  overtimeSourceKey,
  overtimeSourceTone,
  overtimeStatusTone,
  overtimeTierKey,
  overtimeTierTone,
} from './overtime-shared.tsx';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface OvertimeDetailScreenProps {
  overtimeId: string;
}

// ---------------------------------------------------------------------------
// OvertimeDetailScreen
// ---------------------------------------------------------------------------

export function OvertimeDetailScreen({ overtimeId }: OvertimeDetailScreenProps) {
  const { t } = useTranslation('overtime');
  const { t: ta } = useTranslation('approvals');
  const { toast } = useToast();
  const user = useCurrentUser();
  const queryClient = useQueryClient();

  const isAgent = user?.role === 'agent';

  // ---------------------------------------------------------------------------
  // Query — overtime record
  // ---------------------------------------------------------------------------

  const query = useGetOvertime(overtimeId);
  const raw = query.data?.data as { data?: Overtime } | Overtime | undefined;
  const unwrapped =
    raw && typeof raw === 'object' && 'data' in raw && raw.data
      ? (raw as { data?: Overtime }).data
      : (raw as Overtime | undefined);
  const ot: Overtime | undefined =
    unwrapped && 'id' in unwrapped && 'status' in unwrapped ? (unwrapped as Overtime) : undefined;

  const instanceId = (ot?.approval_instance_id as string | null | undefined) ?? undefined;

  // ---------------------------------------------------------------------------
  // Query — E11 approval instance (the chain)
  // ---------------------------------------------------------------------------

  const instanceQuery = useGetApprovalInstance(instanceId ?? '', {
    query: { enabled: Boolean(instanceId) },
  });
  const instance = instanceQuery.data?.data as ApprovalInstanceDetail | undefined;

  // ---------------------------------------------------------------------------
  // Overlay state
  // ---------------------------------------------------------------------------

  const [rejectOpen, setRejectOpen] = useState(false);
  const [withdrawOpen, setWithdrawOpen] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  // ---------------------------------------------------------------------------
  // Mutations
  // ---------------------------------------------------------------------------

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

  const approve = useApproveApprovalInstance();
  const reject = useRejectApprovalInstance();

  const withdrawMutation = useWithdrawOvertime({
    mutation: {
      onSuccess: () => {
        setWithdrawOpen(false);
        toast({
          tone: 'success',
          title: t('detail.toastWithdrawnTitle'),
          description: t('detail.toastWithdrawnBody'),
        });
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

  const confirmMutation = useConfirmOvertime({
    mutation: {
      onSuccess: () => {
        setConfirmOpen(false);
        toast({
          tone: 'success',
          title: t('detail.toastConfirmedTitle'),
          description: t('detail.toastConfirmedBody'),
        });
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
  // Loading / error / not-found
  // ---------------------------------------------------------------------------

  if (query.isLoading) {
    return (
      <div className="flex flex-col gap-4 p-6 animate-pulse">
        <div className="h-8 w-64 rounded-lg bg-surface-2" />
        <div className="h-52 w-full rounded-xl bg-surface-2" />
        <div className="flex gap-5">
          <div className="h-72 flex-1 rounded-xl bg-surface-2" />
          <div className="h-72 w-[420px] rounded-xl bg-surface-2" />
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

  if (!ot) return null;

  // ---------------------------------------------------------------------------
  // Derived state
  // ---------------------------------------------------------------------------

  const calc = ot.calculation;

  const isWorkedWithoutRequest = ot.source === OvertimeSource.WORKED_WITHOUT_REQUEST;
  const isAutoDetected = ot.source === OvertimeSource.AUTO_DETECTED;
  const isCrossMidnight = Boolean(ot.cross_midnight);

  // Approve/reject (any line member) — server-enforced membership. Offered while PENDING with an
  // instance. Agent confirm/withdraw use the dedicated E7 lifecycle hooks.
  const canAct = ot.status === OvertimeStatus.PENDING && Boolean(instanceId) && !isAgent;
  const canConfirm = isAgent && ot.status === OvertimeStatus.PENDING_AGENT_CONFIRM;
  const canWithdraw =
    isAgent &&
    (ot.status === OvertimeStatus.PENDING_AGENT_CONFIRM || ot.status === OvertimeStatus.PENDING);

  const isTerminal =
    ot.status === OvertimeStatus.APPROVED ||
    ot.status === OvertimeStatus.REJECTED ||
    ot.status === OvertimeStatus.CANCELLED;

  const approving = approve.isPending;
  const rejecting = reject.isPending;
  const withdrawing = withdrawMutation.isPending;
  const confirming = confirmMutation.isPending;

  const anySaving = approving || rejecting || withdrawing || confirming;

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

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

  function handleWithdrawConfirm() {
    withdrawMutation.mutate({ id: overtimeId });
  }

  function handleConfirmOt(note?: string) {
    confirmMutation.mutate({ id: overtimeId, data: { note } });
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-4">
      {/* Breadcrumb row */}
      <div className="flex items-center gap-2">
        <div className="flex h-7 w-7 items-center justify-center rounded-lg border border-border bg-surface text-text-3">
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.5"
            aria-hidden="true"
          >
            <title>back</title>
            <polyline points="15 18 9 12 15 6" />
          </svg>
        </div>
        <span className="text-sm font-semibold text-text-2">{t('detail.breadcrumb')}</span>
      </div>

      {/* ── Header card ── */}
      <div className="flex flex-col gap-4 rounded-xl border border-border bg-surface p-6">
        {/* Top row: employee + status */}
        <div className="flex items-start justify-between gap-6">
          {/* Left: employee info */}
          <div className="flex flex-col gap-2.5">
            <div className="flex items-center gap-3.5">
              <div className="flex h-11 w-11 items-center justify-center rounded-full bg-primary-soft text-base font-bold text-primary">
                {(ot.employee.name ?? ot.employee.id)
                  .split(' ')
                  .slice(0, 2)
                  .map((p: string) => p[0] ?? '')
                  .join('')
                  .toUpperCase()}
              </div>
              <div className="flex flex-col gap-0.5">
                <span className="text-xl font-bold text-text">
                  {ot.employee.name ?? ot.employee.id}
                </span>
                <div className="flex items-center gap-2 text-xs">
                  <span className="font-mono text-text-3">{ot.employee.id}</span>
                  <span className="text-text-3">·</span>
                  <span className="text-text-2">{ot.company.name ?? ot.company.id}</span>
                </div>
              </div>
            </div>
            {/* Id + date + time line */}
            <div className="flex items-center gap-2.5 text-[13px]">
              <IdChip id={ot.id} />
              <span className="text-text-3">·</span>
              <span className="text-text-2">
                <DateText kind="date" value={ot.work_date} />
              </span>
              {(ot.actual_start_time ?? ot.planned_start_time) &&
                (ot.actual_end_time ?? ot.planned_end_time) && (
                  <>
                    <span className="text-text-3">·</span>
                    <span className="font-mono text-text-2">
                      {ot.actual_start_time ?? ot.planned_start_time} →{' '}
                      {ot.actual_end_time ?? ot.planned_end_time}
                    </span>
                    {isCrossMidnight && (
                      <span className="rounded-full bg-info-bg px-2 py-0.5 text-[10px] font-semibold text-info-tx">
                        +1
                      </span>
                    )}
                  </>
                )}
            </div>
          </div>

          {/* Right: tier badge + status pill */}
          <div className="flex flex-col items-end gap-2.5">
            <div className="flex items-center gap-1.5 rounded-full border border-border bg-surface-2 px-3 py-1.5 text-xs font-bold text-text">
              <TierIcon tier={ot.tier_indicator} />
              {t(overtimeTierKey(ot.tier_indicator))}
            </div>
            <StatusBadge dot tone={overtimeStatusTone(ot.status)}>
              {t(`status.${ot.status}`)}
            </StatusBadge>
          </div>
        </div>

        {/* Worked-without-request flag banner */}
        {isWorkedWithoutRequest && (
          <div className="flex items-start gap-2.5 rounded-xl border border-warn-bd bg-warn-bg px-3.5 py-3">
            <Flag aria-hidden className="mt-0.5 size-4 shrink-0 text-warn-tx" />
            <div className="flex flex-col gap-0.5">
              <span className="text-sm font-bold text-warn-tx">
                {t('detail.flagNoPreapprovalTitle')}
              </span>
              <span className="text-xs leading-relaxed text-warn-tx">
                {t('detail.flagNoPreapprovalBody')}
              </span>
            </div>
          </div>
        )}

        {/* Meta grid: Pre-approval card + Source card + Duration card */}
        <div className="grid grid-cols-3 gap-3.5">
          <MetaCard
            icon={<ShieldCheck aria-hidden className="size-3.5 text-text-3" />}
            label={t('detail.metaPreApproval')}
          >
            <span
              className={`text-base font-bold ${isWorkedWithoutRequest ? 'text-warn-tx' : 'text-text'}`}
            >
              {isWorkedWithoutRequest ? t('detail.noPreApproval') : t('detail.hasPreApproval')}
            </span>
            {isWorkedWithoutRequest && (
              <span className="text-xs font-semibold text-warn-tx">{t('detail.flaggedLabel')}</span>
            )}
          </MetaCard>

          <MetaCard
            icon={<Sparkles aria-hidden className="size-3.5 text-text-3" />}
            label={t('detail.metaSource')}
          >
            <div className="flex items-center gap-2">
              <span className="text-base font-bold text-text">
                {t(overtimeSourceKey(ot.source))}
              </span>
              <StatusBadge tone={overtimeSourceTone(ot.source)}>
                {t(`source.${ot.source}.short`)}
              </StatusBadge>
            </div>
            {ot.attendance_id && (
              <span className="font-mono text-xs text-text-3">{String(ot.attendance_id)}</span>
            )}
          </MetaCard>

          <MetaCard
            icon={<Timer aria-hidden className="size-3.5 text-text-3" />}
            label={t('detail.metaDuration')}
          >
            <div className="flex items-baseline gap-1.5">
              <span className="text-2xl font-bold text-primary">
                {formatOtMinutes(calc.counted_minutes)}
              </span>
              <span className="font-mono text-xs text-text-3">({calc.worked_minutes}m)</span>
            </div>
            <span
              className={`text-xs ${calc.skipped_too_short ? 'text-warn-tx font-semibold' : 'text-text-3'}`}
            >
              {calc.skipped_too_short
                ? t('detail.calcSkippedTooShort', { min: calc.min_minutes_threshold })
                : t('detail.calcMinThreshold', { min: calc.min_minutes_threshold })}
            </span>
          </MetaCard>
        </div>
      </div>

      {/* skipped_too_short global alert */}
      {calc.skipped_too_short && (
        <div className="flex items-start gap-2.5 rounded-xl border border-warn-bd bg-warn-bg px-4 py-3">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0 text-warn-tx" />
          <div className="flex flex-col gap-0.5">
            <span className="text-sm font-bold text-warn-tx">
              {t('detail.skippedTooShortTitle')}
            </span>
            <span className="text-xs leading-relaxed text-warn-tx">
              {t('detail.skippedTooShortBody', {
                worked: calc.worked_minutes,
                counted: calc.counted_minutes,
                min: calc.min_minutes_threshold,
              })}
            </span>
          </div>
        </div>
      )}

      {/* Two-column layout */}
      <div className="flex gap-[18px]">
        {/* ── Left column ── */}
        <div className="flex flex-1 flex-col gap-[18px]">
          {/* Tier breakdown card */}
          <Section
            title={t('detail.sectionTierBreakdown')}
            badge={
              calc.tier_breakdown.some((tb) => tb.supersedes) ? (
                <div className="flex items-center gap-1.5 rounded-full bg-info-bg px-2.5 py-1 text-[10px] font-bold text-info-tx">
                  <Info aria-hidden className="size-3" />
                  {t('detail.holidayWinsRule')}
                </div>
              ) : undefined
            }
          >
            <div className="flex flex-col gap-2.5">
              {calc.tier_breakdown.map((tb) => {
                const isSuperseded = Boolean(tb.supersedes);
                const isEffective = !isSuperseded;
                return (
                  <div
                    key={tb.tier}
                    className={`flex items-center gap-3.5 rounded-xl px-3 py-2.5 ${
                      isEffective
                        ? 'border-l-[3px] border-primary bg-primary-soft'
                        : 'bg-surface-2 opacity-60'
                    }`}
                  >
                    <div className="flex flex-1 flex-col gap-0.5">
                      <div className="flex items-center gap-2">
                        <StatusBadge tone={overtimeTierTone(tb.tier)}>
                          {t(overtimeTierKey(tb.tier))}
                        </StatusBadge>
                        {isSuperseded && (
                          <span className="text-[10px] font-semibold text-text-3">
                            {t('detail.supersededBy', {
                              tier: t(overtimeTierKey(tb.supersedes as OvertimeTier)),
                            })}
                          </span>
                        )}
                        {tb.overtime_rule_id && (
                          <span className="font-mono text-[10px] text-text-3">
                            {tb.overtime_rule_id}
                          </span>
                        )}
                      </div>
                      <span className="text-xs text-text-3">
                        ×{tb.multiplier.toFixed(1)} {t('detail.multiplierRef')}
                      </span>
                    </div>
                    <span
                      className={`font-mono text-sm font-bold ${isEffective ? 'text-text' : 'text-text-3 line-through'}`}
                    >
                      {formatOtMinutes(tb.minutes)}
                    </span>
                  </div>
                );
              })}
              <p className="text-xs leading-relaxed text-text-3">{t('detail.tierFootnote')}</p>
            </div>
          </Section>

          {/* Attendance source card — only when AUTO_DETECTED */}
          {isAutoDetected && ot.attendance_id && (
            <Section title={t('detail.sectionAttendanceSource')}>
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-3">
                  <div className="flex h-9 w-9 items-center justify-center rounded-lg border border-info-bd bg-info-bg">
                    <Link aria-hidden className="size-4 text-info-tx" />
                  </div>
                  <div className="flex flex-col gap-0.5">
                    <span className="font-mono text-sm font-bold text-text">
                      {String(ot.attendance_id)}
                    </span>
                    {(ot.actual_start_time ?? ot.planned_start_time) && (
                      <span className="text-xs text-text-2">
                        {t('detail.attendanceSourceSub', {
                          company: ot.company.name ?? ot.company.id,
                          start: ot.actual_start_time ?? ot.planned_start_time,
                          end: ot.actual_end_time ?? ot.planned_end_time,
                        })}
                      </span>
                    )}
                  </div>
                </div>
                <a
                  href={`/attendance/${String(ot.attendance_id)}`}
                  className="flex items-center gap-1.5 rounded-lg border border-border bg-surface px-3 py-1.5 text-xs font-semibold text-text-2 hover:bg-surface-2"
                >
                  {t('detail.openAttendance')}
                  <ArrowUpRight aria-hidden className="size-3.5" />
                </a>
              </div>
            </Section>
          )}

          {/* Notes / reason card */}
          {ot.reason && (
            <Section title={t('detail.sectionNotes')}>
              <blockquote className="rounded-lg bg-surface-2 px-4 py-3 text-sm italic leading-relaxed text-text-2">
                {String(ot.reason)}
              </blockquote>
            </Section>
          )}
        </div>

        {/* ── Right column ── */}
        <div className="flex w-[420px] flex-col gap-[18px]">
          {/* E11 approval chain */}
          <Section
            title={t('detail.sectionTimeline')}
            badge={
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
                requesterName={ot.employee.name ?? ot.employee.id}
                t={ta}
              />
            ) : (
              <p className="text-sm text-text-3">{ta('detail.chainEmpty')}</p>
            )}
          </Section>

          {/* Action card — hidden when terminal */}
          {!isTerminal && (canAct || canConfirm || canWithdraw) && (
            <div className="flex flex-col rounded-xl border border-border bg-surface">
              <div className="border-b border-border px-[18px] py-3.5">
                <h2 className="text-sm font-bold text-text">
                  {isAgent ? t('detail.actionCardTitleAgent') : t('detail.actionCardTitle')}
                </h2>
              </div>

              <div className="flex flex-col gap-3.5 px-[18px] py-3.5">
                {canAct && (
                  <p className="text-xs leading-relaxed text-text-2">{t('detail.actionLeadHR')}</p>
                )}
                {canConfirm && (
                  <p className="text-xs leading-relaxed text-text-2">
                    {t('detail.actionLeadAgent')}
                  </p>
                )}
                {canAct && (
                  <div className="flex items-start gap-2 rounded-lg bg-surface-2 px-3 py-2.5">
                    <ShieldCheck aria-hidden className="mt-0.5 size-3.5 shrink-0 text-text-3" />
                    <p className="text-[11px] leading-relaxed text-text-3">
                      {t('detail.auditNotice')}
                    </p>
                  </div>
                )}
              </div>

              <div className="flex items-center gap-2.5 border-t border-border bg-surface-2 px-[18px] py-3.5">
                {canAct && (
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={() => setRejectOpen(true)}
                    disabled={anySaving}
                  >
                    {rejecting ? t('common.processing') : t('detail.actionReject')}
                  </Button>
                )}

                <div className="flex-1" />

                {canAct && (
                  <Button
                    type="button"
                    variant="primary"
                    onClick={handleApprove}
                    disabled={anySaving}
                  >
                    {approving ? t('common.processing') : t('detail.actionApprove')}
                  </Button>
                )}

                {canConfirm && (
                  <Button
                    type="button"
                    variant="primary"
                    onClick={() => setConfirmOpen(true)}
                    disabled={anySaving}
                  >
                    {t('detail.actionConfirm')}
                  </Button>
                )}

                {canWithdraw && (
                  <Button
                    type="button"
                    variant="destructive"
                    onClick={() => setWithdrawOpen(true)}
                    disabled={anySaving}
                  >
                    <CornerUpLeft aria-hidden className="size-3.5" />
                    {withdrawing ? t('common.processing') : t('detail.actionWithdraw')}
                  </Button>
                )}
              </div>
            </div>
          )}

          {/* Terminal state — show contextual summary card */}
          {isTerminal && <TerminalCard ot={ot} t={t} />}
        </div>
      </div>

      {/* Audit footer */}
      <div className="flex items-center gap-2 py-2">
        <ShieldCheck aria-hidden className="size-3 text-text-3" />
        <span className="text-[11px] text-text-3">{t('detail.auditFooter', { id: ot.id })}</span>
      </div>

      {/* ── Overlays ── */}
      {canAct && (
        <RejectOvertimeModal
          open={rejectOpen}
          overtime={ot}
          onOpenChange={setRejectOpen}
          onConfirm={handleRejectConfirm}
          isPending={rejecting}
        />
      )}

      {canWithdraw && (
        <WithdrawOvertimeModal
          open={withdrawOpen}
          overtime={ot}
          onOpenChange={setWithdrawOpen}
          onConfirm={handleWithdrawConfirm}
          isPending={withdrawing}
        />
      )}

      {canConfirm && (
        <ConfirmOvertimeModal
          open={confirmOpen}
          overtime={ot}
          onOpenChange={setConfirmOpen}
          onConfirm={handleConfirmOt}
          isPending={confirming}
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
  badge,
  children,
}: {
  title: string;
  badge?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col rounded-xl border border-border bg-surface">
      <div className="flex items-center justify-between border-b border-border px-[18px] py-3.5">
        <h2 className="text-sm font-bold text-text">{title}</h2>
        {badge}
      </div>
      <div className="px-[18px] py-3.5">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// MetaCard
// ---------------------------------------------------------------------------

function MetaCard({
  icon,
  label,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2 rounded-xl border border-border-soft bg-surface-2 px-3.5 py-3">
      <div className="flex items-center gap-1.5">
        {icon}
        <span className="text-[11px] font-bold uppercase tracking-wide text-text-3">{label}</span>
      </div>
      <div className="flex flex-col gap-0.5">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// TierIcon
// ---------------------------------------------------------------------------

function TierIcon({ tier }: { tier: OvertimeTier }) {
  if (tier === OvertimeTier.HOLIDAY) {
    return <Sparkles aria-hidden className="size-3.5 text-info-tx" />;
  }
  if (tier === OvertimeTier.RESTDAY) {
    return <Clock aria-hidden className="size-3.5 text-warn-tx" />;
  }
  return <Timer aria-hidden className="size-3.5 text-text-2" />;
}

// ---------------------------------------------------------------------------
// TerminalCard — read-only summary shown in terminal states
// ---------------------------------------------------------------------------

type TFunc = (key: string, opts?: Record<string, unknown>) => string;

function TerminalCard({ ot, t }: { ot: Overtime; t: TFunc }) {
  const isApproved = ot.status === OvertimeStatus.APPROVED;
  const isRejected = ot.status === OvertimeStatus.REJECTED;
  const isCancelled = ot.status === OvertimeStatus.CANCELLED;

  if (isApproved) {
    return (
      <div className="flex flex-col gap-3 rounded-xl border border-ok-bd bg-ok-bg px-4 py-4">
        <div className="flex items-center gap-2">
          <CheckCircle2 aria-hidden className="size-4 text-ok-tx" />
          <span className="text-sm font-bold text-ok-tx">{t('detail.terminalApprovedTitle')}</span>
        </div>
        <p className="text-xs leading-relaxed text-ok-tx">
          {t('detail.terminalApprovedBody', {
            hours: formatOtMinutes(ot.calculation.counted_minutes),
            tier: t(overtimeTierKey(ot.tier_indicator)),
          })}
        </p>
      </div>
    );
  }

  if (isRejected) {
    return (
      <div className="flex flex-col gap-2 rounded-xl border border-bad-bd bg-bad-bg px-4 py-4">
        <div className="flex items-center gap-2">
          <XCircle aria-hidden className="size-4 text-bad-tx" />
          <span className="text-sm font-bold text-bad-tx">{t('detail.terminalRejectedTitle')}</span>
        </div>
        <p className="text-xs leading-relaxed text-bad-tx">{t('detail.terminalRejectedBody')}</p>
      </div>
    );
  }

  if (isCancelled) {
    return (
      <div className="flex flex-col gap-2 rounded-xl border border-border bg-surface-2 px-4 py-4 opacity-75">
        <div className="flex items-center gap-2">
          <CornerUpLeft aria-hidden className="size-4 text-text-2" />
          <span className="text-sm font-semibold text-text-2">
            {t('detail.terminalWithdrawnTitle')}
          </span>
        </div>
        <p className="text-xs text-text-3">{t('detail.terminalWithdrawnBody')}</p>
      </div>
    );
  }

  return null;
}
