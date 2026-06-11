/**
 * Shift Leader — Beranda (Home Dashboard) screen.
 *
 * Design reference: .pen frame UMzuO
 * Feature: F10.2 · DB-8 (LeaderDashboard mobile surface)
 * API: useGetMyDashboard() → Dashboard → narrow to LeaderDashboard (role === 'shift_leader')
 *
 * Layout (vertical): AppBar title "Beranda" + company scope subtitle → Body sections:
 *  1. Hari Ini   — date + 4-stat row + pending_verifications line
 *  2. Perlu Tindakan — pending_counts rows with count chips + chevron
 *  3. Peringatan Jadwal — schedule_alerts[] rows with warn/bad tone + deep-link
 *
 * No new API endpoint; no codegen. Types imported from @swp/api-client/e10.
 */
import {
  type Dashboard,
  type LeaderDashboard,
  type LeaderDashboardPendingCounts,
  type LeaderDashboardScheduleAlertsItem,
  type LeaderDashboardToday,
  useGetMyDashboard,
} from '@swp/api-client/e10';
import { formatDate } from '@swp/shared/datetime';
import { useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, View } from 'react-native';
import { Card } from '../../src/ui/Card';
import { Text } from '../../src/ui/Text';

// ─── Sub-components ────────────────────────────────────────────────────────────

/** Single numeric stat chip with a label below. */
function StatBox({
  value,
  label,
  textClass,
}: {
  value: number;
  label: string;
  textClass?: string;
}) {
  return (
    <View className="flex-1 items-center">
      <Text variant="title" className={`font-bold ${textClass ?? 'text-text'}`}>
        {String(value)}
      </Text>
      <Text variant="caption" className="mt-1 text-center text-text-3">
        {label}
      </Text>
    </View>
  );
}

/** "Hari Ini" card — Section 1. */
function HariIniCard({ today }: { today: LeaderDashboardToday }) {
  const { t } = useTranslation();
  return (
    <Card>
      <Text variant="caption" className="text-text-3">
        {formatDate(today.date)}
      </Text>

      {/* 4-stat row */}
      <View className="mt-3 flex-row">
        <StatBox value={today.shifts_total} label={t('m:leaderBeranda.statShifts')} />
        <StatBox
          value={today.clocked_in}
          label={t('m:leaderBeranda.statClockedIn')}
          textClass="text-ok-text"
        />
        <StatBox
          value={today.late_count}
          label={t('m:leaderBeranda.statLate')}
          textClass="text-warn-text"
        />
        <StatBox
          value={today.absent_count}
          label={t('m:leaderBeranda.statAbsent')}
          textClass="text-bad-text"
        />
      </View>

      {/* Pending verifications line */}
      {today.pending_verifications > 0 ? (
        <View className="mt-3 rounded-control bg-warn-bg px-3 py-2">
          <Text variant="caption" className="text-warn-text font-semibold">
            {t('m:leaderBeranda.pendingVerif', { count: today.pending_verifications })}
          </Text>
        </View>
      ) : (
        <Text variant="caption" className="mt-3 text-text-3">
          {t('m:leaderBeranda.noVerifPending')}
        </Text>
      )}
    </Card>
  );
}

/** Count chip pill. */
function CountChip({ count, tone }: { count: number; tone: 'warn' | 'bad' | 'info' }) {
  const bgClass = { warn: 'bg-warn-bg', bad: 'bg-bad-bg', info: 'bg-info-bg' }[tone];
  const textClass = { warn: 'text-warn-text', bad: 'text-bad-text', info: 'text-info-text' }[tone];
  return (
    <View className={`rounded-pill px-2 py-1 ${bgClass}`}>
      <Text className={`text-xs font-semibold ${textClass}`}>{String(count)}</Text>
    </View>
  );
}

/** One row in the "Perlu Tindakan" card. */
function ActionRow({
  label,
  count,
  tone,
  onPress,
}: {
  label: string;
  count: number;
  tone: 'warn' | 'bad' | 'info';
  onPress: () => void;
}) {
  return (
    <Pressable onPress={onPress}>
      <View className="flex-row items-center justify-between py-2">
        <Text variant="body">{label}</Text>
        <View className="flex-row items-center gap-2">
          <CountChip count={count} tone={tone} />
          <Text variant="caption" className="text-text-3">
            ›
          </Text>
        </View>
      </View>
    </Pressable>
  );
}

