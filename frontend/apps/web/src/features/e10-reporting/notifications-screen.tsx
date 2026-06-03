/**
 * E10 · Pusat Notifikasi (Notification Center).
 *
 * .pen frames implemented:
 *   i0qW8   E10 · Pusat Notifikasi (HR)           — main list, filters, mark-all-read, date groups
 *   P2CO7C  State · Web · Empty                    — zero-notification empty state
 *   R0d1wC  State · Mark-as-Read transition        — CQBqd→zTbmw + Toast `ofb0U`
 *   (stale-deep-link graceful inline note covered in NotifCard onClick handler)
 *
 * Route: /notifications
 * Search params: read_state (UNREAD|READ|ALL), kind (NotificationKind), cursor
 *
 * Mutation var shapes (from generated hooks):
 *   useMarkNotificationRead     → { notificationId: string }
 *   useMarkNotificationUnread   → { notificationId: string }
 *   useMarkAllNotificationsRead → { data: MarkAllNotificationsReadBody }
 *
 * F10.1 refs: NT-1, NT-2, NT-3, NT-4, NT-5, NT-6, C-1..C-4
 */

import { classifyError } from '@/lib/api-error.ts';
import {
  type ListNotificationsParams,
  ListNotificationsReadState,
  type MarkAllNotificationsReadBody,
  type Notification,
  NotificationKind,
  getGetNotificationCountQueryKey,
  getListNotificationsQueryKey,
  useListNotifications,
  useMarkAllNotificationsRead,
  useMarkNotificationRead,
} from '@swp/api-client/e10';
import {
  Button,
  CursorPagination,
  EmptyState,
  FilterSelect,
  NotifCard,
  Skeleton,
  StateView,
  useToast,
} from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { RotateCcw } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { notifKindIcon } from './e10-shared.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type NotificationsSearch = {
  read_state?: ListNotificationsReadState;
  kind?: NotificationKind;
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

// ---------------------------------------------------------------------------
// Relative time helper
// Follow-up: replace with a proper relative-time util / DateText when available.
// For now produces a fixed absolute representation in Asia/Jakarta. The .pen
// uses "2 jam lalu" strings that are already relative; this helper formats the
// ISO string into a short Jakarta-tz absolute label (e.g. "17 Jun 14:32") which
// is always correct and never stale. A true relative formatter is a v1.1 follow-up.
// ---------------------------------------------------------------------------

