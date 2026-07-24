package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// WorkoutMovement mirrors one prescribed entry in the API's workout JSON, with the
// movement's name/kind embedded for render.
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

// Workout mirrors the API's workout JSON. Movements is embedded on read.
type Workout struct {
	Theme            *string           `json:"theme"`
	EstimatedMinutes *int              `json:"estimated_minutes"`
	Name             string            `json:"name"`
	Notes            string            `json:"notes"`
	Tags             []string          `json:"tags"`
	Movements        []WorkoutMovement `json:"movements"`
	ID               int64             `json:"id"`
	Favorite         bool              `json:"favorite"`
}

// WorkoutMovementInput is the write shape for one entry (create or single add).
type WorkoutMovementInput struct {
	Sets          *int    `json:"sets,omitempty"`
	Reps          *string `json:"reps,omitempty"`
	Load          *string `json:"load,omitempty"`
	RestSeconds   *int    `json:"rest_seconds,omitempty"`
	SupersetGroup *string `json:"superset_group,omitempty"`
	Notes         string  `json:"notes,omitempty"`
	MovementID    int64   `json:"movement_id"`
}

// WorkoutCreate is the create body sent to POST /api/v1/workouts.
type WorkoutCreate struct {
	Theme            *string                `json:"theme,omitempty"`
	EstimatedMinutes *int                   `json:"estimated_minutes,omitempty"`
	Name             string                 `json:"name"`
	Notes            string                 `json:"notes,omitempty"`
	Tags             []string               `json:"tags"`
	Movements        []WorkoutMovementInput `json:"movements,omitempty"`
	Favorite         bool                   `json:"favorite"`
}

// WorkoutFilter carries the optional list-endpoint query params.
type WorkoutFilter struct {
	Theme    string
	Tag      string
	Search   string
	Favorite *bool
}

func (f WorkoutFilter) query() string {
	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("theme", f.Theme)
	set("tag", f.Tag)
	set("search", f.Search)
	if f.Favorite != nil {
		q.Set("favorite", strconv.FormatBool(*f.Favorite))
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// ListWorkouts returns workouts matching the filter (GET /api/v1/workouts).
func (c *Client) ListWorkouts(ctx context.Context, f WorkoutFilter) ([]Workout, error) {
	var workouts []Workout
	if err := c.get(ctx, "/api/v1/workouts"+f.query(), &workouts); err != nil {
		return nil, err
	}
	return workouts, nil
}

// GetWorkout returns a single workout (GET /api/v1/workouts/{id}).
func (c *Client) GetWorkout(ctx context.Context, id int64) (Workout, error) {
	var w Workout
	if err := c.get(ctx, "/api/v1/workouts/"+strconv.FormatInt(id, 10), &w); err != nil {
		return Workout{}, err
	}
	return w, nil
}

// CreateWorkout creates a workout (POST /api/v1/workouts).
func (c *Client) CreateWorkout(ctx context.Context, in WorkoutCreate) (Workout, error) {
	var w Workout
	if err := c.send(ctx, http.MethodPost, "/api/v1/workouts", in, &w); err != nil {
		return Workout{}, err
	}
	return w, nil
}

// UpdateWorkout applies a partial update to workout-level fields (PUT
// /api/v1/workouts/{id}); only the keys in patch are sent.
func (c *Client) UpdateWorkout(ctx context.Context, id int64, patch map[string]any) (Workout, error) {
	var w Workout
	if err := c.send(ctx, http.MethodPut, "/api/v1/workouts/"+strconv.FormatInt(id, 10), patch, &w); err != nil {
		return Workout{}, err
	}
	return w, nil
}

// DeleteWorkout removes a workout (DELETE /api/v1/workouts/{id}).
func (c *Client) DeleteWorkout(ctx context.Context, id int64) error {
	return c.send(ctx, http.MethodDelete, "/api/v1/workouts/"+strconv.FormatInt(id, 10), nil, nil)
}

// AddWorkoutMovement appends a movement to a workout (POST
// /api/v1/workouts/{id}/movements) and returns the refreshed workout.
func (c *Client) AddWorkoutMovement(ctx context.Context, workoutID int64, in WorkoutMovementInput) (Workout, error) {
	var w Workout
	path := "/api/v1/workouts/" + strconv.FormatInt(workoutID, 10) + "/movements"
	if err := c.send(ctx, http.MethodPost, path, in, &w); err != nil {
		return Workout{}, err
	}
	return w, nil
}

// UpdateWorkoutMovement edits or swaps one entry (PATCH
// /api/v1/workouts/{id}/movements/{entryID}); only the keys in patch are sent.
func (c *Client) UpdateWorkoutMovement(ctx context.Context, workoutID, entryID int64, patch map[string]any) (Workout, error) {
	var w Workout
	path := "/api/v1/workouts/" + strconv.FormatInt(workoutID, 10) + "/movements/" + strconv.FormatInt(entryID, 10)
	if err := c.send(ctx, http.MethodPatch, path, patch, &w); err != nil {
		return Workout{}, err
	}
	return w, nil
}

// ReorderWorkoutMovements sets the entry order (PATCH /api/v1/workouts/{id}/movements).
func (c *Client) ReorderWorkoutMovements(ctx context.Context, workoutID int64, entryIDs []int64) (Workout, error) {
	var w Workout
	path := "/api/v1/workouts/" + strconv.FormatInt(workoutID, 10) + "/movements"
	body := map[string]any{"entry_ids": entryIDs}
	if err := c.send(ctx, http.MethodPatch, path, body, &w); err != nil {
		return Workout{}, err
	}
	return w, nil
}

// RemoveWorkoutMovement drops one entry (DELETE /api/v1/workouts/{id}/movements/{entryID})
// and returns the refreshed workout.
func (c *Client) RemoveWorkoutMovement(ctx context.Context, workoutID, entryID int64) (Workout, error) {
	var w Workout
	path := "/api/v1/workouts/" + strconv.FormatInt(workoutID, 10) + "/movements/" + strconv.FormatInt(entryID, 10)
	if err := c.send(ctx, http.MethodDelete, path, nil, &w); err != nil {
		return Workout{}, err
	}
	return w, nil
}

// ParseWorkoutID parses a workout id argument, giving a clean error for non-numeric
// input (workouts use an integer identity PK).
func ParseWorkoutID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid workout id %q: want a number", raw)
	}
	return id, nil
}
