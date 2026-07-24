package api

import (
	"context"
	"net/http"
	"testing"
)

func TestListCycles_QueryAndDecode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `[
		{"id":1,"name":"Return to 5k","goal_summary":"12-week run return","status":"active",
		 "target_metric":"5k-time","target_value":1500,"start_date":"2026-08-01","target_date":"2026-10-24",
		 "workouts":[{"id":4,"workout_id":7,"workout_name":"Base Week","position":1,"week":1,"phase":"base"}]}
	]`)
	client := New(srv.URL, staticTokenClient("t"))

	cycles, err := client.ListCycles(context.Background(), CycleFilter{Status: "active", Search: "5k"})
	if err != nil {
		t.Fatalf("ListCycles: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/api/v1/cycles" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	for _, want := range []string{"status=active", "search=5k"} {
		if !containsParam(got.query, want) {
			t.Errorf("query %q missing %q", got.query, want)
		}
	}
	if len(cycles) != 1 || cycles[0].Name != "Return to 5k" {
		t.Fatalf("decoded = %+v", cycles)
	}
	if len(cycles[0].Workouts) != 1 || cycles[0].Workouts[0].WorkoutName != "Base Week" {
		t.Errorf("workouts = %+v", cycles[0].Workouts)
	}
}

func TestCreateCycle_SendsBody(t *testing.T) {
	srv, got := recordingServer(t, http.StatusCreated, `{"id":2,"name":"Shoulder rehab","status":"planned","workouts":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	metric := "5k-time"
	date := "2026-10-24"
	_, err := client.CreateCycle(context.Background(), CycleCreate{
		Name: "Shoulder rehab", GoalSummary: "restore ROM", Status: "active",
		TargetMetric: &metric, TargetDate: &date,
		Workouts: []CycleWorkoutInput{{WorkoutID: 9}},
	})
	if err != nil {
		t.Fatalf("CreateCycle: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/cycles" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["name"] != "Shoulder rehab" || got.body["status"] != "active" || got.body["target_metric"] != "5k-time" {
		t.Errorf("body = %v", got.body)
	}
}

func TestAddCycleWorkout_PostsToSubresource(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"id":1,"name":"Block 1","status":"planned","workouts":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	week := 1
	if _, err := client.AddCycleWorkout(context.Background(), 1, CycleWorkoutInput{WorkoutID: 7, Week: &week}); err != nil {
		t.Fatalf("AddCycleWorkout: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/cycles/1/workouts" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["workout_id"] != float64(7) {
		t.Errorf("body = %v", got.body)
	}
}

func TestUpdateCycleWorkout_PatchesEntry(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"id":1,"name":"Block 1","status":"planned","workouts":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	// A swap: re-point the entry to workout 9.
	if _, err := client.UpdateCycleWorkout(context.Background(), 1, 4, map[string]any{"workout_id": 9}); err != nil {
		t.Fatalf("UpdateCycleWorkout: %v", err)
	}
	if got.method != http.MethodPatch || got.path != "/api/v1/cycles/1/workouts/4" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["workout_id"] != float64(9) {
		t.Errorf("body = %v", got.body)
	}
}

func TestReorderCycleWorkouts_PatchesCollection(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{"id":1,"name":"Block 1","status":"planned","workouts":[]}`)
	client := New(srv.URL, staticTokenClient("t"))

	if _, err := client.ReorderCycleWorkouts(context.Background(), 1, []int64{4, 2, 3}); err != nil {
		t.Fatalf("ReorderCycleWorkouts: %v", err)
	}
	if got.method != http.MethodPatch || got.path != "/api/v1/cycles/1/workouts" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if _, ok := got.body["entry_ids"]; !ok {
		t.Errorf("body missing entry_ids: %v", got.body)
	}
}
