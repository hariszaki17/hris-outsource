-- +goose Up
-- E3 product decision 2026-06-11: the employment agreement ("perjanjian") is now
-- OPTIONAL at placement-create time. An agent may be placed before their PKWT/PKWTT
-- paperwork is finalized; such placements are tracked as "pending agreement"
-- (agreement_id IS NULL → derived awaiting_agreement = true) and the agreement is
-- backfilled later via POST /placements/{id}/agreement.
--
-- awaiting_agreement is an ORTHOGONAL compliance flag, NOT a lifecycle state — the
-- placement state machine is untouched. The FK is kept (a present agreement_id must
-- still reference a real employment_agreements row).
ALTER TABLE placements ALTER COLUMN agreement_id DROP NOT NULL;

-- +goose Down
-- Restoring NOT NULL fails if any pending-agreement rows exist; the operator must
-- backfill or delete them first (this is intentional — we never silently drop data).
ALTER TABLE placements ALTER COLUMN agreement_id SET NOT NULL;
