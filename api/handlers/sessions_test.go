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

func decodeSession(t *testing.T, rr interface{ Bytes() []byte }) models.WorkoutSession {
	t.Helper()
	var s models.WorkoutSession
	require.NoError(t, json.Unmarshal(rr.Bytes(), &s))
	return s
}

// createWorkoutWithMovements builds a workout carrying a prescription, returning it
// so its id and entries anchor the session tests.
func createWorkoutWithMovements(t *testing.T, mux *http.ServeMux, name string, movements []map[string]any) models.Workout {
	t.Helper()
	rr := postJSON(t, mux, "/api/v1/workouts", map[string]any{"name": name, "movements": movements})
	require.Equal(t, http.StatusCreated, rr.Code)
	return decodeWorkout(t, rr.Body)
}

func TestSession_LogFromWorkout_CopiesMovements_SeedingActuals(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	squat := createMovement(t, mux, "Back Squat", "exercise")
	row := createMovement(t, mux, "Barbell Row", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Day A", []map[string]any{
		{"movement_id": squat, "sets": 5, "reps": "5", "load": "80% 1RM"},
		{"movement_id": row, "sets": 3, "reps": "8–10", "load": "2 plates"},
	})

	rr := postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": workout.ID})
	require.Equal(t, http.StatusCreated, rr.Code)
	session := decodeSession(t, rr.Body)

	require.NotNil(t, session.WorkoutID)
	assert.Equal(t, workout.ID, *session.WorkoutID)
	require.NotNil(t, session.WorkoutName)
	assert.Equal(t, "Day A", *session.WorkoutName)
	// performed_on defaults to today when omitted.
	assert.Equal(t, time.Now().Format("2006-01-02"), session.PerformedOn)

	// The template's movements are copied in order, with the prescription seeded
	// into actual_* as a starting point and done=false.
	require.Len(t, session.Movements, 2)
	assert.Equal(t, 1, session.Movements[0].Position)
	assert.Equal(t, "Back Squat", session.Movements[0].MovementName)
	assert.False(t, session.Movements[0].Done)
	require.NotNil(t, session.Movements[0].ActualSets)
	assert.Equal(t, 5, *session.Movements[0].ActualSets)
	require.NotNil(t, session.Movements[0].ActualLoad)
	assert.Equal(t, "80% 1RM", *session.Movements[0].ActualLoad)
	assert.Equal(t, "Barbell Row", session.Movements[1].MovementName)

	// A fresh UUID7 id round-trips through the DB and the GET path.
	got := getJSON(t, mux, "/api/v1/sessions/"+session.ID.String())
	require.Equal(t, http.StatusOK, got.Code)
	assert.Len(t, decodeSession(t, got.Body).Movements, 2)
}

