package repository

import "errors"

// ErrNotFound is returned when a lookup by id (or name) matches no row. Handlers
// map it to 404.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a write violates a uniqueness constraint (e.g.
// creating a movement whose name already exists). Handlers map it to 409.
var ErrConflict = errors.New("already exists")

// ErrReferenced is returned when a delete is blocked by a foreign-key reference
// from another table. Handlers map it to 409 instead of surfacing a raw FK
// violation. (No movement references exist in Phase 1; kept for the pattern and
// for Phase 2's workout_movements.)
var ErrReferenced = errors.New("referenced by other records")
