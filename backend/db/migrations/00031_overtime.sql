-- +goose Up
-- Overtime records (E7 F7.1/F7.4 / SWP-OT-*): one OT record for one agent on one
-- work_date, carrying the workflow state machine
-- (PENDING_AGENT_CONFIRM → PENDING_L1 → PENDING_HR → APPROVED; reject at either
-- level → REJECTED; withdraw → WITHDRAWN). The web surface is HR/leader APPROVAL +
-- the holiday calendar — agent OT capture/confirm-from-mobile is OUT OF SCOPE here,
-- so records (incl. PENDING_AGENT_CONFIRM auto-detected candidates) are typically
-- seeded directly in 09-02; InsertOvertime exists for the seed/HR-on-behalf path.
-- IDs allocated via the column DEFAULT 'SWP-OT-' || swp_next_id('OT') (Phase-5
-- column-DEFAULT allocation, decision [05-01]): INSERTs omit id to let DEFAULT
-- fire, OR supply an explicit id (seed/test).
--
-- V1 records HOURS/MINUTES ONLY (INV-2): reference_multiplier is the applicable
-- overtime_rules tier rate, STORED for the tier-breakdown card but NEVER applied —
-- no monetary calc in v1. status / source / day_type enums are owned by the app
-- (domain/overtime) and pinned to openapi schemas.OvertimeStatus/Source/Tier; the
-- CHECKs below are the DB backstops (mirror leave_requests). company_id +
-- service_line_id are DENORMALIZED so the leader-scope queue + rule lookup read
-- with no JOIN.
--
-- FK ORDERING NOTE: holiday_id below is declared as a PLAIN text column (no inline
-- REFERENCES) because goose runs migrations in numeric order (00031 BEFORE 00032),
-- so the holidays table does not yet exist. The overtime.holiday_id → holidays(id)
-- FK is ADDED at the END of 00032_holidays.sql (after CREATE TABLE holidays) via
-- ALTER TABLE overtime ADD CONSTRAINT overtime_holiday_id_fkey.
CREATE TABLE overtime (
    id                      text PRIMARY KEY DEFAULT ('SWP-OT-' || swp_next_id('OT')),
    employee_id             text NOT NULL REFERENCES employees(id),
    company_id              text REFERENCES client_companies(id),       -- DENORMALIZED for leader-scope queue (no JOIN), mirrors leave_requests.company_id
    placement_id            text NOT NULL REFERENCES placements(id),    -- E3 placement active on work_date (scope source)
    attendance_id           text REFERENCES attendance(id),             -- NULLABLE; non-null only when source=AUTO_DETECTED (the E5 trigger record)
    service_line_id         text,                                       -- denormalized (nullable); used for rule lookup by line

    work_date               date NOT NULL,

    -- HH:MM text (matches openapi pattern '^[0-2][0-9]:[0-5][0-9]$' + Phase-6
    -- text-HH:MM decision [06-01]); all nullable.
    planned_start_time      text,
    planned_end_time        text,
    actual_start_time       text,
    actual_end_time         text,
    cross_midnight          boolean NOT NULL DEFAULT false,

    -- How the OT entered the system (openapi schemas.OvertimeSource).
    source                  text NOT NULL
        CHECK (source IN ('REQUESTED','AUTO_DETECTED','WORKED_WITHOUT_REQUEST')),

    -- Lifecycle state (openapi schemas.OvertimeStatus — AUTHORITATIVE). enum lives
    -- in the app; this CHECK is the DB backstop. Candidates start
    -- PENDING_AGENT_CONFIRM.
    status                  text NOT NULL DEFAULT 'PENDING_AGENT_CONFIRM'
        CHECK (status IN ('PENDING_AGENT_CONFIRM','PENDING_L1','PENDING_HR','APPROVED','REJECTED','WITHDRAWN')),

    -- Day-type tier (openapi schemas.OvertimeTier / tier_indicator). Classified
    -- from the schedule (E4) + the holiday calendar in 09-02 (precedence
    -- HOLIDAY > RESTDAY > WORKDAY); stored at seed/record time per CONTEXT.
    day_type text NOT NULL CHECK (day_type IN ('WORKDAY','RESTDAY','HOLIDAY')),

    -- Minutes (OvertimeCalculation). counted = floor(worked/30)*30.
    worked_minutes          integer NOT NULL DEFAULT 0,
    counted_minutes         integer NOT NULL DEFAULT 0,
    min_minutes_threshold   integer NOT NULL DEFAULT 30,                -- locked at 30 (EPICS §8 D4)
    skipped_too_short       boolean NOT NULL DEFAULT false,            -- true when counted_minutes < threshold (OT_BELOW_MIN)

    -- STORED reference only, NOT applied (INV-2): the applicable overtime_rules tier
    -- rate. V1 records hours; multipliers are reference metadata for the breakdown.
    reference_multiplier    numeric(4,2),
    overtime_rule_id        text REFERENCES overtime_rules(id),         -- NULLABLE; the E2 rule applied (tier_breakdown source)
    holiday_id              text,                                       -- NULLABLE; set when day_type=HOLIDAY (HOLIDAY_IN_USE source). FK added in 00032 (see ordering note above).

    flagged_no_preapproval  boolean NOT NULL DEFAULT false,            -- always paired with source=WORKED_WITHOUT_REQUEST (EPICS §8)
    reason                  text,

    created_by              text,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    deleted_at              timestamptz                                 -- soft-delete (CONVENTIONS §6)
);

-- Leader-scope queue / list load: per-company by status (PENDING_L1/PENDING_HR
-- filters), newest first via the keyset cursor (created_at DESC, id).
CREATE INDEX overtime_company_status_idx
    ON overtime (company_id, status)
    WHERE deleted_at IS NULL;
-- Per-employee lookups (employee filter).
CREATE INDEX overtime_employee_idx
    ON overtime (employee_id)
    WHERE deleted_at IS NULL;
-- HOLIDAY_IN_USE lookup: which OT references a given holiday.
CREATE INDEX overtime_holiday_idx
    ON overtime (holiday_id)
    WHERE deleted_at IS NULL;

-- Overtime approvals (E7 decision trail, the OvertimeApproval analog of
-- leave_approvals 00030): one immutable row per approval action (L1 approve, HR
-- final, override, reject) against an OT record. Drives the Overtime.approvals[]
-- timeline the FE renders on GET /overtime/{id}. bigserial PK (no SWP id, avoids
-- touching ids.go) — mirrors placement_history (00021) + leave_approvals (00030).
-- level/decision enums are DB CHECK backstops matching the openapi approvals item.
CREATE TABLE overtime_approvals (
    id              bigserial PRIMARY KEY,
    overtime_id     text NOT NULL REFERENCES overtime(id),
    level           integer NOT NULL CHECK (level IN (1, 2)),
    decision        text NOT NULL CHECK (decision IN ('APPROVED','REJECTED','OVERRIDE_APPROVED')),
    approver_id     text,                                               -- SWP-USR-* / SWP-EMP-*
    approver_name   text,
    reason          text,
    decided_at      timestamptz NOT NULL DEFAULT now()
);

-- Timeline read: all decisions for an OT record in chronological order.
CREATE INDEX overtime_approvals_ot_idx
    ON overtime_approvals (overtime_id);

-- +goose Down
DROP TABLE IF EXISTS overtime_approvals;
DROP TABLE IF EXISTS overtime;
