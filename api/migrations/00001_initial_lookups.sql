-- +goose Up

-- Categorical lookup tables (project rule: lookup tables with Text PRIMARY KEY
-- + FK, never enums). These three are foundational categoricals referenced
-- across later phases; richer lookups (metric_definitions, muscles) arrive with
-- the phases that own them. Rows are seed data, populated by cmd/seed — this
-- migration is DDL only.

CREATE TABLE movement_kinds (
    name TEXT PRIMARY KEY
);

CREATE TABLE relationship_kinds (
    name TEXT PRIMARY KEY
);

CREATE TABLE cycle_statuses (
    name TEXT PRIMARY KEY
);

-- +goose Down

DROP TABLE cycle_statuses;
DROP TABLE relationship_kinds;
DROP TABLE movement_kinds;
