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

// Previous actuals are the number to beat on the logging screen: the most recent
// *performed* result for that movement, strictly before the session being viewed.
func TestSession_PreviousActuals(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	squat := createMovement(t, mux, "Back Squat", "exercise")
	fresh := createMovement(t, mux, "Nordic Curl", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Leg Day", []map[string]any{
		{"movement_id": squat, "sets": 5, "reps": "5", "load": "185lb"},
		{"movement_id": fresh, "sets": 3, "reps": "6"},
	})

	// logSquat starts a session on a date and optionally records a performed squat.
	logSquat := func(date, load string, done bool) models.WorkoutSession {
		s := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{
			"workout_id": workout.ID, "performed_on": date,
		}).Body)
		rr := patchJSON(t, mux, "/api/v1/sessions/"+s.ID.String()+"/movements/"+itoa(s.Movements[0].ID),
			map[string]any{"done": done, "actual_load": load})
		require.Equal(t, http.StatusOK, rr.Code)
		return s
	}

	earliest := logSquat("2026-07-01", "175lb", true)
	logSquat("2026-07-08", "185lb", true)
	// Opened, never performed — its seeded prescription must not count as a result.
	logSquat("2026-07-12", "999lb", false)
	today := logSquat("2026-07-15", "190lb", true)

	detail := decodeSession(t, getJSON(t, mux, "/api/v1/sessions/"+today.ID.String()).Body)

	// The most recent performed session wins, and the abandoned 07-12 one is skipped.
	require.NotNil(t, detail.Movements[0].Previous)
	assert.Equal(t, "2026-07-08", detail.Movements[0].Previous.PerformedOn)
	require.NotNil(t, detail.Movements[0].Previous.ActualLoad)
	assert.Equal(t, "185lb", *detail.Movements[0].Previous.ActualLoad)
	require.NotNil(t, detail.Movements[0].Previous.ActualSets)
	assert.Equal(t, 5, *detail.Movements[0].Previous.ActualSets)

	// A movement never performed has no previous.
	assert.Nil(t, detail.Movements[1].Previous)

	// A session is never its own previous, so the first one ever logged has none.
	first := decodeSession(t, getJSON(t, mux, "/api/v1/sessions/"+earliest.ID.String()).Body)
	assert.Nil(t, first.Movements[0].Previous)

	// The list endpoint stays lean — previous is detail-only.
	var list []models.WorkoutSession
	require.NoError(t, json.Unmarshal(getJSON(t, mux, "/api/v1/sessions").Body.Bytes(), &list))
	require.NotEmpty(t, list)
	assert.Nil(t, list[0].Movements[0].Previous)
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

// An ad-hoc session starts empty and is built up as the workout happens — the path
// that exists so an unplanned gym session can be recorded without authoring a template
// first.
func TestSession_AdHoc_GrowsThroughAddAndRemove(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	pulldown := createMovement(t, mux, "Lat Pulldown", "exercise")
	facePull := createMovement(t, mux, "Face Pull", "exercise")
	mistake := createMovement(t, mux, "Toe Yoga", "exercise")

	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"performed_on": "2026-08-04"}).Body)
	require.Empty(t, session.Movements)
	base := "/api/v1/sessions/" + session.ID.String() + "/movements"

	rr := postJSON(t, mux, base, map[string]any{
		"movement_id": pulldown, "done": true, "actual_sets": 3, "actual_reps": "12", "actual_load": "60lb",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	grown := decodeSession(t, rr.Body)
	require.Len(t, grown.Movements, 1)
	assert.Equal(t, "Lat Pulldown", grown.Movements[0].MovementName)
	assert.Equal(t, 1, grown.Movements[0].Position)
	assert.True(t, grown.Movements[0].Done)

	require.Equal(t, http.StatusCreated, postJSON(t, mux, base, map[string]any{"movement_id": mistake}).Code)
	rr = postJSON(t, mux, base, map[string]any{"movement_id": facePull, "actual_reps": "15"})
	require.Equal(t, http.StatusCreated, rr.Code)
	grown = decodeSession(t, rr.Body)

	// Appended in the order performed, each after every existing entry.
	require.Len(t, grown.Movements, 3)
	assert.Equal(t, []int{1, 2, 3}, []int{grown.Movements[0].Position, grown.Movements[1].Position, grown.Movements[2].Position})
	assert.Equal(t, "Face Pull", grown.Movements[2].MovementName)

	// Drop the one added by mistake; the others are untouched and keep their order.
	rr = deleteReq(t, mux, base+"/"+itoa(grown.Movements[1].ID))
	require.Equal(t, http.StatusOK, rr.Code)
	trimmed := decodeSession(t, rr.Body)
	require.Len(t, trimmed.Movements, 2)
	assert.Equal(t, "Lat Pulldown", trimmed.Movements[0].MovementName)
	assert.Equal(t, "Face Pull", trimmed.Movements[1].MovementName)

	// Unknown movement (FK) -> 409; missing movement_id -> 400; absent entry -> 404.
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, base, map[string]any{"movement_id": 999999}).Code)
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, base, map[string]any{"actual_reps": "10"}).Code)
	assert.Equal(t, http.StatusNotFound, deleteReq(t, mux, base+"/999999").Code)
	assert.Equal(t, http.StatusNotFound, postJSON(t,
		mux, "/api/v1/sessions/00000000-0000-0000-0000-000000000000/movements", map[string]any{"movement_id": facePull}).Code)
}

