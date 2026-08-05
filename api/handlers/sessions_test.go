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

// setsPath addresses the set collection under one entry of one session.
func setsPath(session models.WorkoutSession, entryID int64) string {
	return "/api/v1/sessions/" + session.ID.String() + "/movements/" + itoa(entryID) + "/sets"
}

// logSet posts one set and returns the refreshed session. An empty body is the
// carry-forward path.
func logSet(t *testing.T, mux *http.ServeMux, session models.WorkoutSession, entryID int64, body map[string]any) models.WorkoutSession {
	t.Helper()
	rr := postJSON(t, mux, setsPath(session, entryID), body)
	require.Equal(t, http.StatusCreated, rr.Code)
	return decodeSession(t, rr.Body)
}

func TestSession_LogFromWorkout_CopiesTargetsWithNothingPerformed(t *testing.T) {
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
	// performed_on defaults to today when omitted, and a new session is not finished.
	assert.Equal(t, time.Now().Format("2006-01-02"), session.PerformedOn)
	assert.Nil(t, session.FinishedAt)

	// The template's movements are copied in order as the target. Crucially, nothing
	// is performed yet: the plan is visible but no set is claimed.
	require.Len(t, session.Movements, 2)
	assert.Equal(t, 1, session.Movements[0].Position)
	assert.Equal(t, "Back Squat", session.Movements[0].MovementName)
	assert.False(t, session.Movements[0].Done)
	assert.Empty(t, session.Movements[0].Sets)
	require.NotNil(t, session.Movements[0].TargetSets)
	assert.Equal(t, 5, *session.Movements[0].TargetSets)
	require.NotNil(t, session.Movements[0].TargetLoad)
	assert.Equal(t, "80% 1RM", *session.Movements[0].TargetLoad)
	assert.Equal(t, "Barbell Row", session.Movements[1].MovementName)

	// A fresh UUID7 id round-trips through the DB and the GET path.
	got := getJSON(t, mux, "/api/v1/sessions/"+session.ID.String())
	require.Equal(t, http.StatusOK, got.Code)
	assert.Len(t, decodeSession(t, got.Body).Movements, 2)
}

