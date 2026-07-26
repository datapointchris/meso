package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"meso/api/models"
)

func decodeMeasurement(t *testing.T, rr interface{ Bytes() []byte }) models.Measurement {
	t.Helper()
	var m models.Measurement
	require.NoError(t, json.Unmarshal(rr.Bytes(), &m))
	return m
}

func decodeTrend(t *testing.T, rr interface{ Bytes() []byte }) models.MetricTrend {
	t.Helper()
	var tr models.MetricTrend
	require.NoError(t, json.Unmarshal(rr.Bytes(), &tr))
	return tr
}

// defineMetric defines a strength metric and asserts the 201, returning nothing —
// the name is the handle the measurement tests reference.
func defineMetric(t *testing.T, mux *http.ServeMux, name, unit, direction, category string) {
	t.Helper()
	rr := postJSON(t, mux, "/api/v1/metrics", map[string]any{
		"name": name, "unit": unit, "direction": direction, "category": category,
	})
	require.Equal(t, http.StatusCreated, rr.Code)
}

func TestMetric_Define_ListAndValidation(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	defineMetric(t, mux, "deadlift-working-weight", "lb", "higher_better", "strength")
	defineMetric(t, mux, "5k-time", "seconds", "lower_better", "cardio")

	// Duplicate name -> 409.
	dup := postJSON(t, mux, "/api/v1/metrics", map[string]any{
		"name": "5k-time", "unit": "seconds", "direction": "lower_better", "category": "cardio",
	})
	assert.Equal(t, http.StatusConflict, dup.Code)

	// Out-of-range direction / category (CHECK) -> 400.
	badDir := postJSON(t, mux, "/api/v1/metrics", map[string]any{
		"name": "x", "unit": "lb", "direction": "sideways", "category": "strength",
	})
	assert.Equal(t, http.StatusBadRequest, badDir.Code)
	badCat := postJSON(t, mux, "/api/v1/metrics", map[string]any{
		"name": "y", "unit": "lb", "direction": "higher_better", "category": "vibes",
	})
	assert.Equal(t, http.StatusBadRequest, badCat.Code)

	// Listed, grouped by category then label (cardio before strength). An omitted
	// label is derived from the name, so no metric ever reads as a raw slug.
	rr := getJSON(t, mux, "/api/v1/metrics")
	require.Equal(t, http.StatusOK, rr.Code)
	var metrics []models.MetricDefinition
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &metrics))
	require.Len(t, metrics, 2)
	assert.Equal(t, "5k-time", metrics[0].Name)
	assert.Equal(t, "5k Time", metrics[0].Label)
	assert.Equal(t, "deadlift-working-weight", metrics[1].Name)
	assert.Equal(t, "Deadlift Working Weight", metrics[1].Label)
}

