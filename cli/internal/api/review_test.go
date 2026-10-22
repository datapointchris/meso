package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestGetReview_QueryAndDecode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{
		"schema_version":1,
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
	if review.SchemaVersion != ReviewSchemaVersion || review.Since != "2026-05-01" {
		t.Errorf("schema_version = %d, since = %q", review.SchemaVersion, review.Since)
	}
	if len(review.ActiveCycles) != 1 || review.ActiveCycles[0].Name != "Current block" {
		t.Errorf("active_cycles = %+v", review.ActiveCycles)
	}
	if len(review.Sessions) != 1 || len(review.Measurements) != 1 || len(review.LogEntries) != 1 {
		t.Errorf("history slices = %+v", review)
	}
}

// A payload newer than the client reads is refused whole, never decoded into the
// older shape.
func TestVersionedReads_RefuseNewerSchema(t *testing.T) {
	reads := map[string]func(*Client) error{
		"review": func(c *Client) error { _, err := c.GetReview(context.Background(), Window{}); return err },
		"stats":  func(c *Client) error { _, err := c.Stats(context.Background()); return err },
	}
	for payload, read := range reads {
		t.Run(payload, func(t *testing.T) {
			srv, _ := recordingServer(t, http.StatusOK, `{"schema_version":99}`)
			err := read(New(srv.URL, staticTokenClient("t")))
			var skew *SchemaSkewError
			if !errors.As(err, &skew) || skew.Payload != payload || skew.Got != 99 {
				t.Fatalf("err = %v, want a %s schema skew at 99", err, payload)
			}
		})
	}
}
