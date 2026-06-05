-- E8 payslip audit-note queries (PA-7, §8). Append-only HR annotations keyed to
-- a payslip. The id is a composite "{payslip_id}-NOTE-{seq}" assigned by the
-- service (seq = CountPayslipAuditNotes + 1 in the insert tx). Notes may exist on
-- DECRYPT_FAIL payslips and are never nulled.

-- name: ListPayslipAuditNotes :many
-- Chronological (oldest-first per openapi) list for GET /payslips/{id}/audit-notes.
-- Keyset cursor on (created_at ASC, seq ASC): rows strictly after the cursor.
SELECT id, payslip_id, seq, text, author_id, author_name, created_at
FROM payslip_audit_notes
WHERE payslip_id = sqlc.arg(payslip_id)
  AND (
    sqlc.narg(cursor_seq)::int IS NULL
    OR (created_at, seq) > (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_seq)::int)
  )
ORDER BY created_at ASC, seq ASC
LIMIT sqlc.arg(lim);

-- name: CountPayslipAuditNotes :one
-- The service computes next seq = count + 1 for the composite id.
SELECT count(*) FROM payslip_audit_notes WHERE payslip_id = sqlc.arg(payslip_id);

-- name: InsertPayslipAuditNote :one
-- Append one immutable note. id is the service-assigned composite
-- "{payslip_id}-NOTE-{seq}".
INSERT INTO payslip_audit_notes (id, payslip_id, seq, text, author_id, author_name)
VALUES (sqlc.arg(id), sqlc.arg(payslip_id), sqlc.arg(seq), sqlc.arg(text),
        sqlc.arg(author_id), sqlc.narg(author_name))
RETURNING id, payslip_id, seq, text, author_id, author_name, created_at;

-- name: PayslipExists :one
-- Note-create / list 404 guard (CONVENTIONS §7 — hide existence behind 404).
SELECT EXISTS(
    SELECT 1 FROM payslips WHERE id = sqlc.arg(id) AND deleted_at IS NULL
);
