-- +goose Up

-- The terminal cycle status is `completed`.
--
-- Every other store in the fleet spells the end of a lifecycle `completed`, and a
-- reader moving between them should not have to learn that one of them means the
-- same state by a shorter word. The value is FK-guarded rather than free text, so
-- the vocabulary is a row and changing it is a data move, not a string edit.
--
-- Insert, repoint, delete — in that order, inside goose's transaction. The FK on
-- cycles.status is NO ACTION, so if the UPDATE misses a row the DELETE raises and
-- the whole migration rolls back. A cycle can never be left pointing at a status
-- that is gone.
--
-- cmd/seed carries the same vocabulary and upserts it on every deploy, so it must
-- agree with this file or the old row returns.

INSERT INTO cycle_statuses (name) VALUES ('completed') ON CONFLICT DO NOTHING;

UPDATE cycles SET status = 'completed' WHERE status = 'complete';

DELETE FROM cycle_statuses WHERE name = 'complete';

-- +goose Down

INSERT INTO cycle_statuses (name) VALUES ('complete') ON CONFLICT DO NOTHING;

UPDATE cycles SET status = 'complete' WHERE status = 'completed';

DELETE FROM cycle_statuses WHERE name = 'completed';
