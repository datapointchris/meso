package api

import (
	"context"
	"net/http"
	"testing"
)

func TestListSessions_QueryAndDecode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `[
		{"id":"018f-aaa","workout_id":1,"workout_name":"Push Day","performed_on":"2026-07-24","felt":"strong",
		 "movements":[{"id":4,"movement_id":7,"movement_name":"Bench Press","movement_kind":"exercise","position":1,"done":true,"target_load":"100lb"}]}
	]`)
	client := New(srv.URL, staticTokenClient("t"))

	wid := int64(1)
	sessions, err := client.ListSessions(context.Background(), SessionFilter{From: "2026-07-01", WorkoutID: &wid})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/api/v1/sessions" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	for _, want := range []string{"from=2026-07-01", "workout_id=1"} {
		if !containsParam(got.query, want) {
			t.Errorf("query %q missing %q", got.query, want)
		}
	}
	if len(sessions) != 1 || sessions[0].ID != "018f-aaa" {
		t.Fatalf("decoded = %+v", sessions)
	}
	if len(sessions[0].Movements) != 1 || !sessions[0].Movements[0].Done {
		t.Errorf("movements = %+v", sessions[0].Movements)
	}
}

func TestCreateSession_SendsWorkoutID(t *testing.T) {
	srv, got := recordingServer(t, http.StatusCreated, `{"id":"018f-bbb","workout_id":1,"performed_on":"2026-07-24","movements":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	wid := int64(1)
	if _, err := client.CreateSession(context.Background(), SessionCreate{WorkoutID: &wid, PerformedOn: "2026-07-24"}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/sessions" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["workout_id"] != float64(1) || got.body["performed_on"] != "2026-07-24" {
		t.Errorf("body = %v", got.body)
	}
}

func TestUpdateSessionMovement_PatchesEntry(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"id":"018f-ccc","performed_on":"2026-07-24","movements":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	if _, err := client.UpdateSessionMovement(context.Background(), "018f-ccc", 42,
		map[string]any{"done": true, "actual_load": "100lb"}); err != nil {
		t.Fatalf("UpdateSessionMovement: %v", err)
	}
	if got.method != http.MethodPatch || got.path != "/api/v1/sessions/018f-ccc/movements/42" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["done"] != true || got.body["actual_load"] != "100lb" {
		t.Errorf("body = %v", got.body)
	}
}

func TestDeleteSession_Deletes(t *testing.T) {
	srv, got := recordingServer(t, http.StatusNoContent, ``)
	client := New(srv.URL, staticTokenClient("t"))

	if err := client.DeleteSession(context.Background(), "018f-ddd"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/api/v1/sessions/018f-ddd" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
}

// A bare set POST is the carry-forward path: an empty body is what tells the server to
// repeat the last set, so the client must not invent fields to fill it.
func TestAddSessionSet_PostsAnEmptyBodyByDefault(t *testing.T) {
	srv, got := recordingServer(t, http.StatusCreated,
		`{"id":"018f-aaa","performed_on":"2026-07-24","movements":[
			{"id":4,"movement_id":7,"movement_name":"Bench Press","position":1,"done":true,
			 "sets":[{"id":11,"position":1,"reps":8,"load":"100lb","set_kind":"working"}]}]}`)
	client := New(srv.URL, staticTokenClient("t"))

	session, err := client.AddSessionSet(context.Background(), "018f-aaa", 4, SessionSetInput{})
	if err != nil {
		t.Fatalf("AddSessionSet: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/sessions/018f-aaa/movements/4/sets" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if len(got.body) != 0 {
		t.Errorf("body = %v, want no fields at all", got.body)
	}
	if len(session.Movements) != 1 || len(session.Movements[0].Sets) != 1 {
		t.Fatalf("decoded = %+v", session.Movements)
	}
	set := session.Movements[0].Sets[0]
	if set.ID != 11 || set.Reps == nil || *set.Reps != 8 || set.SetKind != "working" {
		t.Errorf("set = %+v", set)
	}
}

func TestSessionSet_UpdateAndRemoveAddressTheSet(t *testing.T) {
	body := `{"id":"018f-aaa","performed_on":"2026-07-24","movements":[]}`

	srv, got := recordingServer(t, http.StatusOK, body)
	client := New(srv.URL, staticTokenClient("t"))
	if _, err := client.UpdateSessionSet(context.Background(), "018f-aaa", 4, 11, map[string]any{"reps": 6}); err != nil {
		t.Fatalf("UpdateSessionSet: %v", err)
	}
	if got.method != http.MethodPatch || got.path != "/api/v1/sessions/018f-aaa/movements/4/sets/11" {
		t.Errorf("request = %s %s", got.method, got.path)
	}

	srv, got = recordingServer(t, http.StatusOK, body)
	client = New(srv.URL, staticTokenClient("t"))
	if _, err := client.RemoveSessionSet(context.Background(), "018f-aaa", 4, 11); err != nil {
		t.Fatalf("RemoveSessionSet: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/api/v1/sessions/018f-aaa/movements/4/sets/11" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
}

func TestFinishSession_PostsToFinish(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK,
		`{"id":"018f-aaa","performed_on":"2026-07-24","finished_at":"2026-07-24T11:02:00Z","duration_minutes":47,"movements":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	session, err := client.FinishSession(context.Background(), "018f-aaa")
	if err != nil {
		t.Fatalf("FinishSession: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/sessions/018f-aaa/finish" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if session.FinishedAt == nil || session.DurationMinutes == nil || *session.DurationMinutes != 47 {
		t.Errorf("decoded = %+v", session)
	}
}

func TestListSessions_UnfinishedFilter(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `[]`)
	client := New(srv.URL, staticTokenClient("t"))

	if _, err := client.ListSessions(context.Background(), SessionFilter{Unfinished: true}); err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if !containsParam(got.query, "unfinished=true") {
		t.Errorf("query %q missing the unfinished filter", got.query)
	}
}
