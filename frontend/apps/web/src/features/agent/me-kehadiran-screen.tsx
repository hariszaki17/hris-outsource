import { getCurrentCoords } from '@/lib/geolocation.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
/**
 * /me — Agent "Kehadiran" home. Merges the old dashboard + attendance + schedule into one
 * self-service surface (Plan §E · brainstorm.pen frame nwlSV).
 *
 * Layout (matches nwlSV):
 *   1. Title band: greeting + "Absen Sekarang" CTA (opens the Absen modal — frame GHxuN).
 *   2. Clock hero — a 1-second ticking WIB clock side-by-side with today's shift.
 *   3. KPI stat cards (today status / OT this month / leave balance / unread notifications).
 *   4. Two-column row: "Jadwal Minggu Ini" (left) + "Riwayat Kehadiran" (right) — side by side.
 *   5. Modals: Absen (clock-in/out, geofence) · Attendance detail · File-correction.
 *
 * The server resolves the agent from the JWT principal — no employee_id is sent on clock/list.
 * All times go through the Asia/Jakarta TZ layer (@swp/shared); the live clock ticks via a 1s
 * setInterval that is cleared on unmount.
 */
import { ApiError } from '@swp/api-client';
import {
  type GetScheduleByAgent200,
  type ScheduleEntry,
  ScheduleEntryStatus,
  useGetScheduleByAgent,
} from '@swp/api-client/e4';
import {
  type Attendance,
  type AttendancePage,
  AttendanceStatus,
  useClockIn,
  useClockOut,
  useListAttendance,
} from '@swp/api-client/e5';
import { type AgentDashboard, type Dashboard, useGetMyDashboard } from '@swp/api-client/e10';
import type { StatusTone } from '@swp/design-tokens';
import { LOCALE_ID, TZ, formatInstant } from '@swp/shared';
import {
  Button,
  EmptyState,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  StatCard,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import {
  Bell,
  CalendarClock,
  ChevronLeft,
  ChevronRight,
  Fingerprint,
  LogIn,
  LogOut,
  MapPin,
  Plane,
  ScanFace,
  Timer,
} from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  addDays,
  currentJakartaIso,
  getMondayOfWeek,
  parsePlainDate,
  weekDays,
} from '../e4-scheduling/roster-compliance.ts';
import { AgentPage } from './agent-page.tsx';
import { AgentCorrectionModal } from './me-correction-screen.tsx';

// ---------------------------------------------------------------------------
// Time helpers (Asia/Jakarta-safe)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Live WIB clock (ticks every 1s)
// ---------------------------------------------------------------------------

function useLiveJakartaTime(): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(id);
  }, []);
  return now;
}

function formatClockTime(now: Date): string {
  return new Intl.DateTimeFormat(LOCALE_ID, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
    timeZone: TZ,
  }).format(now);
}

function formatClockDate(now: Date): string {
  return new Intl.DateTimeFormat(LOCALE_ID, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: TZ,
  }).format(now);
}

// ---------------------------------------------------------------------------
// Schedule helpers (UTC-midnight plain dates — cf. me-schedule-screen.tsx)
// ---------------------------------------------------------------------------

