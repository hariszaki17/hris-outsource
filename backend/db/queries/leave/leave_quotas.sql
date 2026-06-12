-- E6 leave-quota queries (F6.3 / SWP-LQ-*). remaining = total-used-pending is a
-- DERIVED domain method (not a column). Reads LEFT JOIN employees/leave_types for
-- denormalized names. Dates come back as pgtype.Date (08-02 repo converts).
-- last_adjustment / last_override are jsonb → []byte in sqlc (08-02 marshals).

-- name: ListLeaveQuotas :many
-- Keyset cursor (created_at,id) DESC. Filters (narg): employee_id, leave_type_id,
-- period, company_id (via the employee's covering placement), include_closed.
SELECT lq.id, lq.employee_id, lq.leave_type_id, lq.period, lq.period_start, lq.period_end,
       lq.total, lq.used, lq.pending, lq.is_prorated, lq.prorate_months, lq.closed,
       lq.last_adjustment, lq.last_override, lq.created_at, lq.updated_at,
       e.full_name AS employee_name,
       lt.name     AS leave_type_name,
       lt.code     AS leave_type_code
FROM leave_quotas lq
LEFT JOIN employees e    ON e.id  = lq.employee_id
LEFT JOIN leave_types lt ON lt.id = lq.leave_type_id
WHERE (sqlc.narg(employee_id)::text   IS NULL OR lq.employee_id   = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(leave_type_id)::text IS NULL OR lq.leave_type_id = sqlc.narg(leave_type_id)::text)
  AND (sqlc.narg(period)::int         IS NULL OR lq.period        = sqlc.narg(period)::int)
  AND (sqlc.narg(include_closed)::bool IS TRUE OR lq.closed = false)
  AND (sqlc.narg(company_id)::text IS NULL OR EXISTS (
        SELECT 1 FROM placements p
        WHERE p.employee_id = lq.employee_id
          AND p.client_company_id = sqlc.narg(company_id)::text
          AND p.deleted_at IS NULL))
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL OR
       (lq.created_at, lq.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY lq.created_at DESC, lq.id DESC
LIMIT sqlc.arg(lim);

-- name: GetLeaveQuota :one
SELECT lq.id, lq.employee_id, lq.leave_type_id, lq.period, lq.period_start, lq.period_end,
       lq.total, lq.used, lq.pending, lq.is_prorated, lq.prorate_months, lq.closed,
       lq.last_adjustment, lq.last_override, lq.created_at, lq.updated_at,
       e.full_name AS employee_name,
       lt.name     AS leave_type_name,
       lt.code     AS leave_type_code
FROM leave_quotas lq
LEFT JOIN employees e    ON e.id  = lq.employee_id
LEFT JOIN leave_types lt ON lt.id = lq.leave_type_id
WHERE lq.id = sqlc.arg(id);

-- name: GetLeaveQuotaForUpdate :one
-- Row-lock for :adjust and the final-approval deduct/restore.
SELECT lq.*
FROM leave_quotas lq
WHERE lq.id = sqlc.arg(id)
FOR UPDATE;

-- name: FindQuotaForEmployeeTypePeriod :one
-- INV-1 quota guard lookup by (employee_id, leave_type_id, period).
SELECT lq.*
FROM leave_quotas lq
WHERE lq.employee_id   = sqlc.arg(employee_id)
  AND lq.leave_type_id = sqlc.arg(leave_type_id)
  AND lq.period        = sqlc.arg(period);

-- name: UpsertLeaveQuota :one
-- Bulk-grant: insert or update entitlement total for a (employee,type,period).
-- DOES NOT overwrite used/pending. id allocated by column DEFAULT when inserting.
INSERT INTO leave_quotas (
    employee_id, leave_type_id, period, period_start, period_end,
    total, is_prorated, prorate_months
) VALUES (
    sqlc.arg(employee_id),
    sqlc.arg(leave_type_id),
    sqlc.arg(period),
    sqlc.arg(period_start),
    sqlc.arg(period_end),
    sqlc.arg(total),
    sqlc.arg(is_prorated),
    sqlc.arg(prorate_months)
)
ON CONFLICT (employee_id, leave_type_id, period) DO UPDATE
SET total          = EXCLUDED.total,
    is_prorated    = EXCLUDED.is_prorated,
    prorate_months = EXCLUDED.prorate_months,
    updated_at     = now()
RETURNING *;

-- name: AdjustLeaveQuotaTotal :one
-- :adjust — signed delta on total + audited last_adjustment snapshot. Service
-- guards delta cannot drop total below used (422 RULE_VIOLATION) before calling.
UPDATE leave_quotas
SET total           = total + sqlc.arg(delta),
    last_adjustment = sqlc.arg(last_adjustment),
    updated_at      = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeductLeaveQuota :one
-- Final-approval deduct: move days from the soft-reservation into used.
UPDATE leave_quotas
SET used       = used + sqlc.arg(delta),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: RestoreLeaveQuota :one
-- Cancel/withdraw restore: return days to the balance (used - delta).
UPDATE leave_quotas
SET used       = GREATEST(used - sqlc.arg(delta), 0),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetLeaveQuotaOverride :one
-- HR override that drove remaining negative (LA-8): records last_override.
UPDATE leave_quotas
SET last_override = sqlc.arg(last_override),
    updated_at    = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- ============================================================================
-- Per-type ledger (2026-06-12, EPICS §8). New live path: meter per (employee,
-- leave_type, period_key) window. Reserve at submit, commit at approve, release
-- at reject/cancel; no-negative enforced by the entitled/used/pending CHECKs.
-- Legacy period/period_start/period_end are still supplied (NOT NULL until 00052+
-- drops them); period_key is the authoritative window key.
-- ============================================================================

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
    period, period_start, period_end,
    entitled_days, source, remark, expires_at, created_by,
    total
) VALUES (
    sqlc.arg(employee_id),
    sqlc.arg(leave_type_id),
    sqlc.arg(period_key),
    sqlc.arg(period),
    sqlc.arg(period_start),
    sqlc.arg(period_end),
    sqlc.arg(entitled_days),
    sqlc.arg(source),
    sqlc.arg(remark),
    sqlc.narg(expires_at),
    sqlc.narg(created_by),
    sqlc.arg(entitled_days)
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

-- name: ListActivePlacedEmployeesForGrant :many
-- bulk-grant employee_ids:["all"] sentinel + pro-rate join-date source: employees
-- with an ACTIVE/EXPIRING placement covering any day of the [period_start,period_end].
SELECT DISTINCT e.id AS employee_id,
       e.full_name   AS employee_name,
       p.start_date  AS placement_start_date
FROM employees e
JOIN placements p ON p.employee_id = e.id
WHERE p.lifecycle_status IN ('ACTIVE','EXPIRING')
  AND p.deleted_at IS NULL
  AND e.deleted_at IS NULL
  AND p.start_date <= sqlc.arg(period_end)::date
  AND (p.end_date IS NULL OR p.end_date >= sqlc.arg(period_start)::date)
ORDER BY e.id ASC;