func TestSession_AdHoc_WithSuppliedMovements(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	stretch := createMovement(t, mux, "Couch Stretch", "stretch")

	rr := postJSON(t, mux, "/api/v1/sessions", map[string]any{
		"performed_on":  "2026-07-20",
		"felt":          "loose",
		"overall_notes": "quick mobility flush",
		"movements": []map[string]any{
			{"movement_id": stretch, "done": true, "actual_reps": "60s"},
		},
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	session := decodeSession(t, rr.Body)

	assert.Nil(t, session.WorkoutID) // ad-hoc — no template
	assert.Nil(t, session.WorkoutName)
	assert.Equal(t, "2026-07-20", session.PerformedOn)
	require.NotNil(t, session.Felt)
	assert.Equal(t, "loose", *session.Felt)
	require.Len(t, session.Movements, 1)
	assert.True(t, session.Movements[0].Done)
	require.NotNil(t, session.Movements[0].ActualReps)
	assert.Equal(t, "60s", *session.Movements[0].ActualReps)
}

func TestSession_List_Filters(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	squat := createMovement(t, mux, "Back Squat", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Leg Day", []map[string]any{{"movement_id": squat}})

	mkSession := func(body map[string]any) {
		require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/sessions", body).Code)
	}
	mkSession(map[string]any{"workout_id": workout.ID, "performed_on": "2026-07-01"})
	mkSession(map[string]any{"workout_id": workout.ID, "performed_on": "2026-07-15"})
	mkSession(map[string]any{"performed_on": "2026-06-01"}) // ad-hoc, earlier

	list := func(query string) []models.WorkoutSession {
		rr := getJSON(t, mux, "/api/v1/sessions"+query)
		require.Equal(t, http.StatusOK, rr.Code)
		var out []models.WorkoutSession
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
		return out
	}

	all := list("")
	require.Len(t, all, 3)
	// Newest first.
	assert.Equal(t, "2026-07-15", all[0].PerformedOn)
	assert.Len(t, list("?from=2026-07-01"), 2)
	assert.Len(t, list("?to=2026-06-30"), 1)
	assert.Len(t, list("?from=2026-07-01&to=2026-07-10"), 1)
	assert.Len(t, list("?workout_id="+itoa(workout.ID)), 2)

	// Malformed filters -> 400.
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/sessions?from=notadate").Code)
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/sessions?workout_id=abc").Code)
}

func TestSession_UpdateMovement_DoneActualsAndSwap(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	press := createMovement(t, mux, "Overhead Press", "exercise")
	alt := createMovement(t, mux, "Landmine Press", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Upper", []map[string]any{
		{"movement_id": press, "sets": 5, "reps": "5", "load": "95lb"},
	})
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": workout.ID}).Body)
	entry := session.Movements[0].ID
	base := "/api/v1/sessions/" + session.ID.String() + "/movements/" + itoa(entry)

	// Check off the set and record a real load beating the plan.
	rr := patchJSON(t, mux, base, map[string]any{"done": true, "actual_load": "100lb", "notes": "felt strong"})
	require.Equal(t, http.StatusOK, rr.Code)
	updated := decodeSession(t, rr.Body)
	assert.True(t, updated.Movements[0].Done)
	require.NotNil(t, updated.Movements[0].ActualLoad)
	assert.Equal(t, "100lb", *updated.Movements[0].ActualLoad)
	assert.Equal(t, "felt strong", updated.Movements[0].Notes)
	// Untouched actual carried from the prescription seed remains.
	require.NotNil(t, updated.Movements[0].ActualSets)
	assert.Equal(t, 5, *updated.Movements[0].ActualSets)

	// Mid-session swap to the alternate — the recorded actuals carry over.
	rr = patchJSON(t, mux, base, map[string]any{"movement_id": alt})
	require.Equal(t, http.StatusOK, rr.Code)
	swapped := decodeSession(t, rr.Body)
	assert.Equal(t, alt, swapped.Movements[0].MovementID)
	assert.Equal(t, "Landmine Press", swapped.Movements[0].MovementName)
	require.NotNil(t, swapped.Movements[0].ActualLoad)
	assert.Equal(t, "100lb", *swapped.Movements[0].ActualLoad) // carried to the swap
	assert.True(t, swapped.Movements[0].Done)

	// An entry id from no session is a 404.
	assert.Equal(t, http.StatusNotFound,
		patchJSON(t, mux, "/api/v1/sessions/"+session.ID.String()+"/movements/999999", map[string]any{"done": true}).Code)
}

func TestSession_Update_Partial(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"performed_on": "2026-07-10"}).Body)

	rr := putJSON(t, mux, "/api/v1/sessions/"+session.ID.String(), map[string]any{
		"felt": "tired", "duration_minutes": 52, "overall_notes": "grind",
	})
	require.Equal(t, http.StatusOK, rr.Code)
	updated := decodeSession(t, rr.Body)
	require.NotNil(t, updated.Felt)
	assert.Equal(t, "tired", *updated.Felt)
	require.NotNil(t, updated.DurationMinutes)
	assert.Equal(t, 52, *updated.DurationMinutes)
	assert.Equal(t, "2026-07-10", updated.PerformedOn) // unchanged

	// A bad date on update -> 400.
	assert.Equal(t, http.StatusBadRequest,
		putJSON(t, mux, "/api/v1/sessions/"+session.ID.String(), map[string]any{"performed_on": "07/10/2026"}).Code)
}

func TestSession_DeletePreservesMovementsAndFreesTemplate(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	squat := createMovement(t, mux, "Back Squat", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Legs", []map[string]any{{"movement_id": squat}})
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": workout.ID}).Body)

	// A movement referenced by a logged session can't be deleted (FK RESTRICT) -> 409.
	assert.Equal(t, http.StatusConflict, deleteReq(t, mux, "/api/v1/movements/"+itoa(squat)).Code)

	// Deleting the workout template SET-NULLs the session's workout_id but keeps the
	// logged history — the instance data survives its template.
	require.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/workouts/"+itoa(workout.ID)).Code)
	after := decodeSession(t, getJSON(t, mux, "/api/v1/sessions/"+session.ID.String()).Body)
	assert.Nil(t, after.WorkoutID)
	assert.Nil(t, after.WorkoutName)
	require.Len(t, after.Movements, 1) // the logged movement is preserved

	// Deleting the session cascades its entries and frees the movement.
	require.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/sessions/"+session.ID.String()).Code)
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/sessions/"+session.ID.String()).Code)
	assert.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/movements/"+itoa(squat)).Code)
}

func TestSession_Validation(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	// Unknown workout_id (FK) -> 409.
	assert.Equal(t, http.StatusConflict,
		postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": 999999}).Code)
	// Unknown movement in an ad-hoc session (FK) -> 409.
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/sessions", map[string]any{
		"movements": []map[string]any{{"movement_id": 999999}},
	}).Code)
	// Bad performed_on on create -> 400.
	assert.Equal(t, http.StatusBadRequest,
		postJSON(t, mux, "/api/v1/sessions", map[string]any{"performed_on": "yesterday"}).Code)
	// Malformed session uuid -> 400.
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/sessions/not-a-uuid").Code)
	// A well-formed but absent uuid -> 404.
	assert.Equal(t, http.StatusNotFound,
		getJSON(t, mux, "/api/v1/sessions/00000000-0000-0000-0000-000000000000").Code)
}
