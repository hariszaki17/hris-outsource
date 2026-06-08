import {
  type Overtime,
  useConfirmOvertime,
  useListOvertime,
  useWithdrawOvertime,
} from '@swp/api-client/e7';
import { useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Alert, Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Button } from '../src/ui/Button';
import { Card } from '../src/ui/Card';
import { Text } from '../src/ui/Text';

const tone: Record<string, string> = {
  APPROVED: 'text-ok-text',
  PENDING_AGENT_CONFIRM: 'text-warn-text',
  PENDING_L1: 'text-warn-text',
  PENDING_HR: 'text-warn-text',
  REJECTED: 'text-bad-text',
  WITHDRAWN: 'text-text-3',
};
const isPending = (s: string) =>
  s === 'PENDING_AGENT_CONFIRM' || s === 'PENDING_L1' || s === 'PENDING_HR';

export default function OvertimeScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const qc = useQueryClient();
  const q = useListOvertime({ limit: 20 });
  const confirm = useConfirmOvertime();
  const withdraw = useWithdrawOvertime();

  const items = (q.data?.data as { data?: Overtime[] } | undefined)?.data ?? [];
  const refresh = () => qc.invalidateQueries({ queryKey: ['/overtime'] });

  async function onConfirm(id: string) {
    try {
      await confirm.mutateAsync({ id, data: {} });
      await refresh();
      Alert.alert(t('m:overtime.title'), t('m:overtime.confirmed'));
    } catch {
      Alert.alert(t('m:overtime.title'), t('m:overtime.error'));
    }
  }
  async function onWithdraw(id: string) {
    try {
      await withdraw.mutateAsync({ id });
      await refresh();
      Alert.alert(t('m:overtime.title'), t('m:overtime.withdrawn'));
    } catch {
      Alert.alert(t('m:overtime.title'), t('m:overtime.error'));
    }
  }

  return (
    <SafeAreaView className="flex-1 bg-app-bg">
      <View className="flex-row items-center justify-between px-6 py-4">
        <Text variant="title">{t('m:overtime.title')}</Text>
        <Button label={t('m:overtime.newBtn')} onPress={() => router.push('/overtime-new')} />
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
          <Text variant="caption">{t('m:overtime.empty')}</Text>
        </View>
      ) : (
        <ScrollView>
          <View className="gap-3 px-6 pb-8">
            {items.map((it) => (
              <Card key={it.id}>
                <View className="flex-row items-center justify-between">
                  <Text variant="body" className="font-semibold">
                    {it.work_date}
                  </Text>
                  <Text className={`text-xs font-semibold ${tone[it.status] ?? 'text-text-3'}`}>
                    {t(`m:overtime.status.${it.status}`)}
                  </Text>
                </View>
                <View className="mt-2 flex-row gap-4">
                  {it.status === 'PENDING_AGENT_CONFIRM' ? (
                    <Pressable onPress={() => onConfirm(it.id)}>
                      <Text className="text-primary font-semibold">{t('m:overtime.confirm')}</Text>
                    </Pressable>
                  ) : null}
                  {isPending(it.status) ? (
                    <Pressable onPress={() => onWithdraw(it.id)}>
                      <Text className="text-danger font-semibold">{t('m:overtime.withdraw')}</Text>
                    </Pressable>
                  ) : null}
                </View>
              </Card>
            ))}
          </View>
        </ScrollView>
      )}
    </SafeAreaView>
  );
}
