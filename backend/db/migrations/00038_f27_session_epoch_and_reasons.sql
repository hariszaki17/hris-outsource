-- +goose Up
-- F2.7 session epoch: bumping tokens_valid_after instantly invalidates every access
-- token issued before that instant. The per-request auth middleware rejects a token
-- whose iat < tokens_valid_after (and any non-active user), so offboarding revokes
-- access immediately without a per-token denylist. Default '-infinity' = no tokens
-- pre-revoked for existing users.
ALTER TABLE users ADD COLUMN tokens_valid_after timestamptz NOT NULL DEFAULT 'epoch';

-- F2.7 extended offboarding reasons: DECEASED, RETIRED, ABSCONDED (mangkir).
ALTER TABLE employment_agreements DROP CONSTRAINT employment_agreements_closed_reason_check;
ALTER TABLE employment_agreements ADD CONSTRAINT employment_agreements_closed_reason_check
    CHECK (closed_reason IN ('RESIGNED','TERMINATED','END_OF_TERM','DECEASED','RETIRED','ABSCONDED','OTHER')
           OR closed_reason IS NULL);

ALTER TABLE placements DROP CONSTRAINT placements_ended_reason_check;
ALTER TABLE placements ADD CONSTRAINT placements_ended_reason_check
    CHECK (ended_reason IN ('END_OF_TERM','ENDED','TERMINATED','RESIGNED','TRANSFERRED','SUPERSEDED','DECEASED','RETIRED','ABSCONDED')
           OR ended_reason IS NULL);

-- +goose Down
ALTER TABLE placements DROP CONSTRAINT placements_ended_reason_check;
ALTER TABLE placements ADD CONSTRAINT placements_ended_reason_check
    CHECK (ended_reason IN ('END_OF_TERM','ENDED','TERMINATED','RESIGNED','TRANSFERRED','SUPERSEDED')
           OR ended_reason IS NULL);
ALTER TABLE employment_agreements DROP CONSTRAINT employment_agreements_closed_reason_check;
ALTER TABLE employment_agreements ADD CONSTRAINT employment_agreements_closed_reason_check
    CHECK (closed_reason IN ('RESIGNED','TERMINATED','END_OF_TERM','OTHER')
           OR closed_reason IS NULL);
ALTER TABLE users DROP COLUMN tokens_valid_after;