// A movement added mid-session arrives with its previous actuals, same as one copied
// from a template — the number to beat is a property of the movement's history, not of
// how the entry got into the session.
func TestSession_AddMovement_CarriesPreviousActuals(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	pulldown := createMovement(t, mux, "Lat Pulldown", "exercise")

	earlier := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"performed_on": "2026-07-28"}).Body)
	rr := postJSON(t, mux, "/api/v1/sessions/"+earlier.ID.String()+"/movements", map[string]any{
		"movement_id": pulldown, "done": true, "actual_sets": 3, "actual_load": "55lb",
	})
	require.Equal(t, http.StatusCreated, rr.Code)

	later := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"performed_on": "2026-08-04"}).Body)
	rr = postJSON(t, mux, "/api/v1/sessions/"+later.ID.String()+"/movements", map[string]any{"movement_id": pulldown})
	require.Equal(t, http.StatusCreated, rr.Code)

	added := decodeSession(t, rr.Body).Movements[0]
	require.NotNil(t, added.Previous)
	assert.Equal(t, "2026-07-28", added.Previous.PerformedOn)
	require.NotNil(t, added.Previous.ActualLoad)
	assert.Equal(t, "55lb", *added.Previous.ActualLoad)
}

// Promotion is the point of the ad-hoc path: what got performed becomes the template
// for next time, with the logged actuals as the prescription.
func TestSession_PromoteToWorkout(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	pulldown := createMovement(t, mux, "Lat Pulldown", "exercise")
	facePull := createMovement(t, mux, "Face Pull", "exercise")

	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"performed_on": "2026-08-04"}).Body)
	base := "/api/v1/sessions/" + session.ID.String()
	require.Equal(t, http.StatusCreated, postJSON(t, mux, base+"/movements", map[string]any{
		"movement_id": pulldown, "actual_sets": 3, "actual_reps": "12", "actual_load": "60lb", "notes": "wide grip",
	}).Code)
	require.Equal(t, http.StatusCreated, postJSON(t, mux, base+"/movements", map[string]any{
		"movement_id": facePull, "actual_sets": 3, "actual_reps": "15",
	}).Code)

	rr := postJSON(t, mux, base+"/workout", map[string]any{
		"name": "Ad-hoc pull", "theme": "pull", "tags": []string{"upper"},
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	workout := decodeWorkout(t, rr.Body)

	assert.Equal(t, "Ad-hoc pull", workout.Name)
	require.NotNil(t, workout.Theme)
	assert.Equal(t, "pull", *workout.Theme)
	assert.Equal(t, []string{"upper"}, workout.Tags)

	// The actuals land as the prescription, in the order performed.
	require.Len(t, workout.Movements, 2)
	assert.Equal(t, "Lat Pulldown", workout.Movements[0].MovementName)
	require.NotNil(t, workout.Movements[0].Sets)
	assert.Equal(t, 3, *workout.Movements[0].Sets)
	require.NotNil(t, workout.Movements[0].Load)
	assert.Equal(t, "60lb", *workout.Movements[0].Load)
	assert.Equal(t, "wide grip", workout.Movements[0].Notes)
	assert.Equal(t, "Face Pull", workout.Movements[1].MovementName)
	assert.Nil(t, workout.Movements[1].Load)

	// The session is back-linked, so it reads as the first instance of what it produced.
	linked := decodeSession(t, getJSON(t, mux, base).Body)
	require.NotNil(t, linked.WorkoutID)
	assert.Equal(t, workout.ID, *linked.WorkoutID)
	require.NotNil(t, linked.WorkoutName)
	assert.Equal(t, "Ad-hoc pull", *linked.WorkoutName)

	// And it now counts as that workout's history.
	var byWorkout []models.WorkoutSession
	require.NoError(t, json.Unmarshal(getJSON(t, mux, "/api/v1/sessions?workout_id="+itoa(workout.ID)).Body.Bytes(), &byWorkout))
	assert.Len(t, byWorkout, 1)
}

func TestSession_Promote_Rejections(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	squat := createMovement(t, mux, "Back Squat", "exercise")
	template := createWorkoutWithMovements(t, mux, "Leg Day", []map[string]any{{"movement_id": squat}})

	// A session already backed by a template would silently fork it -> 409.
	fromTemplate := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": template.ID}).Body)
	assert.Equal(t, http.StatusConflict,
		postJSON(t, mux, "/api/v1/sessions/"+fromTemplate.ID.String()+"/workout", map[string]any{"name": "Forked"}).Code)

	adhoc := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{}).Body)
	base := "/api/v1/sessions/" + adhoc.ID.String() + "/workout"

	// Nothing performed -> 400; no name -> 400.
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, base, map[string]any{"name": "Empty"}).Code)
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, base, map[string]any{}).Code)

	// A name already taken by another workout -> 409 (workouts.name is the natural key).
	require.Equal(t, http.StatusCreated,
		postJSON(t, mux, "/api/v1/sessions/"+adhoc.ID.String()+"/movements", map[string]any{"movement_id": squat}).Code)
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, base, map[string]any{"name": "Leg Day"}).Code)

	// The failed promotion left nothing behind — the session is still ad-hoc.
	assert.Nil(t, decodeSession(t, getJSON(t, mux, "/api/v1/sessions/"+adhoc.ID.String()).Body).WorkoutID)

	assert.Equal(t, http.StatusNotFound,
		postJSON(t, mux, "/api/v1/sessions/00000000-0000-0000-0000-000000000000/workout", map[string]any{"name": "Ghost"}).Code)
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
