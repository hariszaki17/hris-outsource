-- +goose Up
-- Shift-leader assignments (E3 F3.4 / SL-*). Models the leadership unit as
-- client_company_id (always) + nullable site_id (set only when the company's
-- leader_scope='site'). INV-2 (one leader per unit) and INV-3 (one unit per
-- leader) are enforced at the DB level via partial unique indexes.
-- IDs allocated via the column DEFAULT 'SWP-SLA-' || swp_next_id('SLA').
CREATE TABLE shift_leader_assignments (
    id                text PRIMARY KEY DEFAULT ('SWP-SLA-' || swp_next_id('SLA')),
    client_company_id text NOT NULL REFERENCES client_companies(id),
    site_id           text REFERENCES client_sites(id),       -- null when leader_scope=company; set when =site
    employee_id       text NOT NULL REFERENCES employees(id),
    assigned_at       timestamptz NOT NULL DEFAULT now(),
    unassigned_at     timestamptz,                             -- null while active
    assigned_by       text,                                    -- SWP-USR-<N>; 'system' for auto-vacate
    vacated_reason    text
        CHECK (vacated_reason IN ('REASSIGNED','PLACEMENT_ENDED','MANUAL','COMPANY_ARCHIVED')
               OR vacated_reason IS NULL),
    notes             text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

-- INV-2 backstop (company-scope): at most one ACTIVE leader per company when site_id IS NULL.
CREATE UNIQUE INDEX sla_active_company_uq
    ON shift_leader_assignments (client_company_id)
    WHERE unassigned_at IS NULL AND site_id IS NULL;

-- INV-2 backstop (site-scope): at most one ACTIVE leader per site when site_id IS NOT NULL.
CREATE UNIQUE INDEX sla_active_site_uq
    ON shift_leader_assignments (site_id)
    WHERE unassigned_at IS NULL AND site_id IS NOT NULL;

-- INV-3 backstop: an employee leads at most one unit at a time (1:1).
CREATE UNIQUE INDEX sla_active_employee_uq
    ON shift_leader_assignments (employee_id)
    WHERE unassigned_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS shift_leader_assignments;
