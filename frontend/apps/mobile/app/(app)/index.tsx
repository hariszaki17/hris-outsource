import { type AgentDashboard, type Dashboard, useGetMyDashboard } from '@swp/api-client/e10';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';
import { useSession } from '../../src/providers/session';
import { Card } from '../../src/ui/Card';
import { Text } from '../../src/ui/Text';

function AgentSummary({ data }: { data: AgentDashboard }) {
  const { t } = useTranslation();
  return (
    <View className="gap-4">
      <Card>
        <Text variant="caption">{t('m:beranda.todayShift')}</Text>
        <Text variant="body">{data.today_shift ? '—' : t('m:beranda.offToday')}</Text>
      </Card>
      <Card>
        <Text variant="caption">{t('m:beranda.otMonth')}</Text>
        <Text variant="body">{t('m:beranda.hours', { count: data.ot_this_month_hours })}</Text>
      </Card>
      <Card>
        <Text variant="caption">{t('m:beranda.unreadNotifs')}</Text>
        <Text variant="body">{data.recent_notifications_unread}</Text>
      </Card>
    </View>
  );
}

export default function Beranda() {
  const { t } = useTranslation();
  const { user } = useSession();
  const dash = useGetMyDashboard();
  // Non-2xx throws; on success .data.data is the role-shaped Dashboard body.
  const payload = dash.data?.data as Dashboard | undefined;

  return (
    <ScrollView className="flex-1 bg-app-bg">
      <View className="gap-4 p-6">
        <Text variant="title">{t('m:beranda.greeting', { name: user?.full_name ?? '' })}</Text>

        {dash.isLoading ? (
          <View className="items-center py-10">
            <ActivityIndicator />
          </View>
        ) : dash.isError ? (
          <Card>
            <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
          </Card>
        ) : payload && payload.role === 'agent' ? (
          <AgentSummary data={payload} />
        ) : (
          <Card>
            <Text variant="caption">{t('m:common.emptyGeneric')}</Text>
          </Card>
        )}
      </View>
    </ScrollView>
  );
}
