// Agen · Cuti Saya — built from brainstorm.pen frame o1BUa.
// Two sections: per-type balance flat-list ("Saldo per jenis") over the request history
// ("Riwayat pengajuan"). Balances come from the per-type QuotaMeter contract
// (useGetEmployeeTypeBalances / LeaveTypeBalance), NOT the retired grant-lot model.
import {
  type LeaveRequest,
  type LeaveTypeBalance,
  useGetEmployeeTypeBalances,
  useListLeaveRequests,
} from '@swp/api-client/e6';
import { formatDate } from '@swp/shared/datetime';
import { useRouter } from 'expo-router';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useSession } from '../../src/providers/session';
import { Button } from '../../src/ui/Button';
import { Card } from '../../src/ui/Card';
import { Text } from '../../src/ui/Text';

type Tone = 'ok' | 'warn' | 'bad' | 'muted';
const toneBg: Record<Tone, string> = {
  ok: 'bg-ok-bg',
  warn: 'bg-warn-bg',
  bad: 'bg-bad-bg',
  muted: 'bg-surface-2',
};
const toneText: Record<Tone, string> = {
  ok: 'text-ok-text',
  warn: 'text-warn-text',
  bad: 'text-bad-text',
  muted: 'text-text-3',
};

// LeaveTypeBalance → right-hand pill {label, tone}. Mirrors the cap_basis taxonomy:
// ANNUAL_POOL / PER_YEAR_COUNT deplete (Sisa r/e); PER_EVENT / PER_MONTH show a per-instance
// cap (Maks n); UNCAPPED = "Sesuai ket."; SERVICE_UNPAID = conditional; LIFETIME_ONCE = used/unused.
function balanceBadge(b: LeaveTypeBalance, t: TFunction): { label: string; tone: Tone } {
  switch (b.cap_basis) {
    case 'UNCAPPED':
      return { label: t('m:leave.bal.uncapped'), tone: 'muted' };
    case 'SERVICE_UNPAID':
      return { label: t('m:leave.bal.conditional'), tone: 'warn' };
    case 'LIFETIME_ONCE':
      return b.used_days > 0
        ? { label: t('m:leave.bal.used', { n: b.used_days }), tone: 'muted' }
        : { label: t('m:leave.bal.unused'), tone: 'ok' };
  }
  if (b.entitled_days != null && b.remaining_days != null) {
    return {
      label: t('m:leave.bal.remaining', { r: b.remaining_days, e: b.entitled_days }),
      tone: b.remaining_days > 0 ? 'ok' : 'bad',
    };
  }
  if (b.cap_value != null) {
    const unit = b.cap_unit === 'COUNT' ? t('m:leave.bal.unitCount') : t('m:leave.bal.unitDays');
    return { label: t('m:leave.bal.max', { n: b.cap_value, unit }), tone: 'muted' };
  }
  return { label: '—', tone: 'muted' };
}

// Subtitle descriptor under the type name: "{code} · {cap descriptor}".
function capDescriptor(b: LeaveTypeBalance, t: TFunction): string {
  switch (b.cap_basis) {
    case 'ANNUAL_POOL':
      return b.expires_at
        ? t('m:leave.bal.expires', { date: formatDate(b.expires_at) })
        : t('m:leave.bal.annual');
    case 'PER_EVENT':
      return t('m:leave.bal.perEvent');
    case 'PER_MONTH':
      return t('m:leave.bal.perMonth');
    case 'PER_YEAR_COUNT':
      return t('m:leave.bal.perYear');
    case 'UNCAPPED':
      return t('m:leave.bal.uncappedSub');
    case 'LIFETIME_ONCE':
      return b.cap_value != null
        ? t('m:leave.bal.onceN', { n: b.cap_value })
        : t('m:leave.bal.once');
    case 'SERVICE_UNPAID':
      return t('m:leave.bal.unpaid');
    default:
      return b.code;
  }
}

function SectionHeader({ label }: { label: string }) {
  return (
    <Text variant="caption" className="px-6 pt-5 pb-2 uppercase tracking-wide text-text-3">
      {label}
    </Text>
  );
}

