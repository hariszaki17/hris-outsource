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
-- not 23505).
INSERT INTO approved_leave_days (employee_id, leave_date, leave_request_id, leave_type)
VALUES (sqlc.arg(employee_id), sqlc.arg(leave_date), sqlc.narg(leave_request_id), sqlc.narg(leave_type))
ON CONFLICT (employee_id, leave_date) DO UPDATE
  SET leave_request_id = EXCLUDED.leave_request_id, leave_type = EXCLUDED.leave_type;

-- name: DeleteApprovedLeaveDaysForRequest :execrows
-- INV-3 reverse (cancel/shorten restore — used by later cancel paths; added now).
DELETE FROM approved_leave_days
WHERE leave_request_id = sqlc.arg(leave_request_id);
