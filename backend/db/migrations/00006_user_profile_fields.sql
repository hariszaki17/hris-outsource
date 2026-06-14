-- +goose Up
-- Add profile fields required by the OpenAPI LoginResponse/MeResponse contract.
-- full_name: denormalized from the Employee (convenience field for auth responses).
-- last_login_at: set on every successful login (AU-3); null on first login.
ALTER TABLE users
    ADD COLUMN full_name     text        NOT NULL DEFAULT '',
    ADD COLUMN last_login_at timestamptz;

-- +goose Down
ALTER TABLE users
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS full_name;
