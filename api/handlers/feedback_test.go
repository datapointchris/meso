package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"meso/api/auth"
	"meso/api/database"
	"meso/api/handlers"
	"meso/api/middleware"
	"meso/api/models"
)

func decodeFeedback(t *testing.T, rr interface{ Bytes() []byte }) models.Feedback {
	t.Helper()
	var f models.Feedback
	require.NoError(t, json.Unmarshal(rr.Bytes(), &f))
	return f
}

// verifiedAs stands in for the JWKS check, answering every bearer JWT with one identity.
type verifiedAs auth.Identity

func (v verifiedAs) Verify(context.Context, string) (auth.Identity, error) {
	return auth.Identity(v), nil
}

// asCaller is the router behind RequireAuth, the way main() serves it, with each
// request carrying headers the edge or the CLI would set.
func asCaller(pool *database.TestPool, verifier middleware.TokenVerifier, headers map[string]string) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authed := middleware.RequireAuth(verifier, "", logger)(handlers.NewRouter(pool.Pool, nil))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for name, value := range headers {
			r.Header.Set(name, value)
		}
		authed.ServeHTTP(w, r)
	})
}

var (
	edgeAdmin  = map[string]string{"Remote-User": "chris", "Remote-Groups": "dev, admin"}
	cliRequest = map[string]string{"Authorization": "Bearer header.payload.signature"}
)

func TestFeedback_CaptureListFilterUpdateDelete(t *testing.T) {
	mux := asCaller(setupTestDB(t), nil, edgeAdmin)

	// Capture — status defaults to open, the route is retained.
	rr := postJSON(t, mux, "/api/v1/feedback", map[string]any{
		"body": "Session timer resets when the screen sleeps", "context_path": "/sessions/3",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	first := decodeFeedback(t, rr.Body)
	assert.Equal(t, "open", first.Status)
	assert.Equal(t, "/sessions/3", first.ContextPath)
	assert.NotEqual(t, "", first.ID.String())

	// A blank body is a 400 — a capture nobody can act on is never what was meant.
	blank := postJSON(t, mux, "/api/v1/feedback", map[string]any{"body": "   "})
	assert.Equal(t, http.StatusBadRequest, blank.Code)

	// context_path is optional.
	second := postJSON(t, mux, "/api/v1/feedback", map[string]any{"body": "Cycle detail should show the week"})
	require.Equal(t, http.StatusCreated, second.Code)
	secondID := decodeFeedback(t, second.Body).ID

	// List is newest first.
	list := getJSON(t, mux, "/api/v1/feedback")
	require.Equal(t, http.StatusOK, list.Code)
	var items []models.Feedback
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &items))
	require.Len(t, items, 2)
	assert.Equal(t, secondID, items[0].ID)

	// Closing one moves it out of the open filter and into the done filter.
	closed := putJSON(t, mux, "/api/v1/feedback/"+first.ID.String(), map[string]any{"status": "done"})
	require.Equal(t, http.StatusOK, closed.Code)
	assert.Equal(t, "done", decodeFeedback(t, closed.Body).Status)

	open := getJSON(t, mux, "/api/v1/feedback?status=open")
	require.NoError(t, json.Unmarshal(open.Body.Bytes(), &items))
	require.Len(t, items, 1)
	assert.Equal(t, secondID, items[0].ID)

	// Search matches the body case-insensitively.
	found := getJSON(t, mux, "/api/v1/feedback?search=TIMER")
	require.NoError(t, json.Unmarshal(found.Body.Bytes(), &items))
	require.Len(t, items, 1)
	assert.Equal(t, first.ID, items[0].ID)

	// An out-of-range status is a 400 (the CHECK), an unknown id a 404, and a
	// malformed uuid a 400 rather than a 404.
	assert.Equal(t, http.StatusBadRequest,
		putJSON(t, mux, "/api/v1/feedback/"+first.ID.String(), map[string]any{"status": "maybe"}).Code)
	assert.Equal(t, http.StatusNotFound,
		getJSON(t, mux, "/api/v1/feedback/019f0000-0000-7000-8000-000000000000").Code)
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/feedback/not-a-uuid").Code)

	// Delete removes it.
	require.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/feedback/"+first.ID.String()).Code)
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/feedback/"+first.ID.String()).Code)
}

