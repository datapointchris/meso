package models

import (
	"time"

	"github.com/google/uuid"
)

// Feedback is one captured papercut or idea about the app itself, raised from inside
// it. Unlike every other entity here it is not training data — it is a fact about the
// software, which is why it lives behind the CLI's `admin` namespace rather than
// alongside movements and sessions.
//
// There is no kind/category: the body says what it is, and a bug and an idea are read
// and acted on identically. ContextPath is the in-app route it was raised from, the
// one piece of triage context that is free to capture now and unreconstructable later.
type Feedback struct {
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Status      string    `json:"status"`
	Body        string    `json:"body"`
	ContextPath string    `json:"context_path"`
	ID          uuid.UUID `json:"id"`
}

// FeedbackCreate captures feedback. Body is required; Status defaults to "open"
// server-side. ContextPath is filled by the web app from the current route.
type FeedbackCreate struct {
	Body        string `json:"body"`
	ContextPath string `json:"context_path"`
}

// FeedbackUpdate is a partial update: a nil field is left unchanged. Status ∈
// open|done — the DB CHECK enforces it, so an unknown value is a 400.
type FeedbackUpdate struct {
	Body        *string `json:"body"`
	ContextPath *string `json:"context_path"`
	Status      *string `json:"status"`
}

// FeedbackFilter carries the list-endpoint query params. Status scopes to open or
// done; Search matches the body case-insensitively.
type FeedbackFilter struct {
	Status string
	Search string
}
