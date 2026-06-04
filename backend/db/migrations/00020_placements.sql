-- +goose Up
-- Placements (E3 F3.1/F3.2 / PLC-*): the project's first-class differentiator.
-- An agent is *placed* at a client company, in a service line, at exactly one
-- site, for a contract period, with full lifecycle history.
-- IDs allocated via the column DEFAULT 'SWP-PL-' || swp_next_id('PL') (per plan).
-- INV-1 (<=1 active placement per agent) is enforced at the DB level via a
-- partial unique index, mirroring Phase-4's employment_agreements_active_employee_uq.
-- INV-5: site_id is required and FKs to a Phase-3 client_sites row.
CREATE TABLE placements (
    id                            text PRIMARY KEY DEFAULT ('SWP-PL-' || swp_next_id('PL')),
    employee_id                   text NOT NULL REFERENCES employees(id),
    agreement_id                  text NOT NULL REFERENCES employment_agreements(id),
    client_company_id             text NOT NULL REFERENCES client_companies(id),
    site_id                       text NOT NULL REFERENCES client_sites(id),   -- INV-5
    service_line_id               text NOT NULL REFERENCES service_lines(id),
    position_id                   text NOT NULL REFERENCES positions(id),
    start_date                    date NOT NULL,
    end_date                      date,                                        -- null = open-ended (PKWTT)
    annual_leave_entitlement_days integer,
    base_salary_ref_idr           bigint,                                      -- IDR amounts exceed int32
    notes                         text,
    lifecycle_status              text NOT NULL DEFAULT 'ACTIVE'
        CHECK (lifecycle_status IN ('PENDING_START','ACTIVE','EXTENDED','EXPIRING',
                                    'ENDED','TRANSFERRED','TERMINATED','RESIGNED','SUPERSEDED')),
    status_changed_at             timestamptz NOT NULL DEFAULT now(),
    ended_reason                  text
        CHECK (ended_reason IN ('END_OF_TERM','ENDED','TERMINATED','RESIGNED','TRANSFERRED','SUPERSEDED')
               OR ended_reason IS NULL),
    ended_at                      date,
    termination_reason            text,
    resign_at                     date,
    predecessor_id                text REFERENCES placements(id),
    successor_id                  text REFERENCES placements(id),
    backdate_reason               text,
    created_by                    text,                                        -- SWP-USR-<N>
    created_at                    timestamptz NOT NULL DEFAULT now(),
    updated_at                    timestamptz NOT NULL DEFAULT now(),
    deleted_at                    timestamptz                                  -- soft-delete (CONVENTIONS §6)
);

-- INV-1 backstop: at most one NON-TERMINAL placement per employee (race-proof).
-- Mirrors Phase-4 employment_agreements_active_employee_uq. 'SCHEDULED' is an
-- inert forward-compat term: it never matches the CHECK-constrained column, so
-- it is a harmless backstop, never a real mismatch (see plan Task 1 DECISION).
CREATE UNIQUE INDEX placements_active_employee_uq
    ON placements (employee_id)
    WHERE lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START','SCHEDULED')
      AND deleted_at IS NULL;

CREATE INDEX placements_company_idx        ON placements (client_company_id) WHERE deleted_at IS NULL;
CREATE INDEX placements_employee_idx       ON placements (employee_id)       WHERE deleted_at IS NULL;
CREATE INDEX placements_status_changed_idx ON placements (status_changed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS placements;
