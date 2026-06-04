-- +goose Up
-- Attendance corrections (E5 F5.3 / SWP-COR-*): a requested correction to an
-- attendance record (wrong clock-in/out time, wrong/missing attendance code, etc.).
-- An agent (or leader on their behalf) files a correction; a shift leader / HR
-- approves (applies the proposed change to the target attendance row) or rejects.
-- IDs allocated via the column DEFAULT 'SWP-COR-' || swp_next_id('COR').
-- State machine (status): PENDING → APPROVED|APPLIED|REJECTED|CANCELLED.
--   approve a PENDING correction → applies whitelisted proposed_* fields to the
--   attendance row and marks status (see 07-02); reject (reason) → REJECTED.
-- Backstops: CORRECTION_ALREADY_PENDING (one open correction per attendance) via the
-- partial unique index; OUTSIDE_CORRECTION_WINDOW (7-day) computed off
-- attendance_shift_date in the service.
CREATE TABLE attendance_corrections (
    id                          text PRIMARY KEY DEFAULT ('SWP-COR-' || swp_next_id('COR')),
    attendance_id               text NOT NULL REFERENCES attendance(id),
    requester_id                text NOT NULL REFERENCES employees(id),
    company_id                  text NOT NULL REFERENCES client_companies(id), -- denormalized from attendance for leader-scope queries
    type                        text NOT NULL
        CHECK (type IN ('CHECK_IN','CHECK_OUT','CODE','OTHER')),

    -- Proposed new values (whitelisted; applied on approve via COALESCE).
    proposed_check_in_at        timestamptz,
    proposed_check_out_at       timestamptz,
    proposed_attendance_code_id text REFERENCES attendance_codes(id),

    reason                      text NOT NULL,                                -- required user-supplied reason
    evidence_file_id            text,                                         -- SWP-FILE-* (required for CHECK_IN/CHECK_OUT)

    status                      text NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING','APPROVED','APPLIED','REJECTED','CANCELLED')),
    decided_by                  text,                                         -- SWP-EMP-* (leader/HR)
    decided_at                  timestamptz,
    reject_reason               text,                                         -- required on REJECTED

    -- Frozen pre-application copy of the affected attendance fields (CR-5 audit
    -- fallback + side-by-side diff source).
    original_snapshot           jsonb NOT NULL DEFAULT '{}'::jsonb,

    -- Shift date of the target attendance (basis for the OUTSIDE_CORRECTION_WINDOW
    -- 7-day check; denormalized so the window check needs no JOIN).
    attendance_shift_date       date NOT NULL,

    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    deleted_at                  timestamptz                                   -- soft-delete (CONVENTIONS §6)
);

-- CORRECTION_ALREADY_PENDING backstop: at most one open (PENDING) correction per
-- attendance record (race-proof; the service pre-checks then relies on this index).
CREATE UNIQUE INDEX corrections_one_pending_per_attendance_uq
    ON attendance_corrections (attendance_id)
    WHERE status = 'PENDING' AND deleted_at IS NULL;

-- Corrections queue: per-company, newest first (cursor keyset on (created_at DESC, id)).
CREATE INDEX corrections_company_created_idx
    ON attendance_corrections (company_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS attendance_corrections;
