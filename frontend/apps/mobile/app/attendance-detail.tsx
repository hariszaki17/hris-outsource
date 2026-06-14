/**
 * Detail Kehadiran (per-day attendance detail) — E5
 * Read-only detail for one attendance record, opened by tapping a Riwayat Kehadiran card.
 * Design reference: brainstorm.pen frame CzLHW "Agen · Detail Kehadiran (Mobile)".
 *
 * Layout (CzLHW):
 *   ← Detail Kehadiran                       (AppBar, safe-area top)
 *   ┌ HeadCard ────────────────────────────┐
 *   │ {date} · {shift subline}     [● pill] │
 *   │ [Telat n menit] [Dalam/Luar geofence] │
 *   └───────────────────────────────────────┘
 *   ┌ Clock-in ─────────────────────────────┐ Waktu / Lokasi / Jarak geofence
 *   ┌ Clock-out ────────────────────────────┐ Waktu / Lokasi / Jarak geofence
 *   ┌ Foto clock-in ────────────────────────┐ image placeholder
 *
 * Data: by-id endpoint `GET /attendance/{id}` (useGetAttendance) for the full record
 * (geofence in/out detail). Falls back to resolving from useListAttendance by id, mirroring
 * app/correction.tsx, in case the by-id read is unavailable.
 */
import {
  type Attendance,
  type AttendancePage,
  type GetAttendance200,
  useGetAttendance,
  useListAttendance,
} from '@swp/api-client/e5';
import { color } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared/datetime';
import { useLocalSearchParams, useRouter } from 'expo-router';
import {
  ChevronLeft,
  ClockAlert,
  Image as ImageIcon,
  LogIn,
  LogOut,
  MapPinCheck,
  MapPinOff,
} from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { Pressable, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Card } from '../src/ui/Card';
import { Text } from '../src/ui/Text';

// ── Helpers ──────────────────────────────────────────────────────────────────

const EMDASH = '—';

function timeWib(iso?: string | null): string {
  return iso ? `${formatInstant(iso, { timeStyle: 'short' })} WIB` : EMDASH;
}

function coords(lat?: number | null, lng?: number | null): string {
  if (lat == null || lng == null) return EMDASH;
  return `${lat.toFixed(5)}, ${lng.toFixed(5)}`;
}

function dayLabel(iso?: string | null): string {
  if (!iso) return EMDASH;
  return new Date(iso).toLocaleDateString('id-ID', {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    timeZone: 'Asia/Jakarta',
  });
}

function shiftPeriodLabel(item: Attendance): string {
  const h = item.shift_start_at ? new Date(item.shift_start_at).getHours() : null;
  if (h === null || h < 12) return 'Pagi';
  if (h < 17) return 'Siang';
  return 'Malam';
}

const statusLabel: Record<string, string> = {
  PRESENT: 'Hadir',
  LATE: 'Terlambat',
  INCOMPLETE: 'Tidak lengkap',
  ABSENT: 'Absen',
  ON_LEAVE: 'Cuti',
};

type PillTone = 'ok' | 'warn' | 'bad' | 'info' | 'orange';
const statusTone: Record<string, PillTone> = {
  PRESENT: 'ok',
  LATE: 'warn',
  INCOMPLETE: 'orange',
  ABSENT: 'bad',
  ON_LEAVE: 'info',
};
const pillBg: Record<PillTone, string> = {
  ok: 'bg-ok-bg',
  warn: 'bg-warn-bg',
  bad: 'bg-bad-bg',
  info: 'bg-info-bg',
  orange: 'bg-orange-bg',
};
const pillBorder: Record<PillTone, string> = {
  ok: 'border-ok-border',
  warn: 'border-warn-border',
  bad: 'border-bad-border',
  info: 'border-info-border',
  orange: 'border-orange-border',
};
const pillText: Record<PillTone, string> = {
  ok: 'text-ok-text',
  warn: 'text-warn-text',
  bad: 'text-bad-text',
  info: 'text-info-text',
  orange: 'text-orange-text',
};
const pillDot: Record<PillTone, string> = {
  ok: color.ok.text,
  warn: color.warn.text,
  bad: color.bad.text,
  info: color.info.text,
  orange: color.orange.text,
};

