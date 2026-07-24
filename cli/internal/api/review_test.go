package api

import (
	"context"
	"net/http"
	"testing"
)

func TestGetReview_QueryAndDecode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{
		"since":"2026-05-01",
		"active_cycles":[{"id":1,"name":"Current block","status":"active","workouts":[]}],
		"sessions":[{"id":"abc","workout_name":"Push Day","performed_on":"2026-07-20","movements":[]}],
		"measurements":[{"id":3,"metric":"5k-time","value":1500,"measured_on":"2026-07-18"}],
		"log_entries":[{"id":"def","entry_date":"2026-07-19","body":"felt strong","tags":[]}]
	}`)
	client := New(srv.URL, staticTokenClient("t"))

	review, err := client.GetReview(context.Background(), "12w")
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/api/v1/review" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if !containsParam(got.query, "since=12w") {
		t.Errorf("query %q missing since=12w", got.query)
	}
	if review.Since != "2026-05-01" {
		t.Errorf("since = %q", review.Since)
	}
	if len(review.ActiveCycles) != 1 || review.ActiveCycles[0].Name != "Current block" {
		t.Errorf("active_cycles = %+v", review.ActiveCycles)
	}
	if len(review.Sessions) != 1 || len(review.Measurements) != 1 || len(review.LogEntries) != 1 {
		t.Errorf("history slices = %+v", review)
	}
}

func TestGetReview_OmitsSinceWhenEmpty(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"since":"2026-06-24","active_cycles":[],"sessions":[],"measurements":[],"log_entries":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	if _, err := client.GetReview(context.Background(), ""); err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if got.query != "" {
		t.Errorf("expected no query string, got %q", got.query)
	}
}
