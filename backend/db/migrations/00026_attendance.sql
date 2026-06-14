-- +goose Up
-- Attendance records (E5 F5.1/F5.2 / SWP-ATT-*): one attendance record for one
-- agent on one shift (or unscheduled). Anchored to the Phase-6 schedule_entries
-- (E4), Phase-5 placements (E3), and employees. The web surface is exceptions-only
-- shift-leader VERIFICATION — clock-in/out is mobile/agent and OUT OF SCOPE here, so
-- geofence/lateness/auto-close are STORED COLUMNS on seeded records (no Haversine at
-- runtime, no clock pipeline). IDs allocated via the column DEFAULT
-- 'SWP-ATT-' || swp_next_id('ATT') (Phase-5 column-DEFAULT allocation, decision [05-01]):
-- INSERTs omit id to let DEFAULT fire, OR supply an explicit id (seed).
-- INV-3 (exceptions-only): a record is verification_status='PENDING' iff it carries an
-- exception (is_late OR out-of-geofence OR auto_closed OR missing clock-out OR a code that
-- needs verification); clean records are 'AUTO_APPROVED' and never enter the queue.
CREATE TABLE attendance (
    id                  text PRIMARY KEY DEFAULT ('SWP-ATT-' || swp_next_id('ATT')),
    employee_id         text NOT NULL REFERENCES employees(id),
    placement_id        text NOT NULL REFERENCES placements(id),       -- E3 anchor (scope source)
    schedule_id         text REFERENCES schedule_entries(id),          -- E4 link; NULL = unscheduled
    company_id          text NOT NULL REFERENCES client_companies(id), -- derived from placement (leader scope)
    service_line        text NOT NULL,                                 -- facility_services|building_management|parking
    attendance_code_id  text REFERENCES attendance_codes(id),

    -- Shift window (snapshot from E4 schedule; nullable when unscheduled).
    shift_start_at      timestamptz,
    shift_end_at        timestamptz,

    -- Clock-in/out instants + coordinates. check_in_at is required (record exists on
    -- clock-in); check_out_at null while open or until auto-close.
    check_in_at         timestamptz NOT NULL,
    check_out_at        timestamptz,
    lat_in              double precision NOT NULL,
    lng_in              double precision NOT NULL,
    lat_out             double precision,
    lng_out             double precision,
    photo_in_id         text,                                          -- SWP-FILE-* (optional)
    photo_out_id        text,

    wfo                 boolean NOT NULL DEFAULT true,                  -- v1 always true (EPICS §8)

    -- Lateness (STORED, not computed): seed sets is_late + late_minutes directly.
    is_late             boolean NOT NULL DEFAULT false,
    late_minutes        integer NOT NULL DEFAULT 0,
    worked_minutes      integer,                                       -- set on clock-out/auto-close
    auto_closed         boolean NOT NULL DEFAULT false,                -- forced-closed by shift-end job (F5.2 EV-3)

    -- Geofence as STORED columns (no Haversine at runtime). in_/out_geofence is the
    -- inside flag; *_distance_m the measured distance; geofence_radius_m the site radius.
    in_geofence         boolean,
    in_distance_m       integer,
    out_geofence        boolean,
    out_distance_m      integer,
    geofence_radius_m   integer NOT NULL DEFAULT 100,

    status              text NOT NULL DEFAULT 'PRESENT'
        CHECK (status IN ('PRESENT','LATE','INCOMPLETE','ABSENT','ON_LEAVE')),
    verification_status text NOT NULL DEFAULT 'PENDING'
        CHECK (verification_status IN ('AUTO_APPROVED','PENDING','VERIFIED','REJECTED','ESCALATED')),
    flags               text[] NOT NULL DEFAULT '{}',                  -- AttendanceFlag values

    -- Verification outcome.
    verified_by         text,                                          -- SWP-EMP-* (leader/HR)
    verified_at         timestamptz,
    rejected_by         text,                                          -- SWP-EMP-*
    rejected_at         timestamptz,
    reject_reason       text,
    last_correction_id  text,                                          -- SWP-COR-* (most recently applied)

    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz                                    -- soft-delete (CONVENTIONS §6)
);

-- List load: per-company verification queue/history, newest first (cursor keyset on
-- (check_in_at DESC, id)).
CREATE INDEX attendance_company_checkin_idx
    ON attendance (company_id, check_in_at DESC)
    WHERE deleted_at IS NULL;
-- Queue narrowing by verification_status (PENDING/ESCALATED filter).
CREATE INDEX attendance_vstatus_idx
    ON attendance (verification_status)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS attendance;
