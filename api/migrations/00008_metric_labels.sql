-- +goose Up

-- A metric's name is its natural key — slug-shaped so the CLI can address it
-- (`meso measurements record back-squat-working-weight`) and so it survives as a
-- stable identifier. That makes it a poor thing to put in front of a human: the
-- record modal was offering "back-squat-working-weight" as the picker option.
--
-- label is the display string, edited independently of the key. It is NOT NULL with
-- an empty-string default so reads stay branch-free (the all-TEXT/no-null convention
-- the array columns already follow); the API fills a derived label on define, so an
-- empty label is not a state the app produces.
ALTER TABLE metric_definitions ADD COLUMN label TEXT NOT NULL DEFAULT '';

-- Backfill the existing vocabulary from the key: initcap over the de-hyphenated
-- name, the same derivation the API applies to a define with no explicit --label.
-- ("back-squat-working-weight" → "Back Squat Working Weight", "5k-time" → "5k Time".)
UPDATE metric_definitions SET label = initcap(replace(name, '-', ' ')) WHERE label = '';

-- +goose Down

ALTER TABLE metric_definitions DROP COLUMN label;
