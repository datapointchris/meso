package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func newLogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Write and review the training journal — dated markdown entries",
		Long: "The fitness log is the dated journal Claude reviews when drafting the next\n" +
			"cycle: how training felt, what stalled, what to carry forward. Add entries,\n" +
			"list and filter them, and edit or delete. Requires a logged-in session\n" +
			"(`meso auth login`).",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newLogAddCommand(),
		newLogListCommand(),
		newLogShowCommand(),
		newLogEditCommand(),
		newLogDeleteCommand(),
	)
	return cmd
}

func newLogAddCommand() *cobra.Command {
	var (
		date, mood string
		tags       []string
		asJSON     bool
	)
	cmd := &cobra.Command{
		Use:   "add <body> [flags]",
		Short: "Add a journal entry (body is markdown)",
		Long: "Add a dated entry. The body is markdown; the date defaults to today. Tags are\n" +
			"repeatable and mood is a free-form note (e.g. strong, tired, focused).",
		Example: "  meso log add \"Deadlifts moved well, knee-to-wall symmetric.\" --tag strength --mood focused\n" +
			"  meso log add \"Rest day, legs sore.\" --date 2026-07-24 --tag rest",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := api.LogEntryCreate{Body: args[0], EntryDate: date, Tags: tags}
			if cmd.Flags().Changed("mood") {
				in.Mood = &mood
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			entry, err := client.CreateLogEntry(cmd.Context(), in)
			if err != nil {
				return handleAPIError(err)
			}
			return echoLogEntry(cmd.OutOrStdout(), entry, asJSON, "Added")
		},
	}
	f := cmd.Flags()
	f.StringVar(&date, "date", "", "Entry date, YYYY-MM-DD (defaults to today)")
	f.StringVar(&mood, "mood", "", "How it felt (e.g. strong, tired, focused)")
	f.StringArrayVar(&tags, "tag", nil, "A tag for the entry (repeatable)")
	f.BoolVar(&asJSON, "json", false, "Output the created entry as JSON")
	return cmd
}

func newLogListCommand() *cobra.Command {
	var (
		from, to, tag string
		asJSON        bool
	)
	cmd := &cobra.Command{
		Use:     "list [flags]",
		Short:   "List journal entries, newest first, optionally filtered",
		Example: "  meso log list\n  meso log list --from 2026-07-01 --tag strength --json",
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			filter := api.LogFilter{From: from, To: to, Tag: tag}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			entries, err := client.ListLog(cmd.Context(), filter)
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), entries)
			}
			printLogTable(cmd.OutOrStdout(), entries)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&from, "from", "", "Only entries on or after this date (YYYY-MM-DD)")
	f.StringVar(&to, "to", "", "Only entries on or before this date (YYYY-MM-DD)")
	f.StringVar(&tag, "tag", "", "Only entries carrying this tag")
	f.BoolVar(&asJSON, "json", false, "Output entries as JSON to stdout")
	return cmd
}

func newLogShowCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "show <id>",
		Short:   "Show a journal entry in full",
		Example: "  meso log show 018f...  --json",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			entry, err := client.GetLogEntry(cmd.Context(), args[0])
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), entry)
			}
			printLogDetail(cmd.OutOrStdout(), entry)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the entry as JSON to stdout")
	return cmd
}

func newLogEditCommand() *cobra.Command {
	var (
		body, date, mood string
		tags             []string
		asJSON           bool
	)
	cmd := &cobra.Command{
		Use:     "edit <id> [flags]",
		Short:   "Edit a journal entry (only the flags you pass change)",
		Example: "  meso log edit 018f... --body \"revised note\" --mood tired\n  meso log edit 018f... --tag strength --tag pr",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			patch := map[string]any{}
			if f.Changed("body") {
				patch["body"] = body
			}
			if f.Changed("date") {
				patch["entry_date"] = date
			}
			if f.Changed("mood") {
				patch["mood"] = mood
			}
			if f.Changed("tag") {
				patch["tags"] = tags
			}
			if len(patch) == 0 {
				return usageError{fmt.Errorf("nothing to update — pass at least one field flag")}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			entry, err := client.UpdateLogEntry(cmd.Context(), args[0], patch)
			if err != nil {
				return handleAPIError(err)
			}
			return echoLogEntry(cmd.OutOrStdout(), entry, asJSON, "Updated")
		},
	}
	f := cmd.Flags()
	f.StringVar(&body, "body", "", "Replace the entry body (markdown)")
	f.StringVar(&date, "date", "", "Change the entry date (YYYY-MM-DD)")
	f.StringVar(&mood, "mood", "", "Change the mood")
	f.StringArrayVar(&tags, "tag", nil, "Replace the tags (repeatable; sets the whole list)")
	f.BoolVar(&asJSON, "json", false, "Output the updated entry as JSON")
	return cmd
}

func newLogDeleteCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a journal entry",
		Example: "  meso log delete 018f... --yes",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			if !yes {
				entry, err := client.GetLogEntry(cmd.Context(), args[0])
				if err != nil {
					return handleAPIError(err)
				}
				if !confirm(cmd.InOrStdin(), cmd.OutOrStdout(),
					fmt.Sprintf("Delete the entry from %s?", entry.EntryDate)) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}
			if err := client.DeleteLogEntry(cmd.Context(), args[0]); err != nil {
				return handleAPIError(err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted log entry %s.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}

func echoLogEntry(out io.Writer, e api.LogEntry, asJSON bool, verb string) error {
	if asJSON {
		return encodeJSON(out, e)
	}
	_, _ = fmt.Fprintf(out, "%s log entry %s on %s\n", verb, e.ID, e.EntryDate)
	return nil
}

func printLogTable(out io.Writer, entries []api.LogEntry) {
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(out, "No log entries match.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATE\tMOOD\tTAGS\tENTRY\tID")
	for _, e := range entries {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			e.EntryDate, orDashPtr(e.Mood), orDash(strings.Join(e.Tags, ", ")), preview(e.Body), e.ID)
	}
	_ = tw.Flush()
}

func printLogDetail(out io.Writer, e api.LogEntry) {
	_, _ = fmt.Fprintf(out, "Entry on %s  (%s)\n", e.EntryDate, e.ID)
	_, _ = fmt.Fprintf(out, "  %-8s %s\n", "mood:", orDashPtr(e.Mood))
	_, _ = fmt.Fprintf(out, "  %-8s %s\n", "tags:", orDash(strings.Join(e.Tags, ", ")))
	if strings.TrimSpace(e.Body) != "" {
		_, _ = fmt.Fprintf(out, "\n%s\n", e.Body)
	}
}

// preview renders a one-line snippet of a (possibly multi-line, long) markdown body
// for the list table: the first line, truncated with an ellipsis past 60 runes.
func preview(body string) string {
	line := body
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "—"
	}
	runes := []rune(line)
	if len(runes) > 60 {
		return string(runes[:59]) + "…"
	}
	return line
}
