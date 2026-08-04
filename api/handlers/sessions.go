package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"meso/api/models"
	"meso/api/repository"
)

// SessionHandler owns the workout repo as well as the session one: promoting a session
// creates a workout, and the response is that workout. The write itself is one
// transaction inside SessionRepo; this only reads the result back.
type SessionHandler struct {
	sessions *repository.SessionRepo
	workouts *repository.WorkoutRepo
}

func NewSessionHandler(sessions *repository.SessionRepo, workouts *repository.WorkoutRepo) *SessionHandler {
	return &SessionHandler{sessions: sessions, workouts: workouts}
}

// List handles GET /api/v1/sessions with optional filters: ?workout_id=&from=&to=.
func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := models.WorkoutSessionFilter{
		From: q.Get("from"),
		To:   q.Get("to"),
	}
	if raw := q.Get("workout_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeBadRequest(w, fmt.Sprintf("invalid workout_id %q: want a number", raw))
			return
		}
		filter.WorkoutID = &id
	}

	sessions, err := h.sessions.List(r.Context(), filter)
	if err != nil {
		if errors.Is(err, repository.ErrInvalid) {
			writeBadRequest(w, err.Error())
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *SessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseSessionID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	session, err := h.sessions.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("session %s not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// Create handles POST /api/v1/sessions — start a session. A workout_id copies that
// workout's movements in (start-from-template); an ad-hoc session may carry its own
// movements. performed_on defaults to today when omitted.
func (h *SessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in models.WorkoutSessionCreate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	session, err := h.sessions.Create(r.Context(), in)
	if err != nil {
		writeSessionWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (h *SessionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseSessionID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.WorkoutSessionUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	session, err := h.sessions.Update(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("session %s not found", id))
			return
		}
		writeSessionWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *SessionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseSessionID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := h.sessions.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("session %s not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeNoContent(w)
}

// UpdateMovement handles PATCH /api/v1/sessions/{id}/movements/{entryID} — check off a
// set, record actuals, or swap the entry for an alternate. Returns the refreshed session.
func (h *SessionHandler) UpdateMovement(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseSessionID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	entryID, err := parseID(r.PathValue("entryID"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.SessionMovementUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	session, err := h.sessions.UpdateMovement(r.Context(), sessionID, entryID, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("session %s entry %d not found", sessionID, entryID))
			return
		}
		writeSessionWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// AddMovement handles POST /api/v1/sessions/{id}/movements — append a movement to a
// session already underway, the ad-hoc logging path. Returns the refreshed session so
// the new entry arrives with its previous-actuals populated.
func (h *SessionHandler) AddMovement(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseSessionID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.SessionMovementInput
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if in.MovementID == 0 {
		writeBadRequest(w, "movement_id is required")
		return
	}

	session, err := h.sessions.AddMovement(r.Context(), sessionID, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("session %s not found", sessionID))
			return
		}
		writeSessionWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

// RemoveMovement handles DELETE /api/v1/sessions/{id}/movements/{entryID} — drop an
// entry added by mistake. Returns the refreshed session rather than 204 so the caller
// re-renders from one response, matching the other composition endpoints.
func (h *SessionHandler) RemoveMovement(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseSessionID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	entryID, err := parseID(r.PathValue("entryID"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	session, err := h.sessions.RemoveMovement(r.Context(), sessionID, entryID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("session %s entry %d not found", sessionID, entryID))
			return
		}
		writeSessionWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// Promote handles POST /api/v1/sessions/{id}/workout — turn what was just performed
// ad-hoc into a reusable workout template, and return the created workout.
func (h *SessionHandler) Promote(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseSessionID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.SessionPromote
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if in.Name == "" {
		writeBadRequest(w, "name is required")
		return
	}

	workoutID, err := h.sessions.PromoteToWorkout(r.Context(), sessionID, in)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			writeNotFound(w, fmt.Sprintf("session %s not found", sessionID))
		case errors.Is(err, repository.ErrConflict):
			writeConflict(w, "cannot promote: the session already belongs to a workout, or that workout name is taken")
		default:
			writeSessionWriteError(w, err)
		}
		return
	}

	workout, err := h.workouts.GetByID(r.Context(), workoutID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, workout)
}

// parseSessionID reads a UUID path value (sessions use a UUID7 PK). A malformed uuid
// is a client error (400), not a 404.
func parseSessionID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid session id %q", raw)
	}
	return id, nil
}

// writeSessionWriteError maps repository write failures to status codes: an unknown
// workout or movement (FK) is 409 with a hint; a semantically invalid request (bad
// date) is 400; anything else is 500.
func writeSessionWriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrReferenced):
		writeConflict(w, "unknown workout or movement — every id must reference an existing row")
	case errors.Is(err, repository.ErrInvalid):
		writeBadRequest(w, err.Error())
	default:
		writeInternalError(w, err)
	}
}
