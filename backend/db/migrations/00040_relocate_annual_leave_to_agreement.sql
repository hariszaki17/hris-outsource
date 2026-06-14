-- +goose Up
-- Product decision 2026-06-07: compensation and statutory leave are terms of the
-- EMPLOYMENT AGREEMENT (E2), not the placement. Drop the redundant comp/leave
-- columns from placements and relocate annual-leave entitlement onto the agreement.
ALTER TABLE placements DROP COLUMN annual_leave_entitlement_days;
ALTER TABLE placements DROP COLUMN base_salary_ref_idr;

ALTER TABLE employment_agreements
    ADD COLUMN annual_leave_entitlement_days integer
        CHECK (annual_leave_entitlement_days IS NULL OR annual_leave_entitlement_days >= 0);

-- +goose Down
ALTER TABLE employment_agreements DROP COLUMN annual_leave_entitlement_days;

ALTER TABLE placements ADD COLUMN base_salary_ref_idr bigint;
ALTER TABLE placements ADD COLUMN annual_leave_entitlement_days integer;
