-- name: UpsertLoginAttempt :exec
INSERT INTO login_attempts (identifier, attempt_count, locked_until, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (identifier) DO UPDATE SET
    attempt_count = EXCLUDED.attempt_count,
    locked_until  = EXCLUDED.locked_until,
    updated_at    = now();

-- name: GetLoginAttempt :one
SELECT id, identifier, attempt_count, locked_until
FROM login_attempts
WHERE identifier = $1;

-- name: DeleteLoginAttempt :exec
DELETE FROM login_attempts WHERE identifier = $1;
