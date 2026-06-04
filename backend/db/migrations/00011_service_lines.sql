-- +goose Up
-- Service lines (E2 F2.1 / SP-1). The three SWP service lines: Facility
-- Services, Building Management, Parking. Positions (F2.2) reference this table.
CREATE TABLE service_lines (
    id         text PRIMARY KEY,              -- SWP-SVC-<N>
    name       text NOT NULL,
    status     text NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'inactive')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz                    -- soft-delete (CONVENTIONS §6)
);

-- Case-insensitive unique name among non-deleted service lines.
CREATE UNIQUE INDEX service_lines_name_uq
    ON service_lines (lower(name))
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS service_lines;
