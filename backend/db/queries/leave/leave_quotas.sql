-- E6 per-type leave-quota window queries (F6.1 LQ-13 / SWP-LQ-*). The cap_basis
-- ledger meters per (employee, leave_type, period_key) window: reserve at submit,
-- commit at approve, release at reject/cancel; remaining = entitled-used-pending is a
-- DERIVED domain method. no-negative enforced by the entitled/used/pending CHECKs.
-- last_adjustment / last_override are jsonb → []byte in sqlc.

-- name: ResolveQuotaWindow :one
-- Row-locked lookup of the per-type window for reserve/commit/release.
SELECT lq.*
FROM leave_quotas lq
WHERE lq.employee_id   = sqlc.arg(employee_id)
  AND lq.leave_type_id = sqlc.arg(leave_type_id)
  AND lq.period_key    = sqlc.arg(period_key)
FOR UPDATE;

-- name: OpenQuotaWindow :one
-- Auto-open a per-type window at its entitlement (annual: agreement days; other
-- quota-bearing: cap_value). Idempotent on the (emp,type,period_key) unique index.
INSERT INTO leave_quotas (
    employee_id, leave_type_id, period_key,
    entitled_days, source, remark, expires_at, created_by
) VALUES (
    sqlc.arg(employee_id),
    sqlc.arg(leave_type_id),
    sqlc.arg(period_key),
    sqlc.arg(entitled_days),
    sqlc.arg(source),
    sqlc.arg(remark),
    sqlc.narg(expires_at),
    sqlc.narg(created_by)
)
ON CONFLICT (employee_id, leave_type_id, period_key) DO UPDATE
SET entitled_days = EXCLUDED.entitled_days,
    expires_at    = EXCLUDED.expires_at,
    updated_at    = now()
RETURNING *;

-- name: ReserveQuotaDays :one
-- Submit: hold pending_days on the window (remaining must cover it — service guards).
UPDATE leave_quotas
SET pending_days = pending_days + sqlc.arg(delta),
    updated_at   = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: CommitQuotaDays :one
-- Final approval: move the reservation pending_days -> used_days.
UPDATE leave_quotas
SET pending_days = GREATEST(pending_days - sqlc.arg(delta), 0),
    used_days    = used_days + sqlc.arg(delta),
    updated_at   = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ReleaseQuotaDays :one
-- Reject/withdraw: release the held reservation.
UPDATE leave_quotas
SET pending_days = GREATEST(pending_days - sqlc.arg(delta), 0),
    updated_at   = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ReverseCommittedQuotaDays :one
-- Cancel/shorten an APPROVED leave: return committed days to the balance.
UPDATE leave_quotas
SET used_days  = GREATEST(used_days - sqlc.arg(delta), 0),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: AdjustQuotaEntitled :one
-- HR per-type quota adjust (LQ-6): signed delta on entitled_days; service guards
-- entitled cannot drop below used+pending and records last_adjustment.
UPDATE leave_quotas
SET entitled_days   = entitled_days + sqlc.arg(delta),
    source          = 'ADJUSTMENT',
    remark          = sqlc.arg(remark),
    last_adjustment = sqlc.arg(last_adjustment),
    updated_at      = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: CountApprovedRequestsForType :one
-- Occurrence/lifetime gate source (PER_YEAR_COUNT, LIFETIME_ONCE): how many
-- non-terminal-rejected requests of this type the employee already has in the
-- window [from,to]. Counts PENDING_*/APPROVED (reserved or committed).
SELECT count(*)
FROM leave_requests lr
WHERE lr.employee_id   = sqlc.arg(employee_id)
  AND lr.leave_type_id = sqlc.arg(leave_type_id)
  AND lr.status IN ('PENDING_L1','PENDING_HR','APPROVED')
  AND lr.start_date >= sqlc.arg(window_start)::date
  AND lr.start_date <= sqlc.arg(window_end)::date
  AND lr.deleted_at IS NULL;