func TestSession_FreeForm_WithSuppliedMovements(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	stretch := createMovement(t, mux, "Couch Stretch", "stretch")

	rr := postJSON(t, mux, "/api/v1/sessions", map[string]any{
		"performed_on":  "2026-07-20",
		"felt":          "loose",
		"overall_notes": "quick mobility flush",
		"movements": []map[string]any{
			{"movement_id": stretch, "done": true, "target_reps": "60s"},
		},
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	session := decodeSession(t, rr.Body)

	assert.Nil(t, session.WorkoutID) // free-form — no template
	assert.Nil(t, session.WorkoutName)
	assert.Equal(t, "2026-07-20", session.PerformedOn)
	require.NotNil(t, session.Felt)
	assert.Equal(t, "loose", *session.Felt)
	require.Len(t, session.Movements, 1)
	assert.True(t, session.Movements[0].Done)
	require.NotNil(t, session.Movements[0].TargetReps)
	assert.Equal(t, "60s", *session.Movements[0].TargetReps)

	// A stretch is timed, and the entry carries that through so the logging screen
	// knows not to ask for a weight.
	assert.Equal(t, "timed", session.Movements[0].LoadMode)
}

func TestSession_List_Filters(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	squat := createMovement(t, mux, "Back Squat", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Leg Day", []map[string]any{{"movement_id": squat}})

	mkSession := func(body map[string]any) models.WorkoutSession {
		rr := postJSON(t, mux, "/api/v1/sessions", body)
		require.Equal(t, http.StatusCreated, rr.Code)
		return decodeSession(t, rr.Body)
	}
	mkSession(map[string]any{"workout_id": workout.ID, "performed_on": "2026-07-01"})
	finished := mkSession(map[string]any{"workout_id": workout.ID, "performed_on": "2026-07-15"})
	mkSession(map[string]any{"performed_on": "2026-06-01"}) // free-form, earlier

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

	// unfinished is what the app asks for to find the session worth resuming.
	assert.Len(t, list("?unfinished"), 3)
	require.Equal(t, http.StatusOK, postJSON(t, mux, "/api/v1/sessions/"+finished.ID.String()+"/finish", nil).Code)
	assert.Len(t, list("?unfinished"), 2)

	// Malformed filters -> 400.
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/sessions?from=notadate").Code)
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/sessions?workout_id=abc").Code)
}

// Logging a set is the most-tapped write in the app, so a bare POST has to mean
// "another one like the last".
func TestSession_AddSet_CarriesForwardAndTicksDone(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	press := createMovement(t, mux, "Overhead Press", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Upper", []map[string]any{
		{"movement_id": press, "sets": 3, "reps": "8", "load": "95lb"},
	})
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": workout.ID}).Body)
	entry := session.Movements[0].ID

	// The first set falls back to the target, since there is no previous set to copy.
	after := logSet(t, mux, session, entry, map[string]any{})
	require.Len(t, after.Movements[0].Sets, 1)
	first := after.Movements[0].Sets[0]
	assert.Equal(t, 1, first.Position)
	require.NotNil(t, first.Reps)
	assert.Equal(t, 8, *first.Reps)
	require.NotNil(t, first.Load)
	assert.Equal(t, "95lb", *first.Load)
	assert.Equal(t, "working", first.SetKind)
	assert.False(t, after.Movements[0].Done, "one of three sets is not done")

	// The second carries the first forward untouched.
	after = logSet(t, mux, session, entry, map[string]any{})
	require.Len(t, after.Movements[0].Sets, 2)
	assert.Equal(t, 2, after.Movements[0].Sets[1].Position)
	require.NotNil(t, after.Movements[0].Sets[1].Load)
	assert.Equal(t, "95lb", *after.Movements[0].Sets[1].Load)

	// The third drops the weight and hits the target, which ticks the entry off with
	// nobody having to count.
	after = logSet(t, mux, session, entry, map[string]any{"load": "85lb", "reps": 6, "set_kind": "drop"})
	require.Len(t, after.Movements[0].Sets, 3)
	last := after.Movements[0].Sets[2]
	require.NotNil(t, last.Load)
	assert.Equal(t, "85lb", *last.Load)
	require.NotNil(t, last.Reps)
	assert.Equal(t, 6, *last.Reps)
	assert.Equal(t, "drop", last.SetKind)
	assert.True(t, after.Movements[0].Done, "reaching the target ticks the entry off")

	// A fourth set is a fact, not an error: doing more than the plan is recordable.
	after = logSet(t, mux, session, entry, map[string]any{})
	require.Len(t, after.Movements[0].Sets, 4)
	require.NotNil(t, after.Movements[0].Sets[3].Load)
	assert.Equal(t, "85lb", *after.Movements[0].Sets[3].Load, "carries the drop set, not the target")
	// The target is untouched by any of it — the plan stays readable.
	require.NotNil(t, after.Movements[0].TargetSets)
	assert.Equal(t, 3, *after.Movements[0].TargetSets)
}

// An entry with no target is done as soon as anything is logged against it: nothing was
// planned, so doing it at all is the whole of it.
func TestSession_AddSet_UntargetedEntryIsDoneOnFirstSet(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	pulldown := createMovement(t, mux, "Lat Pulldown", "exercise")
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{}).Body)
	added := decodeSession(t, postJSON(t, mux,
		"/api/v1/sessions/"+session.ID.String()+"/movements", map[string]any{"movement_id": pulldown}).Body)

	after := logSet(t, mux, session, added.Movements[0].ID, map[string]any{"reps": 12, "load": "60lb"})
	assert.True(t, after.Movements[0].Done)
}

