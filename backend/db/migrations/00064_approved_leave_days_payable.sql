-- +goose Up
-- Per-day payability for approved leave (2026-06-15). A leave day that falls on a
-- scheduled shift is PAYABLE (the agent would have earned that shift) unless the leave
-- type itself is unpaid (cuti di luar tanggungan perusahaan, leave_types.paid=false).
-- A leave day with NO assigned shift is NOT payable by default — SL / HR / super_admin
-- must explicitly flag it payable on the leave-request detail (is_payable NULL -> true).
--
--   had_shift  : the day cancelled a live scheduled shift (CANCELLED_BY_LEAVE).
--   is_payable : resolved payability for payroll. Set at approval:
--                  unpaid type        -> false (never payable, not flaggable)
--                  paid type + shift  -> true
--                  paid type, no shift-> NULL  (pending an SL/HR/super flag; payroll
--                                               treats NULL as non-payable until set)
ALTER TABLE approved_leave_days
    ADD COLUMN had_shift  boolean NOT NULL DEFAULT false,
    ADD COLUMN is_payable boolean;

-- +goose Down
ALTER TABLE approved_leave_days
    DROP COLUMN had_shift,
    DROP COLUMN is_payable;
