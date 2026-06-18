-- E5 attendance activity log queries (F5.8 / SWP-ACT-*). An agent logs free-text activity
-- notes onto their own OPEN attendance record; agent clock-out is gated on >=1 non-deleted
-- activity (INV-7 / AA-7). recorded_at is server-set (DB default now()). `make gen` writes
-- internal/repository/sqlc (NEVER hand-edit).

-- name: CreateActivity :one
-- Insert one activity. id + recorded_at + created_at default server-side. Open-record and
-- scope:self checks are enforced in the service before calling this.
INSERT INTO attendance_activities (attendance_id, employee_id, note)
VALUES (sqlc.arg(attendance_id), sqlc.arg(employee_id), sqlc.arg(note))
RETURNING id, attendance_id, employee_id, note, recorded_at, created_at;

-- name: GetActivity :one
-- One live activity by id — used for the delete ownership/while-open check.
SELECT id, attendance_id, employee_id, note, recorded_at, created_at
FROM attendance_activities
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: ListActivitiesByAttendance :many
-- Chronological (recorded_at asc, id asc) live activities for one attendance record.
-- Keyset cursor: pass cursor_recorded_at + cursor_id from the previous page tail.
SELECT id, attendance_id, employee_id, note, recorded_at, created_at
FROM attendance_activities
WHERE attendance_id = sqlc.arg(attendance_id)
  AND deleted_at IS NULL
  AND (
        sqlc.narg(cursor_recorded_at)::timestamptz IS NULL
        OR recorded_at > sqlc.narg(cursor_recorded_at)::timestamptz
        OR (recorded_at = sqlc.narg(cursor_recorded_at)::timestamptz AND id > sqlc.narg(cursor_id)::text)
      )
ORDER BY recorded_at ASC, id ASC
LIMIT sqlc.arg(page_limit);

-- name: CountActivitiesByAttendance :one
-- Count of live activities on a record — the clock-out gate (AA-7). Read inside the
-- clock-out transaction so a racing add/delete resolves on the persisted count (C-12).
SELECT count(*) AS n
FROM attendance_activities
WHERE attendance_id = sqlc.arg(attendance_id)
  AND deleted_at IS NULL;

-- name: SoftDeleteActivity :execrows
-- Soft-delete the creator's own live activity. Returns rows affected (0 → not found /
-- not owner / already deleted → handler maps to 404). Scope:self enforced via employee_id.
UPDATE attendance_activities
SET deleted_at = now()
WHERE id = sqlc.arg(id)
  AND attendance_id = sqlc.arg(attendance_id)
  AND employee_id = sqlc.arg(employee_id)
  AND deleted_at IS NULL;
