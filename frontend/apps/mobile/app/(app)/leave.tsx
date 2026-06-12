import { type LeaveRequest, useListLeaveRequests } from '@swp/api-client/e6';
import { useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Button } from '../../src/ui/Button';
import { Card } from '../../src/ui/Card';
import { Text } from '../../src/ui/Text';

type Tone = 'ok' | 'warn' | 'bad' | 'muted';
const leaveTone: Record<string, Tone> = {
  APPROVED: 'ok',
  PENDING_L1: 'warn',
  PENDING_HR: 'warn',
  REJECTED: 'bad',
  DRAFT: 'muted',
  CANCELLED: 'muted',
};
const toneText: Record<Tone, string> = {
  ok: 'text-ok-text',
  warn: 'text-warn-text',
  bad: 'text-bad-text',
  muted: 'text-text-3',
};

function Row({ item }: { item: LeaveRequest }) {
  const { t } = useTranslation();
  const tone = leaveTone[item.status] ?? 'muted';
  return (
    <Card>
      <View className="flex-row items-center justify-between">
        <Text variant="body" className="font-semibold">
          {item.leave_type_name ?? item.leave_type_id}
        </Text>
        <Text className={`text-xs font-semibold ${toneText[tone]}`}>
          {t(`m:leave.status.${item.status}`)}
        </Text>
      </View>
      <Text variant="caption" className="mt-1">
        {item.start_date} → {item.end_date}
      </Text>
    </Card>
  );
}

export default function LeaveScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const q = useListLeaveRequests({ limit: 20 });
  const body = q.data?.data as { data?: LeaveRequest[] } | undefined;
  const items = body?.data ?? [];

  return (
    <SafeAreaView className="flex-1 bg-app-bg">
      <View className="flex-row items-center justify-between px-6 py-4">
        <Text variant="title">{t('m:leave.title')}</Text>
        <Button label={t('m:leave.newBtn')} onPress={() => router.push('/leave-new')} />
      </View>
      {q.isLoading ? (
        <View className="items-center py-10">
          <ActivityIndicator />
        </View>
      ) : q.isError ? (
        <View className="px-6">
          <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
        </View>
      ) : items.length === 0 ? (
        <View className="px-6 py-10">
          <Text variant="caption">{t('m:leave.empty')}</Text>
        </View>
      ) : (
        <ScrollView>
          <View className="gap-3 px-6 pb-8">
            {items.map((it) => (
              <Row key={it.id} item={it} />
            ))}
          </View>
        </ScrollView>
      )}
    </SafeAreaView>
  );
}