// Unchecking by hand has to survive the next set, or "I stopped here on purpose" is not
// expressible.
func TestSession_ManualDoneOverrideSurvivesLaterSets(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	curl := createMovement(t, mux, "Nordic Curl", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Posterior", []map[string]any{
		{"movement_id": curl, "sets": 2},
	})
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": workout.ID}).Body)
	entry := session.Movements[0].ID
	entryPath := "/api/v1/sessions/" + session.ID.String() + "/movements/" + itoa(entry)

	logSet(t, mux, session, entry, map[string]any{"reps": 5})
	logSet(t, mux, session, entry, map[string]any{"reps": 4})
	require.True(t, decodeSession(t, getJSON(t, mux, "/api/v1/sessions/"+session.ID.String()).Body).Movements[0].Done)

	// Called it off after the fact.
	require.Equal(t, http.StatusOK, patchJSON(t, mux, entryPath, map[string]any{"done": false}).Code)
	// Logging another set must not silently overrule that.
	after := logSet(t, mux, session, entry, map[string]any{"reps": 3})
	assert.False(t, after.Movements[0].Done)
	assert.Len(t, after.Movements[0].Sets, 3)
}

func TestSession_UpdateAndRemoveSet(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	press := createMovement(t, mux, "Overhead Press", "exercise")
	other := createMovement(t, mux, "Landmine Press", "exercise")
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{}).Body)
	base := "/api/v1/sessions/" + session.ID.String() + "/movements"
	entry := decodeSession(t, postJSON(t, mux, base, map[string]any{"movement_id": press}).Body).Movements[0].ID
	strayEntry := decodeSession(t, postJSON(t, mux, base, map[string]any{"movement_id": other}).Body).Movements[1].ID

	after := logSet(t, mux, session, entry, map[string]any{"reps": 8, "load": "95lb"})
	setID := after.Movements[0].Sets[0].ID
	setPath := setsPath(session, entry) + "/" + itoa(setID)

	rr := patchJSON(t, mux, setPath, map[string]any{"reps": 7, "notes": "grinder"})
	require.Equal(t, http.StatusOK, rr.Code)
	fixed := decodeSession(t, rr.Body).Movements[0].Sets[0]
	require.NotNil(t, fixed.Reps)
	assert.Equal(t, 7, *fixed.Reps)
	assert.Equal(t, "grinder", fixed.Notes)
	require.NotNil(t, fixed.Load)
	assert.Equal(t, "95lb", *fixed.Load, "an unsent field is left alone")

	// A set id belonging to a different entry is a 404, not a cross-entry edit.
	assert.Equal(t, http.StatusNotFound,
		patchJSON(t, mux, setsPath(session, strayEntry)+"/"+itoa(setID), map[string]any{"reps": 1}).Code)
	assert.Equal(t, http.StatusNotFound, deleteReq(t, mux, setsPath(session, strayEntry)+"/"+itoa(setID)).Code)
	assert.Equal(t, http.StatusNotFound, patchJSON(t, mux, setsPath(session, entry)+"/999999", map[string]any{"reps": 1}).Code)
	assert.Equal(t, http.StatusNotFound, postJSON(t, mux, setsPath(session, 999999), map[string]any{}).Code)

	rr = deleteReq(t, mux, setPath)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, decodeSession(t, rr.Body).Movements[0].Sets)
}

// Removing an entry takes its sets with it, and removing a session takes everything.
func TestSession_RemoveMovementCascadesSets(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	pulldown := createMovement(t, mux, "Lat Pulldown", "exercise")
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{}).Body)
	base := "/api/v1/sessions/" + session.ID.String() + "/movements"
	entry := decodeSession(t, postJSON(t, mux, base, map[string]any{"movement_id": pulldown}).Body).Movements[0].ID
	logSet(t, mux, session, entry, map[string]any{"reps": 10})

	rr := deleteReq(t, mux, base+"/"+itoa(entry))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, decodeSession(t, rr.Body).Movements)

	// The movement is free again, which it would not be if a set still referenced it.
	require.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/sessions/"+session.ID.String()).Code)
	assert.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/movements/"+itoa(pulldown)).Code)
}

