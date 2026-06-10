import { getCurrentCoords } from '@/lib/geolocation.ts';
/**
 * /me/attendance — Agent web clock-in/out + own attendance history (F5.1 / F5.6 self surface).
 *
 * Web port of apps/mobile/app/(app)/attendance.tsx (docs/eng/AGENT-WEB-ACCESS.md §5), built to
 * the console's web layout: title band + a clock panel + a DataTable history (NOT mobile card
 * stacks). GPS via the browser Geolocation API (lib/geolocation.ts); out-of-geofence override
 * mirrors mobile; photo deferred. The server resolves the agent from the JWT principal — no
 * employee_id is sent. Each history row links to file a correction (7-day window).
 */
import { ApiError } from '@swp/api-client';
import {
  type Attendance,
  type AttendancePage,
  AttendanceStatus,
  useClockIn,
  useClockOut,
  useListAttendance,
} from '@swp/api-client/e5';
import type { StatusTone } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared';
import {
  Button,
  type Column,
  ConfirmDialog,
  DataTable,
  EmptyState,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { Fingerprint, LogIn, LogOut, MapPin } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AgentPage } from './agent-page.tsx';

// check_in_at is null on a true-ABSENT row (E5 true-absence model) — guard the formatters.
function timeOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { timeStyle: 'short' }) : '—';
}
function dateOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { dateStyle: 'medium' }) : '—';
}

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

