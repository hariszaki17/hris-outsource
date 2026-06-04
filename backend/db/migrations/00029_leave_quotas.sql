-- +goose Up
-- Leave quotas (E6 F6.3 / SWP-LQ-*): one row per (employee, leave_type, period).
-- The soft-reservation balance model: total entitlement, used (deducted from
-- APPROVED requests), pending (held by open PENDING_L1/PENDING_HR requests);
-- remaining = total - used - pending is DERIVED at the DTO boundary (a domain
-- method), NOT stored. period is the calendar year; period_start/period_end pin
-- the YYYY-01-01 / YYYY-12-31 window. IDs allocated via the column DEFAULT
-- 'SWP-LQ-' || swp_next_id('LQ') — INSERTs omit id to let DEFAULT fire.
-- INV-1 quota guard (08-02) reads remaining; bulk-grant upserts total (never
-- overwriting used); :adjust mutates total with an audited last_adjustment; an HR
-- override that drives remaining negative records last_override.
CREATE TABLE leave_quotas (
    id              text PRIMARY KEY DEFAULT ('SWP-LQ-' || swp_next_id('LQ')),
    employee_id     text NOT NULL REFERENCES employees(id),
    leave_type_id   text NOT NULL REFERENCES leave_types(id),
    period          integer NOT NULL,                              -- calendar year
    period_start    date NOT NULL,                                 -- YYYY-01-01
    period_end      date NOT NULL,                                 -- YYYY-12-31
    total           integer NOT NULL DEFAULT 0,                    -- entitlement (possibly pro-rated)
    used            integer NOT NULL DEFAULT 0,                    -- deducted from APPROVED requests
    pending         integer NOT NULL DEFAULT 0,                    -- soft-reservation; remaining = total-used-pending (DERIVED, not stored)
    is_prorated     boolean NOT NULL DEFAULT false,
    prorate_months  integer NOT NULL DEFAULT 0,
    closed          boolean NOT NULL DEFAULT false,                -- set after period-end expiry job (LQ-4; not built this phase)
    last_adjustment jsonb,                                         -- {delta,reason,adjusted_by,adjusted_at} nullable
    last_override   jsonb,                                         -- {leave_request_id,override_reason,overridden_by,overridden_at} nullable
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- One quota row per (employee, leave_type, period) — drives UpsertLeaveQuota's
-- ON CONFLICT and the FindQuotaForEmployeeTypePeriod guard.
CREATE UNIQUE INDEX leave_quotas_emp_type_period_uq
    ON leave_quotas (employee_id, leave_type_id, period);

-- +goose Down
DROP TABLE IF EXISTS leave_quotas;
