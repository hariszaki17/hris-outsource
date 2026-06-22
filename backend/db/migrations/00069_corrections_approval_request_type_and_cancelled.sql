-- +goose Up
-- E11 approval engine ↔ attendance corrections reconciliation (2026-06-18).
--   1. Corrections route through the E11 engine (request_type=CORRECTION, migr 00066),
--      but the approval_instances request_type CHECK was never widened — a real
--      correction :submit would violate it and fail to open the instance. Add CORRECTION.
--   2. Add CANCELLED to the instance status enum: a DIRECT attendance verification
--      (FLOW A) auto-closes any still-open correction (FLOW B) on that record and
--      CANCELs its approval_instance, so the now-moot request drops out of the inbox.
ALTER TABLE approval_instances DROP CONSTRAINT approval_instances_request_type_check;
ALTER TABLE approval_instances ADD CONSTRAINT approval_instances_request_type_check
    CHECK (request_type IN ('LEAVE', 'OVERTIME', 'CORRECTION'));

ALTER TABLE approval_instances DROP CONSTRAINT approval_instances_status_check;
ALTER TABLE approval_instances ADD CONSTRAINT approval_instances_status_check
    CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'CANCELLED'));

-- +goose Down
ALTER TABLE approval_instances DROP CONSTRAINT approval_instances_status_check;
ALTER TABLE approval_instances ADD CONSTRAINT approval_instances_status_check
    CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED'));

ALTER TABLE approval_instances DROP CONSTRAINT approval_instances_request_type_check;
ALTER TABLE approval_instances ADD CONSTRAINT approval_instances_request_type_check
    CHECK (request_type IN ('LEAVE', 'OVERTIME'));
