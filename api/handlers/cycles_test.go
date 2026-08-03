package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"meso/api/models"
)

func decodeCycle(t *testing.T, rr interface{ Bytes() []byte }) models.Cycle {
	t.Helper()
	var c models.Cycle
	require.NoError(t, json.Unmarshal(rr.Bytes(), &c))
	return c
}

// createWorkout posts a bare workout and returns its id — the building block for
// cycle-sequence tests.
func createWorkout(t *testing.T, mux *http.ServeMux, name string) int64 {
	t.Helper()
	rr := postJSON(t, mux, "/api/v1/workouts", map[string]any{"name": name})
	require.Equal(t, http.StatusCreated, rr.Code)
	return decodeWorkout(t, rr.Body).ID
}

func TestCycle_CreateWithWorkouts_AndGet(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	defineMetric(t, mux, "deadlift-working-weight", "lb", "higher_better", "strength")
	base := createWorkout(t, mux, "Base Week")
	build := createWorkout(t, mux, "Build Week")

	body := map[string]any{
		"name":          "Return to 5k",
		"goal_summary":  "12-week run return",
		"target_metric": "deadlift-working-weight",
		"target_value":  315,
		"start_date":    "2026-08-01",
		"target_date":   "2026-10-24",
		"status":        "active",
		"workouts": []map[string]any{
			{"workout_id": base, "week": 1, "phase": "base", "frequency": "3×/week", "intensity": "easy / Zone 2"},
			{"workout_id": build, "week": 5, "phase": "build", "conditions": "when knee-to-wall symmetric, advance"},
		},
	}
	rr := postJSON(t, mux, "/api/v1/cycles", body)
	require.Equal(t, http.StatusCreated, rr.Code)

	created := decodeCycle(t, rr.Body)
	assert.Equal(t, "Return to 5k", created.Name)
	assert.Equal(t, "active", created.Status)
	require.NotNil(t, created.TargetMetric)
	assert.Equal(t, "deadlift-working-weight", *created.TargetMetric)
	require.NotNil(t, created.TargetValue)
	assert.InDelta(t, 315, *created.TargetValue, 0.001)
	require.NotNil(t, created.StartDate)
	assert.Equal(t, "2026-08-01", *created.StartDate)
	require.NotNil(t, created.TargetDate)
	assert.Equal(t, "2026-10-24", *created.TargetDate)

	require.Len(t, created.Workouts, 2)
	// Ordered by position; the workout name/theme are embedded for render.
	assert.Equal(t, 1, created.Workouts[0].Position)
	assert.Equal(t, "Base Week", created.Workouts[0].WorkoutName)
	require.NotNil(t, created.Workouts[0].Week)
	assert.Equal(t, 1, *created.Workouts[0].Week)
	require.NotNil(t, created.Workouts[0].Phase)
	assert.Equal(t, "base", *created.Workouts[0].Phase)
	assert.Equal(t, 2, created.Workouts[1].Position)
	require.NotNil(t, created.Workouts[1].Conditions)

	got := getJSON(t, mux, "/api/v1/cycles/"+itoa(created.ID))
	require.Equal(t, http.StatusOK, got.Code)
	assert.Len(t, decodeCycle(t, got.Body).Workouts, 2)
}

func TestCycle_Create_DefaultsStatusToPlanned(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	created := decodeCycle(t, postJSON(t, mux, "/api/v1/cycles", map[string]any{"name": "Draft cycle"}).Body)
	assert.Equal(t, "planned", created.Status)
	assert.Nil(t, created.StartDate)
	assert.Nil(t, created.TargetMetric)
	assert.Empty(t, created.Workouts)
}

func TestCycle_List_Filters(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Shoulder rehab", "goal_summary": "restore overhead ROM", "status": "active",
	}).Code)
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Dance conditioning", "status": "paused",
	}).Code)

	list := func(query string) []models.Cycle {
		rr := getJSON(t, mux, "/api/v1/cycles"+query)
		require.Equal(t, http.StatusOK, rr.Code)
		var out []models.Cycle
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
		return out
	}

	assert.Len(t, list(""), 2)
	assert.Len(t, list("?status=active"), 1)
	assert.Equal(t, "Shoulder rehab", list("?status=active")[0].Name)
	assert.Len(t, list("?status=paused"), 1)
	assert.Len(t, list("?search=rehab"), 1)
	assert.Len(t, list("?search=overhead"), 1) // matches goal_summary
	assert.Empty(t, list("?status=complete"))
}

