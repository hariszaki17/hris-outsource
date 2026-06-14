-- +goose Up
-- D2 (2026-06-07): login identifier is phone (required, unique) OR email
-- (optional, unique). Phone is the universal identifier — every agent has one,
-- many lack an email. So email loses its NOT NULL and phone is added as a
-- second unique, case-insensitive-for-email login key.
--
-- Phone is nullable at the column level (legacy/seed rows may predate it and the
-- backfill keys on it); the application layer enforces phone-required at create
-- (F2.1 EP-2). Stored normalized to E.164 (+62…) by the app.
ALTER TABLE users
    ALTER COLUMN email DROP NOT NULL,
    ADD COLUMN phone text;

-- Phone is unique among non-deleted users.
CREATE UNIQUE INDEX users_phone_active_uq
    ON users (phone)
    WHERE deleted_at IS NULL AND phone IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS users_phone_active_uq;
ALTER TABLE users
    DROP COLUMN IF EXISTS phone,
    ALTER COLUMN email SET NOT NULL;
