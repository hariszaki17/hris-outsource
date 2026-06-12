/**
 * E10 · Laporan Kehadiran & Jam Billable (HR)
 *
 * .pen frames implemented:
 *   EF8AZ  "E10 · Laporan Kehadiran & Jam Billable (HR)"  — report screen
 *
 * Routes:
 *   /reports/billable   (permission: reports.read → hr_admin | super_admin only)
 *
 * NOTE: shift_leader has NO reports.read in the authoritative role model
 * (rbac.ts ROLE_PERMISSIONS.shift_leader; NAVIGATION-AND-RBAC.md — SL has "no reports").
 * The nav item and the API gate both exclude SL, so this screen is HR/super-admin only.
 * (Per CLAUDE.md the eng RBAC doc wins over the per-feature PRD BR-4, which is stale.)
 *
 * Frame layout (EF8AZ):
 *   Sidebar + Main column:
 *     TitleBand  (title · subtitle)
 *     Filters    (Periode · Perusahaan · Posisi · Kelompok)
 *     PendingCallout (warn Banner — unverified records excluded from billable)
 *     Stats      (4 × StatCard: Jam Billable · Jam Payable · Total Worked · Tingkat Verifikasi)
 *     Table      (group_key rows: AGEN/HARI/SHIFT · PERUSAHAAN · JAM KERJA · JAM BILLABLE ·
 *                 JAM PAYABLE · REKAMAN)
 *     Table footer pagination
 *
 * F10.3 · BR-1..BR-7 · INV-4 (billable = verified-only).
 */

