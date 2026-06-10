-- +goose Up
-- F5.6 manual attendance: track who created each record for traceability.
-- - NULL = system events (auto-close, absence sweep) — no human actor.
-- - Agent's SWP-EMP-* = organic clock-in (the agent themselves).
-- - HR/SuperAdmin's SWP-EMP-* = manually created (HR created_for agent).
ALTER TABLE attendance
    ADD COLUMN created_by text REFERENCES employees(id);

-- +goose Down
ALTER TABLE attendance
    DROP COLUMN created_by;
