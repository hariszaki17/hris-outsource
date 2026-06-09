/**
 * E10 · Approval Inbox Panel — "Perlu Tindakan"
 *
 * .pen frames:
 *   SIsts / yuLeS  Inbox panel (HR / SL variants — same component)
 *   biFs5          State · Approval inbox empty
 *   elJj3          State · Approval inbox filtered-zero
 *
 * Used inside both HR/Super-Admin and Shift-Leader dashboards.
 * Rules: G0 match frame layout · no dead-flow states · tokens only · i18n(dashboard) ns.
 */

import { type ApprovalInboxRow, ApprovalInboxRowKind } from '@swp/api-client/e10';
import { EmptyState } from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { ArrowRight, ChevronRight, Filter, Inbox } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { inboxKindIcon } from './e10-shared.tsx';

// ---------------------------------------------------------------------------
// Tone helpers — maps ApprovalInboxRowKind to icon / badge colour
// (ATTENDANCE_VERIFY → bad-tx, leave/ot/expiring → warn-tx, default → neutral)
// ---------------------------------------------------------------------------

type InboxTone = 'bad' | 'warn' | 'neutral';

function inboxRowTone(kind: ApprovalInboxRowKind): InboxTone {
  switch (kind) {
    case ApprovalInboxRowKind.ATTENDANCE_VERIFY:
      return 'bad';
    case ApprovalInboxRowKind.LEAVE_APPROVE:
    case ApprovalInboxRowKind.OT_APPROVE:
    case ApprovalInboxRowKind.PLACEMENT_EXPIRING:
    case ApprovalInboxRowKind.AGREEMENT_EXPIRING:
    case ApprovalInboxRowKind.HR_CHANGE_REQUEST:
      return 'warn';
    default:
      return 'neutral';
  }
}

const iconToneClass: Record<InboxTone, string> = {
  bad: 'text-bad-tx',
  warn: 'text-warn-tx',
  neutral: 'text-text-3',
};

const badgeBgClass: Record<InboxTone, string> = {
  bad: 'bg-bad-bg text-bad-tx',
  // warn gets its OWN warn token (was collapsed into bad-bg, making warn/bad indistinguishable —
  // DESIGN-SYSTEM: status meaning must not rely on a colour that duplicates another status).
  warn: 'bg-warn-bg text-warn-tx',
  neutral: 'bg-surface-2 text-text-3',
};

// ---------------------------------------------------------------------------
// ApprovalInboxRow — single row
// ---------------------------------------------------------------------------

interface InboxRowProps {
  row: ApprovalInboxRow;
  isLast: boolean;
}

