/**
 * Attendance (Absen) screen — E5 F5.1
 * Design reference: absen-clockin.png / absen-clockout.png / absen-outside-geofence.png /
 *   absen-gps-unavailable.png / absen-no-schedule-flagged.png
 *
 * States (mutually exclusive, derived from GPS + schedule + open record):
 *   GPS_UNAVAILABLE     → disabled button "Aktifkan lokasi untuk clock-in" + bad banner + settings CTA
 *   OUTSIDE_GEOFENCE    → disabled button "Tidak dapat clock-in" + bad banner (no clock action)
 *   NO_SCHEDULE_FLAGGED → amber (danger) button "Lanjut clock-in" + warn banner (allowed + flagged)
 *   NORMAL_CLOCK_IN     → primary green button "Clock In" + ok pill + info note
 *   NORMAL_CLOCK_OUT    → danger button "Clock Out" + ok pill + ok note (elapsed time)
 */
import * as ExpoLinking from 'expo-linking';
import * as Location from 'expo-location';
import { useRouter } from 'expo-router';
import {
  Ban,
  Bell,
  Clock3,
  Info,
  LogIn,
  LogOut,
  type LucideIcon,
  MapPin,
  MapPinCheck,
  MapPinOff,
  Settings,
  TriangleAlert,
} from 'lucide-react-native';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Alert, Pressable, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { ApiError } from '@swp/api-client';
import { type PlacementDetailResponse, useGetPlacement } from '@swp/api-client/e3';
import {
  type GetScheduleByAgent200,
  type ScheduleEntry,
  useGetScheduleByAgent,
} from '@swp/api-client/e4';
import {
  type Attendance,
  type AttendancePage,
  useClockIn,
  useClockOut,
  useListAttendance,
} from '@swp/api-client/e5';
import { color } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared/datetime';
import { useQueryClient } from '@tanstack/react-query';

import { type Coords, getCurrentCoords } from '../../src/lib/location';
import { useSession } from '../../src/providers/session';
import { Text } from '../../src/ui/Text';

// ── Helpers ──────────────────────────────────────────────────────────────────

function timeOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { timeStyle: 'short' }) : '—:—';
}

function dateLong(date: Date): string {
  return date.toLocaleDateString('id-ID', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'Asia/Jakarta',
  });
}

function clockStr(date: Date): string {
  return date.toLocaleTimeString('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: 'Asia/Jakarta',
  });
}

function elapsedStr(checkInIso: string): string {
  const ms = Date.now() - new Date(checkInIso).getTime();
  const totalMin = Math.floor(ms / 60000);
  const h = Math.floor(totalMin / 60);
  const m = totalMin % 60;
  return `${h} jam ${m} menit`;
}

// Great-circle distance in meters. Mirrors the server-side haversine in
// backend clock_service.go so the pre-clock pill matches the clock-in check.
const EARTH_RADIUS_M = 6_371_000;
function haversineM(a: Coords, b: Coords): number {
  const toRad = (d: number) => (d * Math.PI) / 180;
  const dLat = toRad(b.lat - a.lat);
  const dLng = toRad(b.lng - a.lng);
  const lat1 = toRad(a.lat);
  const lat2 = toRad(b.lat);
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLng / 2) ** 2;
  return Math.round(EARTH_RADIUS_M * 2 * Math.atan2(Math.sqrt(h), Math.sqrt(1 - h)));
}

