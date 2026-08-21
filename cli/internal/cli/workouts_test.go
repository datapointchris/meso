package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
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
			{
				ID: 4, MovementID: 7, MovementName: "Bench Press", MovementKind: "exercise",
				Position: 1, Sets: &sets, Reps: &reps, Load: &load, RestSeconds: &rest,
			},
			{ID: 5, MovementID: 9, MovementName: "Push-up", MovementKind: "exercise", Position: 2, Notes: "to failure"},
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

// sampleLibrary is the movement index --detailed expands entries from, keyed the
// way the show command builds it.
func sampleLibrary() map[int64]api.Movement {
	return map[int64]api.Movement{
		7: {
			ID: 7, Name: "Bench Press", MovementKind: "exercise", LoadMode: "weighted",
			Equipment: []string{"barbell", "bench"},
			Muscles: []api.MovementMuscle{
				{Muscle: "chest", Region: "anterior", Role: "primary"},
				{Muscle: "triceps", Region: "anterior", Role: "secondary"},
			},
			FormCues: "Elbows tucked to 45°.\nDrive the bar back over the shoulders.",
			HowTo:    "Lie flat, unrack, lower to the sternum, press.",
		},
		9: {
			ID: 9, Name: "Push-up", MovementKind: "exercise", LoadMode: "bodyweight",
			Muscles: []api.MovementMuscle{{Muscle: "chest", Region: "anterior", Role: "primary"}},
		},
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
	t.Logf("compact:\n%s", out)
}

// TestPrintWorkoutDetail_MovementHandles is the reason the position column went:
// every row has to carry the entry id that `workouts movements update` takes and
// the movement id that `movements show` takes, and two bare integer columns beside
// each other is what made them unreadable.
func TestPrintWorkoutDetail_MovementHandles(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutDetail(&buf, sampleWorkout())
	out := buf.String()

	if !strings.Contains(out, "Bench Press (#7)") {
		t.Errorf("movement id missing from the row:\n%s", out)
	}
	if !strings.Contains(out, "4      Bench Press (#7)") {
		t.Errorf("entry id missing from the row:\n%s", out)
	}
	var header string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "ENTRY") {
			header = line
			break
		}
	}
	if header == "" {
		t.Fatalf("no movements header at all:\n%s", out)
	}
	if strings.HasPrefix(strings.TrimSpace(header), "#") {
		t.Errorf("position column survived: %q", header)
	}
}

func TestPrintWorkoutDetailed(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutDetailed(&buf, sampleWorkout(), sampleLibrary())
	out := buf.String()

	for _, want := range []string{
		"entry 4 — Bench Press (#7)",
		"exercise · weighted · barbell, bench",
		"5 × 5 @ 80% 1RM · rest 120s",
		"primary     chest",
		"secondary   triceps",
		"Elbows tucked to 45°.",
		"entry 5 — Push-up (#9)",
		"exercise · bodyweight",
		"no prescription",
		"notes       to failure",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("detailed missing %q:\n%s", want, out)
		}
	}
	// The how-to stays in `movements show`; carrying it here is what turns a
	// detailed workout into a wall.
	if strings.Contains(out, "Lie flat, unrack") {
		t.Errorf("how-to leaked into the detailed view:\n%s", out)
	}
	t.Logf("detailed:\n%s", out)
}

// TestPrintWorkoutDetailed_ContinuationLines checks a multi-line cue block hangs
// under its label rather than resetting to the left margin.
func TestPrintWorkoutDetailed_ContinuationLines(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutDetailed(&buf, sampleWorkout(), sampleLibrary())
	if !strings.Contains(buf.String(), "                Drive the bar back over the shoulders.") {
		t.Errorf("continuation line not hung under its label:\n%s", buf.String())
	}
}

// TestPrintWorkoutDetailed_UnknownMovement covers an entry whose movement is
// absent from the index: the name and prescription still print.
func TestPrintWorkoutDetailed_UnknownMovement(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutDetailed(&buf, sampleWorkout(), map[int64]api.Movement{})
	out := buf.String()
	if !strings.Contains(out, "entry 4 — Bench Press (#7)") || !strings.Contains(out, "5 × 5 @ 80% 1RM") {
		t.Errorf("unknown movement lost its row:\n%s", out)
	}
}

func TestPrescriptionLine(t *testing.T) {
	sets, reps, load, rest := 5, "5", "80% 1RM", 120
	group := "A"
	cases := []struct {
		name string
		in   api.WorkoutMovement
		want string
	}{
		{
			"full",
			api.WorkoutMovement{Sets: &sets, Reps: &reps, Load: &load, RestSeconds: &rest, SupersetGroup: &group},
			"5 × 5 @ 80% 1RM · rest 120s · superset A",
		},
		{"sets only", api.WorkoutMovement{Sets: &sets}, "5 sets"},
		{"reps only", api.WorkoutMovement{Reps: &reps}, "5"},
		{"load only", api.WorkoutMovement{Load: &load}, "80% 1RM"},
		{"empty", api.WorkoutMovement{}, "no prescription"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := prescriptionLine(tc.in); got != tc.want {
				t.Errorf("prescriptionLine = %q, want %q", got, tc.want)
			}
		})
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

// TestWorkoutsShow_DetailedWithJSON checks the two output flags do not silently
// pick a winner. --detailed is a reading layout the API has no shape for, so the
// caller gets the three views spelled out as commands rather than a refusal.
func TestWorkoutsShow_DetailedWithJSON(t *testing.T) {
	// Through the real root, because SilenceErrors lives there — a bare subcommand
	// would let cobra print its own "Error:" line above the menu.
	var stderr bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{"workouts", "show", "1", "--detailed", "--json"})

	err := root.Execute()
	var ec exitCode
	if !errors.As(err, &ec) || ec != 2 {
		t.Fatalf("want exitCode(2) so Execute prints no error line, got %T: %v", err, err)
	}
	out := stderr.String()
	for _, want := range []string{
		"meso workouts show 1 --detailed",
		"meso workouts show 1 --json",
		"meso movements show <id> --json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view menu missing %q:\n%s", want, out)
		}
	}
	assertNoReprimand(t, out)
	t.Logf("flag menu:\n%s", out)
}

// TestResolveWorkoutArg_NumericFastPath pins that a numeric argument resolves with
// no client call at all — passing nil would panic if it reached the API.
func TestResolveWorkoutArg_NumericFastPath(t *testing.T) {
	id, err := resolveWorkoutArg(context.Background(), io.Discard, nil, "12")
	if err != nil {
		t.Fatalf("resolveWorkoutArg: %v", err)
	}
	if id != 12 {
		t.Errorf("id = %d, want 12", id)
	}
}

func TestPlural(t *testing.T) {
	if plural(1) != "" || plural(0) != "s" || plural(2) != "s" {
		t.Error("plural wrong")
	}
}
