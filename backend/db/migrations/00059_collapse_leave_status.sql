-- +goose Up
-- E11 collapses the per-module two-level leave state machine into a single
-- engine-driven PENDING. The legacy PENDING_L1 / PENDING_HR levels are now lines
-- in the approval template, so the leave_requests.status enum drops them.
-- Create still produces DRAFT (leave_service Create → LeaveStatusDraft); submit
-- sets PENDING. Column default stays 'DRAFT'.

-- Drop the old CHECK first so the row migration can write the new PENDING value
-- (the legacy constraint forbids it, and the new one forbids the legacy values).
ALTER TABLE leave_requests DROP CONSTRAINT leave_requests_status_check;

-- Migrate live rows: both old pending levels collapse to PENDING.
UPDATE leave_requests SET status = 'PENDING'
    WHERE status IN ('PENDING_L1','PENDING_HR');

-- Swap in the CHECK backstop for the collapsed enum.
ALTER TABLE leave_requests ADD CONSTRAINT leave_requests_status_check
    CHECK (status IN ('DRAFT','PENDING','APPROVED','REJECTED','CANCELLED'));

-- The old column default 'PENDING_L1' is no longer a valid status. Create-as-draft
-- (leave_service Create → DRAFT, then submit → PENDING), so the column default
-- becomes 'DRAFT'. The CreateLeaveRequest query passes status explicitly; this only
-- governs DEFAULT-fired inserts (seeds).
ALTER TABLE leave_requests ALTER COLUMN status SET DEFAULT 'DRAFT';

-- +goose Down
ALTER TABLE leave_requests ALTER COLUMN status SET DEFAULT 'PENDING_L1';
-- LOSSY: PENDING cannot be split back into PENDING_L1 vs PENDING_HR (the level
-- distinction is gone). Best-effort: map PENDING → PENDING_HR (the later level).
ALTER TABLE leave_requests DROP CONSTRAINT leave_requests_status_check;
ALTER TABLE leave_requests ADD CONSTRAINT leave_requests_status_check
    CHECK (status IN ('DRAFT','PENDING_L1','PENDING_HR','APPROVED','REJECTED','CANCELLED'));
UPDATE leave_requests SET status = 'PENDING_HR' WHERE status = 'PENDING';
