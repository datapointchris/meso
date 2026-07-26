-- +goose Up

-- In-app feedback capture — a papercut or idea offloaded from the phone mid-session,
-- so a thought had at the gym survives the moment it was had.
--
-- meso owns this outright. It is not a queue that forwards somewhere: the row is
-- stored here, triaged here, and closed here. Nothing in this table refers to any
-- other system, and nothing about it needs another system to be reachable for the
-- capture to succeed. Whoever wants this data comes and reads it (`meso admin
-- feedback list --json`); meso never pushes it anywhere.
--
-- There is deliberately no kind/category column. Bug vs idea vs improvement is a
-- distinction that costs a tap at capture time and changes nothing at read time —
-- the body says what it is, and the response is the same either way. The app's
-- exclusion list applies to its own internals too.
--
-- id is a UUID7 minted by the API — user-generated append-only content, like
-- fitness_log_entries. status is a bounded sub-attribute (CHECK, not a 2-row lookup
-- table), matching measurements.source and metric_definitions.direction.
CREATE TABLE feedback (
    id           UUID PRIMARY KEY,
    status       TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'done')),
    body         TEXT NOT NULL,
    context_path TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The default read is "newest first"; the working read is "what's still open". A
-- partial index on the open rows stays small as closed feedback accumulates, which
-- is the case a partial index exists for.
CREATE INDEX feedback_created_idx ON feedback(created_at DESC);
CREATE INDEX feedback_open_idx ON feedback(created_at DESC) WHERE status = 'open';

-- +goose Down

DROP TABLE feedback;
