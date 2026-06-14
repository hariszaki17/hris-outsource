-- +goose Up
-- Rotating opaque refresh tokens. Only the SHA-256 hash is stored. Each
-- /auth/refresh rotates the token; rotated_from links the chain so a reused
-- (already-rotated or revoked) token can revoke the whole family — standard
-- refresh-token reuse detection.
CREATE TABLE refresh_tokens (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      text NOT NULL REFERENCES users(id),
    token_hash   text NOT NULL UNIQUE,          -- sha256 hex of the plaintext
    family_id    text NOT NULL,                 -- shared across a rotation chain
    rotated_from bigint REFERENCES refresh_tokens(id),
    user_agent   text,
    ip           text,
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz,                   -- set on rotation or logout
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
