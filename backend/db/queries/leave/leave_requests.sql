-- E6 leave-request queries (F6.1/F6.2 / SWP-LR-*). Reads LEFT JOIN employees for
-- employee_name, client_companies for company_name, leave_types for
-- leave_type_name. Dates come back as pgtype.Date (08-02 repo converts <-> time.Time
-- like Phase-5/6). Keyset cursor on (created_at DESC, id) per CONVENTIONS §11.

-- name: ListLeaveRequests :many
-- Queue / list load. Keyset cursor (created_at,id) DESC. Filters (all optional via
-- narg): company_id, status, status__in (text[] → status = ANY), employee_id,
-- leave_type_id, start_date >= / <=, q free-text (ILIKE employee name + id + reason).
SELECT lr.id, lr.employee_id, lr.placement_id, lr.company_id, lr.service_line_id,
       lr.leave_type_id, lr.start_date, lr.end_date, lr.duration_days,
       lr.reason, lr.notes, lr.status, lr.delegate_id, lr.document_file_id,
       lr.backdated, lr.clock_in_conflict, lr.no_leader, lr.assigned_leader_id,
       lr.balance_quota_id, lr.balance_requested_days, lr.balance_remaining_at_check,
       lr.balance_requires_override, lr.balance_earmark, lr.balance_allocation,
       lr.created_by, lr.created_at, lr.updated_at,
       e.full_name AS employee_name,
       c.name      AS company_name,
       lt.name     AS leave_type_name,
       lt.code     AS leave_type_code
