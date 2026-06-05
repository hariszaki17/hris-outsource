-- E10 F10.1 notifications queries (SWP-NTF-*). The in-app notification surface:
-- cursor list (read-state + kind filters), single + bulk mark-read, unread count,
-- and the worker INSERT (called by the un-stubbed NotificationWorker / notify.Dispatch
-- in 11-02). id is allocated by the column DEFAULT 'SWP-NTF-' || swp_next_id('NTF')
-- (the INSERT omits id). Keyset cursor mirrors ListAuditLog: (created_at, id) DESC.

-- name: ListNotifications :many
-- Cursor page for one recipient, newest-first. read_state: 'ALL' (no-op),
-- 'UNREAD' (read_at IS NULL), 'READ' (read_at IS NOT NULL). kind optional.
SELECT id, recipient_id, kind, title, body,
       deep_link_epic, deep_link_entity_id, deep_link_path,
       actor_id, actor_label, is_critical, read_at, created_at
FROM notifications
WHERE recipient_id = sqlc.arg(recipient_id)
  AND (
        sqlc.narg(read_state)::text IS NULL
        OR sqlc.narg(read_state)::text = 'ALL'
        OR (sqlc.narg(read_state)::text = 'UNREAD' AND read_at IS NULL)
        OR (sqlc.narg(read_state)::text = 'READ'   AND read_at IS NOT NULL)
      )
  AND (sqlc.narg(kind)::text IS NULL OR kind = sqlc.narg(kind))
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetNotification :one
-- Single notification scoped to its recipient (scope=self).
SELECT id, recipient_id, kind, title, body,
       deep_link_epic, deep_link_entity_id, deep_link_path,
       actor_id, actor_label, is_critical, read_at, created_at
FROM notifications
WHERE id = sqlc.arg(id) AND recipient_id = sqlc.arg(recipient_id);

-- name: InsertNotification :one
-- The worker / dispatch-helper write path. id via the column DEFAULT.
INSERT INTO notifications (
    recipient_id, kind, title, body,
    deep_link_epic, deep_link_entity_id, deep_link_path,
    actor_id, actor_label, is_critical
) VALUES (
    sqlc.arg(recipient_id), sqlc.arg(kind), sqlc.arg(title), sqlc.arg(body),
    sqlc.narg(deep_link_epic), sqlc.narg(deep_link_entity_id), COALESCE(sqlc.narg(deep_link_path)::text, ''),
    sqlc.narg(actor_id), COALESCE(sqlc.narg(actor_label)::text, 'system'), sqlc.arg(is_critical)
)
RETURNING id, recipient_id, kind, title, body,
          deep_link_epic, deep_link_entity_id, deep_link_path,
          actor_id, actor_label, is_critical, read_at, created_at;

-- name: MarkNotificationRead :one
-- Mark one notification read (no-op if already read — COALESCE keeps the first
-- read_at). Scoped to recipient.
UPDATE notifications
SET read_at = COALESCE(read_at, now())
WHERE id = sqlc.arg(id) AND recipient_id = sqlc.arg(recipient_id)
RETURNING id, recipient_id, kind, title, body,
          deep_link_epic, deep_link_entity_id, deep_link_path,
          actor_id, actor_label, is_critical, read_at, created_at;

-- name: MarkAllNotificationsRead :execrows
-- Bulk mark-read for a recipient. Optional @before cutoff (created_at < before).
-- Returns the affected row count.
UPDATE notifications
SET read_at = now()
WHERE recipient_id = sqlc.arg(recipient_id)
  AND read_at IS NULL
  AND (sqlc.narg(before)::timestamptz IS NULL OR created_at < sqlc.narg(before)::timestamptz);

-- name: CountUnreadNotifications :one
-- Unread badge + AgentDashboard.recent_notifications_unread.
SELECT count(*) AS unread
FROM notifications
WHERE recipient_id = sqlc.arg(recipient_id) AND read_at IS NULL;
