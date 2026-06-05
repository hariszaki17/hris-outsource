-- +goose Up
-- Generalize the Phase-10 export_jobs table (00034) so it carries ANY report_type,
-- not just PAYSLIP_EXPORT — E10 F10.4 generic export framework. This is an ALTER,
-- NOT a recreate: the Phase-10 PAYSLIP_EXPORT path (InsertExportJob + the
-- PayslipExportWorker lifecycle, 10-02) depends on the existing table and MUST keep
-- working unchanged. Every new column is nullable or defaulted so the existing
-- Phase-10 rows + the InsertExportJob insert stay valid.
--
-- DB status enum vs WIRE status enum (decision pinned here for 11-02b):
--   DB keeps QUEUED/RUNNING/DONE/FAILED + adds CANCELLED.
--   WIRE (openapi ExportStatus) is QUEUED/PROCESSING/COMPLETED/FAILED/CANCELLED.
--   The 11-02b service maps RUNNING <-> PROCESSING and DONE <-> COMPLETED at the
--   DTO boundary (the FE e10-shared.tsx consumes the wire enum). We DO NOT migrate
--   stored values — the mapping is a pure read-time DTO concern.
--
-- format: XLSX (Phase-10) stays valid; EXCEL is added because the openapi
-- ExportFormat enum's value is EXCEL. The 11-02b generic export inserts EXCEL; the
-- Phase-10 payslip path still inserts XLSX. confidential is unchanged (Phase-10
-- forces it true for payslips; generic exports set it per request).
ALTER TABLE export_jobs DROP CONSTRAINT IF EXISTS export_jobs_status_check;
ALTER TABLE export_jobs ADD CONSTRAINT export_jobs_status_check
    CHECK (status IN ('QUEUED','RUNNING','DONE','FAILED','CANCELLED'));

ALTER TABLE export_jobs DROP CONSTRAINT IF EXISTS export_jobs_format_check;
ALTER TABLE export_jobs ADD CONSTRAINT export_jobs_format_check
    CHECK (format IN ('XLSX','EXCEL'));

-- report_type (openapi ReportType). DEFAULT 'PAYSLIPS' keeps existing Phase-10
-- rows valid (their kind is PAYSLIP_EXPORT; the generalized report_type for the
-- payslip archive is PAYSLIPS per the openapi enum).
ALTER TABLE export_jobs ADD COLUMN IF NOT EXISTS report_type        text NOT NULL DEFAULT 'PAYSLIPS';
ALTER TABLE export_jobs ADD COLUMN IF NOT EXISTS filters            jsonb NOT NULL DEFAULT '{}'::jsonb;  -- echoed filter set (display + audit)
ALTER TABLE export_jobs ADD COLUMN IF NOT EXISTS audit_log_entry_id text;                                -- INV-5 / EX-4 traceability (SWP-AL-*)
ALTER TABLE export_jobs ADD COLUMN IF NOT EXISTS progress_percent   integer;                             -- worker progress (nullable; PROCESSING only)
ALTER TABLE export_jobs ADD COLUMN IF NOT EXISTS expires_at         timestamptz;                          -- file retention (EX-5; null until completed)

-- +goose Down
ALTER TABLE export_jobs DROP COLUMN IF EXISTS expires_at;
ALTER TABLE export_jobs DROP COLUMN IF EXISTS progress_percent;
ALTER TABLE export_jobs DROP COLUMN IF EXISTS audit_log_entry_id;
ALTER TABLE export_jobs DROP COLUMN IF EXISTS filters;
ALTER TABLE export_jobs DROP COLUMN IF EXISTS report_type;

ALTER TABLE export_jobs DROP CONSTRAINT IF EXISTS export_jobs_format_check;
ALTER TABLE export_jobs ADD CONSTRAINT export_jobs_format_check
    CHECK (format IN ('XLSX'));

ALTER TABLE export_jobs DROP CONSTRAINT IF EXISTS export_jobs_status_check;
ALTER TABLE export_jobs ADD CONSTRAINT export_jobs_status_check
    CHECK (status IN ('QUEUED','RUNNING','DONE','FAILED'));
