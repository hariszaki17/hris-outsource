-- +goose Up
-- Leave approvals (E6 decision trail, per the FEATURE ER diagram): one immutable
-- row per approval action (L1 approve, HR approve, override, reject) against a
-- leave request. Drives the LeaveRequest.timeline[] the FE renders. Uses a
-- bigserial PK (no SWP id, avoids touching ids.go) — mirrors placement_history
-- (00021) and the append-only decision-log pattern. stage/decision enums are DB
-- CHECK backstops matching openapi schemas.LeaveStage / LeaveDecision.
CREATE TABLE leave_approvals (
    id                bigserial PRIMARY KEY,
    leave_request_id  text NOT NULL REFERENCES leave_requests(id),
    stage             text NOT NULL CHECK (stage IN ('L1','HR')),
    decision          text NOT NULL CHECK (decision IN ('APPROVED','REJECTED','OVERRIDE_APPROVED')),
    actor_id          text,                                        -- SWP-USR-* / SWP-EMP-*
    actor_role        text,                                        -- super_admin | hr_admin | shift_leader
    decision_note     text,
    reject_reason     text,
    is_override       boolean NOT NULL DEFAULT false,
    override_reason   text,
    occurred_at       timestamptz NOT NULL DEFAULT now()
);

-- Timeline read: all decisions for a request in chronological order.
CREATE INDEX leave_approvals_request_idx
    ON leave_approvals (leave_request_id, occurred_at);

-- +goose Down
DROP TABLE IF EXISTS leave_approvals;
