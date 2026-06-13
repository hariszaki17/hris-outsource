-- +goose Up
-- Drop the legacy soft-reservation columns on leave_quotas (00029). The per-type
-- cap_basis ledger (00050/00051) is now the sole balance model: period_key +
-- entitled_days/used_days/pending_days + source/remark/expires_at supersede the
-- integer period + total/used/pending/closed/prorate columns, and the deprecated
-- QuotaService.List/Adjust/BulkGrant surface that read them is removed. The
-- period-keyed unique index is replaced by the period_key one (00051).

DROP INDEX IF EXISTS leave_quotas_emp_type_period_uq;

ALTER TABLE leave_quotas
    DROP COLUMN IF EXISTS period,
    DROP COLUMN IF EXISTS period_start,
    DROP COLUMN IF EXISTS period_end,
    DROP COLUMN IF EXISTS total,
    DROP COLUMN IF EXISTS used,
    DROP COLUMN IF EXISTS pending,
    DROP COLUMN IF EXISTS is_prorated,
    DROP COLUMN IF EXISTS prorate_months,
    DROP COLUMN IF EXISTS closed;

-- +goose Down
-- Best-effort restore (data not recovered): re-add the legacy columns + index,
-- backfilling period* from the per-type window where possible.
ALTER TABLE leave_quotas
    ADD COLUMN period         integer NOT NULL DEFAULT 0,
    ADD COLUMN period_start   date NOT NULL DEFAULT '1970-01-01',
    ADD COLUMN period_end     date NOT NULL DEFAULT '1970-12-31',
    ADD COLUMN total          integer NOT NULL DEFAULT 0,
    ADD COLUMN used           integer NOT NULL DEFAULT 0,
    ADD COLUMN pending        integer NOT NULL DEFAULT 0,
    ADD COLUMN is_prorated    boolean NOT NULL DEFAULT false,
    ADD COLUMN prorate_months integer NOT NULL DEFAULT 0,
    ADD COLUMN closed         boolean NOT NULL DEFAULT false;

UPDATE leave_quotas SET
    total = entitled_days,
    used  = used_days,
    pending = pending_days;

CREATE UNIQUE INDEX leave_quotas_emp_type_period_uq
    ON leave_quotas (employee_id, leave_type_id, period);
