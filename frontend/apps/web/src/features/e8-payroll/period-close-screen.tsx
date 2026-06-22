/**
 * E8 · F8.5 — Payroll Period Close Cockpit
 *
 * .pen frames (WEB CONSOLE board, feature group gLZlI, ScreensRow K494zi):
 *   XoMsT  Cockpit PERIOD OPEN — title band, 3 blocker StatCards, filter row,
 *           3 tabs Kehadiran/Lembur/Cuti, completeness table
 *   p2ikRQ Cockpit PERIOD LOCKED — read-only immutable summary table + Buka kembali
 *   uPAkt  Lembur tab (block-and-link)
 *   vq0RK  States (loading / empty / error)
 *   ATzJd  Toasts (success / clarif)
 *
 * DS entry: DESIGN-SYSTEM.md §8.16 (F8.5)
 * Sidebar nav: Penggajian WvCi4 · SECTION_SUBNAV['/payroll'] sub-tab
 * Analog: attendance-dashboard-screen.tsx (server-paged table + tabs + statcards + cursor)
 *
 * Business rules: PC-1..PC-15 (payroll-period-close.md)
 *   PC-5  FIELD gates the lock; INTERNAL is informational
 *   PC-7  Lock: 0 FIELD blockers OR super-admin force
 *   PC-13 Clarification: one round, advisory
 *   PC-15 Force-lock + reopen = super_admin only
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type ClarificationRequest,
  type CompletenessListResponse,
  EmploymentType,
  type GetPeriodCompletenessParams,
  type PayrollPeriod,
  type PeriodEmployeeSummary,
  PeriodStatus,
  type PeriodSummaryListResponse,
  useGetPayrollPeriod,
  useGetPeriodCompleteness,
  useGetPeriodSummary,
} from '@swp/api-client/e8';
import type { ClarificationSourceType } from '@swp/api-client/e8';
import type { CompletenessRow } from '@swp/api-client/e8';
import type { StatusTone } from '@swp/design-tokens';
import { formatInstant, formatMonthYear, shiftMonth, todayJakartaIso } from '@swp/shared';
import {
  Button,
  type Column,
  CursorPagination,
  DataTable,
  EmptyState,
  FilterSelect,
  StatCard,
  StateView,
  StatusBadge,
} from '@swp/ui';
import { useNavigate, useSearch } from '@tanstack/react-router';
import {
  AlertTriangle,
  CalendarCheck,
  ChevronLeft,
  ChevronRight,
  Clock,
  Lock,
  LockOpen,
  MessageSquare,
  ShieldCheck,
  TriangleAlert,
  Users,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ClarificationDialog,
  LockDialog,
  ManageClarificationDialog,
  ReopenDialog,
} from './period-close-overlays.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type PeriodCloseSearch = {
  year?: number;
  month?: number;
  tab?: 'kehadiran' | 'lembur' | 'cuti' | 'ringkasan';
  cursor?: string;
};

type PeriodCloseTab = NonNullable<PeriodCloseSearch['tab']>;

const PAGE_SIZE = 50;

// ---------------------------------------------------------------------------
// Period status helpers
// ---------------------------------------------------------------------------

function periodStatusTone(status: PeriodStatus): StatusTone {
  switch (status) {
    case PeriodStatus.OPEN:
      return 'info';
    case PeriodStatus.LOCKED:
      return 'neutral';
    case PeriodStatus.REOPENED:
      return 'ok';
    default:
      return 'neutral';
  }
}

function periodStatusLabel(status: PeriodStatus, t: (k: string) => string): string {
  switch (status) {
    case PeriodStatus.OPEN:
      return t('periodClose.statusOpen');
    case PeriodStatus.LOCKED:
      return t('periodClose.statusLocked');
    case PeriodStatus.REOPENED:
      return t('periodClose.statusReopened');
    default:
      return status;
  }
}

// ---------------------------------------------------------------------------
// Tab strip (mirrored from attendance-dashboard-screen.tsx:106 — no shared Tabs primitive)
// ---------------------------------------------------------------------------

interface TabItem {
  id: PeriodCloseTab;
  label: string;
  badge?: number;
  active: boolean;
  onClick: () => void;
}

function StatusTabs({ tabs }: { tabs: TabItem[] }) {
  return (
    <div className="flex items-center gap-[26px] px-[18px] pt-[6px] border-b border-border">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          onClick={tab.onClick}
          className={[
            'flex items-center gap-[7px] pb-[14px] pt-[14px] text-[14px] font-medium',
            tab.active
              ? 'border-b-2 border-primary text-primary font-semibold'
              : 'border-b-2 border-transparent text-text-2',
          ].join(' ')}
        >
          {tab.label}
          {tab.badge !== undefined && tab.badge > 0 && (
            <span className="inline-flex items-center justify-center rounded-full bg-warn-bg px-[6px] py-[1px] text-[11px] font-bold text-warn-tx">
              {tab.badge}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Completeness table (Kehadiran tab — frame XoMsT)
// ---------------------------------------------------------------------------

interface CompletenessTableProps {
  year: number;
  month: number;
  period: PayrollPeriod;
  search: PeriodCloseSearch;
  onSetSearch: (partial: Partial<PeriodCloseSearch>) => void;
  onClarify: (sourceType: ClarificationSourceType, sourceId: string, employeeName: string) => void;
  /** Map of employee_id → open ClarificationRequest raised in this session. */
  openClarifications: Map<string, ClarificationRequest>;
  onManageClarification: (clarification: ClarificationRequest, employeeName: string) => void;
  /** True for HR / super-admin (payroll.read holders). */
  canManageClarifications: boolean;
}

