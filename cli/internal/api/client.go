// Package api is the meso CLI's typed client for the Go HTTP API. It depends on
// the wire contract (the JSON shapes the API returns), not the server's internal
// models package, so the two can evolve independently as long as the JSON is
// stable. Every request carries the edge-authorized bearer token supplied by the
// injected *http.Client (built from auth.TokenSource).
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client talks to the meso API at baseURL. The http.Client is expected to inject
// the Authorization: Bearer header itself (via an oauth2 token-source transport),
// so the api package never handles tokens directly.
type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a client for baseURL. Trailing slashes are trimmed so path joining
// is unambiguous.
func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    httpClient,
	}
}

// APIError is a non-2xx response, carrying the HTTP status and the API's decoded
// {"error","message"} body so callers can branch on StatusCode (401 → re-login,
// 404 → not found) and surface the server's message.
type APIError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("API request failed (%s): %s", e.Status, e.Message)
	}
	return fmt.Sprintf("API request failed (%s)", e.Status)
}

// Unauthorized reports whether the error is a 401 — the CLI translates this into
// a "run `meso auth login`" hint rather than a raw API error.
func (e *APIError) Unauthorized() bool { return e.StatusCode == http.StatusUnauthorized }

// NotFound reports whether the error is a 404 — used to give resource commands a
// clean "no such movement" message.
func (e *APIError) NotFound() bool { return e.StatusCode == http.StatusNotFound }

// get issues a GET to path and decodes a 2xx JSON body into out.
func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.send(ctx, http.MethodGet, path, nil, out)
}

// send issues an HTTP request to path with an optional JSON body, and decodes a
// 2xx JSON body into out. A nil body sends no payload; a nil out discards the
// response body (for 204 No Content). A non-2xx response becomes an *APIError.
func (c *Client) send(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("reach meso API at %s: %w", c.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode API response: %w", err)
	}
	return nil
}

// decodeAPIError builds an *APIError from a non-2xx response, best-effort decoding
// the {"error","message"} envelope. A body that is not that shape (a proxy error
// page, an Authelia redirect) still yields a usable status-only error.
func decodeAPIError(resp *http.Response) error {
	apiErr := &APIError{StatusCode: resp.StatusCode, Status: resp.Status}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))

	var envelope struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Message != "" {
		apiErr.Message = envelope.Message
	}
	return apiErr
}
