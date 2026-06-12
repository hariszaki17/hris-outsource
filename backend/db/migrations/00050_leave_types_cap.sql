-- +goose Up
-- E6 per-type leave ledger (resolved 2026-06-12, EPICS §8 "E6 — Leave"):
-- leave_type becomes the entitlement/cap axis. Each type carries its own
-- cap mechanics so statutory/sick/religious leave meters in its own window and
-- never depletes the annual pool (Indonesian law: Pasal 93 vs Pasal 79).
-- Extends leave_types (migr. 00013) with cap_basis + supporting fields.
ALTER TABLE leave_types
    ADD COLUMN category          text NOT NULL DEFAULT 'OTHER',
    ADD COLUMN cap_basis         text NOT NULL DEFAULT 'UNCAPPED'
        CHECK (cap_basis IN (
            'ANNUAL_POOL', 'PER_EVENT', 'PER_MONTH', 'PER_YEAR_COUNT',
            'UNCAPPED', 'LIFETIME_ONCE', 'SERVICE_UNPAID')),
    ADD COLUMN cap_value         integer CHECK (cap_value IS NULL OR cap_value >= 0),
    ADD COLUMN cap_unit          text NOT NULL DEFAULT 'DAYS'
        CHECK (cap_unit IN ('DAYS', 'COUNT')),
    ADD COLUMN paid              boolean NOT NULL DEFAULT true,
    ADD COLUMN gender            text NOT NULL DEFAULT 'ANY'
        CHECK (gender IN ('ANY', 'FEMALE', 'MALE')),
    ADD COLUMN notice_days       integer NOT NULL DEFAULT 0
        CHECK (notice_days >= 0),
    ADD COLUMN min_service_years integer NOT NULL DEFAULT 0
        CHECK (min_service_years >= 0),
    ADD COLUMN lead_days         integer NOT NULL DEFAULT 0
        CHECK (lead_days >= 0),
    ADD COLUMN trail_days        integer NOT NULL DEFAULT 0
        CHECK (trail_days >= 0);

-- Backfill cap_basis from the legacy is_annual flag so existing rows are coherent.
UPDATE leave_types SET cap_basis = 'ANNUAL_POOL' WHERE is_annual = true;

-- +goose Down
ALTER TABLE leave_types
    DROP COLUMN category,
    DROP COLUMN cap_basis,
    DROP COLUMN cap_value,
    DROP COLUMN cap_unit,
    DROP COLUMN paid,
    DROP COLUMN gender,
    DROP COLUMN notice_days,
    DROP COLUMN min_service_years,
    DROP COLUMN lead_days,
    DROP COLUMN trail_days;
