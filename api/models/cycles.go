package models

import "time"

// CycleWorkout is one workout in a cycle's ordered sequence, carrying the
// periodization prescription: which week/phase it belongs to, how often, at what
// effort, and the readiness condition that gates advancing to the next block. It
// embeds the workout's name and theme (denormalized on read) so a cycle renders in
// a single GET, mirroring how a WorkoutMovement embeds its movement. Every
// prescription field is nullable — a mobility block may set none of them.
type CycleWorkout struct {
	Week         *int    `json:"week"`
	Phase        *string `json:"phase"`
	Frequency    *string `json:"frequency"`
	Intensity    *string `json:"intensity"`
	Conditions   *string `json:"conditions"`
	WorkoutTheme *string `json:"workout_theme"`
	WorkoutName  string  `json:"workout_name"`
	ID           int64   `json:"id"`
	WorkoutID    int64   `json:"workout_id"`
	Position     int     `json:"position"`
}

// CycleWorkoutInput is the write shape for one entry — used both when creating a
// cycle with its full sequence in one call (seed/import) and when adding a single
// workout to an existing cycle. Position is implied by array order on create and
// assigned server-side (append) on a single add.
type CycleWorkoutInput struct {
	Week       *int    `json:"week"`
	Phase      *string `json:"phase"`
	Frequency  *string `json:"frequency"`
	Intensity  *string `json:"intensity"`
	Conditions *string `json:"conditions"`
	WorkoutID  int64   `json:"workout_id"`
}

// CycleWorkoutUpdate is a partial update of one entry: a nil field is left
// unchanged. WorkoutID re-points the entry at a different workout — the swap —
// carrying the existing prescription (week/phase/frequency/...) to the substitute.
type CycleWorkoutUpdate struct {
	WorkoutID  *int64  `json:"workout_id"`
	Week       *int    `json:"week"`
	Phase      *string `json:"phase"`
	Frequency  *string `json:"frequency"`
	Intensity  *string `json:"intensity"`
	Conditions *string `json:"conditions"`
}

// CycleWorkoutOrder is the reorder body: the entry ids in their desired order. The
// repository rewrites positions to 1..n from this list.
type CycleWorkoutOrder struct {
	EntryIDs []int64 `json:"entry_ids"`
}

// Cycle is the full DB-owned record: a mesocycle — an ordered sequence of workouts
// toward a goal. TargetMetric optionally names a metric_definition so the goal is a
// real tracked number; TargetValue is the number to reach. Both dates are nullable
// so a planned cycle can be drafted before it is scheduled. Workouts is embedded on
// read (empty in list responses, populated on detail), mirroring a workout's
// movements. Dates are calendar dates on the wire ("2006-01-02").
type Cycle struct {
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	TargetMetric *string        `json:"target_metric"`
	TargetValue  *float64       `json:"target_value"`
	TargetDate   *string        `json:"target_date"`
	StartDate    *string        `json:"start_date"`
	Name         string         `json:"name"`
	GoalSummary  string         `json:"goal_summary"`
	Status       string         `json:"status"`
	Notes        string         `json:"notes"`
	Workouts     []CycleWorkout `json:"workouts"`
	ID           int64          `json:"id"`
}

// CycleCreate is the create body. Name is required (the handler validates); Status
// defaults to "planned" server-side when empty. Workouts may be supplied to build a
// complete cycle in one call (the seed/import path) or omitted and added later via
// the sub-resource.
type CycleCreate struct {
	TargetMetric *string             `json:"target_metric"`
	TargetValue  *float64            `json:"target_value"`
	TargetDate   *string             `json:"target_date"`
	StartDate    *string             `json:"start_date"`
	Name         string              `json:"name"`
	GoalSummary  string              `json:"goal_summary"`
	Status       string              `json:"status"`
	Notes        string              `json:"notes"`
	Workouts     []CycleWorkoutInput `json:"workouts"`
}

// CycleUpdate is a partial update of cycle-level fields; the workout sequence is
// managed through the sub-resource endpoints, not here. A nil pointer means "leave
// unchanged". For a nullable field (dates, target metric/value), an explicit empty
// value clears it while an absent field leaves it untouched.
type CycleUpdate struct {
	Name         *string  `json:"name"`
	GoalSummary  *string  `json:"goal_summary"`
	TargetMetric *string  `json:"target_metric"`
	TargetValue  *float64 `json:"target_value"`
	TargetDate   *string  `json:"target_date"`
	StartDate    *string  `json:"start_date"`
	Status       *string  `json:"status"`
	Notes        *string  `json:"notes"`
}

// CycleFilter carries the optional list-endpoint query params. Status scopes to one
// lifecycle state (planned|active|paused|completed); Search matches name/goal.
type CycleFilter struct {
	Status string
	Search string
}
