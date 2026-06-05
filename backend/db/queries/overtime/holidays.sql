-- E7 public-holiday calendar queries (F7.1 / SWP-HOL-*). The HR-managed master
-- that feeds OT day_type classification. holiday_date comes back as pgtype.Date
-- (09-02 repo converts <-> time.Time). Keyset cursor on (holiday_date ASC, id) per
-- CONVENTIONS §11 (calendar reads ascend by date).

-- name: ListHolidays :many
-- Calendar / list load. Keyset cursor (holiday_date,id) ASC. Filters (all optional
-- via narg): category, service_line_id (matches when applicable_service_lines is
-- empty=global OR contains the line), year (EXTRACT(year FROM holiday_date)).
SELECT h.id, h.name, h.holiday_date, h.category, h.recurring,
       h.applicable_service_lines, h.created_at, h.updated_at
FROM holidays h
WHERE h.deleted_at IS NULL
  AND (sqlc.narg(category)::text IS NULL OR h.category = sqlc.narg(category)::text)
  AND (sqlc.narg(service_line_id)::text IS NULL
       OR h.applicable_service_lines = '{}'
       OR sqlc.narg(service_line_id)::text = ANY(h.applicable_service_lines))
  AND (sqlc.narg(year)::int IS NULL
       OR EXTRACT(year FROM h.holiday_date)::int = sqlc.narg(year)::int)
  -- keyset: rows strictly after the cursor (holiday_date,id) when provided.
  AND (sqlc.narg(cursor_date)::date IS NULL OR
       (h.holiday_date, h.id) > (sqlc.narg(cursor_date)::date, sqlc.narg(cursor_id)::text))
ORDER BY h.holiday_date ASC, h.id ASC
LIMIT sqlc.arg(lim);

-- name: GetHoliday :one
-- Single holiday (for GET after create/update).
SELECT h.id, h.name, h.holiday_date, h.category, h.recurring,
       h.applicable_service_lines, h.created_at, h.updated_at
FROM holidays h
WHERE h.id = sqlc.arg(id)
  AND h.deleted_at IS NULL;

-- name: GetHolidayByDateCategory :one
-- HOLIDAY_DATE_CLASH pre-check: does a non-deleted holiday already exist on this
-- (date, category)? The service pre-checks here, then backstops on the 23505 from
-- holidays_date_category_uq.
SELECT h.id, h.name, h.holiday_date, h.category, h.recurring,
       h.applicable_service_lines, h.created_at, h.updated_at
FROM holidays h
WHERE h.holiday_date = sqlc.arg(holiday_date)
  AND h.category     = sqlc.arg(category)
  AND h.deleted_at IS NULL;

-- name: GetHolidayForDate :one
-- day_type classification: is this work_date a holiday? Highest-priority category
-- (NATIONAL) wins when several share a date.
SELECT h.id, h.name, h.holiday_date, h.category, h.recurring,
       h.applicable_service_lines, h.created_at, h.updated_at
FROM holidays h
WHERE h.holiday_date = sqlc.arg(holiday_date)
  AND h.deleted_at IS NULL
ORDER BY (h.category = 'NATIONAL') DESC, h.id ASC
LIMIT 1;

-- name: InsertHoliday :one
-- Create (POST /holidays). id allocated by the column DEFAULT
-- ('SWP-HOL-' || swp_next_id('HOL')) when omitted, OR supplied explicitly
-- (deterministic E2E targets) via ON CONFLICT (id) DO NOTHING.
INSERT INTO holidays (
    id, name, holiday_date, category, recurring, applicable_service_lines
) VALUES (
    COALESCE(sqlc.narg(id)::text, 'SWP-HOL-' || swp_next_id('HOL')),
    sqlc.arg(name),
    sqlc.arg(holiday_date),
    sqlc.arg(category),
    sqlc.arg(recurring),
    sqlc.arg(applicable_service_lines)
)
ON CONFLICT (id) DO NOTHING
RETURNING id, name, holiday_date, category, recurring,
          applicable_service_lines, created_at, updated_at;

-- name: UpdateHoliday :one
-- Partial update (PATCH /holidays/{id}): COALESCE each field so omitted nargs keep
-- the current value (applicable_service_lines is whole-array replace when supplied).
UPDATE holidays
SET name                     = COALESCE(sqlc.narg(name)::text, name),
    holiday_date             = COALESCE(sqlc.narg(holiday_date)::date, holiday_date),
    category                 = COALESCE(sqlc.narg(category)::text, category),
    recurring                = COALESCE(sqlc.narg(recurring)::boolean, recurring),
    applicable_service_lines = COALESCE(sqlc.narg(applicable_service_lines)::text[], applicable_service_lines),
    updated_at               = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, holiday_date, category, recurring,
          applicable_service_lines, created_at, updated_at;

-- name: SoftDeleteHoliday :one
-- DELETE /holidays/{id} (soft). The service first runs CountOvertimeUsingHoliday to
-- guard HOLIDAY_IN_USE.
UPDATE holidays
SET deleted_at = now(), updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id;

-- name: CountOvertimeUsingHoliday :one
-- HOLIDAY_IN_USE guard + the in_use_by_overtime DTO flag: count of APPROVED OT rows
-- referencing this holiday (openapi: "True if any APPROVED OT references this holiday").
SELECT count(*)::bigint AS in_use_count
FROM overtime ot
WHERE ot.holiday_id = sqlc.arg(holiday_id)
  AND ot.status = 'APPROVED'
  AND ot.deleted_at IS NULL;
