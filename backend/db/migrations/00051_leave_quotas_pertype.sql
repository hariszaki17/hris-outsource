-- +goose Up
-- E6 per-type leave ledger, Phase 1 — foundation (EPICS §8 "E6 — Leave" 2026-06-12).
-- Reverts the live balance model from grant-lots (leave_grants, 00044) back to a
-- per-type quota keyed by (employee, leave_type, window), where the window is
-- driven by the leave type's cap_basis (00050). This migration is ADDITIVE: it
-- evolves leave_quotas (00029) with the new columns and backfills them from the
-- legacy ones, WITHOUT dropping total/used/pending/period yet — so the existing
-- (now-deprecated) sqlc queries keep compiling. Later phases rewire the service
-- layer onto these columns, then a final migration drops the legacy columns and
-- the grant-lot tables.

-- New per-type window columns. period_key generalizes the old integer `period`:
--   '<year>'        for ANNUAL_POOL / PER_YEAR_COUNT
--   '<year>-<MM>'   for PER_MONTH
--   'EMP'           for LIFETIME_ONCE / SERVICE_UNPAID
ALTER TABLE leave_quotas
    ADD COLUMN period_key    text,
    ADD COLUMN entitled_days integer NOT NULL DEFAULT 0
                   CHECK (entitled_days >= 0),
    ADD COLUMN used_days     integer NOT NULL DEFAULT 0
                   CHECK (used_days >= 0),
    ADD COLUMN pending_days  integer NOT NULL DEFAULT 0
                   CHECK (pending_days >= 0),
    ADD COLUMN source        text NOT NULL DEFAULT 'AUTO'
                   CHECK (source IN ('AUTO', 'ADJUSTMENT', 'MIGRATION')),
    ADD COLUMN remark        text NOT NULL DEFAULT '',
    ADD COLUMN expires_at    date,
    ADD COLUMN created_by    text REFERENCES users(id);

-- Backfill the new columns from the legacy ones so existing rows are coherent.
UPDATE leave_quotas SET
    period_key    = period::text,
    entitled_days = total,
    used_days     = used,
    pending_days  = pending,
    expires_at    = period_end
WHERE period_key IS NULL;

-- One quota per (employee, leave_type, window). Coexists with the legacy
-- leave_quotas_emp_type_period_uq until the legacy columns are dropped.
CREATE UNIQUE INDEX leave_quotas_emp_type_periodkey_uq
    ON leave_quotas (employee_id, leave_type_id, period_key);

-- Link a leave request to the per-type quota window it draws from (quota-bearing
-- cap_basis only; PER_EVENT / UNCAPPED leave it null). Replaces the grant-lot
-- snapshot (leave_requests.balance_earmark / balance_allocation, 00044) in later
-- phases.
ALTER TABLE leave_requests
    ADD COLUMN quota_id text REFERENCES leave_quotas(id);

-- +goose Down
ALTER TABLE leave_requests
    DROP COLUMN quota_id;

DROP INDEX IF EXISTS leave_quotas_emp_type_periodkey_uq;

ALTER TABLE leave_quotas
    DROP COLUMN period_key,
    DROP COLUMN entitled_days,
    DROP COLUMN used_days,
    DROP COLUMN pending_days,
    DROP COLUMN source,
    DROP COLUMN remark,
    DROP COLUMN expires_at,
    DROP COLUMN created_by;
