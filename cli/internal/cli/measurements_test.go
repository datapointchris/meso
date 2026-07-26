package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/datapointchris/meso/cli/internal/api"
)

func ptrFloat(v float64) *float64 { return &v }

func TestFormatValue(t *testing.T) {
	cases := map[float64]string{185: "185", 202.5: "202.5", 0: "0", -150: "-150"}
	for in, want := range cases {
		if got := formatValue(in); got != want {
			t.Errorf("formatValue(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestPrintMeasurementsTable(t *testing.T) {
	var buf bytes.Buffer
	printMeasurementsTable(&buf, []api.Measurement{
		{ID: 2, Metric: "deadlift-working-weight", Value: 225, MeasuredOn: "2026-07-22", Source: "manual"},
		{ID: 1, Metric: "deadlift-working-weight", Value: 202.5, MeasuredOn: "2026-07-15", Source: "manual", Notes: "RPE 8"},
	})
	out := buf.String()
	for _, want := range []string{"DATE", "METRIC", "VALUE", "2026-07-22", "225", "202.5", "RPE 8"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}

	var empty bytes.Buffer
	printMeasurementsTable(&empty, nil)
	if !strings.Contains(empty.String(), "No measurements match") {
		t.Errorf("empty message missing: %q", empty.String())
	}
}

// TestChangeSummary checks the direction-aware verdict: a bigger lift and a smaller
// 5k time both read as "improved".
func TestChangeSummary(t *testing.T) {
	up := changeSummary(api.MetricTrend{Direction: "higher_better", Unit: "lb", Change: ptrFloat(40)})
	if !strings.Contains(up, "improved") || !strings.Contains(up, "+40") {
		t.Errorf("higher_better +40 = %q", up)
	}
	fasterFive := changeSummary(api.MetricTrend{Direction: "lower_better", Unit: "seconds", Change: ptrFloat(-150)})
	if !strings.Contains(fasterFive, "improved") || !strings.Contains(fasterFive, "−150") {
		t.Errorf("lower_better −150 = %q", fasterFive)
	}
	worse := changeSummary(api.MetricTrend{Direction: "higher_better", Unit: "lb", Change: ptrFloat(-10)})
	if !strings.Contains(worse, "worse") {
		t.Errorf("higher_better −10 = %q", worse)
	}
	flat := changeSummary(api.MetricTrend{Direction: "higher_better", Unit: "lb", Change: ptrFloat(0)})
	if !strings.Contains(flat, "no change") {
		t.Errorf("zero change = %q", flat)
	}
}

func TestPrintTrend(t *testing.T) {
	var buf bytes.Buffer
	printTrend(&buf, api.MetricTrend{
		Metric: "5k-time", Unit: "seconds", Direction: "lower_better", Category: "cardio",
		Points: []api.TrendPoint{
			{MeasuredOn: "2026-07-01", Value: 1800},
			{MeasuredOn: "2026-07-20", Value: 1700},
			{MeasuredOn: "2026-07-27", Value: 1650},
		},
		First: ptrFloat(1800), Latest: ptrFloat(1650), Change: ptrFloat(-150), Count: 3,
	})
	out := buf.String()
	for _, want := range []string{"5k-time", "3 readings", "first 1800", "latest 1650", "improved", "2026-07-27"} {
		if !strings.Contains(out, want) {
			t.Errorf("trend missing %q:\n%s", want, out)
		}
	}

	var empty bytes.Buffer
	printTrend(&empty, api.MetricTrend{Metric: "toe-reach", Unit: "cm", Direction: "higher_better", Count: 0})
	if !strings.Contains(empty.String(), "No readings yet") {
		t.Errorf("empty trend missing hint: %q", empty.String())
	}
}

func TestSparkline(t *testing.T) {
	// A rising then falling series uses the low and high glyphs at the extremes.
	spark := sparkline([]api.TrendPoint{{Value: 1}, {Value: 5}, {Value: 3}, {Value: 8}})
	runes := []rune(spark)
	if len(runes) != 4 {
		t.Fatalf("sparkline length = %d (%q)", len(runes), spark)
	}
	if runes[0] != '▁' || runes[3] != '█' {
		t.Errorf("sparkline extremes = %q", spark)
	}
	// A flat or single-point series has no shape.
	if sparkline([]api.TrendPoint{{Value: 5}, {Value: 5}}) != "" {
		t.Error("flat series should render no sparkline")
	}
	if sparkline([]api.TrendPoint{{Value: 5}}) != "" {
		t.Error("single point should render no sparkline")
	}
}

func TestPrintMetricsTable(t *testing.T) {
	var buf bytes.Buffer
	printMetricsTable(&buf, []api.MetricDefinition{
		{Name: "5k-time", Unit: "seconds", Direction: "lower_better", Category: "cardio"},
	})
	out := buf.String()
	for _, want := range []string{"NAME", "5k-time", "lower_better", "cardio"} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics table missing %q:\n%s", want, out)
		}
	}

	var empty bytes.Buffer
	printMetricsTable(&empty, nil)
	if !strings.Contains(empty.String(), "No metrics defined") {
		t.Errorf("empty message missing: %q", empty.String())
	}
}

func TestPrintMetricDetail(t *testing.T) {
	var documented bytes.Buffer
	printMetricDetail(&documented, api.MetricDefinition{
		Name: "heel-raise-capacity-right", Label: "Heel Raise Capacity Right",
		Unit: "reps", Direction: "higher_better", Category: "mobility",
		HowToMeasure: "Single-leg heel raises to failure on flat ground.",
	})
	out := documented.String()
	for _, want := range []string{
		"Heel Raise Capacity Right", "heel-raise-capacity-right", "reps",
		"higher_better", "mobility", "How to measure",
		"Single-leg heel raises to failure on flat ground.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metric detail missing %q:\n%s", want, out)
		}
	}

	// An undocumented metric names the gap and the command that closes it — a blank
	// section would read as the protocol failing to load rather than not existing.
	var bare bytes.Buffer
	printMetricDetail(&bare, api.MetricDefinition{
		Name: "toe-reach", Label: "Toe Reach", Unit: "cm",
		Direction: "higher_better", Category: "mobility",
	})
	for _, want := range []string{"No protocol recorded", "meso metrics edit toe-reach --how-to-measure"} {
		if !strings.Contains(bare.String(), want) {
			t.Errorf("undocumented metric missing %q:\n%s", want, bare.String())
		}
	}
	if strings.Contains(bare.String(), "How to measure") {
		t.Errorf("undocumented metric should not print an empty section:\n%s", bare.String())
	}
}

func TestPrintStats(t *testing.T) {
	var buf bytes.Buffer
	printStats(&buf, api.Stats{
		Metrics: []api.MetricTrend{
			{Metric: "deadlift-working-weight", Unit: "lb", Direction: "higher_better", Category: "strength",
				Latest: ptrFloat(225), Change: ptrFloat(40), Count: 2},
		},
		Library:  api.LibraryStats{TotalMovements: 3, Favorites: 1, ByKind: []api.KindCount{{Kind: "exercise", Count: 3}}},
		Sessions: api.SessionStats{Total: 2, Last30Days: 2},
	})
	out := buf.String()
	for _, want := range []string{"Library: 3 movements", "1 favorite", "Sessions: 2 total", "deadlift-working-weight", "225 lb", "improved"} {
		if !strings.Contains(out, want) {
			t.Errorf("stats missing %q:\n%s", want, out)
		}
	}
}
