-- +goose Up
-- Move `position` from PLACEMENT to EMPLOYEE (decision 2026-06-15). Position is the
-- agent's job role/title — an attribute of the person, set on Tambah/Edit Karyawan,
-- NOT of a specific placement. It was previously a free-text column on placements
-- (00054); it now lives on employees and is dropped from placements. Everything that
-- grouped/filtered by placement.position is re-sourced to the employee (org rollups,
-- roster-by-position) or to the attendance row's own stored snapshot (billable reports,
-- attendance.position — a clock-time copy, unchanged here). Transfer/Renew no longer
-- carry a position: moving an agent keeps their employee-level position.

-- 1. employees: new free-text position column. NOT NULL DEFAULT '' so existing rows
--    (and any unplaced employee) satisfy the constraint; '' renders as "—" and the
--    API/form require a non-empty value on create/edit going forward.
ALTER TABLE employees ADD COLUMN position text NOT NULL DEFAULT '';

-- 2. Backfill each employee's position from their placement history: prefer the latest
--    non-terminal placement (the "current" one ListEmployees surfaced), else the most
--    recent placement of any status. Unplaced employees keep '' (no source).
UPDATE employees e
SET position = sub.position
FROM (
    SELECT DISTINCT ON (p.employee_id) p.employee_id, p.position
    FROM placements p
    WHERE p.deleted_at IS NULL
      AND p.position <> ''
    ORDER BY p.employee_id,
             (CASE WHEN p.lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START','EXTENDED')
                   THEN 0 ELSE 1 END),
             p.status_changed_at DESC
) sub
WHERE e.id = sub.employee_id
  AND e.position = '';

-- 3. Drop placements.position — position is now read from the employee.
ALTER TABLE placements DROP COLUMN position;

-- +goose Down
-- Reverse: restore the free-text placement column (nullable→NOT NULL after backfill
-- from the owning employee), then drop employees.position.
ALTER TABLE placements ADD COLUMN position text;
UPDATE placements p
SET position = e.position
FROM employees e
WHERE e.id = p.employee_id;
UPDATE placements SET position = '' WHERE position IS NULL;
ALTER TABLE placements ALTER COLUMN position SET NOT NULL;

ALTER TABLE employees DROP COLUMN position;