FROM leave_requests lr
LEFT JOIN employees e         ON e.id  = lr.employee_id
LEFT JOIN client_companies c  ON c.id  = lr.company_id
LEFT JOIN leave_types lt      ON lt.id = lr.leave_type_id
WHERE lr.deleted_at IS NULL
  AND (sqlc.narg(company_id)::text   IS NULL OR lr.company_id   = sqlc.narg(company_id)::text)
  AND (sqlc.narg(status)::text       IS NULL OR lr.status       = sqlc.narg(status)::text)
  AND (sqlc.narg(status_in)::text[]  IS NULL OR lr.status       = ANY(sqlc.narg(status_in)::text[]))
  AND (sqlc.narg(employee_id)::text  IS NULL OR lr.employee_id  = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(leave_type_id)::text IS NULL OR lr.leave_type_id = sqlc.narg(leave_type_id)::text)
  AND (sqlc.narg(start_from)::date   IS NULL OR lr.start_date >= sqlc.narg(start_from)::date)
  AND (sqlc.narg(start_to)::date     IS NULL OR lr.start_date <= sqlc.narg(start_to)::date)
  AND (sqlc.narg(q)::text IS NULL OR (
        e.full_name ILIKE '%' || sqlc.narg(q)::text || '%'
        OR lr.id     ILIKE '%' || sqlc.narg(q)::text || '%'
        OR lr.reason ILIKE '%' || sqlc.narg(q)::text || '%'))
  -- keyset: rows strictly before the cursor (created_at,id) when provided.
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL OR
       (lr.created_at, lr.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY lr.created_at DESC, lr.id DESC
LIMIT sqlc.arg(lim);

-- name: GetLeaveRequest :one
-- Single request with denormalized names.
SELECT lr.id, lr.employee_id, lr.placement_id, lr.company_id, lr.service_line_id,
       lr.leave_type_id, lr.start_date, lr.end_date, lr.duration_days,
       lr.reason, lr.notes, lr.status, lr.delegate_id, lr.document_file_id,
       lr.backdated, lr.clock_in_conflict, lr.no_leader, lr.assigned_leader_id,
       lr.balance_quota_id, lr.balance_requested_days, lr.balance_remaining_at_check,
       lr.balance_requires_override, lr.balance_earmark, lr.balance_allocation,
       lr.created_by, lr.created_at, lr.updated_at,
       e.full_name AS employee_name,
       c.name      AS company_name,
       lt.name     AS leave_type_name,
       lt.code     AS leave_type_code
FROM leave_requests lr
LEFT JOIN employees e         ON e.id  = lr.employee_id
LEFT JOIN client_companies c  ON c.id  = lr.company_id
LEFT JOIN leave_types lt      ON lt.id = lr.leave_type_id
WHERE lr.id = sqlc.arg(id)
  AND lr.deleted_at IS NULL;

-- name: GetLeaveRequestForUpdate :one
-- Row-lock for the state-machine transitions (approve-l1/final/override/reject).
-- Omits joins; the service re-reads via GetLeaveRequest for the DTO.
SELECT lr.id, lr.employee_id, lr.placement_id, lr.company_id, lr.service_line_id,
       lr.leave_type_id, lr.start_date, lr.end_date, lr.duration_days,
       lr.reason, lr.notes, lr.status, lr.delegate_id, lr.document_file_id,
       lr.backdated, lr.clock_in_conflict, lr.no_leader, lr.assigned_leader_id,
       lr.balance_quota_id, lr.balance_requested_days, lr.balance_remaining_at_check,
       lr.balance_requires_override, lr.balance_earmark, lr.balance_allocation,
       lr.created_by, lr.created_at, lr.updated_at
FROM leave_requests lr
WHERE lr.id = sqlc.arg(id)
  AND lr.deleted_at IS NULL
FOR UPDATE;

-- name: CreateLeaveRequest :one
-- Seed / HR-on-behalf path (FE/web does not create — mobile/agent does). id
-- allocated by the column DEFAULT ('SWP-LR-' || swp_next_id('LR')) when omitted.
INSERT INTO leave_requests (
    employee_id, placement_id, company_id, service_line_id, leave_type_id,
    start_date, end_date, duration_days, reason, notes, status,
    delegate_id, document_file_id, backdated, clock_in_conflict,
    no_leader, assigned_leader_id,
    balance_quota_id, balance_requested_days, balance_remaining_at_check, balance_requires_override,
    created_by
) VALUES (
    sqlc.arg(employee_id),
    sqlc.narg(placement_id),
    sqlc.narg(company_id),
    sqlc.narg(service_line_id),
    sqlc.arg(leave_type_id),
    sqlc.arg(start_date),
    sqlc.arg(end_date),
    sqlc.arg(duration_days),
    sqlc.narg(reason),
    sqlc.narg(notes),
    sqlc.arg(status),
    sqlc.narg(delegate_id),
    sqlc.narg(document_file_id),
    sqlc.arg(backdated),
    sqlc.arg(clock_in_conflict),
    sqlc.arg(no_leader),
    sqlc.narg(assigned_leader_id),
    sqlc.narg(balance_quota_id),
    sqlc.narg(balance_requested_days),
    sqlc.narg(balance_remaining_at_check),
    sqlc.narg(balance_requires_override),
    sqlc.narg(created_by)
)
RETURNING id, employee_id, placement_id, company_id, service_line_id, leave_type_id,
          start_date, end_date, duration_days, reason, notes, status,
          delegate_id, document_file_id, backdated, clock_in_conflict,
          no_leader, assigned_leader_id, balance_quota_id, balance_requested_days,
          balance_remaining_at_check, balance_requires_override, balance_earmark,
          balance_allocation, created_by, created_at, updated_at;

-- name: CreateLeaveRequestWithID :one
-- Seed / test variant that supplies an explicit id (deterministic E2E targets).
INSERT INTO leave_requests (
    id, employee_id, placement_id, company_id, service_line_id, leave_type_id,
    start_date, end_date, duration_days, reason, notes, status,
    delegate_id, document_file_id, backdated, clock_in_conflict,
    no_leader, assigned_leader_id,
    balance_quota_id, balance_requested_days, balance_remaining_at_check, balance_requires_override,
    created_by
) VALUES (
    sqlc.arg(id),
    sqlc.arg(employee_id),
    sqlc.narg(placement_id),
    sqlc.narg(company_id),
    sqlc.narg(service_line_id),
    sqlc.arg(leave_type_id),
    sqlc.arg(start_date),
    sqlc.arg(end_date),
    sqlc.arg(duration_days),
    sqlc.narg(reason),
    sqlc.narg(notes),
    sqlc.arg(status),
    sqlc.narg(delegate_id),
    sqlc.narg(document_file_id),
    sqlc.arg(backdated),
    sqlc.arg(clock_in_conflict),
    sqlc.arg(no_leader),
    sqlc.narg(assigned_leader_id),
    sqlc.narg(balance_quota_id),
    sqlc.narg(balance_requested_days),
    sqlc.narg(balance_remaining_at_check),
    sqlc.narg(balance_requires_override),
    sqlc.narg(created_by)
)
ON CONFLICT (id) DO NOTHING
RETURNING id, employee_id, placement_id, company_id, service_line_id, leave_type_id,
          start_date, end_date, duration_days, reason, notes, status,
          delegate_id, document_file_id, backdated, clock_in_conflict,
          no_leader, assigned_leader_id, balance_quota_id, balance_requested_days,
          balance_remaining_at_check, balance_requires_override, balance_earmark,
          balance_allocation, created_by, created_at, updated_at;

-- name: UpdateLeaveRequestStatus :one
-- The approval state transitions (PENDING_L1→PENDING_HR→APPROVED, →REJECTED,
-- →CANCELLED). Also refreshes the routing + balance_check snapshot columns.
UPDATE leave_requests
SET status                     = sqlc.arg(status),
    no_leader                  = sqlc.arg(no_leader),
    assigned_leader_id         = sqlc.narg(assigned_leader_id),
    clock_in_conflict          = sqlc.arg(clock_in_conflict),
    balance_quota_id           = sqlc.narg(balance_quota_id),
    balance_requested_days     = sqlc.narg(balance_requested_days),
    balance_remaining_at_check = sqlc.narg(balance_remaining_at_check),
    balance_requires_override  = sqlc.narg(balance_requires_override),
    updated_at                 = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, employee_id, placement_id, company_id, service_line_id, leave_type_id,
          start_date, end_date, duration_days, reason, notes, status,
          delegate_id, document_file_id, backdated, clock_in_conflict,
          no_leader, assigned_leader_id, balance_quota_id, balance_requested_days,
          balance_remaining_at_check, balance_requires_override, balance_earmark,
          balance_allocation, created_by, created_at, updated_at;

-- name: UpdateLeaveRequestDates :one
-- HR :shorten — sets a new (earlier) end_date + recomputed duration on an APPROVED
-- request. Status unchanged (stays APPROVED).
UPDATE leave_requests
SET start_date    = sqlc.arg(start_date),
    end_date      = sqlc.arg(end_date),
    duration_days = sqlc.arg(duration_days),
    updated_at    = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, employee_id, placement_id, company_id, service_line_id, leave_type_id,
          start_date, end_date, duration_days, reason, notes, status,
          delegate_id, document_file_id, backdated, clock_in_conflict,
          no_leader, assigned_leader_id, balance_quota_id, balance_requested_days,
          balance_remaining_at_check, balance_requires_override, balance_earmark,
          balance_allocation, created_by, created_at, updated_at;

-- name: SetLeaveBalanceSnapshot :exec
-- Writes the FIFO reservation snapshot (openapi BalanceCheck) at SUBMIT-reserve /
-- APPROVE-commit. balance_allocation is the per-lot split (jsonb array). Clearing
-- (release/reverse) passes nulls.
UPDATE leave_requests
SET balance_requested_days     = sqlc.narg(requested_days),
    balance_remaining_at_check = sqlc.narg(remaining_at_check),
    balance_requires_override  = sqlc.narg(requires_override),
    balance_earmark            = sqlc.narg(earmark),
    balance_allocation         = sqlc.narg(allocation),
    updated_at                 = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: CountPendingLeaveDaysForQuota :one
-- Soft-reservation recompute: sum duration_days of this employee+leave_type's open
-- PENDING_L1/PENDING_HR requests in the period (drives quota.pending on read).
SELECT COALESCE(SUM(lr.duration_days), 0)::bigint AS pending_days
FROM leave_requests lr
WHERE lr.employee_id   = sqlc.arg(employee_id)
  AND lr.leave_type_id = sqlc.arg(leave_type_id)
  AND lr.status IN ('PENDING_L1','PENDING_HR')
  AND lr.start_date >= sqlc.arg(period_start)::date
  AND lr.start_date <= sqlc.arg(period_end)::date
  AND lr.deleted_at IS NULL;
