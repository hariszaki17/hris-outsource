-- +goose Up
-- Agent profile-change requests awaiting HR approval (E2 EP-5).
-- IDs allocated inline: 'SWP-CHG-' || swp_next_id('CHG').
-- Whitelisted editable fields: phone, address, bank_account.
CREATE TABLE change_requests (
    id               text PRIMARY KEY,              -- SWP-CHG-<N>
    employee_id      text NOT NULL REFERENCES employees(id),
    status           text NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'approved', 'rejected')),
    changes          jsonb NOT NULL,                -- {phone?,address?,bank_account?} subset
    request_type     text NOT NULL
                         CHECK (request_type IN ('PHONE', 'ADDRESS', 'BANK_ACCOUNT', 'MULTIPLE')),
    note             text,                          -- free-text note from agent
    submitted_at     timestamptz NOT NULL DEFAULT now(),
    resolved_at      timestamptz,
    resolved_by      text,                          -- SWP-EMP-<N> of resolving HR user
    rejection_reason text
);

-- Compound index for the HR queue: list by status + chrono order + id tiebreak.
CREATE INDEX change_requests_status_idx
    ON change_requests (status, submitted_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS change_requests;
