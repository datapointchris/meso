-- +goose Up

-- Phase 5: Fitness log — the dated training journal, the substrate `meso review`
-- pulls when drafting the next cycle.
--
-- One row is a dated entry: a markdown body, free-form tags, and an optional mood.
-- id is a UUID7 minted by the API (google/uuid NewV7) — user-generated top-level
-- content, like workout_sessions, so the id is time-ordered and offline-creatable
-- (design: UUID7 for user-generated rows, identity for catalog rows). It is NOT a
-- catalog row: there is no natural key to upsert on, entries are appended over time.
--
-- entry_date defaults to today. tags is NOT NULL DEFAULT '{}' and normalized nil->[]
-- on write, so reads are branch-free (the movements.tags precedent). mood is the one
-- nullable field — an entry may carry a mood tag or not.
CREATE TABLE fitness_log_entries (
    id         UUID PRIMARY KEY,
    entry_date DATE NOT NULL DEFAULT CURRENT_DATE,
    body       TEXT NOT NULL DEFAULT '',
    tags       TEXT[] NOT NULL DEFAULT '{}',
    mood       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The list read is "entries, newest first, optionally windowed by date": an index
-- on entry_date serves the ORDER BY and the from/to filter.
CREATE INDEX fitness_log_entries_date_idx ON fitness_log_entries(entry_date DESC);

-- +goose Down

DROP TABLE fitness_log_entries;
