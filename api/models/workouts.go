package models

import "time"

// WorkoutMovement is one prescribed movement in a workout, in order. It embeds the
// movement's name and kind (denormalized on read) so a workout renders in a single
// GET without a second lookup per entry. The prescription fields (sets, reps, load,
// ...) are nullable — a bodyweight circuit may set none of them.
type WorkoutMovement struct {
	Sets          *int    `json:"sets"`
	Reps          *string `json:"reps"`
	Load          *string `json:"load"`
	RestSeconds   *int    `json:"rest_seconds"`
	SupersetGroup *string `json:"superset_group"`
	MovementName  string  `json:"movement_name"`
	MovementKind  string  `json:"movement_kind"`
	Notes         string  `json:"notes"`
	ID            int64   `json:"id"`
	MovementID    int64   `json:"movement_id"`
	Position      int     `json:"position"`
}

// WorkoutMovementInput is the write shape for one entry — used both when creating a
// workout with its movements in one call (seed/import) and when adding a single
// movement to an existing workout. Position is implied by array order on create and
// assigned server-side (append) on a single add.
type WorkoutMovementInput struct {
	Sets          *int    `json:"sets"`
	Reps          *string `json:"reps"`
	Load          *string `json:"load"`
	RestSeconds   *int    `json:"rest_seconds"`
	SupersetGroup *string `json:"superset_group"`
	Notes         string  `json:"notes"`
	MovementID    int64   `json:"movement_id"`
}

// WorkoutMovementUpdate is a partial update of one entry: a nil field is left
// unchanged. MovementID re-points the entry at a different movement — the swap
// action — carrying the existing prescription to the substitute for free.
type WorkoutMovementUpdate struct {
	MovementID    *int64  `json:"movement_id"`
	Sets          *int    `json:"sets"`
	Reps          *string `json:"reps"`
	Load          *string `json:"load"`
	RestSeconds   *int    `json:"rest_seconds"`
	SupersetGroup *string `json:"superset_group"`
	Notes         *string `json:"notes"`
}

// WorkoutMovementOrder is the reorder body: the entry ids in their desired order.
// The repository rewrites positions to 1..n from this list.
type WorkoutMovementOrder struct {
	EntryIDs []int64 `json:"entry_ids"`
}

// Workout is the full DB-owned record: a themed, ordered composition of movements.
// Movements is embedded on read (empty in list responses, populated on detail),
// mirroring how a movement embeds its muscles.
type Workout struct {
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	Theme            *string           `json:"theme"`
	EstimatedMinutes *int              `json:"estimated_minutes"`
	Name             string            `json:"name"`
	Notes            string            `json:"notes"`
	Tags             []string          `json:"tags"`
	Movements        []WorkoutMovement `json:"movements"`
	ID               int64             `json:"id"`
	Favorite         bool              `json:"favorite"`
}

// WorkoutCreate is the create body. Name is required (the handler validates).
// Movements may be supplied to build a complete workout in one call — the write
// path seed/import uses — or omitted and added later via the sub-resource.
type WorkoutCreate struct {
	Theme            *string                `json:"theme"`
	EstimatedMinutes *int                   `json:"estimated_minutes"`
	Name             string                 `json:"name"`
	Notes            string                 `json:"notes"`
	Tags             []string               `json:"tags"`
	Movements        []WorkoutMovementInput `json:"movements"`
	Favorite         bool                   `json:"favorite"`
}

// WorkoutUpdate is a partial update of workout-level fields; the movement list is
// managed through the sub-resource endpoints, not here. A nil pointer means "leave
// unchanged"; Tags uses a pointer-to-slice so an explicit empty array clears it.
type WorkoutUpdate struct {
	Name             *string   `json:"name"`
	Theme            *string   `json:"theme"`
	Tags             *[]string `json:"tags"`
	Notes            *string   `json:"notes"`
	Favorite         *bool     `json:"favorite"`
	EstimatedMinutes *int      `json:"estimated_minutes"`
}

// WorkoutFilter carries the optional list-endpoint query params. A zero value means
// "no filter"; Favorite is a pointer so ?favorite=false is distinct from unset.
type WorkoutFilter struct {
	Theme    string
	Tag      string
	Search   string
	Favorite *bool
}
