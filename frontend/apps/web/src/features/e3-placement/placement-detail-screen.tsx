/**
 * E3 · Detail Penempatan
 *
 * .pen frame: pFR79  "E3 · Detail Penempatan"  (1440 × 1024)
 *
 * Layout (from frame):
 *   AppShell (Sidebar + Topbar)
 *   Content (gap-16, p-24):
 *     Header card (wKzax) — Avatar · Name · StatusBadge · subtitle | action buttons
 *     LifecycleTracker (AiyOw) — horizontal step rail + end-date note
 *     Cols (gap-24):
 *       Left (w-760):
 *         DetailPenempatan card  (K6g53 / xw0YU) — KV grid
 *         RiwayatChain card     (K6g53 / uH4M7) — history_chain from API
 *         AuditTrailInline
 *       Right (fill):
 *         PerjanjianKerja card  (NhP21 / dsbMa)
 *         ShiftLeaderCard       (NhP21 / g3pGV) — INV-2/3/4 assignment + actions
 *
 * Data: `useGetPlacement(id)` returns `{ data: PlacementDetailResponse }` with
 *   `.placement`, `.history_chain[]`, `.current_shift_leader?`.
 *   Mutations use `{ id, data }` variable shape (confirmed from gen).
 *
 * Lifecycle states (F3.2):
 *   PENDING_START → info
 *   ACTIVE / EXTENDED → ok
 *   EXPIRING → warn
 *   ENDED / TRANSFERRED / SUPERSEDED → neutral (terminal)
 *   TERMINATED / RESIGNED → bad (terminal)
 *
 * Shift-leader states (INV-2/3/4):
 *   no leader + active → warn banner + "Tetapkan" button
 *   leader present     → avatar row + "Ganti" | "Akhiri" buttons
 *   terminal           → read-only display
 *
 * i18n namespace: placementDetail
 * Cross-epic: F3 INV-1..4 (FEATURE.md §4)
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type Placement,
  type PlacementDetailResponse,
  PlacementLifecycleStatus,
  type PlacementSummary,
  type ShiftLeaderAssignmentSummary,
  useGetPlacement,
} from '@swp/api-client/e3';
import {
  type AuditEntryCompact,
  AuditTrailInline,
  Banner,
  Button,
  DateText,
  EmptyState,
  IdChip,
  StateView,
  StatusBadge,
  toneForPlacement,
} from '@swp/ui';
import { Link } from '@tanstack/react-router';
import {
  ArrowLeftRight,
  ArrowUpRight,
  Briefcase,
  CalendarCheck,
  FileText,
  RefreshCw,
  SquareX,
  Users,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  EndConfirm,
  RenewModal,
  ResignModal,
  TerminateConfirm,
  TransferModal,
} from './placement-overlays.tsx';
import type { PlacementInfo } from './placement-overlays.tsx';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface PlacementDetailScreenProps {
  placementId: string;
}

// ---------------------------------------------------------------------------
// Lifecycle → derived state
// ---------------------------------------------------------------------------

type ActiveState = 'active-like' | 'expiring' | 'pending' | 'terminal-neutral' | 'terminal-bad';

function lifecycleState(status: PlacementLifecycleStatus): ActiveState {
  switch (status) {
    case PlacementLifecycleStatus.ACTIVE:
    case PlacementLifecycleStatus.EXTENDED:
      return 'active-like';
    case PlacementLifecycleStatus.EXPIRING:
      return 'expiring';
    case PlacementLifecycleStatus.PENDING_START:
      return 'pending';
    case PlacementLifecycleStatus.ENDED:
    case PlacementLifecycleStatus.TRANSFERRED:
    case PlacementLifecycleStatus.SUPERSEDED:
      return 'terminal-neutral';
    case PlacementLifecycleStatus.TERMINATED:
    case PlacementLifecycleStatus.RESIGNED:
      return 'terminal-bad';
    default:
      return 'terminal-neutral';
  }
}

function isTerminal(state: ActiveState): boolean {
  return state === 'terminal-neutral' || state === 'terminal-bad';
}

function canShowActions(state: ActiveState): boolean {
  return state === 'active-like' || state === 'expiring';
}

// Lifecycle step display (.pen AiyOw ILsx8) — 4-step rail
const LIFECYCLE_STEPS: Array<{ forStatuses: PlacementLifecycleStatus[]; label: string }> = [
  { forStatuses: [PlacementLifecycleStatus.PENDING_START], label: 'Draft' },
  {
    forStatuses: [PlacementLifecycleStatus.ACTIVE, PlacementLifecycleStatus.EXTENDED],
    label: 'Aktif',
  },
  { forStatuses: [PlacementLifecycleStatus.EXPIRING], label: 'Akan berakhir' },
  {
    forStatuses: [
      PlacementLifecycleStatus.ENDED,
      PlacementLifecycleStatus.TRANSFERRED,
      PlacementLifecycleStatus.TERMINATED,
      PlacementLifecycleStatus.RESIGNED,
      PlacementLifecycleStatus.SUPERSEDED,
    ],
    label: 'Berakhir',
  },
];

// Bahasa display labels per status
const STATUS_LABEL: Record<PlacementLifecycleStatus, string> = {
  [PlacementLifecycleStatus.PENDING_START]: 'Menunggu mulai',
  [PlacementLifecycleStatus.ACTIVE]: 'Aktif',
  [PlacementLifecycleStatus.EXTENDED]: 'Diperpanjang',
  [PlacementLifecycleStatus.EXPIRING]: 'Akan berakhir',
  [PlacementLifecycleStatus.ENDED]: 'Berakhir',
  [PlacementLifecycleStatus.TRANSFERRED]: 'Ditransfer',
  [PlacementLifecycleStatus.TERMINATED]: 'Dipecat',
  [PlacementLifecycleStatus.RESIGNED]: 'Mengundurkan diri',
  [PlacementLifecycleStatus.SUPERSEDED]: 'Superseded',
};

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function KvRow({
  label,
  value,
  children,
}: {
  label: string;
  value?: string | null;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border-soft py-[11px] last:border-b-0">
      <span className="text-[13px] text-text-3">{label}</span>
      {children ?? <span className="text-[13px] font-semibold text-text">{value ?? '—'}</span>}
    </div>
  );
}

function DetailCard({
  title,
  icon,
  children,
  action,
}: {
  title: string;
  icon?: React.ReactNode;
  children: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-border bg-surface">
      <div className="flex items-center justify-between border-b border-border-soft px-[18px] py-[13px]">
        <div className="flex items-center gap-2">
          {icon}
          <span className="text-[14px] font-bold text-text">{title}</span>
        </div>
        {action}
      </div>
      <div className="px-[18px] py-[4px] pb-[14px]">{children}</div>
    </div>
  );
}

function Initials({ name }: { name: string }) {
  return (
    <>
      {name
        .split(' ')
        .slice(0, 2)
        .map((p) => (p[0] ?? '').toUpperCase())
        .join('')}
    </>
  );
}

// ---------------------------------------------------------------------------
// PlacementDetailScreen
// ---------------------------------------------------------------------------

export function PlacementDetailScreen({ placementId }: PlacementDetailScreenProps) {
  const { t } = useTranslation('placementDetail');
  const currentUser = useCurrentUser();
  // Lifecycle mutations are HR/Admin-only (F3.2/F3.3). SL has placements.read only.
  const canEdit = currentUser?.permissions.includes('placements.write') ?? false;
  // The leader-management link targets /client-companies/$id, which needs clients.read.
  const canManageOnCompany = currentUser?.permissions.includes('clients.read') ?? false;

  // Overlay state
  const [showTransfer, setShowTransfer] = useState(false);
  const [showRenew, setShowRenew] = useState(false);
  const [showEnd, setShowEnd] = useState(false);
  const [showTerminate, setShowTerminate] = useState(false);
  const [showResign, setShowResign] = useState(false);

  // Data — `query.data?.data` is `PlacementDetailResponse`
  const placementQuery = useGetPlacement(placementId);
  const detailResponse = placementQuery.data?.data as PlacementDetailResponse | undefined;
  const placement = detailResponse?.placement;
  const historyChain: PlacementSummary[] = detailResponse?.history_chain ?? [];
  const currentLeader: ShiftLeaderAssignmentSummary | null | undefined =
    detailResponse?.current_shift_leader;

  // ---------------------------------------------------------------------------
  // Loading / error states (no dead-flow per ENGINEERING.md B2)
  // ---------------------------------------------------------------------------

  if (placementQuery.isLoading) {
    return <StateView kind="loading" title={t('loading.title')} />;
  }

  if (placementQuery.isError || !placement) {
    const classified = classifyError(placementQuery.error);
    if (classified.kind === 'not-found') {
      return (
        <StateView
          kind="empty"
          title={t('notFound.title')}
          description={t('notFound.description')}
        />
      );
    }
    // Scope 403: the role can open the screen but this placement row is outside
    // their server-side scope — render a no-permission empty state, no retry.
    if (classified.kind === 'forbidden' || classified.kind === 'unauthenticated') {
      return (
        <EmptyState
          variant="no-permission"
          title={t('forbidden.title')}
          description={t('forbidden.description')}
        />
      );
    }
    return (
      <StateView
        kind="error"
        title={t('error.title')}
        description={classified.message}
        onRetry={() => void placementQuery.refetch()}
        retryLabel={t('error.retry')}
      />
    );
  }

  // ---------------------------------------------------------------------------
  // Derived state
  // ---------------------------------------------------------------------------

  const lcState = lifecycleState(placement.lifecycle_status);
  const terminal = isTerminal(lcState);
  const showActions = canShowActions(lcState);

  const placementInfo: PlacementInfo = {
    id: placement.id,
    employee_name: placement.employee_name ?? placement.employee_id,
    client_company_id: placement.client_company_id,
    client_company_name: placement.client_company_name ?? placement.client_company_id,
    service_line_name: placement.service_line_name ?? placement.service_line_id,
    position_name: placement.position_name ?? placement.position_id,
    start_date: placement.start_date,
    end_date: placement.end_date,
  };

  // Compact audit entries derived from placement record
  const auditEntries: AuditEntryCompact[] = [
    {
      id: `create-${placement.id}`,
      type: 'created',
      actor: placement.created_by ?? 'system',
      verb: t('audit.created'),
      time: placement.created_at,
    },
    ...(placement.status_changed_at &&
    placement.lifecycle_status !== PlacementLifecycleStatus.PENDING_START
      ? [
          {
            id: `status-${placement.id}`,
            type: 'updated' as const,
            actor: placement.created_by ?? 'system',
            verb: `${t('audit.statusChanged')} → ${STATUS_LABEL[placement.lifecycle_status]}`,
            time: placement.status_changed_at,
          },
        ]
      : []),
  ];

  // NO_SHIFT_LEADER_AT_COMPANY soft warning from the API
  const noLeaderWarning = placement.warnings?.includes('NO_SHIFT_LEADER_AT_COMPANY') ?? false;

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto bg-app-bg p-6">
      {/* Header card (.pen wKzax) */}
      <div className="flex items-center justify-between gap-4 rounded-xl border border-border bg-surface p-5">
        <div className="flex items-center gap-3.5">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-full bg-primary-soft text-base font-bold text-primary">
            <Initials name={placement.employee_name ?? '?'} />
          </div>
          <div className="flex flex-col gap-[3px]">
            <div className="flex items-center gap-2.5">
              <span className="text-[20px] font-bold text-text">
                {placement.employee_name ?? placement.employee_id}
              </span>
              <StatusBadge tone={toneForPlacement(placement.lifecycle_status)} dot>
                {STATUS_LABEL[placement.lifecycle_status]}
              </StatusBadge>
            </div>
            <span className="text-[13px] text-text-2">
              {[
                placement.client_company_name,
                placement.service_line_name,
                placement.position_name,
                placement.id,
              ]
                .filter(Boolean)
                .join(' · ')}
            </span>
          </div>
        </div>

        {/* Action buttons (.pen waTmo) — Perpanjang · Transfer · Akhiri.
            HR/Admin-only (placements.write); hidden for shift_leader. */}
        {showActions && canEdit && (
          <div className="flex items-center gap-2.5">
            <Button type="button" variant="primary" size="sm" onClick={() => setShowRenew(true)}>
              <RefreshCw className="mr-1.5 size-4" aria-hidden="true" />
              {t('action.renew')}
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => setShowTransfer(true)}
            >
              <ArrowLeftRight className="mr-1.5 size-4" aria-hidden="true" />
              {t('action.transfer')}
            </Button>
            <Button type="button" variant="destructive" size="sm" onClick={() => setShowEnd(true)}>
              <SquareX className="mr-1.5 size-4" aria-hidden="true" />
              {t('action.end')}
            </Button>
          </div>
        )}
      </div>

      {/* Lifecycle tracker (.pen AiyOw) */}
      <div className="flex items-center justify-between gap-8 rounded-xl border border-border bg-surface px-6 py-[18px]">
        <div className="flex flex-1 items-center">
          {LIFECYCLE_STEPS.map((step, idx) => {
            const currentStepIdx = LIFECYCLE_STEPS.findIndex((s) =>
              s.forStatuses.includes(placement.lifecycle_status),
            );
            const stepIdx = idx;
            const isPast = currentStepIdx > stepIdx;
            const isCurrent = currentStepIdx === stepIdx;

            return (
              <div key={step.label} className="flex flex-1 items-center">
                {idx > 0 && (
                  <div
                    className={[
                      'h-[2px] flex-1 rounded-full',
                      isPast || isCurrent ? 'bg-primary' : 'bg-border',
                    ].join(' ')}
                    aria-hidden="true"
                  />
                )}
                <div className="flex shrink-0 items-center gap-2">
                  <div
                    className={[
                      'flex shrink-0 items-center justify-center rounded-full',
                      isCurrent
                        ? 'size-[18px] border-[3px] border-primary bg-primary'
                        : isPast
                          ? 'size-4 bg-primary'
                          : 'size-4 border border-border bg-surface-2',
                    ].join(' ')}
                    aria-hidden="true"
                  />
                  <span
                    className={[
                      'text-[13px]',
                      isCurrent
                        ? 'font-bold text-text'
                        : isPast
                          ? 'font-semibold text-text'
                          : 'font-medium text-text-3',
                    ].join(' ')}
                  >
                    {step.label}
                  </span>
                </div>
              </div>
            );
          })}
        </div>

        {/* End-date note (.pen YKkQB) */}
        {placement.end_date && (
          <div className="flex shrink-0 flex-col items-end gap-[2px]">
            <span className="text-[13px] font-semibold text-text">
              {t('lifecycle.endsOn')} <DateText kind="date" value={placement.end_date} />
            </span>
            {placement.lifecycle_status === PlacementLifecycleStatus.EXPIRING && (
              <span className="text-[12px] text-warn-tx">{t('lifecycle.expiringSoon')}</span>
            )}
          </div>
        )}
      </div>

      {/* Terminal banner (no dead-flow: every terminal state has a banner) */}
      {terminal && (
        <Banner
          tone={lcState === 'terminal-bad' ? 'bad' : 'info'}
          title={t(`terminalBanner.${placement.lifecycle_status}.title` as const)}
          description={
            placement.lifecycle_status === PlacementLifecycleStatus.SUPERSEDED &&
            placement.successor_id
              ? t('terminalBanner.SUPERSEDED.withSuccessor', { id: placement.successor_id })
              : placement.lifecycle_status === PlacementLifecycleStatus.TERMINATED
                ? (placement.termination_reason ?? undefined)
                : placement.lifecycle_status === PlacementLifecycleStatus.RESIGNED &&
                    placement.resign_at
                  ? t('terminalBanner.RESIGNED.withDate', { date: placement.resign_at })
                  : undefined
          }
        />
      )}

      {/* Secondary actions (Terminate / Resign) — ghost links, active-like +
          expiring only. HR/Admin-only (placements.write). */}
      {showActions && canEdit && (
        <div className="flex items-center justify-end gap-3">
          <button
            type="button"
            className="text-[13px] font-semibold text-text-3 underline-offset-2 hover:text-bad-tx hover:underline"
            onClick={() => setShowTerminate(true)}
          >
            {t('action.terminate')}
          </button>
          <span className="text-text-3" aria-hidden="true">
            ·
          </span>
          <button
            type="button"
            className="text-[13px] font-semibold text-text-3 underline-offset-2 hover:text-warn-tx hover:underline"
            onClick={() => setShowResign(true)}
          >
            {t('action.resign')}
          </button>
        </div>
      )}

      {/* 2-column body (.pen JSKXn) */}
      <div className="flex min-h-0 items-start gap-6">
        {/* Left column — 760px wide (.pen K6g53) */}
        <div className="flex w-[760px] shrink-0 flex-col gap-4">
          {/* Detail Penempatan card (.pen xw0YU) */}
          <DetailCard
            title={t('card.placementDetail')}
            icon={<Briefcase className="size-3.5 text-text-2" aria-hidden="true" />}
          >
            <div className="grid grid-cols-2 gap-x-10">
              <KvRow label={t('field.company')} value={placement.client_company_name} />
              <KvRow label={t('field.serviceLine')} value={placement.service_line_name} />
              <KvRow label={t('field.position')} value={placement.position_name} />
              <KvRow label={t('field.period')}>
                <span className="text-[13px] font-semibold text-text">
                  <DateText kind="date" value={placement.start_date} />
                  {' – '}
                  {placement.end_date ? (
                    <DateText kind="date" value={placement.end_date} />
                  ) : (
                    t('field.openEnded')
                  )}
                </span>
              </KvRow>
              <KvRow label={t('field.createdBy')} value={placement.created_by ?? '—'} />
              <KvRow label={t('field.createdAt')}>
                <DateText
                  kind="instant"
                  value={placement.created_at}
                  className="text-[13px] font-semibold text-text"
                />
              </KvRow>
            </div>
            {placement.notes && (
              <div className="mt-3 rounded-lg border border-border-soft bg-surface-2 px-4 py-3">
                <p className="text-[12px] text-text-3">{placement.notes}</p>
              </div>
            )}
          </DetailCard>

          {/* Chain / history card (.pen uH4M7) — uses history_chain from API */}
          <PlacementChainCard historyChain={historyChain} currentId={placement.id} />

          {/* AuditTrailInline (.pen comp/AuditTrailInline qtz6q) */}
          <AuditTrailInline
            title={t('audit.title')}
            entries={auditEntries}
            viewAllLabel={t('audit.viewAll')}
          />
        </div>

        {/* Right column — fill_container (.pen NhP21) */}
        <div className="flex min-w-0 flex-1 flex-col gap-4">
          {/* Perjanjian Kerja card (.pen dsbMa) */}
          <DetailCard
            title={t('card.agreement')}
            icon={<FileText className="size-3.5 text-text-2" aria-hidden="true" />}
          >
            <KvRow label={t('field.agreementType')} value={placement.agreement_type ?? '—'} />
            <KvRow label={t('field.agreementId')}>
              <IdChip id={placement.agreement_id} />
            </KvRow>
            {/* Info note: placement period sits within agreement (.pen Fpre6) */}
            <div className="mt-3 flex items-start gap-2 rounded-lg border border-info-bd bg-info-bg px-3 py-2">
              <span className="mt-0.5 text-[11px] text-info-tx" aria-hidden="true">
                ⓘ
              </span>
              <p className="text-[12px] text-info-tx">{t('agreement.periodNote')}</p>
            </div>
          </DetailCard>

          {/* Shift-leader card (.pen g3pGV) — INV-2/3/4. Read-only here: leader
              assignment is managed on the company's Pemimpin Shift tab (single
              entry point). */}
          <ShiftLeaderCard
            placement={placement}
            leader={currentLeader ?? null}
            noLeaderWarning={noLeaderWarning}
            terminal={terminal}
            canManageOnCompany={canManageOnCompany}
          />
        </div>
      </div>

      {/* Modals */}
      <TransferModal
        open={showTransfer}
        onClose={() => setShowTransfer(false)}
        placement={placementInfo}
      />
      <RenewModal open={showRenew} onClose={() => setShowRenew(false)} placement={placementInfo} />
      <EndConfirm open={showEnd} onClose={() => setShowEnd(false)} placement={placementInfo} />
      <TerminateConfirm
        open={showTerminate}
        onClose={() => setShowTerminate(false)}
        placement={placementInfo}
      />
      <ResignModal
        open={showResign}
        onClose={() => setShowResign(false)}
        placement={placementInfo}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Placement chain card (.pen uH4M7) — renders history_chain from API
