package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"meso/api/models"
)

func decodeWorkout(t *testing.T, rr interface{ Bytes() []byte }) models.Workout {
	t.Helper()
	var w models.Workout
	require.NoError(t, json.Unmarshal(rr.Bytes(), &w))
	return w
}

// createMovement posts a movement and returns its id — the building block for
// workout composition tests.
func createMovement(t *testing.T, mux *http.ServeMux, name, kind string) int64 {
	t.Helper()
	rr := postJSON(t, mux, "/api/v1/movements", movementPayload(name, kind))
	require.Equal(t, http.StatusCreated, rr.Code)
	return decodeMovement(t, rr.Body).ID
}

func TestWorkout_CreateWithMovements_AndGet(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	squat := createMovement(t, mux, "Back Squat", "exercise")
	row := createMovement(t, mux, "Barbell Row", "exercise")

	theme := "lower + pull"
	body := map[string]any{
		"name":     "Day A",
		"theme":    theme,
		"tags":     []string{"strength"},
		"favorite": true,
		"movements": []map[string]any{
			{"movement_id": squat, "sets": 5, "reps": "5", "load": "80% 1RM"},
			{"movement_id": row, "sets": 3, "reps": "8–10"},
		},
	}
	rr := postJSON(t, mux, "/api/v1/workouts", body)
	require.Equal(t, http.StatusCreated, rr.Code)

	created := decodeWorkout(t, rr.Body)
	assert.Equal(t, "Day A", created.Name)
	require.NotNil(t, created.Theme)
	assert.Equal(t, theme, *created.Theme)
	assert.True(t, created.Favorite)
	require.Len(t, created.Movements, 2)
	// Ordered by position, and the movement name/kind are embedded for render.
	assert.Equal(t, 1, created.Movements[0].Position)
	assert.Equal(t, "Back Squat", created.Movements[0].MovementName)
	assert.Equal(t, "exercise", created.Movements[0].MovementKind)
	require.NotNil(t, created.Movements[0].Sets)
	assert.Equal(t, 5, *created.Movements[0].Sets)
	assert.Equal(t, 2, created.Movements[1].Position)
	assert.Equal(t, "Barbell Row", created.Movements[1].MovementName)

	got := getJSON(t, mux, "/api/v1/workouts/"+itoa(created.ID))
	require.Equal(t, http.StatusOK, got.Code)
	assert.Len(t, decodeWorkout(t, got.Body).Movements, 2)
}

func TestWorkout_List_Filters(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/workouts", map[string]any{
		"name": "Push Day", "theme": "push", "tags": []string{"upper"}, "favorite": true,
	}).Code)
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/workouts", map[string]any{
		"name": "Leg Day", "theme": "legs", "tags": []string{"lower"},
	}).Code)

	list := func(query string) []models.Workout {
		rr := getJSON(t, mux, "/api/v1/workouts"+query)
		require.Equal(t, http.StatusOK, rr.Code)
		var out []models.Workout
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
		return out
	}

	assert.Len(t, list(""), 2)
	assert.Len(t, list("?theme=push"), 1)
	assert.Equal(t, "Push Day", list("?theme=push")[0].Name)
	assert.Len(t, list("?favorite=true"), 1)
	assert.Len(t, list("?favorite=false"), 1)
	assert.Len(t, list("?tag=lower"), 1)
	assert.Len(t, list("?search=leg"), 1)
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/workouts?favorite=maybe").Code)
}

func TestWorkout_Update_Partial(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	created := decodeWorkout(t, postJSON(t, mux, "/api/v1/workouts", map[string]any{"name": "Draft"}).Body)

	rr := putJSON(t, mux, "/api/v1/workouts/"+itoa(created.ID), map[string]any{
		"favorite": true, "theme": "conditioning",
	})
	require.Equal(t, http.StatusOK, rr.Code)
	updated := decodeWorkout(t, rr.Body)
	assert.True(t, updated.Favorite)
	require.NotNil(t, updated.Theme)
	assert.Equal(t, "conditioning", *updated.Theme)
	assert.Equal(t, "Draft", updated.Name) // unchanged
}

