-- +goose Up
-- E11 linkage: leave_requests + overtime each point at their governing
-- approval_instance (nullable — legacy/seed rows and fallback-less requests may
-- have none). The domain Get/List reads expose this so the API can return the
-- chain id alongside the request.
ALTER TABLE leave_requests
    ADD COLUMN approval_instance_id text REFERENCES approval_instances(id);

ALTER TABLE overtime
    ADD COLUMN approval_instance_id text REFERENCES approval_instances(id);

CREATE INDEX leave_requests_approval_instance_idx
    ON leave_requests (approval_instance_id)
    WHERE approval_instance_id IS NOT NULL;
CREATE INDEX overtime_approval_instance_idx
    ON overtime (approval_instance_id)
    WHERE approval_instance_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS overtime_approval_instance_idx;
DROP INDEX IF EXISTS leave_requests_approval_instance_idx;
ALTER TABLE overtime DROP COLUMN IF EXISTS approval_instance_id;
ALTER TABLE leave_requests DROP COLUMN IF EXISTS approval_instance_id;
