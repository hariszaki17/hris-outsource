/**
 * E5 · Verifikasi Kehadiran — Queue (HR + Shift Leader scoped)
 *
 * .pen frames:
 *   UEG2J  HR verification queue — Screen 2 · Verifikasi Kehadiran
 *   MsXnm  SL verification queue — E5 SL · Verifikasi — Plaza Senayan
 *
 * Design: TitleBand → 4× MiniStats → FilterRow (company + type + escalation toggle + search)
 * → QueueCard (BulkBar + Table + Pagination).
 * HR: escalation badge + leader_own filter shortcut.
 * SL: ScopeBanner + company filter locked.
 *
 * F5.3: bulk-select + bulk-verify (ConfirmDialog) + single reject (ConfirmDialog w/ reason).
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { useListClientCompanies } from '@swp/api-client/e2';
import {
  type Attendance,
  AttendanceFlag,
  type AttendancePage,
  AttendanceStatus,
  type ListAttendanceParams,
  VerificationStatus,
  useBulkRejectAttendance,
  useBulkVerifyAttendance,
  useListAttendance,
  useRejectAttendance,
  useVerifyAttendance,
} from '@swp/api-client/e5';
import type { StatusTone } from '@swp/design-tokens';
import {
  Checkbox,
  type Column,
  ConfirmDialog,
  CursorPagination,
  DataTable,
  EmptyState,
  FilterSelect,
  Input,
  SearchField,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { CheckCheck, ClockAlert, MapPinOff, TriangleAlert, XCircle } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

export type AttendanceVerificationSearch = {
  q?: string;
  company_id?: string;
  flag?: AttendanceFlag;
  escalated_only?: boolean;
  cursor?: string;
};

function verificationStatusTone(vs: VerificationStatus): StatusTone {
  switch (vs) {
    case VerificationStatus.PENDING:
      return 'warn';
    case VerificationStatus.ESCALATED:
      return 'bad';
    case VerificationStatus.VERIFIED:
    case VerificationStatus.AUTO_APPROVED:
      return 'ok';
    case VerificationStatus.REJECTED:
      return 'bad';
    default:
      return 'neutral';
  }
}

function attendanceStatusTone(s: AttendanceStatus): StatusTone {
  switch (s) {
    case AttendanceStatus.PRESENT:
      return 'ok';
    case AttendanceStatus.LATE:
      return 'warn';
    case AttendanceStatus.ABSENT:
      return 'bad';
    case AttendanceStatus.INCOMPLETE:
      return 'warn';
    default:
      return 'neutral';
  }
}

// ---------------------------------------------------------------------------
// AttendanceVerificationScreen
// ---------------------------------------------------------------------------

export function AttendanceVerificationScreen() {
  const { t } = useTranslation('attendance');
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as AttendanceVerificationSearch;
  const currentUser = useCurrentUser();
  const { toast } = useToast();
  const isShiftLeader = currentUser?.role === 'shift_leader';

  const [prevCursors, setPrevCursors] = useState<string[]>([]);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  // Modals
  const [bulkVerifyOpen, setBulkVerifyOpen] = useState(false);
  const [bulkRejectOpen, setBulkRejectOpen] = useState(false);
  const [singleVerifyId, setSingleVerifyId] = useState<string | null>(null);
  const [singleRejectId, setSingleRejectId] = useState<string | null>(null);
  const [rejectReason, setRejectReason] = useState('');

  const queryParams: ListAttendanceParams = {
    limit: PAGE_SIZE,
    cursor: search.cursor,
    q: search.q || undefined,
    company_id: search.company_id || undefined,
    flag: search.flag ? [search.flag] : undefined,
    exceptions_only: true,
    source: search.escalated_only && !isShiftLeader ? 'leader_own' : undefined,
    verification_status: [VerificationStatus.PENDING, VerificationStatus.ESCALATED],
  };

  const query = useListAttendance(queryParams);
  const page = query.data?.data as AttendancePage | undefined;
  const rows: Attendance[] = page?.data ?? [];

  // Company options — HR/super_admin only (SL has no company picker; scope is locked server-side).
  const companiesQuery = useListClientCompanies(
    { limit: 200 },
    { query: { enabled: !isShiftLeader, staleTime: 60_000 } },
  );
  const companyOptions = useMemo(() => {
    if (isShiftLeader) return [];
    const cc =
      (companiesQuery.data?.data as { data?: { id: string; name: string }[] } | undefined)?.data ??
      [];
    return cc.map((c) => ({ value: c.id, label: c.name }));
  }, [isShiftLeader, companiesQuery.data]);

  // Mutation hooks
  const verifySingle = useVerifyAttendance();
  const rejectSingle = useRejectAttendance();
  const bulkVerify = useBulkVerifyAttendance();
  const bulkReject = useBulkRejectAttendance();

  const isSaving =
    verifySingle.isPending ||
    rejectSingle.isPending ||
    bulkVerify.isPending ||
    bulkReject.isPending;

  function setSearch(partial: Partial<AttendanceVerificationSearch>) {
    setSelectedIds(new Set());
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    void (
      navigate as (o: {
        to: string;
        search?: Record<string, unknown>;
        params?: Record<string, unknown>;
      }) => void
    )({
      to: '/attendance/verification',
      search: { ...search, ...partial, cursor: undefined },
    });
  }

  // Counts for mini-stats
  const pendingCount = rows.filter(
    (r) => r.verification_status === VerificationStatus.PENDING,
  ).length;
  const lateCount = rows.filter((r) => r.flags.includes(AttendanceFlag.LATE)).length;
  const autoClosedCount = rows.filter((r) => r.flags.includes(AttendanceFlag.AUTO_CLOSED)).length;
  const geofenceCount = rows.filter((r) =>
    r.flags.includes(AttendanceFlag.OUTSIDE_GEOFENCE),
  ).length;

  // Select all
  const allIds = rows.map((r) => r.id);
  const allSelected = allIds.length > 0 && allIds.every((id) => selectedIds.has(id));
  const someSelected = selectedIds.size > 0;

  function toggleAll() {
    if (allSelected) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(allIds));
    }
  }

  function toggleOne(id: string) {
    const next = new Set(selectedIds);
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    setSelectedIds(next);
  }

  // Handlers
  function handleBulkVerifyConfirm() {
    bulkVerify.mutate(
      { data: { ids: Array.from(selectedIds) } },
      {
        onSuccess: () => {
          setBulkVerifyOpen(false);
          setSelectedIds(new Set());
          toast({ tone: 'success', title: t('bulkVerifySuccess', { count: selectedIds.size }) });
        },
        onError: (err) => {
          const e = classifyError(err);
          toast({ tone: 'error', title: t('bulkVerifyError'), description: e.message });
        },
      },
    );
  }

  function handleBulkRejectConfirm() {
    bulkReject.mutate(
      { data: { ids: Array.from(selectedIds), reason: rejectReason } },
      {
        onSuccess: () => {
          setBulkRejectOpen(false);
          setSelectedIds(new Set());
          setRejectReason('');
          toast({ tone: 'success', title: t('bulkRejectSuccess', { count: selectedIds.size }) });
        },
        onError: (err) => {
          const e = classifyError(err);
          toast({ tone: 'error', title: t('bulkRejectError'), description: e.message });
        },
      },
    );
  }

  function handleSingleVerifyConfirm() {
    if (!singleVerifyId) return;
    verifySingle.mutate(
      { id: singleVerifyId, data: {} },
      {
        onSuccess: () => {
          setSingleVerifyId(null);
          toast({ tone: 'success', title: t('verifySuccess') });
        },
        onError: (err) => {
          const e = classifyError(err);
          toast({ tone: 'error', title: t('verifyError'), description: e.message });
        },
      },
    );
  }

  function handleSingleRejectConfirm() {
    if (!singleRejectId) return;
    rejectSingle.mutate(
      { id: singleRejectId, data: { reason: rejectReason } },
      {
        onSuccess: () => {
          setSingleRejectId(null);
          setRejectReason('');
          toast({ tone: 'success', title: t('rejectSuccess') });
        },
        onError: (err) => {
          const e = classifyError(err);
          toast({ tone: 'error', title: t('rejectError'), description: e.message });
        },
      },
    );
  }

  // Columns
  const columns: Column<Attendance>[] = [
    {
      id: 'select',
      header: <Checkbox checked={allSelected} aria-label={t('selectAll')} onChange={toggleAll} />,
      width: 44,
      cell: (row) => (
        <Checkbox
          checked={selectedIds.has(row.id)}
          aria-label={t('selectRow', { name: row.employee_name ?? row.employee_id })}
          onChange={() => toggleOne(row.id)}
        />
      ),
    },
    {
      id: 'employee',
      header: t('colEmployee'),
      width: 200,
      cell: (row) => (
        <div className="flex flex-col gap-[2px]">
          <span className="text-[13px] font-medium text-text">
            {row.employee_name ?? row.employee_id}
          </span>
          <span className="font-mono text-[11px] text-text-3">{row.employee_id}</span>
        </div>
      ),
    },
    {
      id: 'company',
      header: t('colCompany'),
      width: 170,
      cell: (row) => (
        <span className="text-[13px] text-text">{row.company_name ?? row.company_id}</span>
      ),
    },
    {
      id: 'check_in_at',
      header: t('colCheckIn'),
      width: 130,
      cell: (row) => (
        <span className="text-[13px] text-text">
          {row.check_in_at
            ? new Date(row.check_in_at).toLocaleString('id-ID', {
                timeZone: 'Asia/Jakarta',
                dateStyle: 'short',
                timeStyle: 'short',
              })
            : t('colCheckInEmpty')}
        </span>
      ),
    },
    {
      id: 'status',
      header: t('colStatus'),
      width: 110,
      cell: (row) => (
        <StatusBadge dot tone={attendanceStatusTone(row.status)}>
          {t(`status.${row.status}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'verification_status',
      header: t('colVerification'),
      width: 150,
      cell: (row) => (
        <div className="flex items-center gap-2">
          <StatusBadge dot tone={verificationStatusTone(row.verification_status)}>
            {t(`verificationStatus.${row.verification_status}`)}
          </StatusBadge>
          {row.verification_status === VerificationStatus.ESCALATED && !isShiftLeader && (
            <span className="rounded-full bg-bad-bg px-[6px] py-[2px] text-[10px] font-semibold text-bad-tx">
              {t('escalatedBadge')}
            </span>
          )}
        </div>
      ),
    },
    {
      id: 'actions',
      header: '',
      width: 160,
      cell: (row) => (
        <div className="flex items-center gap-2">
          <button
            type="button"
            className="rounded-md border border-border bg-surface px-[10px] py-[5px] text-[12px] font-medium text-text-2 hover:bg-surface-2 disabled:opacity-50"
            disabled={isSaving}
            onClick={(e) => {
              e.stopPropagation();
              setSingleVerifyId(row.id);
            }}
          >
            {t('verifyBtn')}
          </button>
          <button
            type="button"
            className="rounded-md border border-bad/40 bg-surface px-[10px] py-[5px] text-[12px] font-medium text-bad hover:bg-bad-bg disabled:opacity-50"
            disabled={isSaving}
            onClick={(e) => {
              e.stopPropagation();
              setRejectReason('');
              setSingleRejectId(row.id);
            }}
          >
            {t('rejectBtn')}
          </button>
        </div>
      ),
    },
  ];

  const hasFilters = Boolean(search.q || search.company_id || search.flag || search.escalated_only);

  // Error state
  if (query.isError) {
    const err = classifyError(query.error);
    if (err.kind === 'forbidden') {
      return (
        <div className="flex items-center gap-2 rounded-xl border border-border bg-surface px-5 py-[18px]">
          <span className="text-[14px] text-text-2">{t('noPermission')}</span>
        </div>
      );
    }
    return (
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <span className="text-[14px] text-bad">{t('loadError')}</span>
        <button
          type="button"
          className="rounded-lg border border-border bg-surface px-4 py-2 text-[13px] font-medium text-text-2 hover:bg-surface-2"
          onClick={() => query.refetch()}
        >
          {t('retry')}
        </button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[18px]">
      {/* SL scope banner */}
      {isShiftLeader && (
        <div className="flex items-center gap-2 bg-warn-bg px-6 py-[10px] border-b border-warn-bd">
          <span className="text-warn-tx" aria-hidden>
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              aria-hidden="true"
            >
              <title>lock</title>
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
          </span>
          <p className="text-[12px] font-semibold text-warn-tx">{t('scopeBanner')}</p>
        </div>
      )}

      {/* Title band */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl font-bold text-text">{t('verifyTitle')}</h1>
          <p className="text-[13px] text-text-2">{t('verifySubtitle')}</p>
        </div>
      </div>

      {/* Mini-stats */}
      <div className="grid grid-cols-4 gap-3">
        {[
          { label: t('miniPending'), value: pendingCount, icon: TriangleAlert },
          { label: t('miniLate'), value: lateCount, icon: ClockAlert },
          { label: t('miniAutoClosed'), value: autoClosedCount, icon: CheckCheck },
          { label: t('miniGeofence'), value: geofenceCount, icon: MapPinOff },
        ].map(({ label, value, icon: Icon }) => (
          <div
            key={label}
            className="flex items-center gap-3 rounded-[10px] border border-border bg-surface px-4 py-[14px]"
          >
            <span className="shrink-0" aria-hidden>
              <Icon className="size-[18px] text-text-3" aria-hidden />
            </span>
            <div className="flex flex-col">
              <span className="text-[20px] font-bold text-text">
                {query.isLoading ? '—' : String(value)}
              </span>
              <span className="text-[12px] text-text-2">{label}</span>
            </div>
          </div>
        ))}
      </div>

      {/* Filter row */}
      <div className="flex items-center gap-[10px]">
        {!isShiftLeader && (
          <FilterSelect
            aria-label={t('filterCompany')}
            value={search.company_id ?? ''}
            onChange={(e) => setSearch({ company_id: e.target.value || undefined })}
          >
            <option value="">{t('filterCompanyAll')}</option>
            {companyOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </FilterSelect>
        )}
        <FilterSelect
          aria-label={t('filterFlag')}
          value={search.flag ?? ''}
          onChange={(e) => setSearch({ flag: (e.target.value as AttendanceFlag) || undefined })}
        >
          <option value="">{t('filterFlagAll')}</option>
          <option value={AttendanceFlag.LATE}>{t('flagLabel.LATE')}</option>
          <option value={AttendanceFlag.EARLY}>{t('flagLabel.EARLY')}</option>
          <option value={AttendanceFlag.OUTSIDE_GEOFENCE}>{t('flagLabel.OUTSIDE_GEOFENCE')}</option>
          <option value={AttendanceFlag.UNSCHEDULED}>{t('flagLabel.UNSCHEDULED')}</option>
          <option value={AttendanceFlag.AUTO_CLOSED}>{t('flagLabel.AUTO_CLOSED')}</option>
        </FilterSelect>
        {!isShiftLeader && (
          <button
            type="button"
            className={[
              'flex items-center gap-2 rounded-lg border px-3 py-[9px] text-[12px] font-medium',
              search.escalated_only
                ? 'border-bad/40 bg-bad-bg text-bad'
                : 'border-border bg-surface text-text-2 hover:bg-surface-2',
            ].join(' ')}
            onClick={() => setSearch({ escalated_only: !search.escalated_only || undefined })}
          >
            <TriangleAlert className="size-[13px]" aria-hidden />
            {t('filterEscalated')}
          </button>
        )}
        <div className="flex-1" />
        <SearchField
          placeholder={t('searchPlaceholder')}
          defaultValue={search.q ?? ''}
          containerClassName="w-[260px]"
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
        />
      </div>

      {/* Queue card */}
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        {/* Bulk action bar */}
        {someSelected && (
          <div className="flex items-center justify-between bg-primary-soft px-[18px] py-3 border-b border-border-soft">
            <span className="text-[13px] font-medium text-primary">
              {t('selectedCount', { count: selectedIds.size })}
            </span>
            <div className="flex items-center gap-2">
              <button
                type="button"
                className="rounded-lg border border-border bg-surface px-[12px] py-[7px] text-[12px] font-medium text-text-2 hover:bg-surface-2 disabled:opacity-50"
                disabled={isSaving}
                onClick={() => setBulkVerifyOpen(true)}
              >
                {t('bulkVerifyBtn')}
              </button>
              <button
                type="button"
                className="rounded-lg border border-bad/40 bg-surface px-[12px] py-[7px] text-[12px] font-medium text-bad hover:bg-bad-bg disabled:opacity-50"
                disabled={isSaving}
                onClick={() => {
                  setRejectReason('');
                  setBulkRejectOpen(true);
                }}
              >
                {t('bulkRejectBtn')}
              </button>
            </div>
          </div>
        )}

        <DataTable
          aria-label={t('verifyTitle')}
          columns={columns}
          data={rows}
          getRowId={(r) => r.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          onRowClick={(rec) =>
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            void (
              navigate as (o: {
                to: string;
                search?: Record<string, unknown>;
                params?: Record<string, unknown>;
              }) => void
            )({
              to: '/attendance/$attendanceId',
              params: { attendanceId: rec.id },
            })
          }
          empty={
            hasFilters ? (
              <EmptyState
                variant="filtered"
                title={t('filteredTitle')}
                description={t('filteredBody')}
              />
            ) : (
              <EmptyState
                variant="fresh"
                title={t('verifyEmptyTitle')}
                description={t('verifyEmptyBody')}
              />
            )
          }
          footer={
            rows.length > 0 ? (
              <CursorPagination
                rangeLabel={t('resultRange', { count: rows.length })}
                hasPrev={prevCursors.length > 0}
                hasNext={Boolean(page?.has_more)}
                prevLabel={t('common.prev', { ns: 'translation' })}
                nextLabel={t('common.next', { ns: 'translation' })}
                onPrev={() => {
                  const next = [...prevCursors];
                  const cursor = next.pop();
                  setPrevCursors(next);
                  // eslint-disable-next-line @typescript-eslint/no-explicit-any
                  void (
                    navigate as (o: {
                      to: string;
                      search?: Record<string, unknown>;
                      params?: Record<string, unknown>;
                    }) => void
                  )({
                    to: '/attendance/verification',
                    search: { ...search, cursor: cursor || undefined },
                  });
                }}
                onNext={() => {
                  const nextCursor = page?.next_cursor;
                  if (!nextCursor) return;
                  setPrevCursors((s) => [...s, search.cursor ?? '']);
                  // eslint-disable-next-line @typescript-eslint/no-explicit-any
                  void (
                    navigate as (o: {
                      to: string;
                      search?: Record<string, unknown>;
                      params?: Record<string, unknown>;
                    }) => void
                  )({
                    to: '/attendance/verification',
                    search: { ...search, cursor: nextCursor },
                  });
                }}
              />
            ) : undefined
          }
        />
      </div>

      {/* Bulk verify confirm */}
      <ConfirmDialog
        open={bulkVerifyOpen}
        onOpenChange={setBulkVerifyOpen}
        icon={CheckCheck}
        tone="neutral"
        title={t('bulkVerifyDialogTitle')}
        description={t('bulkVerifyDialogDesc', { count: selectedIds.size })}
        cancelLabel={t('cancel')}
        confirmLabel={bulkVerify.isPending ? t('saving') : t('bulkVerifyConfirm')}
        confirmTone="primary"
        loading={bulkVerify.isPending}
        onConfirm={handleBulkVerifyConfirm}
      />

      {/* Bulk reject confirm */}
      <ConfirmDialog
        open={bulkRejectOpen}
        onOpenChange={(open) => {
          setBulkRejectOpen(open);
          if (!open) setRejectReason('');
        }}
        icon={XCircle}
        tone="warn"
        title={t('bulkRejectDialogTitle')}
        description={t('bulkRejectDialogDesc', { count: selectedIds.size })}
        cancelLabel={t('cancel')}
        confirmLabel={bulkReject.isPending ? t('saving') : t('bulkRejectConfirm')}
        confirmTone="danger"
        loading={bulkReject.isPending}
        confirmDisabled={rejectReason.trim().length < 5}
        onConfirm={handleBulkRejectConfirm}
      >
        <div className="mt-3 flex flex-col gap-1">
          <label htmlFor="bulk-reject-reason" className="text-[12px] font-medium text-text-2">
            {t('rejectReasonLabel')}
          </label>
          <Input
            id="bulk-reject-reason"
            placeholder={t('rejectReasonPlaceholder')}
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
          />
        </div>
      </ConfirmDialog>

      {/* Single verify confirm */}
      <ConfirmDialog
        open={singleVerifyId !== null}
        onOpenChange={(open) => {
          if (!open) setSingleVerifyId(null);
        }}
        icon={CheckCheck}
        tone="neutral"
        title={t('verifyDialogTitle')}
        description={t('verifyDialogDesc')}
        cancelLabel={t('cancel')}
        confirmLabel={verifySingle.isPending ? t('saving') : t('verifyConfirm')}
        confirmTone="primary"
        loading={verifySingle.isPending}
        onConfirm={handleSingleVerifyConfirm}
      />

      {/* Single reject confirm */}
      <ConfirmDialog
        open={singleRejectId !== null}
        onOpenChange={(open) => {
          if (!open) {
            setSingleRejectId(null);
            setRejectReason('');
          }
        }}
        icon={XCircle}
        tone="warn"
        title={t('rejectDialogTitle')}
        description={t('rejectDialogDesc')}
        cancelLabel={t('cancel')}
        confirmLabel={rejectSingle.isPending ? t('saving') : t('rejectConfirm')}
        confirmTone="danger"
        loading={rejectSingle.isPending}
        confirmDisabled={rejectReason.trim().length < 5}
        onConfirm={handleSingleRejectConfirm}
      >
        <div className="mt-3 flex flex-col gap-1">
          <label htmlFor="single-reject-reason" className="text-[12px] font-medium text-text-2">
            {t('rejectReasonLabel')}
          </label>
          <Input
            id="single-reject-reason"
            placeholder={t('rejectReasonPlaceholder')}
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
          />
        </div>
      </ConfirmDialog>
    </div>
  );
}
