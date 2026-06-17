-- E5 attendance auto-reconcile queries (F5.7 / AR-1..AR-10). When a workable shift
-- is created (scheduling.CreateEntry), an existing machine-owned UNSCHEDULED
-- attendance record the shift covers is adopted: linked to the new schedule_id, its
-- shift window snapshotted, and (for machine-owned rows) its lateness / status /
-- flags / verification / payability re-derived with the same clock-in rules. The
-- window [shift_start-2h, shift_end+4h] is computed in Go (Asia/Jakarta wall-clock,
-- mirroring clock.sql GetTodayScheduleForEmployee) and passed as timestamptz bounds.
-- `make gen` writes internal/repository/sqlc (NEVER hand-edit).

-- name: FindReconcileCandidate :one
-- The UNSCHEDULED attendance row a new shift adopts (AR-3 window). Match: same
-- employee_id + placement_id, schedule_id IS NULL, not deleted, UNSCHEDULED in
-- flags, and check_in_at within [window_start, window_end]; earliest check_in_at
-- wins (AR-3 — earliest if several, no out-of-window fallback). The machine-owned
-- vs human-decided split (AR-2 vs AR-9) is classified in Go from the returned
-- verification_status / verified_by / rejected_by: a machine-owned row is
-- re-derived (ReconcileAttendance); a human-decided one gets lineage-only
-- (RelinkAttendanceLineage). FOR UPDATE row-locks the adoptee so a mid-review SL is
-- serialized (C-AR-7).
SELECT a.id, a.check_in_at, a.flags, a.status, a.verification_status, a.is_payable,
       a.verified_by, a.rejected_by
FROM attendance a
WHERE a.employee_id = sqlc.arg(employee_id)
  AND a.placement_id = sqlc.arg(placement_id)
  AND a.schedule_id IS NULL
  AND a.deleted_at IS NULL
  AND 'UNSCHEDULED' = ANY(a.flags)
  AND a.check_in_at IS NOT NULL
  AND a.check_in_at >= sqlc.arg(window_start)::timestamptz
  AND a.check_in_at <= sqlc.arg(window_end)::timestamptz
ORDER BY a.check_in_at
LIMIT 1
FOR UPDATE;

-- name: ReconcileAttendance :one
-- Adopt a machine-owned UNSCHEDULED record (AR-5/6/7/8): link schedule_id + shift
-- snapshot and write the Go-re-derived status / is_late / late_minutes / flags /
-- verification_status / is_payable. flags is the caller-computed array (UNSCHEDULED
-- removed, LATE added when derived; OUTSIDE_GEOFENCE / AUTO_CLOSED preserved).
-- is_payable is COALESCE'd so an explicit true/false is never overridden, only a
-- NULL is promoted to true (AR-8). Count-guarded by schedule_id IS NULL (AR-4) so a
-- concurrent link cannot double-apply — zero rows ⇒ already linked (repo no-op).
UPDATE attendance
SET schedule_id         = sqlc.arg(schedule_id),
    shift_start_at      = sqlc.arg(shift_start_at),
    shift_end_at        = sqlc.arg(shift_end_at),
    status              = sqlc.arg(status),
    is_late             = sqlc.arg(is_late),
    late_minutes        = sqlc.arg(late_minutes),
    flags               = sqlc.arg(flags)::text[],
    verification_status = sqlc.arg(verification_status),
    is_payable          = COALESCE(is_payable, sqlc.narg(is_payable)::boolean),
    updated_at          = now()
WHERE id = sqlc.arg(id)
  AND schedule_id IS NULL
  AND deleted_at IS NULL
RETURNING id;

-- name: RelinkAttendanceLineage :one
-- Lineage-only adoption of a HUMAN-decided record (AR-9): a record already
-- VERIFIED/REJECTED/ESCALATED is NOT re-derived — its status / verification_status /
-- flags / is_payable are the human's authoritative decision and stay untouched. Only
-- the schedule_id + shift snapshot are attached so payroll/reporting tie the record
-- to its shift. Count-guarded by schedule_id IS NULL (AR-4).
UPDATE attendance
SET schedule_id    = sqlc.arg(schedule_id),
    shift_start_at = sqlc.arg(shift_start_at),
    shift_end_at   = sqlc.arg(shift_end_at),
    updated_at     = now()
WHERE id = sqlc.arg(id)
  AND schedule_id IS NULL
  AND deleted_at IS NULL
RETURNING id;
