-- name: ListChangeRequests :many
-- Cursor page ordered by (submitted_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: status, employee_id, request_type.
SELECT id, employee_id, status, changes, request_type, note,
       submitted_at, resolved_at, resolved_by, rejection_reason
FROM change_requests
WHERE (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
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
          submitted_at, resolved_at, resolved_by, rejection_reason;

-- name: ResolveChangeRequest :one
-- Drives :approve (status='approved') and :reject (status='rejected').
-- Sets resolved_at, resolved_by (optional), and rejection_reason (on reject).
UPDATE change_requests
SET status           = sqlc.arg(status),
    resolved_at      = sqlc.arg(resolved_at),
    resolved_by      = sqlc.narg(resolved_by),
    rejection_reason = sqlc.narg(rejection_reason)
WHERE id = sqlc.arg(id)
RETURNING id, employee_id, status, changes, request_type, note,
          submitted_at, resolved_at, resolved_by, rejection_reason;
