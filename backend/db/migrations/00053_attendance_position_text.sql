-- +goose Up
-- E5 attendance — position becomes FREE-TEXT and service_line is dropped (decision
-- 2026-06-12: service_line removed entirely; Position = free-text, no master/FK/id).
-- Attendance carried two denormalized placement attributes that no longer exist as
-- masters: service_line (the three-line text enum) and position_id (FK → positions).
-- This migration replaces position_id with a plain `position` text column and drops
-- service_line, matching the updated openapi (Attendance.position: required string;
-- service_line / position_id removed).
--
-- position: added nullable, BACKFILLED from the owning placement's position via the
-- still-present positions master (positions.name is the human label), then SET NOT
-- NULL — every attendance row anchored to a placement that carried a position_id, so
-- positions.name resolves for all live rows. After this, attendance no longer joins
-- positions at all (it stores the free-text label directly).

ALTER TABLE attendance ADD COLUMN position text;

-- Backfill the free-text label from the positions master via the existing position_id.
UPDATE attendance a
SET position = COALESCE(pos.name, '')
FROM positions pos
WHERE pos.id = a.position_id;

-- Any row whose position_id failed to resolve (defensive) gets an empty label so the
-- NOT NULL constraint holds.
UPDATE attendance SET position = '' WHERE position IS NULL;

ALTER TABLE attendance ALTER COLUMN position SET NOT NULL;

-- The position_id filter index (00042) and the FK column are no longer used; the
-- within-company position filter is now an exact-match on the text column.
DROP INDEX IF EXISTS attendance_company_position_idx;
CREATE INDEX attendance_company_position_idx
    ON attendance (company_id, position)
    WHERE deleted_at IS NULL;

ALTER TABLE attendance DROP COLUMN position_id;
ALTER TABLE attendance DROP COLUMN service_line;

-- +goose Down
-- Restore service_line + position_id (NOT NULL with safe backfills). position_id is
-- re-derived from the placement; service_line from the placement's service line. Both
-- existed NOT NULL before this migration, so the down path must repopulate them.
ALTER TABLE attendance ADD COLUMN service_line text;
ALTER TABLE attendance ADD COLUMN position_id  text REFERENCES positions(id);

UPDATE attendance a
SET position_id  = p.position_id,
    service_line = lower(replace(sl.name, ' ', '_'))
FROM placements p
JOIN service_lines sl ON sl.id = p.service_line_id
WHERE p.id = a.placement_id;

UPDATE attendance SET service_line = 'facility_services' WHERE service_line IS NULL;

ALTER TABLE attendance ALTER COLUMN service_line SET NOT NULL;
ALTER TABLE attendance ALTER COLUMN position_id  SET NOT NULL;

DROP INDEX IF EXISTS attendance_company_position_idx;
CREATE INDEX attendance_company_position_idx
    ON attendance (company_id, position_id)
    WHERE deleted_at IS NULL;

ALTER TABLE attendance DROP COLUMN position;
