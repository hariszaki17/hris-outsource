/**
 * E7 · Persetujuan Lembur — approval queue (HR L2 + Shift Leader L1).
 *
 * .pen frames implemented:
 *   H1eBN  "E7 · Persetujuan Lembur (HR L2)"
 *          — HR sees cross-company PENDING_HR rows, bulk-approve + reject actions.
 *   Vh2P9  "E7 SL · Persetujuan Lembur (L1)"
 *          — Shift Leader sees own-company PENDING_L1 rows, same column layout.
 *
 * One screen component, role-branched via useCurrentUser() (mirrors E6 leave-approvals).
 *
 * Routes:
 *   /overtime             → OvertimeApprovalsScreen (HR: hr_admin | super_admin)
 *   /overtime             → OvertimeApprovalsScreen (SL: shift_leader, own-company)
 *
 * validateSearch fields: q, company_id, source, cursor
 *
 * Frame columns (H1eBN / Vh2P9):
 *   AGEN (260) · LEMBUR (250) · SUMBER (140) · DISETUJUI L1 (196) · [ACTIONS] (270)
 *
 * Key OT-specific behaviors vs E6:
 *   - `flagged_no_preapproval` pill surfaced on every row (EPICS §8 audit f8).
 *   - Bulk-approve via BulkApproveModal (optional note).
 *   - Row-level "Setujui" calls useApproveOvertimeL1 (SL) or useApproveOvertimeFinal (HR).
 *   - Row-level "Tolak" opens RejectOvertimeModal.
 *   - Checkbox-selected rows enable the "Setujui Massal" header button.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type ListOvertimeParams,
  type Overtime,
  OvertimeSource,
  OvertimeStatus,
  useApproveOvertimeFinal,
  useApproveOvertimeL1,
  useBulkApproveOvertime,
  useBulkRejectOvertime,
  useListOvertime,
  useRejectOvertime,
} from '@swp/api-client/e7';
import {
  Avatar,
  Button,
  Checkbox,
  type Column,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  FilterSelect,
  SearchField,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { CheckCheck, CircleCheck, Eye, RotateCcw, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  BulkApproveModal,
  BulkRejectModal,
  RejectOvertimeModal,
} from './overtime-queue-overlays.tsx';
import {
  formatOtMinutes,
  overtimeSourceKey,
  overtimeSourceTone,
  overtimeTierKey,
} from './overtime-shared.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type OvertimeApprovalsSearch = {
  q?: string;
  company_id?: string;
  source?: OvertimeSource;
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function hasActiveFilters(s: OvertimeApprovalsSearch): boolean {
  return Boolean(s.q || s.company_id || s.source);
}

// ---------------------------------------------------------------------------
// Inner component
// ---------------------------------------------------------------------------

interface OvertimeApprovalsScreenProps {
  search: OvertimeApprovalsSearch;
  onSearch: (patch: OvertimeApprovalsSearch) => void;
  prevCursors: string[];
  onPrevCursorsChange: (c: string[]) => void;
}

function OvertimeApprovalsScreenInner({
  search,
  onSearch,
  prevCursors,
  onPrevCursorsChange,
}: OvertimeApprovalsScreenProps) {
  const { t } = useTranslation('overtime');
  const { toast } = useToast();
  const user = useCurrentUser();

  const isHR = user?.role === 'hr_admin' || user?.role === 'super_admin';
  const isSL = user?.role === 'shift_leader';
  void isSL;

  // Default status: HR sees PENDING_HR, SL sees PENDING_L1
  const defaultStatus = isHR ? OvertimeStatus.PENDING_HR : OvertimeStatus.PENDING_L1;

  const params: ListOvertimeParams = {
    limit: PAGE_SIZE,
    cursor: search.cursor,
    status: defaultStatus,
    company_id: search.company_id || undefined,
    source: search.source || undefined,
  };

  const query = useListOvertime(params);
  const hasFilters = hasActiveFilters(search);

  // ---------------------------------------------------------------------------
  // Selection state
  // ---------------------------------------------------------------------------

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  function toggleRow(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll(rows: Overtime[]) {
    setSelectedIds((prev) => {
      if (prev.size === rows.length) return new Set();
      return new Set(rows.map((r) => r.id));
    });
  }

  // ---------------------------------------------------------------------------
  // Overlay state
  // ---------------------------------------------------------------------------

  const [rejectTarget, setRejectTarget] = useState<Overtime | null>(null);
  const [bulkApproveOpen, setBulkApproveOpen] = useState(false);
  const [bulkRejectOpen, setBulkRejectOpen] = useState(false);

  // ---------------------------------------------------------------------------
  // Mutations
  // ---------------------------------------------------------------------------

  const approveL1 = useApproveOvertimeL1();
  const approveFinal = useApproveOvertimeFinal();
  const rejectMutation = useRejectOvertime();
  const bulkApprove = useBulkApproveOvertime();
  const bulkReject = useBulkRejectOvertime();

  function handleApproveRow(ot: Overtime) {
    if (isHR) {
      approveFinal.mutate(
        { id: ot.id, data: {} },
        {
          onSuccess: () => {
            toast({ tone: 'success', title: t('approvals.approvedToast') });
          },
          onError: (err) => {
            const { message } = classifyError(err);
            toast({ tone: 'error', title: t(message) });
          },
        },
      );
    } else {
      approveL1.mutate(
        { id: ot.id, data: {} },
        {
          onSuccess: () => {
            toast({ tone: 'success', title: t('approvals.approvedToast') });
          },
          onError: (err) => {
            const { message } = classifyError(err);
            toast({ tone: 'error', title: t(message) });
          },
        },
      );
    }
  }

  function handleRejectConfirm(reason: string) {
    if (!rejectTarget) return;
    rejectMutation.mutate(
      { id: rejectTarget.id, data: { reason } },
      {
        onSuccess: () => {
          setRejectTarget(null);
          toast({ tone: 'success', title: t('approvals.rejectedToast') });
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t(message) });
        },
      },
    );
  }

  function handleBulkApproveConfirm(note: string) {
    const ids = Array.from(selectedIds);
    bulkApprove.mutate(
      { data: { ids, note: note || undefined } },
      {
        onSuccess: () => {
          setBulkApproveOpen(false);
          setSelectedIds(new Set());
          toast({
            tone: 'success',
            title: t('approvals.bulkApprovedToast', { count: ids.length }),
          });
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t(message) });
        },
      },
    );
  }

  function handleBulkRejectConfirm(reason: string) {
    const ids = Array.from(selectedIds);
    bulkReject.mutate(
      { data: { ids, reason } },
      {
        onSuccess: () => {
          setBulkRejectOpen(false);
          setSelectedIds(new Set());
          toast({
            tone: 'success',
            title: t('approvals.bulkRejectedToast', { count: ids.length }),
          });
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t(message) });
        },
      },
    );
  }

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  function setSearch(patch: OvertimeApprovalsSearch) {
    onSearch({ ...search, cursor: undefined, ...patch });
    onPrevCursorsChange([]);
    setSelectedIds(new Set());
  }

  // ---------------------------------------------------------------------------
  // Error states
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <div className="flex flex-col gap-[18px]">
          <ApprovalsTitle isHR={isHR} selectedCount={0} onBulkApprove={() => void 0} />
          <EmptyState
            variant="no-permission"
            title={t('errors.forbidden')}
            description={t('approvals.noPermissionBody')}
          />
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-[18px]">
        <ApprovalsTitle isHR={isHR} selectedCount={0} onBulkApprove={() => void 0} />
        <StateView
          kind="error"
          title={t('approvals.errorTitle')}
          description={t('errors.network')}
          onRetry={() => query.refetch()}
          retryLabel={t('common.retry')}
        />
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Data
  // ---------------------------------------------------------------------------

  const page = query.data?.data as
    | { data?: Overtime[]; has_more?: boolean; next_cursor?: string }
    | undefined;
  const rows: Overtime[] = page?.data ?? [];

  const allSelected = rows.length > 0 && selectedIds.size === rows.length;
  const someSelected = selectedIds.size > 0 && selectedIds.size < rows.length;

  // Total counted minutes in the visible queue
  const queueTotalMinutes = rows.reduce((acc, r) => acc + r.calculation.counted_minutes, 0);

  // ---------------------------------------------------------------------------
  // Columns
  // Frame: AGEN(260) · LEMBUR(250) · SUMBER(140) · DISETUJUI L1(196) · [ACTIONS](270)
  // ---------------------------------------------------------------------------

  const columns: Column<Overtime>[] = [
    {
      id: 'select',
      header: (
        <Checkbox
          checked={allSelected}
          ref={(el: HTMLInputElement | null) => {
            if (el) el.indeterminate = someSelected;
          }}
          aria-label={t('approvals.selectAll')}
          onChange={() => toggleAll(rows)}
        />
      ),
      width: 44,
      cell: (r) => (
        <Checkbox
          checked={selectedIds.has(r.id)}
          aria-label={t('approvals.selectRow', { id: r.id })}
          onChange={() => toggleRow(r.id)}
        />
      ),
    },
    {
      id: 'agent',
      header: t('approvals.colAgent'),
      width: 260,
      cell: (r) => (
        <div className="flex items-center gap-2.5">
          <Avatar
            initials={
              r.employee.name
                ?.split(' ')
                .slice(0, 2)
                .map((n) => n[0])
                .join('') ?? '??'
            }
            size={34}
          />
          <div className="flex flex-col gap-0.5">
            <span className="text-sm font-semibold text-text">
              {r.employee.name ?? r.employee.id}
            </span>
            <span className="text-[12px] text-text-3">{r.company.name ?? r.company.id}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'overtime',
      header: t('approvals.colOvertime'),
      width: 250,
      cell: (r) => (
        <div className="flex flex-col gap-0.5">
          <div className="flex items-center gap-1.5">
            <span className="font-mono text-sm font-semibold text-text">
              <DateText kind="date" value={r.work_date} />
              {' · '}
              {formatOtMinutes(r.calculation.counted_minutes)}
            </span>
          </div>
          <div className="flex items-center gap-1.5">
            <span className="inline-flex items-center rounded-full bg-surface-2 px-2 py-0.5 text-[11px] font-semibold text-text-2">
              {t(overtimeTierKey(r.tier_indicator))}
              {' · ref'}
            </span>
            {r.flagged_no_preapproval && (
              <StatusBadge dot tone="warn">
                {t('approvals.flagNoPreapproval')}
              </StatusBadge>
            )}
          </div>
        </div>
      ),
    },
    {
      id: 'source',
      header: t('approvals.colSource'),
      width: 140,
      cell: (r) => (
        <StatusBadge dot tone={overtimeSourceTone(r.source)}>
          {t(overtimeSourceKey(r.source))}
        </StatusBadge>
      ),
    },
    {
      id: 'l1approval',
      header: t('approvals.colL1Approval'),
      width: 196,
      cell: (r) => {
        // Find L1 approval entry if it exists
        const l1Entry = r.approvals?.find((a) => a.level === 1 && a.decision === 'APPROVED');
        if (!l1Entry) {
          return <span className="text-sm text-text-3">—</span>;
        }
        return (
          <div className="flex items-center gap-1.5">
            <CircleCheck aria-hidden className="size-3.5 shrink-0 text-ok-tx" />
            <span className="text-[12px] text-text-2">
              {l1Entry.approver?.name ?? ''}
              {l1Entry.decided_at && (
                <>
                  {' · '}
                  <DateText kind="date" value={l1Entry.decided_at} />
                </>
              )}
            </span>
          </div>
        );
      },
    },
    {
      id: 'actions',
      header: '',
      width: 270,
      cell: (r) => {
        const isApproving =
          (isHR ? approveFinal.isPending : approveL1.isPending) &&
          (isHR ? approveFinal.variables?.id === r.id : approveL1.variables?.id === r.id);
        const isRejecting = rejectMutation.isPending && rejectMutation.variables?.id === r.id;

        return (
          <div className="flex items-center justify-end gap-1.5">
            {/* Detail link */}
            <a
              href={`/overtime/${r.id}`}
              className="inline-flex items-center gap-1.5 rounded-[7px] border border-border bg-surface px-2.5 py-1.5 text-[12px] font-semibold text-text-2 hover:bg-surface-2"
            >
              <Eye aria-hidden className="size-[13px]" />
              {t('approvals.detail')}
            </a>

            {/* Tolak */}
            <button
              type="button"
              className="inline-flex items-center gap-1.5 rounded-[7px] border border-bad-bd bg-surface px-2.5 py-1.5 text-[12px] font-semibold text-bad-tx hover:bg-bad-bg disabled:opacity-50"
              onClick={() => setRejectTarget(r)}
              disabled={isApproving || isRejecting}
            >
              <X aria-hidden className="size-[13px]" />
              {t('approvals.reject')}
            </button>

            {/* Setujui */}
            <button
              type="button"
              className="inline-flex items-center gap-1.5 rounded-[7px] border border-primary bg-primary px-2.5 py-1.5 text-[12px] font-semibold text-white hover:bg-primary/90 disabled:opacity-50"
              onClick={() => handleApproveRow(r)}
              disabled={isApproving || isRejecting}
            >
              <CheckCheck aria-hidden className="size-[13px]" />
              {isApproving ? t('common.processing') : t('approvals.approve')}
            </button>
          </div>
        );
      },
    },
  ];

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      <ApprovalsTitle
        isHR={isHR}
        selectedCount={selectedIds.size}
        onBulkApprove={() => setBulkApproveOpen(true)}
      />

      {/* Filters — frame: Search · Semua perusahaan (HR only) · Semua sumber */}
      <div className="flex flex-wrap items-center gap-2.5">
        <SearchField
          placeholder={t('approvals.searchPlaceholder')}
          defaultValue={search.q ?? ''}
          containerClassName="w-64"
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
        />

        {isHR && (
          <FilterSelect
            aria-label={t('approvals.filterCompany')}
            value={search.company_id ?? ''}
            onChange={(e) => setSearch({ company_id: e.target.value || undefined })}
          >
            <option value="">{t('approvals.filterCompany')}</option>
          </FilterSelect>
        )}

        <FilterSelect
          aria-label={t('approvals.filterSource')}
          value={search.source ?? ''}
          onChange={(e) => setSearch({ source: (e.target.value as OvertimeSource) || undefined })}
        >
          <option value="">{t('approvals.filterSource')}</option>
          {Object.values(OvertimeSource).map((src) => (
            <option key={src} value={src}>
              {t(overtimeSourceKey(src))}
            </option>
          ))}
        </FilterSelect>

        {hasFilters && (
          <>
            <div className="h-6 w-px bg-border" />
            <Button
              type="button"
              variant="ghost"
              onClick={() =>
                setSearch({
                  q: undefined,
                  company_id: undefined,
                  source: undefined,
                  cursor: undefined,
                })
              }
            >
              <RotateCcw aria-hidden className="size-3.5" />
              {t('common.resetFilters')}
            </Button>
          </>
        )}

        {/* Bulk-reject shortcut when rows are selected */}
        {selectedIds.size > 0 && (
          <Button type="button" variant="destructive" onClick={() => setBulkRejectOpen(true)}>
            <X aria-hidden className="size-4" />
            {t('approvals.bulkReject', { count: selectedIds.size })}
          </Button>
        )}
      </div>

      {/* Table */}
      <DataTable
        aria-label={t('approvals.tableAriaLabel')}
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        isLoading={query.isLoading}
        skeletonRows={8}
        empty={
          hasFilters ? (
            <EmptyState
              variant="filtered"
              title={t('approvals.filteredTitle')}
              description={t('approvals.filteredBody')}
            />
          ) : (
            <EmptyState
              variant="fresh"
              title={t('approvals.emptyTitle')}
              description={t('approvals.emptyBody')}
            />
          )
        }
        footer={
          rows.length > 0 ? (
            <div className="flex items-center justify-between px-[18px] py-3">
              <span className="text-[13px] text-text-3">
                {isHR
                  ? t('approvals.queueSummaryHR', {
                      count: rows.length,
                      total: formatOtMinutes(queueTotalMinutes),
                    })
                  : t('approvals.queueSummarySL', {
                      count: rows.length,
                      total: formatOtMinutes(queueTotalMinutes),
                    })}
              </span>
              <CursorPagination
                rangeLabel={t('approvals.resultRange', { count: rows.length })}
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
            </div>
          ) : undefined
        }
      />

      {/* Overlays */}
      {rejectTarget && (
        <RejectOvertimeModal
          open={Boolean(rejectTarget)}
          overtime={{
            id: rejectTarget.id,
            work_date: rejectTarget.work_date,
            calculation: rejectTarget.calculation,
            employeeName: rejectTarget.employee.name ?? rejectTarget.employee.id,
          }}
          onOpenChange={(open) => {
            if (!open) setRejectTarget(null);
          }}
          onConfirm={handleRejectConfirm}
          isPending={rejectMutation.isPending}
        />
      )}

      <BulkApproveModal
        open={bulkApproveOpen}
        count={selectedIds.size}
        onOpenChange={setBulkApproveOpen}
        onConfirm={handleBulkApproveConfirm}
        isPending={bulkApprove.isPending}
      />

      <BulkRejectModal
        open={bulkRejectOpen}
        count={selectedIds.size}
        onOpenChange={setBulkRejectOpen}
        onConfirm={handleBulkRejectConfirm}
        isPending={bulkReject.isPending}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public export
