package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"meso/api/models"
	"meso/api/repository"
)

type MeasurementHandler struct {
	repo *repository.MeasurementRepo
}

func NewMeasurementHandler(repo *repository.MeasurementRepo) *MeasurementHandler {
	return &MeasurementHandler{repo: repo}
}

// ListMetrics handles GET /api/v1/metrics — the metric-definition vocabulary.
func (h *MeasurementHandler) ListMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.repo.ListMetrics(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

// DefineMetric handles POST /api/v1/metrics — define a metric. A duplicate name is
// 409; an out-of-range direction/category (CHECK) is 400.
func (h *MeasurementHandler) DefineMetric(w http.ResponseWriter, r *http.Request) {
	var in models.MetricDefinitionCreate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	metric, err := h.repo.DefineMetric(r.Context(), in)
	if err != nil {
		writeMetricWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, metric)
}

// DeleteMetric handles DELETE /api/v1/metrics/{name}. A metric still referenced by a
// recorded measurement is 409; an unknown name is 404. A cycle that targets it is
// SET NULL by the FK, so cycle references do not block the delete.
func (h *MeasurementHandler) DeleteMetric(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.repo.DeleteMetric(r.Context(), name); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			writeNotFound(w, fmt.Sprintf("metric %q not found", name))
		case errors.Is(err, repository.ErrReferenced):
			writeConflict(w, "metric is still referenced by a recorded measurement — delete those readings first")
		default:
			writeInternalError(w, err)
		}
		return
	}
	writeNoContent(w)
}

// Trend handles GET /api/v1/metrics/{name}/trend — one metric's series plus summary,
// optionally windowed by ?from=&to=. A 404 when the metric is undefined.
func (h *MeasurementHandler) Trend(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	q := r.URL.Query()
	trend, err := h.repo.Trend(r.Context(), name, models.MeasurementFilter{From: q.Get("from"), To: q.Get("to")})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("metric %q not found", name))
			return
		}
		if errors.Is(err, repository.ErrInvalid) {
			writeBadRequest(w, err.Error())
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, trend)
}

// List handles GET /api/v1/measurements with optional ?metric=&from=&to=.
func (h *MeasurementHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	measurements, err := h.repo.ListMeasurements(r.Context(), models.MeasurementFilter{
		Metric: q.Get("metric"),
		From:   q.Get("from"),
		To:     q.Get("to"),
	})
	if err != nil {
		if errors.Is(err, repository.ErrInvalid) {
			writeBadRequest(w, err.Error())
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, measurements)
}

func (h *MeasurementHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	measurement, err := h.repo.GetMeasurement(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("measurement %d not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, measurement)
}

// Record handles POST /api/v1/measurements — append a reading. An unknown metric is
// 409 (FK); a bad date is 400.
func (h *MeasurementHandler) Record(w http.ResponseWriter, r *http.Request) {
	var in models.MeasurementCreate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	measurement, err := h.repo.RecordMeasurement(r.Context(), in)
	if err != nil {
		writeMeasurementWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, measurement)
}

func (h *MeasurementHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var in models.MeasurementUpdate
	if err := decodeJSON(r, &in); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	measurement, err := h.repo.UpdateMeasurement(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("measurement %d not found", id))
			return
		}
		writeMeasurementWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, measurement)
}

func (h *MeasurementHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := h.repo.DeleteMeasurement(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("measurement %d not found", id))
			return
		}
		writeInternalError(w, err)
		return
	}
	writeNoContent(w)
}

// writeMetricWriteError maps metric-definition write failures: a duplicate name is
// 409; an out-of-range direction/category (CHECK) is 400.
func writeMetricWriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrConflict):
		writeConflict(w, "a metric with that name already exists")
	case errors.Is(err, repository.ErrInvalid):
		writeBadRequest(w, "direction must be higher_better|lower_better and category strength|cardio|mobility|body")
	default:
		writeInternalError(w, err)
	}
}

// writeMeasurementWriteError maps measurement write failures: an unknown metric (FK)
// is 409 with a hint; a bad date is 400.
func writeMeasurementWriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrReferenced):
		writeConflict(w, "unknown metric — define it first with a metric definition")
	case errors.Is(err, repository.ErrInvalid):
		writeBadRequest(w, err.Error())
	default:
		writeInternalError(w, err)
	}
}
