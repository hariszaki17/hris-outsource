-- name: ListChangeRequests :many
-- Cursor page ordered by (submitted_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: status, employee_id, request_type.
SELECT id, employee_id, status, changes, request_type, note,
       field_resolutions, bank_pending,
       submitted_at, resolved_at, resolved_by, rejection_reason
FROM change_requests
WHERE (
        sqlc.narg(status)::text IS NULL
        OR status = sqlc.narg(status)::text
        -- The approval queue passes status='pending'; a partially-approved request
        -- (non-bank applied by the SL, bank escalated to HR) still needs HR action,
        -- so it must stay visible in the queue until fully approved.
        OR (sqlc.narg(status)::text = 'pending' AND status = 'partially_approved')
      )
  AND (sqlc.narg(employee_id)::text IS NULL OR employee_id = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(request_type)::text IS NULL OR request_type = sqlc.narg(request_type)::text)
  AND (
        sqlc.narg(cursor_submitted_at)::timestamptz IS NULL
        OR (submitted_at, id) < (sqlc.narg(cursor_submitted_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY submitted_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetChangeRequestByID :one
SELECT id, employee_id, status, changes, request_type, note,
       field_resolutions, bank_pending,
       submitted_at, resolved_at, resolved_by, rejection_reason
FROM change_requests
WHERE id = sqlc.arg(id);

-- name: CreateChangeRequest :one
-- Allocates the SWP-CHG id inline from the per-prefix sequence.
INSERT INTO change_requests (
    id, employee_id, changes, request_type, note
) VALUES (
    'SWP-CHG-' || swp_next_id('CHG'),
    sqlc.arg(employee_id),
    sqlc.arg(changes),
    sqlc.arg(request_type),
    sqlc.narg(note)
)
RETURNING id, employee_id, status, changes, request_type, note,
          field_resolutions, bank_pending,
          submitted_at, resolved_at, resolved_by, rejection_reason;

-- name: ResolveChangeRequest :one
-- Drives :approve (status='approved'), :reject (status='rejected'), and the
-- SL bank-split partial apply (status='partially_approved' + bank_pending=true,
-- field_resolutions recording which non-bank fields the SL applied + who). For
-- terminal resolutions resolved_at/resolved_by/rejection_reason are set; for a
-- partial apply field_resolutions/bank_pending carry the in-between state.
UPDATE change_requests
SET status            = sqlc.arg(status),
    field_resolutions = sqlc.arg(field_resolutions),
    bank_pending      = sqlc.arg(bank_pending),
    resolved_at       = sqlc.narg(resolved_at),
    resolved_by       = sqlc.narg(resolved_by),
    rejection_reason  = sqlc.narg(rejection_reason)
WHERE id = sqlc.arg(id)
RETURNING id, employee_id, status, changes, request_type, note,
          field_resolutions, bank_pending,
          submitted_at, resolved_at, resolved_by, rejection_reason;

-- name: ListBankPendingChangeRequests :many
-- HR bank-escalation queue: rows whose bank change a shift leader partially
-- applied and escalated. Backed by the change_requests_bank_pending_idx partial
-- index; cursor page ordered by (submitted_at desc, id desc), fetch limit+1.
SELECT id, employee_id, status, changes, request_type, note,
       field_resolutions, bank_pending,
       submitted_at, resolved_at, resolved_by, rejection_reason
FROM change_requests
WHERE bank_pending
  AND (sqlc.narg(employee_id)::text IS NULL OR employee_id = sqlc.narg(employee_id)::text)
  AND (
        sqlc.narg(cursor_submitted_at)::timestamptz IS NULL
        OR (submitted_at, id) < (sqlc.narg(cursor_submitted_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY submitted_at DESC, id DESC
LIMIT sqlc.arg(row_limit);
