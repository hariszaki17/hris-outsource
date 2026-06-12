-- E6 leave-calendar query (GET /leave-calendar). Returns leave entries overlapping
-- a [from,to] date range, scoped by company / leave-type. The status filter is a
-- text[] the service builds: APPROVED only when show_pending=false, else
-- APPROVED + PENDING_L1 + PENDING_HR. Denormalized names via LEFT JOINs.

-- name: ListCalendarEntries :many
SELECT lr.id AS leave_request_id, lr.employee_id, lr.company_id,
       lr.leave_type_id, lr.start_date, lr.end_date, lr.status,
       lr.delegate_id,
       e.full_name AS employee_name,
       c.name      AS company_name,
       lt.name     AS leave_type_name,
       lt.code     AS leave_type_code,
       d.full_name AS delegate_name
FROM leave_requests lr
LEFT JOIN employees e        ON e.id  = lr.employee_id
LEFT JOIN client_companies c ON c.id  = lr.company_id
LEFT JOIN leave_types lt     ON lt.id = lr.leave_type_id
LEFT JOIN employees d        ON d.id  = lr.delegate_id
WHERE lr.deleted_at IS NULL
  -- overlap test: request [start,end] intersects the requested [from,to] window.
  AND lr.start_date <= sqlc.arg(range_to)::date
  AND lr.end_date   >= sqlc.arg(range_from)::date
  AND lr.status = ANY(sqlc.arg(status_in)::text[])
  AND (sqlc.narg(company_id)::text    IS NULL OR lr.company_id      = sqlc.narg(company_id)::text)
  AND (sqlc.narg(leave_type_id)::text IS NULL OR lr.leave_type_id   = sqlc.narg(leave_type_id)::text)
ORDER BY lr.start_date ASC, lr.id ASC;
