package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"meso/api/models"
)

func TestStats_Summary(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	// Library: three movements, one a favorite, spanning two kinds.
	squat := createMovement(t, mux, "Back Squat", "exercise")
	createMovement(t, mux, "Couch Stretch", "stretch")
	fav := postJSON(t, mux, "/api/v1/movements", map[string]any{
		"name": "Pigeon Pose", "movement_kind": "yoga_pose", "favorite": true,
	})
	require.Equal(t, http.StatusCreated, fav.Code)

	// Sessions: two recent ad-hoc sessions feed the frequency + recency counts.
	today := time.Now().Format("2006-01-02")
	recent := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/sessions", map[string]any{
		"performed_on": today, "movements": []map[string]any{{"movement_id": squat}},
	}).Code)
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/sessions", map[string]any{
		"performed_on": recent,
	}).Code)

	// Measurements: two metrics, one with a two-point trend, one unmeasured.
	defineMetric(t, mux, "deadlift-working-weight", "lb", "higher_better", "strength")
	defineMetric(t, mux, "toe-reach", "cm", "higher_better", "mobility")
	postJSON(t, mux, "/api/v1/measurements", map[string]any{
		"metric": "deadlift-working-weight", "value": 185, "measured_on": "2026-07-01",
	})
	postJSON(t, mux, "/api/v1/measurements", map[string]any{
		"metric": "deadlift-working-weight", "value": 225, "measured_on": "2026-07-22",
	})

	rr := getJSON(t, mux, "/api/v1/stats")
	require.Equal(t, http.StatusOK, rr.Code)
	var stats models.StatsSummary
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &stats))

	// Library counts.
	assert.Equal(t, 3, stats.Library.TotalMovements)
	assert.Equal(t, 1, stats.Library.Favorites)
	byKind := map[string]int{}
	for _, kc := range stats.Library.ByKind {
		byKind[kc.Kind] = kc.Count
	}
	assert.Equal(t, 1, byKind["exercise"])
	assert.Equal(t, 1, byKind["stretch"])
	assert.Equal(t, 1, byKind["yoga_pose"])

	// Session counts: both are recent, so total, last-30, and the by-week series agree.
	assert.Equal(t, 2, stats.Sessions.Total)
	assert.Equal(t, 2, stats.Sessions.Last30Days)
	weekTotal := 0
	for _, wc := range stats.Sessions.ByWeek {
		weekTotal += wc.Count
	}
	assert.Equal(t, 2, weekTotal)

	// Every *defined* metric appears, measured or not — the stats payload is the
	// vocabulary as well as the history, so the UI can offer an unmeasured metric to
	// record against. Ordered by (category, label): mobility before strength.
	require.Len(t, stats.Metrics, 2)
	toes := stats.Metrics[0]
	assert.Equal(t, "toe-reach", toes.Metric)
	assert.Equal(t, "Toe Reach", toes.Label)
	assert.Empty(t, toes.Points)
	assert.Equal(t, 0, toes.Count)
	assert.Nil(t, toes.First)
	assert.Nil(t, toes.Latest)
	assert.Nil(t, toes.Change)

	dead := stats.Metrics[1]
	assert.Equal(t, "deadlift-working-weight", dead.Metric)
	assert.Equal(t, "Deadlift Working Weight", dead.Label)
	require.Len(t, dead.Points, 2)
	require.NotNil(t, dead.Change)
	assert.Equal(t, 40.0, *dead.Change)
}
