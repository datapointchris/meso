package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"meso/api/models"
	"meso/api/repository"
)

type MovementHandler struct {
	movements *repository.MovementRepo
}

func NewMovementHandler(movements *repository.MovementRepo) *MovementHandler {
	return &MovementHandler{movements: movements}
}

// List handles GET /api/v1/movements with optional filters:
// ?kind=&favorite=&tag=&equipment=&muscle=&region=&search=. Unknown params are
// ignored; an unparseable favorite is a bad request.
func (h *MovementHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := models.MovementFilter{
		Kind:      q.Get("kind"),
		Tag:       q.Get("tag"),
		Equipment: q.Get("equipment"),
		Muscle:    q.Get("muscle"),
		Region:    q.Get("region"),
		Search:    q.Get("search"),
	}
	if raw := q.Get("favorite"); raw != "" {
		fav, err := strconv.ParseBool(raw)
		if err != nil {
			writeBadRequest(w, fmt.Sprintf("invalid favorite %q: want true or false", raw))
			return
		}
		filter.Favorite = &fav
	}

	movements, err := h.movements.List(r.Context(), filter)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, movements)
}

func (h *MovementHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	movement, err := h.movements.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("movement %d not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, movement)
}

func (h *MovementHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in models.MovementCreate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if in.Name == "" {
		writeBadRequest(w, "name is required")
		return
	}
	if in.MovementKind == "" {
		writeBadRequest(w, "movement_kind is required")
		return
	}
	if msg, ok := validateMuscles(in.Muscles); !ok {
		writeBadRequest(w, msg)
		return
	}

	movement, err := h.movements.Create(r.Context(), in)
	if err != nil {
		writeMovementWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, movement)
}

func (h *MovementHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.MovementUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if in.Muscles != nil {
		if msg, ok := validateMuscles(*in.Muscles); !ok {
			writeBadRequest(w, msg)
			return
		}
	}

	movement, err := h.movements.Update(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("movement %d not found", id))
			return
		}
		writeMovementWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, movement)
}

func (h *MovementHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := h.movements.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("movement %d not found", id))
			return
		}
		if errors.Is(err, repository.ErrReferenced) {
			writeConflict(w, fmt.Sprintf("movement %d is referenced by other records", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeNoContent(w)
}

// AddRelated handles POST /api/v1/movements/{id}/related — record a directional
// relationship (alternate, antagonist, ...) from this movement to another. Returns
// the refreshed movement so the caller sees the updated related list.
func (h *MovementHandler) AddRelated(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.RelationshipInput
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if in.RelatedMovementID == 0 {
		writeBadRequest(w, "related_movement_id is required")
		return
	}
	if in.RelationshipKind == "" {
		writeBadRequest(w, "relationship_kind is required")
		return
	}

	if err := h.movements.AddRelationship(r.Context(), id, in.RelatedMovementID, in.RelationshipKind); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("movement %d not found", id))
			return
		}
		writeMovementWriteError(w, err)
		return
	}
	movement, err := h.movements.GetByID(r.Context(), id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, movement)
}

// RemoveRelated handles DELETE /api/v1/movements/{id}/related/{rid} — drop the
// relationship(s) to {rid}; an optional ?kind= narrows to one kind. Returns the
// refreshed movement.
func (h *MovementHandler) RemoveRelated(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	relatedID, err := parseID(r.PathValue("rid"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	kind := r.URL.Query().Get("kind")

	if err := h.movements.RemoveRelationship(r.Context(), id, relatedID, kind); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("no such relationship from movement %d to %d", id, relatedID))
			return
		}
		writeInternalError(w, err)
		return
	}
	movement, err := h.movements.GetByID(r.Context(), id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, movement)
}

// ListMuscles handles GET /api/v1/muscles — the muscle-tagging vocabulary.
func (h *MovementHandler) ListMuscles(w http.ResponseWriter, r *http.Request) {
	muscles, err := h.movements.ListMuscles(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, muscles)
}

// validateMuscles enforces the role vocabulary up front so a bad role yields a
// clean 400 rather than surfacing as a CHECK-constraint 500 from Postgres. The
// muscle name itself is FK-validated in the repository (unknown -> 409).
func validateMuscles(muscles []models.MovementMuscleInput) (string, bool) {
	for _, mm := range muscles {
		if mm.Muscle == "" {
			return "each muscle tag needs a muscle name", false
		}
		if mm.Role != "primary" && mm.Role != "secondary" {
			return fmt.Sprintf("invalid muscle role %q: want primary or secondary", mm.Role), false
		}
	}
	return "", true
}

// writeMovementWriteError maps repository write failures to status codes: a
// duplicate name is 409; an unknown movement_kind or muscle (FK violation) is
// also 409 with a hint; anything else is 500.
func writeMovementWriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrConflict):
		writeConflict(w, "a movement with that name already exists")
	case errors.Is(err, repository.ErrReferenced):
		writeConflict(w, "unknown reference — the movement_kind, muscle, related movement, or relationship_kind must exist")
	case errors.Is(err, repository.ErrInvalid):
		writeBadRequest(w, err.Error())
	default:
		writeInternalError(w, err)
	}
}
