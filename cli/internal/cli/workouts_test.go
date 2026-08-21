package cli

import (
	"bytes"
	"errors"
	"slices"
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

// TestMovementLabel checks the value the row is built from: a movement's name
// carries the id `movements show` takes.
func TestMovementLabel(t *testing.T) {
	got := movementLabel(api.WorkoutMovement{MovementName: "Bench Press", MovementID: 7})
	if got != "Bench Press (#7)" {
		t.Errorf("movementLabel = %q, want %q", got, "Bench Press (#7)")
	}
}

// TestPrintWorkoutDetail_MovementHandles checks each row carries both handles:
// the entry id `workouts movements update` takes, and the movement id
// `movements show` takes. Column order and padding are the renderer's business,
// so the row is split on whitespace rather than matched against a fixed width.
func TestPrintWorkoutDetail_MovementHandles(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutDetail(&buf, sampleWorkout())

	row := findLine(t, buf.String(), "Bench Press")
	fields := strings.Fields(row)
	if fields[0] != "4" {
		t.Errorf("row should start with the entry id, got %q in %q", fields[0], row)
	}
	if !strings.Contains(row, "Bench Press (#7)") {
		t.Errorf("row is missing the movement id: %q", row)
	}
}

// TestPrintWorkoutDetail_HeaderColumns pins the header's columns as a sequence of
// names, which is the value the table is built from — the position column is not
// among them because nothing takes a position as an argument.
func TestPrintWorkoutDetail_HeaderColumns(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutDetail(&buf, sampleWorkout())

	got := strings.Fields(findLine(t, buf.String(), "ENTRY"))
	want := []string{"ENTRY", "MOVEMENT", "SETS", "REPS", "LOAD", "REST", "SUPERSET"}
	if !slices.Equal(got, want) {
		t.Errorf("header columns = %v, want %v", got, want)
	}
}

// findLine returns the first line containing needle, failing the test when there
// is none — a missing line otherwise reads as a passing assertion about padding.
func findLine(t *testing.T, out, needle string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("no line containing %q in:\n%s", needle, out)
	return ""
}

func TestPrintWorkoutExpanded(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutExpanded(&buf, sampleWorkout(), sampleLibrary())
	out := buf.String()

	for _, want := range []string{
		"entry 4 — Bench Press (#7)",
		"exercise · weighted · barbell, bench",
		"5 × 5 @ 80% 1RM · rest 120s",
		"Elbows tucked to 45°.",
		"entry 5 — Push-up (#9)",
		"exercise · bodyweight",
		"no prescription",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expanded view missing %q:\n%s", want, out)
		}
	}
	// Muscles and notes are labeled rows, so the label and its value are checked
	// on one line rather than against a padding width.
	for label, value := range map[string]string{
		"primary": "chest", "secondary": "triceps", "notes": "to failure",
	} {
		if row := findLine(t, out, label); !strings.Contains(row, value) {
			t.Errorf("%s row = %q, want it to carry %q", label, row, value)
		}
	}
	// The how-to stays in `movements show`; carrying it here is what turns an
	// expanded workout into a wall.
	if strings.Contains(out, "Lie flat, unrack") {
		t.Errorf("how-to leaked into the expanded view:\n%s", out)
	}
	t.Logf("expanded:\n%s", out)
}

// TestDetailRow_ContinuationLines checks a multi-line value hangs under its
// label rather than resetting to the left margin. It asserts the two lines share
// a left edge, not what that edge measures.
func TestDetailRow_ContinuationLines(t *testing.T) {
	var buf bytes.Buffer
	detailRow(&buf, "cues", "First line.\nSecond line.")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), buf.String())
	}
	first := strings.Index(lines[0], "First line.")
	second := strings.Index(lines[1], "Second line.")
	if first != second {
		t.Errorf("value column moved between lines: %d then %d\n%s", first, second, buf.String())
	}
	if strings.Contains(lines[1], "cues") {
		t.Errorf("label repeated on the continuation line: %q", lines[1])
	}
}

// TestPrintWorkoutExpanded_UnknownMovement covers entries whose movements are
// absent from the index. Each keeps its name and prescription, and the view says
// how many it could not expand rather than reading as complete.
func TestPrintWorkoutExpanded_UnknownMovement(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutExpanded(&buf, sampleWorkout(), map[int64]api.Movement{})
	out := buf.String()

	if !strings.Contains(out, "entry 4 — Bench Press (#7)") || !strings.Contains(out, "5 × 5 @ 80% 1RM") {
		t.Errorf("unknown movement lost its row:\n%s", out)
	}
	if !strings.Contains(out, "2 of 2 entries could not be expanded") {
		t.Errorf("partial view did not say it was partial:\n%s", out)
	}
}

// TestPrintWorkoutExpanded_Complete is the other half: a fully expanded view says
// nothing about missing entries.
func TestPrintWorkoutExpanded_Complete(t *testing.T) {
	var buf bytes.Buffer
	printWorkoutExpanded(&buf, sampleWorkout(), sampleLibrary())
	if strings.Contains(buf.String(), "could not be expanded") {
		t.Errorf("complete view claimed entries were missing:\n%s", buf.String())
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
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view menu missing %q:\n%s", want, out)
		}
	}
	// Every line is offered to be pasted, so a placeholder that cannot be run has
	// no place among them.
	if strings.Contains(out, "<id>") {
		t.Errorf("menu offers an unrunnable placeholder:\n%s", out)
	}
	assertNoReprimand(t, out)
	t.Logf("flag menu:\n%s", out)
}

func TestPlural(t *testing.T) {
	if plural(1) != "" || plural(0) != "s" || plural(2) != "s" {
		t.Error("plural wrong")
	}
}
