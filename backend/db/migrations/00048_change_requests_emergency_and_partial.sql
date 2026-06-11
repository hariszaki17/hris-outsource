-- +goose Up
-- E2 EP-5 alignment with today's change-request decisions:
-- - Editable tiers shift: `address` becomes instant-apply (no CR), emergency
--   contact becomes a new approval-tier field. CR whitelist is now phone /
--   emergency_contact / bank_account.
-- - Shift-leader routing + bank-split: an SL may approve non-bank fields while a
--   bank change escalates to HR. Modeled per-field in `field_resolutions` (which
--   non-bank fields the SL already applied + who) plus a denormalized
--   `bank_pending` flag that drives the HR bank-escalation queue, and a new
--   `partially_approved` status for the in-between state.

-- request_type whitelist: ADDRESS → EMERGENCY_CONTACT.
ALTER TABLE change_requests DROP CONSTRAINT change_requests_request_type_check;
ALTER TABLE change_requests ADD CONSTRAINT change_requests_request_type_check
    CHECK (request_type IN ('PHONE', 'EMERGENCY_CONTACT', 'BANK_ACCOUNT', 'MULTIPLE'));

-- status += partially_approved (SL applied non-bank fields, bank still pending HR).
ALTER TABLE change_requests DROP CONSTRAINT change_requests_status_check;
ALTER TABLE change_requests ADD CONSTRAINT change_requests_status_check
    CHECK (status IN ('pending', 'approved', 'rejected', 'partially_approved'));

-- Per-field resolution map + denormalized bank-pending flag.
ALTER TABLE change_requests
    ADD COLUMN field_resolutions jsonb   NOT NULL DEFAULT '{}',
    ADD COLUMN bank_pending      boolean NOT NULL DEFAULT false;

-- Partial index backing the HR bank-escalation queue (only rows awaiting HR bank approval).
CREATE INDEX change_requests_bank_pending_idx
    ON change_requests (submitted_at DESC, id DESC)
    WHERE bank_pending;

-- +goose Down
DROP INDEX IF EXISTS change_requests_bank_pending_idx;
ALTER TABLE change_requests
    DROP COLUMN bank_pending,
    DROP COLUMN field_resolutions;

ALTER TABLE change_requests DROP CONSTRAINT change_requests_status_check;
ALTER TABLE change_requests ADD CONSTRAINT change_requests_status_check
    CHECK (status IN ('pending', 'approved', 'rejected'));

ALTER TABLE change_requests DROP CONSTRAINT change_requests_request_type_check;
ALTER TABLE change_requests ADD CONSTRAINT change_requests_request_type_check
    CHECK (request_type IN ('PHONE', 'ADDRESS', 'BANK_ACCOUNT', 'MULTIPLE'));
