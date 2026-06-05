-- +goose Up
-- Async payslip-export job tracker (E8 payslip-export / SWP-EXP-*). POST
-- /payslips:export enqueues a River job inside the same tx that inserts a QUEUED
-- row here (transactional outbox via jobs.Client.EnqueueTx, 10-02); the
-- PayslipExportWorker transitions QUEUED → RUNNING → DONE/FAILED and writes the
-- artifact ref + row count. E8 has NO FE job-status hook (the generalized status
-- surface is E10/Phase-11) — the E2E observes completion by polling this table.
--
-- IDs allocated via the column DEFAULT 'SWP-EXP-' || swp_next_id('EXP')
-- (decision [05-01]).
--
-- status enum is pinned to openapi PayslipExportJob.status — QUEUED/RUNNING/
-- DONE/FAILED. NOTE: 10-CONTEXT prose uses "COMPLETED" but the openapi enum's
-- terminal-success value is DONE; DONE wins (match the contract byte-for-byte) —
-- the worker sets DONE. confidential is server-enforced true (Wave 2.8 lock).
-- kind defaults PAYSLIP_EXPORT (forward-compat for E10's generalized exports;
-- this milestone only inserts PAYSLIP_EXPORT). format XLSX-only (D5 2026-06-02).
CREATE TABLE export_jobs (
    id                  text PRIMARY KEY DEFAULT ('SWP-EXP-' || swp_next_id('EXP')),
    kind                text NOT NULL DEFAULT 'PAYSLIP_EXPORT',
    status              text NOT NULL DEFAULT 'QUEUED'
        CHECK (status IN ('QUEUED','RUNNING','DONE','FAILED')),
    format              text NOT NULL DEFAULT 'XLSX'
        CHECK (format IN ('XLSX')),
    confidential        boolean NOT NULL DEFAULT true,          -- server-enforced true (Wave 2.8 OptConfidential lock)

    requested_by_id     text NOT NULL,
    requested_by_name   text,

    -- The export scope echoed back (openapi PayslipExportJob.scope).
    scope_period        text,
    scope_year          integer,
    scope_employee_ids  text[] NOT NULL DEFAULT '{}',

    row_count           integer,                               -- set by the worker on completion
    artifact_ref        text,                                  -- file ref / faithful stand-in (worker)
    error_message       text,                                  -- set on FAILED

    requested_at        timestamptz NOT NULL DEFAULT now(),
    started_at          timestamptz,                           -- set when status → RUNNING
    completed_at        timestamptz                            -- set when status → DONE/FAILED
);

-- Worker dequeue / status polling.
CREATE INDEX export_jobs_status_idx ON export_jobs (status);

-- +goose Down
DROP TABLE IF EXISTS export_jobs;
