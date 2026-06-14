-- +goose Up
-- Positions within a service line (E2 F2.2 / SP-3). Unique name per line.
CREATE TABLE positions (
    id              text PRIMARY KEY,              -- SWP-POS-<N>
    service_line_id text NOT NULL REFERENCES service_lines(id),
    name            text NOT NULL,
    alias           text NOT NULL DEFAULT '',      -- English label / alternative name
    status          text NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active', 'inactive')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz                    -- soft-delete (CONVENTIONS §6)
);

-- Case-insensitive unique position name within a service line.
CREATE UNIQUE INDEX positions_line_name_uq
    ON positions (service_line_id, lower(name))
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS positions;
