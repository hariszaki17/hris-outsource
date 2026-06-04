-- E4 shift-master queries (F4.1 / SM-* / SWP-SHF-*). Reads LEFT JOIN service_lines
-- for service_line_name and compute in_use_count via a correlated subquery over
-- live schedule_entries. Times are text columns (HH:MM, Asia/Jakarta).

-- name: ListShiftMasters :many
-- Cursor page ordered by id desc. Filters:
--   service_line_id → masters tagged to that line OR untagged (NULL applies to all, SM-3),
--   status (ACTIVE→is_active=true / INACTIVE→false) via the is_active narg,
--   q ILIKE over name. in_use_count = live schedule_entries referencing this master.
SELECT sm.id, sm.name, sm.start_time, sm.end_time, sm.break_start, sm.break_end,
       sm.service_line_id, sm.cross_midnight, sm.is_active,
       sm.created_by, sm.created_at, sm.updated_at,
       sl.name AS service_line_name,
       (SELECT count(*) FROM schedule_entries se
         WHERE se.shift_master_id = sm.id AND se.deleted_at IS NULL) AS in_use_count
FROM shift_masters sm
LEFT JOIN service_lines sl ON sl.id = sm.service_line_id
WHERE sm.deleted_at IS NULL
  AND (
        sqlc.narg(service_line_id)::text IS NULL
        OR sm.service_line_id IS NULL
        OR sm.service_line_id = sqlc.narg(service_line_id)::text
      )
  AND (sqlc.narg(is_active)::boolean IS NULL OR sm.is_active = sqlc.narg(is_active)::boolean)
  AND (sqlc.narg(q)::text IS NULL OR sm.name ILIKE '%' || sqlc.narg(q)::text || '%')
  AND (sqlc.narg(cursor_id)::text IS NULL OR sm.id < sqlc.narg(cursor_id)::text)
ORDER BY sm.id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetShiftMaster :one
SELECT sm.id, sm.name, sm.start_time, sm.end_time, sm.break_start, sm.break_end,
       sm.service_line_id, sm.cross_midnight, sm.is_active,
       sm.created_by, sm.created_at, sm.updated_at,
       sl.name AS service_line_name,
       (SELECT count(*) FROM schedule_entries se
         WHERE se.shift_master_id = sm.id AND se.deleted_at IS NULL) AS in_use_count
FROM shift_masters sm
LEFT JOIN service_lines sl ON sl.id = sm.service_line_id
WHERE sm.id = sqlc.arg(id)
  AND sm.deleted_at IS NULL;

-- name: GetShiftMasterForUpdate :one
-- Row-lock for the update / activate-toggle path (omits joins; service re-reads for DTO).
SELECT sm.id, sm.name, sm.start_time, sm.end_time, sm.break_start, sm.break_end,
       sm.service_line_id, sm.cross_midnight, sm.is_active,
       sm.created_by, sm.created_at, sm.updated_at
FROM shift_masters sm
WHERE sm.id = sqlc.arg(id)
  AND sm.deleted_at IS NULL
FOR UPDATE;

-- name: CreateShiftMaster :one
-- id allocated by the column DEFAULT ('SWP-SHF-' || swp_next_id('SHF')).
-- cross_midnight is server-derived (end<=start) and passed in by the 06-02 service.
INSERT INTO shift_masters (
    name, start_time, end_time, break_start, break_end,
    service_line_id, cross_midnight, is_active, created_by
) VALUES (
    sqlc.arg(name),
    sqlc.arg(start_time),
    sqlc.arg(end_time),
    sqlc.narg(break_start),
    sqlc.narg(break_end),
    sqlc.narg(service_line_id),
    sqlc.arg(cross_midnight),
    sqlc.arg(is_active),
    sqlc.narg(created_by)
)
RETURNING id, name, start_time, end_time, break_start, break_end,
          service_line_id, cross_midnight, is_active,
          created_by, created_at, updated_at;

-- name: UpdateShiftMaster :one
-- Full overwrite of the editable fields (06-02 builds the full param set from the
-- current row + the PATCH overlay). cross_midnight re-derived by the service.
UPDATE shift_masters
SET name            = sqlc.arg(name),
    start_time      = sqlc.arg(start_time),
    end_time        = sqlc.arg(end_time),
    break_start     = sqlc.narg(break_start),
    break_end       = sqlc.narg(break_end),
    service_line_id = sqlc.narg(service_line_id),
    cross_midnight  = sqlc.arg(cross_midnight),
    is_active       = sqlc.arg(is_active),
    updated_at      = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, start_time, end_time, break_start, break_end,
          service_line_id, cross_midnight, is_active,
          created_by, created_at, updated_at;

-- name: SetShiftMasterActive :one
-- Drives :deactivate (active=false) / :reactivate (active=true).
UPDATE shift_masters
SET is_active  = sqlc.arg(is_active),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, start_time, end_time, break_start, break_end,
          service_line_id, cross_midnight, is_active,
          created_by, created_at, updated_at;
