/**
 * /me/notifications — Agent notification inbox.
 *
 * Web port of apps/mobile/app/(app)/notifications.tsx.
 * Hooks: useListNotifications (cursor-paged), useMarkNotificationRead (POST :mark-read),
 * useMarkAllNotificationsRead (POST :mark-all-read).
 * Unread rows (read_at == null) styled distinctly via NotifCard `unread` prop; clicking
 * marks the notification read. "Mark all read" sits in the AgentPage `actions` header slot.
 *
 * Layout: a single rounded panel (border bg-surface) with divide-y rows — NOT separate
 * bordered mobile cards with gaps. Full-width console design system per AgentPage.
 *
 * F10.1 refs: NT-1, NT-2, NT-4, NT-5, NT-6.
 */
import {
  type ListNotifications200,
  type Notification,
  useListNotifications,
  useMarkAllNotificationsRead,
  useMarkNotificationRead,
} from '@swp/api-client/e10';
import { formatInstant } from '@swp/shared';
import { Button, EmptyState, NotifCard, StateView, useToast } from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { notifKindIcon } from '../e10-reporting/e10-shared.tsx';
import { AgentPage } from './agent-page.tsx';

export function AgentNotificationsScreen() {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const qc = useQueryClient();

  const list = useListNotifications();
  const markOne = useMarkNotificationRead();
  const markAll = useMarkAllNotificationsRead();

  // CursorPage shape: the notifications array is the page's `data` field.
  const body = list.data?.data as ListNotifications200 | undefined;
  const items: Notification[] = body?.data ?? [];
  const unreadCount = items.filter((n) => !n.read_at).length;

  async function refresh() {
    await qc.invalidateQueries({ queryKey: ['/notifications'] });
  }

  async function onMarkOne(id: string) {
    try {
      await markOne.mutateAsync({ notificationId: id });
      await refresh();
    } catch {
      toast({ tone: 'error', title: t('errorGeneric') });
    }
  }

  async function onMarkAll() {
    try {
      await markAll.mutateAsync({ data: {} });
      await refresh();
      toast({ tone: 'success', title: t('notifMarkAllRead') });
    } catch {
      toast({ tone: 'error', title: t('errorGeneric') });
    }
  }

  const headerActions =
    unreadCount > 0 ? (
      <Button variant="ghost" disabled={markAll.isPending} onClick={() => void onMarkAll()}>
        {t('notifMarkAllRead')}
      </Button>
    ) : undefined;

  return (
    <AgentPage title={t('notifTitle')} actions={headerActions}>
      {/* Unread summary — only shown when there are unread items */}
      {unreadCount > 0 && (
        <p className="text-[13px] text-text-2">{t('notifUnread', { count: unreadCount })}</p>
      )}

      {list.isLoading ? (
        <StateView kind="loading" title={t('loading')} />
      ) : list.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void list.refetch()} />
      ) : items.length === 0 ? (
        <EmptyState
          variant="fresh"
          title={t('notifEmpty', { defaultValue: 'Tidak ada notifikasi' })}
        />
      ) : (
        /* Single panel — cards strip their own outer border via className; the panel
           provides the shared border + bg, and divide-y supplies per-row separators. */
        <div className="overflow-hidden rounded-xl border border-border bg-surface divide-y divide-border-soft">
          {items.map((n) => (
            <NotifCard
              key={n.id}
              icon={notifKindIcon(n.kind)}
              title={n.title}
              body={n.body}
              time={formatInstant(n.created_at, { dateStyle: 'medium', timeStyle: 'short' })}
              unread={!n.read_at}
              onClick={n.read_at ? undefined : () => void onMarkOne(n.id)}
              /* Strip the card's own outer border so the panel border + divide-y govern
                 all edges. Unread cards keep their left-4 primary accent. */
              className={
                n.read_at
                  ? 'rounded-none border-0'
                  : 'rounded-none border-0 border-l-4 border-primary'
              }
            />
          ))}
        </div>
      )}
    </AgentPage>
  );
}
