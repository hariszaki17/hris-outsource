import {
  type ListNotifications200,
  type Notification,
  useListNotifications,
  useMarkAllNotificationsRead,
  useMarkNotificationRead,
} from '@swp/api-client/e10';
import { formatInstant } from '@swp/shared/datetime';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, View } from 'react-native';
import { Card } from '../../src/ui/Card';
import { Text } from '../../src/ui/Text';

function Row({ item, onPress }: { item: Notification; onPress: () => void }) {
  const unread = !item.read_at;
  return (
    <Pressable onPress={onPress}>
      <Card className={unread ? 'border-primary-soft' : ''}>
        <View className="flex-row items-center gap-2">
          {unread ? <View className="h-2 w-2 rounded-pill bg-primary" /> : null}
          <Text variant="body" className="flex-1 font-semibold">
            {item.title}
          </Text>
        </View>
        <Text variant="caption" className="mt-1">
          {item.body}
        </Text>
        <Text variant="caption" className="mt-2 text-text-3">
          {formatInstant(item.created_at)}
        </Text>
      </Card>
    </Pressable>
  );
}

export default function Notifications() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const list = useListNotifications();
  const markAll = useMarkAllNotificationsRead();
  const markOne = useMarkNotificationRead();

  // CursorPage shape: the notifications array is the page's `data` field.
  const body = list.data?.data as ListNotifications200 | undefined;
  const items: Notification[] = body?.data ?? [];
  const unread = items.filter((n) => !n.read_at).length;

  async function refresh() {
    await qc.invalidateQueries({ queryKey: ['/notifications'] });
  }
  async function onMarkAll() {
    await markAll.mutateAsync({ data: {} });
    await refresh();
  }
  async function onMarkOne(id: string) {
    await markOne.mutateAsync({ notificationId: id });
    await refresh();
  }

  return (
    <View className="flex-1 bg-app-bg">
      <View className="flex-row items-center justify-between px-6 py-4">
        <Text variant="caption">{t('m:notifications.unread', { count: unread })}</Text>
        {unread > 0 ? (
          <Pressable onPress={onMarkAll} disabled={markAll.isPending}>
            <Text className="text-primary font-semibold">{t('m:notifications.markAllRead')}</Text>
          </Pressable>
        ) : null}
      </View>

      {list.isLoading ? (
        <View className="items-center py-10">
          <ActivityIndicator />
        </View>
      ) : list.isError ? (
        <View className="px-6">
          <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
        </View>
      ) : items.length === 0 ? (
        <View className="px-6 py-10">
          <Text variant="caption">{t('m:notifications.empty')}</Text>
        </View>
      ) : (
        <ScrollView>
          <View className="gap-3 px-6 pb-8">
            {items.map((n) => (
              <Row key={n.id} item={n} onPress={() => void onMarkOne(n.id)} />
            ))}
          </View>
        </ScrollView>
      )}
    </View>
  );
}
