package models

import (
	"time"

	"github.com/google/uuid"
)

// SessionMovement is one logged exercise within a session — the done checkbox and
// the real (actual) sets/reps/load. It embeds the movement's name and kind
// (denormalized on read) so a session renders in a single GET. Rows are seeded from
// the workout's workout_movements when a session starts from a template, then edited
// in place as the workout is performed. The actual_* fields are nullable (a set may
// be checked off without recording a number).
type SessionMovement struct {
	ActualSets   *int    `json:"actual_sets"`
	ActualReps   *string `json:"actual_reps"`
	ActualLoad   *string `json:"actual_load"`
	MovementName string  `json:"movement_name"`
	MovementKind string  `json:"movement_kind"`
	Notes        string  `json:"notes"`
	ID           int64   `json:"id"`
	MovementID   int64   `json:"movement_id"`
	Position     int     `json:"position"`
	Done         bool    `json:"done"`
}

// SessionMovementInput is the write shape for one logged movement when creating an
// ad-hoc session (no template). A session started from a workout copies its entries
// server-side instead, so this is only used for the template-less path.
type SessionMovementInput struct {
	ActualSets *int    `json:"actual_sets"`
	ActualReps *string `json:"actual_reps"`
	ActualLoad *string `json:"actual_load"`
	Notes      string  `json:"notes"`
	MovementID int64   `json:"movement_id"`
	Done       bool    `json:"done"`
}

// SessionMovementUpdate is a partial update of one logged entry: a nil field is left
// unchanged. Done toggles the checkbox; actual_* record the real numbers. MovementID
// re-points the entry at an alternate (the mid-session swap), carrying the actuals to
// the substitute — the design's "target carries over to the swap."
type SessionMovementUpdate struct {
	MovementID *int64  `json:"movement_id"`
	Done       *bool   `json:"done"`
	ActualSets *int    `json:"actual_sets"`
	ActualReps *string `json:"actual_reps"`
	ActualLoad *string `json:"actual_load"`
	Notes      *string `json:"notes"`
}

// WorkoutSession is a workout performed on a date — the instance, distinct from the
// Workout template. WorkoutName is the template's name (null for an ad-hoc session
// or one whose template was deleted). Movements is embedded on read (empty in list
// responses, populated on detail). PerformedOn is a calendar date on the wire
// ("2006-01-02"), stored in a Postgres DATE column.
type WorkoutSession struct {
	CreatedAt       time.Time         `json:"created_at"`
	WorkoutID       *int64            `json:"workout_id"`
	WorkoutName     *string           `json:"workout_name"`
	DurationMinutes *int              `json:"duration_minutes"`
	Felt            *string           `json:"felt"`
	PerformedOn     string            `json:"performed_on"`
	OverallNotes    string            `json:"overall_notes"`
	Movements       []SessionMovement `json:"movements"`
	ID              uuid.UUID         `json:"id"`
}

// WorkoutSessionCreate starts a session. If WorkoutID is set, the workout's
// prescribed movements are copied in as the session's movements (start-from-template,
// the primary flow); otherwise Movements builds an ad-hoc session. PerformedOn
// defaults to today when empty.
type WorkoutSessionCreate struct {
	WorkoutID       *int64                 `json:"workout_id"`
	DurationMinutes *int                   `json:"duration_minutes"`
	Felt            *string                `json:"felt"`
	PerformedOn     string                 `json:"performed_on"`
	OverallNotes    string                 `json:"overall_notes"`
	Movements       []SessionMovementInput `json:"movements"`
}

// WorkoutSessionUpdate is a partial update of session-level fields; the logged
// movements are managed through the sub-resource PATCH. A nil pointer means "leave
// unchanged".
type WorkoutSessionUpdate struct {
	PerformedOn     *string `json:"performed_on"`
	DurationMinutes *int    `json:"duration_minutes"`
	OverallNotes    *string `json:"overall_notes"`
	Felt            *string `json:"felt"`
}

// WorkoutSessionFilter carries the optional list-endpoint query params. From and To
// bound performed_on (inclusive) as "2006-01-02" strings; WorkoutID scopes to one
// template's sessions.
type WorkoutSessionFilter struct {
	WorkoutID *int64
	From      string
	To        string
}
