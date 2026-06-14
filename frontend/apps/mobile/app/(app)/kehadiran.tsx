/**
 * Kehadiran (Attendance) tab — E5
 * Agent self-service attendance hub. Replaces the old "Jadwal" tab: surfaces the
 * agent's own attendance history (status per day) + a prominent "Ajukan Koreksi"
 * entry, since correction is time-sensitive and was previously buried under Pengajuan.
 * Design: brainstorm.pen "Agen · Riwayat Kehadiran (Mobile)" (GJI1a), Kehadiran tab active.
 */
import {
  type Attendance,
  type AttendancePage,
  type AttendanceStatus,
  useListAttendance,
} from '@swp/api-client/e5';
import { color } from '@swp/design-tokens';
import { type DateRange, formatInstant, monthRange } from '@swp/shared/datetime';
import { useRouter } from 'expo-router';
import { Bell, PencilLine } from 'lucide-react-native';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { CalendarRangeSheet } from '../../src/features/attendance/CalendarRangeSheet';
import { DateRangeChip } from '../../src/features/attendance/DateRangeChip';
import { DateRangeSheet } from '../../src/features/attendance/DateRangeSheet';
import { FilteredZero } from '../../src/features/attendance/FilteredZero';
import {
  type StatusCounts,
  StatusFilterChips,
} from '../../src/features/attendance/StatusFilterChips';
import { Card } from '../../src/ui/Card';
import { StatusBadge } from '../../src/ui/StatusBadge';
import { Text } from '../../src/ui/Text';

// ── Helpers ──────────────────────────────────────────────────────────────────

function timeOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { timeStyle: 'short' }) : '—:—';
}

function monthGroup(iso?: string | null): string {
  if (!iso) return '';
  return new Date(iso)
    .toLocaleDateString('id-ID', { month: 'long', year: 'numeric', timeZone: 'Asia/Jakarta' })
    .toUpperCase();
}

function dayLabel(iso?: string | null): string {
  if (!iso) return '—';
  return new Date(iso).toLocaleDateString('id-ID', {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    timeZone: 'Asia/Jakarta',
  });
}

function shiftPeriod(item: Attendance): string {
  const h = item.shift_start_at ? new Date(item.shift_start_at).getHours() : null;
  if (h === null) return 'Pagi';
  if (h < 12) return 'Pagi';
  if (h < 17) return 'Siang';
  return 'Malam';
}

function verifLine(item: Attendance): { label: string; textClass: string } | null {
  switch (item.verification_status) {
    case 'AUTO_APPROVED':
      return { label: 'Terverifikasi otomatis', textClass: 'text-info-text' };
    case 'PENDING':
    case 'ESCALATED':
      return { label: 'Menunggu verifikasi', textClass: 'text-warn-text' };
    default:
      return null;
  }
}

const statusLabel: Record<string, string> = {
  PRESENT: 'Hadir',
  LATE: 'Terlambat',
  INCOMPLETE: 'Tidak lengkap',
  ABSENT: 'Absen',
  ON_LEAVE: 'Cuti',
};

// ── History row ───────────────────────────────────────────────────────────────

function HistoryRow({ item }: { item: Attendance }) {
  const router = useRouter();
  const period = shiftPeriod(item);
  const start = timeOf(item.check_in_at);
  const end = item.check_out_at
    ? timeOf(item.check_out_at)
    : item.auto_closed
      ? `auto ${timeOf(item.shift_end_at)}`
      : '—:—';
  const verif = verifLine(item);
  const label = statusLabel[item.status] ?? item.status;

  const day = item.shift_start_at ?? item.check_in_at;

  return (
    <Card>
      {/* Card body → read-only Detail Kehadiran (CzLHW). Separate press target from Koreksi. */}
      <Pressable
        onPress={() =>
          router.push({
            pathname: '/attendance-detail',
            params: { attendanceId: item.id, ...(day ? { date: day } : {}) },
          })
        }
      >
        <View className="flex-row items-start justify-between">
          <Text variant="body" weight="semibold" className="text-text">
            {dayLabel(item.check_in_at ?? item.shift_start_at)}
          </Text>
          <StatusBadge status={item.status} label={label} />
        </View>
        <Text variant="caption" className="mt-1 text-text-2">
          {period} · {start} – {end}
        </Text>
        {verif ? (
          <Text variant="caption" weight="semibold" className={`mt-2 ${verif.textClass}`}>
            {verif.label}
          </Text>
        ) : item.auto_closed ? (
          <Text variant="caption" className="mt-2 text-text-3">
            Auto clock-out oleh sistem
          </Text>
        ) : null}
      </Pressable>
      {/* Koreksi → correction form — sibling Pressable so it isn't swallowed by the card tap. */}
      <Pressable className="mt-2 self-start" onPress={() => router.push('/correction')}>
        <Text variant="caption" weight="semibold" className="text-primary">
          Koreksi →
        </Text>
      </Pressable>
    </Card>
  );
}