func TestSession_Finish_FillsDurationAndIsIdempotent(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{}).Body)
	require.Nil(t, session.FinishedAt)
	path := "/api/v1/sessions/" + session.ID.String() + "/finish"

	rr := postJSON(t, mux, path, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	finished := decodeSession(t, rr.Body)
	require.NotNil(t, finished.FinishedAt)
	require.NotNil(t, finished.DurationMinutes, "duration is derived, never typed")
	assert.GreaterOrEqual(t, *finished.DurationMinutes, 1)

	// Finishing again must not rewrite when training ended.
	rr = postJSON(t, mux, path, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	again := decodeSession(t, rr.Body)
	require.NotNil(t, again.FinishedAt)
	assert.Equal(t, finished.FinishedAt.UnixNano(), again.FinishedAt.UnixNano())

	assert.Equal(t, http.StatusNotFound,
		postJSON(t, mux, "/api/v1/sessions/00000000-0000-0000-0000-000000000000/finish", nil).Code)
}

// A duration already recorded by hand wins — finishing fills a gap, it does not correct
// someone who took the trouble to say.
func TestSession_Finish_KeepsAnExplicitDuration(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions",
		map[string]any{"duration_minutes": 73}).Body)

	rr := postJSON(t, mux, "/api/v1/sessions/"+session.ID.String()+"/finish", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	finished := decodeSession(t, rr.Body)
	require.NotNil(t, finished.DurationMinutes)
	assert.Equal(t, 73, *finished.DurationMinutes)
}

func TestSession_UpdateMovement_DoneTargetAndSwap(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	press := createMovement(t, mux, "Overhead Press", "exercise")
	alt := createMovement(t, mux, "Landmine Press", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Upper", []map[string]any{
		{"movement_id": press, "sets": 5, "reps": "5", "load": "95lb"},
	})
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": workout.ID}).Body)
	entry := session.Movements[0].ID
	base := "/api/v1/sessions/" + session.ID.String() + "/movements/" + itoa(entry)
	logSet(t, mux, session, entry, map[string]any{"load": "100lb"})

	// The target can be edited for this session without touching the workout.
	rr := patchJSON(t, mux, base, map[string]any{"done": true, "target_load": "105lb", "notes": "felt strong"})
	require.Equal(t, http.StatusOK, rr.Code)
	updated := decodeSession(t, rr.Body)
	assert.True(t, updated.Movements[0].Done)
	require.NotNil(t, updated.Movements[0].TargetLoad)
	assert.Equal(t, "105lb", *updated.Movements[0].TargetLoad)
	assert.Equal(t, "felt strong", updated.Movements[0].Notes)
	require.NotNil(t, updated.Movements[0].TargetSets)
	assert.Equal(t, 5, *updated.Movements[0].TargetSets, "an unsent target field is left alone")

	// The workout it came from is untouched: the session's plan is the session's.
	template := decodeWorkout(t, getJSON(t, mux, "/api/v1/workouts/"+itoa(workout.ID)).Body)
	require.NotNil(t, template.Movements[0].Load)
	assert.Equal(t, "95lb", *template.Movements[0].Load)

	// Mid-session swap to the alternate — the target and the logged sets carry over.
	rr = patchJSON(t, mux, base, map[string]any{"movement_id": alt})
	require.Equal(t, http.StatusOK, rr.Code)
	swapped := decodeSession(t, rr.Body)
	assert.Equal(t, alt, swapped.Movements[0].MovementID)
	assert.Equal(t, "Landmine Press", swapped.Movements[0].MovementName)
	require.Len(t, swapped.Movements[0].Sets, 1)
	require.NotNil(t, swapped.Movements[0].Sets[0].Load)
	assert.Equal(t, "100lb", *swapped.Movements[0].Sets[0].Load)
	assert.True(t, swapped.Movements[0].Done)

	// An entry id from no session is a 404.
	assert.Equal(t, http.StatusNotFound,
		patchJSON(t, mux, "/api/v1/sessions/"+session.ID.String()+"/movements/999999", map[string]any{"done": true}).Code)
}

