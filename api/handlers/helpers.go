// Package handlers holds the HTTP layer: one file per resource, each translating
// requests into repository calls and repository errors into status codes. The
// wire contract (JSON shapes) lives in models; SQL lives in repository.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: http.StatusText(status), Message: msg})
}

func writeBadRequest(w http.ResponseWriter, msg string) { writeError(w, http.StatusBadRequest, msg) }
func writeNotFound(w http.ResponseWriter, msg string)   { writeError(w, http.StatusNotFound, msg) }
func writeConflict(w http.ResponseWriter, msg string)   { writeError(w, http.StatusConflict, msg) }
func writeInternalError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusInternalServerError, err.Error())
}

func decodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	defer func() { _ = r.Body.Close() }()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// parseID reads a numeric path value (movements use a Postgres identity PK, so
// their ids are integers). A non-numeric id is a client error, not a 404.
func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q", raw)
	}
	return id, nil
}
