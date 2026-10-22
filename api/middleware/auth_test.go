package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datapointchris/goapiauth"
)

func TestDevGroups_PlacesEveryRequestInTheNamedGroups(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	DevGroups(goapiauth.RequireGroup("admin", handler), "admin").
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/feedback", nil))
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("development request was refused: status %d", rec.Code)
	}
}
