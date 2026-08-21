package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func ptr[T any](v T) *T { return &v }

func sampleSession() api.Session {
	name := "Push Day"
	felt := "strong"
	wid := int64(1)
	return api.Session{
		ID: "018f-aaa", WorkoutID: &wid, WorkoutName: &name, PerformedOn: "2026-07-24", Felt: &felt,
		Movements: []api.SessionMovement{
			{
				ID: 4, MovementID: 7, MovementName: "Bench Press", MovementKind: "exercise",
				LoadMode: "weighted", Position: 1, Done: true,
				TargetSets: ptr(3), TargetReps: ptr("5"), TargetLoad: ptr("95lb"),
				Sets: []api.SessionSet{
					{ID: 11, Position: 1, Reps: ptr(5), Load: ptr("100lb"), SetKind: "working"},
					{ID: 12, Position: 2, Reps: ptr(5), Load: ptr("100lb"), SetKind: "working"},
					{ID: 13, Position: 3, Reps: ptr(3), Load: ptr("110lb"), SetKind: "failure"},
				},
			},
			{ID: 5, MovementID: 9, MovementName: "Push-up", MovementKind: "exercise", LoadMode: "bodyweight", Position: 2},
		},
	}
}

func TestPrintSessionsTable(t *testing.T) {
	var buf bytes.Buffer
	printSessionsTable(&buf, []api.Session{sampleSession()})
	out := buf.String()
	// The row says what happened and whether it is still open — not a score.
	for _, want := range []string{"DATE", "WORKOUT", "MOVEMENTS", "SETS", "STATUS", "2026-07-24", "Push Day", "in progress", "strong"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}

	finished := sampleSession()
	finished.FinishedAt = ptr("2026-07-24T11:00:00Z")
	var done bytes.Buffer
	printSessionsTable(&done, []api.Session{finished})
	if !strings.Contains(done.String(), "finished") {
		t.Errorf("finished session should say so:\n%s", done.String())
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
	for _, want := range []string{
		"Session on 2026-07-24", "018f-aaa", "Push Day", "in progress",
		"Movements:", "ENTRY", "PERFORMED", "TARGET", "Bench Press", "✓", "Push-up",
		// The per-set list, so a set id is reachable for `sessions set update/rm`.
		"Bench Press (entry 4)", "failure", "110lb",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}
	// The entry id drives `sessions movements update/rm`; the movement id drives
	// `movements show`. Both have to be typeable off the row.
	if !strings.Contains(out, "Bench Press (#7)") {
		t.Errorf("movement id missing from the row:\n%s", out)
	}
	t.Logf("session detail:\n%s", out)
}

// The performed cell collapses sets that shared their numbers and spells out the ones
// that did not, so three identical sets do not read as three separate facts.
func TestFormatPerformed(t *testing.T) {
	cases := []struct {
		name string
		sets []api.SessionSet
		want string
	}{
		{"nothing performed", nil, "—"},
		{
			"all alike",
			[]api.SessionSet{{Reps: ptr(8), Load: ptr("100lb")}, {Reps: ptr(8), Load: ptr("100lb")}},
			"2 × 8 · 100lb",
		},
		{
			"a drop set",
			[]api.SessionSet{{Reps: ptr(8), Load: ptr("100lb")}, {Reps: ptr(5), Load: ptr("85lb")}},
			"2 × 8/5 · 100lb/85lb",
		},
		{"a timed hold", []api.SessionSet{{HoldSeconds: ptr(30)}}, "1 × 30s"},
		{"nothing measured", []api.SessionSet{{}, {}}, "2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatPerformed(api.SessionMovement{Sets: tc.sets})
			if got != tc.want {
				t.Errorf("formatPerformed = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatTarget(t *testing.T) {
	if got := formatTarget(api.SessionMovement{}); got != "—" {
		t.Errorf("a free-form entry has no target, got %q", got)
	}
	got := formatTarget(api.SessionMovement{TargetSets: ptr(3), TargetReps: ptr("8–10"), TargetLoad: ptr("80% 1RM")})
	if got != "3 × 8–10 · 80% 1RM" {
		t.Errorf("formatTarget = %q", got)
	}
}

func TestFormatPrevious(t *testing.T) {
	if got := formatPrevious(nil); got != "—" {
		t.Errorf("never performed = %q", got)
	}
	got := formatPrevious(&api.PreviousActuals{Sets: 3, Reps: ptr(8), Load: ptr("100lb"), PerformedOn: "2026-07-17"})
	if got != "3 × 8 · 100lb (2026-07-17)" {
		t.Errorf("formatPrevious = %q", got)
	}
}

// TestBuildSessionMovementPatch checks the only-changed-fields contract, including a
// swap via --movement and an explicit --done.
func TestBuildSessionMovementPatch(t *testing.T) {
	var a sessionTargetFlags
	cmd := &cobra.Command{Use: "update", RunE: func(*cobra.Command, []string) error { return nil }}
	f := cmd.Flags()
	f.BoolVar(&a.done, "done", false, "")
	f.IntVar(&a.sets, "target-sets", 0, "")
	f.StringVar(&a.reps, "target-reps", "", "")
	f.StringVar(&a.load, "target-load", "", "")
	f.StringVar(&a.notes, "notes", "", "")
	f.Int64Var(&a.movement, "movement", 0, "")
	cmd.SetArgs([]string{"--done", "--target-load", "100lb", "--movement", "9"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	patch := buildSessionMovementPatch(cmd, &a)
	if patch["done"] != true {
		t.Errorf("done patch = %v", patch["done"])
	}
	if patch["target_load"] != "100lb" {
		t.Errorf("target_load patch = %v", patch["target_load"])
	}
	if patch["movement_id"] != int64(9) {
		t.Errorf("movement_id patch = %v", patch["movement_id"])
	}
	// reps never set — must be absent.
	if _, ok := patch["target_reps"]; ok {
		t.Errorf("unset reps should be absent, patch = %v", patch)
	}
}

func TestDoneGlyph(t *testing.T) {
	if doneGlyph(true) != "✓" || doneGlyph(false) != "·" {
		t.Error("doneGlyph wrong")
	}
}
