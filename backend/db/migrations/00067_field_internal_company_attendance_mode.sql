-- +goose Up
-- FIELD vs INTERNAL employees, CLIENT vs INTERNAL companies, ONSITE vs REMOTE
-- attendance (2026-06-16). SWP has its own back-office staff (INTERNAL) alongside
-- the outsourced field agents (FIELD); SWP itself is an INTERNAL "company" record
-- (placement target) distinct from the CLIENT companies agents are placed at; and an
-- attendance record is captured ONSITE (geofenced client/internal site) or REMOTE
-- (work-from-home / off-site). All three are non-null with backfilling defaults so
-- existing rows keep today's semantics (FIELD / CLIENT / ONSITE).
ALTER TABLE employees
    ADD COLUMN employee_type text NOT NULL DEFAULT 'FIELD'
        CHECK (employee_type IN ('FIELD','INTERNAL'));

ALTER TABLE client_companies
    ADD COLUMN type text NOT NULL DEFAULT 'CLIENT'
        CHECK (type IN ('CLIENT','INTERNAL'));

ALTER TABLE attendance
    ADD COLUMN mode text NOT NULL DEFAULT 'ONSITE'
        CHECK (mode IN ('ONSITE','REMOTE'));

-- +goose Down
ALTER TABLE attendance DROP COLUMN IF EXISTS mode;
ALTER TABLE client_companies DROP COLUMN IF EXISTS type;
ALTER TABLE employees DROP COLUMN IF EXISTS employee_type;