function CompletenessTable({
  year,
  month,
  period,
  search,
  onSetSearch,
  onClarify,
  openClarifications,
  onManageClarification,
  canManageClarifications,
}: CompletenessTableProps) {
  const { t } = useTranslation('payroll');
  const [prevCursors, setPrevCursors] = useState<string[]>([]);
  const [blockersOnly, setBlockersOnly] = useState(false);
  const [employmentTypeFilter, setEmploymentTypeFilter] = useState<EmploymentType | ''>('');

  const params: GetPeriodCompletenessParams = {
    // API tab values: 'attendance' | 'overtime' | 'leave'
    tab: 'attendance',
    blockers_only: blockersOnly || undefined,
    employment_type: employmentTypeFilter || undefined,
    cursor: search.cursor,
    limit: PAGE_SIZE,
  };

  const query = useGetPeriodCompleteness(year, month, params);
  const page = query.data?.data as CompletenessListResponse | undefined;
  const rows: CompletenessRow[] = (page?.data as CompletenessRow[]) ?? [];
  // Server-filtered by employment_type; filteredRows = all rows when filter is unset
  const filteredRows = rows;

  // frame XoMsT — completeness table columns:
  // Karyawan (Avatar+name+SWP-EMP id) | Tipe (FIELD/INTERNAL) | Diharapkan | Bersih | Cuti | Blocker | Aksi
  const columns: Column<CompletenessRow>[] = [
    {
      id: 'employee',
      header: t('periodClose.colEmployee'),
      cell: (row) => (
        <div className="flex items-center gap-[10px]">
          <div className="size-8 rounded-full bg-surface-2 flex items-center justify-center text-[12px] font-semibold text-text-2 shrink-0">
            {(row.employee_name ?? row.employee_id).slice(0, 2).toUpperCase()}
          </div>
          <div className="flex flex-col gap-[1px]">
            <span
              className={[
                'text-[13px] font-medium',
                row.employment_type === EmploymentType.INTERNAL ? 'text-text-3' : 'text-text',
              ].join(' ')}
            >
              {row.employee_name ?? row.employee_id}
            </span>
            <span className="font-mono text-[11px] text-text-3">{row.employee_id}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'type',
      header: t('periodClose.colType'),
      width: 110,
      cell: (row) => (
        <StatusBadge dot tone={row.employment_type === EmploymentType.FIELD ? 'info' : 'neutral'}>
          {row.employment_type === EmploymentType.FIELD
            ? t('periodClose.typeField')
            : t('periodClose.typeInternal')}
        </StatusBadge>
      ),
    },
    {
      id: 'expected',
      header: t('periodClose.colExpected'),
      width: 90,
      align: 'right',
      cell: (row) => <span className="text-[13px] tabular-nums text-text">{row.expected}</span>,
    },
    {
      id: 'clean',
      header: t('periodClose.colClean'),
      width: 80,
      align: 'right',
      cell: (row) => (
        <span
          className={[
            'text-[13px] tabular-nums',
            row.clean === row.expected ? 'text-ok-tx font-medium' : 'text-text',
          ].join(' ')}
        >
          {row.clean}
        </span>
      ),
    },
    {
      id: 'on_leave',
      header: t('periodClose.colLeave'),
      width: 70,
      align: 'right',
      cell: (row) => <span className="text-[13px] tabular-nums text-text">{row.on_leave}</span>,
    },
    {
      id: 'blockers',
      header: t('periodClose.colBlockers'),
      width: 90,
      align: 'right',
      cell: (row) => {
        // PC-5: INTERNAL blockers are informational, not gating
        const isInternal = row.employment_type === EmploymentType.INTERNAL;
        if (row.blockers === 0) {
          return (
            <StatusBadge tone="ok" dot>
              {t('periodClose.blockerClean')}
            </StatusBadge>
          );
        }
        return (
          <StatusBadge tone={isInternal ? 'neutral' : 'bad'} dot>
            {row.blockers}
          </StatusBadge>
        );
      },
    },
    {
      id: 'actions',
      header: '',
      width: 200,
      align: 'right',
      cell: (row) => {
        const openClarif = openClarifications.get(row.employee_id);
        return (
          <div className="flex items-center gap-2 justify-end">
            {/* "Menunggu klarifikasi" warn badge + manage button — shown when an
                open clarification exists for this row (PC-13). */}
            {openClarif && canManageClarifications && (
              <button
                type="button"
                data-testid={`clarif-manage-${row.employee_id}`}
                onClick={() =>
                  onManageClarification(
                    openClarif,
                    row.employee_name ?? row.employee_id,
                  )
                }
                className="flex items-center gap-1.5 rounded-md border border-warn-bd bg-warn-bg px-2 py-1 text-[11px] font-medium text-warn-tx transition-opacity hover:opacity-80"
                aria-label={t('periodClose.clarifManageAriaLabel')}
              >
                <MessageSquare aria-hidden className="size-3 shrink-0" />
                {t('periodClose.clarifStatusWaiting')}
              </button>
            )}

            {/* Detail drill — links to attendance dashboard filtered to this employee */}
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                window.open(`/attendance?q=${encodeURIComponent(row.employee_id)}`, '_blank');
              }}
            >
              {t('periodClose.actionDrill')}
            </Button>

            {/* Raise clarification (PC-13) — HR only, only for FIELD with blockers,
                only when no open clarification already exists for this row. */}
            {row.employment_type === EmploymentType.FIELD &&
              row.blockers > 0 &&
              !openClarif && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() =>
                    onClarify('ATTENDANCE', row.employee_id, row.employee_name ?? row.employee_id)
                  }
                >
                  <MessageSquare className="size-3.5" aria-hidden />
                </Button>
              )}
          </div>
        );
      },
    },
  ];

  if (query.isError) {
    return <StateView kind="error" title={t('errors.loadError')} onRetry={() => query.refetch()} />;
  }

  return (
    <div className="flex flex-col">
      {/* Filter row — XoMsT filter row */}
      <div className="flex flex-wrap items-center gap-[10px] border-b border-border-soft px-[18px] py-[14px]">
        <FilterSelect
          aria-label={t('periodClose.filterType')}
          value={employmentTypeFilter}
          onChange={(e) => setEmploymentTypeFilter(e.target.value as EmploymentType | '')}
        >
          <option value="">{t('periodClose.filterTypeAll')}</option>
          <option value={EmploymentType.FIELD}>{t('periodClose.typeField')}</option>
          <option value={EmploymentType.INTERNAL}>{t('periodClose.typeInternal')}</option>
        </FilterSelect>

        {/* Blockers-only toggle */}
        <label className="flex items-center gap-2 text-[13px] text-text-2 cursor-pointer">
          <input
            type="checkbox"
            className="rounded border-border"
            checked={blockersOnly}
            onChange={(e) => {
              setBlockersOnly(e.target.checked);
              setPrevCursors([]);
              onSetSearch({ cursor: undefined });
            }}
          />
          {t('periodClose.filterBlockersOnly')}
        </label>

        <div className="flex-1" />

        {/* Open clarifications count — frame XoMsT */}
        {(period.open_clarifications ?? 0) > 0 && (
          <span className="flex items-center gap-1.5 rounded-md bg-warn-bg px-2.5 py-1 text-[12px] font-medium text-warn-tx">
            <MessageSquare className="size-3.5" aria-hidden />
            {t('periodClose.openClarifications', {
              count: period.open_clarifications,
            })}
          </span>
        )}
      </div>

      <DataTable
        aria-label={t('periodClose.tableAriaLabel')}
        columns={columns}
        data={filteredRows}
        getRowId={(r) => r.employee_id}
        isLoading={query.isLoading}
        skeletonRows={6}
        empty={
          <EmptyState
            variant={blockersOnly ? 'filtered' : 'fresh'}
            title={blockersOnly ? t('periodClose.emptyBlockersTitle') : t('periodClose.emptyTitle')}
            description={
              blockersOnly ? t('periodClose.emptyBlockersBody') : t('periodClose.emptyBody')
            }
          />
        }
        footer={
          rows.length > 0 ? (
            <CursorPagination
              rangeLabel={t('periodClose.resultRange', { count: filteredRows.length })}
              hasPrev={prevCursors.length > 0}
              hasNext={Boolean(page?.has_more)}
              prevLabel={t('common.prev', { ns: 'translation' })}
              nextLabel={t('common.next', { ns: 'translation' })}
              onPrev={() => {
                const next = [...prevCursors];
                const cursor = next.pop();
                setPrevCursors(next);
                onSetSearch({ cursor: cursor || undefined });
              }}
              onNext={() => {
                const nextCursor = page?.next_cursor;
                if (!nextCursor) return;
                setPrevCursors((s) => [...s, search.cursor ?? '']);
                onSetSearch({ cursor: nextCursor });
              }}
            />
          ) : undefined
        }
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Lembur / Cuti tab — block-and-link (frame uPAkt)
// ---------------------------------------------------------------------------

function BlockAndLinkTab({
  tab,
  blockerCount,
}: {
  tab: 'lembur' | 'cuti';
  blockerCount: number;
}) {
  const { t } = useTranslation('payroll');
  const isLembur = tab === 'lembur';
  const linkTo = isLembur ? '/overtime' : '/leave';
  const labelKey = isLembur ? 'periodClose.lemburBlockLink' : 'periodClose.cutiBlockLink';
  const descKey = isLembur ? 'periodClose.lemburBlockDesc' : 'periodClose.cutiBlockDesc';

  return (
    <div className="flex flex-col items-center gap-4 lg:p-6 py-16 px-8 text-center">
      {/* frame uPAkt — block-and-link state */}
      {blockerCount > 0 ? (
        <>
          <div className="flex items-center justify-center size-14 rounded-full bg-warn-bg">
            <TriangleAlert className="size-7 text-warn-tx" aria-hidden />
          </div>
          <div className="flex flex-col gap-2">
            <p className="text-[15px] font-semibold text-text">
              {t('periodClose.blockAndLinkTitle', { count: blockerCount })}
            </p>
            <p className="text-[13px] text-text-2 max-w-sm">{t(descKey)}</p>
          </div>
          <Button variant="primary" onClick={() => window.open(linkTo, '_blank')}>
            {t(labelKey)}
          </Button>
        </>
      ) : (
        <>
          <div className="flex items-center justify-center size-14 rounded-full bg-ok-bg">
            <CalendarCheck className="size-7 text-ok-tx" aria-hidden />
          </div>
          <p className="text-[15px] font-semibold text-text">
            {t('periodClose.blockAndLinkClean')}
          </p>
        </>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Ringkasan tab — immutable summary table (frame p2ikRQ)
// ---------------------------------------------------------------------------

function SummaryTab({ year, month }: { year: number; month: number }) {
  const { t } = useTranslation('payroll');
  const [prevCursors, setPrevCursors] = useState<string[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);

  const query = useGetPeriodSummary(year, month, { cursor, limit: PAGE_SIZE });
  const page = query.data?.data as PeriodSummaryListResponse | undefined;
  const rows: PeriodEmployeeSummary[] = (page?.data as PeriodEmployeeSummary[]) ?? [];
  const notLocked = page?.meta?.code === 'PERIOD_NOT_LOCKED';

  if (query.isError) {
    return <StateView kind="error" title={t('errors.loadError')} onRetry={() => query.refetch()} />;
  }

  if (notLocked) {
    return (
      <EmptyState
        variant="fresh"
        title={t('periodClose.summaryNotLockedTitle')}
        description={t('periodClose.summaryNotLockedBody')}
      />
    );
  }

  // frame p2ikRQ — immutable summary table columns
  const columns: Column<PeriodEmployeeSummary>[] = [
    {
      id: 'employee',
      header: t('periodClose.colEmployee'),
      cell: (row) => (
        <div className="flex flex-col gap-[1px]">
          <span className="text-[13px] font-medium text-text">
            {row.employee_name ?? row.employee_id}
          </span>
          <span className="font-mono text-[11px] text-text-3">{row.employee_id}</span>
        </div>
      ),
    },
    {
      id: 'payable_days',
      header: t('periodClose.summaryPayableDays'),
      width: 100,
      align: 'right',
      cell: (row) => (
        <span className="text-[13px] tabular-nums font-medium text-text">{row.payable_days}</span>
      ),
    },
    {
      id: 'present_days',
      header: t('periodClose.summaryPresentDays'),
      width: 90,
      align: 'right',
      cell: (row) => <span className="text-[13px] tabular-nums text-text">{row.present_days}</span>,
    },
    {
      id: 'late_days',
      header: t('periodClose.summaryLateDays'),
      width: 80,
      align: 'right',
      cell: (row) => <span className="text-[13px] tabular-nums text-text">{row.late_days}</span>,
    },
    {
      id: 'absent_days',
      header: t('periodClose.summaryAbsentDays'),
      width: 80,
      align: 'right',
      cell: (row) => (
        <span
          className={[
            'text-[13px] tabular-nums',
            row.absent_days > 0 ? 'text-bad-tx' : 'text-text',
          ].join(' ')}
        >
          {row.absent_days}
        </span>
      ),
    },
    {
      id: 'paid_leave_days',
      header: t('periodClose.summaryPaidLeave'),
      width: 90,
      align: 'right',
      cell: (row) => (
        <span className="text-[13px] tabular-nums text-text">{row.paid_leave_days}</span>
      ),
    },
    {
      id: 'approved_ot_minutes',
      header: t('periodClose.summaryOtMinutes'),
      width: 90,
      align: 'right',
      cell: (row) => (
        <span className="text-[13px] tabular-nums text-text">{row.approved_ot_minutes}</span>
      ),
    },
    {
      id: 'locked_at',
      header: t('periodClose.summaryLockedAt'),
      width: 140,
      cell: (row) => (
        <span className="text-[12px] text-text-3">
          {formatInstant(row.created_at, { dateStyle: 'short', timeStyle: 'short' })}
        </span>
      ),
    },
  ];

  return (
    <div className="flex flex-col">
      {/* Immutable banner — frame p2ikRQ */}
      <div className="flex items-center gap-2 border-b border-info-bd bg-info-bg px-[18px] py-[10px]">
        <Lock className="size-4 text-info-tx shrink-0" aria-hidden />
        <p className="text-[12px] font-medium text-info-tx">
          {t('periodClose.summaryImmutableBanner')}
        </p>
      </div>
      <DataTable
        aria-label={t('periodClose.summaryAriaLabel')}
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        isLoading={query.isLoading}
        skeletonRows={6}
        empty={
          <EmptyState
            variant="fresh"
            title={t('periodClose.summaryEmptyTitle')}
            description={t('periodClose.summaryEmptyBody')}
          />
        }
        footer={
          rows.length > 0 ? (
            <CursorPagination
              rangeLabel={t('periodClose.resultRange', { count: rows.length })}
              hasPrev={prevCursors.length > 0}
              hasNext={Boolean(page?.has_more)}
              prevLabel={t('common.prev', { ns: 'translation' })}
              nextLabel={t('common.next', { ns: 'translation' })}
              onPrev={() => {
                const next = [...prevCursors];
                const c = next.pop();
                setPrevCursors(next);
                setCursor(c || undefined);
              }}
              onNext={() => {
                const nextCursor = page?.next_cursor;
                if (!nextCursor) return;
                setPrevCursors((s) => [...s, cursor ?? '']);
                setCursor(nextCursor);
              }}
            />
          ) : undefined
        }
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// PeriodCloseScreen (main export)
// ---------------------------------------------------------------------------

export function PeriodCloseScreen() {
  const { t } = useTranslation('payroll');
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as PeriodCloseSearch;
  const currentUser = useCurrentUser();
  const isSuperAdmin = currentUser?.role === 'super_admin';

  // Period picker — default to current month
  const today = todayJakartaIso();
  const defaultYear = Number(today.slice(0, 4));
  const defaultMonth = Number(today.slice(5, 7));
  const year = search.year ?? defaultYear;
  const month = search.month ?? defaultMonth;

  // Anchor ISO for date display utilities (first day of the period)
  const anchorIso = `${year}-${String(month).padStart(2, '0')}-01`;

  const activeTab: PeriodCloseTab = search.tab ?? 'kehadiran';

  // Overlay state
  const [lockOpen, setLockOpen] = useState(false);
  const [reopenOpen, setReopenOpen] = useState(false);
  const [clarifOpen, setClarifOpen] = useState(false);
  const [clarifSource, setClarifSource] = useState<{
    sourceType: ClarificationSourceType;
    sourceId: string;
    label: string;
  } | null>(null);

  // Track open clarifications raised in this session: employee_id → ClarificationRequest.
  // When HR raises a clarification the response is stored here; on resolve/cancel it's
  // removed so the raise button reappears.  Keyed by source_id (= employee_id for the
  // attendance tab).
  const [openClarifications, setOpenClarifications] = useState<Map<string, ClarificationRequest>>(
    new Map(),
  );

  // Manage-clarification dialog state
  const [manageClarifOpen, setManageClarifOpen] = useState(false);
  const [manageClarifTarget, setManageClarifTarget] = useState<{
    clarification: ClarificationRequest;
    employeeName: string;
  } | null>(null);

  // payroll.read gates resolve/cancel affordances (HR + super-admin)
  const canManageClarifications =
    currentUser?.permissions.includes('payroll.read') ?? false;

  // Period query — GET /payroll-periods/{year}/{month}
  // Auto-creates OPEN on first access (PC-1)
  //
  // The backend wraps the period in a { data: PayrollPeriod } envelope (same
  // as every other endpoint). Orval's customFetch returns the raw body as
  // periodQuery.data.data (the outer Orval { data, status, headers } envelope),
  // so we need one more .data to reach the actual PayrollPeriod object.
  const periodQuery = useGetPayrollPeriod(year, month);
  const period = ((periodQuery.data?.data as { data?: PayrollPeriod })?.data) ?? null;

  function setSearch(partial: Partial<PeriodCloseSearch>) {
    void (
      navigate as (o: {
        to: string;
        search?: Record<string, unknown>;
      }) => void
    )({
      to: '/payroll/period-close',
      search: { ...search, ...partial, cursor: undefined },
    });
  }

  function navigateMonth(delta: number) {
    const next = shiftMonth(anchorIso, delta);
    setSearch({
      year: Number(next.slice(0, 4)),
      month: Number(next.slice(5, 7)),
      cursor: undefined,
    });
  }

  function handleClarify(sourceType: ClarificationSourceType, sourceId: string, label: string) {
    setClarifSource({ sourceType, sourceId, label });
    setClarifOpen(true);
  }

  function handleClarificationCreated(clarification: ClarificationRequest) {
    setOpenClarifications((prev) => {
      const next = new Map(prev);
      next.set(clarification.source_id, clarification);
      return next;
    });
  }

  function handleManageClarification(clarification: ClarificationRequest, employeeName: string) {
    setManageClarifTarget({ clarification, employeeName });
    setManageClarifOpen(true);
  }

  function handleClarificationSettled(id: string) {
    setOpenClarifications((prev) => {
      const next = new Map(prev);
      // Remove by value (id) — key is source_id, value carries the id field
      for (const [sourceId, clarif] of next.entries()) {
        if (clarif.id === id) {
          next.delete(sourceId);
          break;
        }
      }
      return next;
    });
  }

  // ---------------------------------------------------------------------------
  // Loading / error states — frame vq0RK
  // ---------------------------------------------------------------------------

  if (periodQuery.isLoading) {
    return <StateView kind="loading" title="Memuat periode…" />;
  }

  if (periodQuery.isError) {
    const err = classifyError(periodQuery.error);
    if (err.kind === 'forbidden') {
      return (
        <StateView
          kind="no-permission"
          title={t('common.noPermission')}
          description={t('common.noPermissionBody')}
        />
      );
    }
    return (
      <StateView kind="error" title={t('errors.loadError')} onRetry={() => periodQuery.refetch()} />
    );
  }

  if (!period) {
    return (
      <StateView
        kind="empty"
        title={t('periodClose.emptyTitle')}
        description={t('periodClose.emptyBody')}
      />
    );
  }

  const isOpen = period.status === PeriodStatus.OPEN || period.status === PeriodStatus.REOPENED;
  const isLocked = period.status === PeriodStatus.LOCKED;
  // blockers is required per the OpenAPI spec but the Go DTO uses omitempty on the pointer;
  // guard against undefined so a stale cached value or a brief mid-truncation 500 that
  // slips through TanStack Query's error state doesn't crash the ErrorBoundary (C-PC-1).
  const blockers = period.blockers ?? { attendance: 0, overtime: 0, leave: 0 };
  const hasBlockers = blockers.attendance > 0 || blockers.overtime > 0 || blockers.leave > 0;

  // tabs — Ringkasan only visible when LOCKED (frame p2ikRQ)
  const tabs: TabItem[] = [
    {
      id: 'kehadiran',
      label: t('periodClose.tabKehadiran'),
      badge: blockers.attendance,
      active: activeTab === 'kehadiran',
      onClick: () => setSearch({ tab: 'kehadiran' }),
    },
    {
      id: 'lembur',
      label: t('periodClose.tabLembur'),
      badge: blockers.overtime,
      active: activeTab === 'lembur',
      onClick: () => setSearch({ tab: 'lembur' }),
    },
    {
      id: 'cuti',
      label: t('periodClose.tabCuti'),
      badge: blockers.leave,
      active: activeTab === 'cuti',
      onClick: () => setSearch({ tab: 'cuti' }),
    },
    ...(isLocked
      ? [
          {
            id: 'ringkasan' as PeriodCloseTab,
            label: t('periodClose.tabRingkasan'),
            active: activeTab === 'ringkasan',
            onClick: () => setSearch({ tab: 'ringkasan' }),
          },
        ]
      : []),
  ];

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band — frame XoMsT (OPEN) / p2ikRQ (LOCKED) */}
      <div className="flex items-start justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl font-bold text-text">{t('periodClose.title')}</h1>
          <p className="text-[13px] text-text-2">{t('periodClose.subtitle')}</p>
        </div>

        {/* Period-lock status badge */}
        <StatusBadge dot tone={periodStatusTone(period.status)}>
          {periodStatusLabel(period.status, t)}
        </StatusBadge>
      </div>

      {/* Period picker — month navigator */}
      <div className="flex items-center gap-3 rounded-xl border border-border bg-surface px-5 py-3">
        <button
          type="button"
          aria-label={t('periodClose.prevMonth')}
          className="flex items-center justify-center size-8 rounded-lg border border-border bg-surface hover:bg-surface-2 text-text-2"
          onClick={() => navigateMonth(-1)}
        >
          <ChevronLeft className="size-4" aria-hidden />
        </button>

        <span className="flex-1 text-center text-[15px] font-semibold text-text">
          {formatMonthYear(anchorIso)}
        </span>

        <button
          type="button"
          aria-label={t('periodClose.nextMonth')}
          className="flex items-center justify-center size-8 rounded-lg border border-border bg-surface hover:bg-surface-2 text-text-2"
          onClick={() => navigateMonth(1)}
        >
          <ChevronRight className="size-4" aria-hidden />
        </button>

        {/* Lock / Reopen actions in period picker row — frame XoMsT CTA */}
        <div className="flex items-center gap-2 ml-4">
          {/* Force-lock warning banner if period has open clarifications */}
          {period.force_locked && (
            <span className="flex items-center gap-1 rounded-md border border-warn-bd bg-warn-bg px-2.5 py-1 text-[11px] font-medium text-warn-tx">
              <ShieldCheck className="size-3.5" aria-hidden />
              {t('periodClose.forceLocked')}
            </span>
          )}

          {/* HR / Super Admin: Lock button (OPEN period) */}
          {isOpen && (
            <Button
              variant={period.lockable || isSuperAdmin ? 'primary' : 'secondary'}
              size="sm"
              disabled={!period.lockable && !isSuperAdmin}
              onClick={() => setLockOpen(true)}
            >
              <Lock className="size-4 mr-1.5" aria-hidden />
              {t('periodClose.lockBtn')}
            </Button>
          )}

          {/* Super Admin: Reopen button (LOCKED period) — PC-12 / PC-15 */}
          {isLocked && isSuperAdmin && (
            <Button variant="secondary" size="sm" onClick={() => setReopenOpen(true)}>
              <LockOpen className="size-4 mr-1.5" aria-hidden />
              {t('periodClose.reopenBtn')}
            </Button>
          )}
        </div>
      </div>

      {/* 3 Blocker StatCards — frame XoMsT stat cards row (OPEN only) */}
      {isOpen && (
        <div className="grid grid-cols-3 gap-4">
          <StatCard
            label={t('periodClose.statAttendance')}
            value={String(blockers.attendance)}
            sub={t('periodClose.statBlockersSub')}
            icon={Users}
            tone={blockers.attendance > 0 ? 'warn' : 'ok'}
          />
          <StatCard
            label={t('periodClose.statOvertime')}
            value={String(blockers.overtime)}
            sub={t('periodClose.statBlockersSub')}
            icon={Clock}
            tone={blockers.overtime > 0 ? 'warn' : 'ok'}
          />
          <StatCard
            label={t('periodClose.statLeave')}
            value={String(blockers.leave)}
            sub={t('periodClose.statBlockersSub')}
            icon={AlertTriangle}
            tone={blockers.leave > 0 ? 'warn' : 'ok'}
          />
        </div>
      )}

      {/* Force-lock warning (super-admin visible when blockers exist) — ties to frame rPFqX banner */}
      {isOpen && hasBlockers && isSuperAdmin && (
        <div className="flex items-start gap-2.5 rounded-xl border border-warn-bd bg-warn-bg px-4 py-3">
          <TriangleAlert className="size-4 text-warn-tx mt-0.5 shrink-0" aria-hidden />
          <p className="text-[13px] text-warn-tx">{t('periodClose.forceAvailable')}</p>
        </div>
      )}

      {/* Table card with tab strip */}
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        <StatusTabs tabs={tabs} />

        {/* Tab content */}
        {activeTab === 'kehadiran' && (
          <CompletenessTable
            year={year}
            month={month}
            period={period}
            search={search}
            onSetSearch={setSearch}
            onClarify={handleClarify}
            openClarifications={openClarifications}
            onManageClarification={handleManageClarification}
            canManageClarifications={canManageClarifications}
          />
        )}
        {activeTab === 'lembur' && (
          <BlockAndLinkTab tab="lembur" blockerCount={blockers.overtime} />
        )}
        {activeTab === 'cuti' && (
          <BlockAndLinkTab tab="cuti" blockerCount={blockers.leave} />
        )}
        {activeTab === 'ringkasan' && isLocked && <SummaryTab year={year} month={month} />}
      </div>

      {/* Overlays — period-close-overlays.tsx */}
      {lockOpen && (
        <LockDialog
          open={lockOpen}
          year={year}
          month={month}
          period={period}
          isSuperAdmin={isSuperAdmin}
          onOpenChange={setLockOpen}
        />
      )}
      {reopenOpen && isSuperAdmin && (
        <ReopenDialog open={reopenOpen} year={year} month={month} onOpenChange={setReopenOpen} />
      )}
      {clarifOpen && clarifSource && (
        <ClarificationDialog
          open={clarifOpen}
          sourceType={clarifSource.sourceType}
          sourceId={clarifSource.sourceId}
          label={clarifSource.label}
          onOpenChange={setClarifOpen}
          onCreated={handleClarificationCreated}
        />
      )}
      {manageClarifOpen && manageClarifTarget && (
        <ManageClarificationDialog
          open={manageClarifOpen}
          clarification={manageClarifTarget.clarification}
          employeeName={manageClarifTarget.employeeName}
          year={year}
          month={month}
          onOpenChange={setManageClarifOpen}
          onSettled={handleClarificationSettled}
        />
      )}
    </div>
  );
}
