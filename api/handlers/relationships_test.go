package handlers_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMovement_Relationships_AddGetRemove(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	row := createMovement(t, mux, "Barbell Row", "exercise")
	dbRow := createMovement(t, mux, "Dumbbell Row", "exercise")
	press := createMovement(t, mux, "Bench Press", "exercise")

	base := "/api/v1/movements/" + itoa(row) + "/related"

	// Add an alternate and an antagonist; each POST returns the refreshed movement.
	rr := postJSON(t, mux, base, map[string]any{"related_movement_id": dbRow, "relationship_kind": "alternate"})
	require.Equal(t, http.StatusOK, rr.Code)
	rr = postJSON(t, mux, base, map[string]any{"related_movement_id": press, "relationship_kind": "antagonist"})
	require.Equal(t, http.StatusOK, rr.Code)

	m := decodeMovement(t, rr.Body)
	require.Len(t, m.Related, 2)
	// Ordered by relationship_kind then name: alternate before antagonist.
	assert.Equal(t, "alternate", m.Related[0].RelationshipKind)
	assert.Equal(t, "Dumbbell Row", m.Related[0].Name)
	assert.Equal(t, "antagonist", m.Related[1].RelationshipKind)

	// GET the movement shows the same embedded relationships.
	got := decodeMovement(t, getJSON(t, mux, "/api/v1/movements/"+itoa(row)).Body)
	assert.Len(t, got.Related, 2)

	// Re-adding the same edge is idempotent (still two).
	rr = postJSON(t, mux, base, map[string]any{"related_movement_id": dbRow, "relationship_kind": "alternate"})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Len(t, decodeMovement(t, rr.Body).Related, 2)

	// Remove the alternate by kind; the antagonist remains.
	rr = deleteReq(t, mux, base+"/"+itoa(dbRow)+"?kind=alternate")
	require.Equal(t, http.StatusOK, rr.Code)
	m = decodeMovement(t, rr.Body)
	require.Len(t, m.Related, 1)
	assert.Equal(t, "antagonist", m.Related[0].RelationshipKind)
}

func TestMovement_Relationships_Validation(t *testing.T) {
	mux := buildTestMux(setupTestDB(t))
	row := createMovement(t, mux, "Barbell Row", "exercise")
	other := createMovement(t, mux, "Pull-up", "exercise")
	base := "/api/v1/movements/" + itoa(row) + "/related"

	// A movement can't relate to itself -> 400.
	assert.Equal(t, http.StatusBadRequest,
		postJSON(t, mux, base, map[string]any{"related_movement_id": row, "relationship_kind": "alternate"}).Code)

	// Unknown relationship_kind (FK) -> 409.
	assert.Equal(t, http.StatusConflict,
		postJSON(t, mux, base, map[string]any{"related_movement_id": other, "relationship_kind": "nemesis"}).Code)

	// Unknown target movement (FK) -> 409.
	assert.Equal(t, http.StatusConflict,
		postJSON(t, mux, base, map[string]any{"related_movement_id": 999999, "relationship_kind": "alternate"}).Code)

	// Missing fields -> 400.
	assert.Equal(t, http.StatusBadRequest,
		postJSON(t, mux, base, map[string]any{"relationship_kind": "alternate"}).Code)
	assert.Equal(t, http.StatusBadRequest,
		postJSON(t, mux, base, map[string]any{"related_movement_id": other}).Code)

	// Adding a relationship to a nonexistent source movement -> 404.
	assert.Equal(t, http.StatusNotFound,
		postJSON(t, mux, "/api/v1/movements/999999/related", map[string]any{
			"related_movement_id": other, "relationship_kind": "alternate",
		}).Code)

	// Removing a relationship that doesn't exist -> 404.
	assert.Equal(t, http.StatusNotFound, deleteReq(t, mux, base+"/"+itoa(other)).Code)
}
