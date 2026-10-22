package api

import (
	"context"
	"net/url"
)

// ReviewSchemaVersion is the newest review payload shape this client reads.
const ReviewSchemaVersion = 1

// Review mirrors the API's capstone read: active cycles plus the recent sessions,
// measurements, and log entries in the window. `meso review --json` prints this
// verbatim for Claude to reason over. Since and Until are the first and last days
// covered, empty where the window is open.
type Review struct {
	SchemaVersion int           `json:"schema_version"`
	Since         string        `json:"since"`
	Until         string        `json:"until"`
	ActiveCycles  []Cycle       `json:"active_cycles"`
	Sessions      []Session     `json:"sessions"`
	Measurements  []Measurement `json:"measurements"`
	LogEntries    []LogEntry    `json:"log_entries"`
}

// GetReview returns the review payload for the window (GET /api/v1/review). An open
// window is every row.
func (c *Client) GetReview(ctx context.Context, w Window) (Review, error) {
	path := "/api/v1/review"
	q := url.Values{}
	w.set(q)
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var review Review
	if err := c.get(ctx, path, &review); err != nil {
		return Review{}, err
	}
	if err := checkSchemaVersion("review", review.SchemaVersion, ReviewSchemaVersion); err != nil {
		return Review{}, err
	}
	return review, nil
}
