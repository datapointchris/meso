package api

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"
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

	// A bound goes on the wire in the machine's offset, whatever zone it was built in.
	since := time.Date(2026, 5, 1, 4, 0, 0, 0, time.UTC)
	until := time.Date(2026, 8, 1, 4, 0, 0, 0, time.UTC)
	review, err := client.GetReview(context.Background(), Window{Since: since, Until: until})
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/api/v1/review" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	want := url.Values{
		"since": {since.In(time.Local).Format(time.RFC3339)},
		"until": {until.In(time.Local).Format(time.RFC3339)},
	}.Encode()
	if got.query != want {
		t.Errorf("query = %q, want %q", got.query, want)
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