// ── Status pill (comp/StatusPill: dot + label) ────────────────────────────────

function StatusPill({ status }: { status: string }) {
  const tone = statusTone[status] ?? 'info';
  return (
    <View
      className={`flex-row items-center gap-1.5 self-start rounded-pill border px-2.5 py-1 ${pillBg[tone]} ${pillBorder[tone]}`}
    >
      <View
        className="rounded-full"
        style={{ width: 7, height: 7, backgroundColor: pillDot[tone] }}
      />
      <Text variant="caption" weight="semibold" className={pillText[tone]}>
        {statusLabel[status] ?? status}
      </Text>
    </View>
  );
}

// ── Geofence flag badge ────────────────────────────────────────────────────────

function FlagBadge({
  tone,
  icon,
  label,
}: {
  tone: 'ok' | 'bad' | 'warn';
  icon: React.ReactNode;
  label: string;
}) {
  return (
    <View
      className={`flex-row items-center gap-1 self-start rounded-control border px-2 py-1 ${pillBg[tone]} ${pillBorder[tone]}`}
    >
      {icon}
      <Text variant="badge" className={pillText[tone]}>
        {label}
      </Text>
    </View>
  );
}

// ── Clock card (Clock-in / Clock-out) ──────────────────────────────────────────

function DetailRow({
  label,
  value,
  valueClass,
  mono,
}: {
  label: string;
  value: string;
  valueClass?: string;
  mono?: boolean;
}) {
  return (
    <View className="flex-row items-center justify-between">
      <Text variant="caption" weight="medium" className="text-text-3">
        {label}
      </Text>
      {mono ? (
        <Text variant="label" mono weight="medium" className={valueClass ?? 'text-text'}>
          {value}
        </Text>
      ) : (
        <Text variant="caption" weight="semibold" className={valueClass ?? 'text-text'}>
          {value}
        </Text>
      )}
    </View>
  );
}

function ClockCard({
  icon,
  title,
  time,
  location,
  distance,
  distanceClass,
}: {
  icon: React.ReactNode;
  title: string;
  time: string;
  location: string;
  distance: string;
  distanceClass: string;
}) {
  const { t } = useTranslation();
  return (
    <View className="gap-2.5 rounded-card border border-border bg-surface p-3.5">
      <View className="flex-row items-center gap-2">
        <View className="h-7 w-7 items-center justify-center rounded-control bg-primary-soft">
          {icon}
        </View>
        <Text variant="label" className="text-text">
          {title}
        </Text>
      </View>
      <DetailRow label={t('m:detailKehadiran.labelWaktu')} value={time} mono />
      <DetailRow label={t('m:detailKehadiran.labelLokasi')} value={location} mono />
      <DetailRow
        label={t('m:detailKehadiran.labelJarak')}
        value={distance}
        valueClass={distanceClass}
      />
    </View>
  );
}

// ── Screen ──────────────────────────────────────────────────────────────────

export default function AttendanceDetail() {
  const { t } = useTranslation();
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { attendanceId } = useLocalSearchParams<{ attendanceId: string; date?: string }>();

  // Prefer the by-id endpoint (richest detail: geofence in/out). Fall back to list-find
  // (mirrors correction.tsx) if the by-id read errors or is unavailable.
  const byId = useGetAttendance(attendanceId, { query: { enabled: !!attendanceId } });
  const byIdRecord = (byId.data?.data as GetAttendance200 | undefined)?.data;

  const list = useListAttendance(
    { limit: 50 },
    { query: { enabled: !!attendanceId && !byIdRecord } },
  );
  const listItems: Attendance[] = (list.data?.data as AttendancePage | undefined)?.data ?? [];
  const record = byIdRecord ?? listItems.find((a) => a.id === attendanceId);

  return (
    <View className="flex-1 bg-app-bg">
      {/* AppBar — safe-area top clears the status bar / Dynamic Island */}
      <View
        className="flex-row items-center gap-2 px-4 pb-3"
        style={{ paddingTop: insets.top + 8 }}
      >
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={22} color={color.text} />
        </Pressable>
        <Text variant="screenTitle" className="text-text">
          {t('m:detailKehadiran.title')}
        </Text>
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{ paddingTop: 6, paddingHorizontal: 16, paddingBottom: 16, gap: 14 }}
      >
        {record ? (
          <Detail record={record} />
        ) : (
          <Card>
            <Text className="text-text-2">{t('m:detailKehadiran.empty')}</Text>
          </Card>
        )}
      </ScrollView>
    </View>
  );
}

