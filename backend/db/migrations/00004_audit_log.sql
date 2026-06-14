-- +goose Up
-- Immutable audit trail written on EVERY mutation (CONVENTIONS §16.1), inside
-- the same transaction as the change. Queryable via E1 GET /audit-log.
CREATE TABLE audit_log (
    id            text PRIMARY KEY,              -- SWP-AL-<N>
    actor_user_id text,                          -- null for system/cron actions
    actor_role    text,
    action        text NOT NULL,                 -- CREATE/UPDATE/DELETE or domain verb
    entity_type   text NOT NULL,                 -- e.g. 'placement','leave_request'
    entity_id     text NOT NULL,                 -- SWP-… id of the affected resource
    before_state  jsonb,                         -- null on create
    after_state   jsonb,                         -- null on delete
    request_id    text,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_entity_idx ON audit_log (entity_type, entity_id);
CREATE INDEX audit_log_actor_idx ON audit_log (actor_user_id);
CREATE INDEX audit_log_created_idx ON audit_log (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