func TestWorkout_ComposeMovements_AddSwapReorderRemove(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	press := createMovement(t, mux, "Overhead Press", "exercise")
	pushup := createMovement(t, mux, "Push-up", "exercise")
	alt := createMovement(t, mux, "Landmine Press", "exercise")

	w := decodeWorkout(t, postJSON(t, mux, "/api/v1/workouts", map[string]any{"name": "Upper"}).Body)
	base := "/api/v1/workouts/" + itoa(w.ID) + "/movements"

	// Add two movements — each POST returns the refreshed workout.
	w = decodeWorkout(t, postJSON(t, mux, base, map[string]any{"movement_id": press, "sets": 5, "reps": "5", "load": "95lb"}).Body)
	w = decodeWorkout(t, postJSON(t, mux, base, map[string]any{"movement_id": pushup, "sets": 3, "reps": "AMRAP"}).Body)
	require.Len(t, w.Movements, 2)
	pressEntry := w.Movements[0].ID
	pushupEntry := w.Movements[1].ID
	assert.Equal(t, press, w.Movements[0].MovementID)

	// Swap the first entry for its alternate — prescription carries over.
	rr := patchJSON(t, mux, base+"/"+itoa(pressEntry), map[string]any{"movement_id": alt})
	require.Equal(t, http.StatusOK, rr.Code)
	w = decodeWorkout(t, rr.Body)
	assert.Equal(t, alt, w.Movements[0].MovementID)
	assert.Equal(t, "Landmine Press", w.Movements[0].MovementName)
	require.NotNil(t, w.Movements[0].Load)
	assert.Equal(t, "95lb", *w.Movements[0].Load) // target carried to the swap

	// Reorder: pushup first, then the (swapped) press entry.
	rr = patchJSON(t, mux, base, map[string]any{"entry_ids": []int64{pushupEntry, pressEntry}})
	require.Equal(t, http.StatusOK, rr.Code)
	w = decodeWorkout(t, rr.Body)
	assert.Equal(t, pushupEntry, w.Movements[0].ID)
	assert.Equal(t, 1, w.Movements[0].Position)
	assert.Equal(t, pressEntry, w.Movements[1].ID)
	assert.Equal(t, 2, w.Movements[1].Position)

	// A reorder that doesn't cover every entry is a 400.
	assert.Equal(t, http.StatusBadRequest,
		patchJSON(t, mux, base, map[string]any{"entry_ids": []int64{pushupEntry}}).Code)

	// Remove one entry.
	rr = deleteReq(t, mux, base+"/"+itoa(pushupEntry))
	require.Equal(t, http.StatusOK, rr.Code)
	w = decodeWorkout(t, rr.Body)
	require.Len(t, w.Movements, 1)
	assert.Equal(t, pressEntry, w.Movements[0].ID)

	// Removing an unknown entry is a 404.
	assert.Equal(t, http.StatusNotFound, deleteReq(t, mux, base+"/999999").Code)
}

func TestWorkout_Validation(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	// Duplicate name -> 409.
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/workouts", map[string]any{"name": "Dup"}).Code)
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/workouts", map[string]any{"name": "Dup"}).Code)

	// Missing name -> 400.
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, "/api/v1/workouts", map[string]any{"theme": "x"}).Code)

	// Unknown movement in the composition (FK) -> 409.
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/workouts", map[string]any{
		"name": "Bad", "movements": []map[string]any{{"movement_id": 999999}},
	}).Code)

	// Adding an unknown movement to an existing workout -> 409.
	w := decodeWorkout(t, postJSON(t, mux, "/api/v1/workouts", map[string]any{"name": "Real"}).Body)
	assert.Equal(t, http.StatusConflict,
		postJSON(t, mux, "/api/v1/workouts/"+itoa(w.ID)+"/movements", map[string]any{"movement_id": 999999}).Code)
	// Missing movement_id on add -> 400.
	assert.Equal(t, http.StatusBadRequest,
		postJSON(t, mux, "/api/v1/workouts/"+itoa(w.ID)+"/movements", map[string]any{"sets": 3}).Code)
}

func TestWorkout_Delete(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	press := createMovement(t, mux, "Bench Press", "exercise")
	w := decodeWorkout(t, postJSON(t, mux, "/api/v1/workouts", map[string]any{
		"name": "Temp", "movements": []map[string]any{{"movement_id": press}},
	}).Body)

	// A movement referenced by a workout can't be deleted (FK RESTRICT) -> 409.
	assert.Equal(t, http.StatusConflict, deleteReq(t, mux, "/api/v1/movements/"+itoa(press)).Code)

	// Deleting the workout cascades its entries and frees the movement.
	assert.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/workouts/"+itoa(w.ID)).Code)
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/workouts/"+itoa(w.ID)).Code)
	assert.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/movements/"+itoa(press)).Code)
}