// ---------------------------------------------------------------------------

function PlacementChainCard({
  historyChain,
  currentId,
}: {
  historyChain: PlacementSummary[];
  currentId: string;
}) {
  const { t } = useTranslation('placementDetail');

  // history_chain is oldest → newest per API spec; reverse for newest-on-top display
  const orderedChain = [...historyChain].reverse();

  return (
    <DetailCard
      title={t('card.chain')}
      icon={<CalendarCheck className="size-3.5 text-text-2" aria-hidden="true" />}
    >
      <div className="flex flex-col gap-3.5 pt-1">
        {orderedChain.length === 0 && <p className="text-[13px] text-text-3">{t('chain.empty')}</p>}
        {orderedChain.map((item, idx) => {
          const isCurrent = item.id === currentId;
          return (
            <div key={item.id}>
              {idx > 0 && (
                <div className="flex items-center gap-1.5 py-1 pl-2">
                  <ArrowUpRight className="size-[13px] text-text-3" aria-hidden="true" />
                </div>
              )}
              <div
                className={[
                  'flex items-center justify-between rounded-xl px-[13px] py-[13px]',
                  isCurrent
                    ? 'border border-primary bg-primary-soft'
                    : 'border border-border bg-surface-2',
                ].join(' ')}
              >
                <div className="flex flex-col gap-[3px]">
                  <span className="text-[14px] font-semibold text-text">
                    {[item.client_company_name, item.service_line_name].filter(Boolean).join(' · ')}
                  </span>
                  <span className="text-[12px] text-text-3">
                    {item.start_date}
                    {item.end_date ? ` – ${item.end_date}` : ` – ${t('field.openEnded')}`}
                    {isCurrent ? ` · ${t('chain.current')}` : ''}
                  </span>
                </div>
                <StatusBadge tone={toneForPlacement(item.lifecycle_status)} dot>
                  {STATUS_LABEL[item.lifecycle_status]}
                </StatusBadge>
              </div>
            </div>
          );
        })}
      </div>
    </DetailCard>
  );
}

