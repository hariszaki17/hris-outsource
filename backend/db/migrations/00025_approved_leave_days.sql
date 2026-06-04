-- +goose Up
-- Minimal approved-leave read source OWNED BY E4 (Phase 6). Exists so the
-- SHIFT_OVER_LEAVE / CANCELLED_BY_LEAVE conflict branch is HONESTLY exercised now
-- without pre-empting E6's leave_requests schema. E6 (Phase 8) will write into
-- this table (or replace it with a view over leave_requests).
-- DO NOT rename to leave_requests — that table belongs to E6 and must not collide.
-- leave_request_id is a denormalized SWP-LR-* display id for ConflictDetails;
-- it intentionally has NO FK (E6 owns the leave_requests / SWP-LR namespace).
CREATE TABLE approved_leave_days (
    id                bigserial PRIMARY KEY,
    employee_id       text NOT NULL REFERENCES employees(id),
    leave_date        date NOT NULL,
    leave_request_id  text,        -- denormalized SWP-LR-* display id (no FK; E6 owns LR)
    leave_type        text,        -- ANNUAL | SICK | MATERNITY | ...
    created_at        timestamptz NOT NULL DEFAULT now()
);

-- One approved-leave row per agent per date (drives the over-leave lookup).
CREATE UNIQUE INDEX approved_leave_days_emp_date_uq
    ON approved_leave_days (employee_id, leave_date);

-- +goose Down
DROP TABLE IF EXISTS approved_leave_days;