// ── Screen ────────────────────────────────────────────────────────────────────

export default function KehadiranScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const insets = useSafeAreaInsets();

  // ── Filter state (AR-10 date-range · AR-11 single-select status + Semua reset) ──
  const [range, setRange] = useState<DateRange>(() => monthRange());
  const [status, setStatus] = useState<AttendanceStatus | null>(null);
  const [presetsOpen, setPresetsOpen] = useState(false);
  const [calendarOpen, setCalendarOpen] = useState(false);

  // date_from/date_to (YYYY-MM-DD, Asia/Jakarta) + single status. Semua omits `status`.
  // Limit kept high: an agent-month of records is small, so chip counts cover the range.
  const list = useListAttendance({
    limit: 100,
    date_from: range.from,
    date_to: range.to,
    sort: 'shift_start_at:desc',
    ...(status ? { status: [status] } : {}),
  });
  const body = list.data?.data as AttendancePage | undefined;
  const items: Attendance[] = body?.data ?? [];

  // Status counts reflect the CURRENT range. We fetch ALL statuses for the count baseline by
  // ignoring the in-flight `status` filter against a separate range-only query.
  const countList = useListAttendance({
    limit: 100,
    date_from: range.from,
    date_to: range.to,
  });
  const countItems: Attendance[] = (countList.data?.data as AttendancePage | undefined)?.data ?? [];

  const counts: StatusCounts = useMemo(() => {
    const c: StatusCounts = {
      total: countItems.length,
      PRESENT: 0,
      LATE: 0,
      INCOMPLETE: 0,
      ABSENT: 0,
      ON_LEAVE: 0,
    };
    for (const a of countItems) {
      if (a.status in c) c[a.status as keyof Omit<StatusCounts, 'total'>] += 1;
    }
    return c;
  }, [countItems]);

  const groups: Map<string, Attendance[]> = new Map();
  for (const item of items) {
    const key = monthGroup(item.shift_start_at ?? item.check_in_at);
    if (!key) continue;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)?.push(item);
  }

  const filteredZero = !list.isLoading && !list.isError && items.length === 0;

  return (
    <View className="flex-1 bg-app-bg">
      {/* AppBar: "Riwayat Kehadiran" + bell */}
      <View
        className="flex-row items-center justify-between px-4 pb-2"
        style={{ paddingTop: insets.top + 8 }}
      >
        <Text variant="screenTitle">{t('m:riwayat.title')}</Text>
        <Pressable onPress={() => router.push('/notifications')} hitSlop={8}>
          <Bell size={22} color={color.text2} />
        </Pressable>
      </View>

      <ScrollView className="flex-1" contentContainerStyle={{ padding: 16, gap: 12 }}>
        {/* Date-range chip → presets sheet */}
        <DateRangeChip range={range} onPress={() => setPresetsOpen(true)} />

        {/* Status filter chips (single-select; Semua resets) */}
        <StatusFilterChips selected={status} counts={counts} onSelect={setStatus} />

        {/* Prominent correction entry — was buried under Pengajuan before */}
        <Pressable
          onPress={() => router.push('/correction')}
          className="flex-row items-center justify-center gap-2.5 rounded-card px-5 py-3.5"
          style={{ backgroundColor: color.primary }}
        >
          <PencilLine size={18} color={color.surface} />
          <Text variant="subtitle" style={{ color: color.surface }}>
            {t('m:pengajuan.koreksi')}
          </Text>
        </Pressable>

        {list.isLoading ? (
          <View className="items-center py-10">
            <ActivityIndicator />
          </View>
        ) : list.isError ? (
          <Card>
            <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
          </Card>
        ) : filteredZero ? (
          <FilteredZero
            showResetStatus={status !== null}
            onResetStatus={() => setStatus(null)}
            onWidenRange={() => setPresetsOpen(true)}
          />
        ) : (
          Array.from(groups.entries()).map(([month, rows]) => (
            <View key={month} className="gap-3">
              <Text variant="caption" weight="semibold" className="text-text-3 tracking-wide">
                {month}
              </Text>
              {rows.map((item) => (
                <HistoryRow key={item.id} item={item} />
              ))}
            </View>
          ))
        )}
      </ScrollView>

      {/* Overlays */}
      <DateRangeSheet
        visible={presetsOpen}
        current={range}
        onClose={() => setPresetsOpen(false)}
        onApply={setRange}
        onCustom={() => {
          setPresetsOpen(false);
          setCalendarOpen(true);
        }}
      />
      <CalendarRangeSheet
        visible={calendarOpen}
        initial={range}
        onClose={() => setCalendarOpen(false)}
        onApply={setRange}
      />
    </View>
  );
}
