-- +goose Up
CREATE TABLE login_attempts (
    id bigint PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    identifier text NOT NULL UNIQUE,
    attempt_count int NOT NULL DEFAULT 0,
    locked_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX login_attempts_locked_until_idx ON login_attempts (locked_until) WHERE locked_until IS NOT NULL;

-- +goose Down
DROP TABLE login_attempts;
