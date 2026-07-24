package api

import (
	"context"
	"net/http"
	"testing"
)

func TestListWorkouts_QueryAndDecode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `[
		{"id":1,"name":"Push Day","theme":"push","tags":["upper"],"favorite":true,
		 "movements":[{"id":4,"movement_id":7,"movement_name":"Bench Press","movement_kind":"exercise","position":1,"sets":5,"reps":"5"}]}
	]`)
	client := New(srv.URL, staticTokenClient("t"))

	fav := true
	workouts, err := client.ListWorkouts(context.Background(), WorkoutFilter{Theme: "push", Favorite: &fav})
	if err != nil {
		t.Fatalf("ListWorkouts: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/api/v1/workouts" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	for _, want := range []string{"theme=push", "favorite=true"} {
		if !containsParam(got.query, want) {
			t.Errorf("query %q missing %q", got.query, want)
		}
	}
	if len(workouts) != 1 || workouts[0].Name != "Push Day" {
		t.Fatalf("decoded = %+v", workouts)
	}
	if len(workouts[0].Movements) != 1 || workouts[0].Movements[0].MovementName != "Bench Press" {
		t.Errorf("movements = %+v", workouts[0].Movements)
	}
}

func TestCreateWorkout_SendsBody(t *testing.T) {
	srv, got := recordingServer(t, http.StatusCreated, `{"id":2,"name":"Leg Day","movements":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	theme := "legs"
	_, err := client.CreateWorkout(context.Background(), WorkoutCreate{
		Name: "Leg Day", Theme: &theme, Tags: []string{"lower"},
		Movements: []WorkoutMovementInput{{MovementID: 9}},
	})
	if err != nil {
		t.Fatalf("CreateWorkout: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/workouts" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["name"] != "Leg Day" || got.body["theme"] != "legs" {
		t.Errorf("body = %v", got.body)
	}
}

func TestAddWorkoutMovement_PostsToSubresource(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"id":1,"name":"Push Day","movements":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	sets := 5
	if _, err := client.AddWorkoutMovement(context.Background(), 1, WorkoutMovementInput{MovementID: 7, Sets: &sets}); err != nil {
		t.Fatalf("AddWorkoutMovement: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/workouts/1/movements" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["movement_id"] != float64(7) {
		t.Errorf("body = %v", got.body)
	}
}

func TestUpdateWorkoutMovement_PatchesEntry(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"id":1,"name":"Push Day","movements":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	// A swap: re-point the entry to movement 9.
	if _, err := client.UpdateWorkoutMovement(context.Background(), 1, 4, map[string]any{"movement_id": 9}); err != nil {
		t.Fatalf("UpdateWorkoutMovement: %v", err)
	}
	if got.method != http.MethodPatch || got.path != "/api/v1/workouts/1/movements/4" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["movement_id"] != float64(9) {
		t.Errorf("body = %v", got.body)
	}
}

func TestReorderWorkoutMovements_PatchesCollection(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"id":1,"name":"Push Day","movements":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	if _, err := client.ReorderWorkoutMovements(context.Background(), 1, []int64{4, 2, 3}); err != nil {
		t.Fatalf("ReorderWorkoutMovements: %v", err)
	}
	if got.method != http.MethodPatch || got.path != "/api/v1/workouts/1/movements" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if _, ok := got.body["entry_ids"]; !ok {
		t.Errorf("body missing entry_ids: %v", got.body)
	}
}

func TestAddRelated_PostsToSubresource(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"id":3,"name":"Barbell Row","related":[{"id":8,"name":"Dumbbell Row","movement_kind":"exercise","relationship_kind":"alternate"}]}`)
	client := New(srv.URL, staticTokenClient("t"))

	m, err := client.AddRelated(context.Background(), 3, RelationshipInput{RelatedMovementID: 8, RelationshipKind: "alternate"})
	if err != nil {
		t.Fatalf("AddRelated: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/movements/3/related" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if len(m.Related) != 1 || m.Related[0].Name != "Dumbbell Row" {
		t.Errorf("related = %+v", m.Related)
	}
}

func TestRemoveRelated_DeletesWithKind(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"id":3,"name":"Barbell Row","related":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	if _, err := client.RemoveRelated(context.Background(), 3, 8, "alternate"); err != nil {
		t.Fatalf("RemoveRelated: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/api/v1/movements/3/related/8" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if !containsParam(got.query, "kind=alternate") {
		t.Errorf("query %q missing kind=alternate", got.query)
	}
}
