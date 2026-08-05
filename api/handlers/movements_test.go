package handlers_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"meso/api/models"
)

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

// movementPayload builds a create body; overrides let each test vary a field.
func movementPayload(name, kind string) map[string]any {
	return map[string]any{
		"name":          name,
		"movement_kind": kind,
		"tags":          []string{"posterior-chain"},
		"equipment":     []string{"barbell"},
		"how_to":        "hinge at the hips, flat back, drive through the floor",
		"form_cues":     "brace, bar close to shins",
		"common_faults": "rounding the lower back",
		"muscles": []map[string]string{
			{"muscle": "hamstrings", "role": "primary"},
			{"muscle": "glutes", "role": "secondary"},
		},
	}
}

func decodeMovement(t *testing.T, rr interface{ Bytes() []byte }) models.Movement {
	t.Helper()
	var m models.Movement
	require.NoError(t, json.Unmarshal(rr.Bytes(), &m))
	return m
}

func TestMovement_CreateAndGet(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	rr := postJSON(t, mux, "/api/v1/movements", movementPayload("Deadlift", "exercise"))
	require.Equal(t, http.StatusCreated, rr.Code)

	created := decodeMovement(t, rr.Body)
	assert.Equal(t, "Deadlift", created.Name)
	assert.Equal(t, "exercise", created.MovementKind)
	assert.False(t, created.Favorite)
	assert.Equal(t, []string{"posterior-chain"}, created.Tags)
	require.Len(t, created.Muscles, 2)
	// Muscles come back ordered by region then name, with region derived from the
	// lookup: glutes(posterior) before hamstrings(posterior).
	assert.Equal(t, "glutes", created.Muscles[0].Muscle)
	assert.Equal(t, "posterior", created.Muscles[0].Region)
	assert.Equal(t, "secondary", created.Muscles[0].Role)
	assert.NotZero(t, created.ID)

	got := getJSON(t, mux, "/api/v1/movements/"+itoa(created.ID))
	require.Equal(t, http.StatusOK, got.Code)
	fetched := decodeMovement(t, got.Body)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Len(t, fetched.Muscles, 2)
}

func TestMovement_Get_NotFound(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/movements/999999").Code)
	// A non-numeric id is a client error (bad request), not a 404.
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/movements/abc").Code)
}

func TestMovement_List_Filters(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	// A favorited stretch tagged mobility, hitting quads.
	stretch := movementPayload("Couch Stretch", "stretch")
	stretch["favorite"] = true
	stretch["tags"] = []string{"mobility"}
	stretch["muscles"] = []map[string]string{{"muscle": "quads", "role": "primary"}}
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/movements", stretch).Code)

	// A non-favorite exercise (hamstrings/glutes, posterior).
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/movements", movementPayload("Deadlift", "exercise")).Code)

	list := func(query string) []models.Movement {
		rr := getJSON(t, mux, "/api/v1/movements"+query)
		require.Equal(t, http.StatusOK, rr.Code)
		var out []models.Movement
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
		return out
	}

	assert.Len(t, list(""), 2)
	assert.Len(t, list("?kind=stretch"), 1)
	assert.Equal(t, "Couch Stretch", list("?kind=stretch")[0].Name)
	assert.Len(t, list("?favorite=true"), 1)
	assert.Len(t, list("?favorite=false"), 1)
	assert.Len(t, list("?tag=mobility"), 1)
	assert.Len(t, list("?equipment=barbell"), 2)
	assert.Len(t, list("?muscle=quads"), 1)
	assert.Len(t, list("?region=posterior"), 1)
	assert.Equal(t, "Deadlift", list("?region=posterior")[0].Name)
	assert.Len(t, list("?search=couch"), 1)
	assert.Empty(t, list("?kind=yoga_pose"))
	// An unparsable favorite is a bad request.
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/movements?favorite=maybe").Code)
}

// Search has to survive how a movement's name is actually punctuated: the library
// writes "Pulldown" and "Pull-up", and the search is typed as neither.
func TestMovement_List_SearchIsPunctuationInsensitive(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	pulldown := movementPayload("Eccentric Straight-Arm Pulldown", "exercise")
	pulldown["tags"] = []string{"back-day"}
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/movements", pulldown).Code)
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/movements", movementPayload("Wide Grip Pull-up", "exercise")).Code)

	names := func(query string) []string {
		rr := getJSON(t, mux, "/api/v1/movements"+query)
		require.Equal(t, http.StatusOK, rr.Code)
		var out []models.Movement
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
		got := make([]string, 0, len(out))
		for _, m := range out {
			got = append(got, m.Name)
		}
		return got
	}

	// Separators are noise on both sides of the match.
	for _, query := range []string{"?search=pull-down", "?search=pulldown", "?search=pull+down", "?search=PULLDOWN"} {
		assert.Equal(t, []string{"Eccentric Straight-Arm Pulldown"}, names(query), query)
	}
	assert.Equal(t, []string{"Wide Grip Pull-up"}, names("?search=pullup"))

	// Every token must match, so words can arrive in any order with anything between.
	assert.Equal(t, []string{"Eccentric Straight-Arm Pulldown"}, names("?search=straight+arm+pull+down"))
	assert.Equal(t, []string{"Eccentric Straight-Arm Pulldown"}, names("?search=pulldown+eccentric"))
	assert.Empty(t, names("?search=cable+pulldown")) // "cable" matches nothing here

	// Tags are searched the same way.
	assert.Equal(t, []string{"Eccentric Straight-Arm Pulldown"}, names("?search=back+day"))

	// A query of nothing but separators is no filter, not an empty result.
	assert.Len(t, names("?search=+-+"), 2)
}

