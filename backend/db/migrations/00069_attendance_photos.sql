-- +goose Up
-- Standalone clock-in/out selfies (E5 F5.1 / CI-10, 2026-06-17). An agent uploads a
-- photo BEFORE clocking, gets back an SWP-FILE-* id, then passes it to clock-in/out as
-- photo_id. The row is orphaned until referenced — orphans expire 24h after upload.
-- IDs allocated inline: 'SWP-FILE-' || swp_next_id('FILE') (same minting as
-- agreement_attachments; both share the FILE prefix sequence).
-- Storage choice: bytea blob in-DB — mirrors agreement_attachments (no external
-- storage dependency; survives container teardown via reseed).
CREATE TABLE attendance_photos (
    id          text PRIMARY KEY,                   -- SWP-FILE-<N>
    employee_id text NOT NULL,                       -- SWP-EMP-<N> owner (self upload)
    caption     text NOT NULL DEFAULT '',
    file_name   text NOT NULL,
    mime        text NOT NULL,                        -- image/jpeg | image/png
    size_bytes  bigint NOT NULL,
    blob        bytea NOT NULL,                        -- file content (max 10 MB enforced in handler)
    created_at  timestamptz NOT NULL DEFAULT now(),
    -- Orphan-expiry horizon (created_at + 24h). A future sweep deletes rows past
    -- expires_at that were never attached to an attendance record. See TODO(orphan-expiry).
    expires_at  timestamptz NOT NULL DEFAULT now() + interval '24 hours'
);

-- Supports the (future) orphan-expiry sweep scanning by horizon.
CREATE INDEX attendance_photos_expires_at_idx ON attendance_photos (expires_at);

-- +goose Down
DROP TABLE IF EXISTS attendance_photos;
