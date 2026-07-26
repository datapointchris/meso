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
		newMetricsEditCommand(),
		newMetricsDeleteCommand(),
	)
	return cmd
}

func newMetricsEditCommand() *cobra.Command {
	var (
		label, unit, direction, category string
		asJSON                           bool
	)
	cmd := &cobra.Command{
		Use:   "edit <name> [flags]",
		Short: "Change a metric's label, unit, direction, or category",
		Long: "Update a metric definition in place. Only the flags you pass change. The name\n" +
			"is the key measurements and cycles reference, so it can't be edited — rename by\n" +
			"deleting and redefining.",
		Example: "  meso metrics edit back-squat-working-weight --label \"Back Squat (working)\"\n" +
			"  meso metrics edit row-machine-500m --unit seconds --direction lower_better",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := api.MetricDefinitionUpdate{}
			f := cmd.Flags()
			if f.Changed("label") {
				in.Label = &label
			}
			if f.Changed("unit") {
				in.Unit = &unit
			}
			if f.Changed("direction") {
				in.Direction = &direction
			}
			if f.Changed("category") {
				in.Category = &category
			}
			if in.Label == nil && in.Unit == nil && in.Direction == nil && in.Category == nil {
				return fmt.Errorf("nothing to change: pass at least one of --label, --unit, --direction, --category")
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			metric, err := client.UpdateMetric(cmd.Context(), args[0], in)
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), metric)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s — %s (%s, %s, %s)\n",
				metric.Name, metric.Label, metric.Unit, metric.Direction, metric.Category)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&label, "label", "", "Display label shown in the app")
	f.StringVar(&unit, "unit", "", "Unit of measure (e.g. lb, seconds, cm, reps)")
	f.StringVar(&direction, "direction", "", "higher_better | lower_better")
	f.StringVar(&category, "category", "", "strength | cardio | mobility | body")
	f.BoolVar(&asJSON, "json", false, "Output the updated metric as JSON")
	return cmd
}

func newMetricsDeleteCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a metric definition",
		Long: "Remove a metric from the tracked-stat vocabulary. A metric with recorded\n" +
			"measurements can't be deleted (delete those readings first); a cycle that\n" +
			"targets it simply loses its target.",
		Example: "  meso metrics delete continuous-easy-run",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !yes && !confirm(cmd.InOrStdin(), cmd.OutOrStdout(), fmt.Sprintf("Delete metric %q?", name)) {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			if err := client.DeleteMetric(cmd.Context(), name); err != nil {
				return handleAPIError(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted metric %s\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
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
		label, unit, direction, category string
		asJSON                           bool
	)
	cmd := &cobra.Command{
		Use:   "define <name> [flags]",
		Short: "Define a metric to track",
		Long: "Define a metric. The name is the slug-shaped key everything addresses it by;\n" +
			"--label is what the app displays, defaulting to the name title-cased. Direction\n" +
			"is higher_better or lower_better (a heavier lift and a faster 5k both improve,\n" +
			"with opposite signs); category is strength, cardio, mobility, or body.",
		Example: "  meso metrics define deadlift-working-weight --unit lb --direction higher_better --category strength\n" +
			"  meso metrics define 5k-time --unit seconds --direction lower_better --category cardio\n" +
			"  meso metrics define row-machine-500m --label \"Row 500m\" --unit seconds --direction lower_better --category cardio",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := api.MetricDefinitionCreate{
				Name: args[0], Label: label, Unit: unit, Direction: direction, Category: category,
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
			fmt.Fprintf(cmd.OutOrStdout(), "Defined %s — %s (%s, %s, %s)\n",
				metric.Name, metric.Label, metric.Unit, metric.Direction, metric.Category)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&label, "label", "", "Display label shown in the app (default: the name, title-cased)")
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
	fmt.Fprintln(tw, "NAME\tLABEL\tUNIT\tDIRECTION\tCATEGORY")
	for _, m := range metrics {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", m.Name, m.Label, m.Unit, m.Direction, m.Category)
	}
	_ = tw.Flush()
}
