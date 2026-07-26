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
// and acted on identically. ContextPath and the viewport are the triage context that
// is free to capture now and unreconstructable later — the route says what was on
// screen, the viewport says how wide it was, which is what separates a density
// complaint on a phone from a line-length one on a desktop.
//
// The viewport is nil for feedback captured from the CLI, which has no viewport.
type Feedback struct {
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Status         string    `json:"status"`
	Body           string    `json:"body"`
	ContextPath    string    `json:"context_path"`
	ViewportWidth  *int      `json:"viewport_width"`
	ViewportHeight *int      `json:"viewport_height"`
	ID             uuid.UUID `json:"id"`
}

// FeedbackCreate captures feedback. Body is required; Status defaults to "open"
// server-side. ContextPath and the viewport are filled by the web app from the
// current route and window.
type FeedbackCreate struct {
	Body           string `json:"body"`
	ContextPath    string `json:"context_path"`
	ViewportWidth  *int   `json:"viewport_width"`
	ViewportHeight *int   `json:"viewport_height"`
}

// FeedbackUpdate is a partial update: a nil field is left unchanged. Status ∈
// open|done — the DB CHECK enforces it, so an unknown value is a 400. The viewport is
// deliberately absent: it records what was on screen at capture, so editing it later
// would only make it lie.
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
