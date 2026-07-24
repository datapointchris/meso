package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"meso/cli/internal/api"
)

func newReviewCommand() *cobra.Command {
	var (
		since  string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "review [flags]",
		Short: "Pull recent training history into one payload — the capstone read",
		Long: "review assembles the active cycles plus the recent sessions, measurements, and\n" +
			"log entries in one window. `--json` prints the full structured payload — the\n" +
			"intended surface for Claude to read (via Bash) and draft the next cycle from real\n" +
			"history, then persist it with ordinary `meso cycles` writes. There is no\n" +
			"server-side AI; the reasoning happens in the conversation.",
		Example: "  meso review\n  meso review --since 12w --json",
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			review, err := client.GetReview(cmd.Context(), since)
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), review)
			}
			printReview(cmd.OutOrStdout(), review)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&since, "since", "", "Look-back window: a count and a unit, e.g. 30d, 12w, 6m (default 30d)")
	f.BoolVar(&asJSON, "json", false, "Output the full review payload as JSON — the surface for Claude")
	return cmd
}

func printReview(out io.Writer, r api.Review) {
	fmt.Fprintf(out, "Review since %s\n", r.Since)

	fmt.Fprintf(out, "\nActive cycles: %d\n", len(r.ActiveCycles))
	for _, c := range r.ActiveCycles {
		fmt.Fprintf(out, "  #%d %s — %s\n", c.ID, c.Name, orDash(c.GoalSummary))
	}

	fmt.Fprintf(out, "\nSessions: %d   Measurements: %d   Log entries: %d\n",
		len(r.Sessions), len(r.Measurements), len(r.LogEntries))

	if len(r.Sessions) > 0 {
		fmt.Fprintln(out, "\nRecent sessions:")
		tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  DATE\tWORKOUT\tFELT")
		for _, s := range r.Sessions {
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", s.PerformedOn, orDashPtr(s.WorkoutName), orDashPtr(s.Felt))
		}
		_ = tw.Flush()
	}

	fmt.Fprintln(out, "\nRun `meso review --json` for the full payload (the surface Claude reads).")
}
