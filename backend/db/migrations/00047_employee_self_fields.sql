-- +goose Up
-- E2 EP-5 (agent self-service): self-editable profile fields surfaced on the
-- redesigned agent web console "Akun" screen.
-- - emergency_contact_{name,phone}: new approval-tier field (routed via change request).
-- - app_language: instant-tier UI language preference (Bahasa default 'id').
-- - photo_object_key: server-built key into the MinIO private bucket
--   (profile-photos/{employee_id}/{ulid}.{ext}); presigned GET resolves photo_url.
ALTER TABLE employees
    ADD COLUMN emergency_contact_name  text,
    ADD COLUMN emergency_contact_phone text,
    ADD COLUMN app_language            text NOT NULL DEFAULT 'id'
                                            CHECK (app_language IN ('id', 'en')),
    ADD COLUMN photo_object_key        text;

-- +goose Down
ALTER TABLE employees
    DROP COLUMN photo_object_key,
    DROP COLUMN app_language,
    DROP COLUMN emergency_contact_phone,
    DROP COLUMN emergency_contact_name;
