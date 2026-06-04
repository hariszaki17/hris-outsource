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
