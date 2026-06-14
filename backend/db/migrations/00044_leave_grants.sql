-- +goose Up
-- E6 leave-balance ledger redesign (resolved 2026-06-08, openapi E6 AUTHORITATIVE):
-- replace the per-(employee,leave_type,period) leave_quotas balance with a single
-- per-employee GRANT-LOT LEDGER. One lot per leave_grants insert, each with its own
-- hard expires_at (no carryover). Consumption is FIFO by soonest expires_at, recorded
-- per-lot as leave_consumptions rows (one per lot a request draws). Balance is
-- DERIVED: remaining = amount_days - consumed_days - pending_days; a lot is ACTIVE
-- while now < expires_at. earmark=null is the flat pool (ordinary FIFO); a non-null
-- earmark restricts a lot to a request of that purpose and hides it from ordinary
-- FIFO (LQ-10 earmark isolation).
--
-- leave_quotas is NOT dropped here (kept for history / rollback); the live balance
-- path is grants. IDs allocated via column DEFAULT ('SWP-LG-' || swp_next_id('LG'),
-- 'SWP-LC-' || swp_next_id('LC')) — INSERTs omit id to let DEFAULT fire, OR supply
-- an explicit id (seed/test).

CREATE TABLE leave_grants (
    id              text PRIMARY KEY DEFAULT ('SWP-LG-' || swp_next_id('LG')),
    employee_id     text NOT NULL REFERENCES employees(id),
    amount_days     integer NOT NULL DEFAULT 0,                  -- days granted by this lot (>=0)
    granted_at      timestamptz NOT NULL DEFAULT now(),          -- when the lot was granted
    effective_from  date NOT NULL,                               -- first date the lot may be drawn
    expires_at      date NOT NULL,                               -- hard per-lot expiry (LQ-4)
    source          text NOT NULL
        CHECK (source IN ('ANNUAL','ADJUSTMENT','MATERNITY','STATUTORY','MIGRATION','BONUS')),
    earmark         text,                                        -- null=general pool; non-null=purpose code (LQ-10)
    remark          text,                                        -- required free-text note (audited per LQ-6)
    consumed_days   integer NOT NULL DEFAULT 0,                  -- committed by APPROVED requests (= Σ this lot's leave_consumptions.days)
    pending_days    integer NOT NULL DEFAULT 0,                  -- reserved by open PENDING_* requests
    -- remaining = amount_days - consumed_days - pending_days is DERIVED, not stored.
    created_by      text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,                                 -- soft-delete (CONVENTIONS §6)
    CHECK (amount_days >= 0),
    CHECK (consumed_days >= 0),
    CHECK (pending_days >= 0),
    -- never-negative invariant (LQ-5): allocation can only reserve available days.
    CHECK (consumed_days + pending_days <= amount_days)
);

-- Ledger list + balance read: per-employee by expiry (FIFO order source), active-lot
-- filter. Partial on the soft-delete predicate.
CREATE INDEX leave_grants_employee_expiry_idx
    ON leave_grants (employee_id, expires_at)
    WHERE deleted_at IS NULL;
-- Active-lot allocation scan (FIFO reserve/commit): employee + earmark + expiry order.
-- earmark is included so an ordinary (earmark IS NULL) scan and a matching-earmark
-- scan both use this index; expires_at/granted_at/id give the FIFO tie-break order.
CREATE INDEX leave_grants_alloc_idx
    ON leave_grants (employee_id, earmark, expires_at, granted_at, id)
    WHERE deleted_at IS NULL;

CREATE TABLE leave_consumptions (
    id               text PRIMARY KEY DEFAULT ('SWP-LC-' || swp_next_id('LC')),
    leave_request_id text NOT NULL REFERENCES leave_requests(id),
    grant_id         text NOT NULL REFERENCES leave_grants(id),
    days             integer NOT NULL,                           -- days drawn from this lot by the request (>=1)
    created_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (days >= 1)
);

-- Reversal (cancel/shorten) + the GET /leave-grants/{id} consumptions[] read both
-- query by leave_request_id and grant_id.
CREATE INDEX leave_consumptions_request_idx ON leave_consumptions (leave_request_id);
CREATE INDEX leave_consumptions_grant_idx   ON leave_consumptions (grant_id);

-- The SUBMIT-time FIFO reservation snapshot (openapi BalanceCheck.allocation +
-- earmark). Stored on the request so REJECT/CANCEL release the EXACT lots that were
-- reserved (pending_days on a lot is not request-linked), and so the
-- balance_check.allocation[] DTO renders without re-deriving. jsonb array of
-- {grant_id, days, expires_at}. Null until the first reserve.
ALTER TABLE leave_requests ADD COLUMN balance_earmark    text;
ALTER TABLE leave_requests ADD COLUMN balance_allocation jsonb;

-- ────────────────────────────────────────────────────────────────────────────────
--  BACKFILL  (E9-style transform; runs in the migration tx)
-- ────────────────────────────────────────────────────────────────────────────────
-- 1) Each existing leave_quotas row → one leave_grants lot. Mapping:
--      source         = 'ANNUAL' for the annual type (SWP-LT-001), else 'MIGRATION'
--      amount_days    = total
--      consumed_days  = used        (set DIRECTLY so the lot is consistent with the
--                                     leave_consumptions rows written in step 2)
--      pending_days   = pending
--      granted_at     = created_at
--      effective_from = period_start
--      expires_at     = period_end
--      earmark        = NULL        (legacy quotas were the flat pool)
--      remark         = 'backfill from <quota id>'
-- A deterministic lot id is derived from the quota id ('SWP-LG-MIG-' || <quota id>)
-- so the per-quota → per-lot mapping is stable and step 2's FIFO attribution is exact
-- (one lot per quota; a request that deducted that quota draws that lot).
INSERT INTO leave_grants (
    id, employee_id, amount_days, granted_at, effective_from, expires_at,
    source, earmark, remark, consumed_days, pending_days, created_by, created_at, updated_at
)
SELECT
    'SWP-LG-MIG-' || lq.id,
    lq.employee_id,
    GREATEST(lq.total, 0),
    lq.created_at,
    lq.period_start,
    lq.period_end,
    CASE WHEN lq.leave_type_id = 'SWP-LT-001' THEN 'ANNUAL' ELSE 'MIGRATION' END,
    NULL,
    'backfill from ' || lq.id,
    GREATEST(LEAST(lq.used, lq.total), 0),   -- clamp: consumed cannot exceed amount
    GREATEST(LEAST(lq.pending, lq.total - GREATEST(LEAST(lq.used, lq.total), 0)), 0),
    'system-migration',
    lq.created_at,
    now()