export function AgentAttendanceScreen() {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const navigate = useNavigate();
  const list = useListAttendance({ limit: 20 });
  const clockIn = useClockIn();
  const clockOut = useClockOut();
  const [busy, setBusy] = useState(false);
  const [geofencePrompt, setGeofencePrompt] = useState<{ distance: string; radius: string } | null>(
    null,
  );

  const body = list.data?.data as AttendancePage | undefined;
  const items: Attendance[] = body?.data ?? [];
  // "open" = actually clocked in (has check-in) and not yet clocked out. An ABSENT row has no
  // check_out_at but no check_in_at either — it must NOT count as open.
  const open = items.find((a) => a.check_in_at && !a.check_out_at);

  function handleClockError(e: unknown) {
    if (e instanceof ApiError) {
      if (e.code === 'OUT_OF_GEOFENCE') {
        setGeofencePrompt({
          distance: String(e.fields?.distance_m ?? '?'),
          radius: String(e.fields?.radius_m ?? '?'),
        });
        return;
      }
      if (e.code === 'ALREADY_CLOCKED_IN') {
        void list.refetch();
        toast({ tone: 'info', title: t('alreadyIn') });
        return;
      }
      if (e.code === 'NOT_CLOCKED_IN') {
        toast({ tone: 'info', title: t('notIn') });
        return;
      }
      if (e.code === 'GPS_UNAVAILABLE') {
        toast({ tone: 'error', title: t('gpsUnavailable') });
        return;
      }
    }
    toast({ tone: 'error', title: t('clockError') });
  }

  async function doClockIn(force: boolean) {
    setBusy(true);
    try {
      const coords = await getCurrentCoords();
      if (!coords) {
        toast({ tone: 'error', title: t('gpsDenied') });
        return;
      }
      try {
        await clockIn.mutateAsync({
          data: {
            lat: coords.lat,
            lng: coords.lng,
            gps_available: true,
            wfo: true,
            force_outside_geofence: force,
          },
        });
        await list.refetch();
        toast({ tone: 'success', title: t('successIn') });
      } catch (e) {
        handleClockError(e);
      }
    } finally {
      setBusy(false);
    }
  }

  async function doClockOut() {
    setBusy(true);
    try {
      const coords = await getCurrentCoords();
      if (!coords) {
        toast({ tone: 'error', title: t('gpsDenied') });
        return;
      }
      try {
        await clockOut.mutateAsync({
          data: { lat: coords.lat, lng: coords.lng, gps_available: true },
        });
        await list.refetch();
        toast({ tone: 'success', title: t('successOut') });
      } catch (e) {
        handleClockError(e);
      }
    } finally {
      setBusy(false);
    }
  }

  const pending = busy || clockIn.isPending || clockOut.isPending;

  const columns: Column<Attendance>[] = [
    {
      id: 'date',
      header: t('clockTitle'),
      width: 200,
      cell: (r) => <span className="font-medium text-text">{dateOf(r.check_in_at)}</span>,
    },
    {
      id: 'in',
      header: t('in'),
      width: 120,
      cell: (r) => (
        <span className="text-sm text-text-2 tabular-nums">{timeOf(r.check_in_at)}</span>
      ),
    },
    {
      id: 'out',
      header: t('out'),
      width: 120,
      cell: (r) => (
        <span className="text-sm text-text-2 tabular-nums">{timeOf(r.check_out_at)}</span>
      ),
    },
    {
      id: 'status',
      header: t('historyTitle'),
      width: 160,
      cell: (r) => (
        <StatusBadge dot tone={attendanceStatusTone(r.status)}>
          {t(`attendance.status.${r.status}`, { ns: 'common', defaultValue: r.status })}
        </StatusBadge>
      ),
    },
    {
      id: 'action',
      header: '',
      width: 120,
      align: 'right',
      cell: (r) => (
        <button
          type="button"
          className="text-sm font-medium text-primary hover:underline"
          onClick={() =>
            navigate({
              to: '/me/correction',
              search: { attendanceId: r.id, date: r.check_in_at ?? r.shift_start_at ?? '' },
            })
          }
        >
          {t('fileCorrection')}
        </button>
      ),
    },
  ];

  return (
    <AgentPage title={t('clockTitle')}>
      {/* Clock panel */}
      <div className="flex items-center justify-between gap-6 rounded-xl border border-border bg-surface p-6">
        <div className="flex items-center gap-4">
          <div
            className={[
              'flex size-12 items-center justify-center rounded-full',
              open ? 'bg-ok-bg text-ok-tx' : 'bg-surface-2 text-text-3',
            ].join(' ')}
          >
            <Fingerprint className="size-6" aria-hidden />
          </div>
          <div className="flex flex-col gap-0.5">
            <span className="text-[12px] text-text-3">{t('clockTitle')}</span>
            <span className="text-[18px] font-semibold text-text">
              {open ? t('clockedInAt', { time: timeOf(open.check_in_at) }) : t('notClockedIn')}
            </span>
          </div>
        </div>
        <Button
          variant="primary"
          size="lg"
          disabled={pending}
          onClick={() => (open ? void doClockOut() : void doClockIn(false))}
        >
          {open ? (
            <LogOut className="size-4" aria-hidden />
          ) : (
            <LogIn className="size-4" aria-hidden />
          )}
          {pending ? t('acquiring') : open ? t('clockOut') : t('clockIn')}
        </Button>
      </div>

      {/* History */}
      <div className="flex flex-col gap-3">
        <h2 className="text-[15px] font-bold text-text">{t('historyTitle')}</h2>
        {list.isError ? (
          <StateView kind="error" title={t('errorGeneric')} onRetry={() => void list.refetch()} />
        ) : (
          <DataTable
            aria-label={t('historyTitle')}
            columns={columns}
            data={items}
            getRowId={(r) => r.id}
            isLoading={list.isLoading}
            skeletonRows={6}
            empty={<EmptyState variant="fresh" title={t('historyEmpty')} />}
          />
        )}
      </div>

      {/* Out-of-geofence override (clock-in only) */}
      <ConfirmDialog
        open={geofencePrompt !== null}
        onOpenChange={(o) => {
          if (!o) setGeofencePrompt(null);
        }}
        icon={MapPin}
        tone="warn"
        title={t('outsideTitle')}
        description={
          geofencePrompt
            ? t('outsideMsg', { distance: geofencePrompt.distance, radius: geofencePrompt.radius })
            : ''
        }
        cancelLabel={t('cancel')}
        confirmLabel={t('clockAnyway')}
        confirmTone="primary"
        loading={pending}
        onConfirm={() => {
          setGeofencePrompt(null);
          void doClockIn(true);
        }}
      />
    </AgentPage>
  );
}
