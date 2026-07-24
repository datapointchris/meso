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

// Review handles GET /api/v1/review?since=30d — the capstone read that pulls active
// cycles plus recent sessions, measurements, and log entries into one payload for
// Claude to reason over. An unparseable since is a 400.
func (h *ReviewHandler) Review(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	review, err := h.repo.Review(r.Context(), since)
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
