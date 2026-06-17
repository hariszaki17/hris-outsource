-- +goose Up
-- F8.5 Payroll Period Close — the month-end reconciliation gate (PC-1..PC-15).
-- One PayrollPeriod per (year, month), global v1 (PC-1). Locking snapshots an
-- immutable per-FIELD-employee summary (PC-8) the F8.3 run will consume (PC-10);
-- post-lock changes carry forward as PayrollAdjustment rows (PC-9). A thin
-- clarification back-channel (PC-13) lets HR ask the record's shift-leader/agent.
--
-- IDs use the per-prefix column-DEFAULT allocator (same swp_next_id() mechanism as
-- attendance/leave/overtime, decision [05-01]): INSERTs omit id to let the DEFAULT
-- fire, OR supply an explicit id (seed/test). Prefixes:
--   payroll_periods            -> SWP-PPD-*
--   period_employee_summaries  -> SWP-PES-*
--   clarification_requests     -> SWP-CLR-*
--   payroll_adjustments        -> SWP-PAJ-*
-- Soft-delete + timestamps per CONVENTIONS §6 (the immutable summary keeps only
-- created_at — it is voided by DELETE on reopen, never edited).

-- payroll_periods (SWP-PPD-*): one row per (year, month), global. Status FSM
-- OPEN -> LOCKED -> (REOPENED -> OPEN). Auto-created OPEN on first access (PC-1).
CREATE TABLE payroll_periods (
    id                text PRIMARY KEY DEFAULT ('SWP-PPD-' || swp_next_id('PPD')),
    year              integer NOT NULL,
    month             integer NOT NULL CHECK (month >= 1 AND month <= 12),
    status            text NOT NULL DEFAULT 'OPEN'
                          CHECK (status IN ('OPEN','LOCKED','REOPENED')),
    locked_by         text,                                  -- SWP-USR-* (PC-8 stamp)
    locked_at         timestamptz,
    force_locked      boolean NOT NULL DEFAULT false,        -- PC-7 super-admin override
    force_lock_reason text,                                  -- required when force_locked
    reopened_by       text,                                  -- SWP-USR-* (PC-12 stamp)
    reopened_at       timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz,                           -- soft-delete (CONVENTIONS §6)
    UNIQUE (year, month)                                     -- PC-1 one period per month
);

-- period_employee_summaries (SWP-PES-*): the IMMUTABLE per-FIELD-employee snapshot
-- materialized on lock (PC-8). Authoritative payroll input (PC-10). No updated_at /
-- deleted_at: a row is never edited; reopen DELETEs the whole set (PC-12 void).
CREATE TABLE period_employee_summaries (
    id                     text PRIMARY KEY DEFAULT ('SWP-PES-' || swp_next_id('PES')),
    period_id              text NOT NULL REFERENCES payroll_periods(id) ON DELETE CASCADE,
    employee_id            text NOT NULL REFERENCES employees(id),
    -- Attendance figures (PC-8).
    payable_days           integer NOT NULL DEFAULT 0,
    present_days           integer NOT NULL DEFAULT 0,
    late_days              integer NOT NULL DEFAULT 0,
    absent_days            integer NOT NULL DEFAULT 0,
    no_shift_payable_days  integer NOT NULL DEFAULT 0,
    worked_minutes         integer NOT NULL DEFAULT 0,
    -- Overtime + leave figures (PC-8).
    approved_ot_minutes    integer NOT NULL DEFAULT 0,
    paid_leave_days        integer NOT NULL DEFAULT 0,
    unpaid_leave_days      integer NOT NULL DEFAULT 0,
    created_at             timestamptz NOT NULL DEFAULT now(),
    UNIQUE (period_id, employee_id)                          -- PC-8 one summary per employee
);

-- clarification_requests (SWP-CLR-*): the thin, notification-backed back-channel
-- (PC-13). One round: OPEN -> ANSWERED -> RESOLVED (or CANCELLED). source_type +
-- source_id is the record under question (loose cross-epic ref, no FK).
CREATE TABLE clarification_requests (
    id                 text PRIMARY KEY DEFAULT ('SWP-CLR-' || swp_next_id('CLR')),
    source_type        text NOT NULL CHECK (source_type IN ('ATTENDANCE','OVERTIME','LEAVE')),
    source_id          text NOT NULL,                        -- the record under question
    period_id          text REFERENCES payroll_periods(id),  -- nullable; the month it surfaced in
    raised_by          text,                                 -- SWP-USR-* (HR)
    target_employee_id text NOT NULL,                        -- SWP-EMP-* (resolved SL else agent)
    question           text NOT NULL,
    status             text NOT NULL DEFAULT 'OPEN'
                           CHECK (status IN ('OPEN','ANSWERED','RESOLVED','CANCELLED')),
    answer             text,
    answered_by        text,                                 -- SWP-USR-* / SWP-EMP-*
    answered_at        timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz                           -- soft-delete (CONVENTIONS §6)
);

-- Cockpit "awaiting reply" surface: open clarifications for a period.
CREATE INDEX clarification_requests_period_status_idx
    ON clarification_requests (period_id, status)
    WHERE deleted_at IS NULL;

-- payroll_adjustments (SWP-PAJ-*): the F8.3 carry-forward ledger (PC-9). A post-lock
-- change to a locked-month record emits a PENDING row; the run (F8.3, not yet built)
-- values + applies it (APPLIED, stamping applied_run_id). amount is NULL until valued.
CREATE TABLE payroll_adjustments (
    id             text PRIMARY KEY DEFAULT ('SWP-PAJ-' || swp_next_id('PAJ')),
    employee_id    text NOT NULL REFERENCES employees(id),
    source_type    text NOT NULL CHECK (source_type IN ('ATTENDANCE','OVERTIME','CORRECTION')),
    source_id      text NOT NULL,                            -- the changed record
    origin_year    integer NOT NULL,                         -- the LOCKED period the change belongs to
    origin_month   integer NOT NULL CHECK (origin_month >= 1 AND origin_month <= 12),
    note           text,
    amount         text,                                     -- signed Money string; NULL until the run values it
    status         text NOT NULL DEFAULT 'PENDING'
                       CHECK (status IN ('PENDING','APPLIED')),
    applied_run_id text,                                     -- set by the run when applied
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz                               -- soft-delete (CONVENTIONS §6)
);

-- Run consumption: pending adjustments for an employee's open run, by origin month.
CREATE INDEX payroll_adjustments_employee_status_idx
    ON payroll_adjustments (employee_id, status)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS payroll_adjustments;
DROP TABLE IF EXISTS clarification_requests;
DROP TABLE IF EXISTS period_employee_summaries;
DROP TABLE IF EXISTS payroll_periods;
