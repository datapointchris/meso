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
