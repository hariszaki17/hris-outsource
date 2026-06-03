-- name: InsertRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, family_id, rotated_from, user_agent, ip, expires_at)
VALUES (
    sqlc.arg(user_id),
    sqlc.arg(token_hash),
    sqlc.arg(family_id),
    sqlc.narg(rotated_from),
    sqlc.narg(user_agent),
    sqlc.narg(ip),
    sqlc.arg(expires_at)
)
RETURNING id, user_id, token_hash, family_id, rotated_from, expires_at, revoked_at, created_at;

-- name: GetRefreshTokenByHash :one
SELECT id, user_id, token_hash, family_id, rotated_from, expires_at, revoked_at, created_at
FROM refresh_tokens
WHERE token_hash = sqlc.arg(token_hash);

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = now()
WHERE id = sqlc.arg(id) AND revoked_at IS NULL;

-- name: RevokeFamily :exec
-- Reuse detection: kill every live token in the family.
UPDATE refresh_tokens
SET revoked_at = now()
WHERE family_id = sqlc.arg(family_id) AND revoked_at IS NULL;
