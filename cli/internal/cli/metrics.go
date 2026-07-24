package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"meso/cli/internal/api"
)

func newMetricsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Define and list the tracked-stat vocabulary",
		Long: "A metric is a thing worth tracking over time — a lift's working weight, a 5k\n" +
			"time, a knee-to-wall ROM. Each carries a unit, a direction (which way is\n" +
			"improvement), and a category. Measurements are recorded against these.",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newMetricsListCommand(),
		newMetricsDefineCommand(),
	)
	return cmd
}

func newMetricsListCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List metric definitions, grouped by category",
		Example: "  meso metrics list\n  meso metrics list --json",
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			metrics, err := client.ListMetrics(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), metrics)
			}
			printMetricsTable(cmd.OutOrStdout(), metrics)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output metrics as JSON to stdout")
	return cmd
}

func newMetricsDefineCommand() *cobra.Command {
	var (
		unit, direction, category string
		asJSON                    bool
	)
	cmd := &cobra.Command{
		Use:   "define <name> [flags]",
		Short: "Define a metric to track",
		Long: "Define a metric. Direction is higher_better or lower_better (a heavier lift and\n" +
			"a faster 5k both improve, with opposite signs); category is strength, cardio,\n" +
			"mobility, or body.",
		Example: "  meso metrics define deadlift-working-weight --unit lb --direction higher_better --category strength\n" +
			"  meso metrics define 5k-time --unit seconds --direction lower_better --category cardio",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := api.MetricDefinitionCreate{
				Name: args[0], Unit: unit, Direction: direction, Category: category,
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			metric, err := client.DefineMetric(cmd.Context(), in)
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), metric)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Defined %s (%s, %s, %s)\n",
				metric.Name, metric.Unit, metric.Direction, metric.Category)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&unit, "unit", "", "Unit of measure (e.g. lb, seconds, cm, reps)")
	f.StringVar(&direction, "direction", "higher_better", "higher_better | lower_better")
	f.StringVar(&category, "category", "", "strength | cardio | mobility | body")
	f.BoolVar(&asJSON, "json", false, "Output the defined metric as JSON")
	return cmd
}

func printMetricsTable(out io.Writer, metrics []api.MetricDefinition) {
	if len(metrics) == 0 {
		fmt.Fprintln(out, "No metrics defined. Define one with `meso metrics define`.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tUNIT\tDIRECTION\tCATEGORY")
	for _, m := range metrics {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", m.Name, m.Unit, m.Direction, m.Category)
	}
	_ = tw.Flush()
}
