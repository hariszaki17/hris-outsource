-- +goose Up
-- Historical, read-only payroll archive (E8 F8.1/F8.2 / SWP-PS-*). Payslips are
-- migrated from lumen_swp.employee_payslips by E9 and are IMMUTABLE in-app
-- (INV-1) — the only writes in this milestone are append-only audit notes and
-- export-job rows; payslips + components + benefits are SEEDED (10-02).
--
-- ENCRYPTION AT REST (INV-2): every monetary field is stored AES-256-GCM
-- CIPHERTEXT in a `*_enc bytea` column — NEVER plaintext. The plaintext is the
-- decimal-string Money e.g. '8500000.00' (openapi schemas.Money, 2 fractional
-- digits). Ciphertext is produced/consumed by internal/platform/crypto; the
-- decrypt happens at the SERVICE boundary in 10-02 (NOT in SQL). A row whose
-- ciphertext fails to decrypt is NOT an error: it surfaces as status
-- DECRYPT_FAIL on a 200 OK response, with the monetary fields nulled
-- (openapi PayslipStatus.DECRYPT_FAIL / locked_reason decrypt_fail). The seed
-- plants a row with deliberately-corrupt ciphertext to exercise this path.
--
-- IDs allocated via the column DEFAULT 'SWP-PS-' || swp_next_id('PS')
-- (Phase-5 column-DEFAULT allocation, decision [05-01]): INSERTs omit id to let
-- the DEFAULT fire, OR supply an explicit id (seed/test).
--
-- status/source enums are owned by the app (internal/domain/payroll) and pinned
-- to openapi schemas.PayslipStatus / SourceRef.system; the CHECKs below are the
-- DB backstops. The AUTHORITATIVE status is re-derived at read time from whether
-- decrypt succeeds (10-02); this column is the persisted hint (and the `status`
-- list-filter source).
CREATE TABLE payslips (
    id                  text PRIMARY KEY DEFAULT ('SWP-PS-' || swp_next_id('PS')),
    employee_id         text NOT NULL REFERENCES employees(id),
    employee_name       text,                                   -- denormalized display (openapi employee_name nullable)
    placement_id        text REFERENCES placements(id),         -- NULLABLE: a migrated payslip may predate a placement row

    year                integer NOT NULL,
    month               integer NOT NULL CHECK (month BETWEEN 1 AND 12),
    paid_on             date,                                   -- NULLABLE (openapi paid_on nullable)
    working_days        integer,                                -- NULLABLE (null on decrypt-fail)

    -- ENCRYPTED monetary summary columns — AES-256-GCM ciphertext (INV-2),
    -- NEVER plaintext. Plaintext is the 2-decimal Money string e.g.
    -- '8500000.00'. Decrypted at the service boundary in 10-02; a row whose
    -- ciphertext fails to decrypt surfaces as status DECRYPT_FAIL (200, not an
    -- error). NULLABLE: a DECRYPT_FAIL row may carry deliberately-corrupt
    -- ciphertext (the seed plants garbage bytes here).
    gross_earnings_enc      bytea,
    gross_deductions_enc    bytea,
    take_home_pay_enc       bytea,

    status              text NOT NULL DEFAULT 'FINAL'
        CHECK (status IN ('FINAL','DECRYPT_FAIL')),

    source_system       text NOT NULL DEFAULT 'lumen_swp'
        CHECK (source_system IN ('lumen_swp')),
    source_id           text NOT NULL,                          -- legacy employee_payslips.id (SourceRef.source_id, a STRING)

    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz                             -- soft-delete (CONVENTIONS §6)
);

-- Per-employee lookups (employee_id filter).
CREATE INDEX payslips_employee_idx ON payslips (employee_id) WHERE deleted_at IS NULL;
-- period / year filters (year+month).
CREATE INDEX payslips_period_idx ON payslips (year, month) WHERE deleted_at IS NULL;
-- default sort paid_on:desc.
CREATE INDEX payslips_paid_on_idx ON payslips (paid_on) WHERE deleted_at IS NULL;

-- Earnings + deductions breakdown (separate-table approach per CONTEXT
-- discretion; `kind` distinguishes earning vs deduction). HR-only on read
-- (INV-3); on a DECRYPT_FAIL payslip the parent earnings/deductions arrays are
-- returned as []. value_enc is ENCRYPTED line value (ciphertext; null on
-- decrypt-fail). bigserial PK (no SWP id — nested line item, mirrors
-- overtime_approvals decision [08-01]/[09-01]).
CREATE TABLE payslip_components (
    id              bigserial PRIMARY KEY,
    payslip_id      text NOT NULL REFERENCES payslips(id),
    kind            text NOT NULL CHECK (kind IN ('EARNING','DEDUCTION')),
    name            text NOT NULL,                              -- component display name e.g. "Gaji Pokok"
    value_enc       bytea,                                      -- ENCRYPTED line value (ciphertext; null on decrypt-fail)
    for_bpjs        boolean NOT NULL DEFAULT false,             -- openapi for_bpjs (non-monetary, plaintext)
    sort_order      integer NOT NULL DEFAULT 0
);

CREATE INDEX payslip_components_payslip_idx
    ON payslip_components (payslip_id, kind, sort_order);

-- Employer-borne benefits (HR-only array, INV-4). value_enc ENCRYPTED.
CREATE TABLE payslip_benefits (
    id              bigserial PRIMARY KEY,
    payslip_id      text NOT NULL REFERENCES payslips(id),
    name            text NOT NULL,
    value_enc       bytea,                                      -- ENCRYPTED benefit value (ciphertext; null on decrypt-fail)
    sort_order      integer NOT NULL DEFAULT 0
);

CREATE INDEX payslip_benefits_payslip_idx
    ON payslip_benefits (payslip_id, sort_order);

-- Append-only HR annotations (PA-7, §8 immutable-with-audited-note). The id is
-- a COMPOSITE "{payslip_id}-NOTE-{seq}" assigned by the SERVICE (openapi
-- PayslipAuditNote.id) — NOT a swp_next_id DEFAULT; declared as plain text
-- PRIMARY KEY (the service supplies it). The service computes seq =
-- count(existing)+1 in the insert tx. text length (minLength 1 / maxLength 4000)
-- is enforced in the service.
CREATE TABLE payslip_audit_notes (
    id              text PRIMARY KEY,                           -- composite "{payslip_id}-NOTE-{seq}" (service-assigned)
    payslip_id      text NOT NULL REFERENCES payslips(id),
    seq             integer NOT NULL,                           -- per-payslip sequence (service: count+1)
    text            text NOT NULL,                              -- note body (length enforced in service)
    author_id       text NOT NULL,
    author_name     text,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX payslip_audit_notes_payslip_idx
    ON payslip_audit_notes (payslip_id, seq);
-- Guard against seq collisions (the service's count+1 under concurrent appends).
CREATE UNIQUE INDEX payslip_audit_notes_seq_uq
    ON payslip_audit_notes (payslip_id, seq);

-- +goose Down
DROP TABLE IF EXISTS payslip_audit_notes;
DROP TABLE IF EXISTS payslip_benefits;
DROP TABLE IF EXISTS payslip_components;
DROP TABLE IF EXISTS payslips;
