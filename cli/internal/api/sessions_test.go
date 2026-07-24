package api

import (
	"context"
	"net/http"
	"testing"
)

func TestListSessions_QueryAndDecode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `[
		{"id":"018f-aaa","workout_id":1,"workout_name":"Push Day","performed_on":"2026-07-24","felt":"strong",
		 "movements":[{"id":4,"movement_id":7,"movement_name":"Bench Press","movement_kind":"exercise","position":1,"done":true,"actual_load":"100lb"}]}
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
