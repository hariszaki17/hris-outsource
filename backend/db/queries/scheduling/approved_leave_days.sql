-- E4-owned approved-leave read source (Phase 6). Drives the SHIFT_OVER_LEAVE /
-- CANCELLED_BY_LEAVE conflict branch until E6 (Phase 8) wires the production
-- leave_requests source. See migration 00025 for the ownership / hand-off note.

-- name: FindApprovedLeaveForAgentDate :one
-- SHIFT_OVER_LEAVE source: the approved-leave row (if any) for an agent on a date.
SELECT ald.leave_request_id, ald.leave_type, ald.leave_date
FROM approved_leave_days ald
WHERE ald.employee_id = sqlc.arg(employee_id)
  AND ald.leave_date = sqlc.arg(leave_date)
LIMIT 1;

-- name: InsertApprovedLeaveDay :exec
-- INV-3 write-through (E6 / Phase 8): on final/override leave approval the REAL
-- leave_requests.id replaces the Phase-6 fixture. ON CONFLICT upsert is required
-- because (employee_id, leave_date) is unique (a re-approve / overlapping day must
-- not 23505). had_shift / is_payable carry per-day payability (migr. 00064).
INSERT INTO approved_leave_days (employee_id, leave_date, leave_request_id, leave_type, had_shift, is_payable)
VALUES (sqlc.arg(employee_id), sqlc.arg(leave_date), sqlc.narg(leave_request_id), sqlc.narg(leave_type),
        sqlc.arg(had_shift), sqlc.narg(is_payable))
ON CONFLICT (employee_id, leave_date) DO UPDATE
  SET leave_request_id = EXCLUDED.leave_request_id, leave_type = EXCLUDED.leave_type,
      had_shift = EXCLUDED.had_shift, is_payable = EXCLUDED.is_payable;

-- name: ListApprovedLeaveDaysForRequest :many
-- Per-day payability breakdown for the leave-request detail (payable flag UI).
SELECT leave_date, leave_type, had_shift, is_payable
FROM approved_leave_days
WHERE leave_request_id = sqlc.arg(leave_request_id)
ORDER BY leave_date;

-- name: SetApprovedLeaveDayPayable :one
-- SL/HR/super flags a NO-SHIFT leave day payable (or not). Guarded to had_shift=false
-- rows only — shift days are auto-payable and never overridden here.
UPDATE approved_leave_days
SET is_payable = sqlc.arg(is_payable)
WHERE leave_request_id = sqlc.arg(leave_request_id)
  AND leave_date       = sqlc.arg(leave_date)
  AND had_shift        = false
RETURNING leave_date, leave_type, had_shift, is_payable;

-- name: DeleteApprovedLeaveDaysForRequest :execrows
-- INV-3 reverse (cancel/shorten restore — used by later cancel paths; added now).
DELETE FROM approved_leave_days
WHERE leave_request_id = sqlc.arg(leave_request_id);
