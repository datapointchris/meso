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
// violation. A movement used by a workout_movements row cannot be deleted.
var ErrReferenced = errors.New("referenced by other records")

// ErrInvalid is returned when a request is semantically invalid in a way the
// repository is best placed to detect (e.g. relating a movement to itself, or a
// reorder whose entry set does not match the workout). Handlers map it to 400.
var ErrInvalid = errors.New("invalid request")
