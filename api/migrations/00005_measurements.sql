-- +goose Up

-- Phase 4: Measurements + Stats — the tracked stats time series.
--
-- A metric_definition names a thing worth tracking (a lift's working weight, a 5k
-- time, a knee-to-wall ROM) and carries the metadata a chart needs to render it
-- honestly: its unit, which direction is "better" (a heavier deadlift improves, a
-- faster 5k improves — opposite signs), and a coarse category for grouping. It is a
-- lookup table (name PK), the vocabulary a measurement references.
--
-- direction and category are bounded sub-attributes with a small, stable value set,
-- so they use a CHECK, not a 2-row/4-row lookup table (the movement_muscles.role
-- precedent — a CHECK is not an enum type). The canonical lookup tables stay the
-- domain-wide categoricals (movement_kinds, muscles, relationship_kinds,
-- cycle_statuses); these live inline on the definition they describe.
CREATE TABLE metric_definitions (
    name       TEXT PRIMARY KEY,
    unit       TEXT NOT NULL,
    direction  TEXT NOT NULL CHECK (direction IN ('higher_better', 'lower_better')),
    category   TEXT NOT NULL CHECK (category IN ('strength', 'cardio', 'mobility', 'body')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The value time series. One row is a dated reading of a metric. id is a surrogate
-- identity (a measurement is a lightweight append to a series, not a top-level user
-- aggregate needing an offline-first UUID7 — cf. session_movements). metric RESTRICTs
-- so a definition with recorded history cannot be dropped out from under its series.
--
-- source is manual today; the CHECK admits 'session' so a future pass can derive a
-- measurement from a logged session (design: "the source field leaves the door
-- open") without a schema change.
CREATE TABLE measurements (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    metric      TEXT NOT NULL REFERENCES metric_definitions(name) ON DELETE RESTRICT,
    value       NUMERIC NOT NULL,
    measured_on DATE NOT NULL DEFAULT CURRENT_DATE,
    source      TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'session')),
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The trend read is "this metric, ordered by date": a composite index on
-- (metric, measured_on) serves both the per-metric filter and the ordering.
CREATE INDEX measurements_metric_date_idx ON measurements(metric, measured_on);

-- +goose Down

DROP TABLE measurements;
DROP TABLE metric_definitions;
