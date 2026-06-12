-- +goose Up
-- Remove the service_line axis and the position master entirely (decision
-- 2026-06-12). service_line is dropped from every table that carried it; Position
-- becomes FREE-TEXT (a plain text column on placements, backfilled from the
-- now-dropped positions master), and the service_lines + positions master tables are
-- dropped. Shift master, overtime rules, holidays, leave, scheduling all lose their
-- service-line scope. Attendance's position→free-text + service_line drop is handled
-- earlier by 00053_attendance_position_text.sql; this migration does NOT re-touch
-- attendance. Done in FK-safe order: convert position_id → free-text first, then drop
-- the now-unreferenced master tables last.
--
-- FK names referenced below are Postgres auto-generated (inline REFERENCES):
--   placements.service_line_id  → placements_service_line_id_fkey   (00020)
--   placements.position_id      → placements_position_id_fkey       (00020)
--   shift_masters.service_line_id → shift_masters_service_line_id_fkey (00023)
--   schedule_entries.service_line_id → schedule_entries_service_line_id_fkey (00024)
--   overtime_rules.service_line_id → overtime_rules_service_line_id_fkey (00015)
-- overtime.service_line_id and leave_requests.service_line_id are plain (no FK).

-- 1. placements: position_id (FK to positions) → free-text position; drop service_line_id.
ALTER TABLE placements ADD COLUMN position text;
UPDATE placements pl
    SET position = p.name
    FROM positions p
    WHERE p.id = pl.position_id;
ALTER TABLE placements ALTER COLUMN position SET NOT NULL;
ALTER TABLE placements DROP CONSTRAINT placements_position_id_fkey;
ALTER TABLE placements DROP COLUMN position_id;
ALTER TABLE placements DROP CONSTRAINT placements_service_line_id_fkey;
ALTER TABLE placements DROP COLUMN service_line_id;

-- 2. shift_masters + schedule_entries: drop service_line_id (+ shift_masters index).
DROP INDEX IF EXISTS shift_masters_service_line_idx;
ALTER TABLE shift_masters DROP COLUMN service_line_id;
ALTER TABLE schedule_entries DROP COLUMN service_line_id;

-- 3. overtime_rules / overtime / leave_requests: drop service_line_id.
ALTER TABLE overtime_rules DROP COLUMN service_line_id;
ALTER TABLE overtime DROP COLUMN service_line_id;
ALTER TABLE leave_requests DROP COLUMN service_line_id;

-- 4. holidays: drop applicable_service_lines (holidays are global only now).
ALTER TABLE holidays DROP COLUMN applicable_service_lines;

-- 5. Drop the now-unreferenced master tables. (attendance no longer references
--    positions after 00053_attendance_position_text.sql.)
DROP TABLE positions;
DROP TABLE service_lines;

-- +goose Down
-- Best-effort reverse: recreate the master tables + columns as they were in their
-- original migrations (00011/00012/00015/00020/00023/00024/00031/00028/00032).
-- Backfill is NOT required on down (decision 2026-06-12); new columns come back
-- nullable / defaulted so the rollback stays syntactically valid and applies cleanly.

-- Recreate service_lines (00011) + positions (00012) masters.
CREATE TABLE service_lines (
    id         text PRIMARY KEY,
    name       text NOT NULL,
    status     text NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'inactive')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE UNIQUE INDEX service_lines_name_uq
    ON service_lines (lower(name))
    WHERE deleted_at IS NULL;

CREATE TABLE positions (
    id              text PRIMARY KEY,
    service_line_id text NOT NULL REFERENCES service_lines(id),
    name            text NOT NULL,
    alias           text NOT NULL DEFAULT '',
    status          text NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active', 'inactive')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);
CREATE UNIQUE INDEX positions_line_name_uq
    ON positions (service_line_id, lower(name))
    WHERE deleted_at IS NULL;

-- holidays.applicable_service_lines (00032).
ALTER TABLE holidays
    ADD COLUMN applicable_service_lines text[] NOT NULL DEFAULT '{}';

-- overtime_rules / overtime / leave_requests service_line_id (00015/00031/00028).
ALTER TABLE overtime_rules
    ADD COLUMN service_line_id text REFERENCES service_lines(id);
ALTER TABLE overtime
    ADD COLUMN service_line_id text;
ALTER TABLE leave_requests
    ADD COLUMN service_line_id text;

-- shift_masters / schedule_entries service_line_id (00023/00024).
ALTER TABLE shift_masters
    ADD COLUMN service_line_id text REFERENCES service_lines(id);
CREATE INDEX shift_masters_service_line_idx
    ON shift_masters (service_line_id)
    WHERE deleted_at IS NULL;
ALTER TABLE schedule_entries
    ADD COLUMN service_line_id text REFERENCES service_lines(id);

-- placements: restore service_line_id + position_id (FKs), drop free-text position.
ALTER TABLE placements DROP COLUMN position;
ALTER TABLE placements
    ADD COLUMN service_line_id text REFERENCES service_lines(id),
    ADD COLUMN position_id     text REFERENCES positions(id);
