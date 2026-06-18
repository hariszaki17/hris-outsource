/**
 * E2 · Karyawan — Detail · cross-epic tab bodies.
 *
 * Read-only embedded sections that wire the employee-detail tabs to live data instead of the
 * old CrossEpicPlaceholder:
 *   Penempatan    → E3 useListPlacements({ employee_id })
 *   Kehadiran     → E5 useListAttendance({ employee_id })   (most-recent window)
 *   Cuti & Lembur → E6 useListLeaveRequests + E7 useListOvertime ({ employee_id })
 *
 * Each is a thin, employee-scoped table (DataTable + StateView error + EmptyState), mirroring the
 * full list screens but without filters/pagination — the dedicated screens own deep analysis.
 * Rows deep-link into the per-record detail route. Status colour only via StatusBadge tones (G3/G4);
 * status labels reuse the source-epic catalogs via full-path keys (i18n fallbackNS = translation).
 */
import {
  type Placement,
  type PlacementLifecycleStatus,
  useListPlacements,
} from '@swp/api-client/e3';
import {
  type Attendance,
  AttendanceStatus,
  type VerificationStatus,
  useListAttendance,
} from '@swp/api-client/e5';
import { type LeaveRequest, useListLeaveRequests } from '@swp/api-client/e6';
import { type Overtime, useListOvertime } from '@swp/api-client/e7';
import type { StatusTone } from '@swp/design-tokens';
import {
  Button,
  type Column,
  DataTable,
  DateText,
  EmptyState,
  StateView,
  StatusBadge,
} from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { leaveStatusTone } from '../e6-leave/leave-overlays.tsx';
import {
  formatOtMinutes,
  overtimeStatusKey,
  overtimeStatusTone,
} from '../e7-overtime/overtime-shared.tsx';

const EMBED_LIMIT = 25;

/** Time-only render options for an instant (Asia/Jakarta TZ applied by formatInstant). */
const TIME_ONLY: Intl.DateTimeFormatOptions = { hour: '2-digit', minute: '2-digit' };

// ---------------------------------------------------------------------------
// Status → tone maps (local copies; the source screens keep their own privately)
// ---------------------------------------------------------------------------

const lifecycleTone: Record<PlacementLifecycleStatus, StatusTone> = {
  ACTIVE: 'ok',
  EXTENDED: 'ok',
  PENDING_START: 'info',
  EXPIRING: 'warn',
  ENDED: 'neutral',
  TRANSFERRED: 'neutral',
  SUPERSEDED: 'neutral',
  TERMINATED: 'bad',
  RESIGNED: 'bad',
};

function attendanceStatusTone(status: AttendanceStatus): StatusTone {
  switch (status) {
    case AttendanceStatus.PRESENT:
      return 'ok';
    case AttendanceStatus.LATE:
    case AttendanceStatus.INCOMPLETE:
      return 'warn';
    case AttendanceStatus.ABSENT:
      return 'bad';
    case AttendanceStatus.ON_LEAVE:
      return 'info';
    default:
      return 'neutral';
  }
}

