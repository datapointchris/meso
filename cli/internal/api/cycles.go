package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// CycleWorkout mirrors one entry in a cycle's ordered sequence, with the workout's
// name/theme embedded for render. The periodization fields are nullable.
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

// Cycle mirrors the API's cycle JSON. Workouts is embedded on read.
type Cycle struct {
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

// CycleWorkoutInput is the write shape for one entry (create or single add).
type CycleWorkoutInput struct {
	Week       *int    `json:"week,omitempty"`
	Phase      *string `json:"phase,omitempty"`
	Frequency  *string `json:"frequency,omitempty"`
	Intensity  *string `json:"intensity,omitempty"`
	Conditions *string `json:"conditions,omitempty"`
	WorkoutID  int64   `json:"workout_id"`
}

// CycleCreate is the create body sent to POST /api/v1/cycles.
type CycleCreate struct {
	TargetMetric *string             `json:"target_metric,omitempty"`
	TargetValue  *float64            `json:"target_value,omitempty"`
	TargetDate   *string             `json:"target_date,omitempty"`
	StartDate    *string             `json:"start_date,omitempty"`
	Name         string              `json:"name"`
	GoalSummary  string              `json:"goal_summary,omitempty"`
	Status       string              `json:"status,omitempty"`
	Notes        string              `json:"notes,omitempty"`
	Workouts     []CycleWorkoutInput `json:"workouts,omitempty"`
}

// CycleFilter carries the optional list-endpoint query params.
type CycleFilter struct {
	Status string
	Search string
}

func (f CycleFilter) query() string {
	q := url.Values{}
	if f.Status != "" {
		q.Set("status", f.Status)
	}
	if f.Search != "" {
		q.Set("search", f.Search)
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// ListCycles returns cycles matching the filter (GET /api/v1/cycles).
func (c *Client) ListCycles(ctx context.Context, f CycleFilter) ([]Cycle, error) {
	var cycles []Cycle
	if err := c.get(ctx, "/api/v1/cycles"+f.query(), &cycles); err != nil {
		return nil, err
	}
	return cycles, nil
}

// GetCycle returns a single cycle (GET /api/v1/cycles/{id}).
func (c *Client) GetCycle(ctx context.Context, id int64) (Cycle, error) {
	var cy Cycle
	if err := c.get(ctx, "/api/v1/cycles/"+strconv.FormatInt(id, 10), &cy); err != nil {
		return Cycle{}, err
	}
	return cy, nil
}

// CreateCycle creates a cycle (POST /api/v1/cycles).
func (c *Client) CreateCycle(ctx context.Context, in CycleCreate) (Cycle, error) {
	var cy Cycle
	if err := c.send(ctx, http.MethodPost, "/api/v1/cycles", in, &cy); err != nil {
		return Cycle{}, err
	}
	return cy, nil
}

// UpdateCycle applies a partial update to cycle-level fields (PUT /api/v1/cycles/{id});
// only the keys in patch are sent.
func (c *Client) UpdateCycle(ctx context.Context, id int64, patch map[string]any) (Cycle, error) {
	var cy Cycle
	if err := c.send(ctx, http.MethodPut, "/api/v1/cycles/"+strconv.FormatInt(id, 10), patch, &cy); err != nil {
		return Cycle{}, err
	}
	return cy, nil
}

// DeleteCycle removes a cycle (DELETE /api/v1/cycles/{id}).
func (c *Client) DeleteCycle(ctx context.Context, id int64) error {
	return c.send(ctx, http.MethodDelete, "/api/v1/cycles/"+strconv.FormatInt(id, 10), nil, nil)
}

// AddCycleWorkout appends a workout to a cycle (POST /api/v1/cycles/{id}/workouts)
// and returns the refreshed cycle.
func (c *Client) AddCycleWorkout(ctx context.Context, cycleID int64, in CycleWorkoutInput) (Cycle, error) {
	var cy Cycle
	path := "/api/v1/cycles/" + strconv.FormatInt(cycleID, 10) + "/workouts"
	if err := c.send(ctx, http.MethodPost, path, in, &cy); err != nil {
		return Cycle{}, err
	}
	return cy, nil
}

// UpdateCycleWorkout edits or swaps one entry (PATCH /api/v1/cycles/{id}/workouts/{entryID});
// only the keys in patch are sent.
func (c *Client) UpdateCycleWorkout(ctx context.Context, cycleID, entryID int64, patch map[string]any) (Cycle, error) {
	var cy Cycle
	path := "/api/v1/cycles/" + strconv.FormatInt(cycleID, 10) + "/workouts/" + strconv.FormatInt(entryID, 10)
	if err := c.send(ctx, http.MethodPatch, path, patch, &cy); err != nil {
		return Cycle{}, err
	}
	return cy, nil
}

// ReorderCycleWorkouts sets the entry order (PATCH /api/v1/cycles/{id}/workouts).
func (c *Client) ReorderCycleWorkouts(ctx context.Context, cycleID int64, entryIDs []int64) (Cycle, error) {
	var cy Cycle
	path := "/api/v1/cycles/" + strconv.FormatInt(cycleID, 10) + "/workouts"
	body := map[string]any{"entry_ids": entryIDs}
	if err := c.send(ctx, http.MethodPatch, path, body, &cy); err != nil {
		return Cycle{}, err
	}
	return cy, nil
}

// RemoveCycleWorkout drops one entry (DELETE /api/v1/cycles/{id}/workouts/{entryID})
// and returns the refreshed cycle.
func (c *Client) RemoveCycleWorkout(ctx context.Context, cycleID, entryID int64) (Cycle, error) {
	var cy Cycle
	path := "/api/v1/cycles/" + strconv.FormatInt(cycleID, 10) + "/workouts/" + strconv.FormatInt(entryID, 10)
	if err := c.send(ctx, http.MethodDelete, path, nil, &cy); err != nil {
		return Cycle{}, err
	}
	return cy, nil
}

// ParseCycleID parses a cycle id argument, giving a clean error for non-numeric
// input (cycles use an integer identity PK).
func ParseCycleID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cycle id %q: want a number", raw)
	}
	return id, nil
}
