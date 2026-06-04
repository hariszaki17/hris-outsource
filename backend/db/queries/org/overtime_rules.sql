-- name: ListOvertimeRules :many
-- Cursor page ordered by (created_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: status, service_line (scopes to a specific line or global).
SELECT id, name, service_line_id, weekday_rate, restday_rate, holiday_rate,
       min_minutes, max_minutes_per_day, pre_approval_required, status, created_at, updated_at
FROM overtime_rules
WHERE deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(service_line_id)::text IS NULL OR service_line_id = sqlc.narg(service_line_id)::text)
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetOvertimeRuleByID :one
SELECT id, name, service_line_id, weekday_rate, restday_rate, holiday_rate,
       min_minutes, max_minutes_per_day, pre_approval_required, status, created_at, updated_at
FROM overtime_rules
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: CreateOvertimeRule :one
-- Allocates the SWP-OTR id inline from the per-prefix sequence.
INSERT INTO overtime_rules (id, name, service_line_id, weekday_rate, restday_rate, holiday_rate,
                            min_minutes, max_minutes_per_day, pre_approval_required)
VALUES (
    'SWP-OTR-' || swp_next_id('OTR'),
    sqlc.arg(name),
    sqlc.narg(service_line_id),
    sqlc.arg(weekday_rate),
    sqlc.arg(restday_rate),
    sqlc.arg(holiday_rate),
    sqlc.arg(min_minutes),
    sqlc.arg(max_minutes_per_day),
    sqlc.arg(pre_approval_required)
)
RETURNING id, name, service_line_id, weekday_rate, restday_rate, holiday_rate,
          min_minutes, max_minutes_per_day, pre_approval_required, status, created_at, updated_at;

-- name: UpdateOvertimeRule :one
UPDATE overtime_rules
SET name                  = sqlc.arg(name),
    service_line_id       = sqlc.narg(service_line_id),
    weekday_rate          = sqlc.arg(weekday_rate),
    restday_rate          = sqlc.arg(restday_rate),
    holiday_rate          = sqlc.arg(holiday_rate),
    min_minutes           = sqlc.arg(min_minutes),
    max_minutes_per_day   = sqlc.arg(max_minutes_per_day),
    pre_approval_required = sqlc.arg(pre_approval_required),
    updated_at            = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, service_line_id, weekday_rate, restday_rate, holiday_rate,
          min_minutes, max_minutes_per_day, pre_approval_required, status, created_at, updated_at;

-- name: SetOvertimeRuleStatus :one
-- Drives :deactivate (status='inactive') and :reactivate (status='active').
UPDATE overtime_rules
SET status     = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, service_line_id, weekday_rate, restday_rate, holiday_rate,
          min_minutes, max_minutes_per_day, pre_approval_required, status, created_at, updated_at;

-- name: SoftDeleteOvertimeRule :exec
UPDATE overtime_rules
SET deleted_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
