-- +goose Up
-- E11 collapses the OT two-level approval state machine into a single
-- engine-driven PENDING (PENDING_L1/PENDING_HR were the L1→L2 levels, now
-- approval-template lines). WITHDRAWN folds into CANCELLED (the engine + domain
-- use CANCELLED for caller-cancelled requests). PENDING_AGENT_CONFIRM is kept (the
-- pre-approval candidate state, before the chain starts) and stays the default.

-- Drop the old CHECK first so the row migration can write the new PENDING value
-- (the legacy constraint forbids it, and the new one forbids the legacy values).
ALTER TABLE overtime DROP CONSTRAINT overtime_status_check;

-- Migrate live rows.
UPDATE overtime SET status = 'PENDING'   WHERE status IN ('PENDING_L1','PENDING_HR');
UPDATE overtime SET status = 'CANCELLED' WHERE status = 'WITHDRAWN';

-- Swap in the CHECK backstop for the collapsed enum.
ALTER TABLE overtime ADD CONSTRAINT overtime_status_check
    CHECK (status IN ('PENDING_AGENT_CONFIRM','PENDING','APPROVED','REJECTED','CANCELLED'));

-- +goose Down
-- LOSSY: PENDING cannot be split back into PENDING_L1 vs PENDING_HR, and CANCELLED
-- cannot be told apart from the old WITHDRAWN. Best-effort: PENDING → PENDING_HR;
-- CANCELLED rows are left as-is (the down enum keeps CANCELLED... so re-add the old
-- enum WITH WITHDRAWN but do not attempt to reclassify CANCELLED).
ALTER TABLE overtime DROP CONSTRAINT overtime_status_check;
ALTER TABLE overtime ADD CONSTRAINT overtime_status_check
    CHECK (status IN ('PENDING_AGENT_CONFIRM','PENDING_L1','PENDING_HR','APPROVED','REJECTED','WITHDRAWN','CANCELLED'));
UPDATE overtime SET status = 'PENDING_HR' WHERE status = 'PENDING';
