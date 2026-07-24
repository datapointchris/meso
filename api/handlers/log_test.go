package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"meso/api/models"
)

func decodeLogEntry(t *testing.T, rr interface{ Bytes() []byte }) models.FitnessLogEntry {
	t.Helper()
	var e models.FitnessLogEntry
	require.NoError(t, json.Unmarshal(rr.Bytes(), &e))
	return e
}

func TestLog_CreateGetListFilterUpdateDelete(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	// Create — body/tags/mood/date round-trip; the id is a minted UUID7.
	rr := postJSON(t, mux, "/api/v1/log", map[string]any{
		"entry_date": "2026-07-20",
		"body":       "Deadlifts felt heavy but moved. Knee-to-wall symmetric.",
		"tags":       []string{"strength", "knee"},
		"mood":       "focused",
	})
	require.Equal(t, http.StatusCreated, rr.Code)
	created := decodeLogEntry(t, rr.Body)
	assert.NotEqual(t, "00000000-0000-0000-0000-000000000000", created.ID.String())
	assert.Equal(t, "2026-07-20", created.EntryDate)
	assert.Equal(t, []string{"strength", "knee"}, created.Tags)
	require.NotNil(t, created.Mood)
	assert.Equal(t, "focused", *created.Mood)

	// A minimal entry: no date defaults to today, no mood is null, no tags is [] not
	// null. Deleted straight after so it doesn't perturb the date-window counts below
	// (its "today" is the machine clock, not a fixed date).
	bare := postJSON(t, mux, "/api/v1/log", map[string]any{"body": "quick note"})
	require.Equal(t, http.StatusCreated, bare.Code)
	bareEntry := decodeLogEntry(t, bare.Body)
	assert.Nil(t, bareEntry.Mood)
	require.NotNil(t, bareEntry.Tags)
	assert.Len(t, bareEntry.Tags, 0)
	assert.NotEmpty(t, bareEntry.EntryDate)
	require.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/log/"+bareEntry.ID.String()).Code)

	// Get by id.
	got := getJSON(t, mux, "/api/v1/log/"+created.ID.String())
	require.Equal(t, http.StatusOK, got.Code)
	assert.Equal(t, created.ID, decodeLogEntry(t, got.Body).ID)

	// A few more, out of date order, to prove newest-first ordering and date/tag filters.
	postJSON(t, mux, "/api/v1/log", map[string]any{"entry_date": "2026-07-25", "body": "mobility", "tags": []string{"mobility"}})
	postJSON(t, mux, "/api/v1/log", map[string]any{"entry_date": "2026-07-10", "body": "rest day", "tags": []string{"rest"}})

	list := func(query string) []models.FitnessLogEntry {
		lr := getJSON(t, mux, "/api/v1/log"+query)
		require.Equal(t, http.StatusOK, lr.Code)
		var out []models.FitnessLogEntry
		require.NoError(t, json.Unmarshal(lr.Body.Bytes(), &out))
		return out
	}

	all := list("")
	require.Len(t, all, 3)                          // created(07-20), 07-25, 07-10
	assert.Equal(t, "2026-07-25", all[0].EntryDate) // newest first
	assert.Len(t, list("?from=2026-07-15"), 2)      // 07-20, 07-25
	assert.Len(t, list("?to=2026-07-10"), 1)        // only 07-10
	assert.Len(t, list("?tag=knee"), 1)             // only the tagged entry
	assert.Len(t, list("?tag=nope"), 0)

	// Update — change body/tags/date; a nil field is left unchanged.
	up := putJSON(t, mux, "/api/v1/log/"+created.ID.String(), map[string]any{
		"body": "edited: deadlifts moved well",
		"tags": []string{"strength"},
	})
	require.Equal(t, http.StatusOK, up.Code)
	updated := decodeLogEntry(t, up.Body)
	assert.Equal(t, "edited: deadlifts moved well", updated.Body)
	assert.Equal(t, []string{"strength"}, updated.Tags)
	assert.Equal(t, "2026-07-20", updated.EntryDate) // unchanged
	require.NotNil(t, updated.Mood)
	assert.Equal(t, "focused", *updated.Mood) // unchanged

	// Tags can be cleared with an explicit empty array.
	cleared := decodeLogEntry(t, putJSON(t, mux, "/api/v1/log/"+created.ID.String(), map[string]any{"tags": []string{}}).Body)
	assert.Len(t, cleared.Tags, 0)

	// Delete frees it; a second delete is a 404.
	require.Equal(t, http.StatusNoContent, deleteReq(t, mux, "/api/v1/log/"+created.ID.String()).Code)
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/log/"+created.ID.String()).Code)
	assert.Equal(t, http.StatusNotFound, deleteReq(t, mux, "/api/v1/log/"+created.ID.String()).Code)
}

func TestLog_Validation(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))

	// Bad create date -> 400.
	assert.Equal(t, http.StatusBadRequest, postJSON(t, mux, "/api/v1/log", map[string]any{
		"body": "x", "entry_date": "last tuesday",
	}).Code)
	// Bad list filter -> 400.
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/log?from=nope").Code)
	// Malformed uuid -> 400; well-formed but absent -> 404.
	assert.Equal(t, http.StatusBadRequest, getJSON(t, mux, "/api/v1/log/not-a-uuid").Code)
	assert.Equal(t, http.StatusNotFound, getJSON(t, mux, "/api/v1/log/00000000-0000-0000-0000-000000000000").Code)
}