/** "Perlu Tindakan" card — Section 2. */
function PerluTindakanCard({ pending }: { pending: LeaderDashboardPendingCounts }) {
  const { t } = useTranslation();
  const router = useRouter();

  const allZero =
    pending.attendance_verify === 0 && pending.leave_approve === 0 && pending.ot_approve === 0;

  return (
    <Card>
      <Text variant="body" className="font-semibold">
        {t('m:leaderBeranda.sectionAction')}
      </Text>
      {allZero ? (
        <Text variant="caption" className="mt-2 text-text-3">
          {t('m:leaderBeranda.actionEmpty')}
        </Text>
      ) : (
        <View className="mt-2 divide-y divide-border">
          {pending.attendance_verify > 0 ? (
            <ActionRow
              label={t('m:leaderBeranda.actionVerifAttendance')}
              count={pending.attendance_verify}
              tone="warn"
              // DB-5: deep-link to the attendance verification screen (E5).
              onPress={() => router.push('/attendance')}
            />
          ) : null}
          {pending.leave_approve > 0 ? (
            <ActionRow
              label={t('m:leaderBeranda.actionApproveLeave')}
              count={pending.leave_approve}
              tone="info"
              onPress={() => router.push('/leave')}
            />
          ) : null}
          {pending.ot_approve > 0 ? (
            <ActionRow
              label={t('m:leaderBeranda.actionApproveOT')}
              count={pending.ot_approve}
              tone="info"
              onPress={() => router.push('/overtime')}
            />
          ) : null}
        </View>
      )}
    </Card>
  );
}

/** Tone map for schedule alert kind (warn for coverage/unassigned, bad for expiring). */
const alertKindTone: Record<string, 'warn' | 'bad'> = {
  COVERAGE_GAP: 'warn',
  UNASSIGNED_SHIFT: 'warn',
  PLACEMENT_EXPIRING: 'bad',
};

/** One alert row. */
function AlertRow({
  alert,
  onPress,
}: {
  alert: LeaderDashboardScheduleAlertsItem;
  onPress: () => void;
}) {
  const tone = alertKindTone[alert.kind] ?? 'warn';
  const textClass = tone === 'bad' ? 'text-bad-text' : 'text-warn-text';
  const bgClass = tone === 'bad' ? 'bg-bad-bg' : 'bg-warn-bg';

  return (
    <Pressable onPress={onPress}>
      <View className={`mb-2 rounded-control px-3 py-2 ${bgClass}`}>
        <View className="flex-row items-center justify-between">
          <Text variant="caption" className={`flex-1 font-semibold ${textClass}`}>
            {alert.label}
          </Text>
          {alert.date ? (
            <Text variant="caption" className={`ml-2 ${textClass}`}>
              {formatDate(alert.date)}
            </Text>
          ) : null}
          <Text variant="caption" className={`ml-2 ${textClass}`}>
            ›
          </Text>
        </View>
      </View>
    </Pressable>
  );
}

/** "Peringatan Jadwal" card — Section 3. */
function PeringatanJadwalCard({ alerts }: { alerts: LeaderDashboardScheduleAlertsItem[] }) {
  const { t } = useTranslation();
  const router = useRouter();

  return (
    <Card>
      <Text variant="body" className="font-semibold">
        {t('m:leaderBeranda.sectionAlerts')}
      </Text>
      {alerts.length === 0 ? (
        <Text variant="caption" className="mt-2 text-text-3">
          {t('m:leaderBeranda.alertsEmpty')}
        </Text>
      ) : (
        <View className="mt-2">
          {alerts.map((a, idx) => (
            <AlertRow
              // DB-5: deep-link via alert.deep_link.path when target screen exists.
              // For now route to the closest available screen; TODO: use a:deep_link.path when
              // the E4/E3 screens are wired in the leader tab set.
              key={`${a.kind}-${idx}`}
              alert={a}
              onPress={() => router.push('/attendance')}
            />
          ))}
        </View>
      )}
    </Card>
  );
}

// ─── Main screen ───────────────────────────────────────────────────────────────

export default function LeaderBerandaScreen() {
  const { t } = useTranslation();
  const dash = useGetMyDashboard();
  const payload = dash.data?.data as Dashboard | undefined;

  // Narrow to the shift_leader variant (DB-8: role discriminator).
  const leaderData: LeaderDashboard | null =
    payload && 'role' in payload && payload.role === 'shift_leader'
      ? (payload as LeaderDashboard)
      : null;

  return (
    <ScrollView className="flex-1 bg-app-bg">
      <View className="gap-4 p-6">
        {/* Screen header: title + company scope subtitle (from company.name) */}
        <View>
          <Text variant="title">{t('m:leaderBeranda.title')}</Text>
          {leaderData ? (
            <Text variant="caption" className="mt-1 text-text-3">
              {leaderData.company.name}
            </Text>
          ) : null}
        </View>

        {dash.isLoading ? (
          <View className="items-center py-10">
            <ActivityIndicator />
          </View>
        ) : dash.isError ? (
          <Card>
            <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
          </Card>
        ) : !leaderData ? (
          // DB C-1: dashboard returned but not the shift_leader variant
          <Card>
            <Text variant="caption">{t('m:common.emptyGeneric')}</Text>
          </Card>
        ) : (
          <>
            {/* Section 1: Hari Ini */}
            <HariIniCard today={leaderData.today} />

            {/* Section 2: Perlu Tindakan */}
            <PerluTindakanCard pending={leaderData.pending_counts} />

            {/* Section 3: Peringatan Jadwal */}
            <PeringatanJadwalCard alerts={leaderData.schedule_alerts} />
          </>
        )}
      </View>
    </ScrollView>
  );
}