func TestCycle_Update_Partial_AndClearDate(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	created := decodeCycle(t, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Draft", "start_date": "2026-08-01", "status": "planned",
	}).Body)

	// Partial update: advance status, set a goal — start_date left untouched.
	rr := putJSON(t, mux, "/api/v1/cycles/"+itoa(created.ID), map[string]any{
		"status": "active", "goal_summary": "peak for the meet",
	})
	require.Equal(t, http.StatusOK, rr.Code)
	updated := decodeCycle(t, rr.Body)
	assert.Equal(t, "active", updated.Status)
	assert.Equal(t, "peak for the meet", updated.GoalSummary)
	assert.Equal(t, "Draft", updated.Name) // unchanged
	require.NotNil(t, updated.StartDate)
	assert.Equal(t, "2026-08-01", *updated.StartDate) // unchanged

	// An explicit empty start_date clears it to null.
	rr = putJSON(t, mux, "/api/v1/cycles/"+itoa(created.ID), map[string]any{"start_date": ""})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Nil(t, decodeCycle(t, rr.Body).StartDate)
}

func TestCycle_Update_ChangingMetricClearsStaleValue(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	defineMetric(t, mux, "continuous-easy-run", "minutes", "higher_better", "cardio")
	defineMetric(t, mux, "5k-time", "seconds", "lower_better", "cardio")

	created := decodeCycle(t, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Return to running", "target_metric": "continuous-easy-run", "target_value": 30,
	}).Body)
	require.NotNil(t, created.TargetValue)

	// Repointing the metric without a new value clears the stale value — 30 minutes
	// means nothing against 5k-time.
	updated := decodeCycle(t, putJSON(t, mux, "/api/v1/cycles/"+itoa(created.ID), map[string]any{
		"target_metric": "5k-time",
	}).Body)
	require.NotNil(t, updated.TargetMetric)
	assert.Equal(t, "5k-time", *updated.TargetMetric)
	assert.Nil(t, updated.TargetValue)

	// Supplying a value alongside the metric keeps it.
	kept := decodeCycle(t, putJSON(t, mux, "/api/v1/cycles/"+itoa(created.ID), map[string]any{
		"target_metric": "5k-time", "target_value": 1500,
	}).Body)
	require.NotNil(t, kept.TargetValue)
	assert.Equal(t, 1500.0, *kept.TargetValue)
}

