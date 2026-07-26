package api

import (
	"context"
	"net/http"
	"net/url"
)

// Feedback mirrors the API's feedback JSON — one captured papercut or idea about the
// app itself. The id is a UUID7 string, treated opaquely by the CLI. Status is
// open|done; ContextPath is the in-app route it was raised from and the viewport is
// how wide the window was, both nil/empty for feedback captured from the CLI.
type Feedback struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Body           string `json:"body"`
	ContextPath    string `json:"context_path"`
	ViewportWidth  *int   `json:"viewport_width"`
	ViewportHeight *int   `json:"viewport_height"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// FeedbackCreate is the body for POST /api/v1/feedback. Status defaults to open
// server-side. The viewport is omitted — a terminal has no window to measure.
type FeedbackCreate struct {
	Body        string `json:"body"`
	ContextPath string `json:"context_path,omitempty"`
}

// FeedbackFilter carries the optional list-endpoint query params.
type FeedbackFilter struct {
	Status string
	Search string
}

func (f FeedbackFilter) query() string {
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

// ListFeedback returns feedback matching the filter, newest first
// (GET /api/v1/feedback).
func (c *Client) ListFeedback(ctx context.Context, f FeedbackFilter) ([]Feedback, error) {
	var items []Feedback
	if err := c.get(ctx, "/api/v1/feedback"+f.query(), &items); err != nil {
		return nil, err
	}
	return items, nil
}

// GetFeedback returns one item (GET /api/v1/feedback/{id}).
func (c *Client) GetFeedback(ctx context.Context, id string) (Feedback, error) {
	var f Feedback
	if err := c.get(ctx, "/api/v1/feedback/"+url.PathEscape(id), &f); err != nil {
		return Feedback{}, err
	}
	return f, nil
}

// CreateFeedback captures feedback (POST /api/v1/feedback) — the same write the
// in-app button makes.
func (c *Client) CreateFeedback(ctx context.Context, in FeedbackCreate) (Feedback, error) {
	var f Feedback
	if err := c.send(ctx, http.MethodPost, "/api/v1/feedback", in, &f); err != nil {
		return Feedback{}, err
	}
	return f, nil
}

// UpdateFeedback applies a partial update (PUT /api/v1/feedback/{id}); only the keys
// in patch are sent, so an untouched field is left unchanged server-side.
func (c *Client) UpdateFeedback(ctx context.Context, id string, patch map[string]any) (Feedback, error) {
	var f Feedback
	if err := c.send(ctx, http.MethodPut, "/api/v1/feedback/"+url.PathEscape(id), patch, &f); err != nil {
		return Feedback{}, err
	}
	return f, nil
}

// DeleteFeedback removes an item (DELETE /api/v1/feedback/{id}).
func (c *Client) DeleteFeedback(ctx context.Context, id string) error {
	return c.send(ctx, http.MethodDelete, "/api/v1/feedback/"+url.PathEscape(id), nil, nil)
}
