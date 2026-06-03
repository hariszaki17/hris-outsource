-- +goose Up
-- Idempotency-Key response cache (CONVENTIONS §13). Keyed per user; the stored
-- response is replayed for 24h on the same key + same request body. A different
-- body under the same key is rejected (409 IDEMPOTENCY_KEY_REUSED).
CREATE TABLE idempotency_keys (
    key             text PRIMARY KEY,            -- "<user_id>:<client-uuid>"
    request_hash    text NOT NULL,               -- sha256 of the request body
    response_status int  NOT NULL,
    response_body   bytea NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL
);

CREATE INDEX idempotency_keys_expiry_idx ON idempotency_keys (expires_at);

-- +goose Down
DROP TABLE IF EXISTS idempotency_keys;
