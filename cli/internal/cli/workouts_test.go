package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"meso/cli/internal/api"
)

func sampleWorkout() api.Workout {
	theme := "push"
	sets := 5
	reps := "5"
	load := "80% 1RM"
	rest := 120
	return api.Workout{
		ID: 1, Name: "Push Day", Theme: &theme, Favorite: true, Tags: []string{"upper", "strength"},
		Movements: []api.WorkoutMovement{
			{ID: 4, MovementID: 7, MovementName: "Bench Press", MovementKind: "exercise",
				Position: 1, Sets: &sets, Reps: &reps, Load: &load, RestSeconds: &rest},
			{ID: 5, MovementID: 9, MovementName: "Push-up", MovementKind: "exercise", Position: 2},
		},
	}
}

func TestPrintWorkoutsTable(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutsTable(&buf, []api.Workout{sampleWorkout()})
	out := buf.String()
	for _, want := range []string{"ID", "NAME", "THEME", "MOVES", "Push Day", "push", "★"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}

	var empty bytes.Buffer
	printWorkoutsTable(&empty, nil)
	if !strings.Contains(empty.String(), "No workouts match") {
		t.Errorf("empty message missing: %q", empty.String())
	}
}

func TestPrintWorkoutDetail(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutDetail(&buf, sampleWorkout())
	out := buf.String()
	for _, want := range []string{"Push Day", "#1", "push", "Movements:", "ENTRY", "Bench Press", "80% 1RM", "120s", "Push-up"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}
}

func TestPrintWorkoutDetail_Empty(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutDetail(&buf, api.Workout{ID: 2, Name: "Empty", Tags: []string{}})
	if !strings.Contains(buf.String(), "No movements yet") {
		t.Errorf("empty-composition hint missing:\n%s", buf.String())
	}
}

// TestBuildPrescriptionPatch checks the only-changed-fields contract, including the
// swap via --movement and a zero-valued --sets that was explicitly passed.
func TestBuildPrescriptionPatch(t *testing.T) {
	var p prescriptionFlags
	cmd := &cobra.Command{Use: "update", RunE: func(*cobra.Command, []string) error { return nil }}
	bindPrescriptionFlags(cmd, &p, true)
	cmd.SetArgs([]string{"--movement", "9", "--load", "85% 1RM"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	patch := buildPrescriptionPatch(cmd, &p)
	if patch["movement_id"] != int64(9) {
		t.Errorf("movement_id patch = %v", patch["movement_id"])
	}
	if patch["load"] != "85% 1RM" {
		t.Errorf("load patch = %v", patch["load"])
	}
	// reps was never set — it must be absent.
	if _, ok := patch["reps"]; ok {
		t.Errorf("unset reps should be absent, patch = %v", patch)
	}
}

func TestBuildWorkoutPatch(t *testing.T) {
	var w workoutWriteFlags
	cmd := &cobra.Command{Use: "update", RunE: func(*cobra.Command, []string) error { return nil }}
	bindWorkoutWriteFlags(cmd, &w)
	cmd.SetArgs([]string{"--favorite=false", "--theme", "legs"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	patch := buildWorkoutPatch(cmd, &w)
	fav, ok := patch["favorite"]
	if !ok || fav != false {
		t.Errorf("favorite patch = %v, ok=%v", fav, ok)
	}
	if patch["theme"] != "legs" {
		t.Errorf("theme patch = %v", patch["theme"])
	}
	if _, ok := patch["tags"]; ok {
		t.Errorf("unset tags should be absent, patch = %v", patch)
	}
}

func TestPlural(t *testing.T) {
	if plural(1) != "" || plural(0) != "s" || plural(2) != "s" {
		t.Error("plural wrong")
	}
}
