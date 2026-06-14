import { type Payslip, useListPayslips } from '@swp/api-client/e8';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Card } from '../src/ui/Card';
import { Text } from '../src/ui/Text';

export default function PayslipScreen() {
  const { t } = useTranslation();
  const q = useListPayslips({ limit: 24 });
  const items = (q.data?.data as { data?: Payslip[] } | undefined)?.data ?? [];

  return (
    <SafeAreaView className="flex-1 bg-app-bg">
      <View className="px-6 py-4">
        <Text variant="title">{t('m:payslip.title')}</Text>
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
          <Text variant="caption">{t('m:payslip.empty')}</Text>
        </View>
      ) : (
        <ScrollView>
          <View className="gap-3 px-6 pb-8">
            {items.map((p) => (
              <Card key={p.id}>
                <Text variant="strong">{p.period}</Text>
                <View className="mt-2 flex-row justify-between">
                  <Text variant="caption">{t('m:payslip.takeHome')}</Text>
                  <Text variant="strong">{p.take_home_pay ?? '—'}</Text>
                </View>
                <View className="flex-row justify-between">
                  <Text variant="caption">{t('m:payslip.gross')}</Text>
                  <Text variant="caption">{p.gross_earnings ?? '—'}</Text>
                </View>
                <Text variant="caption" className="mt-2 text-text-3">
                  {p.paid_on ? `${t('m:payslip.paid')} ${p.paid_on}` : t('m:payslip.notPaid')}
                </Text>
              </Card>
            ))}
          </View>
        </ScrollView>
      )}
    </SafeAreaView>
  );
}
