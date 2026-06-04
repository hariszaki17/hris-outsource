-- +goose Up
-- Employment agreements (PKWT / PKWTT) for employees (E2 F2.2 / EA-*).
-- IDs allocated inline: 'SWP-AG-' || swp_next_id('AG').
-- Compensation stored as plain columns this milestone; encryption deferred (EA-4).
CREATE TABLE employment_agreements (
    id               text PRIMARY KEY,                  -- SWP-AG-<N>
    employee_id      text NOT NULL REFERENCES employees(id),
    type             text NOT NULL
                         CHECK (type IN ('PKWT', 'PKWTT')),
    agreement_no     text NOT NULL DEFAULT '',
    start_date       date NOT NULL,
    end_date         date,                              -- required for PKWT; null for PKWTT
    status           text NOT NULL DEFAULT 'active'
                         CHECK (status IN ('active', 'superseded', 'closed')),
    predecessor_id   text,                              -- set on the successor created via :renew (EA-3)
    successor_id     text,                              -- set on the old agreement when superseded
    closed_reason    text
                         CHECK (closed_reason IN ('RESIGNED', 'TERMINATED', 'END_OF_TERM', 'OTHER')
                                OR closed_reason IS NULL),
    closed_at        timestamptz,
    -- Compensation columns (flat; encryption at rest deferred)
    base_salary_idr  numeric,
    bpjs_terms       jsonb,                             -- BpjsTerms object
    tax_profile      text,                              -- PTKP code e.g. PTKP_TK0
    comp_effective_date date,                           -- effective date of these comp terms
    created_by       text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz                        -- soft-delete (CONVENTIONS §6)
);

-- EA-2: exactly one active agreement per employee at the DB level.
-- Partial unique index only across non-deleted, active rows.
CREATE UNIQUE INDEX employment_agreements_active_employee_uq
    ON employment_agreements (employee_id)
    WHERE status = 'active' AND deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS employment_agreements;