// The viewport is what separates a density complaint on a phone from a line-length
// one on a desktop, so it has to survive the round trip — and stay absent when the
// client had no window to measure.
func TestFeedback_ViewportRoundTrip(t *testing.T) {
	mux := asCaller(setupTestDB(t), nil, edgeAdmin)

	rr := postJSON(t, mux, "/api/v1/feedback", map[string]any{
		"body": "how-to is a wall of text", "context_path": "/movements/21",
		"viewport_width": 390, "viewport_height": 844,
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	phone := decodeFeedback(t, rr.Body)
	require.NotNil(t, phone.ViewportWidth)
	require.NotNil(t, phone.ViewportHeight)
	assert.Equal(t, 390, *phone.ViewportWidth)
	assert.Equal(t, 844, *phone.ViewportHeight)

	// A CLI capture omits it entirely; a client that sends 0 failed to measure rather
	// than measuring zero, and both read back as "no viewport" rather than as data.
	terminal := decodeFeedback(t, postJSON(t, mux, "/api/v1/feedback", map[string]any{
		"body": "filed from a shell",
	}).Body)
	assert.Nil(t, terminal.ViewportWidth)
	assert.Nil(t, terminal.ViewportHeight)

	zeroed := decodeFeedback(t, postJSON(t, mux, "/api/v1/feedback", map[string]any{
		"body": "measured nothing", "viewport_width": 0, "viewport_height": 0,
	}).Body)
	assert.Nil(t, zeroed.ViewportWidth)
	assert.Nil(t, zeroed.ViewportHeight)

	// Editing the body leaves the capture-time viewport alone.
	edited := putJSON(t, mux, "/api/v1/feedback/"+phone.ID.String(),
		map[string]any{"body": "how-to is a wall of text on the phone"})
	require.Equal(t, http.StatusOK, edited.Code)
	after := decodeFeedback(t, edited.Body)
	require.NotNil(t, after.ViewportWidth)
	assert.Equal(t, 390, *after.ViewportWidth)
}

// Feedback is the one resource here that is not training data, so it must not leak
// into the reads that compose the training picture — `meso review` in particular,
// which is what Claude parses when drafting the next cycle.
func TestFeedback_AbsentFromReviewAndStats(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	require.Equal(t, http.StatusCreated,
		postJSON(t, mux, "/api/v1/feedback", map[string]any{"body": "unrelated to training"}).Code)

	review := getJSON(t, mux, "/api/v1/review")
	require.Equal(t, http.StatusOK, review.Code)
	assert.NotContains(t, review.Body.String(), "unrelated to training")

	stats := getJSON(t, mux, "/api/v1/stats")
	require.Equal(t, http.StatusOK, stats.Code)
	assert.NotContains(t, stats.Body.String(), "unrelated to training")
}

// Reading, editing and deleting feedback is administration, which only the admin
// group may do from the browser or the CLI. Capturing it is what every user does.
func TestFeedback_AdministrationNeedsTheAdminGroupOnEitherPath(t *testing.T) {
	pool := setupTestDB(t)
	seeded := decodeFeedback(t, postJSON(t, asCaller(pool, nil, edgeAdmin), "/api/v1/feedback",
		map[string]any{"body": "seeded by an admin"}).Body)
	item := "/api/v1/feedback/" + seeded.ID.String()

	outsiders := map[string]http.Handler{
		"browser outside admin":  asCaller(pool, nil, map[string]string{"Remote-User": "guest", "Remote-Groups": "dev"}),
		"browser with no groups": asCaller(pool, nil, map[string]string{"Remote-User": "guest"}),
		"CLI outside admin": asCaller(pool,
			verifiedAs{Subject: "guest", ClientID: "meso-cli-laptop", Groups: []string{"dev"}}, cliRequest),
		"CLI token with no groups claim": asCaller(pool,
			verifiedAs{Subject: "guest", ClientID: "meso-cli-laptop"}, cliRequest),
		"CLI outside admin with a forged Remote-Groups": asCaller(pool,
			verifiedAs{Subject: "guest", ClientID: "meso-cli-laptop"},
			map[string]string{"Authorization": cliRequest["Authorization"], "Remote-User": "chris", "Remote-Groups": "admin"}),
	}
	for name, mux := range outsiders {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, http.StatusForbidden, getJSON(t, mux, "/api/v1/feedback").Code)
			assert.Equal(t, http.StatusForbidden, getJSON(t, mux, item).Code)
			assert.Equal(t, http.StatusForbidden, putJSON(t, mux, item, map[string]any{"status": "done"}).Code)
			assert.Equal(t, http.StatusForbidden, deleteReq(t, mux, item).Code)
			assert.Equal(t, http.StatusCreated,
				postJSON(t, mux, "/api/v1/feedback", map[string]any{"body": "captured by " + name}).Code)
		})
	}

	cliAdmin := asCaller(pool, verifiedAs{Subject: "chris", ClientID: "meso-cli-laptop", Groups: []string{"admin"}}, cliRequest)
	assert.Equal(t, http.StatusOK, getJSON(t, cliAdmin, "/api/v1/feedback").Code)
	assert.Equal(t, http.StatusOK, getJSON(t, cliAdmin, item).Code)
	assert.Equal(t, http.StatusOK, putJSON(t, cliAdmin, item, map[string]any{"status": "done"}).Code)
	assert.Equal(t, http.StatusNoContent, deleteReq(t, cliAdmin, item).Code)
	assert.Equal(t, http.StatusCreated,
		postJSON(t, cliAdmin, "/api/v1/feedback", map[string]any{"body": "captured by an admin's CLI"}).Code)
}
