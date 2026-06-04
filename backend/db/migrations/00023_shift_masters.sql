-- +goose Up
-- Shift masters (E4 F4.1 / SM-* / SWP-SHF-*): catalog templates for shifts.
-- A shift master holds a reusable HH:MM window (Asia/Jakarta, CONVENTIONS §10),
-- an optional break window, an optional service-line tag (NULL = applies to all
-- lines, SM-3), and a server-derived cross_midnight flag (06-02 sets it on write).
-- IDs allocated via the column DEFAULT 'SWP-SHF-' || swp_next_id('SHF') (mirrors
-- Phase-5 placements). Soft-delete (CONVENTIONS §6); name unique within the live
-- catalog (SM-4) via a partial unique index.
CREATE TABLE shift_masters (
    id                text PRIMARY KEY DEFAULT ('SWP-SHF-' || swp_next_id('SHF')),
    name              text NOT NULL,
    start_time        text NOT NULL,                      -- 'HH:MM' Asia/Jakarta (CONVENTIONS §10)
    end_time          text NOT NULL,                      -- 'HH:MM'
    break_start       text,                               -- 'HH:MM' optional
    break_end         text,                               -- 'HH:MM' optional
    service_line_id   text REFERENCES service_lines(id),  -- nullable: untagged = all lines (SM-3)
    cross_midnight    boolean NOT NULL DEFAULT false,     -- server-derived (end<=start); 06-02 sets it
    is_active         boolean NOT NULL DEFAULT true,
    created_by        text,                               -- SWP-USR-<N>
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz                         -- soft-delete (CONVENTIONS §6)
);

-- SM-4: unique name within the live catalog (exact match per FE Zod, not ci).
CREATE UNIQUE INDEX shift_masters_name_uq
    ON shift_masters (name)
    WHERE deleted_at IS NULL;
CREATE INDEX shift_masters_service_line_idx
    ON shift_masters (service_line_id)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS shift_masters;