func TestMovement_Update_Partial(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	created := decodeMovement(t, postJSON(t, mux, "/api/v1/movements", movementPayload("Deadlift", "exercise")).Body)

	// Favorite it and replace its muscles, leaving name/kind untouched.
	body := map[string]any{
		"favorite": true,
		"rating":   4,
		"muscles":  []map[string]string{{"muscle": "glutes", "role": "primary"}},
	}
	rr := putJSON(t, mux, "/api/v1/movements/"+itoa(created.ID), body)
	require.Equal(t, http.StatusOK, rr.Code)

	updated := decodeMovement(t, rr.Body)
	assert.True(t, updated.Favorite)
	require.NotNil(t, updated.Rating)
	assert.Equal(t, 4, *updated.Rating)
	assert.Equal(t, "Deadlift", updated.Name) // unchanged
	require.Len(t, updated.Muscles, 1)
	assert.Equal(t, "glutes", updated.Muscles[0].Muscle)
}

func TestMovement_Delete(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	created := decodeMovement(t, postJSON(t, mux, "/api/v1/movements", movementPayload("Deadlift", "exercise")).Body)

	assert.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/movements/"+itoa(created.ID)).Code)
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/movements/"+itoa(created.ID)).Code)
	assert.Equal(t, http.StatusNotFound, deleteReq(t, mux, "/api/v1/movements/"+itoa(created.ID)).Code)
}

// Load mode decides whether the logging screen asks for a weight at all, which is why
// a movement can never be without one.
func TestMovement_LoadMode(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	// Unstated, it is inferred from the kind: a pose is held, a lift is loaded.
	lift := decodeMovement(t, postJSON(t, mux, "/api/v1/movements", movementPayload("Deadlift", "exercise")).Body)
	assert.Equal(t, "weighted", lift.LoadMode)
	pose := decodeMovement(t, postJSON(t, mux, "/api/v1/movements", movementPayload("Pigeon", "yoga_pose")).Body)
	assert.Equal(t, "timed", pose.LoadMode)

	// Stated, it is taken as given — the inference is only ever a starting point.
	abs := movementPayload("Hanging Leg Raise", "exercise")
	abs["load_mode"] = "bodyweight"
	created := decodeMovement(t, postJSON(t, mux, "/api/v1/movements", abs).Body)
	assert.Equal(t, "bodyweight", created.LoadMode)

	// Correcting the guess is a partial update like any other.
	rr := putJSON(t, mux, "/api/v1/movements/"+itoa(lift.ID), map[string]any{"load_mode": "assisted"})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "assisted", decodeMovement(t, rr.Body).LoadMode)
	// And an update silent about it leaves the correction alone.
	rr = putJSON(t, mux, "/api/v1/movements/"+itoa(lift.ID), map[string]any{"favorite": true})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "assisted", decodeMovement(t, rr.Body).LoadMode)

	// The list filters on it, which is how a correction pass finds what to look at.
	var filtered []models.Movement
	require.NoError(t, json.Unmarshal(getJSON(t, mux, "/api/v1/movements?load_mode=bodyweight").Body.Bytes(), &filtered))
	require.Len(t, filtered, 1)
	assert.Equal(t, "Hanging Leg Raise", filtered[0].Name)

	// A mode outside the vocabulary fails the FK -> 409.
	bogus := movementPayload("Vibes Press", "exercise")
	bogus["load_mode"] = "telekinetic"
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/movements", bogus).Code)
}

func TestMovement_Create_Validation(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	// Duplicate name -> 409.
	require.Equal(t, http.StatusCreated, postJSON(t, mux, "/api/v1/movements", movementPayload("Deadlift", "exercise")).Code)
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/movements", movementPayload("Deadlift", "exercise")).Code)

	// Missing name / kind -> 400.
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, "/api/v1/movements", map[string]any{"movement_kind": "exercise"}).Code)
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, "/api/v1/movements", map[string]any{"name": "Nameless"}).Code)

	// Unknown movement_kind (FK) -> 409.
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/movements", movementPayload("Odd One", "not-a-kind")).Code)

	// Invalid muscle role -> 400.
	bad := movementPayload("Bad Role", "exercise")
	bad["muscles"] = []map[string]string{{"muscle": "quads", "role": "tertiary"}}
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, "/api/v1/movements", bad).Code)

	// Unknown muscle (FK) -> 409.
	unknown := movementPayload("Unknown Muscle", "exercise")
	unknown["muscles"] = []map[string]string{{"muscle": "gluteus-imaginarius", "role": "primary"}}
	assert.Equal(t, http.StatusConflict, postJSON(t, mux, "/api/v1/movements", unknown).Code)
}

func TestMuscles_List(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	rr := getJSON(t, mux, "/api/v1/muscles")
	require.Equal(t, http.StatusOK, rr.Code)

	var muscles []models.Muscle
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &muscles))
	assert.NotEmpty(t, muscles)
}
