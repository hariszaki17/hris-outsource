-- +goose Up
-- Single-use, time-limited password reset tokens (AU-4).
-- The plaintext is shown once (in the reset email / test helper); only the
-- SHA-256 hex hash is persisted so a DB leak cannot be replayed.
CREATE TABLE password_reset_tokens (
    id         bigserial    PRIMARY KEY,
    user_id    text         NOT NULL REFERENCES users(id),
    token_hash text         NOT NULL UNIQUE,   -- sha256 hex of the plaintext token
    expires_at timestamptz  NOT NULL,
    used_at    timestamptz,                    -- set when consumed; NULL = still live
    created_at timestamptz  NOT NULL DEFAULT now()
);

CREATE INDEX password_reset_tokens_hash_idx ON password_reset_tokens (token_hash);
CREATE INDEX password_reset_tokens_user_idx ON password_reset_tokens (user_id);

-- +goose Down
DROP TABLE IF EXISTS password_reset_tokens;