import { PositionPicker } from '@/features/e2-identity/pickers/position-picker.tsx';
import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { useCompanyOptions } from '@/lib/use-company-options.ts';
import {
  type BillableReport,
  type BillableReportRow,
  ExportFormat,
  type ExportJob,
  ExportStatus,
  GetBillableAttendanceReportGroupBy,
  type GetBillableAttendanceReportParams,
  ReportType,
  useCreateExport,
  useGetBillableAttendanceReport,
  useGetExport,
} from '@swp/api-client/e10';
import {
  Banner,
  Button,
  type Column,
  CursorPagination,
  DataTable,
  EmptyState,
  ExportModal,
  type ExportStep,
  FilterSelect,
  StatCard,
  StateView,
  StatusBadge,
} from '@swp/ui';
import { BarChart3, Clock, Download, FileCheck2, RotateCcw, ShieldAlert } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** Local filter state — not URL-serialized; report is stateless navigation-wise. */
export type BillableReportSearch = {
  period_start?: string;
  period_end?: string;
  company_id?: string;
  position?: string;
  group_by?: GetBillableAttendanceReportGroupBy;
};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/** Current month bounds — safe defaults for initial render. */
const today = new Date();
const DEFAULT_PERIOD_START = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-01`;
const DEFAULT_PERIOD_END = new Date(today.getFullYear(), today.getMonth() + 1, 0)
  .toISOString()
  .slice(0, 10);

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatHours(h: number): string {
  return `${h.toLocaleString('id-ID')} j`;
}

function hasActiveFilters(s: BillableReportSearch): boolean {
  // A non-default group_by is an active filter too, so the Reset affordance surfaces when only
  // the grouping has changed (resetFilters restores group_by to `employee`).
  const groupByChanged = Boolean(
    s.group_by && s.group_by !== GetBillableAttendanceReportGroupBy.employee,
  );
  return Boolean(s.company_id || s.position) || groupByChanged;
}

// ---------------------------------------------------------------------------
// Inner screen component
// ---------------------------------------------------------------------------

interface BillableReportScreenInnerProps {
  filters: BillableReportSearch;
  onFilters: (patch: BillableReportSearch) => void;
}

function BillableReportScreenInner({ filters, onFilters }: BillableReportScreenInnerProps) {
  const { t } = useTranslation();
  const user = useCurrentUser();
  const isHR = user?.role === 'hr_admin' || user?.role === 'super_admin';

  // Filter option lists. Company picker is HR-only (SL is server-scoped to one company);
  // the position filter (free-text typeahead) is available to everyone who can reach the screen.
  const { options: companyOptions } = useCompanyOptions({ enabled: isHR });

  const periodStart = filters.period_start ?? DEFAULT_PERIOD_START;
  const periodEnd = filters.period_end ?? DEFAULT_PERIOD_END;

  // ---------------------------------------------------------------------------
  // Query
  // ---------------------------------------------------------------------------

  const params: GetBillableAttendanceReportParams = {
    period_start: periodStart,
    period_end: periodEnd,
    company_id: filters.company_id || undefined,
    position: filters.position || undefined,
    group_by: filters.group_by || undefined,
  };

  const query = useGetBillableAttendanceReport(params);

  // ---------------------------------------------------------------------------
  // Export flow — "Ekspor" → POST /exports (ATTENDANCE_BILLABLE, EXCEL) → poll
  // GET /exports/{id} until COMPLETED → ExportModal success step ("Unduh").
  // F10.4 · EX-1..EX-6. ExportModal (.pen FJ6hX) is the canonical multi-step modal.
  // ---------------------------------------------------------------------------

  const [exportOpen, setExportOpen] = useState(false);
  const [exportJobId, setExportJobId] = useState<string | null>(null);
  const createExport = useCreateExport();

  // Unwrap envelopes like the report query: Orval customFetch wraps in { data,status,headers }
  // and the BE wraps the ExportJob in { data: <ExportJob> }.
  function unwrapJob(raw: unknown): ExportJob | undefined {
    const outer = (raw as { data?: { data?: ExportJob } | ExportJob } | undefined)?.data;
    return ((outer as { data?: ExportJob } | undefined)?.data ??
      (outer as ExportJob | undefined)) as ExportJob | undefined;
  }

  const exportPoll = useGetExport(exportJobId ?? '', {
    query: {
      enabled: Boolean(exportJobId),
      refetchInterval: (q) => {
        const job = unwrapJob(q.state.data);
        return job && (job.status === ExportStatus.QUEUED || job.status === ExportStatus.PROCESSING)
          ? 2500
          : false;
      },
    },
  });
  const exportJob = unwrapJob(exportPoll.data);

  const exportStep: ExportStep = createExport.isError
    ? 'error'
    : exportJob?.status === ExportStatus.COMPLETED
      ? 'success'
      : exportJob?.status === ExportStatus.FAILED || exportJob?.status === ExportStatus.CANCELLED
        ? 'error'
        : exportJobId
          ? 'progress'
          : 'format';

  function openExport() {
    setExportJobId(null);
    createExport.reset();
    setExportOpen(true);
  }

  function runExport() {
    createExport.mutate(
      {
        data: {
          report_type: ReportType.ATTENDANCE_BILLABLE,
          format: ExportFormat.EXCEL,
          filters: {
            period_start: periodStart,
            period_end: periodEnd,
            company_id: filters.company_id || undefined,
            position: filters.position || undefined,
            group_by: filters.group_by || undefined,
          },
        },
      },
      {
        onSuccess: (res) => {
          const job = unwrapJob(res);
          if (job?.id) setExportJobId(job.id);
        },
      },
    );
  }

  // ---------------------------------------------------------------------------
  // Helpers to update filters
  // ---------------------------------------------------------------------------

  function setFilter(patch: BillableReportSearch) {
    onFilters({ ...filters, ...patch });
  }

  function resetFilters() {
    onFilters({
      period_start: DEFAULT_PERIOD_START,
      period_end: DEFAULT_PERIOD_END,
      group_by: GetBillableAttendanceReportGroupBy.employee,
    });
  }

  // ---------------------------------------------------------------------------
  // Error states
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <div className="flex flex-col gap-[18px]">
          <TitleBand />
          <EmptyState
            variant="no-permission"
            title={t('report.noPermissionTitle')}
            description={t('report.noPermissionBody')}
          />
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-[18px]">
        <TitleBand />
        <StateView
          kind="error"
          title={t('report.errorTitle')}
          description={t('report.errorBody')}
          onRetry={() => query.refetch()}
          retryLabel={t('report.retry')}
        />
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Data
  // ---------------------------------------------------------------------------

  // Unwrap BOTH envelopes: Orval's customFetch wraps the HTTP body in { data, status,
  // headers }, and the BE handler wraps the BillableReport in { data: <BillableReport> }
  // (dataResponse) even though the E10 openapi declares the bare BillableReport. The real
  // report lives at query.data.data.data — fixed toward what the BE returns (recurring
  // {data}-unwrap; cf. [08-04]/[10-04]). Bare fallback keeps it robust if it ever flattens.
  const outer = (query.data as { data?: { data?: BillableReport } | BillableReport } | undefined)
    ?.data;
  const report = ((outer as { data?: BillableReport } | undefined)?.data ??
    (outer as BillableReport | undefined)) as BillableReport | undefined;
  const rows: BillableReportRow[] = report?.rows ?? [];
  const summary = report?.summary;
  const pending = report?.pending_summary;

  // ---------------------------------------------------------------------------
  // Columns
  // Frame columns (EF8AZ): AGEN/HARI/SHIFT (250) · PERUSAHAAN (190) ·
  //   JAM KERJA (170) · JAM BILLABLE (150) · JAM PAYABLE (150) · REKAMAN (156)
  // ---------------------------------------------------------------------------

  const groupByLabel =
    filters.group_by === GetBillableAttendanceReportGroupBy.position
      ? t('report.groupBy.position')
      : filters.group_by === GetBillableAttendanceReportGroupBy.day
        ? t('report.groupBy.day')
        : filters.group_by === GetBillableAttendanceReportGroupBy.shift_master
          ? t('report.groupBy.shift_master')
          : t('report.groupBy.employee');

  const columns: Column<BillableReportRow>[] = [
    {
      id: 'group',
      header: groupByLabel,
      width: 250,
      cell: (r) => (
        <div className="flex flex-col gap-0.5">
          <span className="text-sm font-semibold text-text">{r.group_label}</span>
          <span className="font-mono text-[11px] text-text-3">{r.group_key}</span>
        </div>
      ),
    },
    {
      id: 'company',
      header: t('report.colCompany'),
      width: 190,
      cell: (r) => (
        <div className="flex flex-col gap-0.5">
          <span className="text-sm text-text-2">{r.company_name ?? '—'}</span>
          {r.position && <span className="text-[11px] text-text-3">{r.position}</span>}
        </div>
      ),
    },
    {
      id: 'worked',
      header: t('report.colWorked'),
      width: 170,
      cell: (r) => (
        <span className="font-mono text-sm text-text">{formatHours(r.worked_hours)}</span>
      ),
    },
    {
      id: 'billable',
      header: t('report.colBillable'),
      width: 150,
      cell: (r) => (
        <span className="font-mono text-sm font-semibold text-primary-strong">
          {formatHours(r.billable_hours)}
        </span>
      ),
    },
    {
      id: 'payable',
      header: t('report.colPayable'),
      width: 150,
      cell: (r) => (
        <span className="font-mono text-sm text-text">{formatHours(r.payable_hours)}</span>
      ),
    },
    {
      id: 'records',
      header: t('report.colRecords'),
      width: 156,
      cell: (r) => (
        <div className="flex flex-col gap-0.5">
          <div className="flex items-center gap-1.5">
            <StatusBadge dot tone="ok">
              {r.verified_record_count} {t('report.verified')}
            </StatusBadge>
          </div>
          {r.unverified_record_count > 0 && (
            <div className="flex items-center gap-1.5">
              <StatusBadge dot tone="warn">
                {r.unverified_record_count} {t('report.pending')}
              </StatusBadge>
            </div>
          )}
        </div>
      ),
    },
  ];

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  const hasFilters = hasActiveFilters(filters);

  return (
    /* Title band — EF8AZ TitleBand */
    <div className="flex flex-col gap-[18px]">
      <TitleBand onExport={openExport} />

      {/* Filters — EF8AZ Filters strip */}
      <div className="flex flex-wrap items-center gap-2.5">
        {/* Period start */}
        <div className="flex items-center gap-1.5">
          <input
            type="date"
            aria-label={t('report.filterPeriodFrom')}
            className="rounded-md border border-border bg-surface px-2.5 py-1.5 text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/30"
            value={periodStart}
            onChange={(e) => setFilter({ period_start: e.target.value || DEFAULT_PERIOD_START })}
          />
          <span className="text-sm text-text-3">–</span>
          <input
            type="date"
            aria-label={t('report.filterPeriodTo')}
            className="rounded-md border border-border bg-surface px-2.5 py-1.5 text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/30"
            value={periodEnd}
            onChange={(e) => setFilter({ period_end: e.target.value || DEFAULT_PERIOD_END })}
          />
        </div>

        {/* Company — HR only (SL is server-scoped) */}
        {isHR && (
          <FilterSelect
            aria-label={t('report.filterCompany')}
            value={filters.company_id ?? ''}
            onChange={(e) => setFilter({ company_id: e.target.value || undefined })}
          >
            <option value="">{t('report.filterCompany')}</option>
            {companyOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </FilterSelect>
        )}

        {/* Position — free-text typeahead (replaces the former service-line filter) */}
        <div className="w-[200px]">
          <PositionPicker
            value={filters.position ?? null}
            onChange={(v) => setFilter({ position: v ?? undefined })}
            placeholder={t('report.filterPosition')}
          />
        </div>

        {/* Group by — EF8AZ "Kelompok: per agen" */}
        <FilterSelect
          aria-label={t('report.filterGroupBy')}
          value={filters.group_by ?? ''}
          onChange={(e) =>
            setFilter({
              group_by: (e.target.value as GetBillableAttendanceReportGroupBy) || undefined,
            })
          }
        >
          <option value="">{t('report.filterGroupByDefault')}</option>
          {Object.values(GetBillableAttendanceReportGroupBy).map((g) => (
            <option key={g} value={g}>
              {t(`report.groupBy.${g}`)}
            </option>
          ))}
        </FilterSelect>

        {hasFilters && (
          <>
            <div className="h-6 w-px bg-border" />
            <Button type="button" variant="ghost" onClick={resetFilters}>
              <RotateCcw aria-hidden className="size-3.5" />
              {t('report.resetFilters')}
            </Button>
          </>
        )}
      </div>

      {/* Pending-records callout — EF8AZ PendingCallout (warn, INV-4 / BR-6 / C-1) */}
      {/* Always render when pending_summary has data; hide only while loading with no report yet */}
      {pending && pending.pending_records > 0 && (
        <Banner
          tone="warn"
          icon={ShieldAlert}
          title={t('report.pendingCalloutTitle', {
            count: pending.pending_records,
            hours: pending.pending_hours_estimate.toFixed(1),
          })}
          description={t('report.pendingCalloutBody')}
        />
      )}

      {/* Stat cards — EF8AZ Stats row */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard
          label={t('report.statBillable')}
          value={summary ? formatHours(summary.total_billable_hours) : '—'}
          sub={t('report.statBillableSub')}
          icon={FileCheck2}
          tone="brand"
        />
        <StatCard
          label={t('report.statPayable')}
          value={summary ? formatHours(summary.total_payable_hours) : '—'}
          sub={t('report.statPayableSub')}
          icon={BarChart3}
          tone="info"
        />
        <StatCard
          label={t('report.statWorked')}
          value={summary ? formatHours(summary.total_worked_hours) : '—'}
          sub={t('report.statWorkedSub')}
          icon={Clock}
          tone="neutral"
        />
        <StatCard
          label={t('report.statVerificationRate')}
          value={
            summary?.verification_rate_pct != null
              ? `${summary.verification_rate_pct.toFixed(0)}%`
              : '—'
          }
          sub={`${periodStart.slice(0, 7)}`}
          icon={ShieldAlert}
          tone="ok"
        />
      </div>

      {/* Table — EF8AZ Table (H + rows + Foot) */}
      <DataTable
        aria-label={t('report.tableAriaLabel')}
        columns={columns}
        data={rows}
        getRowId={(r) => r.group_key}
        isLoading={query.isLoading}
        skeletonRows={6}
        empty={
          hasFilters || (filters.period_start && filters.period_end) ? (
            <EmptyState
              variant="filtered"
              title={t('report.filteredTitle')}
              description={t('report.filteredBody')}
            />
          ) : (
            <EmptyState
              variant="fresh"
              title={t('report.emptyTitle')}
              description={t('report.emptyBody')}
            />
          )
        }
        footer={
          report?.generated_at ? (
            <CursorPagination
              rangeLabel={t('report.resultCount', { count: rows.length })}
              hasPrev={false}
              hasNext={false}
              prevLabel={t('report.prev')}
              nextLabel={t('report.next')}
              onPrev={() => {}}
              onNext={() => {}}
            />
          ) : undefined
        }
      />

      {/* Export modal — EF8AZ "Ekspor" → .pen FJ6hX. XLSX-only (D5). */}
      <ExportModal
        open={exportOpen}
        onOpenChange={setExportOpen}
        step={exportStep}
        labels={{ title: t('report.exportModalTitle') }}
        rangeStart={periodStart}
        rangeEnd={periodEnd}
        onExport={runExport}
        exporting={createExport.isPending || exportStep === 'progress'}
        progressPct={exportJob?.progress_percent ?? 0}
        onDownload={() => {
          if (exportJob?.file_url) window.open(exportJob.file_url, '_blank', 'noopener');
        }}
        onRetry={openExport}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// TitleBand — EF8AZ TitleBand (title · subtitle)
// ---------------------------------------------------------------------------

function TitleBand({ onExport }: { onExport?: () => void }) {
  const { t } = useTranslation();
  return (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-bold text-text">{t('report.title')}</h1>
        <p className="text-sm text-text-3">{t('report.subtitle')}</p>
      </div>
      {onExport && (
        <Button type="button" onClick={onExport}>
          <Download aria-hidden className="size-4" />
          {t('report.exportBtn')}
        </Button>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public export — BillableReportScreen
// ---------------------------------------------------------------------------

/**
 * BillableReportScreen — F10.3 Laporan Kehadiran & Jam Billable (HR).
 *
 * Proposed route: /reports/billable
 * No URL search params — filters are local state (report is non-bookmarkable by design;
 * HR runs ad-hoc queries). Use `BillableReportSearch` as the local state type if needed.
 */
export function BillableReportScreen() {
  const [filters, setFilters] = useState<BillableReportSearch>({
    period_start: DEFAULT_PERIOD_START,
    period_end: DEFAULT_PERIOD_END,
    group_by: GetBillableAttendanceReportGroupBy.employee,
  });

  return <BillableReportScreenInner filters={filters} onFilters={(patch) => setFilters(patch)} />;
}
