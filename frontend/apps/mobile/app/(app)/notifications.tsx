import {
  type ListNotifications200,
  type Notification,
  useListNotifications,
  useMarkAllNotificationsRead,
  useMarkNotificationRead,
} from '@swp/api-client/e10';
import { formatInstant } from '@swp/shared/datetime';
import { useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Card } from '../../src/ui/Card';
import { Text } from '../../src/ui/Text';
import { usePullToRefresh } from '../../src/ui/usePullToRefresh';

// ── Deep-link routing ──────────────────────────────────────────────────────────
//
// Notifications carry a `deep_link: { epic, entity_id, path }` field.
// This function maps the deep_link to an Expo Router push call when a
// notification is tapped. Currently handled:
//
//   E8 · path starts with /clarification-answer  → clarification-answer screen
//        (F8.5 · PC-13; entity_id = SWP-CLR-xxx)
//
// All other deep-links are not yet routed in mobile v1 (no-op; mark-read only).
//
// NOTE: `question` is passed as a URL-encoded query param sourced from the
// notification body because GET /clarifications/{id} does not exist in v1
// (CONTRACT GAP). The body text is the generic backend copy; if needed, HR
// could be asked to surface the question text via a future API addition.

function resolveDeepLink(
  n: Notification,
): { pathname: string; params: Record<string, string> } | null {
  const { epic, path, entity_id } = n.deep_link;

  if (epic === 'E8' && path.startsWith('/clarification-answer')) {
    // entity_id = SWP-CLR-xxx (clarification ID — set by backend on create).
    const id = entity_id ?? '';
    if (!id) return null;
    return {
      pathname: '/clarification-answer',
      params: {
        id,
        // Body text is the closest we have to a question until a GET endpoint exists.
        question: encodeURIComponent(n.body),
      },
    };
  }

  return null;
}

// ── Row ────────────────────────────────────────────────────────────────────────

function Row({ item, onPress }: { item: Notification; onPress: () => void }) {
  const unread = !item.read_at;
  const hasDeepLink = resolveDeepLink(item) !== null;
  return (
    <Pressable onPress={onPress}>
      <Card className={unread ? 'border-primary-soft' : ''}>
        <View className="flex-row items-center gap-2">
          {unread ? <View className="h-2 w-2 rounded-pill bg-primary" /> : null}
          <Text variant="body" weight="semibold" className="flex-1">
            {item.title}
          </Text>
          {hasDeepLink ? (
            <View className="rounded-pill bg-primary px-2 py-0.5">
              <Text variant="badge" className="text-white">
                {'→'}
              </Text>
            </View>
          ) : null}
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

// ── Screen ─────────────────────────────────────────────────────────────────────

export default function Notifications() {
  const { t } = useTranslation();
  const insets = useSafeAreaInsets();
  const router = useRouter();
  const qc = useQueryClient();
  const list = useListNotifications();
  const markAll = useMarkAllNotificationsRead();
  const markOne = useMarkNotificationRead();
  const refreshControl = usePullToRefresh();

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

  async function onTap(n: Notification) {
    // Always mark as read first.
    await markOne.mutateAsync({ notificationId: n.id });
    await refresh();
    // Then navigate to the target screen if a deep-link is mapped.
    const target = resolveDeepLink(n);
    if (target) {
      router.push({ pathname: target.pathname as never, params: target.params });
    }
  }

  return (
    <View className="flex-1 bg-app-bg">
      <View
        className="flex-row items-center justify-between px-6 py-4"
        style={{ paddingTop: insets.top + 8 }}
      >
        <Text variant="caption">{t('m:notifications.unread', { count: unread })}</Text>
        {unread > 0 ? (
          <Pressable onPress={onMarkAll} disabled={markAll.isPending}>
            <Text variant="body" weight="semibold" className="text-primary">
              {t('m:notifications.markAllRead')}
            </Text>
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
        <ScrollView refreshControl={refreshControl}>
          <View className="gap-3 px-6 pb-8">
            {items.map((n) => (
              <Row key={n.id} item={n} onPress={() => void onTap(n)} />
            ))}
          </View>
        </ScrollView>
      )}
    </View>
  );
}
