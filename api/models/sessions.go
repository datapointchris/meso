package models

import (
	"time"

	"github.com/google/uuid"
)

// SessionSet is one set as it was performed. Sets are the record of what happened;
// the entry's target_* is the plan they are measured against, and the two are
// deliberately not the same fields.
//
// Load stays free text to match the prescription ("80% 1RM", "2 plates"). Reps is an
// int because SetKind absorbs "AMRAP" and HoldSeconds absorbs "30s", so nothing is
// left for prose to carry. Every value is nullable: a set that happened is worth
// recording even when nothing about it was measured.
type SessionSet struct {
	LoggedAt    time.Time `json:"logged_at"`
	Reps        *int      `json:"reps"`
	Load        *string   `json:"load"`
	HoldSeconds *int      `json:"hold_seconds"`
	SetKind     string    `json:"set_kind"`
	Notes       string    `json:"notes"`
	ID          int64     `json:"id"`
	Position    int       `json:"position"`
}

// SessionSetInput logs one set. Every field is optional: an empty body carries the
// previous set forward, which is what makes logging a set one tap. SetKind defaults
// to "working" when empty.
type SessionSetInput struct {
	Reps        *int    `json:"reps"`
	Load        *string `json:"load"`
	HoldSeconds *int    `json:"hold_seconds"`
	SetKind     string  `json:"set_kind"`
	Notes       string  `json:"notes"`
}

// SessionSetUpdate is a partial update of one logged set: a nil field is left
// unchanged.
type SessionSetUpdate struct {
	Reps        *int    `json:"reps"`
	Load        *string `json:"load"`
	HoldSeconds *int    `json:"hold_seconds"`
	SetKind     *string `json:"set_kind"`
	Notes       *string `json:"notes"`
}

// SessionMovement is one movement within a session: the target it was performed
// against, and the sets that were actually performed. It embeds the movement's name,
// kind and load mode (denormalized on read) so a session renders in a single GET —
// LoadMode is what tells the logging screen whether to ask for a weight at all.
//
// Target_* is the prescription, copied from the workout when the session starts from a
// template and left empty when it doesn't. It is never overwritten by performance;
// that is what Sets is for.
//
// Done is derived: it flips true once the logged sets reach TargetSets. It stays
// writable so stopping early on purpose can be recorded as a decision.
type SessionMovement struct {
	TargetSets   *int             `json:"target_sets"`
	TargetReps   *string          `json:"target_reps"`
	TargetLoad   *string          `json:"target_load"`
	Previous     *PreviousActuals `json:"previous"`
	Sets         []SessionSet     `json:"sets"`
	MovementName string           `json:"movement_name"`
	MovementKind string           `json:"movement_kind"`
	LoadMode     string           `json:"load_mode"`
	Notes        string           `json:"notes"`
	ID           int64            `json:"id"`
	MovementID   int64            `json:"movement_id"`
	Position     int              `json:"position"`
	Done         bool             `json:"done"`
}

// PreviousActuals is the last recorded performance of a movement before the session
// being viewed — the number to beat, shown inline next to each input on the logging
// screen. Null when the movement has never been performed. Populated on session detail
// only; list responses leave it nil.
//
// Sets is how many sets were logged and Reps/Load come from the last of them, which is
// the working set once warmups are in the same list.
type PreviousActuals struct {
	Reps        *int    `json:"reps"`
	Load        *string `json:"load"`
	PerformedOn string  `json:"performed_on"`
	Sets        int     `json:"sets"`
}

// SessionMovementInput is the write shape for adding a movement to a session — the
// entries of a free-form session at create time, and every movement appended to a
// session afterwards. A session started from a workout copies its entries server-side
// instead, so nothing on the template path sends this.
//
// The target_* fields exist for the case where a movement is added mid-session with an
// intention ("3 × 8 at 100") rather than logged blind; sets are logged separately.
type SessionMovementInput struct {
	TargetSets *int    `json:"target_sets"`
	TargetReps *string `json:"target_reps"`
	TargetLoad *string `json:"target_load"`
	Notes      string  `json:"notes"`
	MovementID int64   `json:"movement_id"`
	Done       bool    `json:"done"`
}

// SessionPromote is the write shape for turning a free-form session into a reusable
// workout template: the session's logged sets become the prescription, so the only
// thing the caller supplies is how to label the workout. Name is required and must be
// unique (workouts.name is the natural key).
type SessionPromote struct {
	Theme            *string  `json:"theme"`
	EstimatedMinutes *int     `json:"estimated_minutes"`
	Name             string   `json:"name"`
	Notes            string   `json:"notes"`
	Tags             []string `json:"tags"`
}

// SessionMovementUpdate is a partial update of one logged entry: a nil field is left
// unchanged. Done overrides the derived value; target_* edit the plan for this session
// without touching the workout it came from. MovementID re-points the entry at an
// alternate (the mid-session swap), carrying the target and the already-logged sets to
// the substitute — the design's "target carries over to the swap."
type SessionMovementUpdate struct {
	MovementID *int64  `json:"movement_id"`
	Done       *bool   `json:"done"`
	TargetSets *int    `json:"target_sets"`
	TargetReps *string `json:"target_reps"`
	TargetLoad *string `json:"target_load"`
	Notes      *string `json:"notes"`
}

// WorkoutSession is a workout performed on a date — the instance, distinct from the
// Workout template. WorkoutName is the template's name (null for a free-form session
// or one whose template was deleted). Movements is embedded on read (empty in list
// responses, populated on detail). PerformedOn is a calendar date on the wire
// ("2006-01-02"), stored in a Postgres DATE column.
//
// FinishedAt is null while the session is being logged. It is the only thing that
// separates training happening right now from history, and it is what the app offers
// to resume.
type WorkoutSession struct {
	CreatedAt       time.Time         `json:"created_at"`
	FinishedAt      *time.Time        `json:"finished_at"`
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
// prescribed movements are copied in as the session's targets (start-from-template,
// the primary flow); otherwise Movements builds a free-form session. PerformedOn
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
// template's sessions. Unfinished scopes to sessions still in progress, which is how
// the app finds the one to offer resuming.
type WorkoutSessionFilter struct {
	WorkoutID  *int64
	From       string
	To         string
	Unfinished bool
}
