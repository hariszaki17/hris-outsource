-- +goose Up
-- Classify leave types by audience and surface priority (2026-06-15).
--   applies_to — who the type is for: AGENT (outsourced PKWT), HEAD_OFFICE (internal
--                SWP staff), or ALL (universal statutory leave, UU 13/2003 Pasal 93).
--                Drives the self-service picker/balance grid: an agent sees AGENT + ALL,
--                never HEAD_OFFICE-only types (CTHO, unpaid long leave).
--   common     — surface this type by default on the agent quick views; rarer types
--                (pilgrimage, civic duty, unpaid) hide behind "lihat semua".
ALTER TABLE leave_types
    ADD COLUMN applies_to text NOT NULL DEFAULT 'ALL'
        CHECK (applies_to IN ('AGENT', 'HEAD_OFFICE', 'ALL')),
    ADD COLUMN common     boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE leave_types
    DROP COLUMN applies_to,
    DROP COLUMN common;
