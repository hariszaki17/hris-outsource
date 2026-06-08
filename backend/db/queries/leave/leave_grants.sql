-- E6 leave-grant ledger queries (F6.1 / SWP-LG-* / SWP-LC-*). One lot per
-- leave_grants row; remaining_days = amount_days - consumed_days - pending_days is a
-- DERIVED domain method (not a column). FIFO allocation orders by soonest expires_at,
-- then granted_at, then id. Dates come back as pgtype.Date (repo converts).

-- name: CreateLeaveGrant :one
-- HR POST /leave-grants — inserts one lot. id allocated by column DEFAULT when omitted.
INSERT INTO leave_grants (
    employee_id, amount_days, granted_at, effective_from, expires_at,
    source, earmark, remark, created_by
) VALUES (
    sqlc.arg(employee_id),
    sqlc.arg(amount_days),
    now(),
    sqlc.arg(effective_from),
    sqlc.arg(expires_at),
    sqlc.arg(source),
    sqlc.narg(earmark),
    sqlc.narg(remark),
    sqlc.narg(created_by)
)
RETURNING id, employee_id, amount_days, granted_at, effective_from, expires_at,
          source, earmark, remark, consumed_days, pending_days, created_by,
          created_at, updated_at;

-- name: CreateLeaveGrantWithID :one
-- Seed / migration-repair variant that supplies an explicit id.
INSERT INTO leave_grants (
    id, employee_id, amount_days, granted_at, effective_from, expires_at,
    source, earmark, remark, consumed_days, pending_days, created_by
) VALUES (
    sqlc.arg(id),
    sqlc.arg(employee_id),
    sqlc.arg(amount_days),
    now(),
    sqlc.arg(effective_from),
    sqlc.arg(expires_at),
    sqlc.arg(source),
    sqlc.narg(earmark),
    sqlc.narg(remark),
    sqlc.arg(consumed_days),
    sqlc.arg(pending_days),
    sqlc.narg(created_by)
)
ON CONFLICT (id) DO NOTHING
RETURNING id, employee_id, amount_days, granted_at, effective_from, expires_at,
          source, earmark, remark, consumed_days, pending_days, created_by,
          created_at, updated_at;

-- name: GetLeaveGrant :one
-- Single lot with denormalized employee_name.
SELECT lg.id, lg.employee_id, lg.amount_days, lg.granted_at, lg.effective_from,
       lg.expires_at, lg.source, lg.earmark, lg.remark, lg.consumed_days,
       lg.pending_days, lg.created_by, lg.created_at, lg.updated_at,
       e.full_name AS employee_name
FROM leave_grants lg
LEFT JOIN employees e ON e.id = lg.employee_id
WHERE lg.id = sqlc.arg(id)
  AND lg.deleted_at IS NULL;

-- name: GetLeaveGrantForUpdate :one
-- Row-lock for PATCH (adjust amount/expires_at/earmark).
SELECT lg.id, lg.employee_id, lg.amount_days, lg.granted_at, lg.effective_from,
       lg.expires_at, lg.source, lg.earmark, lg.remark, lg.consumed_days,
       lg.pending_days, lg.created_by, lg.created_at, lg.updated_at
FROM leave_grants lg
WHERE lg.id = sqlc.arg(id)
  AND lg.deleted_at IS NULL
FOR UPDATE;

-- name: ListLeaveGrants :many
-- The ledger. Keyset cursor (expires_at ASC, id) — FIFO-aligned order. Filters
-- (narg): employee_id, earmark ('__null' sentinel → unearmarked only), source,
-- include_expired (default: active only), company_id (via covering placement).
SELECT lg.id, lg.employee_id, lg.amount_days, lg.granted_at, lg.effective_from,
       lg.expires_at, lg.source, lg.earmark, lg.remark, lg.consumed_days,
       lg.pending_days, lg.created_by, lg.created_at, lg.updated_at,
       e.full_name AS employee_name
