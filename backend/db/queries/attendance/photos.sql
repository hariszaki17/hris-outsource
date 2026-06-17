-- name: CreateAttendancePhoto :one
-- Stores one clock-in/out selfie (E5 F5.1 / CI-10). Allocates the SWP-FILE id inline
-- from the shared per-prefix sequence. Returns metadata only (not the blob).
INSERT INTO attendance_photos (
    id, employee_id, caption, file_name, mime, size_bytes, blob
) VALUES (
    'SWP-FILE-' || swp_next_id('FILE'),
    sqlc.arg(employee_id),
    sqlc.arg(caption),
    sqlc.arg(file_name),
    sqlc.arg(mime),
    sqlc.arg(size_bytes),
    sqlc.arg(blob)
)
RETURNING id, file_name, mime, size_bytes, created_at;

-- name: GetAttendancePhotoByID :one
-- Returns selfie metadata + blob for the authenticated file-download handler.
SELECT id, file_name, mime, size_bytes, blob, created_at
FROM attendance_photos
WHERE id = sqlc.arg(id);
