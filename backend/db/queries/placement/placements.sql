-- E3 placement queries (F3.1/F3.2 / PLC-*). All reads LEFT JOIN the Phase-3/4
-- master tables to fill the denormalized *_name fields the spec returns.
-- Param→column note: the FE/spec params are `status` / `status__in`; both filter
-- the `lifecycle_status` column. No param is literally named `lifecycle_status`.

-- name: ListPlacements :many
-- Cursor page ordered by (status_changed_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: company_id, service_line_id, employee_id, agreement_id,
--   status (single → lifecycle_status =), status__in (CSV → lifecycle_status = ANY),
--   q (ILIKE over agent name / employee_id / company name),
--   end_date__lte (expiring cutoff), include_history (exclude terminal states unless true).
SELECT p.id, p.employee_id, p.agreement_id, p.client_company_id, p.site_id,
       p.service_line_id, p.position_id, p.start_date, p.end_date,
       p.notes,
       p.lifecycle_status, p.status_changed_at, p.ended_reason, p.ended_at,
       p.termination_reason, p.resign_at, p.predecessor_id, p.successor_id,
       p.backdate_reason, p.created_by, p.created_at, p.updated_at,
       e.full_name      AS employee_name,
       c.name           AS client_company_name,
       s.name           AS site_name,
       sl.name          AS service_line_name,
       pos.name         AS position_name,
       a.type           AS agreement_type
FROM placements p
LEFT JOIN employees e             ON e.id   = p.employee_id
LEFT JOIN client_companies c      ON c.id   = p.client_company_id
LEFT JOIN client_sites s          ON s.id   = p.site_id
LEFT JOIN service_lines sl        ON sl.id  = p.service_line_id
LEFT JOIN positions pos           ON pos.id = p.position_id
LEFT JOIN employment_agreements a ON a.id   = p.agreement_id
WHERE p.deleted_at IS NULL
  AND (sqlc.narg(company_id)::text      IS NULL OR p.client_company_id = sqlc.narg(company_id)::text)
  AND (sqlc.narg(service_line_id)::text IS NULL OR p.service_line_id   = sqlc.narg(service_line_id)::text)
  AND (sqlc.narg(employee_id)::text     IS NULL OR p.employee_id       = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(agreement_id)::text    IS NULL OR p.agreement_id      = sqlc.narg(agreement_id)::text)
  AND (sqlc.narg(status)::text          IS NULL OR p.lifecycle_status  = sqlc.narg(status)::text)
  AND (sqlc.narg(status_in)::text[]     IS NULL OR p.lifecycle_status  = ANY(sqlc.narg(status_in)::text[]))
  AND (sqlc.narg(end_date__lte)::date   IS NULL OR (p.end_date IS NOT NULL AND p.end_date <= sqlc.narg(end_date__lte)::date))
  AND (
        sqlc.arg(include_history)::boolean
        OR p.lifecycle_status NOT IN ('ENDED','TRANSFERRED','TERMINATED','RESIGNED','SUPERSEDED')
      )
  AND (
        sqlc.narg(q)::text IS NULL
        OR e.full_name   ILIKE '%' || sqlc.narg(q)::text || '%'
        OR p.employee_id ILIKE '%' || sqlc.narg(q)::text || '%'
        OR c.name        ILIKE '%' || sqlc.narg(q)::text || '%'
      )
  AND (
        sqlc.narg(cursor_status_changed_at)::timestamptz IS NULL
        OR (p.status_changed_at, p.id) < (sqlc.narg(cursor_status_changed_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY p.status_changed_at DESC, p.id DESC
LIMIT sqlc.arg(row_limit);

-- name: ListExpiringPlacements :many
-- Backs GET /placements/expiring. Keyset on (end_date asc, id asc).
-- @cutoff = today(Asia/Jakarta) + within_days (computed in the service).
SELECT p.id, p.employee_id, p.agreement_id, p.client_company_id, p.site_id,
       p.service_line_id, p.position_id, p.start_date, p.end_date,
       p.notes,
       p.lifecycle_status, p.status_changed_at, p.ended_reason, p.ended_at,
       p.termination_reason, p.resign_at, p.predecessor_id, p.successor_id,
       p.backdate_reason, p.created_by, p.created_at, p.updated_at,
       e.full_name      AS employee_name,
       c.name           AS client_company_name,
       s.name           AS site_name,
       sl.name          AS service_line_name,
       pos.name         AS position_name,
       a.type           AS agreement_type
FROM placements p
LEFT JOIN employees e             ON e.id   = p.employee_id
LEFT JOIN client_companies c      ON c.id   = p.client_company_id
LEFT JOIN client_sites s          ON s.id   = p.site_id
LEFT JOIN service_lines sl        ON sl.id  = p.service_line_id
LEFT JOIN positions pos           ON pos.id = p.position_id
LEFT JOIN employment_agreements a ON a.id   = p.agreement_id
WHERE p.deleted_at IS NULL
  AND p.lifecycle_status IN ('ACTIVE','EXPIRING')
  AND p.end_date IS NOT NULL
  AND p.end_date <= sqlc.arg(cutoff)::date
  AND (sqlc.narg(company_id)::text IS NULL OR p.client_company_id = sqlc.narg(company_id)::text)
  AND (
        sqlc.narg(cursor_end_date)::date IS NULL
        OR (p.end_date, p.id) > (sqlc.narg(cursor_end_date)::date, sqlc.narg(cursor_id)::text)
      )
ORDER BY p.end_date ASC, p.id ASC
LIMIT sqlc.arg(row_limit);

-- name: PlacementGlobalStats :one
-- Dashboard stat cards (C2SSLA): global placement aggregates over non-deleted
-- placements. Optional company_id scopes the counts (shift-leader RBAC).
SELECT
  COUNT(DISTINCT p.client_company_id) FILTER (
    WHERE p.lifecycle_status NOT IN ('ENDED','TRANSFERRED','SUPERSEDED','TERMINATED','RESIGNED')
  )::bigint AS client_company_count,
  COUNT(*) FILTER (WHERE p.lifecycle_status IN ('ACTIVE','EXTENDED'))::bigint AS active_count,
  COUNT(*) FILTER (WHERE p.lifecycle_status = 'EXPIRING')::bigint AS expiring_count,
  COUNT(*) FILTER (WHERE p.lifecycle_status = 'PENDING_START')::bigint AS pending_count
FROM placements p
WHERE p.deleted_at IS NULL
  AND (sqlc.narg(company_id)::text IS NULL OR p.client_company_id = sqlc.narg(company_id)::text);

-- name: GetPlacementByID :one
SELECT p.id, p.employee_id, p.agreement_id, p.client_company_id, p.site_id,
       p.service_line_id, p.position_id, p.start_date, p.end_date,
       p.notes,
       p.lifecycle_status, p.status_changed_at, p.ended_reason, p.ended_at,
       p.termination_reason, p.resign_at, p.predecessor_id, p.successor_id,
       p.backdate_reason, p.created_by, p.created_at, p.updated_at,
       e.full_name      AS employee_name,
       c.name           AS client_company_name,
       s.name           AS site_name,
       sl.name          AS service_line_name,
       pos.name         AS position_name,
       a.type           AS agreement_type
FROM placements p
LEFT JOIN employees e             ON e.id   = p.employee_id
LEFT JOIN client_companies c      ON c.id   = p.client_company_id
LEFT JOIN client_sites s          ON s.id   = p.site_id
LEFT JOIN service_lines sl        ON sl.id  = p.service_line_id
LEFT JOIN positions pos           ON pos.id = p.position_id
LEFT JOIN employment_agreements a ON a.id   = p.agreement_id
WHERE p.id = sqlc.arg(id)
  AND p.deleted_at IS NULL;

-- name: GetPlacementChain :many
-- All placements sharing a predecessor/successor chain with the given placement
-- (for history_chain). Walks both directions from the seed via a recursive CTE.
WITH RECURSIVE chain AS (
    SELECT p0.id, p0.predecessor_id, p0.successor_id
    FROM placements p0
    WHERE p0.id = sqlc.arg(id)
  UNION
    SELECT p1.id, p1.predecessor_id, p1.successor_id
    FROM placements p1
    JOIN chain ch ON p1.id = ch.predecessor_id OR p1.id = ch.successor_id
)
SELECT p.id, p.employee_id, p.agreement_id, p.client_company_id, p.site_id,
       p.service_line_id, p.position_id, p.start_date, p.end_date,
       p.notes,
       p.lifecycle_status, p.status_changed_at, p.ended_reason, p.ended_at,
       p.termination_reason, p.resign_at, p.predecessor_id, p.successor_id,
       p.backdate_reason, p.created_by, p.created_at, p.updated_at,
       e.full_name      AS employee_name,
       c.name           AS client_company_name,
       s.name           AS site_name,
       sl.name          AS service_line_name,
       pos.name         AS position_name,
       a.type           AS agreement_type
FROM placements p
JOIN chain ch                     ON ch.id  = p.id
LEFT JOIN employees e             ON e.id   = p.employee_id
LEFT JOIN client_companies c      ON c.id   = p.client_company_id
LEFT JOIN client_sites s          ON s.id   = p.site_id
LEFT JOIN service_lines sl        ON sl.id  = p.service_line_id
LEFT JOIN positions pos           ON pos.id = p.position_id
LEFT JOIN employment_agreements a ON a.id   = p.agreement_id
ORDER BY p.start_date ASC, p.id ASC;

-- name: GetActivePlacementForEmployee :one
-- INV-1 service pre-check (friendly 409 before hitting the partial unique index).
SELECT p.id, p.employee_id, p.agreement_id, p.client_company_id, p.site_id,
       p.service_line_id, p.position_id, p.start_date, p.end_date,
       p.notes,
       p.lifecycle_status, p.status_changed_at, p.ended_reason, p.ended_at,
       p.termination_reason, p.resign_at, p.predecessor_id, p.successor_id,
       p.backdate_reason, p.created_by, p.created_at, p.updated_at,
       e.full_name      AS employee_name,
       c.name           AS client_company_name,
       s.name           AS site_name,
       sl.name          AS service_line_name,
       pos.name         AS position_name,
       a.type           AS agreement_type
FROM placements p
LEFT JOIN employees e             ON e.id   = p.employee_id
LEFT JOIN client_companies c      ON c.id   = p.client_company_id
LEFT JOIN client_sites s          ON s.id   = p.site_id
LEFT JOIN service_lines sl        ON sl.id  = p.service_line_id
LEFT JOIN positions pos           ON pos.id = p.position_id
LEFT JOIN employment_agreements a ON a.id   = p.agreement_id
WHERE p.employee_id = sqlc.arg(employee_id)
  AND p.lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START')
  AND p.deleted_at IS NULL;

-- name: GetActivePlacementForEmployeeAtCompanyForUpdate :one
-- INV-4 lock: the agent's active placement at a specific company, row-locked.
SELECT p.id, p.employee_id, p.agreement_id, p.client_company_id, p.site_id,
       p.service_line_id, p.position_id, p.start_date, p.end_date,
       p.notes,
       p.lifecycle_status, p.status_changed_at, p.ended_reason, p.ended_at,
       p.termination_reason, p.resign_at, p.predecessor_id, p.successor_id,
       p.backdate_reason, p.created_by, p.created_at, p.updated_at
FROM placements p
WHERE p.employee_id = sqlc.arg(employee_id)
  AND p.client_company_id = sqlc.arg(client_company_id)
  AND p.lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START')
  AND p.deleted_at IS NULL
FOR UPDATE;

-- name: LockEmployeePlacements :many
-- INV-1 / period-overlap lock: all of the agent's placements, row-locked.
SELECT p.id, p.employee_id, p.agreement_id, p.client_company_id, p.site_id,
       p.service_line_id, p.position_id, p.start_date, p.end_date,
       p.notes,
       p.lifecycle_status, p.status_changed_at, p.ended_reason, p.ended_at,
       p.termination_reason, p.resign_at, p.predecessor_id, p.successor_id,
       p.backdate_reason, p.created_by, p.created_at, p.updated_at
FROM placements p
WHERE p.employee_id = sqlc.arg(employee_id)
  AND p.deleted_at IS NULL
FOR UPDATE;

-- name: CreatePlacement :one
-- id allocated by the column DEFAULT ('SWP-PL-' || swp_next_id('PL')).
INSERT INTO placements (
    employee_id, agreement_id, client_company_id, site_id, service_line_id,
    position_id, start_date, end_date, notes, lifecycle_status, predecessor_id,
    backdate_reason, created_by
) VALUES (
    sqlc.arg(employee_id),
    sqlc.arg(agreement_id),
    sqlc.arg(client_company_id),
    sqlc.arg(site_id),
    sqlc.arg(service_line_id),
    sqlc.arg(position_id),
    sqlc.arg(start_date),
    sqlc.narg(end_date),
    sqlc.narg(notes),
    sqlc.arg(lifecycle_status),
    sqlc.narg(predecessor_id),
    sqlc.narg(backdate_reason),
    sqlc.narg(created_by)
)
RETURNING id, employee_id, agreement_id, client_company_id, site_id,
          service_line_id, position_id, start_date, end_date,
          notes,
          lifecycle_status, status_changed_at, ended_reason, ended_at,
          termination_reason, resign_at, predecessor_id, successor_id,
          backdate_reason, created_by, created_at, updated_at;

-- name: UpdatePlacementFields :one
-- Limited-field PATCH (position_id, end_date, notes).
UPDATE placements
SET position_id                   = sqlc.arg(position_id),
    end_date                      = sqlc.narg(end_date),
    notes                         = sqlc.narg(notes),
    updated_at                    = now()
WHERE id = sqlc.arg(id)
RETURNING id, employee_id, agreement_id, client_company_id, site_id,
          service_line_id, position_id, start_date, end_date,
          notes,
          lifecycle_status, status_changed_at, ended_reason, ended_at,
          termination_reason, resign_at, predecessor_id, successor_id,
          backdate_reason, created_by, created_at, updated_at;

-- name: SetPlacementLifecycle :one
-- Drives end/terminate/resign/transfer/supersede. status_changed_at=now().
UPDATE placements
SET lifecycle_status   = sqlc.arg(lifecycle_status),
    status_changed_at  = now(),
    ended_reason       = sqlc.narg(ended_reason),
    ended_at           = sqlc.narg(ended_at),
    termination_reason = sqlc.narg(termination_reason),
    resign_at          = sqlc.narg(resign_at),
    successor_id       = sqlc.narg(successor_id),
    updated_at         = now()
WHERE id = sqlc.arg(id)
RETURNING id, employee_id, agreement_id, client_company_id, site_id,
          service_line_id, position_id, start_date, end_date,
          notes,
          lifecycle_status, status_changed_at, ended_reason, ended_at,
          termination_reason, resign_at, predecessor_id, successor_id,
          backdate_reason, created_by, created_at, updated_at;

-- name: EndPlacementsForEmployee :many
-- Offboard cascade (OB-1): end every non-terminal placement of an employee.
UPDATE placements
SET lifecycle_status  = sqlc.arg(lifecycle_status)::text,
    ended_reason      = sqlc.arg(ended_reason)::text,
    ended_at          = sqlc.arg(ended_at)::date,
    status_changed_at = now(),
    updated_at        = now()
WHERE employee_id = sqlc.arg(employee_id)
  AND deleted_at IS NULL
  AND lifecycle_status NOT IN ('ENDED','TRANSFERRED','SUPERSEDED','TERMINATED','RESIGNED')
RETURNING id, client_company_id, lifecycle_status, ended_reason;

-- name: SetPlacementPredecessor :exec
UPDATE placements SET predecessor_id = sqlc.arg(predecessor_id), updated_at = now()
WHERE id = sqlc.arg(id);

-- name: SetPlacementSuccessor :exec
UPDATE placements SET successor_id = sqlc.arg(successor_id), updated_at = now()
WHERE id = sqlc.arg(id);

-- name: RosterForCompany :many
-- Company roster (RO-*). Filters: status (single), status__in (CSV),
-- service_line_id, include_history. Keyset on (status_changed_at desc, id desc).
SELECT p.id, p.employee_id, p.agreement_id, p.client_company_id, p.site_id,
       p.service_line_id, p.position_id, p.start_date, p.end_date,
       p.notes,
       p.lifecycle_status, p.status_changed_at, p.ended_reason, p.ended_at,
       p.termination_reason, p.resign_at, p.predecessor_id, p.successor_id,
       p.backdate_reason, p.created_by, p.created_at, p.updated_at,
       e.full_name      AS employee_name,
       c.name           AS client_company_name,
       s.name           AS site_name,
       sl.name          AS service_line_name,
       pos.name         AS position_name,
       a.type           AS agreement_type
FROM placements p
LEFT JOIN employees e             ON e.id   = p.employee_id
LEFT JOIN client_companies c      ON c.id   = p.client_company_id
LEFT JOIN client_sites s          ON s.id   = p.site_id
LEFT JOIN service_lines sl        ON sl.id  = p.service_line_id
LEFT JOIN positions pos           ON pos.id = p.position_id
LEFT JOIN employment_agreements a ON a.id   = p.agreement_id
WHERE p.client_company_id = sqlc.arg(client_company_id)
  AND p.deleted_at IS NULL
  AND (sqlc.narg(service_line_id)::text IS NULL OR p.service_line_id  = sqlc.narg(service_line_id)::text)
  AND (sqlc.narg(status)::text          IS NULL OR p.lifecycle_status = sqlc.narg(status)::text)
  AND (sqlc.narg(status_in)::text[]     IS NULL OR p.lifecycle_status = ANY(sqlc.narg(status_in)::text[]))
  AND (
        sqlc.arg(include_history)::boolean
        OR p.lifecycle_status NOT IN ('ENDED','TRANSFERRED','TERMINATED','RESIGNED','SUPERSEDED')
      )
  AND (
        sqlc.narg(cursor_status_changed_at)::timestamptz IS NULL
        OR (p.status_changed_at, p.id) < (sqlc.narg(cursor_status_changed_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY p.status_changed_at DESC, p.id DESC
LIMIT sqlc.arg(row_limit);

-- name: RosterSummaryByStatus :many
-- CompanyRosterSummary by_status counts (non-deleted; non-terminal unless caller filters).
SELECT p.lifecycle_status AS status, COUNT(*) AS count
FROM placements p
WHERE p.client_company_id = sqlc.arg(client_company_id)
  AND p.deleted_at IS NULL
GROUP BY p.lifecycle_status;

-- name: RosterSummaryByServiceLine :many
-- CompanyRosterSummary by_service_line counts (active placements only).
SELECT p.service_line_id AS service_line_id,
       sl.name           AS service_line_name,
       COUNT(*)          AS count
FROM placements p
LEFT JOIN service_lines sl ON sl.id = p.service_line_id
WHERE p.client_company_id = sqlc.arg(client_company_id)
  AND p.deleted_at IS NULL
  AND p.lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START')
GROUP BY p.service_line_id, sl.name;