FROM leave_grants lg
LEFT JOIN employees e ON e.id = lg.employee_id
WHERE lg.deleted_at IS NULL
  AND (sqlc.narg(employee_id)::text IS NULL OR lg.employee_id = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(source)::text     IS NULL OR lg.source      = sqlc.narg(source)::text)
  AND (sqlc.narg(earmark_filter)::text IS NULL
       OR (sqlc.narg(earmark_filter)::text = '__null' AND lg.earmark IS NULL)
       OR (sqlc.narg(earmark_filter)::text <> '__null' AND lg.earmark = sqlc.narg(earmark_filter)::text))
  AND (sqlc.narg(include_expired)::bool IS TRUE OR lg.expires_at > sqlc.arg(now_date)::date)
  AND (sqlc.narg(company_id)::text IS NULL OR EXISTS (
        SELECT 1 FROM placements p
        WHERE p.employee_id = lg.employee_id
          AND p.client_company_id = sqlc.narg(company_id)::text
          AND p.deleted_at IS NULL))
  AND (sqlc.narg(cursor_expires_at)::date IS NULL OR
       (lg.expires_at, lg.id) > (sqlc.narg(cursor_expires_at)::date, sqlc.narg(cursor_id)::text))
ORDER BY lg.expires_at ASC, lg.id ASC
LIMIT sqlc.arg(lim);

-- name: GetActiveLotsForAllocation :many
-- FIFO allocation source. Returns the employee's ACTIVE lots (now < expires_at,
-- effective_from <= now) matching the request's earmark: ordinary requests draw ONLY
-- unearmarked lots (earmark_match='__null'); an earmarked request draws ONLY lots with
-- the matching earmark (LQ-10 isolation). Ordered FIFO (expires_at, granted_at, id).
-- FOR UPDATE locks the lots for the reserve/commit/release tx.
SELECT lg.id, lg.employee_id, lg.amount_days, lg.granted_at, lg.effective_from,
       lg.expires_at, lg.source, lg.earmark, lg.remark, lg.consumed_days,
       lg.pending_days, lg.created_by, lg.created_at, lg.updated_at
FROM leave_grants lg
WHERE lg.employee_id = sqlc.arg(employee_id)
  AND lg.deleted_at IS NULL
  AND lg.expires_at > sqlc.arg(now_date)::date
  AND lg.effective_from <= sqlc.arg(now_date)::date
  AND ((sqlc.arg(earmark_match)::text = '__null' AND lg.earmark IS NULL)
       OR (sqlc.arg(earmark_match)::text <> '__null' AND lg.earmark = sqlc.arg(earmark_match)::text))
ORDER BY lg.expires_at ASC, lg.granted_at ASC, lg.id ASC
FOR UPDATE;

-- name: ReservePending :one
-- Move days into pending_days at SUBMIT (FIFO reserve). Never drives a lot negative:
-- the service caps the delta at the lot's remaining before calling.
UPDATE leave_grants
SET pending_days = pending_days + sqlc.arg(days),
    updated_at   = now()
WHERE id = sqlc.arg(id)
RETURNING id, employee_id, amount_days, granted_at, effective_from, expires_at,
          source, earmark, remark, consumed_days, pending_days, created_by,
          created_at, updated_at;

-- name: CommitReservation :one
-- Move days pending_days → consumed_days at APPROVE (commit). Paired with an
-- ApplyConsumption (leave_consumptions) insert in the same tx.
UPDATE leave_grants
SET pending_days  = GREATEST(pending_days - sqlc.arg(days), 0),
    consumed_days = consumed_days + sqlc.arg(days),
    updated_at    = now()
WHERE id = sqlc.arg(id)
RETURNING id, employee_id, amount_days, granted_at, effective_from, expires_at,
          source, earmark, remark, consumed_days, pending_days, created_by,
          created_at, updated_at;

-- name: ReleasePending :one
-- Release pending_days at REJECT/CANCEL (a still-pending request never consumed).
UPDATE leave_grants
SET pending_days = GREATEST(pending_days - sqlc.arg(days), 0),
    updated_at   = now()
WHERE id = sqlc.arg(id)
RETURNING id, employee_id, amount_days, granted_at, effective_from, expires_at,
          source, earmark, remark, consumed_days, pending_days, created_by,
          created_at, updated_at;

