/**
 * Riwayat Kehadiran (Attendance History) — E5
 * Design reference: riwayat-kehadiran.png
 *
 * Layout:
 *   - Header: "Riwayat Kehadiran" + calendar icon (right)
 *   - Stat row: 3 tiles (Hadir=ok/teal, Telat=warn, Tdk lengkap=bad)
 *   - Month group label ("JUNI 2026")
 *   - List of attendance cards: date, shift time, status badge, verification note
 *
 * Status → StatusBadge tone: PRESENT=ok(teal), LATE=warn, INCOMPLETE→bad, ABSENT=bad
 * Verification line: TERVERIFIKASI_OTOMATIS=info(blue), MENUNGGU=warn, badge per item
 */
import {
  type Attendance,
  type AttendancePage,
  useListAttendance,
} from '@swp/api-client/e5';
import { formatInstant } from '@swp/shared/datetime';
import { useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, View } from 'react-native';
import { Card } from '../src/ui/Card';
import { StatusBadge } from '../src/ui/StatusBadge';
import { Text } from '../src/ui/Text';

// ── Helpers ──────────────────────────────────────────────────────────────────

function timeOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { timeStyle: 'short' }) : '—:—';
}

function monthGroup(iso?: string | null): string {
  if (!iso) return '';
  return new Date(iso).toLocaleDateString('id-ID', {
    month: 'long',
    year: 'numeric',
    timeZone: 'Asia/Jakarta',
  }).toUpperCase();
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

/** Shift period label from shift_start_at/shift_end_at */
function shiftPeriod(item: Attendance): string {
  const h = item.shift_start_at
    ? new Date(item.shift_start_at).getHours()
    : null;
  if (h === null) return 'Pagi';
  if (h < 12) return 'Pagi';
  if (h < 17) return 'Siang';
  return 'Malam';
}

// Verification status label for the attendance row sub-line
function verifLine(item: Attendance): { label: string; textClass: string } | null {
  switch (item.verification_status) {
    case 'AUTO_APPROVED':
      return { label: 'Terverifikasi otomatis', textClass: 'text-info-text' };
    case 'PENDING':
      return { label: 'Menunggu verifikasi', textClass: 'text-warn-text' };
    case 'ESCALATED':
      return { label: 'Menunggu verifikasi', textClass: 'text-warn-text' };
    default:
      return null;
  }
}

// Map attendance status to a display label from the .pen PNGs
const statusLabel: Record<string, string> = {
  PRESENT: 'Hadir',
  LATE: 'Terlambat',
  INCOMPLETE: 'Tidak lengkap',
  ABSENT: 'Absen',
  ON_LEAVE: 'Cuti',
};

// INCOMPLETE should map to the orange/bad tone — extend StatusBadge by status key
// (StatusBadge already has ok/warn/bad/info → INCOMPLETE=warn in shared component but .pen
// uses a distinct "Tidak lengkap" red badge; we keep it as-is through StatusBadge 'bad' tone
// via a custom status key 'INCOMPLETE_BAD' passed along with its label — simpler: just pass
// status='ABSENT' for incomplete to get bad tone, but that mislabels. Instead we rely on
// the existing StatusBadge mapping where INCOMPLETE→warn and add the correct label.)
// Note: per REFERENCE.md TDK_LENGKAP=orange. The StatusBadge 'warn' tone uses amber.
// Since we cannot edit src/ui/StatusBadge, we pass the exact status string and rely on
// the existing mapping (INCOMPLETE→warn). This is a foundation gap noted in findings.

// ── Stat tile ────────────────────────────────────────────────────────────────

function StatTile({
  value,
  label,
  textClass,
}: {
  value: number;
  label: string;
  textClass: string;
}) {
  return (
    <Card className="flex-1 items-center py-3">
      <Text
        className={`font-bold ${textClass}`}
        style={{ fontSize: 28, fontWeight: '700' }}
      >
        {String(value)}
      </Text>
      <Text variant="caption" className="mt-1 text-center text-text-3">
        {label}
      </Text>
    </Card>
  );
}

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

  return (
    <Card>
      <View className="flex-row items-start justify-between">
        <Text variant="body" className="font-semibold text-text">
          {dayLabel(item.check_in_at ?? item.shift_start_at)}
        </Text>
        <StatusBadge status={item.status} label={label} />
      </View>
      <Text variant="caption" className="mt-1 text-text-2">
        {period} · {start} – {end}
      </Text>
      {verif ? (
        <View className="mt-2 flex-row items-center gap-1">
          <Text
            variant="caption"
            className={`font-semibold ${verif.textClass}`}
          >
            {verif.label}
          </Text>
        </View>
      ) : item.auto_closed ? (
        <Text variant="caption" className="mt-2 text-text-3">
          Auto clock-out oleh sistem
        </Text>
      ) : null}
      {/* Correction shortcut */}
      <Pressable
        className="mt-2 self-start"
        onPress={() =>
          router.push({
            pathname: '/correction-tracker',
          })
        }
      >
        <Text className="text-primary text-xs font-semibold">Koreksi →</Text>
      </Pressable>
    </Card>
  );
}

// ── Screen ────────────────────────────────────────────────────────────────────

export default function AttendanceHistoryScreen() {
  const { t } = useTranslation();
  const list = useListAttendance({ limit: 30 });
  const body = list.data?.data as AttendancePage | undefined;
  const items: Attendance[] = body?.data ?? [];

  // Stats
  const hadir = items.filter((a) => a.status === 'PRESENT').length;
  const telat = items.filter((a) => a.status === 'LATE').length;
  const tdkLengkap = items.filter(
    (a) => a.status === 'INCOMPLETE' || a.status === 'ABSENT',
  ).length;

  // Group by month
  const groups: Map<string, Attendance[]> = new Map();
  for (const item of items) {
    const key = monthGroup(item.check_in_at ?? item.shift_start_at);
    if (!key) continue;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(item);
  }

  return (
    <ScrollView className="flex-1 bg-app-bg" contentContainerStyle={{ padding: 16, gap: 12 }}>
      {/* Stat tiles */}
      <View className="flex-row gap-3">
        <StatTile value={hadir} label={t('m:riwayat.statHadir')} textClass="text-ok-text" />
        <StatTile value={telat} label={t('m:riwayat.statTelat')} textClass="text-warn-text" />
        <StatTile
          value={tdkLengkap}
          label={t('m:riwayat.statTdkLengkap')}
          textClass="text-bad-text"
        />
      </View>

      {list.isLoading ? (
        <View className="items-center py-10">
          <ActivityIndicator />
        </View>
      ) : list.isError ? (
        <Card>
          <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
        </Card>
      ) : items.length === 0 ? (
        <Card>
          <Text variant="caption" className="text-text-3">
            {t('m:riwayat.empty')}
          </Text>
        </Card>
      ) : (
        Array.from(groups.entries()).map(([month, rows]) => (
          <View key={month} className="gap-3">
            <Text
              variant="caption"
              className="font-semibold text-text-3 tracking-wide"
            >
              {month}
            </Text>
            {rows.map((item) => (
              <HistoryRow key={item.id} item={item} />
            ))}
          </View>
        ))
      )}
    </ScrollView>
  );
}
