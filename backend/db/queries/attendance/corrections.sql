-- E5 corrections queries (F5.3 / SWP-COR-*). Cursor lists keyset on
-- (created_at DESC, id). `make gen` writes internal/repository/sqlc (NEVER hand-edit).

-- name: ListCorrections :many
-- Corrections queue for a company over filters, newest first.
-- Keyset cursor: pass cursor_created_at + cursor_id from the previous page tail.
--   status_in / type_in: text[] = ANY membership.
--   employee_id maps to requester_id.
--   date_from/date_to: bound on attendance_shift_date.
SELECT co.id, co.attendance_id, co.requester_id, co.company_id, co.type,
       co.proposed_check_in_at, co.proposed_check_out_at,
       co.proposed_attendance_code_id, co.reason, co.evidence_file_id, co.status,
       co.decided_by, co.decided_at, co.reject_reason, co.original_snapshot,
       co.attendance_shift_date, co.created_at, co.updated_at,
       e.full_name AS requester_name,
       c.name      AS company_name
FROM attendance_corrections co
LEFT JOIN employees e        ON e.id = co.requester_id
LEFT JOIN client_companies c ON c.id = co.company_id
WHERE co.deleted_at IS NULL
  AND (sqlc.narg(company_id)::text IS NULL OR co.company_id = sqlc.narg(company_id)::text)
  AND (sqlc.narg(employee_id)::text IS NULL OR co.requester_id = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(status_in)::text[] IS NULL OR co.status = ANY(sqlc.narg(status_in)::text[]))
  AND (sqlc.narg(type_in)::text[] IS NULL OR co.type = ANY(sqlc.narg(type_in)::text[]))
  AND (sqlc.narg(date_from)::date IS NULL OR co.attendance_shift_date >= sqlc.narg(date_from)::date)
  AND (sqlc.narg(date_to)::date IS NULL OR co.attendance_shift_date <= sqlc.narg(date_to)::date)
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR co.created_at < sqlc.narg(cursor_created_at)::timestamptz
        OR (co.created_at = sqlc.narg(cursor_created_at)::timestamptz AND co.id < sqlc.narg(cursor_id)::text)
      )
ORDER BY co.created_at DESC, co.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetPendingCorrectionForAttendance :one
-- Active-pending guard for the agent CREATE path (F5.4 / one open correction per
-- attendance): returns the PENDING correction id for a target attendance, if any.
SELECT id
FROM attendance_corrections
WHERE attendance_id = sqlc.arg(attendance_id)
  AND status = 'PENDING'
  AND deleted_at IS NULL
LIMIT 1;

-- name: CreateCorrection :one
-- Insert a new agent/leader-filed correction in PENDING. company_id +
-- attendance_shift_date are denormalized from the target attendance by the service.
INSERT INTO attendance_corrections (
    attendance_id, requester_id, company_id, type,
    proposed_check_in_at, proposed_check_out_at, proposed_attendance_code_id,
    reason, evidence_file_id, attendance_shift_date, status
) VALUES (
    sqlc.arg(attendance_id), sqlc.arg(requester_id), sqlc.arg(company_id), sqlc.arg(type),
    sqlc.narg(proposed_check_in_at), sqlc.narg(proposed_check_out_at), sqlc.narg(proposed_attendance_code_id),
    sqlc.arg(reason), sqlc.narg(evidence_file_id), sqlc.arg(attendance_shift_date), 'PENDING'
)
RETURNING id;

-- name: GetCorrection :one
-- Single correction with denormalized requester/company names.
SELECT co.id, co.attendance_id, co.requester_id, co.company_id, co.type,
       co.proposed_check_in_at, co.proposed_check_out_at,
       co.proposed_attendance_code_id, co.reason, co.evidence_file_id, co.status,
       co.decided_by, co.decided_at, co.reject_reason, co.original_snapshot,
       co.attendance_shift_date, co.created_at, co.updated_at,
       e.full_name AS requester_name,
       c.name      AS company_name
FROM attendance_corrections co
LEFT JOIN employees e        ON e.id = co.requester_id
LEFT JOIN client_companies c ON c.id = co.company_id
WHERE co.id = sqlc.arg(id)
  AND co.deleted_at IS NULL;

-- name: GetCorrectionForUpdate :one
-- Row-lock for approve/reject: reads status/company_id/proposed_* for scope + state
-- guards + apply (omits joins; service re-reads for DTO).
SELECT co.id, co.attendance_id, co.requester_id, co.company_id, co.type,
       co.proposed_check_in_at, co.proposed_check_out_at,
       co.proposed_attendance_code_id, co.reason, co.evidence_file_id, co.status,
       co.decided_by, co.decided_at, co.reject_reason, co.original_snapshot,
       co.attendance_shift_date, co.created_at, co.updated_at
FROM attendance_corrections co
WHERE co.id = sqlc.arg(id)
  AND co.deleted_at IS NULL
FOR UPDATE;

-- name: ApproveCorrection :one
-- Mark a PENDING correction APPLIED (the proposed change is applied to the target
-- attendance row in the same tx via ApplyCorrectionToAttendance). Only PENDING is
-- decidable; zero rows ⇒ terminal state (service emits 409).
UPDATE attendance_corrections
SET status     = 'APPLIED',
    decided_by = sqlc.arg(decided_by),
    decided_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND status = 'PENDING'
RETURNING id, attendance_id, requester_id, company_id, type,
          proposed_check_in_at, proposed_check_out_at, proposed_attendance_code_id,
          reason, evidence_file_id, status, decided_by, decided_at, reject_reason,
          original_snapshot, attendance_shift_date, created_at, updated_at;

-- name: RejectCorrection :one
-- Reject a PENDING correction (reason required). Same PENDING guard.
UPDATE attendance_corrections
SET status        = 'REJECTED',
    decided_by    = sqlc.arg(decided_by),
    decided_at    = now(),
    reject_reason = sqlc.arg(reject_reason),
    updated_at    = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND status = 'PENDING'
RETURNING id, attendance_id, requester_id, company_id, type,
          proposed_check_in_at, proposed_check_out_at, proposed_attendance_code_id,
          reason, evidence_file_id, status, decided_by, decided_at, reject_reason,
          original_snapshot, attendance_shift_date, created_at, updated_at;
