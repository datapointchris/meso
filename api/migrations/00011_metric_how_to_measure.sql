-- +goose Up

-- What the metric is and how to take the reading, as markdown.
--
-- A metric definition carried only the machinery a chart needs — unit, direction,
-- category — and nothing that says what the thing actually is. "Heel Raise Capacity
-- Right" names a number without naming the protocol that produces it, and the number
-- is worthless if the next reading is taken a different way. It also doesn't match
-- any movement in the library by name, so there was nowhere to go and look it up.
--
-- One field, not a description plus a how-to: for every metric in the vocabulary
-- those are the same sentence ("max single-leg heel raises to failure, knee straight,
-- full range"), and splitting them would only pose an unanswerable filing question.
-- The name says measure so the prose stays actionable rather than definitional.
--
-- NOT NULL with an empty-string default, matching label — reads stay branch-free per
-- the all-TEXT/no-null convention. Empty means undocumented, which is a real state
-- here: unlike label there is nothing sensible to derive one from.
ALTER TABLE metric_definitions ADD COLUMN how_to_measure TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE metric_definitions DROP COLUMN how_to_measure;
