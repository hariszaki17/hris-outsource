-- +goose Up
-- Attendance code master data (E2 / E5 attendance). Maps raw attendance events
-- to display labels + behavior flags (workday, paid, billable, verification).
CREATE TABLE attendance_codes (
    id                 text PRIMARY KEY,              -- SWP-AC-<N>
    code               text NOT NULL,                 -- machine-readable, e.g. PRESENT, LATE, ABSENT
    label              text NOT NULL,                 -- Indonesian display label, e.g. Hadir
    description        text NOT NULL DEFAULT '',
    color              text NOT NULL DEFAULT '',      -- hex color (present = teal #0F8B8D per DESIGN-SYSTEM §2)
    is_workday         boolean NOT NULL DEFAULT false,
    is_paid            boolean NOT NULL DEFAULT false,
    is_billable        boolean NOT NULL DEFAULT false,
    needs_verification boolean NOT NULL DEFAULT false,
    status             text NOT NULL DEFAULT 'active'
                           CHECK (status IN ('active', 'inactive')),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz                    -- soft-delete (CONVENTIONS §6)
);

-- Case-insensitive unique code among non-deleted attendance codes.
CREATE UNIQUE INDEX attendance_codes_code_uq
    ON attendance_codes (lower(code))
    WHERE deleted_at IS NULL;

-- Case-insensitive unique label among non-deleted attendance codes.
CREATE UNIQUE INDEX attendance_codes_label_uq
    ON attendance_codes (lower(label))
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS attendance_codes;
