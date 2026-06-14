-- +goose Up
-- Leave requests (E6 F6.1/F6.2 / SWP-LR-*): one agent's request for leave over a
-- date range, carrying the two-level approval state machine (PENDING_L1 →
-- PENDING_HR → APPROVED; reject at either level → REJECTED; withdraw → CANCELLED).
-- The web surface is HR/leader APPROVAL — agent CREATE is mobile-only, so requests
-- are typically seeded directly in PENDING states; CreateLeaveRequest exists for the
-- seed/HR-on-behalf path. IDs allocated via the column DEFAULT
-- 'SWP-LR-' || swp_next_id('LR') (Phase-5 column-DEFAULT allocation, decision [05-01]):
-- INSERTs omit id to let DEFAULT fire, OR supply an explicit id (seed/test).
--
-- SWP-LR display-id reconciliation [08-01 DECISION]: Phase-6 seeded
-- approved_leave_days.leave_request_id = 'SWP-LR-44210' as a DISPLAY-ONLY id with NO
-- FK (it owns no leave_requests row). The new leave_requests.id uses the SAME
-- swp_next_id('LR') sequence — that is fine: 44210 was a fixed literal far above the
-- sequence cursor, so freshly-created/seeded requests allocate ids well below it and
-- never collide. The INV-3 write-through (08-02) inserts the REAL new
-- leave_requests.id into approved_leave_days, replacing the Phase-6 fixture mechanism.
--
-- Status enum is owned by the app (domain/leave) and pinned to openapi
-- schemas.LeaveStatus; the CHECK below is the DB backstop. The routing.* and
-- balance_check.* snapshot columns are denormalized so the leader-scope queue +
-- calendar + balance-banner read with no JOIN.
CREATE TABLE leave_requests (
    id                          text PRIMARY KEY DEFAULT ('SWP-LR-' || swp_next_id('LR')),
    employee_id                 text NOT NULL REFERENCES employees(id),
    placement_id                text REFERENCES placements(id),          -- nullable; resolved at submit (scope source)
    company_id                  text REFERENCES client_companies(id),    -- DENORMALIZED for leader-scope queue + calendar (no JOIN)
    service_line_id             text,                                    -- denormalized service-line slug source (nullable)
    leave_type_id               text NOT NULL REFERENCES leave_types(id),
    start_date                  date NOT NULL,
    end_date                    date NOT NULL,
    duration_days               integer NOT NULL DEFAULT 0,
    reason                      text,
    notes                       text,

    -- Lifecycle state (openapi schemas.LeaveStatus — AUTHORITATIVE). enum lives in
    -- the app; this CHECK is the DB backstop.
    status                      text NOT NULL DEFAULT 'PENDING_L1'
        CHECK (status IN ('DRAFT','PENDING_L1','PENDING_HR','APPROVED','REJECTED','CANCELLED')),

    delegate_id                 text,
    document_file_id            text,
    backdated                   boolean NOT NULL DEFAULT false,
    clock_in_conflict           boolean NOT NULL DEFAULT false,

    -- routing.* (LA-2 no-leader routing snapshot).
    no_leader                   boolean NOT NULL DEFAULT false,
    assigned_leader_id          text,

    -- balance_check.* snapshot taken at submit/transition (all nullable).
    balance_quota_id            text,
    balance_requested_days      integer,
    balance_remaining_at_check  integer,
    balance_requires_override   boolean,

    created_by                  text,
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    deleted_at                  timestamptz                              -- soft-delete (CONVENTIONS §6)
);

-- Leader-scope queue / list load: per-company by status (PENDING_L1/PENDING_HR
-- filters), newest first via the keyset cursor (created_at DESC, id).
CREATE INDEX leave_requests_company_status_idx
    ON leave_requests (company_id, status)
    WHERE deleted_at IS NULL;
-- Per-employee lookups (quota pending recompute, employee filter).
CREATE INDEX leave_requests_employee_idx
    ON leave_requests (employee_id)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS leave_requests;
