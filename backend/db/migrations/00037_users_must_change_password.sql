-- +goose Up
-- EP-3 login provisioning: a freshly provisioned agent (or a regenerated temp
-- password) must rotate the temporary password on first login. Cleared when the
-- user sets their own password (change/reset). Show-once model — the temp password
-- itself is never stored, only its argon2id hash.
ALTER TABLE users ADD COLUMN must_change_password boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN must_change_password;
