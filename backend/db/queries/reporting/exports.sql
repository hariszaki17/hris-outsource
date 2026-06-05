-- E10 F10.4 generic export-framework queries over the GENERALIZED export_jobs
-- table (00036). These are NEW, generic queries (report_type + filters jsonb);
-- the Phase-10 payroll/export_jobs.sql queries (InsertExportJob / GetExportJob /
-- UpdateExportJobStatus, PAYSLIP path) stay UNTOUCHED. The 11-02b export service
-- uses these for ATTENDANCE_BILLABLE etc.
--
-- DB status QUEUED/RUNNING/DONE/FAILED/CANCELLED; the service maps RUNNING<->
-- PROCESSING and DONE<->COMPLETED to the wire ExportStatus at the DTO boundary.
-- id allocated by the column DEFAULT 'SWP-EXP-' || swp_next_id('EXP') (omit id).

-- name: InsertExportJobGeneric :one
-- Queue a generic report export. status defaults QUEUED. kind is set to the
-- report_type for forward-compat with the Phase-10 'kind' column (the worker
-- branches on report_type). format/confidential/filters per request.
INSERT INTO export_jobs (
    kind, report_type, format, confidential, filters,
    requested_by_id, requested_by_name, audit_log_entry_id, expires_at
) VALUES (
    sqlc.arg(report_type), sqlc.arg(report_type), sqlc.arg(format),
    sqlc.arg(confidential), sqlc.arg(filters),
    sqlc.arg(requested_by_id), sqlc.narg(requested_by_name),
    sqlc.narg(audit_log_entry_id), sqlc.narg(expires_at)
)
RETURNING id, report_type, status, format, confidential, filters,
          progress_percent, row_count, artifact_ref, error_message,
          audit_log_entry_id, expires_at,
          requested_by_id, requested_by_name,
          requested_at, started_at, completed_at;

-- name: GetExportJobGeneric :one
-- Status poll / GET /exports/{id}. Requester scope enforced in the service.
-- Named *Generic to avoid colliding with the Phase-10 payroll GetExportJob
-- (sqlc shares one sqlcgen package — query names are globally unique).
SELECT id, report_type, status, format, confidential, filters,
       progress_percent, row_count, artifact_ref, error_message,
       audit_log_entry_id, expires_at,
       requested_by_id, requested_by_name,
       requested_at, started_at, completed_at
FROM export_jobs
WHERE id = sqlc.arg(id);

-- name: UpdateExportJobStatusGeneric :one
-- The generic worker's lifecycle writer. Sets status + progress + result fields;
-- stamps started_at on RUNNING (once) and completed_at on the terminal states
-- (DONE/FAILED/CANCELLED). filename mirrors artifact_ref on the wire (11-02b DTO).
UPDATE export_jobs
SET status           = sqlc.arg(status),
    progress_percent = COALESCE(sqlc.narg(progress_percent)::int, progress_percent),
    row_count        = COALESCE(sqlc.narg(row_count)::int, row_count),
    artifact_ref     = COALESCE(sqlc.narg(artifact_ref)::text, artifact_ref),
    error_message    = COALESCE(sqlc.narg(error_message)::text, error_message),
    expires_at       = COALESCE(sqlc.narg(expires_at)::timestamptz, expires_at),
    started_at       = COALESCE(started_at, CASE WHEN sqlc.arg(status) = 'RUNNING' THEN now() END),
    completed_at     = CASE WHEN sqlc.arg(status) IN ('DONE','FAILED','CANCELLED') THEN now() ELSE completed_at END
WHERE id = sqlc.arg(id)
RETURNING id, report_type, status, format, confidential, filters,
          progress_percent, row_count, artifact_ref, error_message,
          audit_log_entry_id, expires_at,
          requested_by_id, requested_by_name,
          requested_at, started_at, completed_at;

-- name: CancelExportJob :one
-- Cancel a still-running job (QUEUED/RUNNING -> CANCELLED). No-op-safe: the
-- service re-reads via GetExportJob when 0 rows return (already terminal).
UPDATE export_jobs
SET status       = 'CANCELLED',
    completed_at = now()
WHERE id = sqlc.arg(id) AND status IN ('QUEUED','RUNNING')
RETURNING id, report_type, status, format, confidential, filters,
          progress_percent, row_count, artifact_ref, error_message,
          audit_log_entry_id, expires_at,
          requested_by_id, requested_by_name,
          requested_at, started_at, completed_at;