func TestCycle_ComposeWorkouts_AddSwapReorderRemove(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	wA := createWorkout(t, mux, "Week A")
	wB := createWorkout(t, mux, "Week B")
	wAlt := createWorkout(t, mux, "Deload Week")

	c := decodeCycle(t, postJSON(t, mux, "/api/v1/cycles", map[string]any{"name": "Block 1"}).Body)
	base := "/api/v1/cycles/" + itoa(c.ID) + "/workouts"

	// Add two workouts — each POST returns the refreshed cycle.
	_ = decodeCycle(t, postJSON(t, mux, base, map[string]any{"workout_id": wA, "week": 1, "phase": "base"}).Body)
	c = decodeCycle(t, postJSON(t, mux, base, map[string]any{"workout_id": wB, "week": 2, "phase": "build"}).Body)
	require.Len(t, c.Workouts, 2)
	entryA := c.Workouts[0].ID
	entryB := c.Workouts[1].ID
	assert.Equal(t, wA, c.Workouts[0].WorkoutID)

	// Swap the first entry for a deload — the prescription (week/phase) carries over.
	rr := patchJSON(t, mux, base+"/"+itoa(entryA), map[string]any{"workout_id": wAlt})
	require.Equal(t, http.StatusOK, rr.Code)
	c = decodeCycle(t, rr.Body)
	assert.Equal(t, wAlt, c.Workouts[0].WorkoutID)
	assert.Equal(t, "Deload Week", c.Workouts[0].WorkoutName)
	require.NotNil(t, c.Workouts[0].Phase)
	assert.Equal(t, "base", *c.Workouts[0].Phase) // prescription carried to the swap

	// Reorder: B first, then the swapped A entry.
	rr = patchJSON(t, mux, base, map[string]any{"entry_ids": []int64{entryB, entryA}})
	require.Equal(t, http.StatusOK, rr.Code)
	c = decodeCycle(t, rr.Body)
	assert.Equal(t, entryB, c.Workouts[0].ID)
	assert.Equal(t, 1, c.Workouts[0].Position)
	assert.Equal(t, entryA, c.Workouts[1].ID)
	assert.Equal(t, 2, c.Workouts[1].Position)

	// A reorder that doesn't cover every entry is a 400.
	assert.Equal(t, http.StatusBadRequest,
		patchJSON(t, mux, base, map[string]any{"entry_ids": []int64{entryB}}).Code)

	// Remove one entry.
	rr = deleteReq(t, mux, base+"/"+itoa(entryB))
	require.Equal(t, http.StatusOK, rr.Code)
	c = decodeCycle(t, rr.Body)
	require.Len(t, c.Workouts, 1)
	assert.Equal(t, entryA, c.Workouts[0].ID)

	// Removing an unknown entry is a 404.
	assert.Equal(t, http.StatusNotFound, deleteReq(t, mux, base+"/999999").Code)
}

func TestCycle_Validation(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	w := createWorkout(t, mux, "Some Week")

	// Duplicate name -> 409.
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/cycles", map[string]any{"name": "Dup"}).Code)
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/cycles", map[string]any{"name": "Dup"}).Code)

	// Missing name -> 400.
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, "/api/v1/cycles", map[string]any{"goal_summary": "x"}).Code)

	// Unknown status (FK) -> 409.
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Bad status", "status": "nonsense",
	}).Code)

	// Unknown target_metric (FK) -> 409.
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Bad metric", "target_metric": "does-not-exist",
	}).Code)

	// Malformed date -> 400.
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Bad date", "start_date": "08/01/2026",
	}).Code)

	// Unknown workout in the sequence (FK) -> 409.
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Bad seq", "workouts": []map[string]any{{"workout_id": 999999}},
	}).Code)

	// Adding an unknown workout to an existing cycle -> 409.
	c := decodeCycle(t, postJSON(t, mux, "/api/v1/cycles", map[string]any{"name": "Real"}).Body)
	assert.Equal(t, http.StatusConflict,
		postJSON(t, mux, "/api/v1/cycles/"+itoa(c.ID)+"/workouts", map[string]any{"workout_id": 999999}).Code)
	// Missing workout_id on add -> 400.
	assert.Equal(t, http.StatusBadRequest,
		postJSON(t, mux, "/api/v1/cycles/"+itoa(c.ID)+"/workouts", map[string]any{"week": 3}).Code)
	// A real workout adds fine.
	assert.Equal(t, http.StatusOK,
		postJSON(t, mux, "/api/v1/cycles/"+itoa(c.ID)+"/workouts", map[string]any{"workout_id": w}).Code)
}

func TestCycle_Delete_RestrictsWorkout(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	w := createWorkout(t, mux, "Referenced Week")
	c := decodeCycle(t, postJSON(t, mux, "/api/v1/cycles", map[string]any{
		"name": "Temp", "workouts": []map[string]any{{"workout_id": w}},
	}).Body)

	// A workout referenced by a cycle can't be deleted (FK RESTRICT) -> 409.
	assert.Equal(t, http.StatusConflict, deleteReq(t, mux, "/api/v1/workouts/"+itoa(w)).Code)

	// Deleting the cycle cascades its entries and frees the workout.
	assert.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/cycles/"+itoa(c.ID)).Code)
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/cycles/"+itoa(c.ID)).Code)
	assert.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/workouts/"+itoa(w)).Code)
}
