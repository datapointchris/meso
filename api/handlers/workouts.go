package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"meso/api/models"
	"meso/api/repository"
)

type WorkoutHandler struct {
	workouts *repository.WorkoutRepo
}

func NewWorkoutHandler(workouts *repository.WorkoutRepo) *WorkoutHandler {
	return &WorkoutHandler{workouts: workouts}
}

// List handles GET /api/v1/workouts with optional filters: ?theme=&tag=&favorite=&search=.
func (h *WorkoutHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := models.WorkoutFilter{
		Theme:  q.Get("theme"),
		Tag:    q.Get("tag"),
		Search: q.Get("search"),
	}
	if raw := q.Get("favorite"); raw != "" {
		fav, err := strconv.ParseBool(raw)
		if err != nil {
			writeBadRequest(w, fmt.Sprintf("invalid favorite %q: want true or false", raw))
			return
		}
		filter.Favorite = &fav
	}

	workouts, err := h.workouts.List(r.Context(), filter)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workouts)
}

func (h *WorkoutHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	workout, err := h.workouts.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("workout %d not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workout)
}

func (h *WorkoutHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in models.WorkoutCreate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if in.Name == "" {
		writeBadRequest(w, "name is required")
		return
	}

	workout, err := h.workouts.Create(r.Context(), in)
	if err != nil {
		writeWorkoutWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, workout)
}

func (h *WorkoutHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.WorkoutUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	workout, err := h.workouts.Update(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("workout %d not found", id))
			return
		}
		writeWorkoutWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workout)
}

func (h *WorkoutHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := h.workouts.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("workout %d not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeNoContent(w)
}

// AddMovement handles POST /api/v1/workouts/{id}/movements — append a movement to
// the workout with its prescription. Returns the refreshed workout.
func (h *WorkoutHandler) AddMovement(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.WorkoutMovementInput
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if in.MovementID == 0 {
		writeBadRequest(w, "movement_id is required")
		return
	}

	workout, err := h.workouts.AddMovement(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("workout %d not found", id))
			return
		}
		writeWorkoutWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workout)
}

// UpdateMovement handles PATCH /api/v1/workouts/{id}/movements/{entryID} — edit one
// entry's prescription, or re-point it at another movement (the swap).
func (h *WorkoutHandler) UpdateMovement(w http.ResponseWriter, r *http.Request) {
	workoutID, entryID, ok := parseWorkoutEntryIDs(w, r)
	if !ok {
		return
	}
	var in models.WorkoutMovementUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	workout, err := h.workouts.UpdateMovement(r.Context(), workoutID, entryID, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("workout %d entry %d not found", workoutID, entryID))
			return
		}
		writeWorkoutWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workout)
}

// ReorderMovements handles PATCH /api/v1/workouts/{id}/movements — set the order of
// the workout's entries from the supplied id list.
func (h *WorkoutHandler) ReorderMovements(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.WorkoutMovementOrder
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	workout, err := h.workouts.ReorderMovements(r.Context(), id, in.EntryIDs)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("workout %d not found", id))
			return
		}
		writeWorkoutWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workout)
}

// RemoveMovement handles DELETE /api/v1/workouts/{id}/movements/{entryID}.
func (h *WorkoutHandler) RemoveMovement(w http.ResponseWriter, r *http.Request) {
	workoutID, entryID, ok := parseWorkoutEntryIDs(w, r)
	if !ok {
		return
	}
	workout, err := h.workouts.RemoveMovement(r.Context(), workoutID, entryID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("workout %d entry %d not found", workoutID, entryID))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workout)
}

// parseWorkoutEntryIDs reads the {id} and {entryID} path values, writing a 400 and
// returning ok=false on a bad value.
func parseWorkoutEntryIDs(w http.ResponseWriter, r *http.Request) (workoutID, entryID int64, ok bool) {
	workoutID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return 0, 0, false
	}
	entryID, err = parseID(r.PathValue("entryID"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return 0, 0, false
	}
	return workoutID, entryID, true
}

// writeWorkoutWriteError maps repository write failures to status codes: a duplicate
// name is 409; an unknown movement (FK) is 409 with a hint; a semantically invalid
// request (bad reorder set) is 400; anything else is 500.
func writeWorkoutWriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrConflict):
		writeConflict(w, "a workout with that name already exists")
	case errors.Is(err, repository.ErrReferenced):
		writeConflict(w, "unknown movement — every movement_id must reference an existing movement")
	case errors.Is(err, repository.ErrInvalid):
		writeBadRequest(w, err.Error())
	default:
		writeInternalError(w, err)
	}
}
