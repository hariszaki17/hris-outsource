/**
 * E7 · Detail Lembur (HR Review) — full OT detail + decision UI.
 *
 * .pen frames implemented:
 *   uG6mQ   E7 · Detail Lembur (HR Review)           — main layout with all sections
 *   YGLK3   E7 · Overlays & States Showcase           — overlay variants
 *   STI8j   E7 · Wave 3.4 — Tarik kembali OT (agent) — withdraw modal pattern
 *
 * Route: /overtime/$overtimeId
 *
 * Status machine (D4 final 2026-06-02):
 *   PENDING_AGENT_CONFIRM → agent can confirm or withdraw
 *   PENDING_L1            → SL or HR can approve-L1 or reject
 *   PENDING_HR            → HR can approve-final or reject
 *   APPROVED | REJECTED | WITHDRAWN → terminal / read-only
 *
 * Calculation block (OvertimeCalculation) is server-computed at GET time —
 * this component renders it verbatim; NO client-side arithmetic.
 *
 * F7-refs: F7.1 (OT rules/tiers), F7.2 (auto-detect), F7.3 (OA agent actions),
 *          F7.4 (aggregations). Business rules: BR-OA-1..8, EPICS §8 D4.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type Overtime,
  OvertimeSource,
  OvertimeStatus,
  OvertimeTier,
  useApproveOvertimeFinal,
  useApproveOvertimeL1,
  useConfirmOvertime,
  useGetOvertime,
  useRejectOvertime,
  useWithdrawOvertime,
} from '@swp/api-client/e7';
import type { StatusTone } from '@swp/design-tokens';
import { Button, DateText, EmptyState, IdChip, StateView, StatusBadge, useToast } from '@swp/ui';
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
  const { toast } = useToast();
  const user = useCurrentUser();

  const isHR = user?.role === 'hr_admin' || user?.role === 'super_admin';
  const isSL = user?.role === 'shift_leader';
  const isAgent = user?.role === 'agent';

  // ---------------------------------------------------------------------------
  // Query
  // ---------------------------------------------------------------------------

  const query = useGetOvertime(overtimeId);
  // The handler wraps the detail in a {data:<Overtime>} envelope even though the E7
  // openapi declares the bare object (so Orval narrows query.data.data to Overtime).
  // Unwrap with a bare fallback (the recurring Phase-8 detail-GET fix): prefer the
  // nested .data when present, else treat the response body as the bare Overtime.
  const raw = query.data?.data as { data?: Overtime } | Overtime | undefined;
  const unwrapped =
    raw && typeof raw === 'object' && 'data' in raw && raw.data
      ? (raw as { data?: Overtime }).data
      : (raw as Overtime | undefined);
  const ot: Overtime | undefined =
    unwrapped && 'id' in unwrapped && 'status' in unwrapped ? (unwrapped as Overtime) : undefined;

  // ---------------------------------------------------------------------------
  // Overlay state
  // ---------------------------------------------------------------------------

  const [rejectOpen, setRejectOpen] = useState(false);
  const [withdrawOpen, setWithdrawOpen] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  // ---------------------------------------------------------------------------
  // Mutations
  // ---------------------------------------------------------------------------

  const approveL1 = useApproveOvertimeL1({
    mutation: {
      onSuccess: () => {
        toast({
          tone: 'success',
          title: t('detail.toastForwardedTitle'),
          description: t('detail.toastForwardedBody'),
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

  const approveFinal = useApproveOvertimeFinal({
    mutation: {
      onSuccess: () => {
        toast({
          tone: 'success',
          title: t('detail.toastApprovedTitle'),
          description: t('detail.toastApprovedBody'),
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

  const rejectMutation = useRejectOvertime({
    mutation: {
      onSuccess: () => {
        setRejectOpen(false);
        toast({
          tone: 'success',
          title: t('detail.toastRejectedTitle'),
          description: t('detail.toastRejectedBody'),
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

  // Action gating by status + role
  const canApproveL1 = (isSL || isHR) && ot.status === OvertimeStatus.PENDING_L1;
  const canApproveFinal = isHR && ot.status === OvertimeStatus.PENDING_HR;
  const canReject =
    ((isSL || isHR) && ot.status === OvertimeStatus.PENDING_L1) ||
    (isHR && ot.status === OvertimeStatus.PENDING_HR);
  // Agent can confirm or withdraw from PENDING_AGENT_CONFIRM
  const canConfirm = isAgent && ot.status === OvertimeStatus.PENDING_AGENT_CONFIRM;
  // Agent can withdraw from PENDING_L1 (before SL acts) — OA C-3
  const canWithdraw =
    isAgent &&
    (ot.status === OvertimeStatus.PENDING_AGENT_CONFIRM || ot.status === OvertimeStatus.PENDING_L1);

  const isTerminal =
    ot.status === OvertimeStatus.APPROVED ||
    ot.status === OvertimeStatus.REJECTED ||
    ot.status === OvertimeStatus.WITHDRAWN;

  const approvingL1 = approveL1.isPending;
  const approvingFinal = approveFinal.isPending;
  const rejecting = rejectMutation.isPending;
  const withdrawing = withdrawMutation.isPending;
  const confirming = confirmMutation.isPending;

  const anySaving = approvingL1 || approvingFinal || rejecting || withdrawing || confirming;

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  function handleApproveL1() {
    approveL1.mutate({ id: overtimeId, data: {} });
  }

  function handleApproveFinal() {
    approveFinal.mutate({ id: overtimeId, data: {} });
  }

  function handleRejectConfirm(reason: string) {
    rejectMutation.mutate({ id: overtimeId, data: { reason } });
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

      {/* ── Header card (uG6mQ › sd2Om) ── */}
      <div className="flex flex-col gap-4 rounded-xl border border-border bg-surface p-6">
        {/* Top row: employee + status */}
        <div className="flex items-start justify-between gap-6">
          {/* Left: employee info */}
          <div className="flex flex-col gap-2.5">
            {/* Employee row */}
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
            {/* Tier indicator badge */}
            <div className="flex items-center gap-1.5 rounded-full border border-border bg-surface-2 px-3 py-1.5 text-xs font-bold text-text">
              <TierIcon tier={ot.tier_indicator} />
              {t(overtimeTierKey(ot.tier_indicator))}
            </div>
            {/* Status badge */}
            <StatusBadge dot tone={overtimeStatusTone(ot.status)}>
              {t(`status.${ot.status}`)}
            </StatusBadge>
          </div>
        </div>

        {/* Worked-without-request flag banner (uG6mQ › Nz1Fg) */}
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

        {/* Meta grid: Pre-approval card + Source card + Duration card (uG6mQ › NIDHJ) */}
        <div className="grid grid-cols-3 gap-3.5">
          {/* Pre-approval */}
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

          {/* Source */}
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

          {/* Duration counted */}
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
          {/* Tier breakdown card (uG6mQ › dm6w3) */}
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
              {/* Footnote */}
              <p className="text-xs leading-relaxed text-text-3">{t('detail.tierFootnote')}</p>
            </div>
          </Section>

          {/* Attendance source card (uG6mQ › Ra0Hz) — only when AUTO_DETECTED */}
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

          {/* Notes / reason card (uG6mQ › S6CeO) */}
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
          {/* Approval timeline card (uG6mQ › F7PO5S) */}
          <Section title={t('detail.sectionTimeline')}>
            <OvertimeTimeline ot={ot} t={t} />
          </Section>

          {/* Action card (uG6mQ › qTCEX) — hidden when terminal */}
          {!isTerminal && (
            <div className="flex flex-col rounded-xl border border-border bg-surface">
              {/* Header */}
              <div className="border-b border-border px-[18px] py-3.5">
                <h2 className="text-sm font-bold text-text">
                  {isHR && ot.status === OvertimeStatus.PENDING_HR
                    ? t('detail.actionCardTitleHR')
                    : isSL && ot.status === OvertimeStatus.PENDING_L1
                      ? t('detail.actionCardTitleSL')
                      : isAgent
                        ? t('detail.actionCardTitleAgent')
                        : t('detail.actionCardTitle')}
                </h2>
              </div>

              {/* Body */}
              <div className="flex flex-col gap-3.5 px-[18px] py-3.5">
                {/* Decision lead text */}
                {(canApproveL1 || canApproveFinal) && (
                  <p className="text-xs leading-relaxed text-text-2">
                    {canApproveFinal ? t('detail.actionLeadHR') : t('detail.actionLeadSL')}
                  </p>
                )}

                {/* Agent confirmation hint */}
                {canConfirm && (
                  <p className="text-xs leading-relaxed text-text-2">
                    {t('detail.actionLeadAgent')}
                  </p>
                )}

                {/* Audit notice */}
                {(canApproveL1 || canApproveFinal || canReject) && (
                  <div className="flex items-start gap-2 rounded-lg bg-surface-2 px-3 py-2.5">
                    <ShieldCheck aria-hidden className="mt-0.5 size-3.5 shrink-0 text-text-3" />
                    <p className="text-[11px] leading-relaxed text-text-3">
                      {t('detail.auditNotice')}
                    </p>
                  </div>
                )}
              </div>

              {/* Footer with action buttons (uG6mQ › z736oS) */}
              <div className="flex items-center gap-2.5 border-t border-border bg-surface-2 px-[18px] py-3.5">
                {/* Reject */}
                {canReject && (
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={() => setRejectOpen(true)}
                    disabled={anySaving}
                  >
                    {rejecting ? t('common.processing') : t('detail.actionReject')}
                  </Button>
                )}

                {/* Spacer */}
                <div className="flex-1" />

                {/* SL approve-L1 */}
                {canApproveL1 && (
                  <Button
                    type="button"
                    variant="primary"
                    onClick={handleApproveL1}
                    disabled={anySaving}
                  >
                    {approvingL1 ? t('common.processing') : t('detail.actionApproveL1')}
                  </Button>
                )}

                {/* HR final approve */}
                {canApproveFinal && (
                  <Button
                    type="button"
                    variant="primary"
                    onClick={handleApproveFinal}
                    disabled={anySaving}
                  >
                    {approvingFinal ? t('common.processing') : t('detail.actionApproveFinal')}
                  </Button>
                )}

                {/* Agent confirm auto-detected */}
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

                {/* Agent withdraw */}
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

      {/* Audit footer (uG6mQ › CnDG3) */}
      <div className="flex items-center gap-2 py-2">
        <ShieldCheck aria-hidden className="size-3 text-text-3" />
        <span className="text-[11px] text-text-3">{t('detail.auditFooter', { id: ot.id })}</span>
      </div>

      {/* ── Overlays ── */}
      {canReject && (
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
// Section wrapper (matches uG6mQ card style with optional badge in header)
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
// MetaCard — small stat card used in the header grid (uG6mQ › NIDHJ)
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
// TierIcon — decorative icon per tier (design-system: not emoji, real icons)
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
// OvertimeTimeline (uG6mQ › F7PO5S › ZmvjV)
// ---------------------------------------------------------------------------

type TFunc = (key: string, opts?: Record<string, unknown>) => string;

function OvertimeTimeline({ ot, t }: { ot: Overtime; t: TFunc }) {
  const approvals = ot.approvals ?? [];

  // Build steps: synthetic "auto-detected" + agent confirm step (if applicable) + approvals
  type Step =
    | { kind: 'system'; label: string; sub: string; tone: StatusTone }
    | {
        kind: 'approval';
        level: number;
        decision: string;
        actorName?: string;
        at: string;
        reason?: string;
        tone: StatusTone;
      }
    | { kind: 'pending'; label: string };

  const steps: Step[] = [];

  // Step 0: system created / auto-detected
  steps.push({
    kind: 'system',
    label:
      ot.source === OvertimeSource.AUTO_DETECTED
        ? t('detail.timelineAutoDetected')
        : ot.source === OvertimeSource.WORKED_WITHOUT_REQUEST
          ? t('detail.timelineWorkedWithoutRequest')
          : t('detail.timelineRequested'),
    sub: ot.created_at,
    tone: 'info',
  });

  // Recorded approvals
  for (const ap of approvals) {
    const isApproved = ap.decision === 'APPROVED' || ap.decision === 'OVERRIDE_APPROVED';
    const isRejected = ap.decision === 'REJECTED';
    steps.push({
      kind: 'approval',
      level: ap.level,
      decision: ap.decision,
      actorName: ap.approver?.name ?? ap.approver?.id,
      at: ap.decided_at,
      reason: ap.reason != null ? String(ap.reason) : undefined,
      tone: isApproved ? 'ok' : isRejected ? 'bad' : 'neutral',
    });
  }

  // Pending next step (if not terminal)
  if (ot.status === OvertimeStatus.PENDING_AGENT_CONFIRM) {
    steps.push({ kind: 'pending', label: t('detail.timelinePendingAgentConfirm') });
  } else if (ot.status === OvertimeStatus.PENDING_L1) {
    steps.push({ kind: 'pending', label: t('detail.timelinePendingL1') });
  } else if (ot.status === OvertimeStatus.PENDING_HR) {
    steps.push({ kind: 'pending', label: t('detail.timelinePendingHR') });
  }

  const toneCircle: Record<StatusTone, string> = {
    ok: 'bg-ok-bg border-ok-bd',
    bad: 'bg-bad-bg border-bad-bd',
    warn: 'bg-warn-bg border-warn-bd',
    info: 'bg-info-bg border-info-bd',
    onprogress: 'bg-warn-bg border-warn-bd',
    neutral: 'bg-surface-2 border-border',
  };

  const toneIcon: Record<StatusTone, React.ReactNode> = {
    ok: <CheckCircle2 aria-hidden className="size-3 text-ok-tx" />,
    bad: <XCircle aria-hidden className="size-3 text-bad-tx" />,
    warn: <AlertTriangle aria-hidden className="size-3 text-warn-tx" />,
    info: <Sparkles aria-hidden className="size-3 text-info-tx" />,
    onprogress: <Clock aria-hidden className="size-3 text-warn-tx" />,
    neutral: <Clock aria-hidden className="size-3 text-text-3" />,
  };

  return (
    <ol className="flex flex-col gap-3.5">
      {steps.map((step, idx) => {
        const tone: StatusTone = step.kind === 'pending' ? 'warn' : step.tone;
        const isLast = idx === steps.length - 1;
        const stepKey =
          step.kind === 'approval'
            ? `approval-${step.level}-${step.at}`
            : step.kind === 'system'
              ? `system-${step.sub}`
              : `pending-${step.label}`;

        return (
          <li key={stepKey} className="flex gap-3">
            {/* Dot + connector line */}
            <div className="flex flex-col items-center">
              <div
                className={`flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded-full border ${toneCircle[tone]}`}
              >
                {step.kind === 'pending' ? (
                  <Clock aria-hidden className="size-3 text-warn-tx" />
                ) : (
                  toneIcon[tone]
                )}
              </div>
              {!isLast && <div className="mt-1 h-full min-h-[16px] w-0.5 bg-border-soft" />}
            </div>

            {/* Text content */}
            <div className="flex flex-1 flex-col gap-0.5 pb-1">
              {step.kind === 'system' && (
                <>
                  <span className="text-sm font-semibold text-text">{step.label}</span>
                  <DateText kind="instant" value={step.sub} className="text-xs text-text-2" />
                </>
              )}
              {step.kind === 'approval' && (
                <>
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-semibold text-text">
                      {step.level === 1 ? t('detail.timelineL1Label') : t('detail.timelineL2Label')}{' '}
                      <span
                        className={`text-xs font-semibold ${step.tone === 'ok' ? 'text-ok-tx' : step.tone === 'bad' ? 'text-bad-tx' : 'text-text-3'}`}
                      >
                        {t(`approvalDecision.${step.decision}`)}
                      </span>
                    </span>
                    <DateText kind="instant" value={step.at} className="text-xs text-text-3" />
                  </div>
                  {step.actorName && <span className="text-xs text-text-2">{step.actorName}</span>}
                  {step.reason && (
                    <p className="mt-1 rounded-lg bg-bad-bg px-2.5 py-2 text-xs leading-relaxed text-bad-tx">
                      {t('detail.rejectionReason')}: {step.reason}
                    </p>
                  )}
                </>
              )}
              {step.kind === 'pending' && <span className="text-sm text-text-2">{step.label}</span>}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

// ---------------------------------------------------------------------------
// TerminalCard — read-only summary shown in terminal states
// ---------------------------------------------------------------------------

function TerminalCard({ ot, t }: { ot: Overtime; t: TFunc }) {
  const isApproved = ot.status === OvertimeStatus.APPROVED;
  const isRejected = ot.status === OvertimeStatus.REJECTED;
  const isWithdrawn = ot.status === OvertimeStatus.WITHDRAWN;

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
        {ot.approvals
          ?.filter((a) => a.decision === 'REJECTED')
          .map((a) => (
            <p key={`rej-${a.level}`} className="text-xs leading-relaxed text-bad-tx">
              {a.reason != null ? String(a.reason) : t('detail.noReasonGiven')}
            </p>
          ))}
      </div>
    );
  }

  if (isWithdrawn) {
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
