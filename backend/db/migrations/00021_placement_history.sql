-- +goose Up
-- Placement lifecycle history (E3 / CONTEXT "history on every action").
-- One row per lifecycle transition (create/renew/transfer/end/resign/terminate/
-- supersede). Uses a bigserial PK (no SWP id needed — avoids touching ids.go).
CREATE TABLE placement_history (
    id                bigserial PRIMARY KEY,
    placement_id      text NOT NULL REFERENCES placements(id),
    action            text NOT NULL,                 -- 'create','renew','transfer','end','resign','terminate','supersede'
    actor_user_id     text,                          -- SWP-USR-<N>; null for system
    reason            text,
    effective_date    date,
    status_before     text,
    status_after      text,
    notes             text,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX placement_history_placement_idx ON placement_history (placement_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS placement_history;
