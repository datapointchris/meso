package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func newStatsCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show the aggregated stats overview — metrics, library, and sessions",
		Long: "A one-shot overview: every measured metric's latest value and net change, the\n" +
			"movement library counts, and recent session frequency. `--json` gives the full\n" +
			"structured payload (per-metric point series included) for scripting.",
		Example: "  meso stats\n  meso stats --json",
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			stats, err := client.Stats(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), stats)
			}
			printStats(cmd.OutOrStdout(), stats)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the full stats payload as JSON")
	return cmd
}

func printStats(out io.Writer, s api.Stats) {
	_, _ = fmt.Fprintf(out, "Library: %d movement%s (%d favorite%s)\n",
		s.Library.TotalMovements, plural(s.Library.TotalMovements),
		s.Library.Favorites, plural(s.Library.Favorites))
	for _, kc := range s.Library.ByKind {
		_, _ = fmt.Fprintf(out, "  %-12s %d\n", kc.Kind, kc.Count)
	}

	_, _ = fmt.Fprintf(out, "\nSessions: %d total, %d in the last 30 days\n", s.Sessions.Total, s.Sessions.Last30Days)

	_, _ = fmt.Fprintln(out, "\nMetrics:")
	if len(s.Metrics) == 0 {
		_, _ = fmt.Fprintln(out, "  No measured metrics yet. Record one with `meso measurements record`.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "  METRIC\tCATEGORY\tLATEST\tCHANGE\tREADINGS")
	for _, m := range s.Metrics {
		latest := "—"
		if m.Latest != nil {
			latest = formatValue(*m.Latest) + " " + m.Unit
		}
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%d\n", m.Metric, m.Category, latest, changeSummary(m), m.Count)
	}
	_ = tw.Flush()
}
