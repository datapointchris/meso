package handlers

import (
	"errors"
	"net/http"

	"meso/api/repository"
)

type ReviewHandler struct {
	repo *repository.ReviewRepo
}

func NewReviewHandler(repo *repository.ReviewRepo) *ReviewHandler {
	return &ReviewHandler{repo: repo}
}

// Review handles GET /api/v1/review?since=&until= — the capstone read that pulls
// active cycles plus the sessions, measurements, and log entries in the window into
// one payload for Claude to reason over. An unparsable bound is a 400.
func (h *ReviewHandler) Review(w http.ResponseWriter, r *http.Request) {
	first, last, err := readWindow(r.URL.Query())
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	review, err := h.repo.Review(r.Context(), first, last)
	if err != nil {
		if errors.Is(err, repository.ErrInvalid) {
			writeBadRequest(w, err.Error())
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, review)
}
