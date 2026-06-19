-- +goose Up
-- F8.3 Payroll Run + F8.4 Payment Recording tables (SWP-PRR-* / SWP-PPY-*).
-- A payroll run groups a set of generated payslips for a (year, month); HR opens
-- a DRAFT run, assembles + reviews, then POSTs to make payslips immutable.
-- Payment recording is a manual transfer reference + evidence upload.
--
-- IDs use the per-prefix column-DEFAULT allocator (swp_next_id), same mechanism as
-- the existing payroll_periods / payslips tables. Prefixes:
--   payroll_runs    -> SWP-PRR-*
--   payroll_payments -> SWP-PPY-*
--
-- payroll_runs: one row per (year, month) — UNIQUE constraint prevents duplicate runs.
CREATE TABLE payroll_runs (
    id text PRIMARY KEY DEFAULT ('SWP-PRR-' || swp_next_id('PRR')),
    year integer NOT NULL,
    month integer NOT NULL CHECK (month >= 1 AND month <= 12),
    status text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','POSTED')),
    cutoff_date date NOT NULL,
    created_by text NOT NULL,
    posted_by text,
    posted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (year, month)
);

-- Extend payslips with the columns generated payslips need: payroll_run_id links
-- the payslip to its run; is_posted gates immutability per INV-1; payment_status
-- tracks whether the agent has been paid; source_type distinguishes migrated-vs-generated.
ALTER TABLE payslips
    ADD COLUMN IF NOT EXISTS payroll_run_id text REFERENCES payroll_runs(id),
    ADD COLUMN IF NOT EXISTS is_posted boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS payment_status text NOT NULL DEFAULT 'Unpaid'
        CHECK (payment_status IN ('Unpaid','Paid')),
    ADD COLUMN IF NOT EXISTS source_type text NOT NULL DEFAULT 'Migrated'
        CHECK (source_type IN ('Migrated','Generated'));

-- payroll_payments: one row per recorded payment against a payslip. amount_enc is
-- ENCRYPTED (AES-256-GCM ciphertext of the decimal Money string). evidence_file_id
-- is the SWP-FILE-* id from the file upload (agreement-attachment store).
CREATE TABLE payroll_payments (
    id text PRIMARY KEY DEFAULT ('SWP-PPY-' || swp_next_id('PPY')),
    payslip_id text NOT NULL REFERENCES payslips(id),
    amount_enc bytea,
    method text NOT NULL CHECK (method IN ('BankTransfer','Cash')),
    reference_no text,
    evidence_file_id text,
    paid_on date NOT NULL,
    paid_by text NOT NULL,
    voided_at timestamptz,
    void_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX payroll_payments_payslip_idx ON payroll_payments (payslip_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE payroll_payments;
ALTER TABLE payslips DROP COLUMN IF EXISTS source_type, DROP COLUMN IF EXISTS payment_status, DROP COLUMN IF EXISTS is_posted, DROP COLUMN IF EXISTS payroll_run_id;
DROP TABLE payroll_runs;
