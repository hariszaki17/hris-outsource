-- Position typeahead query (E2 GET /positions:search, backed by E3 placements).
-- Position is FREE-TEXT (no master, no FK, no ID) — the typeahead just surfaces
-- the DISTINCT existing labels already recorded across placements so admins can
-- reuse a consistent string or type a new one (decision 2026-06-12).

-- name: SearchPositions :many
-- Distinct existing free-text position labels matching the (case-insensitive)
-- substring. The handler passes '%' || q || '%' so an empty q matches everything.
SELECT DISTINCT position
FROM placements
WHERE position ILIKE sqlc.arg(q)::text
  AND position <> ''
ORDER BY position
LIMIT 30;
