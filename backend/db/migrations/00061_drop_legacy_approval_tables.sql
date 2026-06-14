-- +goose Up
-- E11 removes the per-module decision-trail tables (leave_approvals 00030,
-- overtime_approvals 00031) and the profile change-request queue (change_requests
-- 00019 + 00048) — all three are superseded by the unified approval engine
-- (approval_actions trail) and instant profile self-edit (E2). The Down recreates
-- them verbatim from their original DDL.
DROP TABLE IF EXISTS leave_approvals;
DROP TABLE IF EXISTS overtime_approvals;
DROP TABLE IF EXISTS change_requests;

-- +goose Down
-- Recreate change_requests (00019 + 00048 merged: emergency-contact whitelist,
-- partially_approved status, field_resolutions + bank_pending).
CREATE TABLE change_requests (
    id               text PRIMARY KEY,              -- SWP-CHG-<N>
    employee_id      text NOT NULL REFERENCES employees(id),
    status           text NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'approved', 'rejected', 'partially_approved')),
    changes          jsonb NOT NULL,
    request_type     text NOT NULL
                         CHECK (request_type IN ('PHONE', 'EMERGENCY_CONTACT', 'BANK_ACCOUNT', 'MULTIPLE')),
    note             text,
    submitted_at     timestamptz NOT NULL DEFAULT now(),
    resolved_at      timestamptz,
    resolved_by      text,
    rejection_reason text,
    field_resolutions jsonb   NOT NULL DEFAULT '{}',
    bank_pending      boolean NOT NULL DEFAULT false
);
CREATE INDEX change_requests_status_idx
    ON change_requests (status, submitted_at DESC, id DESC);
CREATE INDEX change_requests_bank_pending_idx
    ON change_requests (submitted_at DESC, id DESC)
    WHERE bank_pending;

-- Recreate overtime_approvals (00031).
CREATE TABLE overtime_approvals (
    id              bigserial PRIMARY KEY,
    overtime_id     text NOT NULL REFERENCES overtime(id),
    level           integer NOT NULL CHECK (level IN (1, 2)),
    decision        text NOT NULL CHECK (decision IN ('APPROVED','REJECTED','OVERRIDE_APPROVED')),
    approver_id     text,
    approver_name   text,
    reason          text,
    decided_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX overtime_approvals_ot_idx
    ON overtime_approvals (overtime_id);

-- Recreate leave_approvals (00030).
CREATE TABLE leave_approvals (
    id                bigserial PRIMARY KEY,
    leave_request_id  text NOT NULL REFERENCES leave_requests(id),
    stage             text NOT NULL CHECK (stage IN ('L1','HR')),
    decision          text NOT NULL CHECK (decision IN ('APPROVED','REJECTED','OVERRIDE_APPROVED')),
    actor_id          text,
    actor_role        text,
    decision_note     text,
    reject_reason     text,
    is_override       boolean NOT NULL DEFAULT false,
    override_reason   text,
    occurred_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX leave_approvals_request_idx
    ON leave_approvals (leave_request_id, occurred_at);