// Local (device = WIB) YYYY-MM-DD — avoids the UTC shift of toISOString().
function ymd(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${m}-${day}`;
}

// YYYY-MM-DD of an instant in Asia/Jakarta (WIB), regardless of device TZ.
// en-CA locale renders ISO-shaped YYYY-MM-DD. Used to scope the Masuk/Keluar
// tiles to TODAY's shift — slicing the raw UTC timestamp would mis-date the
// early-morning WIB shifts that fall on the previous UTC day.
function jakartaYmd(d: Date): string {
  return d.toLocaleDateString('en-CA', { timeZone: 'Asia/Jakarta' });
}

// Mon–Sun dates of the week containing `base`.
function weekOf(base: Date): Date[] {
  const mondayOffset = (base.getDay() + 6) % 7;
  const monday = new Date(base);
  monday.setDate(base.getDate() - mondayOffset);
  return Array.from({ length: 7 }, (_, i) => {
    const d = new Date(monday);
    d.setDate(monday.getDate() + i);
    return d;
  });
}

// Weekday abbreviation only (Sen, Sel, …) — id-ID appends a period, so strip it.
function dowShort(d: Date): string {
  return d
    .toLocaleDateString('id-ID', { weekday: 'short', timeZone: 'Asia/Jakarta' })
    .replace('.', '');
}

// "8 – 14 Jun" within a month, "29 Mei – 4 Jun" across a month boundary.
function weekRangeLabel(week: Date[]): string {
  const first = week[0];
  const last = week[6];
  const mShort = (d: Date) =>
    d.toLocaleDateString('id-ID', { month: 'short', timeZone: 'Asia/Jakarta' });
  return first.getMonth() === last.getMonth()
    ? `${first.getDate()} – ${last.getDate()} ${mShort(last)}`
    : `${first.getDate()} ${mShort(first)} – ${last.getDate()} ${mShort(last)}`;
}

// ── Screen ───────────────────────────────────────────────────────────────────

type GpsState = 'checking' | 'unavailable' | 'ok';

export default function AttendanceScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const qc = useQueryClient();

  // Live clock (tick every second)
  const [now, setNow] = useState(new Date());
  const clockRef = useRef<ReturnType<typeof setInterval> | null>(null);
  useEffect(() => {
    clockRef.current = setInterval(() => setNow(new Date()), 1000);
    return () => {
      if (clockRef.current) clearInterval(clockRef.current);
    };
  }, []);

  // GPS permission state (checked once on mount; re-checked after settings flow)
  const [gpsState, setGpsState] = useState<GpsState>('checking');
  const [distanceM, setDistanceM] = useState<number | null>(null);
  // isOutside: set true when server returns OUT_OF_GEOFENCE on a clock-in attempt
  const [isOutside, setIsOutside] = useState(false);

  useEffect(() => {
    void (async () => {
      const { status } = await Location.getForegroundPermissionsAsync();
      setGpsState(status === 'granted' ? 'ok' : 'unavailable');
    })();
  }, []);

  // Attendance list — today's context
  const list = useListAttendance({ limit: 20 });
  const body = list.data?.data as AttendancePage | undefined;
  const items: Attendance[] = body?.data ?? [];
  // "open" = agent has clocked in and not yet clocked out AND checkout window is still open
  const open = items.find((a) => a.check_in_at && !a.check_out_at && a.can_check_out);
  // Most recent closed record for the Keluar tile
  const lastClosed = items.find((a) => a.check_in_at && a.check_out_at);

  // Agent's placement → site geofence (E3). The site centroid + radius are not on
  // the device, so without this the app can't compute distance pre-clock. The list
  // endpoint is HR/SL-only, but GET /placements/{id} is agent-self-allowed — so we
  // resolve the placement id from the attendance records the agent already owns.
  const placementId =
    open?.placement_id ?? lastClosed?.placement_id ?? items[0]?.placement_id ?? '';
  const placementQ = useGetPlacement(placementId, { query: { enabled: !!placementId } });
  const siteGeofence =
    (placementQ.data?.data as PlacementDetailResponse | undefined)?.placement.site_geofence ?? null;

  // Live distance to the work point: haversine(device GPS, site centroid). Computed
  // once GPS is granted and the geofence is loaded — matches the server clock-in check.
  useEffect(() => {
    if (gpsState !== 'ok' || !siteGeofence) return;
    void (async () => {
      const coords = await getCurrentCoords();
      if (!coords) {
        setGpsState('unavailable');
        return;
      }
      const d = haversineM(coords, { lat: siteGeofence.geo_lat, lng: siteGeofence.geo_lng });
      setDistanceM(d);
      setIsOutside(d > siteGeofence.radius_m);
    })();
  }, [gpsState, siteGeofence]);

  // Weekly schedule — surfaced on Beranda (the Jadwal tab was replaced by Kehadiran).
  const { user } = useSession();
  const employeeId = user?.employee_id ?? '';
  const week = weekOf(now);
  const scheduleQ = useGetScheduleByAgent(
    employeeId,
    { start_date: ymd(week[0]), end_date: ymd(week[6]), include_company: true },
    { query: { enabled: !!employeeId } },
  );
  const scheduleBody = scheduleQ.data?.data as GetScheduleByAgent200 | undefined;
  const weekEntries: ScheduleEntry[] = scheduleBody?.data ?? [];
  const todayYmd = ymd(now);
  // Detect "no schedule today" from the attendance list — if no item has shift_start_at
  // within today's date, the agent likely has no scheduled shift today.
  const todayStr = now.toISOString().slice(0, 10);
  const hasScheduleToday = items.some((a) => {
    if (!a.shift_start_at) return false;
    return a.shift_start_at.slice(0, 10) === todayStr;
  });

  const clockInMut = useClockIn();
  const clockOutMut = useClockOut();
  const [busy, setBusy] = useState(false);
  const pending = busy || clockInMut.isPending || clockOutMut.isPending;

  async function refresh() {
    await qc.invalidateQueries({ queryKey: ['/attendance'] });
  }

  function handleApiError(e: unknown, onForce?: () => void) {
    if (e instanceof ApiError) {
      if (e.code === 'OUT_OF_GEOFENCE' && onForce) {
        const dm = (e.fields?.distance_m as number | undefined) ?? null;
        setIsOutside(true);
        if (dm !== null) setDistanceM(dm);
        // The .pen shows disabled button + bad banner (no "clock anyway" flow in the outside-geofence state).
        // We do NOT prompt the user to force-clock here — that state is the disabled UI.
        return;
      }
      if (e.code === 'GPS_UNAVAILABLE') {
        setGpsState('unavailable');
        return;
      }
      if (e.code === 'ALREADY_CLOCKED_IN') {
        void refresh();
        return;
      }
      if (e.code === 'NOT_CLOCKED_IN') {
        Alert.alert(t('m:clock.title'), t('m:clock.notIn'));
        return;
      }
    }
    Alert.alert(t('m:clock.title'), t('m:clock.error'));
  }

  async function doClockIn(noScheduleForce: boolean) {
    if (gpsState === 'unavailable') return;
    setBusy(true);
    try {
      const coords = await getCurrentCoords();
      if (!coords) {
        setGpsState('unavailable');
        return;
      }
      setIsOutside(false); // optimistic reset
      try {
        await clockInMut.mutateAsync({
          data: {
            lat: coords.lat,
            lng: coords.lng,
            gps_available: true,
            wfo: true,
            force_outside_geofence: noScheduleForce,
          },
        });
        setDistanceM(null);
        await refresh();
        Alert.alert(t('m:clock.title'), t('m:clock.successIn'));
      } catch (e) {
        handleApiError(e);
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
        setGpsState('unavailable');
        return;
      }
      try {
        await clockOutMut.mutateAsync({
          data: { lat: coords.lat, lng: coords.lng, gps_available: true },
        });
        await refresh();
        Alert.alert(t('m:clock.title'), t('m:clock.successOut'));
      } catch (e) {
        handleApiError(e);
      }
    } finally {
      setBusy(false);
    }
  }

  // ── Derived shift header values ─────────────────────────────────────────────

  const siteName: string =
    open?.site_name ??
    open?.company_name ??
    lastClosed?.site_name ??
    lastClosed?.company_name ??
    'Plaza Senayan'; // placeholder until first API load

  const noScheduleState = !hasScheduleToday && !open && !list.isLoading;

  // ShiftToday card: dot color + label + site + card tint (.pen ShiftToday per state).
  let shiftDotColor: string;
  let shiftLabelText: string;
  let shiftSite: string;
  let shiftCardBg: string = color.surface;
  let shiftCardBorder: string = color.border;
  let shiftLabelColor: string = color.text;

  if (noScheduleState) {
    // No schedule today (warn-tinted card)
    shiftDotColor = color.warn.text; // #B54708
    shiftLabelText = t('m:absen.noScheduleLabel');
    shiftSite = `Penempatan: ${siteName}`;
    shiftCardBg = color.warn.bg;
    shiftCardBorder = color.warn.border;
    shiftLabelColor = color.warn.text;
  } else if (open) {
    // Clocked in — teal dot, "Sedang bekerja"
    const start = open.shift_start_at ? timeOf(open.shift_start_at) : null;
    const end = open.shift_end_at ? timeOf(open.shift_end_at) : null;
    shiftDotColor = color.ok.text; // #0F766E
    shiftLabelText =
      start && end
        ? `Sedang bekerja · ${t('m:absen.shiftLabel')} ${start} – ${end}`
        : 'Sedang bekerja';
    shiftSite = siteName;
  } else {
    // Pre-clock — gold dot
    const sample = items.find((a) => a.shift_start_at);
    const start = sample ? timeOf(sample.shift_start_at) : '07:00';
    const end = sample ? timeOf(sample.shift_end_at) : '15:00';
    shiftDotColor = color.accent.gold; // #F5A800
    shiftLabelText = `${t('m:absen.shiftLabel')} · ${start} – ${end}`;
    shiftSite = siteName;
  }

  // ── Location pill ───────────────────────────────────────────────────────────

  let locPillLabel: string;
  let locOk: boolean; // teal in-radius vs red
  let locPillIcon: LucideIcon;

  if (gpsState === 'unavailable') {
    locPillLabel = t('m:absen.gpsUnavailable');
    locOk = false;
    locPillIcon = MapPinOff;
  } else if (isOutside && distanceM !== null) {
    locPillLabel = t('m:absen.outsideRadius', { distance: distanceM });
    locOk = false;
    locPillIcon = MapPinOff;
  } else if (distanceM !== null) {
    locPillLabel = t('m:absen.inRadius', { distance: distanceM });
    locOk = true;
    locPillIcon = MapPinCheck;
  } else {
    locOk = gpsState === 'ok';
    locPillLabel = locOk ? t('m:absen.inRadiusNoDistance') : t('m:absen.gpsUnavailable');
    locPillIcon = locOk ? MapPinCheck : MapPinOff;
  }
  const locPillBg = locOk ? 'bg-ok-bg' : 'bg-bad-bg';
  const locPillBorder = locOk ? 'border-ok-border' : 'border-bad-border';
  const locTextClass = locOk ? 'text-ok-text' : 'text-bad-text';
  const locIconColor = locOk ? color.ok.text : color.bad.text;

  // ── Action button + banner ──────────────────────────────────────────────────
  // Per .pen ClockInBtn: r12, label 16/700 white, leading icon 22. Disabled = #D4D4D8 @0.7.

  let actionLabel: string;
  let actionIcon: LucideIcon;
  let actionBg: string; // solid fill (hex)
  let actionDisabled: boolean;
  let actionOnPress: () => void;

  let bannerBg: string | null = null;
  let bannerBorder: string | null = null;
  let bannerText: string | null = null;
  let bannerTextClass = 'text-info-text';
  let bannerIcon: LucideIcon = Info;
  let bannerIconColor: string = color.info.text;

  if (gpsState === 'unavailable') {
    // GPS unavailable — disabled
    actionLabel = t('m:absen.activateLocation');
    actionIcon = Ban;
    actionBg = color.controlOff; // #D1D5DB-ish gray
    actionDisabled = true;
    actionOnPress = () => {};
    bannerBg = 'bg-bad-bg';
    bannerBorder = 'border-bad-border';
    bannerText = t('m:absen.gpsWarning');
    bannerTextClass = 'text-bad-text';
    bannerIcon = MapPin;
    bannerIconColor = color.bad.text;
  } else if (isOutside && !open) {
    // Outside geofence — disabled
    actionLabel = t('m:absen.cannotClockIn');
    actionIcon = Ban;
    actionBg = color.controlOff;
    actionDisabled = true;
    actionOnPress = () => {};
    bannerBg = 'bg-bad-bg';
    bannerBorder = 'border-bad-border';
    bannerText = t('m:absen.outsideWarning');
    bannerTextClass = 'text-bad-text';
    bannerIcon = TriangleAlert;
    bannerIconColor = color.bad.text;
  } else if (noScheduleState) {
    // No schedule today — allowed but flagged (orange button)
    actionLabel = t('m:absen.proceedClockIn');
    actionIcon = LogIn;
    actionBg = color.warn.text; // #B54708
    actionDisabled = false;
    actionOnPress = () => void doClockIn(true);
    bannerBg = 'bg-warn-bg';
    bannerBorder = 'border-warn-border';
    bannerText = t('m:absen.noScheduleWarning');
    bannerTextClass = 'text-warn-text';
    bannerIcon = TriangleAlert;
    bannerIconColor = color.warn.text;
  } else if (open) {
    // Clocked in — show Clock Out (red)
    actionLabel = t('m:absen.clockOut');
    actionIcon = LogOut;
    actionBg = color.bad.text; // #BF4A40
    actionDisabled = false;
    actionOnPress = () => void doClockOut();
    bannerBg = 'bg-ok-bg';
    bannerBorder = 'border-ok-border';
    bannerText = open.check_in_at
      ? t('m:absen.clockedInNote', {
          duration: elapsedStr(open.check_in_at),
          time: timeOf(open.check_in_at),
        })
      : null;
    bannerTextClass = 'text-ok-text';
    bannerIcon = Clock3;
    bannerIconColor = color.primaryStrong;
  } else {
    // Normal clock-in (green)
    actionLabel = t('m:absen.clockIn');
    actionIcon = LogIn;
    actionBg = color.primary; // #188E4D
    actionDisabled = false;
    actionOnPress = () => void doClockIn(false);
    bannerBg = 'bg-info-bg';
    bannerBorder = 'border-info-border';
    bannerText = t('m:absen.outsideInfoNote');
    bannerTextClass = 'text-info-text';
    bannerIcon = Info;
    bannerIconColor = color.info.text;
  }

  // ── Masuk / Keluar tiles ────────────────────────────────────────────────────
  // Both tiles MUST read the SAME record, else they show a mismatched pair (e.g.
  // empty Masuk + a stale Keluar from an older closed shift). Scope to TODAY's
  // shift only: prefer the open record (mid-shift), else today's latest shift by
  // shift_start_at. A closed shift from a previous day must NOT leak in — without
  // this scope, yesterday's in/out renders as if it were today's.
  const todayJkt = jakartaYmd(now);
  const todayLatest = items
    .filter((a) => a.shift_start_at && jakartaYmd(new Date(a.shift_start_at)) === todayJkt)
    .sort((a, b) => (b.shift_start_at ?? '').localeCompare(a.shift_start_at ?? ''))[0];
  const summaryRec = open ?? todayLatest;
  const masukTime = summaryRec?.check_in_at ? timeOf(summaryRec.check_in_at) : '—:—';
  const keluarTime = summaryRec?.check_out_at ? timeOf(summaryRec.check_out_at) : '—:—';

  // ── Render ──────────────────────────────────────────────────────────────────
  // JSX requires component identifiers to be capitalized.
  const LocPillIcon = locPillIcon;
  const ActionIcon = actionIcon;
  const BannerIcon = bannerIcon;

  return (
    <View className="flex-1 bg-app-bg">
      {/* ── AppBar: "Absen" title + bell (design header) ──────────────────── */}
      <View
        className="flex-row items-center justify-between px-4 pb-2"
        style={{ paddingTop: insets.top + 8 }}
      >
        <Text variant="screenTitle">{t('m:absen.title')}</Text>
        <Pressable onPress={() => router.push('/notifications')} hitSlop={8}>
          <Bell size={22} color={color.text2} />
        </Pressable>
      </View>

      <ScrollView className="flex-1" contentContainerStyle={{ padding: 16, gap: 14 }}>
        {/* ── ShiftToday card (.pen: r10, padding 10/14, dot 8 + label + site) ── */}
        <View
          className="flex-row items-center gap-2 rounded-[10px] border px-3.5 py-2.5"
          style={{ backgroundColor: shiftCardBg, borderColor: shiftCardBorder }}
        >
          <View className="h-2 w-2 rounded-pill" style={{ backgroundColor: shiftDotColor }} />
          <Text
            variant="label"
            weight="semibold"
            className="flex-1"
            style={{ color: shiftLabelColor }}
          >
            {shiftLabelText}
          </Text>
          <Text variant="caption">{shiftSite}</Text>
        </View>

        {/* ── Clock card (.pen: r16, padding 24/20, mono time + date + GPS pill) ── */}
        <View className="items-center gap-1.5 rounded-[16px] border border-border bg-surface px-5 py-6">
          <Text variant="monoHero" style={{ letterSpacing: -1 }}>
            {clockStr(now)}
          </Text>
          <Text variant="secondary" className="text-text-3">
            {dateLong(now)}
          </Text>
          <View
            className={`mt-0.5 flex-row items-center gap-1.5 rounded-pill border px-3.5 py-1.5 ${locPillBg} ${locPillBorder}`}
          >
            <LocPillIcon size={14} color={locIconColor} />
            <Text variant="caption" weight="semibold" className={locTextClass}>
              {locPillLabel}
            </Text>
          </View>
        </View>

        {/* ── Action button (.pen ClockInBtn: r12, icon 22 + label 16/700 white) ── */}
        {list.isLoading || gpsState === 'checking' ? (
          <View className="items-center py-4">
            <ActivityIndicator />
          </View>
        ) : (
          <Pressable
            disabled={actionDisabled || pending}
            onPress={actionOnPress}
            className="flex-row items-center justify-center gap-3 rounded-card px-5 py-4"
            style={{ backgroundColor: actionBg, opacity: actionDisabled ? 0.7 : 1 }}
          >
            {pending ? (
              <ActivityIndicator color={color.surface} />
            ) : (
              <>
                <ActionIcon size={22} color={color.surface} />
                <Text variant="buttonLg" style={{ color: color.surface }}>
                  {actionLabel}
                </Text>
              </>
            )}
          </Pressable>
        )}

        {/* ── Banner (.pen Note: r8, icon 14 + text 12/1.4) ──────────────────── */}
        {bannerText ? (
          <View
            className={`flex-row items-start gap-2 rounded-input border px-3 py-2.5 ${bannerBg ?? ''} ${bannerBorder ?? ''}`}
          >
            <View style={{ paddingTop: 1 }}>
              <BannerIcon size={14} color={bannerIconColor} />
            </View>
            <Text variant="caption" className={`flex-1 ${bannerTextClass}`}>
              {bannerText}
            </Text>
          </View>
        ) : null}

        {/* ── Masuk / Keluar tiles (.pen: icon 14 + label, mono 20/700 value) ── */}
        <View className="flex-row gap-3">
          {(
            [
              { label: t('m:absen.masuk'), value: masukTime, Icon: LogIn },
              { label: t('m:absen.keluar'), value: keluarTime, Icon: LogOut },
            ] as const
          ).map(({ label, value, Icon }) => (
            <View
              key={label}
              className="flex-1 items-center gap-1.5 rounded-card border border-border bg-surface px-3.5 py-4"
            >
              <View className="flex-row items-center gap-1.5">
                <Icon size={14} color={color.text3} />
                <Text variant="caption">{label}</Text>
              </View>
              <Text variant="monoLg" style={{ color: value === '—:—' ? color.text2 : color.text }}>
                {value}
              </Text>
            </View>
          ))}
        </View>

        {/* ── GPS unavailable → open settings (.pen: red outline, settings icon) ── */}
        {gpsState === 'unavailable' ? (
          <Pressable
            onPress={() => void ExpoLinking.openSettings()}
            className="flex-row items-center justify-center gap-2.5 rounded-card border bg-surface px-5 py-3.5"
            style={{ borderColor: color.bad.text, borderWidth: 1.5 }}
          >
            <Settings size={18} color={color.bad.text} />
            <Text variant="subtitle" style={{ color: color.bad.text }}>
              {t('m:absen.openSettings')}
            </Text>
          </Pressable>
        ) : null}

        {/* ── Jadwal Minggu Ini (.pen WeekSchedule: card · 7-day strip · today highlighted ── */}
        {/*    · scheduled-dot legend). Tap → full Jadwal. ───────────────────────────── */}
        <View
          className="rounded-card border border-border bg-surface"
          style={{ padding: 14, gap: 12 }}
        >
          <View className="flex-row items-center justify-between">
            <Text variant="strong" weight="bold">
              {t('m:absen.weekSchedule')}
            </Text>
            <Text variant="caption" weight="medium">
              {weekRangeLabel(week)}
            </Text>
          </View>
          {/* Fractional spacing keys (1.5/2.5/0.5) and arbitrary radius don't resolve in this
              NativeWind theme (custom spacing scale is integer-keyed), so the dots/cell-padding/
              rounding are set inline in px — same pattern as the clock tiles above. */}
          <Pressable
            onPress={() => router.push('/schedule')}
            className="flex-row"
            style={{ gap: 4 }}
          >
            {week.map((d) => {
              const entry = weekEntries.find((e) => e.work_date === ymd(d));
              const scheduled =
                !!entry && !entry.is_day_off && entry.status !== 'CANCELLED_BY_LEAVE';
              const isToday = ymd(d) === todayYmd;
              return (
                <View
                  key={ymd(d)}
                  className={isToday ? 'border border-primary bg-primary-soft' : ''}
                  style={{
                    flex: 1,
                    alignItems: 'center',
                    gap: 6,
                    borderRadius: 12,
                    paddingVertical: 8,
                    paddingHorizontal: 2,
                  }}
                >
                  <Text variant="badge" className={isToday ? 'text-primary-strong' : 'text-text-3'}>
                    {dowShort(d)}
                  </Text>
                  <Text
                    variant="subtitle"
                    className={isToday ? 'text-primary-strong' : 'text-text'}
                  >
                    {d.getDate()}
                  </Text>
                  <View
                    style={{
                      width: 7,
                      height: 7,
                      borderRadius: 999,
                      backgroundColor: scheduled ? color.ok.text : color.text3,
                      opacity: scheduled ? 1 : 0.3,
                    }}
                  />
                </View>
              );
            })}
          </Pressable>
          <View className="flex-row items-center" style={{ gap: 16 }}>
            <View className="flex-row items-center" style={{ gap: 6 }}>
              <View
                style={{ width: 7, height: 7, borderRadius: 999, backgroundColor: color.ok.text }}
              />
              <Text variant="badge" weight="medium" className="text-text-3">
                {t('m:absen.scheduledLegend')}
              </Text>
            </View>
            <View className="flex-row items-center" style={{ gap: 6 }}>
              <View
                style={{
                  width: 7,
                  height: 7,
                  borderRadius: 999,
                  backgroundColor: color.text3,
                  opacity: 0.3,
                }}
              />
              <Text variant="badge" weight="medium" className="text-text-3">
                {t('m:schedule.dayOff')}
              </Text>
            </View>
          </View>
        </View>
      </ScrollView>
    </View>
  );
}
