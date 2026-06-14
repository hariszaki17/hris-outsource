/**
 * E11 · Approval chain-progress timeline + action trail.
 *
 * .pen frame: OHseV (E11 · Detail Permintaan — rantai persetujuan), ChainCard `p6pcsL`.
 * Specs: F11.2 / F11.3 · IB-4 (chain timeline) · INV-3 (self-approval) · INV-5 (bypass).
 *
 * Renders the resolved chain (`lines[]`) overlaid with the decision trail (`actions[]`):
 *   - CLEARED line   → solid teal node + check; "Selesai" pill; OR-clearer sentence
 *                      ("Disetujui oleh X …  · Y tidak perlu bertindak (OR).").
 *   - CURRENT line   → amber ringed node + line number; "Menunggu keputusan" pill;
 *                      the current user's chip highlighted as "Anda".
 *   - UPCOMING line  → muted neutral node + plain number; "Belum mulai" pill.
 *   - RIWAYAT TINDAKAN → every action row (APPROVE / REJECT / BYPASS), BYPASS in accent-purple.
 *
 * Local organism (domain-specific) — stays in the feature folder per ENGINEERING.md G1.
 */

import {
  type ApprovalAction,
  ApprovalActionAction,
  type ApprovalLine,
  InstanceStatus,
  type InstanceStatus as InstanceStatusType,
  type LineMember,
} from '@swp/api-client/e11';
import { DateText } from '@swp/ui';
import type { TFunction } from 'i18next';
import { Check, FilePlus, ShieldCheck, XCircle } from 'lucide-react';

interface ApprovalChainTimelineProps {
  lines: ApprovalLine[];
  actions: ApprovalAction[];
  /** 1-based line currently being decided. */
  currentLine: number;
  status: InstanceStatusType;
  /** Current user id (`SWP-USR-…`) — used to mark the "Anda" chip. */
  currentUserId?: string;
  /** Display label for the requester (for the submission trail row). */
  requesterName?: string;
  t: TFunction;
}

type LinePhase = 'cleared' | 'current' | 'upcoming';

function initials(name: string): string {
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0] ?? '')
    .join('')
    .toUpperCase();
}

function memberLabel(m: LineMember): string {
  return m.display_name?.trim() || m.user_id;
}

