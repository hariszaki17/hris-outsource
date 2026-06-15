-- +goose Up
-- HR-assigned per-employee leave entitlement (PRD leave-entitlement-assignment,
-- RATIFIED 2026-06-15). Leave stops auto-applying to everyone with hard eligibility
-- gates; instead HR explicitly enrols each employee into the leave types they get,
-- with a per-period quota. Only assigned (active) types are requestable, and the
-- balance grid / picker list only assigned types (ELE-1).
--
-- This is the BASE POLICY (which types + how many days per period). The per-period
-- window instance stays in leave_quotas, auto-opened from this row's entitled_days
-- and reset by period_key (cap_basis cadence) — no leave_quotas schema change.
CREATE TABLE employee_leave_entitlements (
    id            text PRIMARY KEY,                 -- SWP-ELE-<N>
    employee_id   text NOT NULL REFERENCES employees(id),
    leave_type_id text NOT NULL REFERENCES leave_types(id),
    -- Base quota per period. NULL = no fixed quota (event/uncapped types HR toggles
    -- on); the per-event cap then comes from the leave type. For COUNT types this is
    -- the occurrences. CHECK allows NULL or >= 0.
    entitled_days integer CHECK (entitled_days IS NULL OR entitled_days >= 0),
    -- Soft on/off: removing a type from an employee = active=false (ELE-6); the type
    -- vanishes from the picker/grid while existing windows + history are retained.
    active        boolean     NOT NULL DEFAULT true,
    note          text        NOT NULL DEFAULT '',  -- optional HR note (why granted/changed)
    assigned_by   text        REFERENCES users(id), -- audit (who assigned/last changed)
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);

-- One assignment per (employee, leave_type). Deactivation keeps the row (active=false)
-- so re-adding upserts onto it; the slot is freed only on hard soft-delete.
CREATE UNIQUE INDEX employee_leave_entitlements_emp_type_uq
    ON employee_leave_entitlements (employee_id, leave_type_id)
    WHERE deleted_at IS NULL;

CREATE INDEX employee_leave_entitlements_emp_idx
    ON employee_leave_entitlements (employee_id)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE employee_leave_entitlements;
