-- +goose Up

-- Phase 7: what actually happened, one row per set.
--
-- session_movements carried the plan and the performance in the same columns. actual_*
-- was seeded from the workout's prescription when the session started, then overwritten
-- as the session was performed, so the plan was unrecoverable the moment anything
-- diverged — and divergence itself could only be expressed as an unchecked box.
-- Training that went differently than programmed read as training that failed.
--
-- Splitting them: session_movements.target_* is the prescription (copied from the
-- workout, or empty when free-form), and session_sets is the record of performance.
-- Doing four sets of a programmed three is now a fact rather than an overwrite, and
-- the done flag has something to derive itself from.

-- Set vocabulary. This is what lets a set carry reps as a real INT: "AMRAP" is a kind,
-- and a 30s hold is hold_seconds, so neither needs the free text that INT displaces.
CREATE TABLE set_kinds (
    name TEXT PRIMARY KEY
);

-- How a movement is loaded, which decides what the logging screen asks for. A weight
-- box on an ab exercise is a question with no answer. movement_kind cannot stand in —
-- a back squat and a nordic curl are both 'exercise'. Neither can equipment: it is
-- free-form and empty is its default, so it cannot tell bodyweight from unfilled.
CREATE TABLE load_modes (
    name TEXT PRIMARY KEY
);

-- Vocabulary rows belong to cmd/seed by project rule, but the FK-backed backfill below
-- needs them to exist now. Idempotent, so the seeder stays authoritative for a fresh
-- database without conflicting here.
INSERT INTO set_kinds (name) VALUES ('working'), ('warmup'), ('amrap'), ('drop'), ('failure')
ON CONFLICT DO NOTHING;

INSERT INTO load_modes (name) VALUES ('weighted'), ('bodyweight'), ('timed'), ('assisted')
ON CONFLICT DO NOTHING;

ALTER TABLE movements ADD COLUMN load_mode TEXT NOT NULL DEFAULT 'weighted' REFERENCES load_modes(name);

CREATE INDEX movements_load_mode_idx ON movements(load_mode);

-- A heuristic, not an answer: an 'exercise' with equipment listed is assumed weighted,
-- which is wrong for every bodyweight movement that names a bar or a bench. Correcting
-- those is what `meso movements update --load-mode` is for.
UPDATE movements SET load_mode = CASE
    WHEN movement_kind IN ('stretch', 'yoga_pose') THEN 'timed'
    WHEN cardinality(equipment) = 0 OR 'bodyweight' = ANY(equipment) THEN 'bodyweight'
    ELSE 'weighted'
END;

-- finished_at separates a session being logged right now from one that is history.
-- Nullable timestamp rather than a status column: it answers "is this in progress" and
-- "how long did it take" at once, so duration_minutes stops being typed by hand.
--
-- Finishing means "I am done training", not "I completed the plan" — nothing compares
-- it to the targets.
ALTER TABLE workout_sessions ADD COLUMN finished_at TIMESTAMPTZ;

-- Every session predating this column is history, not a log left open forever.
UPDATE workout_sessions SET finished_at = created_at;

ALTER TABLE session_movements RENAME COLUMN actual_sets TO target_sets;
ALTER TABLE session_movements RENAME COLUMN actual_reps TO target_reps;
ALTER TABLE session_movements RENAME COLUMN actual_load TO target_load;

-- Deferrable to match workout_movements and cycle_workouts, which is what a reorder
-- needs to swap two positions inside one transaction. It was the only one of the three
-- ordered joins without it.
ALTER TABLE session_movements DROP CONSTRAINT session_movements_position_uniq;
ALTER TABLE session_movements ADD CONSTRAINT session_movements_position_uniq
    UNIQUE (session_id, position) DEFERRABLE INITIALLY IMMEDIATE;

-- One row per set performed, cascading from the entry it belongs to.
--
-- load stays TEXT to match the prescription it is measured against: "80% 1RM" and
-- "2 plates" are the vocabulary here and no NUMERIC column could hold them. logged_at
-- is what makes the set list a timeline rather than a form.
CREATE TABLE session_sets (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_movement_id BIGINT NOT NULL REFERENCES session_movements(id) ON DELETE CASCADE,
    position            INT NOT NULL,
    reps                INT,
    load                TEXT,
    hold_seconds        INT,
    set_kind            TEXT NOT NULL DEFAULT 'working' REFERENCES set_kinds(name),
    notes               TEXT NOT NULL DEFAULT '',
    logged_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT session_sets_position_uniq UNIQUE (session_movement_id, position) DEFERRABLE INITIALLY IMMEDIATE
);

CREATE INDEX session_sets_movement_idx ON session_sets(session_movement_id);

-- Materialize the old aggregates so existing history survives the split: an entry that
-- recorded 3 sets becomes three rows.
--
-- Only done entries. Every session started from a template arrived with actual_*
-- already filled from the prescription, so materializing the rest would invent
-- performances that never happened — the same reason attachPreviousActuals has always
-- filtered on done.
--
-- reps is taken only where the old free text was a plain number; anything else ("8–10",
-- "AMRAP") is preserved verbatim in the set's notes rather than guessed at or dropped.
--
-- A done entry that never recorded a count still gets one set. It was ticked off, so
-- something was performed, and one set carrying the recorded load is closer to the truth
-- than no record at all — which would drop the entry out of previous-actuals entirely.
INSERT INTO session_sets (session_movement_id, position, reps, load, notes)
SELECT sm.id,
    g.position,
    CASE WHEN sm.target_reps ~ '^\d+$' THEN sm.target_reps::INT END,
    sm.target_load,
    CASE WHEN sm.target_reps IS NOT NULL AND sm.target_reps !~ '^\d+$' THEN sm.target_reps ELSE '' END
FROM session_movements sm
CROSS JOIN LATERAL generate_series(1, GREATEST(COALESCE(sm.target_sets, 1), 1)) AS g(position)
WHERE sm.done;

-- +goose Down

DROP TABLE session_sets;

ALTER TABLE session_movements DROP CONSTRAINT session_movements_position_uniq;
ALTER TABLE session_movements ADD CONSTRAINT session_movements_position_uniq
    UNIQUE (session_id, position);

ALTER TABLE session_movements RENAME COLUMN target_load TO actual_load;
ALTER TABLE session_movements RENAME COLUMN target_reps TO actual_reps;
ALTER TABLE session_movements RENAME COLUMN target_sets TO actual_sets;

ALTER TABLE workout_sessions DROP COLUMN finished_at;

ALTER TABLE movements DROP COLUMN load_mode;

DROP TABLE load_modes;
DROP TABLE set_kinds;
