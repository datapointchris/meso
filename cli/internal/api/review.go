package api

import (
	"context"
	"net/url"
)

// Review mirrors the API's capstone read: active cycles plus the recent sessions,
// measurements, and log entries in the window. `meso review --json` prints this
// verbatim for Claude to reason over.
type Review struct {
	Since        string        `json:"since"`
	ActiveCycles []Cycle       `json:"active_cycles"`
	Sessions     []Session     `json:"sessions"`
	Measurements []Measurement `json:"measurements"`
	LogEntries   []LogEntry    `json:"log_entries"`
}

// GetReview returns the review payload for the given window (GET /api/v1/review).
// since is a relative duration like "30d"; empty uses the server's default window.
func (c *Client) GetReview(ctx context.Context, since string) (Review, error) {
	path := "/api/v1/review"
	if since != "" {
		path += "?" + url.Values{"since": {since}}.Encode()
	}
	var review Review
	if err := c.get(ctx, path, &review); err != nil {
		return Review{}, err
	}
	return review, nil
}