function formatJakartaTime(iso: string): string {
  try {
    return new Intl.DateTimeFormat('id-ID', {
      timeZone: 'Asia/Jakarta',
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

// ---------------------------------------------------------------------------
// Group notifications by date label (HARI INI / KEMARIN / older date)
// ---------------------------------------------------------------------------

type DateGroup = { label: string; items: Notification[] };

function groupByDate(
  rows: Notification[],
  labelToday: string,
  labelYesterday: string,
): DateGroup[] {
  const groups: Map<string, Notification[]> = new Map();
  const nowJkt = new Date(new Date().toLocaleString('en-US', { timeZone: 'Asia/Jakarta' }));
  const todayKey = `${nowJkt.getFullYear()}-${nowJkt.getMonth()}-${nowJkt.getDate()}`;
  const yestDate = new Date(nowJkt);
  yestDate.setDate(yestDate.getDate() - 1);
  const yestKey = `${yestDate.getFullYear()}-${yestDate.getMonth()}-${yestDate.getDate()}`;

  for (const n of rows) {
    const d = new Date(
      new Date(n.created_at).toLocaleString('en-US', { timeZone: 'Asia/Jakarta' }),
    );
    const key = `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
    let label: string;
    if (key === todayKey) {
      label = labelToday;
    } else if (key === yestKey) {
      label = labelYesterday;
    } else {
      label = new Intl.DateTimeFormat('id-ID', {
        timeZone: 'Asia/Jakarta',
        day: 'numeric',
        month: 'long',
        year: 'numeric',
      }).format(d);
    }
    if (!groups.has(label)) groups.set(label, []);
    groups.get(label)!.push(n);
  }

  return Array.from(groups.entries()).map(([label, items]) => ({ label, items }));
}

// ---------------------------------------------------------------------------
// Inner component
// ---------------------------------------------------------------------------

interface NotificationsScreenInnerProps {
  search: NotificationsSearch;
  onSearch: (patch: NotificationsSearch) => void;
  prevCursors: string[];
  onPrevCursorsChange: (c: string[]) => void;
}

function NotificationsScreenInner({
  search,
  onSearch,
  prevCursors,
  onPrevCursorsChange,
}: NotificationsScreenInnerProps) {
  const { t } = useTranslation('notifications');
  const { toast } = useToast();
  const navigate = useNavigate();
  const qc = useQueryClient();

  // Optimistic local override: IDs currently being marked read (card-level)
  const [pendingReadIds, setPendingReadIds] = useState<Set<string>>(new Set());

  const params: ListNotificationsParams = {
    limit: PAGE_SIZE,
    cursor: search.cursor,
    read_state: search.read_state ?? ListNotificationsReadState.ALL,
    kind: search.kind,
  };

  const query = useListNotifications(params);

  const markRead = useMarkNotificationRead({
    mutation: {
      onSuccess: (_data, { notificationId }) => {
        setPendingReadIds((prev) => {
          const next = new Set(prev);
          next.delete(notificationId);
          return next;
        });
        void qc.invalidateQueries({ queryKey: getListNotificationsQueryKey() });
        void qc.invalidateQueries({ queryKey: getGetNotificationCountQueryKey() });
      },
      onError: (_err, { notificationId }) => {
        setPendingReadIds((prev) => {
          const next = new Set(prev);
          next.delete(notificationId);
          return next;
        });
        toast({ tone: 'error', title: t('errors.markReadFailed') });
      },
    },
  });

  const markAllRead = useMarkAllNotificationsRead({
    mutation: {
      onSuccess: (res) => {
        const affected = (res.data as { marked?: number } | undefined)?.marked ?? 0;
        toast({
          tone: 'success',
          title: t('markAllSuccess', { count: affected }),
        });
        void qc.invalidateQueries({ queryKey: getListNotificationsQueryKey() });
        void qc.invalidateQueries({ queryKey: getGetNotificationCountQueryKey() });
      },
      onError: () => {
        toast({ tone: 'error', title: t('errors.markAllFailed') });
      },
    },
  });

  // ---------------------------------------------------------------------------
  // Filter helpers
  // ---------------------------------------------------------------------------

  const hasFilters = Boolean(search.read_state || search.kind);

  function setSearch(patch: NotificationsSearch) {
    onSearch({ ...search, cursor: undefined, ...patch });
    onPrevCursorsChange([]);
  }

  function resetFilters() {
    setSearch({ read_state: undefined, kind: undefined, cursor: undefined });
  }

  // ---------------------------------------------------------------------------
  // Error state
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <div className="flex flex-col gap-[18px]">
          <TitleBand unreadCount={0} onMarkAll={() => {}} markAllPending={false} />
          <EmptyState
            variant="no-permission"
            title={t('errors.forbidden')}
            description={t('errors.forbiddenBody')}
          />
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-[18px]">
        <TitleBand unreadCount={0} onMarkAll={() => {}} markAllPending={false} />
        <StateView
          kind="error"
          title={t('errors.loadTitle')}
          description={t('errors.network')}
          onRetry={() => query.refetch()}
          retryLabel={t('common.retry')}
        />
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Data
  // ---------------------------------------------------------------------------

  const page = query.data?.data as
    | { data?: Notification[]; has_more?: boolean; next_cursor?: string | null }
    | undefined;
  const rows: Notification[] = page?.data ?? [];

  const unreadCount = rows.filter((n) => n.read_at === null).length;

  // Apply optimistic-unread override for in-flight marks
  function isUnread(n: Notification): boolean {
    if (pendingReadIds.has(n.id)) return false; // optimistic read
    return n.read_at === null;
  }

  const groups = groupByDate(rows, t('dateGroup.today'), t('dateGroup.yesterday'));

  // ---------------------------------------------------------------------------
  // Card click: mark read + navigate
  // ---------------------------------------------------------------------------

  function handleCardClick(n: Notification) {
    if (n.read_at === null && !pendingReadIds.has(n.id)) {
      setPendingReadIds((prev) => new Set([...prev, n.id]));
      markRead.mutate({ notificationId: n.id });
    }
    if (n.deep_link?.path) {
      // Graceful stale-deep-link: navigate, the target screen handles 404 inline.
      // NT-C4: if the target no longer exists the screen shows a "sudah tidak tersedia" note.
      void navigate({ to: n.deep_link.path as never }).catch(() => {
        // Path not registered in router → show inline note via toast (stale deep-link C-4)
        toast({ tone: 'error', title: t('errors.staleDeepLink') });
      });
    }
  }

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  function handleMarkAll() {
    const body: MarkAllNotificationsReadBody = {};
    markAllRead.mutate({ data: body });
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band */}
      <TitleBand
        unreadCount={unreadCount}
        onMarkAll={handleMarkAll}
        markAllPending={markAllRead.isPending}
      />

      {/* Filters — segmented read-state pills + kind select (from .pen `y7TVBK` Seg row) */}
      <div className="flex flex-wrap items-center gap-2.5">
        {/* Read-state segmented control (pill style from .pen) */}
        <div className="flex items-center gap-2">
          {(
            [
              { value: undefined, label: t('filter.all') },
              { value: ListNotificationsReadState.UNREAD, label: t('filter.unread') },
              { value: ListNotificationsReadState.READ, label: t('filter.read') },
            ] as { value: ListNotificationsReadState | undefined; label: string }[]
          ).map(({ value, label }) => {
            const active = (search.read_state ?? undefined) === value;
            return (
              <button
                key={value ?? 'all'}
                type="button"
                onClick={() => setSearch({ read_state: value })}
                className={
                  active
                    ? 'rounded-full border border-text bg-text px-3.5 py-[7px] text-[13px] font-semibold text-bg'
                    : 'rounded-full border border-border bg-surface px-3.5 py-[7px] text-[13px] font-medium text-text-2 transition-colors hover:bg-surface-2'
                }
              >
                {label}
              </button>
            );
          })}
        </div>

        {/* Kind filter */}
        <FilterSelect
          aria-label={t('filter.kindLabel')}
          value={search.kind ?? ''}
          onChange={(e) => setSearch({ kind: (e.target.value as NotificationKind) || undefined })}
        >
          <option value="">{t('filter.kindLabel')}</option>
          {/* Grouped by epic for readability */}
          <optgroup label={t('filter.groupSchedule')}>
            <option value={NotificationKind.SCHEDULE_PUBLISHED}>
              {t('kind.SCHEDULE_PUBLISHED')}
            </option>
            <option value={NotificationKind.SCHEDULE_CHANGED}>{t('kind.SCHEDULE_CHANGED')}</option>
            <option value={NotificationKind.SHIFT_REMINDER}>{t('kind.SHIFT_REMINDER')}</option>
          </optgroup>
          <optgroup label={t('filter.groupLeave')}>
            <option value={NotificationKind.LEAVE_REQUEST_SUBMITTED}>
              {t('kind.LEAVE_REQUEST_SUBMITTED')}
            </option>
            <option value={NotificationKind.LEAVE_APPROVED}>{t('kind.LEAVE_APPROVED')}</option>
            <option value={NotificationKind.LEAVE_REJECTED}>{t('kind.LEAVE_REJECTED')}</option>
          </optgroup>
          <optgroup label={t('filter.groupOT')}>
            <option value={NotificationKind.OT_REQUEST_SUBMITTED}>
              {t('kind.OT_REQUEST_SUBMITTED')}
            </option>
            <option value={NotificationKind.OT_AUTO_DETECTED}>{t('kind.OT_AUTO_DETECTED')}</option>
            <option value={NotificationKind.OT_APPROVED}>{t('kind.OT_APPROVED')}</option>
            <option value={NotificationKind.OT_REJECTED}>{t('kind.OT_REJECTED')}</option>
          </optgroup>
          <optgroup label={t('filter.groupAttendance')}>
            <option value={NotificationKind.ATTENDANCE_VERIFY_NEEDED}>
              {t('kind.ATTENDANCE_VERIFY_NEEDED')}
            </option>
            <option value={NotificationKind.ATTENDANCE_CORRECTION_SUBMITTED}>
              {t('kind.ATTENDANCE_CORRECTION_SUBMITTED')}
            </option>
            <option value={NotificationKind.ATTENDANCE_AUTO_CLOSED}>
              {t('kind.ATTENDANCE_AUTO_CLOSED')}
            </option>
          </optgroup>
          <optgroup label={t('filter.groupHR')}>
            <option value={NotificationKind.HR_CHANGE_REQUEST_SUBMITTED}>
              {t('kind.HR_CHANGE_REQUEST_SUBMITTED')}
            </option>
            <option value={NotificationKind.AGREEMENT_EXPIRING}>
              {t('kind.AGREEMENT_EXPIRING')}
            </option>
            <option value={NotificationKind.PLACEMENT_EXPIRING}>
              {t('kind.PLACEMENT_EXPIRING')}
            </option>
            <option value={NotificationKind.PLACEMENT_LEADER_CHANGED}>
              {t('kind.PLACEMENT_LEADER_CHANGED')}
            </option>
          </optgroup>
          <optgroup label={t('filter.groupExport')}>
            <option value={NotificationKind.EXPORT_READY}>{t('kind.EXPORT_READY')}</option>
            <option value={NotificationKind.EXPORT_FAILED}>{t('kind.EXPORT_FAILED')}</option>
          </optgroup>
        </FilterSelect>

        {hasFilters && (
          <>
            <div className="h-6 w-px bg-border" />
            <Button type="button" variant="ghost" onClick={resetFilters}>
              <RotateCcw aria-hidden className="size-3.5" />
              {t('filter.reset')}
            </Button>
          </>
        )}
      </div>

      {/* Loading skeleton */}
      {query.isLoading && (
        <div className="flex flex-col gap-3">
          {Array.from({ length: 6 }).map((_, i) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: skeleton rows
            <Skeleton key={i} className="h-20 w-full rounded-xl" />
          ))}
        </div>
      )}

      {/* Empty states */}
      {!query.isLoading &&
        rows.length === 0 &&
        (hasFilters ? (
          <EmptyState
            variant="filtered"
            title={t('empty.filteredTitle')}
            description={t('empty.filteredBody')}
          />
        ) : (
          /* P2CO7C — Belum ada notifikasi */
          <EmptyState variant="fresh" title={t('empty.title')} description={t('empty.body')} />
        ))}

      {/* Notification list — grouped by date (HARI INI / KEMARIN / ...) */}
      {!query.isLoading && rows.length > 0 && (
        <div className="flex flex-col gap-6">
          {groups.map((group) => (
            <section key={group.label} aria-label={group.label}>
              {/* Date label — matches .pen `cM3U8` text style */}
              <p className="mb-3 text-[12px] font-bold uppercase tracking-[0.4px] text-text-3">
                {group.label}
              </p>

              {/* Card list — rounded container matching .pen `LZn66`/`nqQJU` */}
              <div className="overflow-hidden rounded-xl border border-border bg-surface">
                {group.items.map((n, idx) => {
                  const Icon = notifKindIcon(n.kind);
                  const unread = isUnread(n);
                  return (
                    <div
                      key={n.id}
                      className={
                        idx < group.items.length - 1 ? 'border-b border-border-soft' : undefined
                      }
                    >
                      <NotifCard
                        icon={Icon}
                        title={n.title}
                        body={n.body}
                        time={formatJakartaTime(n.created_at)}
                        unread={unread}
                        onClick={() => handleCardClick(n)}
                      />
                    </div>
                  );
                })}
              </div>
            </section>
          ))}
        </div>
      )}

      {/* Pagination */}
      {rows.length > 0 && (
        <CursorPagination
          rangeLabel={t('pagination.range', { count: rows.length })}
          hasPrev={prevCursors.length > 0}
          hasNext={Boolean(page?.has_more)}
          prevLabel={t('common.prev')}
          nextLabel={t('common.next')}
          onPrev={() => {
            const next = [...prevCursors];
            const cursor = next.pop();
            onPrevCursorsChange(next);
            onSearch({ ...search, cursor: cursor || undefined });
          }}
          onNext={() => {
            const nextCursor = page?.next_cursor;
            if (!nextCursor) return;
            onPrevCursorsChange([...prevCursors, search.cursor ?? '']);
            onSearch({ ...search, cursor: nextCursor });
          }}
        />
      )}

      {/* D10 preferences stub — matches .pen `F0x8S` (absolute-positioned disabled affordance) */}
      <div className="mt-2 flex items-center gap-1.5 rounded-lg border border-border bg-surface-2 px-2.5 py-1.5 opacity-60 w-fit">
        <span className="text-[11px] font-semibold text-text-3">{t('prefsStub')}</span>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// TitleBand
// ---------------------------------------------------------------------------

interface TitleBandProps {
  unreadCount: number;
  onMarkAll: () => void;
  markAllPending: boolean;
}

function TitleBand({ unreadCount, onMarkAll, markAllPending }: TitleBandProps) {
  const { t } = useTranslation('notifications');

  return (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-bold text-text">{t('title')}</h1>
        <p className="text-sm text-text-3">{t('subtitle')}</p>
      </div>

      {/* "Tandai semua dibaca" — .pen `B9XPDU`; hidden when unreadCount === 0 (P2CO7C) */}
      {unreadCount > 0 && (
        <Button type="button" variant="secondary" onClick={onMarkAll} disabled={markAllPending}>
          {markAllPending ? t('markAllPending') : t('markAll')}
        </Button>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public export
// ---------------------------------------------------------------------------

export function NotificationsScreen() {
  const [search, setSearch] = useState<NotificationsSearch>({});
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  return (
    <NotificationsScreenInner
      search={search}
      onSearch={(patch) => setSearch(patch)}
      prevCursors={prevCursors}
      onPrevCursorsChange={setPrevCursors}
    />
  );
}
