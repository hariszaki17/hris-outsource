/**
 * Shift Leader — Beranda (Home Dashboard) screen.
 *
 * Design reference: brainstorm.pen frame UMzuO ("E10 · Beranda Pemimpin Shift (Mobile)").
 * Feature: F10.2 · DB-8 (LeaderDashboard mobile surface)
 * API: useGetMyDashboard() → Dashboard → narrow to LeaderDashboard (role === 'shift_leader')
 *
 * Layout: AppBar (title "Beranda" + bell, company-scope subtitle, bottom border) → Body:
 *  1. Hari Ini        — date + 4 stat tiles (surface-2) + pending-verifications warn line
 *  2. Perlu Tindakan  — pending_counts rows with count chips + chevron
 *  3. Peringatan Jadwal — schedule_alerts[] rows with warn/bad tone icon
 *
 * Scope subtitle = company.name only (service line was removed 2026-06-12; the .pen frame still
 * shows "· Facility Services" but that is stale). No new API endpoint; no codegen.
 */
import {
  type Dashboard,
  type LeaderDashboard,
  type LeaderDashboardPendingCounts,
  type LeaderDashboardScheduleAlertsItem,
  type LeaderDashboardToday,
  useGetMyDashboard,
} from '@swp/api-client/e10';
import { color } from '@swp/design-tokens';
import { formatDate } from '@swp/shared/datetime';
import { useRouter } from 'expo-router';
import { Bell, ChevronRight, TriangleAlert } from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Text } from '../../src/ui/Text';

// ─── Sub-components ────────────────────────────────────────────────────────────

// Card shell — .pen sections: rounded-card, 1px border, surface, padding 14, vertical gap 12.
function SectionCard({ children }: { children: React.ReactNode }) {
  return (
    <View className="gap-3 rounded-card border border-border bg-surface p-3.5">{children}</View>
  );
}

/** One stat tile (surface-2 chip): big colored value + small label. */
function StatTile({
  value,
  label,
  textClass,
}: { value: number; label: string; textClass: string }) {
  return (
    <View className="flex-1 items-center gap-0.5 rounded-[10px] bg-surface-2 p-2">
      <Text variant="cardTitle" className={textClass}>
        {String(value)}
      </Text>
      <Text variant="caption" weight="medium" className="text-text-2">
        {label}
      </Text>
    </View>
  );
}

/** "Hari Ini" card — Section 1. */
function HariIniCard({ today }: { today: LeaderDashboardToday }) {
  const { t } = useTranslation();
  return (
    <SectionCard>
      <View className="flex-row items-center justify-between">
        <Text variant="strong">{t('m:leaderBeranda.sectionToday')}</Text>
        <Text variant="caption" weight="medium" className="text-text-2">
          {formatDate(today.date)}
        </Text>
      </View>

      <View className="flex-row gap-2">
        <StatTile
          value={today.shifts_total}
          label={t('m:leaderBeranda.statShifts')}
          textClass="text-text"
        />
        <StatTile
          value={today.clocked_in}
          label={t('m:leaderBeranda.statClockedIn')}
          textClass="text-ok-text"
        />
        <StatTile
          value={today.late_count}
          label={t('m:leaderBeranda.statLate')}
          textClass="text-orange-text"
        />
        <StatTile
          value={today.absent_count}
          label={t('m:leaderBeranda.statAbsent')}
          textClass="text-bad-text"
        />
      </View>

      {today.pending_verifications > 0 ? (
        <View className="flex-row items-center justify-between rounded-input bg-warn-bg px-2.5 py-2">
          <Text variant="caption" weight="medium" className="text-warn-text">
            {t('m:leaderBeranda.pendingVerif')}
          </Text>
          <Text variant="caption" weight="bold" className="text-warn-text">
            {String(today.pending_verifications)}
          </Text>
        </View>
      ) : null}
    </SectionCard>
  );
}

/** Count chip pill. */
function CountChip({ count, tone }: { count: number; tone: 'warn' | 'bad' | 'info' }) {
  const bgClass = { warn: 'bg-warn-bg', bad: 'bg-bad-bg', info: 'bg-info-bg' }[tone];
  const textClass = { warn: 'text-warn-text', bad: 'text-bad-text', info: 'text-info-text' }[tone];
  return (
    <View className={`rounded-pill px-2 py-0.5 ${bgClass}`}>
      <Text variant="caption" weight="semibold" className={textClass}>
        {String(count)}
      </Text>
    </View>
  );
}

/** One row in the "Perlu Tindakan" card. */
function ActionRow({
  label,
  count,
  tone,
  onPress,
  divider,
}: {
  label: string;
  count: number;
  tone: 'warn' | 'bad' | 'info';
  onPress: () => void;
  divider: boolean;
}) {
  return (
    <Pressable
      onPress={onPress}
      className={`flex-row items-center justify-between py-2 ${divider ? 'border-t border-border-soft' : ''}`}
    >
      <Text variant="secondary" weight="medium" className="text-text">
        {label}
      </Text>
      <View className="flex-row items-center gap-2">
        <CountChip count={count} tone={tone} />
        <ChevronRight size={16} color={color.text2} />
      </View>
    </Pressable>
  );
}

