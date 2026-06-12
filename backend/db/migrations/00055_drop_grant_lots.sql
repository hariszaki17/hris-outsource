-- +goose Up
-- Retire the grant-lot ledger (decision 2026-06-12). The per-type cap_basis ledger
-- (leave_quotas windows, migr. 00050/00051) is now the sole balance model, so the
-- grant-lot storage and the leave_requests per-lot snapshot columns are dropped.
--
-- Done in FK-safe order: leave_consumptions references leave_grants, so it goes first.
-- The legacy leave_quotas columns (total/used/pending/period/...) are intentionally
-- NOT dropped here — the deprecated QuotaService surface and OpenQuotaWindow's
-- transitional inserts still reference them; that removal is a separate migration.

-- 1. grant-lot tables (leave_consumptions FKs leave_grants → drop it first).
DROP TABLE IF EXISTS leave_consumptions;
DROP TABLE IF EXISTS leave_grants;

-- 2. leave_requests per-lot snapshot columns (the per-type meter has no FIFO split).
ALTER TABLE leave_requests
    DROP COLUMN IF EXISTS balance_earmark,
    DROP COLUMN IF EXISTS balance_allocation;

-- +goose Down
-- Best-effort restore: recreate the columns + tables (data is NOT recovered).
ALTER TABLE leave_requests
    ADD COLUMN balance_earmark    text,
    ADD COLUMN balance_allocation jsonb;

CREATE TABLE leave_grants (
    id              text PRIMARY KEY DEFAULT ('SWP-LG-' || swp_next_id('LG')),
    employee_id     text NOT NULL REFERENCES employees(id),
    amount_days     integer NOT NULL DEFAULT 0,
    granted_at      timestamptz NOT NULL DEFAULT now(),
    effective_from  date NOT NULL,
    expires_at      date NOT NULL,
    source          text NOT NULL
        CHECK (source IN ('ANNUAL','ADJUSTMENT','MATERNITY','STATUTORY','MIGRATION','BONUS')),
    earmark         text,
    remark          text,
    consumed_days   integer NOT NULL DEFAULT 0,
    pending_days    integer NOT NULL DEFAULT 0,
    created_by      text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,
    CHECK (amount_days >= 0),
    CHECK (consumed_days >= 0),
    CHECK (pending_days >= 0),
    CHECK (consumed_days + pending_days <= amount_days)
);
CREATE INDEX leave_grants_employee_expiry_idx
    ON leave_grants (employee_id, expires_at) WHERE deleted_at IS NULL;
CREATE INDEX leave_grants_alloc_idx
    ON leave_grants (employee_id, earmark, expires_at, granted_at, id) WHERE deleted_at IS NULL;

CREATE TABLE leave_consumptions (
    id               text PRIMARY KEY DEFAULT ('SWP-LC-' || swp_next_id('LC')),
    leave_request_id text NOT NULL REFERENCES leave_requests(id),
    grant_id         text NOT NULL REFERENCES leave_grants(id),
    days             integer NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX leave_consumptions_request_idx ON leave_consumptions (leave_request_id);
CREATE INDEX leave_consumptions_grant_idx   ON leave_consumptions (grant_id);