// Previous actuals are the number to beat on the logging screen: the last set of the
// most recent *performed* entry for that movement, strictly before the session viewed.
func TestSession_PreviousActuals(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	squat := createMovement(t, mux, "Back Squat", "exercise")
	fresh := createMovement(t, mux, "Nordic Curl", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Leg Day", []map[string]any{
		{"movement_id": squat, "sets": 2, "reps": "5", "load": "185lb"},
		{"movement_id": fresh, "sets": 3, "reps": "6"},
	})

	// logSquat starts a session on a date and optionally performs the squat.
	logSquat := func(date, load string, perform bool) models.WorkoutSession {
		s := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{
			"workout_id": workout.ID, "performed_on": date,
		}).Body)
		if perform {
			logSet(t, mux, s, s.Movements[0].ID, map[string]any{"load": load, "reps": 5})
			logSet(t, mux, s, s.Movements[0].ID, map[string]any{})
		}
		return s
	}

	earliest := logSquat("2026-07-01", "175lb", true)
	logSquat("2026-07-08", "185lb", true)
	// Opened and walked away from. Its target must not read as a result.
	logSquat("2026-07-12", "999lb", false)
	today := logSquat("2026-07-15", "190lb", true)

	detail := decodeSession(t, getJSON(t, mux, "/api/v1/sessions/"+today.ID.String()).Body)

	// The most recent performed session wins, and the abandoned 07-12 one is skipped.
	require.NotNil(t, detail.Movements[0].Previous)
	assert.Equal(t, "2026-07-08", detail.Movements[0].Previous.PerformedOn)
	require.NotNil(t, detail.Movements[0].Previous.Load)
	assert.Equal(t, "185lb", *detail.Movements[0].Previous.Load)
	assert.Equal(t, 2, detail.Movements[0].Previous.Sets)
	require.NotNil(t, detail.Movements[0].Previous.Reps)
	assert.Equal(t, 5, *detail.Movements[0].Previous.Reps)

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

// Ticking a box without logging anything is not a number to beat.
func TestSession_PreviousActuals_IgnoresDoneWithNoSets(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	squat := createMovement(t, mux, "Back Squat", "exercise")
	workout := createWorkoutWithMovements(t, mux, "Leg Day", []map[string]any{{"movement_id": squat}})

	earlier := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{
		"workout_id": workout.ID, "performed_on": "2026-07-01",
	}).Body)
	require.Equal(t, http.StatusOK, patchJSON(t, mux,
		"/api/v1/sessions/"+earlier.ID.String()+"/movements/"+itoa(earlier.Movements[0].ID),
		map[string]any{"done": true}).Code)

	later := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{
		"workout_id": workout.ID, "performed_on": "2026-07-08",
	}).Body)
	detail := decodeSession(t, getJSON(t, mux, "/api/v1/sessions/"+later.ID.String()).Body)
	assert.Nil(t, detail.Movements[0].Previous)
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

