-- +goose Up

-- Phase 2: Workouts and movement relationships.
--
-- A Workout is an ordered, themed composition of movements (the "plan"); its
-- ordered list of prescribed movements lives in workout_movements — a
-- payload-bearing join carrying order + prescription, read on every render. A
-- workout is a catalog row (identity PK) with a natural key (name UNIQUE) so
-- seed/import can upsert idempotently, mirroring movements.
--
-- movement_relationships is the self-referential alternate/antagonist join
-- deferred from Phase 1 — it earns a table because swapping a movement for an
-- alternate while building a workout is a concrete, query-driven UI action.

CREATE TABLE workouts (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- name is the natural key: seed/import upsert on it, so it is unique even
    -- though id is the surrogate PK.
    name              TEXT NOT NULL UNIQUE,
    theme             TEXT,
    tags              TEXT[] NOT NULL DEFAULT '{}',
    notes             TEXT NOT NULL DEFAULT '',
    favorite          BOOLEAN NOT NULL DEFAULT false,
    estimated_minutes INT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX workouts_favorite_idx ON workouts(favorite) WHERE favorite;

-- The ordered, prescribed movements of a workout. A surrogate id addresses one
-- entry for edit/remove/swap; position orders the render. workout_id cascades so
-- deleting a workout clears its entries; movement_id RESTRICTs so a movement
-- referenced by a workout cannot be deleted out from under it (surfaced as a 409).
--
-- UNIQUE(workout_id, position) is DEFERRABLE so a reorder — which swaps two
-- positions inside one transaction — is checked at commit rather than mid-update,
-- where an intermediate collision would otherwise trip an immediate constraint.
CREATE TABLE workout_movements (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    workout_id     BIGINT NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    movement_id    BIGINT NOT NULL REFERENCES movements(id) ON DELETE RESTRICT,
    position       INT NOT NULL,
    sets           INT,
    reps           TEXT,
    load           TEXT,
    rest_seconds   INT,
    superset_group TEXT,
    notes          TEXT NOT NULL DEFAULT '',
    CONSTRAINT workout_movements_position_uniq UNIQUE (workout_id, position) DEFERRABLE INITIALLY IMMEDIATE
);

CREATE INDEX workout_movements_workout_idx ON workout_movements(workout_id);
CREATE INDEX workout_movements_movement_idx ON workout_movements(movement_id);

-- Directional self-referential relationships between movements. relationship_kind
-- is FK-guarded by the lookup seeded in 00001 (alternate|antagonist|progression|
-- regression|see_also). Both endpoints cascade so deleting a movement clears the
-- relationships that name it. The CHECK forbids a movement relating to itself.
CREATE TABLE movement_relationships (
    movement_id         BIGINT NOT NULL REFERENCES movements(id) ON DELETE CASCADE,
    related_movement_id BIGINT NOT NULL REFERENCES movements(id) ON DELETE CASCADE,
    relationship_kind   TEXT NOT NULL REFERENCES relationship_kinds(name),
    PRIMARY KEY (movement_id, related_movement_id, relationship_kind),
    CONSTRAINT movement_relationships_not_self CHECK (movement_id <> related_movement_id)
);

CREATE INDEX movement_relationships_related_idx ON movement_relationships(related_movement_id);

-- +goose Down

DROP TABLE movement_relationships;
DROP TABLE workout_movements;
DROP TABLE workouts;
