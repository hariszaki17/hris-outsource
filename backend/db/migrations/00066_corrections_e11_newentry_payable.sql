-- +goose Up
-- Corrections → E11 + NEW_ENTRY + attendance payable (2026-06-15, EPICS §8).
--   1. Corrections route through the E11 approval engine (request_type=CORRECTION):
--      add approval_instance_id; approve/reject move to the engine (the OnApproved hook
--      applies the change). decided_by/decided_at/reject_reason are kept (the hook still
--      stamps the applied/rejected bookkeeping).
--   2. NEW_ENTRY type: an agent creates an attendance record for a day with NO record
--      (and possibly no shift). attendance_id becomes nullable (null until APPLIED);
--      work_date carries the target day.
--   3. Attendance payable: attendance.is_payable (nullable) mirrors approved_leave_days.is_payable
--      (true payable · false not · null pending an SL/HR/super flag for a no-shift day).
ALTER TABLE attendance_corrections
    ALTER COLUMN attendance_id DROP NOT NULL,
    ADD COLUMN work_date date,
    ADD COLUMN approval_instance_id text REFERENCES approval_instances(id);

-- Extend the type CHECK with NEW_ENTRY (drop + re-add the inline-named constraint).
ALTER TABLE attendance_corrections DROP CONSTRAINT attendance_corrections_type_check;
ALTER TABLE attendance_corrections ADD CONSTRAINT attendance_corrections_type_check
    CHECK (type IN ('CHECK_IN','CHECK_OUT','CODE','OTHER','NEW_ENTRY'));

-- A NEW_ENTRY carries work_date (not attendance_id); a non-NEW_ENTRY carries attendance_id.
ALTER TABLE attendance_corrections ADD CONSTRAINT attendance_corrections_target_check
    CHECK (
        (type = 'NEW_ENTRY' AND work_date IS NOT NULL)
        OR (type <> 'NEW_ENTRY' AND attendance_id IS NOT NULL)
    );

-- Race-proof dedupe: at most one open (PENDING) NEW_ENTRY per (requester, work_date).
-- (The existing one-pending-per-attendance index does not apply — attendance_id is null.)
CREATE UNIQUE INDEX corrections_one_pending_newentry_per_day_uq
    ON attendance_corrections (requester_id, work_date)
    WHERE status = 'PENDING' AND type = 'NEW_ENTRY' AND deleted_at IS NULL;

-- Attendance payability (mirrors approved_leave_days.is_payable, migr. 00064).
-- null = pending (no-shift day awaiting an SL/HR/super flag) · true payable · false not.
ALTER TABLE attendance ADD COLUMN is_payable boolean;

-- +goose Down
ALTER TABLE attendance DROP COLUMN IF EXISTS is_payable;
DROP INDEX IF EXISTS corrections_one_pending_newentry_per_day_uq;
ALTER TABLE attendance_corrections DROP CONSTRAINT IF EXISTS attendance_corrections_target_check;
ALTER TABLE attendance_corrections DROP CONSTRAINT attendance_corrections_type_check;
ALTER TABLE attendance_corrections ADD CONSTRAINT attendance_corrections_type_check
    CHECK (type IN ('CHECK_IN','CHECK_OUT','CODE','OTHER'));
ALTER TABLE attendance_corrections
    DROP COLUMN IF EXISTS approval_instance_id,
    DROP COLUMN IF EXISTS work_date;
-- NOTE: attendance_id is left nullable on down (cannot safely re-impose NOT NULL if
-- NEW_ENTRY rows with null attendance_id exist).
