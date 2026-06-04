-- +goose Up
-- Overtime rule master data (E2 / E7 overtime). Null service_line_id = global
-- default; line-scoped rule wins when both apply (C-4 / OR-2).
-- min_minutes locked at 30 per EPICS.md §8 D4 (resolved 2026-06-02).
CREATE TABLE overtime_rules (
    id                   text PRIMARY KEY,              -- SWP-OTR-<N>
    name                 text NOT NULL,
    service_line_id      text REFERENCES service_lines(id), -- NULL = global default (OR-2)
    weekday_rate         real NOT NULL,
    restday_rate         real NOT NULL,
    holiday_rate         real NOT NULL,
    min_minutes          integer NOT NULL DEFAULT 30
                             CHECK (min_minutes >= 30),
    max_minutes_per_day  integer NOT NULL DEFAULT 240,
    pre_approval_required boolean NOT NULL DEFAULT true,
    status               text NOT NULL DEFAULT 'active'
                             CHECK (status IN ('active', 'inactive')),
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    deleted_at           timestamptz                    -- soft-delete (CONVENTIONS §6)
);

-- Case-insensitive unique name among non-deleted overtime rules.
CREATE UNIQUE INDEX overtime_rules_name_uq
    ON overtime_rules (lower(name))
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS overtime_rules;
