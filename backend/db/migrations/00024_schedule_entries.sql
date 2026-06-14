-- +goose Up
-- Schedule entries (E4 F4.2/F4.3 / SWP-SCH-*): one scheduled shift for one agent
-- on one work_date, anchored to a placement (INV-2). start_time/end_time are a
-- snapshot taken from the shift master at write time (HH:MM, Asia/Jakarta) so the
-- grid renders the historically-correct window even if the master later changes.
-- IDs allocated via the column DEFAULT 'SWP-SCH-' || swp_next_id('SCH').
-- INV-1 (<=1 live entry per agent per date — the DOUBLE_SHIFT backstop) is
-- enforced at the DB level via the partial unique index below, mirroring Phase-5's
-- placements_active_employee_uq.
CREATE TABLE schedule_entries (
    id                text PRIMARY KEY DEFAULT ('SWP-SCH-' || swp_next_id('SCH')),
    employee_id       text NOT NULL REFERENCES employees(id),
    placement_id      text NOT NULL REFERENCES placements(id),   -- INV-2 anchor (E3)
    service_line_id   text REFERENCES service_lines(id),
    shift_master_id   text REFERENCES shift_masters(id),          -- NULL only when is_day_off
    start_time        text,                                       -- snapshot from master at write (HH:MM)
    end_time          text,                                       -- snapshot from master at write (HH:MM)
    cross_midnight    boolean NOT NULL DEFAULT false,
    work_date         date NOT NULL,
    status            text NOT NULL DEFAULT 'SCHEDULED'
        CHECK (status IN ('SCHEDULED','CANCELLED_BY_LEAVE','MODIFIED')),
    is_day_off        boolean NOT NULL DEFAULT false,
    replaced_entry_id text REFERENCES schedule_entries(id),       -- self-FK: MODIFIED replacement link
    created_by        text,                                       -- SWP-USR-<N>
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz                                 -- soft-delete (CONVENTIONS §6)
);

-- INV-1 (DOUBLE_SHIFT backstop): at most one live entry per agent per date.
-- Mirrors placements_active_employee_uq — race-proof; 06-02 service pre-checks
-- (FindLiveEntryForAgentDate) then relies on this index as the final backstop.
CREATE UNIQUE INDEX schedule_entries_active_agent_date_uq
    ON schedule_entries (employee_id, work_date)
    WHERE deleted_at IS NULL;
CREATE INDEX schedule_entries_placement_idx
    ON schedule_entries (placement_id)
    WHERE deleted_at IS NULL;
CREATE INDEX schedule_entries_company_range_idx
    ON schedule_entries (work_date)
    WHERE deleted_at IS NULL;
CREATE INDEX schedule_entries_shift_master_idx
    ON schedule_entries (shift_master_id)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS schedule_entries;
