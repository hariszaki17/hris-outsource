-- +goose Up
-- Leave type master data (E2 operational master data / LT-2, LT-3). Drives E6
-- quota seeding and leave-request eligibility.
CREATE TABLE leave_types (
    id                   text PRIMARY KEY,              -- SWP-LT-<N>
    name                 text NOT NULL,
    code                 text NOT NULL,                 -- machine-readable, e.g. ANNUAL, SICK
    description          text NOT NULL DEFAULT '',
    default_annual_quota integer NOT NULL DEFAULT 0
                             CHECK (default_annual_quota >= 0),
    is_annual            boolean NOT NULL DEFAULT false,
    requires_document    boolean NOT NULL DEFAULT false,
    color                text NOT NULL DEFAULT '',      -- hex color for calendar UIs (E6)
    status               text NOT NULL DEFAULT 'active'
                             CHECK (status IN ('active', 'inactive')),
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    deleted_at           timestamptz                    -- soft-delete (CONVENTIONS §6)
);

-- Case-insensitive unique name among non-deleted leave types.
CREATE UNIQUE INDEX leave_types_name_uq
    ON leave_types (lower(name))
    WHERE deleted_at IS NULL;

-- Case-insensitive unique code among non-deleted leave types.
CREATE UNIQUE INDEX leave_types_code_uq
    ON leave_types (lower(code))
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS leave_types;
