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
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ActivityIndicator,
  Alert,
  Pressable,
  ScrollView,
  View,
} from 'react-native';

import { ApiError } from '@swp/api-client';
import {
  type Attendance,
  type AttendancePage,
  useClockIn,
  useClockOut,
  useListAttendance,
} from '@swp/api-client/e5';
import { formatInstant } from '@swp/shared/datetime';
import { useQueryClient } from '@tanstack/react-query';

import { getCurrentCoords } from '../../src/lib/location';
import { Button } from '../../src/ui/Button';
import { Card } from '../../src/ui/Card';
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

// ── Screen ───────────────────────────────────────────────────────────────────

type GpsState = 'checking' | 'unavailable' | 'ok';

export default function AttendanceScreen() {
  const { t } = useTranslation();
  const router = useRouter();
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

  const shiftSite: string =
    open?.site_name ??
    open?.company_name ??
    lastClosed?.site_name ??
    lastClosed?.company_name ??
    'Plaza Senayan'; // placeholder until first API load

  let shiftDotClass: string;
  let shiftLabelText: string;

  if (!hasScheduleToday && !open && !list.isLoading) {
    // No-schedule-flagged state (amber)
    shiftDotClass = 'bg-warn-text';
    shiftLabelText = t('m:absen.noScheduleLabel');
  } else if (open) {
    // Clock-out state (green dot)
    const start = open.shift_start_at ? timeOf(open.shift_start_at) : null;
    const end = open.shift_end_at ? timeOf(open.shift_end_at) : null;
    shiftDotClass = 'bg-ok-text';
    shiftLabelText = start && end
      ? `Sedang bekerja · Shift Pagi ${start} – ${end}`
      : 'Sedang bekerja';
  } else {
    // Normal clock-in state (amber/warm dot = pre-clock)
    const sample = items.find((a) => a.shift_start_at);
    const start = sample ? timeOf(sample.shift_start_at) : '07:00';
    const end = sample ? timeOf(sample.shift_end_at) : '15:00';
    shiftDotClass = 'bg-warn-text';
    shiftLabelText = `Shift Pagi · ${start} – ${end}`;
  }

  // ── Location pill ───────────────────────────────────────────────────────────

  let locPillLabel: string;
  let locPillBg: string;
  let locPillBorder: string;
  let locTextClass: string;

  if (gpsState === 'unavailable') {
    locPillLabel = t('m:absen.gpsUnavailable');
    locPillBg = 'bg-bad-bg';
    locPillBorder = 'border-bad-border';
    locTextClass = 'text-bad-text';
  } else if (isOutside && distanceM !== null) {
    locPillLabel = t('m:absen.outsideRadius', { distance: distanceM });
    locPillBg = 'bg-bad-bg';
    locPillBorder = 'border-bad-border';
    locTextClass = 'text-bad-text';
  } else if (distanceM !== null) {
    locPillLabel = t('m:absen.inRadius', { distance: distanceM });
    locPillBg = 'bg-ok-bg';
    locPillBorder = 'border-ok-border';
    locTextClass = 'text-ok-text';
  } else {
    locPillLabel = gpsState === 'ok'
      ? t('m:absen.inRadius', { distance: '…' })
      : t('m:absen.gpsUnavailable');
    locPillBg = gpsState === 'ok' ? 'bg-ok-bg' : 'bg-bad-bg';
    locPillBorder = gpsState === 'ok' ? 'border-ok-border' : 'border-bad-border';
    locTextClass = gpsState === 'ok' ? 'text-ok-text' : 'text-bad-text';
  }

  // ── Action button + banner ──────────────────────────────────────────────────

  type BtnVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

  let actionLabel: string;
  let actionVariant: BtnVariant;
  let actionDisabled: boolean;
  let actionOnPress: () => void;

  let bannerBg: string | null = null;
  let bannerBorder: string | null = null;
  let bannerText: string | null = null;
  let bannerTextClass = 'text-info-text';

  if (gpsState === 'unavailable') {
    // GPS unavailable
    actionLabel = t('m:absen.activateLocation');
    actionVariant = 'secondary';
    actionDisabled = true;
    actionOnPress = () => {};
    bannerBg = 'bg-bad-bg';
    bannerBorder = 'border-bad-border';
    bannerText = t('m:absen.gpsWarning');
    bannerTextClass = 'text-bad-text';
  } else if (isOutside && !open) {
    // Outside geofence — disabled
    actionLabel = t('m:absen.cannotClockIn');
    actionVariant = 'secondary';
    actionDisabled = true;
    actionOnPress = () => {};
    bannerBg = 'bg-bad-bg';
    bannerBorder = 'border-bad-border';
    bannerText = t('m:absen.outsideWarning');
    bannerTextClass = 'text-bad-text';
  } else if (!hasScheduleToday && !open && !list.isLoading) {
    // No schedule today — allowed but flagged
    actionLabel = t('m:absen.proceedClockIn');
    actionVariant = 'danger';
    actionDisabled = false;
    actionOnPress = () => void doClockIn(true);
    bannerBg = 'bg-warn-bg';
    bannerBorder = 'border-warn-border';
    bannerText = t('m:absen.noScheduleWarning');
    bannerTextClass = 'text-warn-text';
  } else if (open) {
    // Clocked in — show Clock Out
    actionLabel = t('m:absen.clockOut');
    actionVariant = 'danger';
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
  } else {
    // Normal clock-in
    actionLabel = t('m:absen.clockIn');
    actionVariant = 'primary';
    actionDisabled = false;
    actionOnPress = () => void doClockIn(false);
    // Info note about outside-radius still being recorded
    bannerBg = 'bg-info-bg';
    bannerBorder = 'border-info-border';
    bannerText = t('m:absen.outsideInfoNote');
    bannerTextClass = 'text-info-text';
  }

  // ── Masuk / Keluar tiles ────────────────────────────────────────────────────

  const masukTime = open?.check_in_at ? timeOf(open.check_in_at) : '—:—';
  const keluarTime = open?.check_out_at
    ? timeOf(open.check_out_at)
    : lastClosed?.check_out_at
      ? timeOf(lastClosed.check_out_at)
      : '—:—';

  // ── Render ──────────────────────────────────────────────────────────────────

  return (
    <ScrollView
      className="flex-1 bg-app-bg"
      contentContainerStyle={{ padding: 16, gap: 12 }}
    >
      {/* ── Shift header card ─────────────────────────────────────────────── */}
      <Card>
        <View className="flex-row items-center justify-between">
          <View className="flex-row items-center gap-2 flex-1 flex-wrap">
            <View className={`h-2.5 w-2.5 rounded-pill ${shiftDotClass}`} />
            <Text
              variant="body"
              className="font-semibold text-text"
              style={{ flexShrink: 1 }}
            >
              {shiftLabelText}
            </Text>
          </View>
          <Text variant="caption" className="text-text-3 ml-2 shrink-0">
            {shiftSite}
          </Text>
        </View>
      </Card>

      {/* ── Live clock card ────────────────────────────────────────────────── */}
      <Card>
        {/* Large digital clock */}
        <Text
          className="text-center text-text font-bold"
          style={{ fontSize: 52, fontWeight: '700', letterSpacing: -1 }}
        >
          {clockStr(now)}
        </Text>
        {/* Date line */}
        <Text variant="caption" className="mt-1 text-center text-text-2">
          {dateLong(now)}
        </Text>
        {/* Location status pill */}
        <View className="mt-3 self-center">
          <View
            className={`flex-row items-center gap-1.5 rounded-pill border px-3 py-1 ${locPillBg} ${locPillBorder}`}
          >
            <Text className={`text-xs font-semibold ${locTextClass}`}>
              {locPillLabel}
            </Text>
          </View>
        </View>
      </Card>

      {/* ── Action button ──────────────────────────────────────────────────── */}
      {list.isLoading || gpsState === 'checking' ? (
        <View className="items-center py-4">
          <ActivityIndicator />
        </View>
      ) : (
        <Button
          label={actionLabel}
          variant={actionVariant}
          disabled={actionDisabled || pending}
          loading={pending}
          onPress={actionOnPress}
          className="rounded-card py-4"
        />
      )}

      {/* ── Banner (info / warn / bad / ok) ────────────────────────────────── */}
      {bannerText ? (
        <View
          className={`flex-row items-start gap-2 rounded-card border px-3 py-3 ${bannerBg ?? ''} ${bannerBorder ?? ''}`}
        >
          <Text className={`flex-1 text-sm ${bannerTextClass}`}>{bannerText}</Text>
        </View>
      ) : null}

      {/* ── GPS unavailable → open settings CTA ────────────────────────────── */}
      {gpsState === 'unavailable' ? (
        <Button
          label={t('m:absen.openSettings')}
          variant="danger"
          onPress={() => void ExpoLinking.openSettings()}
          className="rounded-card"
        />
      ) : null}

      {/* ── Masuk / Keluar tiles ───────────────────────────────────────────── */}
      <View className="flex-row gap-3">
        <Card className="flex-1">
          <View className="flex-row items-center gap-1.5">
            <Text variant="caption" className="text-text-3">
              {t('m:absen.masuk')}
            </Text>
          </View>
          <Text
            className={`mt-1 font-bold ${masukTime === '—:—' ? 'text-text-3' : 'text-text'}`}
            style={{ fontSize: 20 }}
          >
            {masukTime}
          </Text>
        </Card>
        <Card className="flex-1">
          <View className="flex-row items-center gap-1.5">
            <Text variant="caption" className="text-text-3">
              {t('m:absen.keluar')}
            </Text>
          </View>
          <Text
            className={`mt-1 font-bold ${keluarTime === '—:—' ? 'text-text-3' : 'text-text'}`}
            style={{ fontSize: 20 }}
          >
            {keluarTime}
          </Text>
        </Card>
      </View>

      {/* ── History navigation ─────────────────────────────────────────────── */}
      <Pressable
        className="items-center py-2"
        onPress={() => router.push('/attendance-history')}
      >
        <Text className="text-primary font-semibold text-sm">
          {t('m:attendance.historyTitle')} →
        </Text>
      </Pressable>
    </ScrollView>
  );
}
