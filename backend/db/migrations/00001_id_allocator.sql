-- +goose Up
-- Per-prefix monotonic ID allocator backing the SWP-<ENTITY>-<N> convention
-- (CONVENTIONS §4). Gaps are allowed (soft-deletes). swp_next_id() is called
-- inside the same transaction as the insert so allocation is atomic with it.
CREATE TABLE id_counters (
    prefix   text   PRIMARY KEY,
    next_val bigint NOT NULL DEFAULT 0
);

-- +goose StatementBegin
CREATE FUNCTION swp_next_id(p_prefix text) RETURNS bigint AS $$
DECLARE
    v bigint;
BEGIN
    INSERT INTO id_counters (prefix, next_val)
    VALUES (p_prefix, 1)
    ON CONFLICT (prefix)
    DO UPDATE SET next_val = id_counters.next_val + 1
    RETURNING next_val INTO v;
    RETURN v;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS swp_next_id(text);
DROP TABLE IF EXISTS id_counters;
