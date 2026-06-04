-- +goose Up
-- Client company directory (E2 F2.3). Statutory/billing info; physical
-- locations + geofence live on client_sites (F2.6).
CREATE TABLE client_companies (
    id           text PRIMARY KEY,              -- SWP-CMP-<N>
    name         text NOT NULL,
    address      text NOT NULL DEFAULT '',
    leader_scope text NOT NULL DEFAULT 'company'
                     CHECK (leader_scope IN ('company', 'site')),
    npwp         text,                          -- optional Indonesian tax ID, unique when set (CC-2)
    pic_name     text,
    phone        text,
    email        text,
    status       text NOT NULL DEFAULT 'active'
                     CHECK (status IN ('active', 'inactive')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz                    -- soft-delete (CONVENTIONS §6)
);

-- Case-insensitive unique name among non-deleted companies.
CREATE UNIQUE INDEX client_companies_name_uq
    ON client_companies (lower(name))
    WHERE deleted_at IS NULL;

-- NPWP unique among non-deleted companies that have one.
CREATE UNIQUE INDEX client_companies_npwp_uq
    ON client_companies (npwp)
    WHERE deleted_at IS NULL AND npwp IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS client_companies;