function BalanceRow({ item }: { item: LeaveTypeBalance }) {
  const { t } = useTranslation();
  const badge = balanceBadge(item, t);
  return (
    <Card>
      <View className="flex-row items-center justify-between gap-3">
        <View className="flex-1">
          <Text variant="body" weight="semibold">
            {item.name}
          </Text>
          <Text variant="caption" className="mt-0.5">
            {item.code} · {capDescriptor(item, t)}
          </Text>
        </View>
        <View className={`self-start rounded-pill px-2 py-1 ${toneBg[badge.tone]}`}>
          <Text variant="caption" weight="semibold" className={toneText[badge.tone]}>
            {badge.label}
          </Text>
        </View>
      </View>
    </Card>
  );
}

// Collapsed LeaveStatus (E11): DRAFT | PENDING | APPROVED | REJECTED | CANCELLED.
const requestTone: Record<string, Tone> = {
  APPROVED: 'ok',
  PENDING: 'warn',
  REJECTED: 'bad',
  DRAFT: 'muted',
  CANCELLED: 'muted',
};

function RequestRow({ item, onPress }: { item: LeaveRequest; onPress: () => void }) {
  const { t } = useTranslation();
  const tone = requestTone[item.status] ?? 'muted';
  return (
    <Pressable onPress={onPress}>
      <Card>
        <View className="flex-row items-center justify-between">
          <Text variant="body" weight="semibold">
            {item.leave_type_name ?? item.leave_type_id}
          </Text>
          <Text variant="caption" weight="semibold" className={toneText[tone]}>
            {t(`m:leave.status.${item.status}`)}
          </Text>
        </View>
        <Text variant="caption" className="mt-1">
          {item.start_date} → {item.end_date}
        </Text>
      </Card>
    </Pressable>
  );
}

export default function LeaveScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { user } = useSession();
  const employeeId = user?.employee_id ?? '';

  const balancesQ = useGetEmployeeTypeBalances(employeeId, {
    query: { enabled: !!employeeId },
  });
  const balancesBody = balancesQ.data?.data as { data?: LeaveTypeBalance[] } | undefined;
  const balances = balancesBody?.data ?? [];

  const requestsQ = useListLeaveRequests({ limit: 20 });
  const requestsBody = requestsQ.data?.data as { data?: LeaveRequest[] } | undefined;
  const requests = requestsBody?.data ?? [];

  const loading = balancesQ.isLoading || requestsQ.isLoading;
  const errored = balancesQ.isError || requestsQ.isError;

  return (
    <SafeAreaView className="flex-1 bg-app-bg">
      <View className="flex-row items-center justify-between px-6 py-4">
        <Text variant="title">{t('m:leave.title')}</Text>
        <Button label={t('m:leave.newBtn')} onPress={() => router.push('/leave-new')} />
      </View>

      {loading ? (
        <View className="items-center py-10">
          <ActivityIndicator />
        </View>
      ) : errored ? (
        <View className="px-6">
          <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
        </View>
      ) : (
        <ScrollView>
          <SectionHeader label={t('m:leave.balanceSection')} />
          {balances.length === 0 ? (
            <View className="px-6 pb-2">
              <Text variant="caption">{t('m:leave.balEmpty')}</Text>
            </View>
          ) : (
            <View className="gap-3 px-6">
              {balances.map((b) => (
                <BalanceRow key={b.leave_type_id} item={b} />
              ))}
            </View>
          )}

          <SectionHeader label={t('m:leave.historySection')} />
          {requests.length === 0 ? (
            <View className="px-6 pb-8">
              <Text variant="caption">{t('m:leave.empty')}</Text>
            </View>
          ) : (
            <View className="gap-3 px-6 pb-8">
              {requests.map((r) => (
                <RequestRow
                  key={r.id}
                  item={r}
                  onPress={() =>
                    router.push({
                      pathname: '/approval-status',
                      params: {
                        approval_instance_id: r.approval_instance_id ?? undefined,
                        request_type: 'LEAVE',
                        request_label: r.id,
                        request_title: r.leave_type_name ?? r.leave_type_id,
                      },
                    })
                  }
                />
              ))}
            </View>
          )}
        </ScrollView>
      )}
    </SafeAreaView>
  );
}