-- name: ReverseConsumption :one
-- Reverse committed consumed_days at CANCEL-APPROVED / SHORTEN (paired with deleting
-- the leave_consumptions rows). Returns days to the lot.
UPDATE leave_grants
SET consumed_days = GREATEST(consumed_days - sqlc.arg(days), 0),
    updated_at    = now()
WHERE id = sqlc.arg(id)
RETURNING id, employee_id, amount_days, granted_at, effective_from, expires_at,
          source, earmark, remark, consumed_days, pending_days, created_by,
          created_at, updated_at;

-- name: PatchLeaveGrant :one
-- HR PATCH /leave-grants/{id}: adjust amount_days/expires_at/earmark (+ remark). The
-- service guards amount_days >= consumed_days+pending_days (422) before calling. nargs
-- left NULL keep the current value (COALESCE).
UPDATE leave_grants
SET amount_days = COALESCE(sqlc.narg(amount_days), amount_days),
    expires_at  = COALESCE(sqlc.narg(expires_at), expires_at),
    earmark     = CASE WHEN sqlc.arg(set_earmark)::bool THEN sqlc.narg(earmark) ELSE earmark END,
    remark      = sqlc.arg(remark),
    updated_at  = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, employee_id, amount_days, granted_at, effective_from, expires_at,
          source, earmark, remark, consumed_days, pending_days, created_by,
          created_at, updated_at;

-- name: ListConsumptionsForGrant :many
-- GET /leave-grants/{id} consumptions[] embed.
SELECT lc.id, lc.leave_request_id, lc.grant_id, lc.days, lc.created_at
FROM leave_consumptions lc
WHERE lc.grant_id = sqlc.arg(grant_id)
ORDER BY lc.created_at ASC, lc.id ASC;

-- name: ListConsumptionsForRequest :many
-- Reversal source (cancel/shorten): the exact lots a request drew, to reverse.
SELECT lc.id, lc.leave_request_id, lc.grant_id, lc.days, lc.created_at
FROM leave_consumptions lc
WHERE lc.leave_request_id = sqlc.arg(leave_request_id)
ORDER BY lc.created_at ASC, lc.id ASC;

-- name: ApplyConsumption :one
-- Write one lot-drawdown row at APPROVE (one per lot the request drew). id allocated
-- by column DEFAULT.
INSERT INTO leave_consumptions (leave_request_id, grant_id, days)
VALUES (sqlc.arg(leave_request_id), sqlc.arg(grant_id), sqlc.arg(days))
RETURNING id, leave_request_id, grant_id, days, created_at;

-- name: DeleteConsumptionsForRequest :exec
-- Reversal: drop the request's consumption rows after reversing each lot's consumed.
DELETE FROM leave_consumptions WHERE leave_request_id = sqlc.arg(leave_request_id);

-- name: SumActiveBalanceByEarmark :many
-- Computed balance over ACTIVE lots (now < expires_at), grouped by earmark. The
-- unearmarked group (earmark IS NULL) is the flat pool; each non-null earmark is one
-- per-purpose line. Returns Σ(amount-consumed-pending) and the soonest expiry per group.
SELECT lg.earmark,
       COALESCE(SUM(lg.amount_days - lg.consumed_days - lg.pending_days), 0)::bigint AS remaining_days,
       COALESCE(SUM(lg.pending_days), 0)::bigint AS pending_days,
       MIN(lg.expires_at)::date AS next_expiry
FROM leave_grants lg
WHERE lg.employee_id = sqlc.arg(employee_id)
  AND lg.deleted_at IS NULL
  AND lg.expires_at > sqlc.arg(now_date)::date
GROUP BY lg.earmark;

