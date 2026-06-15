-- Position typeahead query (E2 GET /positions:search, backed by employees).
-- Position is FREE-TEXT (no master, no FK, no ID) — the typeahead just surfaces
-- the DISTINCT existing labels already recorded across employees so admins can
-- reuse a consistent string or type a new one (position moved to employee 2026-06-15).

-- name: SearchPositions :many
-- Distinct existing free-text position labels matching the (case-insensitive)
-- substring. The handler passes '%' || q || '%' so an empty q matches everything.
SELECT DISTINCT position
FROM employees
WHERE position ILIKE sqlc.arg(q)::text
  AND position <> ''
  AND deleted_at IS NULL
ORDER BY position
LIMIT 30;
