-- +goose Up
-- Employee records for outsourced agents (E2 F2.1 / EP-*).
-- IDs allocated inline: 'SWP-EMP-' || swp_next_id('EMP').
-- user_id is nullable — linked to E1 User when provisioned (EP-3); no FK to
-- avoid circular dependency; kept loose like Phase-1 persona literals.
CREATE TABLE employees (
    id                       text PRIMARY KEY,              -- SWP-EMP-<N>
    user_id                  text,                          -- nullable; SWP-USR-<N> when provisioned
    full_name                text NOT NULL,
    nik                      text NOT NULL,                 -- Indonesian KTP; unique (EP-2)
    nip                      text NOT NULL DEFAULT '',      -- SWP internal employee number
    join_at                  date NOT NULL,
    gender                   text CHECK (gender IN ('MALE', 'FEMALE') OR gender IS NULL),
    birth_date               date,
    birth_place              text,
    phone                    text,
    email_personal           text,
    address                  text,
    npwp                     text,                          -- Indonesian tax ID; HR-only edit
    bpjs_kesehatan           text,
    bpjs_ketenagakerjaan     text,
    -- Flat columns for the bank_account object (avoids jsonb for indexed fields)
    bank_name                text,
    bank_account_number      text,
    bank_account_holder_name text,
    status                   text NOT NULL DEFAULT 'active'
                                 CHECK (status IN ('active', 'inactive')),
    created_by               text,                          -- SWP-EMP-<N> of creating HR user
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    deleted_at               timestamptz                    -- soft-delete (CONVENTIONS §6)
);

-- NIK unique among non-deleted employees (EP-2).
CREATE UNIQUE INDEX employees_nik_uq
    ON employees (nik)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS employees;
