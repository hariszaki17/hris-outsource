import { getCurrentCoords } from '@/lib/geolocation.ts';
/**
 * /me/attendance — Agent web clock-in/out + own attendance history (F5.1 / F5.6 self surface).
 *
 * Web port of apps/mobile/app/(app)/attendance.tsx (docs/eng/AGENT-WEB-ACCESS.md §5). GPS via the
 * browser Geolocation API (lib/geolocation.ts); out-of-geofence override mirrors mobile; photo is
 * deferred (photo_id stays unwired). The server resolves the agent from the JWT principal — no
 * employee_id is sent. History below is the agent's own records (server self-filtered); each row
 * links to file a correction within the 7-day window.
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
import { Button, ConfirmDialog, StateView, StatusBadge, useToast } from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { Clock, Fingerprint, MapPin } from 'lucide-react';
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
  // Out-of-geofence confirm dialog state (carries the distance/radius for the message).
  const [geofencePrompt, setGeofencePrompt] = useState<{ distance: string; radius: string } | null>(
    null,
  );

  const body = list.data?.data as AttendancePage | undefined;
  const items: Attendance[] = body?.data ?? [];
  // "open" = actually clocked in (has check-in) and not yet clocked out. An ABSENT row has no
  // check_out_at but no check_in_at either — it must NOT count as open.
  const open = items.find((a) => a.check_in_at && !a.check_out_at);

  function handleClockError(e: unknown, onForce?: () => void) {
    if (e instanceof ApiError) {
      if (e.code === 'OUT_OF_GEOFENCE' && onForce) {
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
        handleClockError(e, () => undefined);
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

  return (
    <AgentPage title={t('clockTitle')}>
      {/* Clock card */}
      <div className="rounded-xl border border-border bg-surface p-6">
        <div className="flex items-center gap-2 text-text-3">
          <Fingerprint size={16} aria-hidden />
          <span className="text-[12px]">{t('clockTitle')}</span>
        </div>
        <p className="mt-2 text-[18px] font-semibold text-text">
          {open ? t('clockedInAt', { time: timeOf(open.check_in_at) }) : t('notClockedIn')}
        </p>
        <div className="mt-4">
          <Button
            variant="primary"
            disabled={pending}
            onClick={() => (open ? void doClockOut() : void doClockIn(false))}
          >
            {pending ? t('acquiring') : open ? t('clockOut') : t('clockIn')}
          </Button>
        </div>
      </div>

      {/* History */}
      <div className="flex flex-col gap-3">
        <h2 className="text-[15px] font-semibold text-text">{t('historyTitle')}</h2>
        {list.isLoading ? (
          <StateView kind="loading" title={t('loading')} />
        ) : list.isError ? (
          <StateView kind="error" title={t('errorGeneric')} onRetry={() => void list.refetch()} />
        ) : items.length === 0 ? (
          <StateView kind="empty" title={t('historyEmpty')} />
        ) : (
          items.map((a) => (
            <div key={a.id} className="rounded-xl border border-border bg-surface p-4">
              <div className="flex items-center justify-between">
                <span className="text-[14px] font-semibold text-text">{dateOf(a.check_in_at)}</span>
                <StatusBadge dot tone={attendanceStatusTone(a.status)}>
                  {t(`attendance.status.${a.status}`, { ns: 'common', defaultValue: a.status })}
                </StatusBadge>
              </div>
              <div className="mt-1 flex items-center gap-1.5 text-[12px] text-text-2">
                <Clock size={13} aria-hidden />
                {t('in')} {timeOf(a.check_in_at)}
                {a.check_out_at ? ` · ${t('out')} ${timeOf(a.check_out_at)}` : ''}
              </div>
              <button
                type="button"
                className="mt-3 text-[12px] font-semibold text-primary hover:underline"
                onClick={() =>
                  navigate({
                    to: '/me/correction',
                    search: {
                      attendanceId: a.id,
                      date: a.check_in_at ?? a.shift_start_at ?? '',
                    },
                  })
                }
              >
                {t('fileCorrection')}
              </button>
            </div>
          ))
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