/** "Perlu Tindakan" card — Section 2. */
function PerluTindakanCard({ pending }: { pending: LeaderDashboardPendingCounts }) {
  const { t } = useTranslation();
  const router = useRouter();
  const rows = [
    {
      label: t('m:leaderBeranda.actionVerifAttendance'),
      count: pending.attendance_verify,
      tone: 'warn' as const,
      route: '/sl-verifikasi',
    },
    {
      label: t('m:leaderBeranda.actionApproveLeave'),
      count: pending.leave_approve,
      tone: 'info' as const,
      route: '/leave',
    },
    {
      label: t('m:leaderBeranda.actionApproveOT'),
      count: pending.ot_approve,
      tone: 'info' as const,
      route: '/overtime',
    },
  ];

  return (
    <SectionCard>
      <Text variant="strong">{t('m:leaderBeranda.sectionAction')}</Text>
      <View>
        {rows.map((r, i) => (
          <ActionRow
            key={r.route}
            label={r.label}
            count={r.count}
            tone={r.tone}
            divider={i > 0}
            onPress={() => router.push(r.route)}
          />
        ))}
      </View>
    </SectionCard>
  );
}

/** Tone map for schedule alert kind (warn for coverage/unassigned, bad for expiring). */
const alertKindTone: Record<string, 'warn' | 'bad'> = {
  COVERAGE_GAP: 'bad',
  UNASSIGNED_SHIFT: 'warn',
  PLACEMENT_EXPIRING: 'bad',
};

/** One alert row — tone-tinted bg + triangle-alert icon (matches .pen CardAlerts). */
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
  const iconColor = tone === 'bad' ? color.bad.text : color.warn.text;

  return (
    <Pressable
      onPress={onPress}
      className={`flex-row items-center gap-2.5 rounded-input px-2.5 py-1.5 ${bgClass}`}
    >
      <TriangleAlert size={16} color={iconColor} />
      <View className="flex-1 gap-px">
        <Text variant="caption" weight="semibold" className={textClass}>
          {alert.label}
        </Text>
        {alert.date ? (
          <Text variant="badge" weight="medium" className={textClass}>
            {formatDate(alert.date)}
          </Text>
        ) : null}
      </View>
    </Pressable>
  );
}

/** "Peringatan Jadwal" card — Section 3. */
function PeringatanJadwalCard({ alerts }: { alerts: LeaderDashboardScheduleAlertsItem[] }) {
  const { t } = useTranslation();
  const router = useRouter();

  return (
    <SectionCard>
      <Text variant="strong">{t('m:leaderBeranda.sectionAlerts')}</Text>
      {alerts.length === 0 ? (
        <Text variant="caption" className="text-text-3">
          {t('m:leaderBeranda.alertsEmpty')}
        </Text>
      ) : (
        <View className="gap-2">
          {alerts.map((a, idx) => (
            <AlertRow
              key={`${a.kind}-${idx}`}
              alert={a}
              onPress={() => router.push('/sl-verifikasi')}
            />
          ))}
        </View>
      )}
    </SectionCard>
  );
}

// ─── Main screen ───────────────────────────────────────────────────────────────

export default function LeaderBerandaScreen() {
  const { t } = useTranslation();
  const insets = useSafeAreaInsets();
  const router = useRouter();
  const dash = useGetMyDashboard();
  // /dashboards/me 200 is typed unenveloped but the API double-envelopes in `{ data }`, so the
  // payload is one level deeper than the generated type (same unwrap as leave.tsx / index.tsx).
  // TODO: envelope the dashboard 200 schema in docs/api/E10 and regenerate, then drop this cast.
  const env = dash.data?.data as { data?: Dashboard } | undefined;
  const payload = env?.data;
  const leaderData: LeaderDashboard | null =
    payload && 'role' in payload && payload.role === 'shift_leader'
      ? (payload as LeaderDashboard)
      : null;

  return (
    <View className="flex-1 bg-app-bg">
      {/* AppBar: title + bell, company-scope subtitle, bottom border */}
      <View
        className="gap-px border-b border-border bg-surface px-4 pb-3"
        style={{ paddingTop: insets.top + 8 }}
      >
        <View className="flex-row items-center justify-between">
          <Text variant="section">{t('m:leaderBeranda.title')}</Text>
          <Pressable onPress={() => router.push('/notifications')} hitSlop={8}>
            <Bell size={24} color={color.text2} />
          </Pressable>
        </View>
        {leaderData ? (
          <Text variant="secondary" weight="medium" className="text-text-2">
            {leaderData.company.name}
          </Text>
        ) : null}
      </View>

      <ScrollView contentContainerStyle={{ padding: 16, gap: 14 }}>
        {dash.isLoading ? (
          <View className="items-center py-10">
            <ActivityIndicator />
          </View>
        ) : dash.isError ? (
          <SectionCard>
            <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
          </SectionCard>
        ) : !leaderData ? (
          <SectionCard>
            <Text variant="caption">{t('m:common.emptyGeneric')}</Text>
          </SectionCard>
        ) : (
          <>
            <HariIniCard today={leaderData.today} />
            <PerluTindakanCard pending={leaderData.pending_counts} />
            <PeringatanJadwalCard alerts={leaderData.schedule_alerts} />
          </>
        )}
      </ScrollView>
    </View>
  );
}
