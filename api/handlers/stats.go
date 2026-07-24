package handlers

import (
	"net/http"

	"meso/api/repository"
)

type StatsHandler struct {
	repo *repository.StatsRepo
}

func NewStatsHandler(repo *repository.StatsRepo) *StatsHandler {
	return &StatsHandler{repo: repo}
}

// Summary handles GET /api/v1/stats — the aggregated stats-page payload (every
// measured metric's trend, plus library and session summaries) in one read.
func (h *StatsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.repo.Summary(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