// A session grows as the workout happens — and it does so whether or not it came from a
// template, because doing something the plan did not call for has to be recordable.
func TestSession_GrowsThroughAddAndRemove_TemplateBackedToo(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	pulldown := createMovement(t, mux, "Lat Pulldown", "exercise")
	facePull := createMovement(t, mux, "Face Pull", "exercise")
	mistake := createMovement(t, mux, "Toe Yoga", "exercise")

	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"performed_on": "2026-08-04"}).Body)
	require.Empty(t, session.Movements)
	base := "/api/v1/sessions/" + session.ID.String() + "/movements"

	rr := postJSON(t, mux, base, map[string]any{
		"movement_id": pulldown, "done": true, "target_sets": 3, "target_reps": "12", "target_load": "60lb",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	grown := decodeSession(t, rr.Body)
	require.Len(t, grown.Movements, 1)
	assert.Equal(t, "Lat Pulldown", grown.Movements[0].MovementName)
	assert.Equal(t, 1, grown.Movements[0].Position)
	assert.True(t, grown.Movements[0].Done)

	require.Equal(t, http.StatusCreated, postJSON(t, mux, base, map[string]any{"movement_id": mistake}).Code)
	rr = postJSON(t, mux, base, map[string]any{"movement_id": facePull, "target_reps": "15"})
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

	// The same works on a session backed by a template: an extra movement, and one
	// skipped, are both part of what happened.
	workout := createWorkoutWithMovements(t, mux, "Pull Day", []map[string]any{{"movement_id": pulldown}})
	planned := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": workout.ID}).Body)
	plannedBase := "/api/v1/sessions/" + planned.ID.String() + "/movements"
	rr = postJSON(t, mux, plannedBase, map[string]any{"movement_id": facePull})
	require.Equal(t, http.StatusCreated, rr.Code)
	extended := decodeSession(t, rr.Body)
	require.Len(t, extended.Movements, 2)
	require.Equal(t, http.StatusOK, deleteReq(t, mux, plannedBase+"/"+itoa(extended.Movements[0].ID)).Code)

	// Unknown movement (FK) -> 409; missing movement_id -> 400; absent entry -> 404.
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, base, map[string]any{"movement_id": 999999}).Code)
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, base, map[string]any{"target_reps": "10"}).Code)
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
	rr := postJSON(t, mux, "/api/v1/sessions/"+earlier.ID.String()+"/movements", map[string]any{"movement_id": pulldown})
	require.Equal(t, http.StatusCreated, rr.Code)
	logSet(t, mux, earlier, decodeSession(t, rr.Body).Movements[0].ID, map[string]any{"load": "55lb", "reps": 10})

	later := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"performed_on": "2026-08-04"}).Body)
	rr = postJSON(t, mux, "/api/v1/sessions/"+later.ID.String()+"/movements", map[string]any{"movement_id": pulldown})
	require.Equal(t, http.StatusCreated, rr.Code)

	added := decodeSession(t, rr.Body).Movements[0]
	require.NotNil(t, added.Previous)
	assert.Equal(t, "2026-07-28", added.Previous.PerformedOn)
	require.NotNil(t, added.Previous.Load)
	assert.Equal(t, "55lb", *added.Previous.Load)
}

