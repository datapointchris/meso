package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"meso/api/models"
	"meso/api/repository"
)

type FeedbackHandler struct {
	repo *repository.FeedbackRepo
}

func NewFeedbackHandler(repo *repository.FeedbackRepo) *FeedbackHandler {
	return &FeedbackHandler{repo: repo}
}

// List handles GET /api/v1/feedback with optional ?status=&search=. Reads are the
// CLI's admin surface; the web app only ever POSTs here.
func (h *FeedbackHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := h.repo.List(r.Context(), models.FeedbackFilter{
		Status: q.Get("status"),
		Search: q.Get("search"),
	})
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *FeedbackHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseFeedbackID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	item, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("feedback %s not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// Create handles POST /api/v1/feedback — the capture the in-app button makes. An
// empty body is 400.
func (h *FeedbackHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in models.FeedbackCreate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	item, err := h.repo.Create(r.Context(), in)
	if err != nil {
		writeFeedbackWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *FeedbackHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseFeedbackID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.FeedbackUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	item, err := h.repo.Update(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("feedback %s not found", id))
			return
		}
		writeFeedbackWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *FeedbackHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseFeedbackID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("feedback %s not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeNoContent(w)
}

// parseFeedbackID reads a UUID path value (feedback uses a UUID7 PK). A malformed
// uuid is a client error (400), not a 404.
func parseFeedbackID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid feedback id %q", raw)
	}
	return id, nil
}

// writeFeedbackWriteError maps repository write failures: an empty body or an
// out-of-range status is 400, anything else 500. Feedback has no FK or unique
// constraints, so there is no conflict/referenced case.
func writeFeedbackWriteError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrInvalid) {
		writeBadRequest(w, err.Error())
		return
	}
	writeInternalError(w, err)
}
