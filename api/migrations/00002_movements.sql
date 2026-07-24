-- +goose Up

-- Phase 1: the Movement library — exercises, stretches, and yoga poses unified
-- under one table keyed by a movement_kind lookup (see 00001). Muscles are
-- lookup-backed (not free tags) so region filtering and honest primary/secondary
-- tagging are queryable; movement_muscles is the payload-bearing join. Movement
-- *relationships* (the self-ref alternate/antagonist join) arrive in Phase 2.

-- Muscle catalog. region groups muscles for coarse filtering ("posterior chain")
-- and leaves the door open to a body-map UI. Rows are seed data (cmd/seed).
CREATE TABLE muscles (
    name   TEXT PRIMARY KEY,
    region TEXT NOT NULL
);

CREATE TABLE movements (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- name is the natural key for catalog rows: the seed/import upserts on it and
    -- the UI dedupes by it, so it is unique even though id is the surrogate PK.
    name                 TEXT NOT NULL UNIQUE,
    movement_kind        TEXT NOT NULL REFERENCES movement_kinds(name),
    favorite             BOOLEAN NOT NULL DEFAULT false,
    rating               INT CHECK (rating BETWEEN 1 AND 5),
    tags                 TEXT[] NOT NULL DEFAULT '{}',
    equipment            TEXT[] NOT NULL DEFAULT '{}',
    how_to               TEXT NOT NULL DEFAULT '',
    form_cues            TEXT NOT NULL DEFAULT '',
    common_faults        TEXT NOT NULL DEFAULT '',
    -- kind-specific, nullable and used as appropriate: a lift carries a default
    -- rep scheme, a pose a hold and a Sanskrit name.
    default_sets         INT,
    default_reps         TEXT,
    default_hold_seconds INT,
    sanskrit_name        TEXT,
    measurable_rom       BOOLEAN NOT NULL DEFAULT false,
    source_url           TEXT,
    source_name          TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX movements_kind_idx ON movements(movement_kind);
CREATE INDEX movements_favorite_idx ON movements(favorite) WHERE favorite;

-- The primary/secondary muscle tagging for each movement. role is a bounded
-- sub-attribute of the join (primary|secondary), guarded by a CHECK rather than a
-- 2-row lookup — it is not a first-class categorical entity. movement_id cascades
-- so deleting a movement clears its tags; muscle is FK-guarded so only known
-- muscles can be attached.
CREATE TABLE movement_muscles (
    movement_id BIGINT NOT NULL REFERENCES movements(id) ON DELETE CASCADE,
    muscle      TEXT NOT NULL REFERENCES muscles(name),
    role        TEXT NOT NULL CHECK (role IN ('primary', 'secondary')),
    PRIMARY KEY (movement_id, muscle, role)
);

CREATE INDEX movement_muscles_muscle_idx ON movement_muscles(muscle);

-- +goose Down

DROP TABLE movement_muscles;
DROP TABLE movements;
DROP TABLE muscles;
