-- +goose Up

-- Phase 6: Cycles (mesocycles) — an ordered sequence of workouts toward a goal.
--
-- A Cycle is the unit of training planning: a multi-week block aimed at a target
-- (a race date, a working weight, a symmetric knee-to-wall). Like a Workout it is a
-- catalog row (identity PK) with a natural key (name UNIQUE) so seed/import can
-- upsert idempotently, and its ordered contents live in a payload-bearing join
-- (cycle_workouts) read on every render.
--
-- status is FK-guarded by the cycle_statuses lookup seeded in cmd/seed
-- (planned|active|paused|complete). target_metric optionally points at a
-- metric_definition so a cycle's goal is a real tracked number, not prose; it SETs
-- NULL if that definition is ever removed. Both target_date and start_date are
-- nullable — a planned cycle can be drafted before it is scheduled (the humane,
-- "sliding reschedule" model where a cycle advances by readiness, not wall-clock).

CREATE TABLE cycles (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- name is the natural key: seed/import upsert on it, so it is unique even
    -- though id is the surrogate PK.
    name          TEXT NOT NULL UNIQUE,
    goal_summary  TEXT NOT NULL DEFAULT '',
    target_metric TEXT REFERENCES metric_definitions(name) ON DELETE SET NULL,
    target_value  NUMERIC,
    target_date   DATE,
    start_date    DATE,
    status        TEXT NOT NULL DEFAULT 'planned' REFERENCES cycle_statuses(name),
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX cycles_status_idx ON cycles(status);

-- The ordered, prescribed workouts of a cycle. A surrogate id addresses one entry
-- for edit/remove/swap; position orders the sequence. cycle_id cascades so deleting
-- a cycle clears its entries; workout_id RESTRICTs so a workout referenced by a
-- cycle cannot be deleted out from under it (surfaced as a 409). The per-entry
-- fields carry the periodization prescription: which week/phase it belongs to, how
-- often, at what effort, and the readiness condition that gates advancing.
--
-- UNIQUE(cycle_id, position) is DEFERRABLE so a reorder — which swaps positions
-- inside one transaction — is checked at commit rather than mid-update, matching
-- workout_movements.
CREATE TABLE cycle_workouts (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cycle_id   BIGINT NOT NULL REFERENCES cycles(id) ON DELETE CASCADE,
    workout_id BIGINT NOT NULL REFERENCES workouts(id) ON DELETE RESTRICT,
    position   INT NOT NULL,
    week       INT,
    phase      TEXT,
    frequency  TEXT,
    intensity  TEXT,
    conditions TEXT,
    CONSTRAINT cycle_workouts_position_uniq UNIQUE (cycle_id, position) DEFERRABLE INITIALLY IMMEDIATE
);

CREATE INDEX cycle_workouts_cycle_idx ON cycle_workouts(cycle_id);
CREATE INDEX cycle_workouts_workout_idx ON cycle_workouts(workout_id);

-- +goose Down

DROP TABLE cycle_workouts;
DROP TABLE cycles;
