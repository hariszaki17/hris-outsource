-- +goose Up
-- One-time heal of ALREADY-diverged schedule snapshots (E4 F4.1 SM-2 ripple).
--
-- Going forward, editing a shift master re-syncs its FUTURE schedule_entries (with
-- attendance freezing) inside the update tx (schedule_propagation.go). This
-- migration converges the data that diverged BEFORE that ripple existed — masters
-- whose times were edited while their snapshot entries kept the old window.
--
-- The rule mirrors the runtime exactly (per future, live, non-day-off,
-- non-cancelled, master-linked entry; LEFT JOIN its single live attendance row):
--   * checked OUT (attendance.check_out_at present) → frozen: leave the entry as-is
--     (the CASE keeps se.start_time / se.end_time, so no net change for these).
--   * checked IN, not out (check_in_at present, check_out_at NULL) → freeze start
--     (keep se.start_time), re-sync end to the master's end.
--   * no attendance row → re-sync both start and end to the master's window.
-- cross_midnight is recomputed from the EFFECTIVE (post-CASE) start vs end as
-- HH:MM ::time (effective_end <= effective_start), matching the Go derivation.
--
-- A second statement then pushes shift_end_at on the OPEN attendance rows (checked
-- in, not out) whose entry end moved, to (work_date + effective_end) AT TIME ZONE
-- 'Asia/Jakarta', + 1 day when the recomputed cross applies — mirroring clock.sql
-- and the runtime SyncOpenAttendanceShiftEnd. shift_start_at is left frozen.
--
-- Scope guard (NOTE): unlike the runtime path (which only fires when a master's
-- window actually changed), this heal recomputes from m.* for EVERY qualifying
-- entry. That is intentionally idempotent — entries already in sync resolve to the
-- same values (no-op), so re-running is safe.

-- 1) Re-sync the schedule_entries snapshot times (start frozen iff checked in).
-- The entry's single live attendance row (≤1 via attendance_schedule_uq) is read
-- via correlated EXISTS rather than a LATERAL join: Postgres forbids referencing
-- the UPDATE target (se) inside a LATERAL in its own FROM (SQLSTATE 42P10), but a
-- correlated subquery may reference it. check_in_at is NOT NULL on every attendance
-- row (the row is born at clock-in), so "has a live attendance row" == checked in.
UPDATE schedule_entries se
SET
    start_time = CASE
        -- checked in (in or out) → keep the frozen start
        WHEN EXISTS (
            SELECT 1 FROM attendance a
            WHERE a.schedule_id = se.id AND a.deleted_at IS NULL
              AND a.check_in_at IS NOT NULL
        ) THEN se.start_time
        ELSE m.start_time
    END,
    end_time = CASE
        -- checked OUT → fully frozen, keep the recorded end
        WHEN EXISTS (
            SELECT 1 FROM attendance a
            WHERE a.schedule_id = se.id AND a.deleted_at IS NULL
              AND a.check_out_at IS NOT NULL
        ) THEN se.end_time
        ELSE m.end_time
    END,
    cross_midnight = (
        -- effective_end <= effective_start (recompute from post-CASE values)
        (CASE
            WHEN EXISTS (
                SELECT 1 FROM attendance a
                WHERE a.schedule_id = se.id AND a.deleted_at IS NULL
                  AND a.check_out_at IS NOT NULL
            ) THEN se.end_time
            ELSE m.end_time
         END)::time
        <=
        (CASE
            WHEN EXISTS (
                SELECT 1 FROM attendance a
                WHERE a.schedule_id = se.id AND a.deleted_at IS NULL
                  AND a.check_in_at IS NOT NULL
            ) THEN se.start_time
            ELSE m.start_time
         END)::time
    ),
    updated_at = now()
FROM shift_masters m
WHERE m.id = se.shift_master_id
  AND se.shift_master_id IS NOT NULL
  AND se.deleted_at IS NULL
  AND se.is_day_off = false
  AND se.status <> 'CANCELLED_BY_LEAVE'
  AND se.work_date >= CURRENT_DATE;

-- 2) Push shift_end_at forward on OPEN attendance rows (checked in, not out) whose
--    entry's end is now the master's end. shift_start_at stays frozen. The entry's
--    (just-healed) cross_midnight decides the +1 day. Bounded to the same future /
--    live / master-linked entry set as step 1.
UPDATE attendance a
SET shift_end_at = (
        ((se.work_date + se.end_time::time)
            + (CASE WHEN se.cross_midnight THEN interval '1 day' ELSE interval '0' END))
        AT TIME ZONE 'Asia/Jakarta'
    )::timestamptz,
    updated_at = now()
FROM schedule_entries se
WHERE a.schedule_id = se.id
  AND a.check_out_at IS NULL
  AND a.deleted_at IS NULL
  AND se.deleted_at IS NULL
  AND se.is_day_off = false
  AND se.status <> 'CANCELLED_BY_LEAVE'
  AND se.shift_master_id IS NOT NULL
  AND se.work_date >= CURRENT_DATE
  AND se.end_time IS NOT NULL;

-- +goose Down
-- No-op: this is a convergence-to-correct heal, not a reversible transform. The
-- pre-heal (diverged) snapshot/shift_end_at values are not recorded anywhere, so
-- there is nothing to restore — and restoring divergence is not desirable. Rolling
-- this migration back simply leaves the healed (correct) data in place.
SELECT 1;