FROM leave_quotas lq
ON CONFLICT (id) DO NOTHING;

-- 2) Each APPROVED leave_request that deducted a quota → a leave_consumptions row
-- against that employee's backfilled lot for the same leave_type+period. Attribution
-- is EXACT (not approximate): the request's leave_type_id + start_date's year pins
-- the single quota → single lot it drew. days = LEAST(request.duration_days,
-- lot.consumed_days) so the sum of consumption rows per lot stays == lot.consumed_days
-- even if (legacy) multiple approved requests share a lot — they are attributed in
-- created_at order, draining the lot's consumed_days, and any remainder once the lot
-- is exhausted is skipped (legacy data drift, documented).
WITH approved AS (
    SELECT lr.id AS request_id, lr.duration_days, lr.created_at,
           'SWP-LG-MIG-' || lq.id AS grant_id,
           lg.consumed_days AS lot_consumed
    FROM leave_requests lr
    JOIN leave_quotas lq
      ON lq.employee_id   = lr.employee_id
     AND lq.leave_type_id = lr.leave_type_id
     AND lq.period        = EXTRACT(YEAR FROM lr.start_date)::int
    JOIN leave_grants lg
      ON lg.id = 'SWP-LG-MIG-' || lq.id
    WHERE lr.status = 'APPROVED'
      AND lr.deleted_at IS NULL
      AND lr.duration_days > 0
),
attributed AS (
    -- Running sum per lot in created_at order; only attribute up to lot_consumed so
    -- Σ days per lot == lot.consumed_days exactly.
    SELECT request_id, grant_id, duration_days, lot_consumed,
           COALESCE(SUM(duration_days) OVER (
               PARTITION BY grant_id ORDER BY created_at, request_id
               ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING), 0) AS prior_sum
    FROM approved
)
INSERT INTO leave_consumptions (leave_request_id, grant_id, days)
SELECT request_id, grant_id,
       LEAST(duration_days, GREATEST(lot_consumed - prior_sum, 0))
FROM attributed
WHERE GREATEST(lot_consumed - prior_sum, 0) > 0;

-- +goose Down
ALTER TABLE leave_requests DROP COLUMN IF EXISTS balance_allocation;
ALTER TABLE leave_requests DROP COLUMN IF EXISTS balance_earmark;
DROP TABLE IF EXISTS leave_consumptions;
DROP TABLE IF EXISTS leave_grants;
