-- +goose Up
-- Physical placement location of a client company (E2 F2.6). Holds the
-- attendance geofence (center + radius) that E5 clock-in validates against.
-- Every company has >=1 site; exactly one is_primary per company (INV-5).
CREATE TABLE client_sites (
    id                text PRIMARY KEY,                          -- SWP-SITE-<N>
    client_company_id text NOT NULL REFERENCES client_companies(id),
    name              text NOT NULL,
    code              text,                                      -- optional, unique within company when set
    address           text NOT NULL DEFAULT '',
    geo_lat           double precision,                          -- nullable; geofence_active derived (not stored)
    geo_lng           double precision,
    geofence_radius_m integer NOT NULL DEFAULT 100
                          CHECK (geofence_radius_m BETWEEN 25 AND 1000),
    is_primary        boolean NOT NULL DEFAULT false,
    pic_name          text,
    phone             text,
    status            text NOT NULL DEFAULT 'active'
                          CHECK (status IN ('active', 'inactive')),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz                                -- soft-delete (CONVENTIONS §6)
);

-- Case-insensitive unique site name within a company among non-deleted sites.
CREATE UNIQUE INDEX client_sites_name_uq
    ON client_sites (client_company_id, lower(name))
    WHERE deleted_at IS NULL;

-- Enforces INV-5: exactly one primary site per company.
CREATE UNIQUE INDEX client_sites_one_primary_uq
    ON client_sites (client_company_id)
    WHERE is_primary = true AND deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS client_sites;
