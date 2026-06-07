-- +goose Up
-- Repair: 00038 originally created tokens_valid_after with DEFAULT '-infinity',
-- which pgx cannot scan into a Go time.Time (every authed request 401'd). Move to
-- 'epoch' and rewrite any rows still holding the infinity sentinel.
ALTER TABLE users ALTER COLUMN tokens_valid_after SET DEFAULT 'epoch';
UPDATE users SET tokens_valid_after = 'epoch' WHERE tokens_valid_after = '-infinity';

-- +goose Down
ALTER TABLE users ALTER COLUMN tokens_valid_after SET DEFAULT '-infinity';
