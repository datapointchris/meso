package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"meso/api/models"
	"meso/api/repository"
)

type CycleHandler struct {
	cycles *repository.CycleRepo
}

func NewCycleHandler(cycles *repository.CycleRepo) *CycleHandler {
	return &CycleHandler{cycles: cycles}
}

// List handles GET /api/v1/cycles with optional filters: ?status=&search=.
func (h *CycleHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := models.CycleFilter{
		Status: q.Get("status"),
		Search: q.Get("search"),
	}
	cycles, err := h.cycles.List(r.Context(), filter)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cycles)
}

func (h *CycleHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	cycle, err := h.cycles.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("cycle %d not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cycle)
}

func (h *CycleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in models.CycleCreate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if in.Name == "" {
		writeBadRequest(w, "name is required")
		return
	}

	cycle, err := h.cycles.Create(r.Context(), in)
	if err != nil {
		writeCycleWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cycle)
}

func (h *CycleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.CycleUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	cycle, err := h.cycles.Update(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("cycle %d not found", id))
			return
		}
		writeCycleWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cycle)
}

func (h *CycleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := h.cycles.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("cycle %d not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeNoContent(w)
}

// AddWorkout handles POST /api/v1/cycles/{id}/workouts — append a workout to the
// cycle with its periodization prescription. Returns the refreshed cycle.
func (h *CycleHandler) AddWorkout(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.CycleWorkoutInput
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if in.WorkoutID == 0 {
		writeBadRequest(w, "workout_id is required")
		return
	}

	cycle, err := h.cycles.AddWorkout(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("cycle %d not found", id))
			return
		}
		writeCycleWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cycle)
}

// UpdateWorkout handles PATCH /api/v1/cycles/{id}/workouts/{entryID} — edit one
// entry's prescription, or re-point it at another workout (the swap).
func (h *CycleHandler) UpdateWorkout(w http.ResponseWriter, r *http.Request) {
	cycleID, entryID, ok := parseCycleEntryIDs(w, r)
	if !ok {
		return
	}
	var in models.CycleWorkoutUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	cycle, err := h.cycles.UpdateWorkout(r.Context(), cycleID, entryID, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("cycle %d entry %d not found", cycleID, entryID))
			return
		}
		writeCycleWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cycle)
}

// ReorderWorkouts handles PATCH /api/v1/cycles/{id}/workouts — set the order of the
// cycle's entries from the supplied id list.
func (h *CycleHandler) ReorderWorkouts(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.CycleWorkoutOrder
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	cycle, err := h.cycles.ReorderWorkouts(r.Context(), id, in.EntryIDs)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("cycle %d not found", id))
			return
		}
		writeCycleWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cycle)
}

// RemoveWorkout handles DELETE /api/v1/cycles/{id}/workouts/{entryID}.
func (h *CycleHandler) RemoveWorkout(w http.ResponseWriter, r *http.Request) {
	cycleID, entryID, ok := parseCycleEntryIDs(w, r)
	if !ok {
		return
	}
	cycle, err := h.cycles.RemoveWorkout(r.Context(), cycleID, entryID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("cycle %d entry %d not found", cycleID, entryID))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cycle)
}

// parseCycleEntryIDs reads the {id} and {entryID} path values, writing a 400 and
// returning ok=false on a bad value.
func parseCycleEntryIDs(w http.ResponseWriter, r *http.Request) (cycleID, entryID int64, ok bool) {
	cycleID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return 0, 0, false
	}
	entryID, err = parseID(r.PathValue("entryID"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return 0, 0, false
	}
	return cycleID, entryID, true
}

// writeCycleWriteError maps repository write failures to status codes: a duplicate
// name is 409; an unknown workout / target metric (FK) is 409 with a hint; a
// semantically invalid request (bad status, bad date, bad reorder set) is 400;
// anything else is 500.
func writeCycleWriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrConflict):
		writeConflict(w, "a cycle with that name already exists")
	case errors.Is(err, repository.ErrReferenced):
		writeConflict(w, "unknown reference — workout_id and target_metric must reference existing rows, and status must be a valid cycle status")
	case errors.Is(err, repository.ErrInvalid):
		writeBadRequest(w, err.Error())
	default:
		writeInternalError(w, err)
	}
}
