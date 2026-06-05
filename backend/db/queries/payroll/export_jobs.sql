-- E8 payslip-export job queries (SWP-EXP-*). InsertExportJob writes a QUEUED row
-- inside the tx that EnqueueTx's the River job (transactional outbox, 10-02); the
-- PayslipExportWorker drives the lifecycle via UpdateExportJobStatus. status enum
-- pinned to openapi PayslipExportJob.status (QUEUED/RUNNING/DONE/FAILED — DONE is
-- the terminal-success value).

-- name: InsertExportJob :one
-- Queue a payslip export. status defaults QUEUED, confidential server-enforced
-- true, kind PAYSLIP_EXPORT, format XLSX. id allocated by the column DEFAULT
-- ('SWP-EXP-' || swp_next_id('EXP')) when omitted.
INSERT INTO export_jobs (
    id, kind, format, confidential, requested_by_id, requested_by_name,
    scope_period, scope_year, scope_employee_ids
) VALUES (
    COALESCE(sqlc.narg(id)::text, 'SWP-EXP-' || swp_next_id('EXP')),
    COALESCE(sqlc.narg(kind)::text, 'PAYSLIP_EXPORT'),
    COALESCE(sqlc.narg(format)::text, 'XLSX'),
    COALESCE(sqlc.narg(confidential)::boolean, true),
    sqlc.arg(requested_by_id),
    sqlc.narg(requested_by_name),
    sqlc.narg(scope_period),
    sqlc.narg(scope_year),
    sqlc.arg(scope_employee_ids)
)
RETURNING id, kind, status, format, confidential, requested_by_id,
          requested_by_name, scope_period, scope_year, scope_employee_ids,
          row_count, artifact_ref, error_message, requested_at, started_at, completed_at;

-- name: GetExportJob :one
SELECT id, kind, status, format, confidential, requested_by_id,
       requested_by_name, scope_period, scope_year, scope_employee_ids,
       row_count, artifact_ref, error_message, requested_at, started_at, completed_at
FROM export_jobs
WHERE id = sqlc.arg(id);

-- name: UpdateExportJobStatus :one
-- The worker's lifecycle writer. Sets status + result fields; stamps started_at
-- on RUNNING (once) and completed_at on the terminal states (DONE/FAILED).
UPDATE export_jobs
SET status        = sqlc.arg(status),
    row_count     = COALESCE(sqlc.narg(row_count)::int, row_count),
    artifact_ref  = COALESCE(sqlc.narg(artifact_ref)::text, artifact_ref),
    error_message = COALESCE(sqlc.narg(error_message)::text, error_message),
    started_at    = COALESCE(started_at, CASE WHEN sqlc.arg(status) = 'RUNNING' THEN now() END),
    completed_at  = CASE WHEN sqlc.arg(status) IN ('DONE','FAILED') THEN now() ELSE completed_at END
WHERE id = sqlc.arg(id)
RETURNING id, kind, status, format, confidential, requested_by_id,
          requested_by_name, scope_period, scope_year, scope_employee_ids,
          row_count, artifact_ref, error_message, requested_at, started_at, completed_at;

-- name: CountPayslipsInScope :one
-- EXPORT_TOO_LARGE guard (10-02 compares to the threshold). Same period/year/
-- employee_ids scope as the export request.
SELECT count(*)
FROM payslips
WHERE deleted_at IS NULL
  AND (sqlc.narg(year)::int  IS NULL OR year  = sqlc.narg(year)::int)
  AND (sqlc.narg(month)::int IS NULL OR month = sqlc.narg(month)::int)
  AND (
    sqlc.narg(employee_ids)::text[] IS NULL
    OR cardinality(sqlc.narg(employee_ids)::text[]) = 0
    OR employee_id = ANY(sqlc.narg(employee_ids)::text[])
  );
