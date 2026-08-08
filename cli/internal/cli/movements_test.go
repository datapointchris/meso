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

func TestPrintMusclesTable(t *testing.T) {
	var buf bytes.Buffer
	printMusclesTable(&buf, []api.Muscle{
		{Name: "lats", Region: "posterior"},
		{Name: "front_delts", Region: "anterior"},
	})
	out := buf.String()
	for _, want := range []string{"MUSCLE", "REGION", "lats", "posterior", "front_delts"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}

	var empty bytes.Buffer
	printMusclesTable(&empty, nil)
	if !strings.Contains(empty.String(), "No muscles defined") {
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
// Rename is update-only: on create the name is the positional argument, so a --name
// flag there would be two ways to say the same thing.
func TestBuildMovementPatch_Rename(t *testing.T) {
	cmd := newMovementsUpdateCommand()
	cmd.SetArgs([]string{"1", "--name", "Straight-Arm Pulldown"})
	if cmd.Flags().Lookup("name") == nil {
		t.Fatal("update should expose --name")
	}
	if newMovementsCreateCommand().Flags().Lookup("name") != nil {
		t.Error("create should not expose --name — the name is positional there")
	}

	var w movementWriteFlags
	probe := &cobra.Command{Use: "update", RunE: func(*cobra.Command, []string) error { return nil }}
	bindMovementWriteFlags(probe, &w)
	probe.Flags().StringVar(&w.name, "name", "", "")
	probe.SetArgs([]string{"--name", "Straight-Arm Pulldown"})
	if err := probe.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	patch, err := buildMovementPatch(probe, &w)
	if err != nil {
		t.Fatalf("buildMovementPatch: %v", err)
	}
	if patch["name"] != "Straight-Arm Pulldown" {
		t.Errorf("name patch = %v", patch["name"])
	}
	if len(patch) != 1 {
		t.Errorf("only the changed flag belongs in the patch, got %v", patch)
	}
}

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

func TestReadConfirmation(t *testing.T) {
	answered := func(input string) bool {
		var out bytes.Buffer
		got, err := readConfirmation(&out, strings.NewReader(input), "Delete?")
		if err != nil {
			t.Fatalf("readConfirmation(%q): %v", input, err)
		}
		if !strings.Contains(out.String(), "Delete? [y/N]") {
			t.Errorf("prompt = %q, want the question and the default-no hint", out.String())
		}
		return got
	}

	if !answered("y\n") {
		t.Error("y should confirm")
	}
	if answered("\n") {
		t.Error("empty should not confirm (default no)")
	}
	if answered("n\n") {
		t.Error("n should not confirm")
	}
	if answered("") {
		t.Error("EOF should decline rather than error")
	}
}

// newConfirmTarget is a command with the streams a test controls. Its stdin is a
// buffer rather than a terminal, which is exactly the caller confirm must refuse
// to prompt.
func newConfirmTarget(stdin string) (*cobra.Command, *bytes.Buffer) {
	var errOut bytes.Buffer
	cmd := &cobra.Command{Use: "target"}
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetErr(&errOut)
	cmd.SetOut(&bytes.Buffer{})
	return cmd, &errOut
}

func TestConfirm_RefusesToPromptWithoutATerminal(t *testing.T) {
	noInput = false
	cmd, errOut := newConfirmTarget("y\n")

	ok, err := confirm(cmd, "Delete everything?")

	if err == nil {
		t.Fatal("confirm returned no error — a delete would report itself aborted instead of saying --yes was needed")
	}
	if ok {
		t.Error("confirm approved without a terminal")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error = %q, want it to name the flag that would have answered", err)
	}
	if errOut.Len() != 0 {
		t.Errorf("prompt was written anyway: %q", errOut.String())
	}
}

func TestInteractive_NoInputRefusesEvenOnATerminal(t *testing.T) {
	cmd, _ := newConfirmTarget("")

	noInput = true
	defer func() { noInput = false }()

	if interactive(cmd) {
		t.Error("--no-input must force the non-interactive path")
	}
}

func TestNoInputFlag_IsPersistentAndDefaultsOff(t *testing.T) {
	noInput = true // a previous command's value must not survive

	root := NewRootCommand()

	flag := root.PersistentFlags().Lookup("no-input")
	if flag == nil {
		t.Fatal("--no-input is not registered on the root command")
	}
	if noInput {
		t.Error("noInput survived NewRootCommand — a test would inherit the previous value")
	}
	if flag.DefValue != "false" {
		t.Errorf("--no-input default = %q, want false: interactivity is the terminal default", flag.DefValue)
	}
}