export function ApprovalChainTimeline({
  lines,
  actions,
  currentLine,
  status,
  currentUserId,
  requesterName,
  t,
}: ApprovalChainTimelineProps) {
  const sortedLines = [...lines].sort((a, b) => a.line_no - b.line_no);
  const isRejected = status === InstanceStatus.REJECTED;

  return (
    <div className="flex flex-col">
      {sortedLines.map((line, idx) => {
        const phase: LinePhase =
          line.line_no < currentLine || status === InstanceStatus.APPROVED
            ? 'cleared'
            : line.line_no === currentLine && status === InstanceStatus.PENDING
              ? 'current'
              : line.line_no === currentLine && isRejected
                ? 'current'
                : 'upcoming';

        // The action that cleared this line (the OR-approver).
        const clearer = actions.find(
          (a) => a.line_no === line.line_no && a.action === ApprovalActionAction.APPROVE,
        );

        // testid state: phase, except a current line on a REJECTED instance → 'rejected';
        // 'cleared' phase serializes to 'done' to match the data-state contract.
        const dataState =
          phase === 'current' && isRejected
            ? 'rejected'
            : phase === 'cleared'
              ? 'done'
              : phase;

        return (
          <ChainStep
            key={line.id || line.line_no}
            line={line}
            phase={phase}
            clearer={clearer}
            currentUserId={currentUserId}
            isLast={idx === sortedLines.length - 1}
            dataState={dataState}
            t={t}
          />
        );
      })}

      {/* RIWAYAT TINDAKAN — action trail */}
      <div className="flex flex-col gap-2.5 pt-5">
        <p className="font-semibold text-[11px] uppercase tracking-wide text-text-3">
          {t('approvals.detail.trailTitle')}
        </p>

        {/* Submission row (synthetic — the request creation precedes any action). */}
        <TrailRow icon={<FilePlus aria-hidden className="size-3.5 text-text-2" />}>
          {t('approvals.detail.trailSubmitted', { name: requesterName ?? '—' })}
        </TrailRow>

        {actions.length === 0 ? (
          <p className="text-xs text-text-3">{t('approvals.detail.trailEmpty')}</p>
        ) : (
          [...actions]
            .sort((a, b) => a.created_at.localeCompare(b.created_at))
            .map((a) => <ActionTrailRow key={a.id} action={a} t={t} />)
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// ChainStep — one line of the chain
// ---------------------------------------------------------------------------

function ChainStep({
  line,
  phase,
  clearer,
  currentUserId,
  isLast,
  dataState,
  t,
}: {
  line: ApprovalLine;
  phase: LinePhase;
  clearer?: ApprovalAction;
  currentUserId?: string;
  isLast: boolean;
  dataState: string;
  t: TFunction;
}) {
  const clearerName = clearer
    ? line.members.find((m) => m.user_id === clearer.actor_user_id)?.display_name?.trim() ||
      clearer.actor_name?.trim() ||
      clearer.actor_user_id
    : undefined;
  const others = clearer
    ? line.members.filter((m) => m.user_id !== clearer.actor_user_id).map(memberLabel)
    : [];

  return (
    <div
      className="flex gap-3.5 pt-4"
      data-testid={`chain-line-${line.line_no}`}
      data-state={dataState}
    >
      {/* Node + spine */}
      <div className="flex flex-col items-center">
        <StepNode phase={phase} lineNo={line.line_no} />
        {!isLast && <div className="mt-1 w-0.5 flex-1 bg-border" />}
      </div>

      {/* Body */}
      <div className="flex flex-1 flex-col gap-2.5 pb-1">
        <div className="flex items-center gap-2">
          <span className="font-bold text-sm text-text">
            {t('approvals.detail.lineLabel', { n: line.line_no })}
          </span>
          <StepPill phase={phase} t={t} />
        </div>

        {/* Member chips */}
        <div className="flex flex-wrap gap-2">
          {line.members.map((m) => {
            const isYou = Boolean(currentUserId) && m.user_id === currentUserId;
            return (
              <span
                key={m.user_id}
                className={`flex items-center gap-2 rounded-full py-1 pr-2.5 pl-1 ${
                  isYou
                    ? 'border border-primary bg-primary-soft'
                    : 'border border-border bg-surface-2'
                }`}
              >
                <span
                  className={`flex size-[22px] items-center justify-center rounded-full text-[9px] font-bold ${
                    isYou ? 'bg-primary text-white' : 'bg-primary-soft text-primary-strong'
                  }`}
                >
                  {initials(memberLabel(m))}
                </span>
                <span className="text-xs text-text">
                  {memberLabel(m)}
                  {isYou ? ` · ${t('approvals.detail.youSuffix')}` : ''}
                </span>
              </span>
            );
          })}
        </div>

        {/* Resolution / waiting sentence */}
        {phase === 'cleared' && clearer && (
          <p className="text-xs leading-relaxed text-ok-tx">
            {t('approvals.detail.lineClearedBy', {
              name: clearerName,
            })}{' '}
            <DateText kind="instant" value={clearer.created_at} className="text-ok-tx" />
            {others.length > 0 &&
              ` · ${t('approvals.detail.lineOthersSkipped', { names: others.join(', ') })}`}
          </p>
        )}
        {phase === 'current' && (
          <p className="text-xs leading-relaxed text-text-2">{t('approvals.detail.lineWaiting')}</p>
        )}
        {phase === 'upcoming' && (
          <p className="text-xs leading-relaxed text-text-3">
            {t('approvals.detail.lineUpcoming')}
          </p>
        )}
      </div>
    </div>
  );
}

function StepNode({ phase, lineNo }: { phase: LinePhase; lineNo: number }) {
  if (phase === 'cleared') {
    return (
      <span className="flex size-7 items-center justify-center rounded-full bg-ok-tx text-white">
        <Check aria-hidden className="size-[15px]" />
      </span>
    );
  }
  if (phase === 'current') {
    return (
      <span className="flex size-7 items-center justify-center rounded-full border-2 border-warn-tx bg-warn-bg font-bold text-[13px] text-warn-tx">
        {lineNo}
      </span>
    );
  }
  return (
    <span className="flex size-7 items-center justify-center rounded-full border border-border bg-surface-2 font-bold text-[13px] text-text-3">
      {lineNo}
    </span>
  );
}

function StepPill({ phase, t }: { phase: LinePhase; t: TFunction }) {
  if (phase === 'cleared') {
    return (
      <span className="rounded-full border border-ok-bd bg-ok-bg px-2 py-0.5 text-[11px] font-semibold text-ok-tx">
        {t('approvals.detail.pillCleared')}
      </span>
    );
  }
  if (phase === 'current') {
    return (
      <span className="rounded-full border border-warn-bd bg-warn-bg px-2 py-0.5 text-[11px] font-semibold text-warn-tx">
        {t('approvals.detail.pillCurrent')}
      </span>
    );
  }
  return (
    <span className="rounded-full border border-border bg-surface-2 px-2 py-0.5 text-[11px] font-semibold text-text-3">
      {t('approvals.detail.pillUpcoming')}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Action trail rows
// ---------------------------------------------------------------------------

function TrailRow({ icon, children }: { icon: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-2 text-xs leading-relaxed text-text-2">
      <span className="mt-0.5 shrink-0">{icon}</span>
      <span>{children}</span>
    </div>
  );
}

function ActionTrailRow({ action, t }: { action: ApprovalAction; t: TFunction }) {
  const actor = action.actor_name?.trim() || action.actor_user_id;

  if (action.action === ApprovalActionAction.BYPASS) {
    return (
      <div className="flex items-start gap-2 rounded-lg border border-[var(--color-accent-purple,#8E0E8E)] bg-[#8E0E8E1A] px-2.5 py-2 text-xs leading-relaxed text-[var(--color-accent-purple,#8E0E8E)]">
        <ShieldCheck aria-hidden className="mt-0.5 size-3.5 shrink-0" />
        <span>
          {t('approvals.detail.trailBypass', { name: actor })}{' '}
          <DateText
            kind="instant"
            value={action.created_at}
            className="text-[var(--color-accent-purple,#8E0E8E)]"
          />
          {action.reason ? ` — ${action.reason}` : ''}
        </span>
      </div>
    );
  }

  if (action.action === ApprovalActionAction.REJECT) {
    return (
      <TrailRow icon={<XCircle aria-hidden className="size-3.5 text-bad-tx" />}>
        {t('approvals.detail.trailRejected', { name: actor, line: action.line_no })}{' '}
        <DateText kind="instant" value={action.created_at} className="text-text-3" />
        {action.reason ? ` — ${action.reason}` : ''}
      </TrailRow>
    );
  }

  return (
    <TrailRow icon={<Check aria-hidden className="size-3.5 text-ok-tx" />}>
      {t('approvals.detail.trailApproved', { name: actor, line: action.line_no })}{' '}
      <DateText kind="instant" value={action.created_at} className="text-text-3" />
    </TrailRow>
  );
}
