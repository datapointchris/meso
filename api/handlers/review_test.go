package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"meso/api/models"
)

func decodeReview(t *testing.T, rr interface{ Bytes() []byte }) models.Review {
	t.Helper()
	var rev models.Review
	require.NoError(t, json.Unmarshal(rr.Bytes(), &rev))
	return rev
}

func TestReview_WindowsRecentHistoryAndActiveCycles(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	defineMetric(t, mux, "deadlift-working-weight", "lb", "higher_better", "strength")
	w := createWorkout(t, mux, "Session Template")

	// Recent reality (defaults to today — inside any window).
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": w}).Code)
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/measurements", map[string]any{
		"metric": "deadlift-working-weight", "value": 305,
	}).Code)
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/log", map[string]any{"body": "felt strong today"}).Code)

	// Old reality (well outside a 30-day window).
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/sessions", map[string]any{
		"workout_id": w, "performed_on": "2020-01-01",
	}).Code)
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/measurements", map[string]any{
		"metric": "deadlift-working-weight", "value": 225, "measured_on": "2020-01-01",
	}).Code)
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/log", map[string]any{
		"body": "ancient entry", "entry_date": "2020-01-01",
	}).Code)

	// Only an active cycle is context; a planned one is not surfaced by review.
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Current block", "status": "active",
	}).Code)
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Next block", "status": "planned",
	}).Code)

	// Default window (30d): recent slice only, active cycles only.
	rr := getJSON(t, mux, "/api/v1/review")
	require.Equal(t, http.StatusOK, rr.Code)
	rev := decodeReview(t, rr.Body)
	assert.NotEmpty(t, rev.Since)
	require.Len(t, rev.ActiveCycles, 1)
	assert.Equal(t, "Current block", rev.ActiveCycles[0].Name)
	assert.Len(t, rev.Sessions, 1)
	assert.Len(t, rev.Measurements, 1)
	assert.Len(t, rev.LogEntries, 1)

	// A wide window pulls in the old rows too.
	rev = decodeReview(t, getJSON(t, mux, "/api/v1/review?since=520w").Body)
	assert.Len(t, rev.Sessions, 2)
	assert.Len(t, rev.Measurements, 2)
	assert.Len(t, rev.LogEntries, 2)

	// A malformed window is a 400.
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/review?since=soon").Code)
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/review?since=30x").Code)
}
