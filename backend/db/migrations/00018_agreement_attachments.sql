-- +goose Up
-- File attachments for employment agreements (E2 F2.2 / CONVENTIONS §15).
-- IDs allocated inline: 'SWP-FILE-' || swp_next_id('FILE').
-- Storage choice: bytea blob in-DB — simplest approach that passes E2E and
-- survives container teardown via reseed. No external storage dependency.
CREATE TABLE agreement_attachments (
    id          text PRIMARY KEY,                   -- SWP-FILE-<N>
    agreement_id text NOT NULL REFERENCES employment_agreements(id),
    category    text NOT NULL DEFAULT 'signed_agreement'
                    CHECK (category IN ('signed_agreement', 'addendum', 'supporting_doc')),
    caption     text NOT NULL DEFAULT '',
    file_name   text NOT NULL,
    mime        text NOT NULL,
    size_bytes  bigint NOT NULL,
    blob        bytea NOT NULL,                     -- file content (max 10 MB enforced in handler)
    uploaded_by text,                               -- SWP-USR-<N> of uploader
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS agreement_attachments;