function verificationStatusTone(vs: VerificationStatus): StatusTone {
  switch (vs) {
    case 'VERIFIED':
    case 'AUTO_APPROVED':
      return 'ok';
    case 'PENDING':
    case 'ESCALATED':
      return 'warn';
    case 'REJECTED':
      return 'bad';
    default:
      return 'neutral';
  }
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

function unwrap<T>(data: unknown): T[] {
  return ((data as { data?: { data?: T[] } } | undefined)?.data?.data ?? []) as T[];
}

/** Card shell matching the e2 DetailCard look (kept local to avoid a cross-screen import). */
function SectionCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="overflow-hidden rounded-xl border border-border bg-surface">
      <div className="flex items-center justify-between border-b border-border-soft px-[18px] py-[13px]">
        <span className="text-[14px] font-bold text-text">{title}</span>
      </div>
      <div className="px-[6px] py-[2px]">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Penempatan (E3)
// ---------------------------------------------------------------------------

export function EmployeePlacementsSection({ employeeId }: { employeeId: string }) {
  const { t } = useTranslation('employees');
  const navigate = useNavigate();
  const query = useListPlacements({ employee_id: employeeId, limit: EMBED_LIMIT });
  const rows = unwrap<Placement>(query.data);

  const columns: Column<Placement>[] = [
    {
      id: 'company',
      header: t('embedColCompany'),
      flex: 2,
      cell: (pl) => <span className="text-[13px] text-text">{pl.client_company_name ?? '—'}</span>,
    },
    {
      id: 'site',
      header: t('embedColSite'),
      flex: 1.6,
      cell: (pl) => <span className="text-[13px] text-text-2">{pl.site_name ?? '—'}</span>,
    },
    {
      id: 'period',
      header: t('embedColPeriod'),
      flex: 1.6,
      cell: (pl) => (
        <span className="text-[13px] text-text-2 tabular-nums">
          <DateText kind="date" value={pl.start_date} />
          {pl.end_date ? (
            <>
              {' – '}
              <DateText kind="date" value={pl.end_date} />
            </>
          ) : null}
        </span>
      ),
    },
    {
      id: 'status',
      header: t('embedColStatus'),
      flex: 1.4,
      cell: (pl) => (
        <StatusBadge dot tone={lifecycleTone[pl.lifecycle_status]}>
          {t(`placements.lifecycle.${pl.lifecycle_status}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'detail',
      header: '',
      width: 96,
      align: 'right',
      cell: (pl) => (
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            void navigate({ to: '/placements/$placementId', params: { placementId: pl.id } })
          }
        >
          {t('common.detail', { ns: 'translation' })}
        </Button>
      ),
    },
  ];

  if (query.isError) {
    return (
      <SectionCard title={t('tabDetailPenempatan')}>
        <div className="p-3">
          <StateView kind="error" title={t('embedErrorTitle')} onRetry={() => query.refetch()} />
        </div>
      </SectionCard>
    );
  }

  return (
    <SectionCard title={t('tabDetailPenempatan')}>
      <DataTable
        aria-label={t('tabDetailPenempatan')}
        columns={columns}
        data={rows}
        getRowId={(p) => p.id}
        isLoading={query.isLoading}
        skeletonRows={3}
        empty={
          <EmptyState
            variant="fresh"
            title={t('embedPlacementsEmptyTitle')}
            description={t('embedPlacementsEmptyBody')}
          />
        }
      />
    </SectionCard>
  );
}

// ---------------------------------------------------------------------------
// Kehadiran (E5) — most-recent window
// ---------------------------------------------------------------------------

export function EmployeeAttendanceSection({ employeeId }: { employeeId: string }) {
  const { t } = useTranslation('employees');
  const navigate = useNavigate();
  const query = useListAttendance({ employee_id: employeeId, limit: EMBED_LIMIT });
  const rows = unwrap<Attendance>(query.data);

  const columns: Column<Attendance>[] = [
    {
      id: 'date',
      header: t('embedColDate'),
      flex: 1.4,
      cell: (row) => (
        <span className="text-[13px] text-text tabular-nums">
          <DateText kind="date" value={row.check_in_at ?? row.created_at} />
        </span>
      ),
    },
    {
      id: 'window',
      header: t('embedColWindow'),
      flex: 1.4,
      // check_in_at / check_out_at are INSTANTS (RFC 3339 `…Z`), not plain shift times — render
      // them via the instant formatter with time-only options (kind="time"/formatLocalTime only
      // accepts a bare "HH:MM" and throws "Z designator not supported" on an instant).
      cell: (row) => (
        <span className="text-[13px] text-text-2 tabular-nums">
          {row.check_in_at ? (
            <DateText kind="instant" value={row.check_in_at} options={TIME_ONLY} />
          ) : (
            <span className="text-text-3">—</span>
          )}
          {' – '}
          {row.check_out_at ? (
            <DateText kind="instant" value={row.check_out_at} options={TIME_ONLY} />
          ) : (
            <span className="text-text-3">—</span>
          )}
        </span>
      ),
    },
    {
      id: 'status',
      header: t('embedColStatus'),
      flex: 1.2,
      cell: (row) => (
        <StatusBadge dot tone={attendanceStatusTone(row.status)}>
          {t(`attendance.status.${row.status}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'verification',
      header: t('embedColVerification'),
      flex: 1.4,
      cell: (row) => (
        <StatusBadge dot tone={verificationStatusTone(row.verification_status)}>
          {t(`attendance.verificationStatus.${row.verification_status}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'detail',
      header: '',
      width: 96,
      align: 'right',
      cell: (row) => (
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            void navigate({ to: '/attendance/$attendanceId', params: { attendanceId: row.id } })
          }
        >
          {t('common.detail', { ns: 'translation' })}
        </Button>
      ),
    },
  ];

  if (query.isError) {
    return (
      <SectionCard title={t('tabDetailKehadiran')}>
        <div className="p-3">
          <StateView kind="error" title={t('embedErrorTitle')} onRetry={() => query.refetch()} />
        </div>
      </SectionCard>
    );
  }

  return (
    <SectionCard title={t('embedAttendanceTitle')}>
      <DataTable
        aria-label={t('tabDetailKehadiran')}
        columns={columns}
        data={rows}
        getRowId={(r) => r.id}
        isLoading={query.isLoading}
        skeletonRows={4}
        empty={
          <EmptyState
            variant="fresh"
            title={t('embedAttendanceEmptyTitle')}
            description={t('embedAttendanceEmptyBody')}
          />
        }
      />
    </SectionCard>
  );
}

// ---------------------------------------------------------------------------
// Cuti & Lembur (E6 + E7) — two stacked tables
// ---------------------------------------------------------------------------

export function EmployeeLeaveOvertimeSection({ employeeId }: { employeeId: string }) {
  const { t } = useTranslation('employees');
  const navigate = useNavigate();

  const leaveQ = useListLeaveRequests({ employee_id: employeeId, limit: EMBED_LIMIT });
  const otQ = useListOvertime({ employee_id: employeeId, limit: EMBED_LIMIT });

  const leaveRows = unwrap<LeaveRequest>(leaveQ.data);
  const otRows = unwrap<Overtime>(otQ.data);

  const leaveColumns: Column<LeaveRequest>[] = [
    {
      id: 'type',
      header: t('embedColLeaveType'),
      flex: 1.8,
      cell: (r) => (
        <span className="text-[13px] text-text">{r.leave_type_name ?? r.leave_type_id}</span>
      ),
    },
    {
      id: 'dates',
      header: t('embedColPeriod'),
      flex: 2,
      cell: (r) => (
        <span className="text-[13px] text-text-2 tabular-nums">
          <DateText kind="date" value={r.start_date} /> –{' '}
          <DateText kind="date" value={r.end_date} />
        </span>
      ),
    },
    {
      id: 'status',
      header: t('embedColStatus'),
      flex: 1.4,
      cell: (r) => (
        <StatusBadge dot tone={leaveStatusTone(r.status)}>
          {t(`leave.status.${r.status}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'detail',
      header: '',
      width: 96,
      align: 'right',
      cell: (r) => (
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            void navigate({ to: '/leave/$leaveRequestId', params: { leaveRequestId: r.id } })
          }
        >
          {t('common.detail', { ns: 'translation' })}
        </Button>
      ),
    },
  ];

  const otColumns: Column<Overtime>[] = [
    {
      id: 'workDate',
      header: t('embedColWorkDate'),
      flex: 1.6,
      cell: (r) => (
        <span className="text-[13px] text-text tabular-nums">
          <DateText kind="date" value={r.work_date} />
        </span>
      ),
    },
    {
      id: 'total',
      header: t('embedColOtTotal'),
      flex: 1.4,
      cell: (r) => (
        <span className="font-mono text-[13px] font-medium text-text">
          {formatOtMinutes(r.calculation.counted_minutes)}
        </span>
      ),
    },
    {
      id: 'status',
      header: t('embedColStatus'),
      flex: 1.4,
      cell: (r) => (
        <StatusBadge dot tone={overtimeStatusTone(r.status)}>
          {t(`overtime.${overtimeStatusKey(r.status)}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'detail',
      header: '',
      width: 96,
      align: 'right',
      cell: (r) => (
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            void navigate({ to: '/overtime/$overtimeId', params: { overtimeId: r.id } })
          }
        >
          {t('common.detail', { ns: 'translation' })}
        </Button>
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-4">
      <SectionCard title={t('embedLeaveTitle')}>
        {leaveQ.isError ? (
          <div className="p-3">
            <StateView kind="error" title={t('embedErrorTitle')} onRetry={() => leaveQ.refetch()} />
          </div>
        ) : (
          <DataTable
            aria-label={t('embedLeaveTitle')}
            columns={leaveColumns}
            data={leaveRows}
            getRowId={(r) => r.id}
            isLoading={leaveQ.isLoading}
            skeletonRows={3}
            empty={
              <EmptyState
                variant="fresh"
                title={t('embedLeaveEmptyTitle')}
                description={t('embedLeaveEmptyBody')}
              />
            }
          />
        )}
      </SectionCard>

      <SectionCard title={t('embedOvertimeTitle')}>
        {otQ.isError ? (
          <div className="p-3">
            <StateView kind="error" title={t('embedErrorTitle')} onRetry={() => otQ.refetch()} />
          </div>
        ) : (
          <DataTable
            aria-label={t('embedOvertimeTitle')}
            columns={otColumns}
            data={otRows}
            getRowId={(r) => r.id}
            isLoading={otQ.isLoading}
            skeletonRows={3}
            empty={
              <EmptyState
                variant="fresh"
                title={t('embedOvertimeEmptyTitle')}
                description={t('embedOvertimeEmptyBody')}
              />
            }
          />
        )}
      </SectionCard>
    </div>
  );
}