function formatDayAbbr(iso: string): string {
  return new Intl.DateTimeFormat(LOCALE_ID, { weekday: 'short', timeZone: 'UTC' }).format(
    parsePlainDate(iso),
  );
}
function formatDayNum(iso: string): string {
  return new Intl.DateTimeFormat(LOCALE_ID, { day: 'numeric', month: 'short', timeZone: 'UTC' })
    .format(parsePlainDate(iso))
    .toUpperCase();
}
function formatWeekRange(monday: string): string {
  const sunday = addDays(monday, 6);
  const start = new Intl.DateTimeFormat(LOCALE_ID, {
    day: 'numeric',
    month: 'short',
    timeZone: 'UTC',
  }).format(parsePlainDate(monday));
  const end = new Intl.DateTimeFormat(LOCALE_ID, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(parsePlainDate(sunday));
  return `${start} – ${end}`;
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function AgentKehadiranScreen() {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const user = useCurrentUser();
  const employeeId = user?.employeeId ?? '';
  const greeting = t('dashGreeting', { name: user?.name ?? '' });

  const now = useLiveJakartaTime();

  // ---- Data ----
  const dashQ = useGetMyDashboard();
  const list = useListAttendance({ limit: 20 });
  const clockIn = useClockIn();
  const clockOut = useClockOut();

  const [busy, setBusy] = useState(false);
  const [absenOpen, setAbsenOpen] = useState(false);
  const [geofencePrompt, setGeofencePrompt] = useState<{ distance: string; radius: string } | null>(
    null,
  );
  const [detail, setDetail] = useState<Attendance | null>(null);
  const [correction, setCorrection] = useState<{ attendanceId: string; date: string } | null>(null);

  // ---- Schedule (this-week state, local) ----
  const [monday, setMonday] = useState<string>(() => getMondayOfWeek(currentJakartaIso()));
  const sunday = addDays(monday, 6);
  const days: string[] = weekDays(monday);
  const scheduleQ = useGetScheduleByAgent(
    employeeId,
    { start_date: monday, end_date: sunday, include_company: true },
    { query: { enabled: !!employeeId } },
  );
  const scheduleBody = scheduleQ.data?.data as GetScheduleByAgent200 | undefined;
  const entries: ScheduleEntry[] =
    (scheduleBody as { data?: ScheduleEntry[] } | undefined)?.data ?? [];
  const entryByDate = (iso: string) => entries.find((e) => e.work_date === iso);

  // ---- Dashboard unwrap (double envelope — cf. me-dashboard-screen.tsx) ----
  const outer = (dashQ.data as { data?: { data?: Dashboard } | Dashboard } | undefined)?.data;
  const dash = ((outer as { data?: Dashboard })?.data ?? outer) as AgentDashboard | undefined;

  // ---- Attendance list ----
  const body = list.data?.data as AttendancePage | undefined;
  const items: Attendance[] = body?.data ?? [];
  // "open" = actually clocked in (has check-in) and not yet clocked out. An ABSENT row has no
  // check_out_at but no check_in_at either — it must NOT count as open.
  const open = items.find((a) => a.check_in_at && !a.check_out_at);

  // ---- Clock handlers ----
  function handleClockError(e: unknown) {
    if (e instanceof ApiError) {
      if (e.code === 'OUT_OF_GEOFENCE') {
        setAbsenOpen(false);
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
      if (e.code === 'ON_LEAVE') {
        toast({ tone: 'error', title: t('onLeaveToday') });
        return;
      }
    }
    toast({ tone: 'error', title: t('clockError') });
  }

  async function doClockIn(force: boolean) {
    if (!hasShift) {
      toast({ tone: 'error', title: onLeaveToday ? t('onLeaveToday') : t('noShiftClockBlocked') });
      setAbsenOpen(false);
      return;
    }
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
        void dashQ.refetch();
        setAbsenOpen(false);
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
        void dashQ.refetch();
        setAbsenOpen(false);
        toast({ tone: 'success', title: t('successOut') });
      } catch (e) {
        handleClockError(e);
      }
    } finally {
      setBusy(false);
    }
  }

  const pending = busy || clockIn.isPending || clockOut.isPending;

  const shift = dash?.today_shift;
  const hasShift = Boolean(shift?.schedule_id);
  const shiftRange = hasShift ? `${shift?.start_time}–${shift?.end_time}` : t('dashOffToday');
  // No shift today → cannot clock in. Clock-out of an already-open record stays allowed.
  const canClockIn = hasShift;
  const absenDisabled = !open && !canClockIn;
  // On an approved-leave day the schedule entry is cancelled by leave; prefer the
  // leave-specific message over the generic no-shift message.
  const onLeaveToday =
    entryByDate(currentJakartaIso())?.status === ScheduleEntryStatus.CANCELLED_BY_LEAVE;
  const absenBlockedTitle = onLeaveToday ? t('onLeaveToday') : t('noShiftClockBlocked');

  // ---- Week nav (rendered inside the schedule panel header) ----
  const weekNav = (
    <div className="flex items-center gap-1">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setMonday((m) => addDays(m, -7))}
        aria-label={t('schedulePrevWeek')}
      >
        <ChevronLeft size={16} aria-hidden />
      </Button>
      <span className="min-w-[150px] text-center text-[13px] font-medium text-text">
        {formatWeekRange(monday)}
      </span>
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setMonday((m) => addDays(m, 7))}
        aria-label={t('scheduleNextWeek')}
      >
        <ChevronRight size={16} aria-hidden />
      </Button>
    </div>
  );

  return (
    <AgentPage
      title={greeting}
      subtitle={t('clockTitle')}
      actions={
        <Button
          variant="primary"
          onClick={() => setAbsenOpen(true)}
          disabled={absenDisabled}
          title={absenDisabled ? absenBlockedTitle : undefined}
        >
          <ScanFace className="size-4" aria-hidden />
          {t('absenNow')}
        </Button>
      }
    >
      {/* 1 · Clock hero — live clock | today's shift (side by side) */}
      <div className="flex items-center justify-between gap-6 rounded-xl border border-border bg-surface p-6">
        <div className="flex flex-col gap-1">
          <span className="text-[11px] font-semibold uppercase tracking-wide text-text-3">
            {t('nowLabel')}
          </span>
          <span className="font-mono text-[52px] font-bold leading-none text-text tabular-nums">
            {formatClockTime(now)}
          </span>
          <span className="text-[13px] text-text-3">{formatClockDate(now)}</span>
        </div>
        <div className="h-16 w-px shrink-0 bg-border" aria-hidden />
        <div className="flex flex-col gap-1.5">
          <span className="text-[11px] font-semibold uppercase tracking-wide text-text-3">
            {t('shiftTodayLabel')}
          </span>
          <span className="font-mono text-2xl font-bold text-text tabular-nums">{shiftRange}</span>
          <span className="text-[13px] text-text-3">
            {hasShift ? shift?.company_name : t('dashNoShift')}
          </span>
        </div>
      </div>

      {/* 2 · KPI cards */}
      {dash && (
        <div className="grid grid-cols-4 gap-4">
          <StatCard
            label={t('statusTodayLabel')}
            value={open ? timeOf(open.check_in_at) : t('notClockedInShort')}
            sub={open ? t('clockedInSub') : t('notClockedInSub')}
            icon={Fingerprint}
            tone={open ? 'brand' : 'neutral'}
          />
          <StatCard
            label={t('dashOtMonth')}
            value={t('dashHours', { count: dash.ot_this_month_hours })}
            icon={Timer}
            tone="warn"
          />
          <StatCard
            label={t('leaveTitle')}
            value={`${dash.leave_balance.annual_remaining_days}/${dash.leave_balance.annual_quota_days}`}
            sub={dash.leave_balance.period_label}
            icon={Plane}
            tone="info"
          />
          <StatCard
            label={t('notifTitle')}
            value={String(dash.recent_notifications_unread)}
            sub={t('dashUnread', { count: dash.recent_notifications_unread })}
            icon={Bell}
            tone={dash.recent_notifications_unread > 0 ? 'warn' : 'neutral'}
          />
        </div>
      )}

      {/* 3 · Two-column: Jadwal Minggu Ini | Riwayat Kehadiran (side by side) */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {/* Left: this week's schedule */}
        <section className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
          <header className="flex items-center justify-between border-border-soft border-b bg-surface-2 px-[18px] py-3.5">
            <h2 className="text-[15px] font-bold text-text">{t('weekScheduleTitle')}</h2>
            {weekNav}
          </header>
          {scheduleQ.isLoading ? (
            <div className="p-6">
              <StateView kind="loading" title={t('loading')} />
            </div>
          ) : scheduleQ.isError ? (
            <div className="p-6">
              <StateView
                kind="error"
                title={t('errorGeneric')}
                onRetry={() => void scheduleQ.refetch()}
              />
            </div>
          ) : entries.length === 0 ? (
            <div className="p-6">
              <StateView kind="empty" title={t('scheduleEmpty')} />
            </div>
          ) : (
            <div className="divide-y divide-border-soft">
              {days.map((iso) => (
                <ScheduleDayRow key={iso} iso={iso} entry={entryByDate(iso)} />
              ))}
            </div>
          )}
        </section>

        {/* Right: attendance history */}
        <section className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
          <header className="flex items-center justify-between border-border-soft border-b bg-surface-2 px-[18px] py-3.5">
            <h2 className="text-[15px] font-bold text-text">{t('historyTitle')}</h2>
          </header>
          {list.isLoading ? (
            <div className="p-6">
              <StateView kind="loading" title={t('loading')} />
            </div>
          ) : list.isError ? (
            <div className="p-6">
              <StateView
                kind="error"
                title={t('errorGeneric')}
                onRetry={() => void list.refetch()}
              />
            </div>
          ) : items.length === 0 ? (
            <div className="p-6">
              <EmptyState variant="fresh" title={t('historyEmpty')} />
            </div>
          ) : (
            <div className="divide-y divide-border-soft">
              {items.slice(0, 6).map((r) => (
                <button
                  key={r.id}
                  type="button"
                  onClick={() => setDetail(r)}
                  className="flex w-full items-center gap-3 px-[18px] py-3 text-left hover:bg-surface-2"
                >
                  <span className="w-16 shrink-0 text-[13px] font-semibold text-text">
                    {dateOf(r.check_in_at ?? r.shift_start_at)}
                  </span>
                  <span className="flex flex-1 items-center gap-3 text-[13px] tabular-nums text-text-3">
                    <span>↓ {timeOf(r.check_in_at)}</span>
                    <span>↑ {timeOf(r.check_out_at)}</span>
                  </span>
                  <StatusBadge dot tone={attendanceStatusTone(r.status)}>
                    {t(`attendance.status.${r.status}`, { ns: 'common', defaultValue: r.status })}
                  </StatusBadge>
                </button>
              ))}
            </div>
          )}
        </section>
      </div>

      {/* Absen modal (clock in/out) — frame GHxuN */}
      <AbsenModal
        open={absenOpen}
        onOpenChange={setAbsenOpen}
        now={now}
        clockTime={formatClockTime(now)}
        clockDate={formatClockDate(now)}
        shiftRange={shiftRange}
        shiftPlace={hasShift ? (shift?.company_name ?? '') : t('dashNoShift')}
        checkIn={open ? timeOf(open.check_in_at) : '—'}
        checkOut="—"
        isOpenRecord={Boolean(open)}
        pending={pending}
        onConfirm={() => (open ? void doClockOut() : void doClockIn(false))}
      />

      {/* Out-of-geofence override (clock-in only) */}
      <Modal open={geofencePrompt !== null} onOpenChange={(o) => !o && setGeofencePrompt(null)}>
        <ModalHeader icon={MapPin} tone="warn" title={t('outsideTitle')} />
        <ModalBody>
          <p className="text-sm text-text-2">
            {geofencePrompt
              ? t('outsideMsg', {
                  distance: geofencePrompt.distance,
                  radius: geofencePrompt.radius,
                })
              : ''}
          </p>
        </ModalBody>
        <ModalFooter>
          <Button variant="secondary" onClick={() => setGeofencePrompt(null)} disabled={pending}>
            {t('cancel')}
          </Button>
          <Button
            variant="primary"
            disabled={pending}
            onClick={() => {
              setGeofencePrompt(null);
              void doClockIn(true);
            }}
          >
            {t('clockAnyway')}
          </Button>
        </ModalFooter>
      </Modal>

      {/* Attendance detail modal */}
      <AttendanceDetailModal
        record={detail}
        onOpenChange={(o) => !o && setDetail(null)}
        onFileCorrection={(r) => {
          setDetail(null);
          setCorrection({ attendanceId: r.id, date: r.check_in_at ?? r.shift_start_at ?? '' });
        }}
      />

      {/* File-correction modal (opened from the detail modal) */}
      {correction && (
        <AgentCorrectionModal
          open
          onOpenChange={(o) => {
            if (!o) setCorrection(null);
          }}
          attendanceId={correction.attendanceId}
          date={correction.date}
          onSuccess={() => void list.refetch()}
        />
      )}
    </AgentPage>
  );
}

