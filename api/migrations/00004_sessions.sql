-- +goose Up

-- Phase 3: Sessions (logging) — the tracked reality against the plan.
--
-- A WorkoutSession is a workout performed on a date (the instance), kept distinct
-- from the Workout template so history is real data, not an edited plan. It is a
-- user-generated row, so its PK is a UUID7 minted in Go — time-ordered, so
-- recent-session reads and the `meso review` capstone stay index-friendly, and the
-- id can be generated client-side when offline logging lands (v1 is online-only).
--
-- workout_id is nullable and SET NULL on delete: deleting a template must not erase
-- the logged history of having done it (CASCADE) nor be forbidden by it (RESTRICT) —
-- the session survives as an ad-hoc record of what was actually performed.

CREATE TABLE workout_sessions (
    id               UUID PRIMARY KEY,
    workout_id       BIGINT REFERENCES workouts(id) ON DELETE SET NULL,
    performed_on     DATE NOT NULL DEFAULT CURRENT_DATE,
    duration_minutes INT,
    overall_notes    TEXT NOT NULL DEFAULT '',
    felt             TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX workout_sessions_performed_on_idx ON workout_sessions(performed_on DESC);
CREATE INDEX workout_sessions_workout_idx ON workout_sessions(workout_id);

-- The per-exercise actuals — the done checkbox and real sets/reps/load. Seeded from
-- the workout's workout_movements when a session starts from a template (the
-- prescription copied into actual_* as a starting point), then edited in place as
-- the workout is performed. Like workout_movements it is an ordered join with a
-- surrogate identity id addressing one entry for the done/actuals PATCH.
--
-- session_id cascades so deleting a session clears its logged movements; movement_id
-- RESTRICTs so a movement referenced by logged history cannot be deleted (409).
CREATE TABLE session_movements (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id   UUID NOT NULL REFERENCES workout_sessions(id) ON DELETE CASCADE,
    movement_id  BIGINT NOT NULL REFERENCES movements(id) ON DELETE RESTRICT,
    position     INT NOT NULL,
    done         BOOLEAN NOT NULL DEFAULT false,
    actual_sets  INT,
    actual_reps  TEXT,
    actual_load  TEXT,
    notes        TEXT NOT NULL DEFAULT '',
    CONSTRAINT session_movements_position_uniq UNIQUE (session_id, position)
);

CREATE INDEX session_movements_session_idx ON session_movements(session_id);
CREATE INDEX session_movements_movement_idx ON session_movements(movement_id);

-- +goose Down

DROP TABLE session_movements;
DROP TABLE workout_sessions;
