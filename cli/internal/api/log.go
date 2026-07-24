package api

import (
	"context"
	"net/http"
	"net/url"
)

// LogEntry mirrors the API's fitness-log-entry JSON — one dated journal entry. The id
// is a UUID7 string; the CLI treats it opaquely (passes it through URLs, prints it).
// Mood is null when unset.
type LogEntry struct {
	Mood      *string  `json:"mood"`
	ID        string   `json:"id"`
	EntryDate string   `json:"entry_date"`
	Body      string   `json:"body"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Tags      []string `json:"tags"`
}

// LogEntryCreate is the create body sent to POST /api/v1/log. EntryDate defaults to
// today server-side when empty; Mood is omitted when nil.
type LogEntryCreate struct {
	Mood      *string  `json:"mood,omitempty"`
	EntryDate string   `json:"entry_date,omitempty"`
	Body      string   `json:"body"`
	Tags      []string `json:"tags,omitempty"`
}

// LogFilter carries the optional list-endpoint query params.
type LogFilter struct {
	From string
	To   string
	Tag  string
}

func (f LogFilter) query() string {
	q := url.Values{}
	if f.From != "" {
		q.Set("from", f.From)
	}
	if f.To != "" {
		q.Set("to", f.To)
	}
	if f.Tag != "" {
		q.Set("tag", f.Tag)
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// ListLog returns entries matching the filter, newest first (GET /api/v1/log).
func (c *Client) ListLog(ctx context.Context, f LogFilter) ([]LogEntry, error) {
	var entries []LogEntry
	if err := c.get(ctx, "/api/v1/log"+f.query(), &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetLogEntry returns a single entry (GET /api/v1/log/{id}).
func (c *Client) GetLogEntry(ctx context.Context, id string) (LogEntry, error) {
	var e LogEntry
	if err := c.get(ctx, "/api/v1/log/"+url.PathEscape(id), &e); err != nil {
		return LogEntry{}, err
	}
	return e, nil
}

// CreateLogEntry appends a journal entry (POST /api/v1/log).
func (c *Client) CreateLogEntry(ctx context.Context, in LogEntryCreate) (LogEntry, error) {
	var e LogEntry
	if err := c.send(ctx, http.MethodPost, "/api/v1/log", in, &e); err != nil {
		return LogEntry{}, err
	}
	return e, nil
}

// UpdateLogEntry applies a partial update (PUT /api/v1/log/{id}); only the keys in
// patch are sent, so an untouched field is left unchanged server-side.
func (c *Client) UpdateLogEntry(ctx context.Context, id string, patch map[string]any) (LogEntry, error) {
	var e LogEntry
	if err := c.send(ctx, http.MethodPut, "/api/v1/log/"+url.PathEscape(id), patch, &e); err != nil {
		return LogEntry{}, err
	}
	return e, nil
}

// DeleteLogEntry removes an entry (DELETE /api/v1/log/{id}).
func (c *Client) DeleteLogEntry(ctx context.Context, id string) error {
	return c.send(ctx, http.MethodDelete, "/api/v1/log/"+url.PathEscape(id), nil, nil)
}