// ---------------------------------------------------------------------------
// AbsenModal — clock in/out (frame GHxuN: live clock + shift card + CTA)
// ---------------------------------------------------------------------------

interface AbsenModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  now: Date;
  clockTime: string;
  clockDate: string;
  shiftRange: string;
  shiftPlace: string;
  checkIn: string;
  checkOut: string;
  isOpenRecord: boolean;
  pending: boolean;
  onConfirm: () => void;
}

function AbsenModal({
  open,
  onOpenChange,
  clockTime,
  clockDate,
  shiftRange,
  shiftPlace,
  checkIn,
  checkOut,
  isOpenRecord,
  pending,
  onConfirm,
}: AbsenModalProps) {
  const { t } = useTranslation('agent');
  return (
    <Modal open={open} onOpenChange={onOpenChange} size="sm">
      <ModalHeader
        icon={ScanFace}
        tone="brand"
        title={isOpenRecord ? t('clockOut') : t('clockIn')}
      />
      <ModalBody>
        <div className="flex flex-col gap-4">
          {/* Live clock */}
          <div className="flex flex-col items-center gap-0.5">
            <span className="text-[11px] font-semibold uppercase tracking-wide text-text-3">
              {t('nowLabel')}
            </span>
            <span className="font-mono text-[40px] font-bold leading-none text-text tabular-nums">
              {clockTime}
            </span>
            <span className="text-[13px] text-text-3">{clockDate}</span>
          </div>

          {/* Shift card */}
          <div className="flex flex-col gap-3 rounded-lg border border-border-soft bg-surface-2 p-4">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2.5">
                <span className="flex size-9 items-center justify-center rounded-lg bg-brand-bg text-brand">
                  <CalendarClock size={18} aria-hidden />
                </span>
                <div className="flex flex-col">
                  <span className="text-[12px] font-semibold text-text-3">
                    {t('shiftTodayLabel')}
                  </span>
                  <span className="text-[14px] font-semibold text-text">{shiftPlace}</span>
                </div>
              </div>
              <span className="font-mono text-[15px] font-semibold text-text tabular-nums">
                {shiftRange}
              </span>
            </div>
            <div className="h-px w-full bg-border-soft" aria-hidden />
            <div className="flex gap-3">
              <div className="flex flex-1 flex-col gap-0.5">
                <span className="text-[12px] font-semibold text-text-3">{t('jamMasuk')}</span>
                <span className="font-mono text-[18px] font-bold text-text tabular-nums">
                  {checkIn}
                </span>
              </div>
              <div className="flex flex-1 flex-col gap-0.5">
                <span className="text-[12px] font-semibold text-text-3">{t('jamKeluar')}</span>
                <span className="font-mono text-[18px] font-bold text-text tabular-nums">
                  {checkOut}
                </span>
              </div>
            </div>
          </div>

          {/* Location hint */}
          <div className="flex items-center gap-2 rounded-lg border border-border bg-surface-2 px-3 py-2.5">
            <MapPin size={14} className="shrink-0 text-text-3" aria-hidden />
            <span className="text-[12px] font-medium text-text-2">{t('locationHint')}</span>
          </div>
        </div>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={pending}>
          {t('cancel')}
        </Button>
        <Button variant="primary" onClick={onConfirm} disabled={pending}>
          {isOpenRecord ? (
            <LogOut className="size-4" aria-hidden />
          ) : (
            <LogIn className="size-4" aria-hidden />
          )}
          {pending ? t('acquiring') : isOpenRecord ? t('clockOut') : t('clockIn')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// AttendanceDetailModal — read-only detail of one history row
// ---------------------------------------------------------------------------

function durationOf(inIso?: string | null, outIso?: string | null): string {
  if (!inIso || !outIso) return '—';
  const ms = new Date(outIso).getTime() - new Date(inIso).getTime();
  if (Number.isNaN(ms) || ms <= 0) return '—';
  const mins = Math.round(ms / 60000);
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  return `${h}j ${m}m`;
}

function AttendanceDetailModal({
  record,
  onOpenChange,
  onFileCorrection,
}: {
  record: Attendance | null;
  onOpenChange: (open: boolean) => void;
  onFileCorrection: (r: Attendance) => void;
}) {
  const { t } = useTranslation('agent');
  return (
    <Modal open={record !== null} onOpenChange={onOpenChange} size="sm">
      <ModalHeader icon={Fingerprint} tone="neutral" title={t('detailTitle')} />
      {record && (
        <>
          <ModalBody>
            <div className="flex flex-col gap-4">
              <div className="flex items-center justify-between gap-3">
                <span className="text-[15px] font-bold text-text">
                  {dateOf(record.check_in_at ?? record.shift_start_at)}
                </span>
                <StatusBadge dot tone={attendanceStatusTone(record.status)}>
                  {t(`attendance.status.${record.status}`, {
                    ns: 'common',
                    defaultValue: record.status,
                  })}
                </StatusBadge>
              </div>
              <dl className="grid grid-cols-2 gap-3">
                <DetailField label={t('jamMasuk')} value={timeOf(record.check_in_at)} />
                <DetailField label={t('jamKeluar')} value={timeOf(record.check_out_at)} />
                <DetailField
                  label={t('detailDuration')}
                  value={durationOf(record.check_in_at, record.check_out_at)}
                />
                <DetailField
                  label={t('detailShift')}
                  value={`${timeOf(record.shift_start_at)}–${timeOf(record.shift_end_at)}`}
                />
              </dl>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button variant="secondary" onClick={() => onOpenChange(false)}>
              {t('cancel')}
            </Button>
            <Button variant="primary" onClick={() => onFileCorrection(record)}>
              {t('fileCorrection')}
            </Button>
          </ModalFooter>
        </>
      )}
    </Modal>
  );
}

function DetailField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-[12px] font-semibold text-text-3">{label}</dt>
      <dd className="font-mono text-[15px] font-semibold text-text tabular-nums">{value}</dd>
    </div>
  );
}

// ---------------------------------------------------------------------------
// ScheduleDayRow — one divided row per day (ported from me-schedule-screen.tsx)
// ---------------------------------------------------------------------------

function ScheduleDayRow({ iso, entry }: { iso: string; entry?: ScheduleEntry }) {
  const { t } = useTranslation('agent');

  let statusLabel: string;
  let tone: 'neutral' | 'info' | 'ok' | 'warn' | 'bad' = 'neutral';
  let shiftTime: string | null = null;

  if (!entry) {
    statusLabel = t('scheduleNoShift');
    tone = 'neutral';
  } else if (entry.is_day_off) {
    statusLabel = t('scheduleDayOff');
    tone = 'neutral';
  } else if (entry.status === ScheduleEntryStatus.CANCELLED_BY_LEAVE) {
    statusLabel = t('scheduleCancelledLeave');
    tone = 'info';
  } else {
    shiftTime = `${entry.start_time ?? '—'}–${entry.end_time ?? '—'}`;
    statusLabel = shiftTime;
    tone = 'ok';
  }

  return (
    <div className="flex items-center gap-4 px-[18px] py-3">
      <div className="flex w-12 shrink-0 flex-col gap-0.5">
        <span className="text-[11px] uppercase tracking-wide text-text-3">
          {formatDayAbbr(iso)}
        </span>
        <span className="text-[13px] font-semibold leading-tight text-text">
          {formatDayNum(iso)}
        </span>
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        {entry?.company_name ? (
          <span className="truncate text-[13px] font-medium text-text">{entry.company_name}</span>
        ) : null}
        {shiftTime ? (
          <span className="text-[12px] tabular-nums text-text-3">{shiftTime}</span>
        ) : null}
      </div>
      <div className="shrink-0">
        <StatusBadge dot tone={tone}>
          {statusLabel}
        </StatusBadge>
      </div>
    </div>
  );
}
