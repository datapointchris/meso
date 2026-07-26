package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func sampleMovements() []api.Movement {
	reps := "4–6"
	sets := 3
	return []api.Movement{
		{
			ID: 1, Name: "Barbell Deadlift", MovementKind: "exercise", Favorite: true,
			Tags: []string{"strength", "posterior-chain"}, Equipment: []string{"barbell"},
			DefaultSets: &sets, DefaultReps: &reps,
			Muscles: []api.MovementMuscle{
				{Muscle: "hamstrings", Region: "posterior", Role: "primary"},
				{Muscle: "quads", Region: "anterior", Role: "secondary"},
			},
			HowTo: "hinge at the hips", FormCues: "flat back", CommonFaults: "rounding",
		},
		{ID: 2, Name: "Child's Pose", MovementKind: "yoga_pose"},
	}
}

func TestParseMuscleFlags(t *testing.T) {
	got, err := parseMuscleFlags([]string{"quads:primary", "glutes:secondary", "hamstrings"})
	if err != nil {
		t.Fatalf("parseMuscleFlags: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	// Bare name defaults to primary.
	if got[2].Muscle != "hamstrings" || got[2].Role != "primary" {
		t.Errorf("default role wrong: %+v", got[2])
	}
	if got[1].Role != "secondary" {
		t.Errorf("explicit role wrong: %+v", got[1])
	}

	if _, err := parseMuscleFlags([]string{"quads:tertiary"}); err == nil {
		t.Error("expected error for invalid role")
	}
	if _, err := parseMuscleFlags([]string{":primary"}); err == nil {
		t.Error("expected error for empty muscle name")
	}
}

func TestPrintMovementsTable(t *testing.T) {
	var buf bytes.Buffer
	printMovementsTable(&buf, sampleMovements())
	out := buf.String()
	for _, want := range []string{"ID", "PRIMARY MUSCLES", "Barbell Deadlift", "hamstrings", "★", "Child's Pose"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}

	var empty bytes.Buffer
	printMovementsTable(&empty, nil)
	if !strings.Contains(empty.String(), "No movements match") {
		t.Errorf("empty message missing: %q", empty.String())
	}
}

func TestPrintMovementDetail(t *testing.T) {
	var buf bytes.Buffer
	printMovementDetail(&buf, sampleMovements()[0])
	out := buf.String()
	for _, want := range []string{"Barbell Deadlift", "#1", "exercise", "hamstrings (primary, posterior)", "How to", "Common faults", "3 sets × 4–6"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}
}

func TestWriteMovementsCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := writeMovementsCSV(&buf, sampleMovements()); err != nil {
		t.Fatalf("writeMovementsCSV: %v", err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 { // header + 2 rows
		t.Fatalf("got %d CSV lines, want 3:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "id,name,kind,favorite") {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.Contains(lines[1], "Barbell Deadlift") || !strings.Contains(lines[1], "hamstrings") {
		t.Errorf("row = %q", lines[1])
	}
}

// TestBuildMovementPatch checks the only-changed-fields contract: unset flags are
// absent from the patch, and set flags (including a false bool) are present.
func TestBuildMovementPatch(t *testing.T) {
	var w movementWriteFlags
	cmd := &cobra.Command{Use: "update", RunE: func(*cobra.Command, []string) error { return nil }}
	bindMovementWriteFlags(cmd, &w)
	cmd.SetArgs([]string{"--favorite=false", "--tag", "mobility", "--muscle", "glutes:primary"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	patch, err := buildMovementPatch(cmd, &w)
	if err != nil {
		t.Fatalf("buildMovementPatch: %v", err)
	}
	// favorite was explicitly set to false — it must be present as false.
	fav, ok := patch["favorite"]
	if !ok || fav != false {
		t.Errorf("favorite patch = %v, ok=%v", fav, ok)
	}
	if _, ok := patch["tags"]; !ok {
		t.Error("tags should be in patch")
	}
	if _, ok := patch["muscles"]; !ok {
		t.Error("muscles should be in patch")
	}
	// kind was never set — it must be absent.
	if _, ok := patch["movement_kind"]; ok {
		t.Errorf("unset kind should be absent, patch = %v", patch)
	}
}

func TestRenderHelpers(t *testing.T) {
	if orDash("") != "—" || orDash("x") != "x" {
		t.Error("orDash wrong")
	}
	s := "hi"
	if orDashPtr(nil) != "—" || orDashPtr(&s) != "hi" {
		t.Error("orDashPtr wrong")
	}
	n := 3
	if orDashIntPtr(nil) != "—" || orDashIntPtr(&n) != "3" {
		t.Error("orDashIntPtr wrong")
	}
	if yesNo(true) != "★" || yesNo(false) != "" {
		t.Error("yesNo wrong")
	}
}

func TestConfirm(t *testing.T) {
	var out bytes.Buffer
	if !confirm(strings.NewReader("y\n"), &out, "Delete?") {
		t.Error("y should confirm")
	}
	if confirm(strings.NewReader("\n"), &out, "Delete?") {
		t.Error("empty should not confirm (default no)")
	}
	if confirm(strings.NewReader("n\n"), &out, "Delete?") {
		t.Error("n should not confirm")
	}
}
