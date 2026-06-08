-- +goose Up
-- E5 attendance — contract update (CR site/position filters + true-ABSENT rows).
-- Denormalizes the placement's site_id + position_id onto attendance (mirrors the
-- existing company_id / service_line denormalization), makes a real ABSENT record
-- representable (a scheduled shift with NO clock-in and NO clock-in GPS), and adds
-- the within-company filter indexes (site/position only NARROW within leader scope).
--
-- site_id/position_id: added nullable, BACKFILLED from the owning placement, then
-- SET NOT NULL — every attendance row anchors to a placement, and placements carry
-- both columns NOT NULL (E3 INV-5 site; E2 position). This matches the openapi,
-- which marks site_id/position_id as REQUIRED (non-null) on the Attendance schema.

ALTER TABLE attendance
    ADD COLUMN site_id     text REFERENCES client_sites(id),
    ADD COLUMN position_id text REFERENCES positions(id);

-- Backfill the denormalized columns from each row's placement before NOT NULL.
UPDATE attendance a
SET site_id     = p.site_id,
    position_id = p.position_id
FROM placements p
WHERE p.id = a.placement_id;

ALTER TABLE attendance
    ALTER COLUMN site_id     SET NOT NULL,
    ALTER COLUMN position_id SET NOT NULL;

-- ABSENT = scheduled shift, no clock-in. Drop the clock-in NOT NULLs so a true
-- absence (null check_in_at + null clock-in coordinates) is storable. lat_in/lng_in
-- were left NOT NULL by 00026; relax them here (flagged by the contract review).
ALTER TABLE attendance ALTER COLUMN check_in_at DROP NOT NULL;
ALTER TABLE attendance ALTER COLUMN lat_in      DROP NOT NULL;
ALTER TABLE attendance ALTER COLUMN lng_in      DROP NOT NULL;

-- Within-company filter support, matching the style of attendance_company_checkin_idx
-- (partial on deleted_at IS NULL). site/position narrow inside the company scope.
CREATE INDEX attendance_company_site_idx
    ON attendance (company_id, site_id)
    WHERE deleted_at IS NULL;
CREATE INDEX attendance_company_position_idx
    ON attendance (company_id, position_id)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS attendance_company_position_idx;
DROP INDEX IF EXISTS attendance_company_site_idx;
ALTER TABLE attendance ALTER COLUMN lng_in      SET NOT NULL;
ALTER TABLE attendance ALTER COLUMN lat_in      SET NOT NULL;
ALTER TABLE attendance ALTER COLUMN check_in_at SET NOT NULL;
ALTER TABLE attendance DROP COLUMN position_id;
ALTER TABLE attendance DROP COLUMN site_id;
