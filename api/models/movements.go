// Package models holds the domain structs and their request/response shapes. The
// DB owns computed and server-stamped values (id, created_at, updated_at); the
// client never sets them. Detail responses embed relations (e.g. a movement's
// muscles) so a single GET renders the whole view.
package models

import "time"

// MovementMuscle is one primary/secondary muscle tag on a movement, joined with
// the muscle's region for display and region filtering.
type MovementMuscle struct {
	Muscle string `json:"muscle"`
	Region string `json:"region"`
	Role   string `json:"role"` // primary | secondary
}

// MovementMuscleInput is the write shape for a muscle tag — region is derived
// from the muscles lookup on the server, never supplied by the client.
type MovementMuscleInput struct {
	Muscle string `json:"muscle"`
	Role   string `json:"role"`
}

// RelatedMovement is a compact summary of a movement reached through a
// relationship (an alternate, antagonist, ...), embedded on a movement's detail
// read. It carries enough to render the swap/see-also lists and to link through.
type RelatedMovement struct {
	Name             string `json:"name"`
	MovementKind     string `json:"movement_kind"`
	RelationshipKind string `json:"relationship_kind"`
	ID               int64  `json:"id"`
	Favorite         bool   `json:"favorite"`
}

// RelationshipInput is the write shape for POST /movements/{id}/related — the
// target movement and the kind of relationship. relationship_kind is FK-validated
// against the lookup server-side; an unknown kind is a 409.
type RelationshipInput struct {
	RelationshipKind  string `json:"relationship_kind"`
	RelatedMovementID int64  `json:"related_movement_id"`
}

// Movement is the full DB-owned record: an exercise, stretch, or yoga pose,
// unified under movement_kind. Kind-specific fields (default_reps, sanskrit_name,
// ...) are nullable and populated as appropriate. Muscles is embedded on read.
type Movement struct {
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	Rating             *int              `json:"rating"`
	DefaultSets        *int              `json:"default_sets"`
	DefaultReps        *string           `json:"default_reps"`
	DefaultHoldSeconds *int              `json:"default_hold_seconds"`
	SanskritName       *string           `json:"sanskrit_name"`
	SourceURL          *string           `json:"source_url"`
	SourceName         *string           `json:"source_name"`
	Name               string            `json:"name"`
	MovementKind       string            `json:"movement_kind"`
	HowTo              string            `json:"how_to"`
	FormCues           string            `json:"form_cues"`
	CommonFaults       string            `json:"common_faults"`
	Tags               []string          `json:"tags"`
	Equipment          []string          `json:"equipment"`
	Muscles            []MovementMuscle  `json:"muscles"`
	Related            []RelatedMovement `json:"related"`
	ID                 int64             `json:"id"`
	Favorite           bool              `json:"favorite"`
	MeasurableROM      bool              `json:"measurable_rom"`
}

// MovementCreate is the create body. Name and MovementKind are required (the
// handler validates); everything else has a sensible zero.
type MovementCreate struct {
	Rating             *int                  `json:"rating"`
	DefaultSets        *int                  `json:"default_sets"`
	DefaultReps        *string               `json:"default_reps"`
	DefaultHoldSeconds *int                  `json:"default_hold_seconds"`
	SanskritName       *string               `json:"sanskrit_name"`
	SourceURL          *string               `json:"source_url"`
	SourceName         *string               `json:"source_name"`
	Name               string                `json:"name"`
	MovementKind       string                `json:"movement_kind"`
	HowTo              string                `json:"how_to"`
	FormCues           string                `json:"form_cues"`
	CommonFaults       string                `json:"common_faults"`
	Tags               []string              `json:"tags"`
	Equipment          []string              `json:"equipment"`
	Muscles            []MovementMuscleInput `json:"muscles"`
	Favorite           bool                  `json:"favorite"`
	MeasurableROM      bool                  `json:"measurable_rom"`
}

// MovementUpdate is a partial update: a nil pointer means "leave unchanged".
// Slice/muscle fields use pointers so an explicit empty array clears the set
// while omitting the field leaves it as-is.
type MovementUpdate struct {
	Name               *string                `json:"name"`
	MovementKind       *string                `json:"movement_kind"`
	Favorite           *bool                  `json:"favorite"`
	Rating             *int                   `json:"rating"`
	Tags               *[]string              `json:"tags"`
	Equipment          *[]string              `json:"equipment"`
	HowTo              *string                `json:"how_to"`
	FormCues           *string                `json:"form_cues"`
	CommonFaults       *string                `json:"common_faults"`
	DefaultSets        *int                   `json:"default_sets"`
	DefaultReps        *string                `json:"default_reps"`
	DefaultHoldSeconds *int                   `json:"default_hold_seconds"`
	SanskritName       *string                `json:"sanskrit_name"`
	MeasurableROM      *bool                  `json:"measurable_rom"`
	SourceURL          *string                `json:"source_url"`
	SourceName         *string                `json:"source_name"`
	Muscles            *[]MovementMuscleInput `json:"muscles"`
}

// MovementFilter carries the optional list-endpoint query params. A zero value
// means "no filter"; the repository builds the WHERE clause from whichever fields
// are set. Favorite is a pointer so ?favorite=false is distinguishable from unset.
type MovementFilter struct {
	Kind      string
	Tag       string
	Equipment string
	Muscle    string
	Region    string
	Search    string
	Favorite  *bool
}

// Muscle is a row of the muscle lookup — name plus the region it belongs to.
type Muscle struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}
