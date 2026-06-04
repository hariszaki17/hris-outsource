-- name: ListPlatformSettings :many
SELECT key, value, label, locked FROM platform_settings ORDER BY sort ASC;
