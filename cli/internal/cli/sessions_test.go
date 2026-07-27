package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func sampleSession() api.Session {
	name := "Push Day"
	felt := "strong"
	sets := 5
	reps := "5"
	load := "100lb"
	wid := int64(1)
	return api.Session{
		ID: "018f-aaa", WorkoutID: &wid, WorkoutName: &name, PerformedOn: "2026-07-24", Felt: &felt,
		Movements: []api.SessionMovement{
			{
				ID: 4, MovementID: 7, MovementName: "Bench Press", MovementKind: "exercise",
				Position: 1, Done: true, ActualSets: &sets, ActualReps: &reps, ActualLoad: &load,
			},
			{ID: 5, MovementID: 9, MovementName: "Push-up", MovementKind: "exercise", Position: 2},
		},
	}
}

func TestPrintSessionsTable(t *testing.T) {
	var buf bytes.Buffer
	printSessionsTable(&buf, []api.Session{sampleSession()})
	out := buf.String()
	for _, want := range []string{"DATE", "WORKOUT", "DONE", "2026-07-24", "Push Day", "1/2", "strong"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}

	var empty bytes.Buffer
	printSessionsTable(&empty, nil)
	if !strings.Contains(empty.String(), "No sessions match") {
		t.Errorf("empty message missing: %q", empty.String())
	}
}

func TestPrintSessionDetail(t *testing.T) {
	var buf bytes.Buffer
	printSessionDetail(&buf, sampleSession())
	out := buf.String()
	for _, want := range []string{"Session on 2026-07-24", "018f-aaa", "Push Day", "Movements:", "ENTRY", "Bench Press", "100lb", "✓", "Push-up"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}
}

// TestBuildSessionMovementPatch checks the only-changed-fields contract, including a
// swap via --movement and an explicit --done.
func TestBuildSessionMovementPatch(t *testing.T) {
	var a sessionActualFlags
	cmd := &cobra.Command{Use: "update", RunE: func(*cobra.Command, []string) error { return nil }}
	f := cmd.Flags()
	f.BoolVar(&a.done, "done", false, "")
	f.IntVar(&a.sets, "sets", 0, "")
	f.StringVar(&a.reps, "reps", "", "")
	f.StringVar(&a.load, "load", "", "")
	f.StringVar(&a.notes, "notes", "", "")
	f.Int64Var(&a.movement, "movement", 0, "")
	cmd.SetArgs([]string{"--done", "--load", "100lb", "--movement", "9"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	patch := buildSessionMovementPatch(cmd, &a)
	if patch["done"] != true {
		t.Errorf("done patch = %v", patch["done"])
	}
	if patch["actual_load"] != "100lb" {
		t.Errorf("actual_load patch = %v", patch["actual_load"])
	}
	if patch["movement_id"] != int64(9) {
		t.Errorf("movement_id patch = %v", patch["movement_id"])
	}
	// reps never set — must be absent.
	if _, ok := patch["actual_reps"]; ok {
		t.Errorf("unset reps should be absent, patch = %v", patch)
	}
}

func TestDoneGlyph(t *testing.T) {
	if doneGlyph(true) != "✓" || doneGlyph(false) != "·" {
		t.Error("doneGlyph wrong")
	}
}
