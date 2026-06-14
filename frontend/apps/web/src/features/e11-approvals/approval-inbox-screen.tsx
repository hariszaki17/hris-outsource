/**
 * E11 · Kotak Masuk Persetujuan — aggregated approval inbox (F11.3, IB-1..IB-7).
 *
 * .pen frame implemented:
 *   yv7Gs  "E11 · Kotak Masuk Persetujuan (HR/Lead)"
 *          — segmented type tabs (Semua / Cuti / Lembur) + table:
 *            PEMOHON · TIPE · RINGKASAN · PERUSAHAAN · BARIS (N/M) · DIAJUKAN · AKSI(Setujui/Tolak)
 *   EnabP  comp/ModalReject (via approval-inbox-overlays.tsx) for the "Tolak" action.
 *
 * Single source of truth: this is a *view* over the same `ApprovalInstance`s the per-domain
 * approval tabs read (IB-5). Lists `mine: true` — only current-line instances the viewer can
 * act on, excluding their own requests (INV-3, server-enforced).
 *
 * Acting is server-side membership-gated (IB-6, ENGINEERING C1); the UI handles
 * 403 SELF_APPROVAL_FORBIDDEN + 409 LINE_ALREADY_CLEARED gracefully (toast + refetch).
 *
 * Route: see header note at the bottom of this file — proposed `/inbox`.
 *
 * Opening detail: the screen does NOT depend on the router. It exposes an optional
 * `onOpenInstance(id)` prop; integration wires it to the request-detail route. When absent,
 * rows are non-navigable (actions still work).
 */