function Detail({ record }: { record: Attendance }) {
  const { t } = useTranslation();

  const period = shiftPeriodLabel(record);
  const site = record.site_name ?? record.company_name ?? '';
  const shiftLine = record.shift_start_at
    ? `Shift ${period} · ${formatInstant(record.shift_start_at, { timeStyle: 'short' })} – ${formatInstant(record.shift_end_at ?? record.shift_start_at, { timeStyle: 'short' })}${site ? ` · ${site}` : ''}`
    : site;

  const isLate = (record.late_minutes ?? 0) > 0;
  const geoIn = record.geofence_in;

  function distanceText(geo?: { inside: boolean; distance_m: number } | null): {
    text: string;
    cls: string;
  } {
    if (!geo) return { text: EMDASH, cls: 'text-text' };
    return geo.inside
      ? {
          text: t('m:detailKehadiran.distInside', { distance: geo.distance_m }),
          cls: 'text-ok-text',
        }
      : {
          text: t('m:detailKehadiran.distOutside', { distance: geo.distance_m }),
          cls: 'text-bad-text',
        };
  }

  const inDist = distanceText(record.geofence_in);
  const outDist = distanceText(record.geofence_out);

  return (
    <>
      {/* HeadCard — date + shift subline + status pill + flag badges */}
      <View className="gap-2 rounded-card border border-border bg-surface p-4">
        <View className="flex-row items-start justify-between">
          <View className="flex-1 gap-0.5">
            <Text variant="subtitle" className="text-text">
              {dayLabel(record.shift_start_at ?? record.check_in_at)}
            </Text>
            {shiftLine ? (
              <Text variant="caption" weight="medium" className="text-text-2">
                {shiftLine}
              </Text>
            ) : null}
          </View>
          <StatusPill status={record.status} />
        </View>

        {isLate || geoIn ? (
          <View className="flex-row flex-wrap gap-1.5 pt-1">
            {isLate ? (
              <FlagBadge
                tone="warn"
                icon={<ClockAlert size={11} color={color.warn.text} />}
                label={t('m:detailKehadiran.lateBadge', { count: record.late_minutes })}
              />
            ) : null}
            {geoIn ? (
              <FlagBadge
                tone={geoIn.inside ? 'ok' : 'bad'}
                icon={
                  geoIn.inside ? (
                    <MapPinCheck size={11} color={color.ok.text} />
                  ) : (
                    <MapPinOff size={11} color={color.bad.text} />
                  )
                }
                label={
                  geoIn.inside
                    ? t('m:detailKehadiran.geoInside')
                    : t('m:detailKehadiran.geoOutside')
                }
              />
            ) : null}
          </View>
        ) : null}
      </View>

      {/* Clock-in */}
      <ClockCard
        icon={<LogIn size={16} color={color.primary} />}
        title={t('m:detailKehadiran.clockIn')}
        time={timeWib(record.check_in_at)}
        location={coords(record.lat_in, record.lng_in)}
        distance={inDist.text}
        distanceClass={inDist.cls}
      />

      {/* Clock-out */}
      <ClockCard
        icon={<LogOut size={16} color={color.primary} />}
        title={`${t('m:detailKehadiran.clockOut')}${record.auto_closed ? ` ${t('m:detailKehadiran.auto')}` : ''}`}
        time={timeWib(record.check_out_at)}
        location={coords(record.lat_out, record.lng_out)}
        distance={outDist.text}
        distanceClass={outDist.cls}
      />

      {/* Foto clock-in — placeholder (file-id→URL resolution not wired in app) */}
      <View className="gap-2 rounded-card border border-border bg-surface p-3.5">
        <Text variant="caption" weight="bold" className="text-text">
          {t('m:detailKehadiran.photoLabel')}
        </Text>
        <View
          className="items-center justify-center rounded-control border border-border-soft bg-surface-2"
          style={{ height: 120 }}
        >
          <ImageIcon size={28} color={color.text3} />
        </View>
      </View>
    </>
  );
}
