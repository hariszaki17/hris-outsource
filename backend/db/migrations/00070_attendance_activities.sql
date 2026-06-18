-- +goose Up
-- Attendance activity log (E5 F5.8 / SWP-ACT-*, 2026-06-18). While a shift is open the
-- agent logs one or more free-text activity notes onto their attendance record; each row
-- captures a server-set recorded_at (Asia/Jakarta "when it was logged"). Agent clock-out
-- is gated on >=1 non-deleted activity (INV-7 / AA-7) — enforced in the clock-out service,
-- not by a DB constraint (system auto-close + HR/leader manual entry are exempt). Soft-delete
-- lets the creator remove a mistaken note while the record is still open (AA-9).
-- IDs minted inline via swp_next_id('ACT') (same allocator as every other entity, 00001).
CREATE TABLE attendance_activities (
    id            text PRIMARY KEY DEFAULT ('SWP-ACT-' || swp_next_id('ACT')),
    attendance_id text NOT NULL REFERENCES attendance (id),  -- SWP-ATT-<N>
    employee_id   text NOT NULL,                              -- SWP-EMP-<N> creator (scope self)
    note          text NOT NULL,                              -- 1..500 chars (validated in handler)
    recorded_at   timestamptz NOT NULL DEFAULT now(),         -- server-set capture time (AA-5)
    created_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz                                  -- soft-delete (AA-9)
);

-- The list (by attendance, chronological) and the clock-out gate count both scan live rows
-- for one attendance_id; partial index keeps both cheap.
CREATE INDEX attendance_activities_attendance_idx
    ON attendance_activities (attendance_id, recorded_at)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS attendance_activities;