import { classifyError } from '@/lib/api-error.ts';
import {
  type ApprovalInstance,
  InstanceStatus,
  type ListApprovalInstancesParams,
  RequestType,
  useApproveApprovalInstance,
  useListApprovalInstances,
  useRejectApprovalInstance,
} from '@swp/api-client/e11';
import {
  Avatar,
  type Column,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  SearchField,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { Check, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { RejectInstanceModal } from './approval-inbox-overlays.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** Tab filter: undefined = "Semua" (all request types). */
export type ApprovalInboxSearch = {
  q?: string;
  request_type?: RequestType;
  cursor?: string;
};

export interface ApprovalInboxScreenProps {
  /**
   * Open the request-detail (approval-chain) view for an instance. Integration provides this
   * (typed route link). When omitted, rows are not clickable but inline actions still work.
   */
  onOpenInstance?: (id: string) => void;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Two-letter initials for an avatar from a display name (falls back to "?"). */
function initialsOf(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  return parts
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? '')
    .join('');
}

/**
 * Request-type pill tone. Frame: leave = info (blue), overtime = orange→neutral
 * (no orange StatusTone exists; map overtime to `warn` keeps it warm without colliding
 * with the brand/teal "approved" colors). Status colors only via StatusBadge.
 */
function requestTypeTone(rt: RequestType): 'info' | 'warn' {
  return rt === RequestType.OVERTIME ? 'warn' : 'info';
}

/**
 * "Baris N/M" indicator tone. Frame uses amber while awaiting the current row. Every inbox
 * row is at its current (awaiting) line by definition (`mine:true`), so warn is correct.
 */
const baris = (i: ApprovalInstance) => ({
  current: i.current_line,
  total: i.line_count ?? 1,
});

// ---------------------------------------------------------------------------
// Inner component
// ---------------------------------------------------------------------------

interface InnerProps extends ApprovalInboxScreenProps {
  search: ApprovalInboxSearch;
  onSearch: (patch: ApprovalInboxSearch) => void;
  prevCursors: string[];
  onPrevCursorsChange: (c: string[]) => void;
}

function ApprovalInboxScreenInner({
  search,
  onSearch,
  prevCursors,
  onPrevCursorsChange,
  onOpenInstance,
}: InnerProps) {
  const { t } = useTranslation('approvals');
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const params: ListApprovalInstancesParams = {
    mine: true,
    status: InstanceStatus.PENDING,
    limit: PAGE_SIZE,
    cursor: search.cursor,
    request_type: search.request_type,
  };

  const query = useListApprovalInstances(params);

  const [rejectTarget, setRejectTarget] = useState<ApprovalInstance | null>(null);

  const approve = useApproveApprovalInstance();
  const reject = useRejectApprovalInstance();

  // -------------------------------------------------------------------------
  // Mutations + graceful concurrent-state handling
  // -------------------------------------------------------------------------

  /** Invalidate every `/approval-instances` list so cleared rows drop out (IB-3). */
  function invalidateLists() {
    queryClient.invalidateQueries({
      predicate: (q) =>
        q.queryKey.some((k) => typeof k === 'string' && k.includes('/approval-instances')),
    });
  }

  /** Map approval-engine error codes to friendly toasts, then refetch (drops stale rows). */
  function handleActionError(err: unknown) {
    if (err instanceof Object && 'code' in err) {
      const code = (err as { code?: string }).code;
      if (code === 'SELF_APPROVAL_FORBIDDEN') {
        toast({ tone: 'error', title: t('inbox.errors.selfApproval') });
        invalidateLists();
        return;
      }
      if (code === 'LINE_ALREADY_CLEARED') {
        toast({ tone: 'info', title: t('inbox.errors.lineAlreadyCleared') });
        invalidateLists();
        return;
      }
    }
    const { message } = classifyError(err);
    toast({ tone: 'error', title: t(message, { defaultValue: t('inbox.errors.generic') }) });
  }

  function handleApprove(instance: ApprovalInstance) {
    approve.mutate(
      { id: instance.id, data: {} },
      {
        onSuccess: () => {
          invalidateLists();
          toast({ tone: 'success', title: t('inbox.toast.approved') });
        },
        onError: handleActionError,
      },
    );
  }

  function handleRejectConfirm(reason: string) {
    if (!rejectTarget) return;
    reject.mutate(
      { id: rejectTarget.id, data: { reason } },
      {
        onSuccess: () => {
          setRejectTarget(null);
          invalidateLists();
          toast({ tone: 'success', title: t('inbox.toast.rejected') });
        },
        onError: (err) => {
          setRejectTarget(null);
          handleActionError(err);
        },
      },
    );
  }

  // -------------------------------------------------------------------------
  // Handlers
  // -------------------------------------------------------------------------

  function setSearch(patch: ApprovalInboxSearch) {
    onSearch({ ...search, cursor: undefined, ...patch });
    onPrevCursorsChange([]);
  }

  const hasFilters = Boolean(search.q || search.request_type);

  // -------------------------------------------------------------------------
  // Error state
  // -------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <div className="flex flex-col gap-[18px]">
          <TitleBand />
          <EmptyState
            variant="no-permission"
            title={t('inbox.forbiddenTitle')}
            description={t('inbox.forbiddenBody')}
          />
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-[18px]">
        <TitleBand />
        <StateView
          kind="error"
          title={t('inbox.errorTitle')}
          description={t('inbox.errorBody')}
          onRetry={() => query.refetch()}
          retryLabel={t('common.retry')}
        />
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // Data
  // -------------------------------------------------------------------------

  const page = query.data?.data as
    | { data?: ApprovalInstance[]; has_more?: boolean; next_cursor?: string }
    | undefined;
  const rows: ApprovalInstance[] = page?.data ?? [];

  // -------------------------------------------------------------------------
  // Columns — frame yv7Gs
  //   PEMOHON(240) · TIPE(90) · RINGKASAN(fill) · PERUSAHAAN(150) · BARIS(120) · DIAJUKAN(110) · AKSI(180)
  // -------------------------------------------------------------------------

  const columns: Column<ApprovalInstance>[] = [
    {
      id: 'requester',
      header: t('inbox.colRequester'),
      width: 240,
      cell: (r) => {
        // No requester display-name field on the instance — show id (mono) + entity prefix.
        const display = r.requester_id ?? t('inbox.unknownRequester');
        return (
          <div className="flex items-center gap-2.5" data-testid={`instance-row-${r.id}`}>
            <Avatar initials={initialsOf(display)} size={32} />
            <div className="flex flex-col gap-0.5">
              <span className="font-mono text-xs text-text-3">{display}</span>
            </div>
          </div>
        );
      },
    },
    {
      id: 'type',
      header: t('inbox.colType'),
      width: 90,
      cell: (r) => (
        <StatusBadge tone={requestTypeTone(r.request_type)}>
          {t(`inbox.requestType.${r.request_type}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'summary',
      header: t('inbox.colSummary'),
      width: undefined,
      cell: (r) => (
        <span className="text-sm text-text">
          {/* Prefer the server summary; fall back to a derived "<type> · <request id>" label. */}
          {r.summary ?? `${t(`inbox.requestType.${r.request_type}`)} · ${r.request_id}`}
        </span>
      ),
    },
    {
      id: 'company',
      header: t('inbox.colCompany'),
      width: 150,
      cell: (r) => <span className="text-sm text-text-2">{r.company_id}</span>,
    },
    {
      id: 'line',
      header: t('inbox.colLine'),
      width: 120,
      cell: (r) => {
        const b = baris(r);
        return (
          <StatusBadge tone="warn">
            {t('inbox.lineLabel', { current: b.current, total: b.total })}
          </StatusBadge>
        );
      },
    },
    {
      id: 'submitted',
      header: t('inbox.colSubmitted'),
      width: 110,
      cell: (r) =>
        r.created_at ? (
          <DateText kind="instant" value={r.created_at} className="text-xs text-text-3" />
        ) : (
          <span className="text-xs text-text-3">—</span>
        ),
    },
    {
      id: 'actions',
      header: t('inbox.colActions'),
      width: 180,
      cell: (r) => {
        const isApproving = approve.isPending && approve.variables?.id === r.id;
        const isRejecting = reject.isPending && reject.variables?.id === r.id;
        const busy = isApproving || isRejecting;
        return (
          <div className="flex items-center justify-end gap-2">
            <button
              type="button"
              data-testid={`inbox-reject-${r.id}`}
              className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-surface px-3 py-1.5 text-xs font-semibold text-bad-tx hover:bg-bad-bg disabled:opacity-50"
              // stopPropagation: the row is clickable (→ detail); the action must not navigate.
              onClick={(e) => {
                e.stopPropagation();
                setRejectTarget(r);
              }}
              disabled={busy}
            >
              <X aria-hidden className="size-3.5" />
              {t('inbox.reject.action')}
            </button>
            <button
              type="button"
              data-testid={`inbox-approve-${r.id}`}
              className="inline-flex items-center gap-1.5 rounded-lg border border-primary bg-primary px-3 py-1.5 text-xs font-semibold text-white hover:bg-primary/90 disabled:opacity-50"
              onClick={(e) => {
                e.stopPropagation();
                handleApprove(r);
              }}
              disabled={busy}
            >
              <Check aria-hidden className="size-3.5" />
              {isApproving ? t('common.processing') : t('inbox.approve.action')}
            </button>
          </div>
        );
      },
    },
  ];

  // -------------------------------------------------------------------------
  // Tabs (segmented type filter) — frame: Semua / Cuti / Lembur
  // Counts come from the current page only (no aggregate count endpoint in v1).
  // -------------------------------------------------------------------------

  const tabs: { value: RequestType | undefined; label: string }[] = [
    { value: undefined, label: t('inbox.tabs.all') },
    { value: RequestType.LEAVE, label: t('inbox.tabs.leave') },
    { value: RequestType.OVERTIME, label: t('inbox.tabs.overtime') },
  ];

  // -------------------------------------------------------------------------
  // Render
  // -------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      <TitleBand
        right={
          <SearchField
            placeholder={t('inbox.searchPlaceholder')}
            defaultValue={search.q ?? ''}
            containerClassName="w-72"
            onChange={(e) => setSearch({ q: e.target.value || undefined })}
          />
        }
      />

      {/* Type tabs */}
      <div
        className="flex flex-wrap items-center gap-2"
        role="tablist"
        aria-label={t('inbox.tabsAria')}
      >
        {tabs.map((tab) => {
          const active = (search.request_type ?? undefined) === tab.value;
          return (
            <button
              key={tab.label}
              type="button"
              role="tab"
              aria-selected={active}
              className={
                active
                  ? 'inline-flex items-center gap-2 rounded-full bg-primary px-3.5 py-1.5 text-[13px] font-semibold text-white'
                  : 'inline-flex items-center gap-2 rounded-full border border-border bg-surface px-3.5 py-1.5 text-[13px] font-semibold text-text-2 hover:bg-surface-2'
              }
              onClick={() => setSearch({ request_type: tab.value })}
            >
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* Table */}
      <DataTable
        aria-label={t('inbox.tableAria')}
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        isLoading={query.isLoading}
        skeletonRows={8}
        onRowClick={onOpenInstance ? (r) => onOpenInstance(r.id) : undefined}
        empty={
          hasFilters ? (
            <EmptyState
              variant="filtered"
              title={t('inbox.filteredTitle')}
              description={t('inbox.filteredBody')}
            />
          ) : (
            <EmptyState
              variant="fresh"
              title={t('inbox.emptyTitle')}
              description={t('inbox.emptyBody')}
            />
          )
        }
        footer={
          rows.length > 0 ? (
            <CursorPagination
              rangeLabel={t('inbox.resultRange', { count: rows.length })}
              hasPrev={prevCursors.length > 0}
              hasNext={Boolean(page?.has_more)}
              prevLabel={t('common.prev')}
              nextLabel={t('common.next')}
              onPrev={() => {
                const next = [...prevCursors];
                const cursor = next.pop();
                onPrevCursorsChange(next);
                onSearch({ ...search, cursor: cursor || undefined });
              }}
              onNext={() => {
                const nextCursor = page?.next_cursor;
                if (!nextCursor) return;
                onPrevCursorsChange([...prevCursors, search.cursor ?? '']);
                onSearch({ ...search, cursor: nextCursor });
              }}
            />
          ) : undefined
        }
      />

      {/* Reject overlay (comp/ModalReject — EnabP) */}
      {rejectTarget && (
        <RejectInstanceModal
          open={Boolean(rejectTarget)}
          summary={
            rejectTarget.summary ??
            `${t(`inbox.requestType.${rejectTarget.request_type}`)} · ${rejectTarget.request_id}`
          }
          onOpenChange={(open) => {
            if (!open) setRejectTarget(null);
          }}
          onConfirm={handleRejectConfirm}
          isPending={reject.isPending}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// TitleBand — frame: "Kotak Masuk" + subtitle + SearchField (right)
// ---------------------------------------------------------------------------

function TitleBand({ right }: { right?: React.ReactNode }) {
  const { t } = useTranslation('approvals');
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="flex flex-col gap-1">
        <h1 className="font-bold text-3xl text-text">{t('inbox.title')}</h1>
        <p className="text-sm text-text-2">{t('inbox.subtitle')}</p>
      </div>
      {right}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public export
// ---------------------------------------------------------------------------

export function ApprovalInboxScreen({ onOpenInstance }: ApprovalInboxScreenProps) {
  const [search, setSearch] = useState<ApprovalInboxSearch>({});
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  return (
    <ApprovalInboxScreenInner
      search={search}
      onSearch={setSearch}
      prevCursors={prevCursors}
      onPrevCursorsChange={setPrevCursors}
      onOpenInstance={onOpenInstance}
    />
  );
}

export default ApprovalInboxScreen;