func TestMetric_ExplicitLabelAndUpdate(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	// An explicit label wins over the derivation.
	rr := postJSON(t, mux, "/api/v1/metrics", map[string]any{
		"name": "row-machine-500m", "label": "Row 500m",
		"unit": "seconds", "direction": "lower_better", "category": "cardio",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	var created models.MetricDefinition
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	assert.Equal(t, "Row 500m", created.Label)

	// A partial update touches only the fields it carries.
	updated := putJSON(t, mux, "/api/v1/metrics/row-machine-500m", map[string]any{
		"label": "Row (500m split)",
	})
	require.Equal(t, http.StatusOK, updated.Code)
	var after models.MetricDefinition
	require.NoError(t, json.Unmarshal(updated.Body.Bytes(), &after))
	assert.Equal(t, "Row (500m split)", after.Label)
	assert.Equal(t, "seconds", after.Unit)
	assert.Equal(t, "lower_better", after.Direction)
	assert.Equal(t, "cardio", after.Category)

	// An out-of-range category is still a 400 on update, and an unknown name a 404.
	bad := putJSON(t, mux, "/api/v1/metrics/row-machine-500m", map[string]any{"category": "vibes"})
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	missing := putJSON(t, mux, "/api/v1/metrics/nope", map[string]any{"label": "Nope"})
	assert.Equal(t, http.StatusNotFound, missing.Code)
}

// A metric's name says what to call the number, never what it is or how to produce
// it. The protocol has to survive define, come back on the single-metric GET (the
// list is a table with no room for a paragraph), and ride along on the stats payload
// so the app can explain a stat without a second request per card.
func TestMetric_HowToMeasure(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	const protocol = "Single-leg heel raises to failure on flat ground.\n\nKnee straight, full range."
	rr := postJSON(t, mux, "/api/v1/metrics", map[string]any{
		"name": "heel-raise-capacity-right", "unit": "reps",
		"direction": "higher_better", "category": "mobility",
		"how_to_measure": protocol,
	})
	require.Equal(t, http.StatusCreated, rr.Code)

	got := getJSON(t, mux, "/api/v1/metrics/heel-raise-capacity-right")
	require.Equal(t, http.StatusOK, got.Code)
	var definition models.MetricDefinition
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &definition))
	assert.Equal(t, protocol, definition.HowToMeasure)

	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/metrics/nope").Code)

	// Undocumented is a real state — empty, never derived from the name the way a
	// label is, because there is nothing in "toe-reach" to derive a protocol from.
	defineMetric(t, mux, "toe-reach", "cm", "higher_better", "mobility")
	bare := getJSON(t, mux, "/api/v1/metrics/toe-reach")
	require.NoError(t, json.Unmarshal(bare.Body.Bytes(), &definition))
	assert.Equal(t, "", definition.HowToMeasure)

	// Editable after the fact, and a partial update leaves it alone.
	const revised = "Single-leg heel raises to failure, knee straight."
	updated := putJSON(t, mux, "/api/v1/metrics/heel-raise-capacity-right",
		map[string]any{"how_to_measure": revised})
	require.Equal(t, http.StatusOK, updated.Code)
	require.NoError(t, json.Unmarshal(updated.Body.Bytes(), &definition))
	assert.Equal(t, revised, definition.HowToMeasure)

	relabeled := putJSON(t, mux, "/api/v1/metrics/heel-raise-capacity-right",
		map[string]any{"label": "Heel Raise Capacity (R)"})
	require.NoError(t, json.Unmarshal(relabeled.Body.Bytes(), &definition))
	assert.Equal(t, revised, definition.HowToMeasure)

	// The stats payload carries it, so tapping a card explains itself offline of a
	// second fetch — including for the metric that has no readings at all.
	stats := getJSON(t, mux, "/api/v1/stats")
	require.Equal(t, http.StatusOK, stats.Code)
	var summary models.StatsSummary
	require.NoError(t, json.Unmarshal(stats.Body.Bytes(), &summary))
	byName := map[string]models.MetricTrend{}
	for _, trend := range summary.Metrics {
		byName[trend.Metric] = trend
	}
	assert.Equal(t, revised, byName["heel-raise-capacity-right"].HowToMeasure)
	assert.Equal(t, "", byName["toe-reach"].HowToMeasure)
}

func TestMeasurement_RecordListFilterUpdateDelete(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	defineMetric(t, mux, "deadlift-working-weight", "lb", "higher_better", "strength")

	// Record a reading — measured_on and value round-trip; source defaults to manual.
	rr := postJSON(t, mux, "/api/v1/measurements", map[string]any{
		"metric": "deadlift-working-weight", "value": 185, "measured_on": "2026-07-01", "notes": "RPE 7",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	first := decodeMeasurement(t, rr.Body)
	assert.Equal(t, 185.0, first.Value)
	assert.Equal(t, "2026-07-01", first.MeasuredOn)
	assert.Equal(t, "manual", first.Source)
	assert.Equal(t, "RPE 7", first.Notes)

	// A decimal value survives the NUMERIC column round-trip.
	rrMid := postJSON(t, mux, "/api/v1/measurements", map[string]any{
		"metric": "deadlift-working-weight", "value": 202.5, "measured_on": "2026-07-15",
	})
	require.Equal(t, http.StatusCreated, rrMid.Code)
	assert.Equal(t, 202.5, decodeMeasurement(t, rrMid.Body).Value)

	postJSON(t, mux, "/api/v1/measurements", map[string]any{
		"metric": "deadlift-working-weight", "value": 225, "measured_on": "2026-07-22",
	})

	// List newest-first, with date filtering owned by the server.
	list := func(query string) []models.Measurement {
		lr := getJSON(t, mux, "/api/v1/measurements"+query)
		require.Equal(t, http.StatusOK, lr.Code)
		var out []models.Measurement
		require.NoError(t, json.Unmarshal(lr.Body.Bytes(), &out))
		return out
	}
	all := list("?metric=deadlift-working-weight")
	require.Len(t, all, 3)
	assert.Equal(t, "2026-07-22", all[0].MeasuredOn) // newest first
	assert.Len(t, list("?from=2026-07-10"), 2)
	assert.Len(t, list("?to=2026-07-01"), 1)

	// Update value/notes; metric stays fixed.
	up := putJSON(t, mux, "/api/v1/measurements/"+itoa(first.ID), map[string]any{
		"value": 190, "notes": "re-tested",
	})
	require.Equal(t, http.StatusOK, up.Code)
	updated := decodeMeasurement(t, up.Body)
	assert.Equal(t, 190.0, updated.Value)
	assert.Equal(t, "re-tested", updated.Notes)
	assert.Equal(t, "2026-07-01", updated.MeasuredOn) // unchanged

	// Delete frees it; a second delete is a 404.
	require.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/measurements/"+itoa(first.ID)).Code)
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/measurements/"+itoa(first.ID)).Code)
}

func TestMeasurement_Validation(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	defineMetric(t, mux, "bodyweight", "lb", "higher_better", "body")

	// Unknown metric (FK) -> 409.
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/measurements", map[string]any{
		"metric": "not-a-metric", "value": 100,
	}).Code)
	// Bad date -> 400.
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, "/api/v1/measurements", map[string]any{
		"metric": "bodyweight", "value": 100, "measured_on": "last tuesday",
	}).Code)
	// Bad list filter -> 400.
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/measurements?from=nope").Code)
	// Non-numeric id -> 400; well-formed but absent -> 404.
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/measurements/abc").Code)
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/measurements/999999").Code)
}