// Promotion is the point of the free-form path: what got performed becomes the template
// for next time, with the logged sets as the prescription.
func TestSession_PromoteToWorkout(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	pulldown := createMovement(t, mux, "Lat Pulldown", "exercise")
	facePull := createMovement(t, mux, "Face Pull", "exercise")

	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{"performed_on": "2026-08-04"}).Body)
	base := "/api/v1/sessions/" + session.ID.String()
	rr := postJSON(t, mux, base+"/movements", map[string]any{"movement_id": pulldown, "notes": "wide grip"})
	require.Equal(t, http.StatusCreated, rr.Code)
	pulldownEntry := decodeSession(t, rr.Body).Movements[0].ID
	rr = postJSON(t, mux, base+"/movements", map[string]any{"movement_id": facePull})
	require.Equal(t, http.StatusCreated, rr.Code)
	facePullEntry := decodeSession(t, rr.Body).Movements[1].ID

	// Three sets at 12, with the last one heavier — the prescription should take the
	// reps most of them shared and the load actually finished on.
	logSet(t, mux, session, pulldownEntry, map[string]any{"reps": 12, "load": "60lb"})
	logSet(t, mux, session, pulldownEntry, map[string]any{})
	logSet(t, mux, session, pulldownEntry, map[string]any{"reps": 10, "load": "70lb"})
	logSet(t, mux, session, facePullEntry, map[string]any{"reps": 15})

	rr = postJSON(t, mux, base+"/workout", map[string]any{
		"name": "Free-form pull", "theme": "pull", "tags": []string{"upper"},
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	workout := decodeWorkout(t, rr.Body)

	assert.Equal(t, "Free-form pull", workout.Name)
	require.NotNil(t, workout.Theme)
	assert.Equal(t, "pull", *workout.Theme)
	assert.Equal(t, []string{"upper"}, workout.Tags)

	// What was performed lands as the prescription, in the order performed.
	require.Len(t, workout.Movements, 2)
	assert.Equal(t, "Lat Pulldown", workout.Movements[0].MovementName)
	require.NotNil(t, workout.Movements[0].Sets)
	assert.Equal(t, 3, *workout.Movements[0].Sets)
	require.NotNil(t, workout.Movements[0].Reps)
	assert.Equal(t, "12", *workout.Movements[0].Reps, "the reps most sets shared")
	require.NotNil(t, workout.Movements[0].Load)
	assert.Equal(t, "70lb", *workout.Movements[0].Load, "the load finished on")
	assert.Equal(t, "wide grip", workout.Movements[0].Notes)
	assert.Equal(t, "Face Pull", workout.Movements[1].MovementName)
	assert.Nil(t, workout.Movements[1].Load)

	// The session is back-linked, so it reads as the first instance of what it produced.
	linked := decodeSession(t, getJSON(t, mux, base).Body)
	require.NotNil(t, linked.WorkoutID)
	assert.Equal(t, workout.ID, *linked.WorkoutID)
	require.NotNil(t, linked.WorkoutName)
	assert.Equal(t, "Free-form pull", *linked.WorkoutName)

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

	freeForm := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{}).Body)
	base := "/api/v1/sessions/" + freeForm.ID.String() + "/workout"

	// Nothing performed -> 400; no name -> 400.
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, base, map[string]any{"name": "Empty"}).Code)
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, base, map[string]any{}).Code)

	// A name already taken by another workout -> 409 (workouts.name is the natural key).
	require.Equal(t, http.StatusCreated,
		postJSON(t, mux, "/api/v1/sessions/"+freeForm.ID.String()+"/movements", map[string]any{"movement_id": squat}).Code)
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, base, map[string]any{"name": "Leg Day"}).Code)

	// The failed promotion left nothing behind — the session is still free-form.
	assert.Nil(t, decodeSession(t, getJSON(t, mux, "/api/v1/sessions/"+freeForm.ID.String()).Body).WorkoutID)

	assert.Equal(t, http.StatusNotFound,
		postJSON(t, mux, "/api/v1/sessions/00000000-0000-0000-0000-000000000000/workout", map[string]any{"name": "Ghost"}).Code)
}

func TestSession_Validation(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	// Unknown workout_id (FK) -> 409.
	assert.Equal(t, http.StatusConflict,
		postJSON(t, mux, "/api/v1/sessions", map[string]any{"workout_id": 999999}).Code)
	// Unknown movement in a free-form session (FK) -> 409.
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
	// A set kind outside the vocabulary fails the FK -> 409.
	session := decodeSession(t, postJSON(t, mux, "/api/v1/sessions", map[string]any{}).Body)
	movement := createMovement(t, mux, "Back Squat", "exercise")
	entry := decodeSession(t, postJSON(t, mux, "/api/v1/sessions/"+session.ID.String()+"/movements",
		map[string]any{"movement_id": movement}).Body).Movements[0].ID
	assert.Equal(t, http.StatusConflict,
		postJSON(t, mux, setsPath(session, entry), map[string]any{"set_kind": "vibes"}).Code)
}
