-- name: InsertResetToken :one
-- Stores a new (hashed) password reset token for the user (AU-4).
INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
VALUES (
    sqlc.arg(user_id),
    sqlc.arg(token_hash),
    sqlc.arg(expires_at)
)
RETURNING id, user_id, token_hash, expires_at, used_at, created_at;

-- name: GetResetTokenByHash :one
-- Looks up a reset token by its SHA-256 hash (AU-4 verify step).
SELECT id, user_id, token_hash, expires_at, used_at, created_at
FROM password_reset_tokens
WHERE token_hash = sqlc.arg(token_hash);

-- name: MarkResetTokenUsed :exec
-- Marks a token as consumed (single-use enforcement, AU-4).
UPDATE password_reset_tokens
SET used_at = now()
WHERE id = sqlc.arg(id)
  AND used_at IS NULL;
