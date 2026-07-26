package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"meso/api/models"
)

func decodeFeedback(t *testing.T, rr interface{ Bytes() []byte }) models.Feedback {
	t.Helper()
	var f models.Feedback
	require.NoError(t, json.Unmarshal(rr.Bytes(), &f))
	return f
}

func TestFeedback_CaptureListFilterUpdateDelete(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

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
	mux := buildTestMux(setupTestDB(t))

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