-- name: ExpireLeaveLots :many
-- Expiry sweep: find lots whose expires_at < today that still hold pending_days
-- (dangling reservations on an expired lot) so the sweep can release them, and report
-- the lot for the audit/zeroing. We do NOT mutate amount/consumed — remaining is
-- DERIVED and already 0 for an inactive lot (now >= expires_at) at the read boundary.
-- We DO zero any leftover pending_days (releasing dangling reservations). FOR UPDATE
-- locks the batch.
SELECT lg.id, lg.employee_id, lg.expires_at, lg.pending_days
FROM leave_grants lg
WHERE lg.deleted_at IS NULL
  AND lg.expires_at < sqlc.arg(today)::date
  AND lg.pending_days > 0
ORDER BY lg.expires_at ASC, lg.id ASC
LIMIT sqlc.arg(lim)
FOR UPDATE;

-- name: ZeroLotPending :one
-- Expiry sweep: release dangling pending on an expired lot.
UPDATE leave_grants
SET pending_days = 0,
    updated_at   = now()
WHERE id = sqlc.arg(id)
RETURNING id, employee_id, amount_days, granted_at, effective_from, expires_at,
          source, earmark, remark, consumed_days, pending_days, created_by,
          created_at, updated_at;

-- name: ListLeaveBalances :many
-- The /leave/quotas screen: ONE ROW PER EMPLOYEE, aggregating ALL of the employee's
-- ACTIVE lots (now < expires_at AND deleted_at IS NULL). JOINs employees for the
-- name/nik/nip display + the q ILIKE filter (mirrors people/employees.sql ListEmployees:
-- ILIKE over full_name/nik/nip ONLY). An employee appears iff they hold >= 1 ACTIVE lot
-- regardless of remaining (an employee whose lots are all consumed still lists as long
-- as a lot is non-expired); an employee with ONLY expired lots is excluded by the
-- expires_at > now_date predicate. Pool fields aggregate unearmarked lots (earmark IS
-- NULL); earmarked_remaining sums remaining across earmarked lots. next_expiry is the
-- MIN(expires_at) over active lots that still have remaining > 0. Keyset cursor on
-- (full_name, employee_id); deterministic ORDER BY full_name ASC, employee_id ASC.
SELECT e.id                                                         AS employee_id,
       e.full_name                                                  AS full_name,
       e.nik                                                        AS nik,
       e.nip                                                        AS nip,
       COALESCE(SUM(lg.amount_days)   FILTER (WHERE lg.earmark IS NULL), 0)::bigint AS pool_total,
       COALESCE(SUM(lg.consumed_days) FILTER (WHERE lg.earmark IS NULL), 0)::bigint AS pool_consumed,
       COALESCE(SUM(lg.pending_days)  FILTER (WHERE lg.earmark IS NULL), 0)::bigint AS pool_pending,
       COALESCE(SUM(lg.amount_days - lg.consumed_days - lg.pending_days)
                FILTER (WHERE lg.earmark IS NULL), 0)::bigint        AS pool_remaining,
       COALESCE(SUM(lg.amount_days - lg.consumed_days - lg.pending_days)
                FILTER (WHERE lg.earmark IS NOT NULL), 0)::bigint    AS earmarked_remaining,
       MIN(lg.expires_at) FILTER (
           WHERE (lg.amount_days - lg.consumed_days - lg.pending_days) > 0
       )::date                                                       AS next_expiry,
       COUNT(*)::bigint                                              AS lot_count
FROM leave_grants lg
JOIN employees e ON e.id = lg.employee_id AND e.deleted_at IS NULL
WHERE lg.deleted_at IS NULL
  AND lg.expires_at > sqlc.arg(now_date)::date
  AND (
        sqlc.narg(q)::text IS NULL
        OR e.full_name ILIKE '%' || sqlc.narg(q)::text || '%'
        OR e.nik       ILIKE '%' || sqlc.narg(q)::text || '%'
        OR e.nip       ILIKE '%' || sqlc.narg(q)::text || '%'
      )
  AND (
        sqlc.narg(cursor_full_name)::text IS NULL
        OR (e.full_name, e.id) > (sqlc.narg(cursor_full_name)::text, sqlc.narg(cursor_id)::text)
      )
GROUP BY e.id, e.full_name, e.nik, e.nip
ORDER BY e.full_name ASC, e.id ASC
LIMIT sqlc.arg(lim);
