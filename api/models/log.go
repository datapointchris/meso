package models

import (
	"time"

	"github.com/google/uuid"
)

// FitnessLogEntry is one dated journal entry — a markdown body with free-form tags
// and an optional mood. It is the substrate `meso review` reads when drafting the
// next cycle. EntryDate is a calendar date on the wire ("2006-01-02"), stored in a
// DATE column. Tags is never null (nil normalizes to []). ID is a UUID7 (user-
// generated top-level content, time-ordered).
type FitnessLogEntry struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	EntryDate string    `json:"entry_date"`
	Body      string    `json:"body"`
	Mood      *string   `json:"mood"`
	Tags      []string  `json:"tags"`
	ID        uuid.UUID `json:"id"`
}

// FitnessLogEntryCreate writes a new entry. EntryDate defaults to today when empty.
// Mood is optional (nil = no mood). Tags nil normalizes to an empty array server-side.
type FitnessLogEntryCreate struct {
	EntryDate string   `json:"entry_date"`
	Body      string   `json:"body"`
	Mood      *string  `json:"mood"`
	Tags      []string `json:"tags"`
}

// FitnessLogEntryUpdate is a partial update: a nil pointer field is left unchanged.
// Tags is replaced wholesale when supplied (a non-nil slice, even empty, clears/sets).
type FitnessLogEntryUpdate struct {
	EntryDate *string   `json:"entry_date"`
	Body      *string   `json:"body"`
	Mood      *string   `json:"mood"`
	Tags      *[]string `json:"tags"`
}

// FitnessLogEntryFilter carries the list-endpoint query params. From/To bound
// entry_date (inclusive) as "2006-01-02" strings; Tag scopes to entries carrying it.
type FitnessLogEntryFilter struct {
	From string
	To   string
	Tag  string
}
