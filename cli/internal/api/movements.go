package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// MovementMuscle mirrors one muscle tag in the API's movement JSON.
type MovementMuscle struct {
	Muscle string `json:"muscle"`
	Region string `json:"region"`
	Role   string `json:"role"`
}

// Movement mirrors the API's movement JSON. Kind-specific fields are nullable
// pointers; muscles are embedded on read.
type Movement struct {
	Rating             *int             `json:"rating"`
	DefaultSets        *int             `json:"default_sets"`
	DefaultReps        *string          `json:"default_reps"`
	DefaultHoldSeconds *int             `json:"default_hold_seconds"`
	SanskritName       *string          `json:"sanskrit_name"`
	SourceURL          *string          `json:"source_url"`
	SourceName         *string          `json:"source_name"`
	Name               string           `json:"name"`
	MovementKind       string           `json:"movement_kind"`
	HowTo              string           `json:"how_to"`
	FormCues           string           `json:"form_cues"`
	CommonFaults       string           `json:"common_faults"`
	Tags               []string         `json:"tags"`
	Equipment          []string         `json:"equipment"`
	Muscles            []MovementMuscle `json:"muscles"`
	ID                 int64            `json:"id"`
	Favorite           bool             `json:"favorite"`
	MeasurableROM      bool             `json:"measurable_rom"`
}

// Muscle mirrors a muscle-lookup row.
type Muscle struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

// MuscleInput is the write shape for a muscle tag (region is derived server-side).
type MuscleInput struct {
	Muscle string `json:"muscle"`
	Role   string `json:"role"`
}

// MovementCreate is the create body sent to POST /api/v1/movements.
type MovementCreate struct {
	Rating             *int          `json:"rating,omitempty"`
	DefaultSets        *int          `json:"default_sets,omitempty"`
	DefaultReps        *string       `json:"default_reps,omitempty"`
	DefaultHoldSeconds *int          `json:"default_hold_seconds,omitempty"`
	SanskritName       *string       `json:"sanskrit_name,omitempty"`
	SourceURL          *string       `json:"source_url,omitempty"`
	SourceName         *string       `json:"source_name,omitempty"`
	Name               string        `json:"name"`
	MovementKind       string        `json:"movement_kind"`
	HowTo              string        `json:"how_to"`
	FormCues           string        `json:"form_cues"`
	CommonFaults       string        `json:"common_faults"`
	Tags               []string      `json:"tags"`
	Equipment          []string      `json:"equipment"`
	Muscles            []MuscleInput `json:"muscles"`
	Favorite           bool          `json:"favorite"`
	MeasurableROM      bool          `json:"measurable_rom"`
}

// MovementFilter carries the optional list-endpoint query params.
type MovementFilter struct {
	Kind      string
	Tag       string
	Equipment string
	Muscle    string
	Region    string
	Search    string
	Favorite  *bool
}

func (f MovementFilter) query() string {
	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("kind", f.Kind)
	set("tag", f.Tag)
	set("equipment", f.Equipment)
	set("muscle", f.Muscle)
	set("region", f.Region)
	set("search", f.Search)
	if f.Favorite != nil {
		q.Set("favorite", strconv.FormatBool(*f.Favorite))
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// ListMovements returns movements matching the filter (GET /api/v1/movements).
func (c *Client) ListMovements(ctx context.Context, f MovementFilter) ([]Movement, error) {
	var movements []Movement
	if err := c.get(ctx, "/api/v1/movements"+f.query(), &movements); err != nil {
		return nil, err
	}
	return movements, nil
}

// GetMovement returns a single movement (GET /api/v1/movements/{id}).
func (c *Client) GetMovement(ctx context.Context, id int64) (Movement, error) {
	var m Movement
	if err := c.get(ctx, "/api/v1/movements/"+strconv.FormatInt(id, 10), &m); err != nil {
		return Movement{}, err
	}
	return m, nil
}

// CreateMovement creates a movement (POST /api/v1/movements).
func (c *Client) CreateMovement(ctx context.Context, in MovementCreate) (Movement, error) {
	var m Movement
	if err := c.send(ctx, http.MethodPost, "/api/v1/movements", in, &m); err != nil {
		return Movement{}, err
	}
	return m, nil
}

// UpdateMovement applies a partial update (PUT /api/v1/movements/{id}). The body
// is a field->value map so only the flags the user changed are sent; the server
// leaves omitted fields untouched.
func (c *Client) UpdateMovement(ctx context.Context, id int64, patch map[string]any) (Movement, error) {
	var m Movement
	if err := c.send(ctx, http.MethodPut, "/api/v1/movements/"+strconv.FormatInt(id, 10), patch, &m); err != nil {
		return Movement{}, err
	}
	return m, nil
}

// DeleteMovement removes a movement (DELETE /api/v1/movements/{id}).
func (c *Client) DeleteMovement(ctx context.Context, id int64) error {
	return c.send(ctx, http.MethodDelete, "/api/v1/movements/"+strconv.FormatInt(id, 10), nil, nil)
}

// ListMuscles returns the muscle lookup (GET /api/v1/muscles).
func (c *Client) ListMuscles(ctx context.Context) ([]Muscle, error) {
	var muscles []Muscle
	if err := c.get(ctx, "/api/v1/muscles", &muscles); err != nil {
		return nil, err
	}
	return muscles, nil
}

// PrimaryMuscles returns the names of a movement's primary muscles, for compact
// table output.
func (m Movement) PrimaryMuscles() []string {
	var out []string
	for _, mm := range m.Muscles {
		if mm.Role == "primary" {
			out = append(out, mm.Muscle)
		}
	}
	return out
}

// ParseID parses a movement id argument, giving a clean error for non-numeric
// input (movements use an integer identity PK).
func ParseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid movement id %q: want a number", raw)
	}
	return id, nil
}
