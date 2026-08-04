package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// SessionMovement mirrors one logged entry in the API's session JSON — the done
// checkbox and the real (actual) numbers, with the movement's name/kind embedded.
type SessionMovement struct {
	ActualSets   *int             `json:"actual_sets"`
	ActualReps   *string          `json:"actual_reps"`
	ActualLoad   *string          `json:"actual_load"`
	Previous     *PreviousActuals `json:"previous"`
	MovementName string           `json:"movement_name"`
	MovementKind string           `json:"movement_kind"`
	Notes        string           `json:"notes"`
	ID           int64            `json:"id"`
	MovementID   int64            `json:"movement_id"`
	Position     int              `json:"position"`
	Done         bool             `json:"done"`
}

// PreviousActuals is the last performed result for a movement before this session —
// the number to beat. Null when never performed, and only populated on session detail
// (`sessions show`); the list endpoint leaves it out.
type PreviousActuals struct {
	ActualSets  *int    `json:"actual_sets"`
	ActualReps  *string `json:"actual_reps"`
	ActualLoad  *string `json:"actual_load"`
	PerformedOn string  `json:"performed_on"`
}

// Session mirrors the API's workout-session JSON. The id is a UUID7 string; the CLI
// treats it opaquely (passes it through URLs, prints it) and never needs the uuid
// package. WorkoutName is the template's name (null for an ad-hoc session).
type Session struct {
	WorkoutID       *int64            `json:"workout_id"`
	WorkoutName     *string           `json:"workout_name"`
	DurationMinutes *int              `json:"duration_minutes"`
	Felt            *string           `json:"felt"`
	ID              string            `json:"id"`
	PerformedOn     string            `json:"performed_on"`
	OverallNotes    string            `json:"overall_notes"`
	Movements       []SessionMovement `json:"movements"`
}

// SessionCreate is the create body sent to POST /api/v1/sessions. A WorkoutID copies
// that template's movements in (start-from-template); omitting it starts an ad-hoc
// session. PerformedOn defaults to today server-side when empty.
type SessionCreate struct {
	WorkoutID       *int64  `json:"workout_id,omitempty"`
	DurationMinutes *int    `json:"duration_minutes,omitempty"`
	Felt            *string `json:"felt,omitempty"`
	PerformedOn     string  `json:"performed_on,omitempty"`
	OverallNotes    string  `json:"overall_notes,omitempty"`
}

// SessionMovementInput appends one movement to a session already underway (POST
// /api/v1/sessions/{id}/movements) — the ad-hoc path, where the session starts empty
// and is filled in as the workout happens.
type SessionMovementInput struct {
	ActualSets *int    `json:"actual_sets,omitempty"`
	ActualReps *string `json:"actual_reps,omitempty"`
	ActualLoad *string `json:"actual_load,omitempty"`
	Notes      string  `json:"notes,omitempty"`
	MovementID int64   `json:"movement_id"`
	Done       bool    `json:"done,omitempty"`
}

// SessionPromote turns a performed ad-hoc session into a workout template (POST
// /api/v1/sessions/{id}/workout). The logged actuals become the prescription, so only
// the name and its labels are sent.
type SessionPromote struct {
	Theme *string  `json:"theme,omitempty"`
	Name  string   `json:"name"`
	Notes string   `json:"notes,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

// SessionFilter carries the optional list-endpoint query params.
type SessionFilter struct {
	WorkoutID *int64
	From      string
	To        string
}

func (f SessionFilter) query() string {
	q := url.Values{}
	if f.From != "" {
		q.Set("from", f.From)
	}
	if f.To != "" {
		q.Set("to", f.To)
	}
	if f.WorkoutID != nil {
		q.Set("workout_id", strconv.FormatInt(*f.WorkoutID, 10))
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// ListSessions returns sessions matching the filter (GET /api/v1/sessions).
func (c *Client) ListSessions(ctx context.Context, f SessionFilter) ([]Session, error) {
	var sessions []Session
	if err := c.get(ctx, "/api/v1/sessions"+f.query(), &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// GetSession returns a single session (GET /api/v1/sessions/{id}).
func (c *Client) GetSession(ctx context.Context, id string) (Session, error) {
	var s Session
	if err := c.get(ctx, "/api/v1/sessions/"+url.PathEscape(id), &s); err != nil {
		return Session{}, err
	}
	return s, nil
}

// CreateSession starts a session (POST /api/v1/sessions).
func (c *Client) CreateSession(ctx context.Context, in SessionCreate) (Session, error) {
	var s Session
	if err := c.send(ctx, http.MethodPost, "/api/v1/sessions", in, &s); err != nil {
		return Session{}, err
	}
	return s, nil
}

// UpdateSession applies a partial update to session-level fields (PUT
// /api/v1/sessions/{id}); only the keys in patch are sent.
func (c *Client) UpdateSession(ctx context.Context, id string, patch map[string]any) (Session, error) {
	var s Session
	if err := c.send(ctx, http.MethodPut, "/api/v1/sessions/"+url.PathEscape(id), patch, &s); err != nil {
		return Session{}, err
	}
	return s, nil
}

// DeleteSession removes a session (DELETE /api/v1/sessions/{id}).
func (c *Client) DeleteSession(ctx context.Context, id string) error {
	return c.send(ctx, http.MethodDelete, "/api/v1/sessions/"+url.PathEscape(id), nil, nil)
}

// AddSessionMovement appends a movement to a session (POST
// /api/v1/sessions/{id}/movements), returning the refreshed session.
func (c *Client) AddSessionMovement(ctx context.Context, sessionID string, in SessionMovementInput) (Session, error) {
	var s Session
	path := "/api/v1/sessions/" + url.PathEscape(sessionID) + "/movements"
	if err := c.send(ctx, http.MethodPost, path, in, &s); err != nil {
		return Session{}, err
	}
	return s, nil
}

// RemoveSessionMovement drops one logged entry (DELETE
// /api/v1/sessions/{id}/movements/{entryID}), returning the refreshed session.
func (c *Client) RemoveSessionMovement(ctx context.Context, sessionID string, entryID int64) (Session, error) {
	var s Session
	path := "/api/v1/sessions/" + url.PathEscape(sessionID) + "/movements/" + strconv.FormatInt(entryID, 10)
	if err := c.send(ctx, http.MethodDelete, path, nil, &s); err != nil {
		return Session{}, err
	}
	return s, nil
}

// PromoteSession turns an ad-hoc session into a workout template (POST
// /api/v1/sessions/{id}/workout), returning the created workout.
func (c *Client) PromoteSession(ctx context.Context, sessionID string, in SessionPromote) (Workout, error) {
	var w Workout
	path := "/api/v1/sessions/" + url.PathEscape(sessionID) + "/workout"
	if err := c.send(ctx, http.MethodPost, path, in, &w); err != nil {
		return Workout{}, err
	}
	return w, nil
}

// UpdateSessionMovement checks off / records actuals / swaps one entry (PATCH
// /api/v1/sessions/{id}/movements/{entryID}); only the keys in patch are sent.
func (c *Client) UpdateSessionMovement(ctx context.Context, sessionID string, entryID int64, patch map[string]any) (Session, error) {
	var s Session
	path := "/api/v1/sessions/" + url.PathEscape(sessionID) + "/movements/" + strconv.FormatInt(entryID, 10)
	if err := c.send(ctx, http.MethodPatch, path, patch, &s); err != nil {
		return Session{}, err
	}
	return s, nil
}
