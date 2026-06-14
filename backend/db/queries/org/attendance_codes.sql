-- name: ListAttendanceCodes :many
-- Cursor page ordered by (created_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: status, is_billable.
SELECT id, code, label, description, color, is_workday, is_paid,
       is_billable, needs_verification, status, created_at, updated_at
FROM attendance_codes
WHERE deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(is_billable)::boolean IS NULL OR is_billable = sqlc.narg(is_billable)::boolean)
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetAttendanceCodeByID :one
SELECT id, code, label, description, color, is_workday, is_paid,
       is_billable, needs_verification, status, created_at, updated_at
FROM attendance_codes
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: CreateAttendanceCode :one
-- Allocates the SWP-AC id inline from the per-prefix sequence.
INSERT INTO attendance_codes (id, code, label, description, color, is_workday, is_paid,
                              is_billable, needs_verification)
VALUES (
    'SWP-AC-' || swp_next_id('AC'),
    sqlc.arg(code),
    sqlc.arg(label),
    sqlc.arg(description),
    sqlc.arg(color),
    sqlc.arg(is_workday),
    sqlc.arg(is_paid),
    sqlc.arg(is_billable),
    sqlc.arg(needs_verification)
)
RETURNING id, code, label, description, color, is_workday, is_paid,
          is_billable, needs_verification, status, created_at, updated_at;

-- name: UpdateAttendanceCode :one
UPDATE attendance_codes
SET code               = sqlc.arg(code),
    label              = sqlc.arg(label),
    description        = sqlc.arg(description),
    color              = sqlc.arg(color),
    is_workday         = sqlc.arg(is_workday),
    is_paid            = sqlc.arg(is_paid),
    is_billable        = sqlc.arg(is_billable),
    needs_verification = sqlc.arg(needs_verification),
    updated_at         = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, code, label, description, color, is_workday, is_paid,
          is_billable, needs_verification, status, created_at, updated_at;

-- name: SetAttendanceCodeStatus :one
-- Drives :deactivate (status='inactive') and :reactivate (status='active').
UPDATE attendance_codes
SET status     = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, code, label, description, color, is_workday, is_paid,
          is_billable, needs_verification, status, created_at, updated_at;

-- name: SoftDeleteAttendanceCode :exec
UPDATE attendance_codes
SET deleted_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
