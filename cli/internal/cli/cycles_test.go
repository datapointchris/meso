package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func sampleCycle() api.Cycle {
	metric := "deadlift-working-weight"
	value := 315.0
	start := "2026-08-01"
	target := "2026-10-24"
	week := 1
	phase := "base"
	freq := "3×/week"
	intensity := "easy / Zone 2"
	return api.Cycle{
		ID: 1, Name: "Return to 5k", GoalSummary: "12-week run return", Status: "active",
		TargetMetric: &metric, TargetValue: &value, StartDate: &start, TargetDate: &target,
		Workouts: []api.CycleWorkout{
			{
				ID: 4, WorkoutID: 7, WorkoutName: "Base Week", Position: 1,
				Week: &week, Phase: &phase, Frequency: &freq, Intensity: &intensity,
			},
			{ID: 5, WorkoutID: 9, WorkoutName: "Build Week", Position: 2},
		},
	}
}

func TestPrintCyclesTable(t *testing.T) {
	var buf bytes.Buffer
	printCyclesTable(&buf, []api.Cycle{sampleCycle()})
	out := buf.String()
	for _, want := range []string{"ID", "NAME", "STATUS", "TARGET", "Return to 5k", "active", "deadlift-working-weight", "315"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}

	var empty bytes.Buffer
	printCyclesTable(&empty, nil)
	if !strings.Contains(empty.String(), "No cycles match") {
		t.Errorf("empty message missing: %q", empty.String())
	}
}

func TestPrintCycleDetail(t *testing.T) {
	var buf bytes.Buffer
	printCycleDetail(&buf, sampleCycle())
	out := buf.String()
	for _, want := range []string{"Return to 5k", "#1", "active", "Workouts:", "ENTRY", "Base Week", "base", "3×/week", "Build Week"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}
	// The entry id drives `cycles workouts update/rm`; the workout id drives
	// `workouts show`. Both have to be typeable off the row.
	if !strings.Contains(out, "4      Base Week (#7)") {
		t.Errorf("row is missing an entry id or a workout id:\n%s", out)
	}
	t.Logf("cycle detail:\n%s", out)
}

func TestPrintCycleDetail_Empty(t *testing.T) {
	var buf bytes.Buffer
	printCycleDetail(&buf, api.Cycle{ID: 2, Name: "Draft", Status: "planned"})
	if !strings.Contains(buf.String(), "No workouts yet") {
		t.Errorf("empty-sequence hint missing:\n%s", buf.String())
	}
}

// TestBuildCyclePatch checks the only-changed-fields contract, including clearing a
// nullable date by passing an explicit empty string.
func TestBuildCyclePatch(t *testing.T) {
	var c cycleWriteFlags
	cmd := &cobra.Command{Use: "update", RunE: func(*cobra.Command, []string) error { return nil }}
	bindCycleWriteFlags(cmd, &c)
	cmd.SetArgs([]string{"--status", "active", "--start-date", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	patch := buildCyclePatch(cmd, &c)
	if patch["status"] != "active" {
		t.Errorf("status patch = %v", patch["status"])
	}
	// start-date was passed explicitly (empty) — it must be present, to clear the date.
	if v, ok := patch["start_date"]; !ok || v != "" {
		t.Errorf("start_date patch = %v, ok=%v (want present and empty)", v, ok)
	}
	// goal was never set — it must be absent.
	if _, ok := patch["goal_summary"]; ok {
		t.Errorf("unset goal should be absent, patch = %v", patch)
	}
}

// TestBuildPeriodizationPatch checks the swap via --workout and a passed --phase.
func TestBuildPeriodizationPatch(t *testing.T) {
	var p periodizationFlags
	cmd := &cobra.Command{Use: "update", RunE: func(*cobra.Command, []string) error { return nil }}
	bindPeriodizationFlags(cmd, &p, true)
	cmd.SetArgs([]string{"--workout", "9", "--phase", "taper"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	patch := buildPeriodizationPatch(cmd, &p)
	if patch["workout_id"] != int64(9) {
		t.Errorf("workout_id patch = %v", patch["workout_id"])
	}
	if patch["phase"] != "taper" {
		t.Errorf("phase patch = %v", patch["phase"])
	}
	if _, ok := patch["week"]; ok {
		t.Errorf("unset week should be absent, patch = %v", patch)
	}
}

func TestPrintReview(t *testing.T) {
	felt := "strong"
	name := "Push Day"
	review := api.Review{
		Since:        "2026-05-01",
		ActiveCycles: []api.Cycle{{ID: 1, Name: "Current block", GoalSummary: "peak"}},
		Sessions:     []api.Session{{ID: "x", PerformedOn: "2026-07-20", WorkoutName: &name, Felt: &felt}},
		Measurements: []api.Measurement{{ID: 3, Metric: "5k-time", Value: 1500}},
		LogEntries:   []api.LogEntry{{ID: "y", EntryDate: "2026-07-19", Body: "note"}},
	}
	var buf bytes.Buffer
	printReview(&buf, review)
	out := buf.String()
	for _, want := range []string{"Review since 2026-05-01", "Active cycles: 1", "Current block", "Sessions: 1", "Push Day", "--json"} {
		if !strings.Contains(out, want) {
			t.Errorf("review output missing %q:\n%s", want, out)
		}
	}
}
