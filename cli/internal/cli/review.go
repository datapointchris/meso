package cli

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func newReviewCommand() *cobra.Command {
	var (
		window timeWindow
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
		Example: "  meso review\n  meso review --since 12w --json\n  meso review --since 2026-07-01 --until 2026-07-31",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			w, err := window.resolve(cmd, time.Now())
			if err != nil {
				return err
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			review, err := client.GetReview(cmd.Context(), w)
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
	window.bind(cmd, "30d")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the full review payload as JSON — the surface for Claude")
	return cmd
}

func printReview(out io.Writer, r api.Review) {
	first, last := r.Since, r.Until
	if first == "" {
		first = "the first row"
	}
	if last == "" {
		last = "now"
	}
	_, _ = fmt.Fprintf(out, "Review %s through %s\n", first, last)

	_, _ = fmt.Fprintf(out, "\nActive cycles: %d\n", len(r.ActiveCycles))
	for _, c := range r.ActiveCycles {
		_, _ = fmt.Fprintf(out, "  #%d %s — %s\n", c.ID, c.Name, orDash(c.GoalSummary))
	}

	_, _ = fmt.Fprintf(out, "\nSessions: %d   Measurements: %d   Log entries: %d\n",
		len(r.Sessions), len(r.Measurements), len(r.LogEntries))

	if len(r.Sessions) > 0 {
		_, _ = fmt.Fprintln(out, "\nRecent sessions:")
		tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "  DATE\tWORKOUT\tFELT")
		for _, s := range r.Sessions {
			_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\n", s.PerformedOn, orDashPtr(s.WorkoutName), orDashPtr(s.Felt))
		}
		_ = tw.Flush()
	}

	_, _ = fmt.Fprintln(out, "\nRun `meso review --json` for the full payload (the surface Claude reads).")
}
