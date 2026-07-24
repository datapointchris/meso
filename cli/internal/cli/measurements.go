package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"meso/cli/internal/api"
)

func newMeasurementsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "measurements",
		Short: "Record and review measurements — the tracked stat time series",
		Long: "A measurement is a dated reading of a metric (define metrics first with\n" +
			"`meso metrics define`). Record readings, list them, and view a metric's trend\n" +
			"over time with its improvement summary.",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newMeasurementsRecordCommand(),
		newMeasurementsListCommand(),
		newMeasurementsTrendCommand(),
	)
	return cmd
}

func newMeasurementsRecordCommand() *cobra.Command {
	var (
		date, notes string
		value       float64
		asJSON      bool
	)
	cmd := &cobra.Command{
		Use:   "record <metric> --value <n> [flags]",
		Short: "Record a reading of a metric",
		Long: "Record a dated reading. The date defaults to today; the metric must already be\n" +
			"defined (`meso metrics define`).",
		Example: "  meso measurements record deadlift-working-weight --value 225\n" +
			"  meso measurements record 5k-time --value 1650 --date 2026-07-27 --notes 'negative split'",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("value") {
				return usageError{fmt.Errorf("a --value is required")}
			}
			in := api.MeasurementCreate{Metric: args[0], Value: value, MeasuredOn: date, Notes: notes}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			m, err := client.RecordMeasurement(cmd.Context(), in)
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), m)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Recorded %s = %s on %s\n", m.Metric, formatValue(m.Value), m.MeasuredOn)
			return nil
		},
	}
	f := cmd.Flags()
	f.Float64Var(&value, "value", 0, "The measured value (required)")
	f.StringVar(&date, "date", "", "Date measured, YYYY-MM-DD (defaults to today)")
	f.StringVar(&notes, "notes", "", "Notes for the reading")
	f.BoolVar(&asJSON, "json", false, "Output the recorded measurement as JSON")
	return cmd
}

func newMeasurementsListCommand() *cobra.Command {
	var (
		metric, from, to string
		asJSON           bool
	)
	cmd := &cobra.Command{
		Use:     "list [flags]",
		Short:   "List measurements, newest first, optionally filtered",
		Example: "  meso measurements list --metric deadlift-working-weight\n  meso measurements list --from 2026-07-01 --json",
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			filter := api.MeasurementFilter{Metric: metric, From: from, To: to}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			measurements, err := client.ListMeasurements(cmd.Context(), filter)
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), measurements)
			}
			printMeasurementsTable(cmd.OutOrStdout(), measurements)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&metric, "metric", "", "Only readings of this metric")
	f.StringVar(&from, "from", "", "Only readings on or after this date (YYYY-MM-DD)")
	f.StringVar(&to, "to", "", "Only readings on or before this date (YYYY-MM-DD)")
	f.BoolVar(&asJSON, "json", false, "Output measurements as JSON to stdout")
	return cmd
}

func newMeasurementsTrendCommand() *cobra.Command {
	var (
		from, to string
		asJSON   bool
	)
	cmd := &cobra.Command{
		Use:     "trend <metric> [flags]",
		Short:   "Show a metric's trend over time with its improvement summary",
		Example: "  meso measurements trend deadlift-working-weight\n  meso measurements trend 5k-time --from 2026-07-01 --json",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			trend, err := client.Trend(cmd.Context(), args[0], api.MeasurementFilter{From: from, To: to})
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), trend)
			}
			printTrend(cmd.OutOrStdout(), trend)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&from, "from", "", "Only points on or after this date (YYYY-MM-DD)")
	f.StringVar(&to, "to", "", "Only points on or before this date (YYYY-MM-DD)")
	f.BoolVar(&asJSON, "json", false, "Output the trend as JSON to stdout")
	return cmd
}

func printMeasurementsTable(out io.Writer, measurements []api.Measurement) {
	if len(measurements) == 0 {
		fmt.Fprintln(out, "No measurements match.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "DATE\tMETRIC\tVALUE\tSOURCE\tNOTES\tID")
	for _, m := range measurements {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n",
			m.MeasuredOn, m.Metric, formatValue(m.Value), m.Source, orDash(m.Notes), m.ID)
	}
	_ = tw.Flush()
}

func printTrend(out io.Writer, t api.MetricTrend) {
	fmt.Fprintf(out, "%s  (%s, %s, %s)\n", t.Metric, t.Unit, t.Direction, t.Category)
	if t.Count == 0 {
		fmt.Fprintln(out, "No readings yet. Record one with `meso measurements record`.")
		return
	}

	fmt.Fprintf(out, "%d reading%s: first %s → latest %s   %s\n",
		t.Count, plural(t.Count), formatValue(*t.First), formatValue(*t.Latest), changeSummary(t))
	if spark := sparkline(t.Points); spark != "" {
		fmt.Fprintf(out, "%s\n", spark)
	}

	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "  DATE\tVALUE")
	for _, p := range t.Points {
		fmt.Fprintf(tw, "  %s\t%s\n", p.MeasuredOn, formatValue(p.Value))
	}
	_ = tw.Flush()
}

// changeSummary renders the net change with a direction-aware verdict, so a smaller
// 5k time reads as an improvement even though the number went down.
func changeSummary(t api.MetricTrend) string {
	if t.Change == nil || *t.Change == 0 {
		return "(no change)"
	}
	change := *t.Change
	sign := "+"
	magnitude := change
	if change < 0 {
		sign = "−"
		magnitude = -change
	}

	improved := (change > 0 && t.Direction == "higher_better") ||
		(change < 0 && t.Direction == "lower_better")
	verdict := "worse"
	if improved {
		verdict = "improved"
	}
	return fmt.Sprintf("(%s%s %s, %s)", sign, formatValue(magnitude), t.Unit, verdict)
}

// sparkline renders the value series as block-glyph bars, min→max scaled across the
// eight levels. A flat or single-point series has no meaningful shape, so it returns
// empty and the caller skips it.
func sparkline(points []api.TrendPoint) string {
	if len(points) < 2 {
		return ""
	}
	min, max := points[0].Value, points[0].Value
	for _, p := range points {
		if p.Value < min {
			min = p.Value
		}
		if p.Value > max {
			max = p.Value
		}
	}
	if max == min {
		return ""
	}
	levels := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, p := range points {
		idx := int((p.Value - min) / (max - min) * float64(len(levels)-1))
		b.WriteRune(levels[idx])
	}
	return b.String()
}

// formatValue renders a value without a trailing ".0" for whole numbers (185 not
// 185.0) while keeping decimals (202.5) — the shared measurement number format.
func formatValue(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
