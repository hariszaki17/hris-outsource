-- +goose Up
-- Public-holiday calendar (E7 F7.1 / SWP-HOL-*): the HR-managed master table that
-- feeds OT day_type classification (a work_date in this calendar resolves to the
-- HOLIDAY tier; precedence HOLIDAY > RESTDAY > WORKDAY). A small master table:
-- HR/super manage via GET/POST/PATCH/DELETE. IDs allocated via the column DEFAULT
-- 'SWP-HOL-' || swp_next_id('HOL') (Phase-5 column-DEFAULT allocation, decision
-- [05-01]).
--
-- The column is named holiday_date (not `date`) to avoid the SQL reserved-word
-- friction; the DTO maps it to the openapi `date` field. name length (maxLength
-- 120) is enforced in the service, not the DB. category enum is the openapi
-- schemas.HolidayCategory backstop. applicable_service_lines empty = global.
CREATE TABLE holidays (
    id                          text PRIMARY KEY DEFAULT ('SWP-HOL-' || swp_next_id('HOL')),
    name                        text NOT NULL,
    holiday_date                date NOT NULL,
    category                    text NOT NULL
        CHECK (category IN ('NATIONAL','REGIONAL','CUSTOM')),
    recurring                   boolean NOT NULL DEFAULT false,
    applicable_service_lines    text[] NOT NULL DEFAULT '{}',           -- empty = global; non-empty = restricted to listed service lines
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    deleted_at                  timestamptz                             -- soft-delete (CONVENTIONS §6)
);

-- HOLIDAY_DATE_CLASH backstop: a duplicate (date, category) among non-deleted
-- holidays is rejected (the service pre-checks via GetHolidayByDateCategory then
-- catches the 23505).
CREATE UNIQUE INDEX holidays_date_category_uq
    ON holidays (holiday_date, category)
    WHERE deleted_at IS NULL;

-- Deferred FK (see the ordering note in 00031_overtime.sql): now that holidays
-- exists, wire overtime.holiday_id → holidays(id). goose runs 00031 before 00032,
-- so the constraint could not be declared inline on the overtime table.
ALTER TABLE overtime
    ADD CONSTRAINT overtime_holiday_id_fkey
    FOREIGN KEY (holiday_id) REFERENCES holidays(id);

-- +goose Down
ALTER TABLE overtime DROP CONSTRAINT IF EXISTS overtime_holiday_id_fkey;
DROP TABLE IF EXISTS holidays;