function InboxRow({ row, isLast }: InboxRowProps) {
  const { t } = useTranslation('dashboard');
  const navigate = useNavigate();
  const tone = inboxRowTone(row.kind);
  const Icon = inboxKindIcon(row.kind);
  // Convey kind + count for SR users so urgency isn't communicated by badge colour alone.
  const countLabel = t('inbox.rowCount', { kind: t(`inbox.tone.${tone}`), count: row.count });

  function handleNavigate() {
    // Deep-link: path is a string route path. Cast to any to avoid strict router type mismatch.
    // DEVIATION: deep_link.path is untyped at the router level; cast via `as string` then pass
    // as `to` with a loose cast so TanStack Router doesn't reject unknown routes at compile time.
    void (navigate as (opts: { to: string }) => Promise<void>)({
      to: row.deep_link.path,
    });
  }

  return (
    <button
      type="button"
      onClick={handleNavigate}
      className={[
        'flex w-full items-center justify-between px-[18px] py-[13px] text-left',
        'hover:bg-surface-2 transition-colors',
        isLast ? '' : 'border-b border-border-soft',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      {/* Left: icon + label */}
      <div className="flex items-center gap-[10px]">
        <Icon aria-hidden className={['size-4', iconToneClass[tone]].join(' ')} />
        <span className="text-[14px] font-medium text-text">{row.label}</span>
      </div>

      {/* Right: count badge + chevron */}
      <div className="flex items-center gap-2">
        <div
          aria-label={countLabel}
          className={[
            'flex h-[22px] w-[24px] items-center justify-center rounded-full',
            badgeBgClass[tone],
          ].join(' ')}
        >
          <span aria-hidden className="text-[12px] font-bold">
            {row.count}
          </span>
        </div>
        <ChevronRight aria-hidden className="size-4 text-text-3" />
      </div>
    </button>
  );
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ApprovalInboxPanelProps {
  rows: ApprovalInboxRow[];
  isLoading?: boolean;
  /** When non-null, the user has an active kind-filter applied (filtered-zero state). */
  activeFilter?: ApprovalInboxRowKind | null;
  onClearFilter?: () => void;
}

// ---------------------------------------------------------------------------
// ApprovalInboxPanel
// ---------------------------------------------------------------------------

export function ApprovalInboxPanel({
  rows,
  isLoading,
  activeFilter,
  onClearFilter,
}: ApprovalInboxPanelProps) {
  const { t } = useTranslation('dashboard');
  const navigate = useNavigate();

  // ----- Skeleton -----
  if (isLoading) {
    return (
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        {/* Panel header */}
        <div className="flex items-center justify-between border-b border-border-soft px-[18px] py-[14px]">
          <span className="text-[15px] font-bold text-text">{t('inbox.title')}</span>
          <div className="h-[22px] w-[120px] animate-pulse rounded-full bg-surface-2" />
        </div>
        {/* Skeleton rows */}
        {[1, 2, 3].map((i) => (
          <div
            key={i}
            className="flex items-center justify-between border-b border-border-soft px-[18px] py-[13px] last:border-b-0"
          >
            <div className="flex items-center gap-[10px]">
              <div className="size-4 animate-pulse rounded bg-surface-2" />
              <div className="h-[14px] w-[140px] animate-pulse rounded bg-surface-2" />
            </div>
            <div className="h-[22px] w-[24px] animate-pulse rounded-full bg-surface-2" />
          </div>
        ))}
      </div>
    );
  }

  // ----- Empty state (biFs5): no rows and no active filter -----
  if (rows.length === 0 && !activeFilter) {
    return (
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        {/* Panel header */}
        <div className="flex items-center justify-between border-b border-border-soft px-[18px] py-[14px]">
          <span className="text-[15px] font-bold text-text">{t('inbox.title')}</span>
          {/* Inbox badge */}
          <div className="flex items-center gap-[5px] rounded-full border border-primary bg-primary-soft px-[8px] py-[3px]">
            <Inbox aria-hidden className="size-[11px] text-primary-strong" />
            <span className="text-[10px] font-bold text-primary-strong">
              {t('inbox.badgeLabel')}
            </span>
          </div>
        </div>
        {/* Empty body (biFs5) */}
        <div className="flex items-center justify-center px-[12px] py-[24px]">
          <EmptyState
            variant="fresh"
            icon={Inbox}
            title={t('inbox.emptyTitle')}
            description={t('inbox.emptyBody')}
            className="w-full"
          />
        </div>
      </div>
    );
  }

  // ----- Filtered-zero state (elJj3): rows came back empty after a kind filter -----
  if (rows.length === 0 && activeFilter) {
    const FilterIcon = inboxKindIcon(activeFilter);
    return (
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        {/* Panel header with filter pill */}
        <div className="flex items-center justify-between border-b border-border-soft px-[18px] py-[14px]">
          <span className="text-[15px] font-bold text-text">{t('inbox.title')}</span>
          {/* Filter pill (elJj3 FilterPill pattern) */}
          <div className="flex items-center gap-[5px] rounded-full border border-info-bd bg-info-bg px-[8px] py-[3px]">
            <Filter aria-hidden className="size-[11px] text-info-tx" />
            <FilterIcon aria-hidden className="size-[11px] text-info-tx" />
          </div>
        </div>
        {/* Filtered-zero body (elJj3) */}
        <div className="flex items-center justify-center px-[12px] py-[24px]">
          <div className="flex w-full flex-col items-center gap-3">
            <EmptyState
              variant="filtered"
              title={t('inbox.filteredTitle')}
              description={t('inbox.filteredBody')}
              className="w-full"
              action={
                onClearFilter ? (
                  <button
                    type="button"
                    className="text-[13px] font-semibold text-primary hover:underline"
                    onClick={onClearFilter}
                  >
                    {t('inbox.clearFilter')}
                  </button>
                ) : undefined
              }
            />
          </div>
        </div>
      </div>
    );
  }

  // ----- Default: rows present -----
  return (
    <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
      {/* Panel header */}
      <div className="flex items-center justify-between border-b border-border-soft px-[18px] py-[14px]">
        <span className="text-[15px] font-bold text-text">{t('inbox.title')}</span>
        {/* Inbox badge */}
        <div className="flex items-center gap-[5px] rounded-full border border-primary bg-primary-soft px-[8px] py-[3px]">
          <Inbox aria-hidden className="size-[11px] text-primary-strong" />
          <span className="text-[10px] font-bold text-primary-strong">{t('inbox.badgeLabel')}</span>
        </div>
      </div>

      {/* Rows */}
      {rows.map((row, idx) => (
        <InboxRow key={row.kind} row={row} isLast={idx === rows.length - 1} />
      ))}

      {/* Footer: "Buka antrean lengkap" */}
      <button
        type="button"
        onClick={() => void navigate({ to: '/inbox' })}
        className="flex items-center justify-center gap-[6px] border-t border-border-soft bg-surface-2 px-[16px] py-[10px] hover:bg-surface transition-colors"
      >
        <span className="text-[12px] font-semibold text-primary-strong">{t('inbox.openFull')}</span>
        <ArrowRight aria-hidden className="size-3 text-primary-strong" />
      </button>
    </div>
  );
}