func TestMetric_Delete(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	defineMetric(t, mux, "continuous-easy-run", "minutes", "higher_better", "cardio")
	defineMetric(t, mux, "deadlift-working-weight", "lb", "higher_better", "strength")

	// An unreferenced metric deletes -> 204 and is gone (its trend 404s).
	require.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/metrics/continuous-easy-run").Code)
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/metrics/continuous-easy-run/trend").Code)

	// A metric with a recorded measurement is RESTRICT-guarded -> 409.
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/measurements", map[string]any{
		"metric": "deadlift-working-weight", "value": 225,
	}).Code)
	assert.Equal(t, http.StatusConflict, deleteReq(t, mux, "/api/v1/metrics/deadlift-working-weight").Code)

	// Unknown metric -> 404.
	assert.Equal(t, http.StatusNotFound, deleteReq(t, mux, "/api/v1/metrics/ghost").Code)
}

func TestMetric_Trend_SeriesAndSummary(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	defineMetric(t, mux, "5k-time", "seconds", "lower_better", "cardio")

	// Record out of date order to prove the trend orders by date, not insert order.
	for _, m := range []map[string]any{
		{"metric": "5k-time", "value": 1700, "measured_on": "2026-07-20"},
		{"metric": "5k-time", "value": 1800, "measured_on": "2026-07-01"},
		{"metric": "5k-time", "value": 1650, "measured_on": "2026-07-27"},
	} {
		require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/measurements", m).Code)
	}

	rr := getJSON(t, mux, "/api/v1/metrics/5k-time/trend")
	require.Equal(t, http.StatusOK, rr.Code)
	trend := decodeTrend(t, rr.Body)

	assert.Equal(t, "lower_better", trend.Direction)
	require.Len(t, trend.Points, 3)
	// Oldest-first for a left-to-right chart.
	assert.Equal(t, "2026-07-01", trend.Points[0].MeasuredOn)
	assert.Equal(t, 1800.0, trend.Points[0].Value)
	assert.Equal(t, "2026-07-27", trend.Points[2].MeasuredOn)
	// Summary: first, latest, change (latest − first), count.
	require.NotNil(t, trend.First)
	require.NotNil(t, trend.Latest)
	require.NotNil(t, trend.Change)
	assert.Equal(t, 1800.0, *trend.First)
	assert.Equal(t, 1650.0, *trend.Latest)
	assert.Equal(t, -150.0, *trend.Change) // improvement for a lower_better metric
	assert.Equal(t, 3, trend.Count)

	// Windowed by ?from — only the two on/after the bound.
	windowed := decodeTrend(t, getJSON(t, mux, "/api/v1/metrics/5k-time/trend?from=2026-07-15").Body)
	assert.Len(t, windowed.Points, 2)

	// A defined-but-unmeasured metric trends empty (not null), summary nil.
	defineMetric(t, mux, "toe-reach", "cm", "higher_better", "mobility")
	empty := decodeTrend(t, getJSON(t, mux, "/api/v1/metrics/toe-reach/trend").Body)
	assert.Equal(t, 0, empty.Count)
	require.NotNil(t, empty.Points)
	assert.Len(t, empty.Points, 0)
	assert.Nil(t, empty.Latest)

	// An unknown metric -> 404.
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/metrics/ghost/trend").Code)
}
