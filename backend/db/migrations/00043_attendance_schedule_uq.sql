-- +goose Up
-- E5 attendance — idempotency backstop for the absence-sweep cron + future clock-in.
-- A scheduled shift may have AT MOST ONE attendance row. The absence-sweep writes an
-- ABSENT row for every scheduled shift that ended (plus a grace) without a clock-in;
-- a partial UNIQUE index on schedule_id is the race-proof guard so a re-run (or a
-- concurrent real clock-in) cannot double-insert. The sweep's INSERT relies on this
-- index for its ON CONFLICT (schedule_id) ... DO NOTHING inference.
--
-- Partial: only NON-deleted rows that ARE schedule-linked participate. schedule_id is
-- NULLABLE (unscheduled walk-ins) — those are intentionally exempt (NULLs never
-- collide), and soft-deleted rows are excluded so a corrected/re-created row is allowed.
--
-- Seed safety: the seed writes exactly one attendance row per schedule_id (the ABSENT
-- fixture included), so no existing data violates this. If a future seed/migration
-- introduces a duplicate live schedule_id this CREATE will fail loudly — that is the
-- intended signal (find the dupe with:
--   SELECT schedule_id, count(*) FROM attendance
--   WHERE schedule_id IS NOT NULL AND deleted_at IS NULL
--   GROUP BY schedule_id HAVING count(*) > 1;).
CREATE UNIQUE INDEX attendance_schedule_uq
    ON attendance (schedule_id)
    WHERE schedule_id IS NOT NULL AND deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS attendance_schedule_uq;
