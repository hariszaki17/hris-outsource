-- +goose Up
-- In-app notifications (E10 F10.1 / SWP-NTF-*): one durable in-app notification
-- row per recipient. This is the loop-closer table — every prior epic's
-- TODO(Phase-11) dispatch point (leave/OT/attendance/change-request/placement)
-- will INSERT a row here via the reusable notify.Dispatch helper + the un-stubbed
-- NotificationWorker (11-02). Push/email are a separate side-channel; this row is
-- the durable record (openapi NT-6).
--
-- IDs allocated via the column DEFAULT 'SWP-NTF-' || swp_next_id('NTF')
-- (Phase-5 column-DEFAULT allocation, decision [05-01]): INSERTs omit id to let
-- DEFAULT fire. The NTF prefix already exists in internal/platform/ids/ids.go —
-- ids.go is NOT touched by this phase.
--
-- The row maps 1:1 onto openapi components.schemas.Notification:
--   { id, kind(NotificationKind), title, body, read_at(nullable), created_at,
--     deep_link:{epic,entity_id,path}, actor:{id(nullable),label}, is_critical }
-- deep_link.* and actor.* are flattened onto columns; the service re-nests them
-- at the DTO boundary (11-02). kind has NO CHECK constraint — forward-compat with
-- new NotificationKind values without a migration (the app owns the enum).
CREATE TABLE notifications (
    id                  text PRIMARY KEY DEFAULT ('SWP-NTF-' || swp_next_id('NTF')),
    recipient_id        text NOT NULL,                          -- SWP-USR-* or SWP-EMP-* (the dispatch helper resolves the recipient)
    kind                text NOT NULL,                          -- NotificationKind enum string (no CHECK — forward-compat)
    title               text NOT NULL,
    body                text NOT NULL,

    -- deep_link.* (openapi DeepLink). epic 'E2'..'E8'; entity_id the target
    -- prefixed id; path the client route. epic + entity_id nullable (some notifs
    -- have no entity), path defaults '' (always present on the wire).
    deep_link_epic      text,                                   -- 'E2'..'E8' (nullable)
    deep_link_entity_id text,                                   -- SWP-LR-* etc (nullable)
    deep_link_path      text NOT NULL DEFAULT '',               -- client route path

    -- actor.* (openapi Notification.actor). actor_id null = system actor.
    actor_id            text,                                   -- SWP-USR-* (nullable = system)
    actor_label         text NOT NULL DEFAULT 'system',

    is_critical         boolean NOT NULL DEFAULT false,         -- critical categories stay on regardless of mute prefs (NT-5)
    read_at             timestamptz,                            -- null = unread
    created_at          timestamptz NOT NULL DEFAULT now()
);

-- Cursor list load: per-recipient, newest-first via the keyset cursor
-- (created_at DESC, id DESC) — mirrors the audit-log / leave-request keyset.
CREATE INDEX notifications_recipient_created_idx
    ON notifications (recipient_id, created_at DESC, id DESC);

-- Unread filter + count (GET /notifications?read_state=UNREAD + the unread badge).
CREATE INDEX notifications_unread_idx
    ON notifications (recipient_id)
    WHERE read_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS notifications;