// ---------------------------------------------------------------------------
// Shift-leader card (.pen g3pGV) — INV-2/3/4
// ---------------------------------------------------------------------------

interface ShiftLeaderCardProps {
  placement: Placement;
  leader: ShiftLeaderAssignmentSummary | null;
  noLeaderWarning: boolean;
  terminal: boolean;
  /** Whether the viewer can open the company's Pemimpin Shift tab (clients.read). */
  canManageOnCompany: boolean;
}

function ShiftLeaderCard({
  placement,
  leader,
  noLeaderWarning,
  terminal,
  canManageOnCompany,
}: ShiftLeaderCardProps) {
  const { t } = useTranslation('placementDetail');

  // Read-only on placement detail: the leader is ASSIGNED/REPLACED/REVOKED from the
  // company's "Pemimpin Shift" tab (single entry point), an HR/Admin action. The link
  // is shown only to roles with clients.read so SL doesn't get a dead link.
  const actionArea =
    !terminal && canManageOnCompany ? (
      <Link
        to="/client-companies/$clientCompanyId"
        params={{ clientCompanyId: placement.client_company_id }}
        className="flex items-center gap-1 text-[12px] font-semibold text-primary hover:underline"
      >
        {t('sl.manageOnCompany')}
        <ArrowUpRight className="size-3" aria-hidden="true" />
      </Link>
    ) : null;

  return (
    <DetailCard
      title={t('card.shiftLeader')}
      icon={<Users className="size-3.5 text-text-2" aria-hidden="true" />}
      action={actionArea ?? undefined}
    >
      {/* INV-2 warning: no leader, placement is active (.pen R2VCaU) */}
      {(noLeaderWarning || leader == null) && !terminal && (
        <Banner
          tone="warn"
          title={t('sl.noLeaderTitle')}
          description={t('sl.noLeaderDesc', {
            company: placement.client_company_name ?? placement.client_company_id,
          })}
          className="mt-2"
        />
      )}

      {/* Leader present (.pen N2zAkh) */}
      {leader != null && (
        <div className="mt-2 flex items-center gap-2.5">
          <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary-soft text-[13px] font-bold text-primary">
            <Initials name={leader.employee_name ?? '?'} />
          </div>
          <div className="flex flex-col gap-[2px]">
            <span className="text-[14px] font-semibold text-text">
              {leader.employee_name ?? leader.employee_id}
            </span>
            <span className="text-[12px] text-text-3">
              {leader.employee_id === placement.employee_id
                ? t('sl.isThisAgent')
                : t('sl.otherAgent')}
            </span>
          </div>
          <Link
            to="/employees/$employeeId"
            params={{ employeeId: leader.employee_id }}
            className="ml-auto flex items-center gap-1 text-[12px] font-semibold text-primary hover:underline"
          >
            {t('sl.viewProfile')}
            <ArrowUpRight className="size-3" aria-hidden="true" />
          </Link>
        </div>
      )}

      {/* Terminal state with no leader */}
      {leader == null && terminal && (
        <p className="mt-2 text-[13px] text-text-3">{t('sl.vacated')}</p>
      )}
    </DetailCard>
  );
}
