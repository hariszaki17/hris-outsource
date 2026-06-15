-- E5 corrections queries (F5.4 / SWP-COR-*). Cursor lists keyset on
-- (created_at DESC, id). `make gen` writes internal/repository/sqlc (NEVER hand-edit).
-- Corrections route through the E11 approval engine (request_type=CORRECTION):
-- approval_instance_id links the instance; the OnApproved hook marks APPLIED (and, for
-- a NEW_ENTRY, attaches the created attendance_id). work_date carries a NEW_ENTRY's day.

-- name: ListCorrections :many
-- Corrections queue for a company over filters, newest first.
-- Keyset cursor: pass cursor_created_at + cursor_id from the previous page tail.
--   status_in / type_in: text[] = ANY membership.
--   employee_id maps to requester_id.
--   date_from/date_to: bound on attendance_shift_date.
SELECT co.id, co.attendance_id, co.work_date, co.requester_id, co.company_id, co.type,
       co.proposed_check_in_at, co.proposed_check_out_at,
       co.proposed_attendance_code_id, co.reason, co.evidence_file_id, co.status,
       co.approval_instance_id, co.decided_by, co.decided_at, co.reject_reason,
       co.original_snapshot, co.attendance_shift_date, co.created_at, co.updated_at,
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
-- attendance_shift_date are denormalized by the service. attendance_id is null for a
-- NEW_ENTRY (work_date set instead); approval_instance_id is linked right after via
-- SetCorrectionApprovalInstance (same tx).
INSERT INTO attendance_corrections (
    attendance_id, work_date, requester_id, company_id, type,
    proposed_check_in_at, proposed_check_out_at, proposed_attendance_code_id,
    reason, evidence_file_id, attendance_shift_date, status
) VALUES (
    sqlc.narg(attendance_id), sqlc.narg(work_date), sqlc.arg(requester_id), sqlc.arg(company_id), sqlc.arg(type),
    sqlc.narg(proposed_check_in_at), sqlc.narg(proposed_check_out_at), sqlc.narg(proposed_attendance_code_id),
    sqlc.arg(reason), sqlc.narg(evidence_file_id), sqlc.arg(attendance_shift_date), 'PENDING'
)
RETURNING id;

-- name: SetCorrectionApprovalInstance :exec
-- Link the E11 approval_instance created at submit (mirrors leave SetApprovalInstanceID).
UPDATE attendance_corrections
SET approval_instance_id = sqlc.arg(approval_instance_id),
    updated_at           = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: GetCorrection :one
-- Single correction with denormalized requester/company names.
SELECT co.id, co.attendance_id, co.work_date, co.requester_id, co.company_id, co.type,
       co.proposed_check_in_at, co.proposed_check_out_at,
       co.proposed_attendance_code_id, co.reason, co.evidence_file_id, co.status,
       co.approval_instance_id, co.decided_by, co.decided_at, co.reject_reason,
       co.original_snapshot, co.attendance_shift_date, co.created_at, co.updated_at,
       e.full_name AS requester_name,
       c.name      AS company_name
FROM attendance_corrections co
LEFT JOIN employees e        ON e.id = co.requester_id
LEFT JOIN client_companies c ON c.id = co.company_id
WHERE co.id = sqlc.arg(id)
  AND co.deleted_at IS NULL;

-- name: GetCorrectionForUpdate :one
-- Row-lock for the E11 OnApproved/OnRejected hooks: reads status/company_id/proposed_*
-- for the apply (omits joins; service re-reads for DTO).
SELECT co.id, co.attendance_id, co.work_date, co.requester_id, co.company_id, co.type,
       co.proposed_check_in_at, co.proposed_check_out_at,
       co.proposed_attendance_code_id, co.reason, co.evidence_file_id, co.status,
       co.approval_instance_id, co.decided_by, co.decided_at, co.reject_reason,
       co.original_snapshot, co.attendance_shift_date, co.created_at, co.updated_at
FROM attendance_corrections co
WHERE co.id = sqlc.arg(id)
  AND co.deleted_at IS NULL
FOR UPDATE;

-- name: ApproveCorrection :one
-- Mark a PENDING correction APPLIED (the OnApproved hook applies the proposed change to
-- the target attendance row in the same tx). For a NEW_ENTRY, attendance_id is attached
-- here via COALESCE(narg, existing). Only PENDING is decidable; zero rows ⇒ terminal.
UPDATE attendance_corrections
SET status        = 'APPLIED',
    attendance_id = COALESCE(sqlc.narg(attendance_id)::text, attendance_id),
    decided_by    = sqlc.arg(decided_by),
    decided_at    = now(),
    updated_at    = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND status = 'PENDING'
RETURNING id, attendance_id, work_date, requester_id, company_id, type,
          proposed_check_in_at, proposed_check_out_at, proposed_attendance_code_id,
          reason, evidence_file_id, status, approval_instance_id, decided_by, decided_at,
          reject_reason, original_snapshot, attendance_shift_date, created_at, updated_at;

-- name: RejectCorrection :one
-- Reject a PENDING correction (reason from the E11 approval action). Same PENDING guard.
UPDATE attendance_corrections
SET status        = 'REJECTED',
    decided_by    = sqlc.arg(decided_by),
    decided_at    = now(),
    reject_reason = sqlc.arg(reject_reason),
    updated_at    = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND status = 'PENDING'
RETURNING id, attendance_id, work_date, requester_id, company_id, type,
          proposed_check_in_at, proposed_check_out_at, proposed_attendance_code_id,
          reason, evidence_file_id, status, approval_instance_id, decided_by, decided_at,
          reject_reason, original_snapshot, attendance_shift_date, created_at, updated_at;
