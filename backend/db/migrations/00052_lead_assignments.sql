-- +goose Up
-- Adds the `lead` system role (service-line operational approver) and its
-- multi-company assignment table. A lead is SWP staff with a STORED users.role
-- ('lead'), unlike shift_leader which is derived from a placement. A lead covers
-- MANY client companies; lead_assignments holds one ACTIVE row per (lead, company),
-- mirroring shift_leader_assignments but WITHOUT the one-unit-per-leader (INV-3)
-- restriction. The auth middleware derives a lead's company SET (Principal.CompanyIDs)
-- from the active rows here at request time.

-- Widen the users.role CHECK to include 'lead'. The inline CHECK in 00002 is
-- auto-named users_role_check by Postgres.
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('super_admin','hr_admin','shift_leader','agent','lead'));

-- Lead assignments: a lead (employee_id) is assigned to a client company (and
-- optionally a specific site). Unlike shift_leader_assignments there may be MANY
-- active rows per lead — one per covered company.
-- IDs allocated via the column DEFAULT 'SWP-LA-' || swp_next_id('LA').
CREATE TABLE lead_assignments (
    id                text PRIMARY KEY DEFAULT ('SWP-LA-' || swp_next_id('LA')),
    employee_id       text NOT NULL REFERENCES employees(id),
    client_company_id text NOT NULL REFERENCES client_companies(id),
    site_id           text REFERENCES client_sites(id),       -- optional site narrowing; null = whole company
    assigned_at       timestamptz NOT NULL DEFAULT now(),
    unassigned_at     timestamptz,                             -- null while active
    assigned_by       text,                                    -- SWP-USR-<N>; null/'system' for auto-assign
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

-- At most one ACTIVE assignment per (lead, company): re-assigning is idempotent and
-- the active set is well-defined for the middleware's company-scope derivation.
CREATE UNIQUE INDEX lead_assignment_active_uq
    ON lead_assignments (employee_id, client_company_id)
    WHERE unassigned_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS lead_assignments;

ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('super_admin','hr_admin','shift_leader','agent'));