// ---------------------------------------------------------------------------

export function OvertimeApprovalsScreen() {
  const [search, setSearch] = useState<OvertimeApprovalsSearch>({});
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  return (
    <OvertimeApprovalsScreenInner
      search={search}
      onSearch={(patch) => setSearch(patch)}
      prevCursors={prevCursors}
      onPrevCursorsChange={setPrevCursors}
    />
  );
}

// ---------------------------------------------------------------------------
// TitleBand
// Frame: title + subtitle · "Setujui Massal" secondary button (enabled when selection > 0)
// ---------------------------------------------------------------------------

interface ApprovalsTitleProps {
  isHR: boolean;
  selectedCount: number;
  onBulkApprove: () => void;
}

function ApprovalsTitle({ isHR, selectedCount, onBulkApprove }: ApprovalsTitleProps) {
  const { t } = useTranslation('overtime');
  return (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-bold text-text">{t('approvals.title')}</h1>
        <p className="text-sm text-text-3">
          {isHR ? t('approvals.subtitleHR') : t('approvals.subtitleSL')}
        </p>
      </div>
      <div className="flex items-center gap-2.5">
        <Button
          type="button"
          variant="secondary"
          onClick={onBulkApprove}
          disabled={selectedCount === 0}
        >
          <CheckCheck aria-hidden className="size-4" />
          {selectedCount > 0
            ? t('approvals.bulkApproveCount', { count: selectedCount })
            : t('approvals.bulkApprove')}
        </Button>
      </div>
    </div>
  );
}
