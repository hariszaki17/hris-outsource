-- name: CreateAttachment :one
-- Allocates the SWP-FILE id inline from the per-prefix sequence.
-- Returns metadata only (not the blob) — callers use GetAttachmentByID for download.
INSERT INTO agreement_attachments (
    id, agreement_id, category, caption, file_name, mime, size_bytes, blob, uploaded_by
) VALUES (
    'SWP-FILE-' || swp_next_id('FILE'),
    sqlc.arg(agreement_id),
    sqlc.arg(category),
    sqlc.arg(caption),
    sqlc.arg(file_name),
    sqlc.arg(mime),
    sqlc.arg(size_bytes),
    sqlc.arg(blob),
    sqlc.narg(uploaded_by)
)
RETURNING id, agreement_id, category, caption, file_name, mime, size_bytes, created_at;

-- name: GetAttachmentByID :one
-- Returns file metadata + blob for the authenticated file-download handler.
SELECT id, file_name, mime, size_bytes, blob
FROM agreement_attachments
WHERE id = sqlc.arg(id);
