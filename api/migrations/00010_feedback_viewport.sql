-- +goose Up

-- The viewport the feedback was raised at, in CSS pixels.
--
-- Same test context_path passes: free to capture at the moment of the complaint,
-- unreconstructable afterwards. It earns its place because most feedback about a
-- mobile-first app is about layout, and "this is hard to read" means two different
-- defects at 390px (density) and at 1400px (line length past a readable measure).
-- Without it every layout report starts by guessing which one was on screen.
--
-- Nullable, not defaulted: feedback captured through `meso admin feedback add` has
-- no viewport at all, and 0 would read as a real measurement.
ALTER TABLE feedback
    ADD COLUMN viewport_width  INTEGER,
    ADD COLUMN viewport_height INTEGER;

-- +goose Down

ALTER TABLE feedback
    DROP COLUMN viewport_width,
    DROP COLUMN viewport_height;
