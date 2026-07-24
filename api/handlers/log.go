package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"meso/api/models"
	"meso/api/repository"
)

type LogHandler struct {
	repo *repository.LogRepo
}

func NewLogHandler(repo *repository.LogRepo) *LogHandler {
	return &LogHandler{repo: repo}
}

// List handles GET /api/v1/log with optional ?from=&to=&tag=.
func (h *LogHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	entries, err := h.repo.List(r.Context(), models.FitnessLogEntryFilter{
		From: q.Get("from"),
		To:   q.Get("to"),
		Tag:  q.Get("tag"),
	})
	if err != nil {
		if errors.Is(err, repository.ErrInvalid) {
			writeBadRequest(w, err.Error())
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *LogHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseLogID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	entry, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("log entry %s not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

// Create handles POST /api/v1/log — append a journal entry. entry_date defaults to
// today when omitted; a bad date is 400.
func (h *LogHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in models.FitnessLogEntryCreate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	entry, err := h.repo.Create(r.Context(), in)
	if err != nil {
		writeLogWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (h *LogHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseLogID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.FitnessLogEntryUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	entry, err := h.repo.Update(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("log entry %s not found", id))
			return
		}
		writeLogWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (h *LogHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseLogID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("log entry %s not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeNoContent(w)
}

// parseLogID reads a UUID path value (log entries use a UUID7 PK). A malformed uuid
// is a client error (400), not a 404.
func parseLogID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid log entry id %q", raw)
	}
	return id, nil
}

// writeLogWriteError maps repository write failures: a semantically invalid request
// (bad date) is 400; anything else is 500. The log has no FK/unique constraints, so
// there is no conflict/referenced case.
func writeLogWriteError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrInvalid) {
		writeBadRequest(w, err.Error())
		return
	}
	writeInternalError(w, err)
}
