package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func newAdminFeedbackCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Read and triage feedback captured in the app",
		Long: "Feedback is captured by the button in the web app — a papercut or an idea,\n" +
			"offloaded mid-session without leaving what you were doing. This is where it\n" +
			"gets read and worked through. meso stores it in its own database and sends it\n" +
			"nowhere; `list --json` is how it gets anywhere else.\n\n" +
			"There is no bug/idea/improvement category on purpose: the body says what it\n" +
			"is, and the response is the same either way.",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newFeedbackListCommand(),
		newFeedbackShowCommand(),
		newFeedbackAddCommand(),
		newFeedbackEditCommand(),
		newFeedbackDoneCommand(),
		newFeedbackDeleteCommand(),
	)
	return cmd
}

// feedbackFilters is the four ways `list` can be scoped, resolved into the one status
// the API takes.
type feedbackFilters struct {
	status                     string
	statusSet, open, done, all bool
}

// resolve collapses the flags to a status, where the empty string means every status.
// Reaching that empty string takes an explicit --all: this is a triage queue, and a
// default that includes closed feedback buries the items still waiting on a response
// behind every one already dealt with.
func (f feedbackFilters) resolve() (string, error) {
	chosen := 0
	for _, set := range []bool{f.statusSet, f.open, f.done, f.all} {
		if set {
			chosen++
		}
	}
	if chosen > 1 {
		return "", usageError{fmt.Errorf("--status, --open, --done and --all are mutually exclusive")}
	}
	switch {
	case f.all:
		return "", nil
	case f.done:
		return "done", nil
	case f.open:
		return "open", nil
	}
	return f.status, nil
}

func newFeedbackListCommand() *cobra.Command {
	var (
		status, search  string
		open, done, all bool
		asJSON          bool
	)
	cmd := &cobra.Command{
		Use:   "list [flags]",
		Short: "List open feedback, newest first",
		Long: "List feedback. This is a triage queue, so it shows open feedback by default —\n" +
			"closed items would otherwise pile up in front of the ones still needing a\n" +
			"response. --all drops the filter; --done and --open are shorthands for the\n" +
			"matching --status. --json emits the full rows for a tool or an agent to act on.",
		Example: "  meso admin feedback list\n" +
			"  meso admin feedback list --all\n" +
			"  meso admin feedback list --search timer --json",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			wanted, err := feedbackFilters{
				status:    status,
				statusSet: cmd.Flags().Changed("status"),
				open:      open,
				done:      done,
				all:       all,
			}.resolve()
			if err != nil {
				return err
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			items, err := client.ListFeedback(cmd.Context(), api.FeedbackFilter{Status: wanted, Search: search})
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), items)
			}
			printFeedbackTable(cmd.OutOrStdout(), items)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&status, "status", "open", "Only feedback with this status (open | done)")
	f.StringVar(&search, "search", "", "Only feedback whose body matches this text")
	f.BoolVar(&open, "open", false, "Only open feedback (shorthand for --status open)")
	f.BoolVar(&done, "done", false, "Only closed feedback (shorthand for --status done)")
	f.BoolVar(&all, "all", false, "Every piece of feedback, open and closed")
	f.BoolVar(&asJSON, "json", false, "Output feedback as JSON to stdout")
	return cmd
}

func newFeedbackShowCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "show <id>",
		Short:   "Show one piece of feedback in full",
		Example: "  meso admin feedback show 019f... --json",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			item, err := client.GetFeedback(cmd.Context(), args[0])
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), item)
			}
			printFeedbackDetail(cmd.OutOrStdout(), item)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the item as JSON to stdout")
	return cmd
}

func newFeedbackAddCommand() *cobra.Command {
	var (
		context string
		asJSON  bool
	)
	cmd := &cobra.Command{
		Use:   "add <body> [flags]",
		Short: "Capture feedback from the terminal",
		Long: "Capture feedback without going through the app — the same write the in-app\n" +
			"button makes. Useful when the thought arrives while you're already in a shell,\n" +
			"or for filing something on your behalf from a conversation.",
		Example: "  meso admin feedback add \"Session timer keeps resetting when the screen sleeps\"\n" +
			"  meso admin feedback add \"Cycle detail should show the week number\" --context /cycles/3",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			item, err := client.CreateFeedback(cmd.Context(), api.FeedbackCreate{
				Body: args[0], ContextPath: context,
			})
			if err != nil {
				return handleAPIError(err)
			}
			return echoFeedback(cmd.OutOrStdout(), item, asJSON, "Captured")
		},
	}
	f := cmd.Flags()
	f.StringVar(&context, "context", "", "The app route it concerns (e.g. /cycles/3)")
	f.BoolVar(&asJSON, "json", false, "Output the captured item as JSON")
	return cmd
}

func newFeedbackEditCommand() *cobra.Command {
	var (
		body, context, status string
		asJSON                bool
	)
	cmd := &cobra.Command{
		Use:     "edit <id> [flags]",
		Short:   "Edit feedback (only the flags you pass change)",
		Example: "  meso admin feedback edit 019f... --body \"clearer restatement\"\n  meso admin feedback edit 019f... --status done",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			patch := map[string]any{}
			if f.Changed("body") {
				patch["body"] = body
			}
			if f.Changed("context") {
				patch["context_path"] = context
			}
			if f.Changed("status") {
				patch["status"] = status
			}
			if len(patch) == 0 {
				return usageError{fmt.Errorf("nothing to update — pass at least one field flag")}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			item, err := client.UpdateFeedback(cmd.Context(), args[0], patch)
			if err != nil {
				return handleAPIError(err)
			}
			return echoFeedback(cmd.OutOrStdout(), item, asJSON, "Updated")
		},
	}
	f := cmd.Flags()
	f.StringVar(&body, "body", "", "Replace the feedback body")
	f.StringVar(&context, "context", "", "Change the app route it concerns")
	f.StringVar(&status, "status", "", "Change the status (open | done)")
	f.BoolVar(&asJSON, "json", false, "Output the updated item as JSON")
	return cmd
}

func newFeedbackDoneCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "done <id>",
		Short:   "Close a piece of feedback",
		Long:    "Mark feedback done. Sugar over `edit --status done`, since closing is the verb\n you reach for constantly and editing is the one you rarely do.",
		Example: "  meso admin feedback done 019f...",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			item, err := client.UpdateFeedback(cmd.Context(), args[0], map[string]any{"status": "done"})
			if err != nil {
				return handleAPIError(err)
			}
			return echoFeedback(cmd.OutOrStdout(), item, asJSON, "Closed")
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the closed item as JSON")
	return cmd
}

func newFeedbackDeleteCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a piece of feedback",
		Example: "  meso admin feedback delete 019f... --yes",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			if !yes {
				item, err := client.GetFeedback(cmd.Context(), args[0])
				if err != nil {
					return handleAPIError(err)
				}
				ok, confirmErr := confirm(cmd, fmt.Sprintf("Delete %q?", preview(item.Body)))
				if confirmErr != nil {
					return confirmErr
				}
				if !ok {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Aborted.")
					return nil
				}
			}
			if err := client.DeleteFeedback(cmd.Context(), args[0]); err != nil {
				return handleAPIError(err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted feedback %s.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}

func echoFeedback(out io.Writer, f api.Feedback, asJSON bool, verb string) error {
	if asJSON {
		return encodeJSON(out, f)
	}
	_, _ = fmt.Fprintf(out, "%s feedback %s\n", verb, f.ID)
	return nil
}

func printFeedbackTable(out io.Writer, items []api.Feedback) {
	if len(items) == 0 {
		_, _ = fmt.Fprintln(out, "No feedback matches.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "STATUS\tCAPTURED\tWHERE\tFEEDBACK\tID")
	for _, f := range items {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			f.Status, dateOf(f.CreatedAt), orDash(f.ContextPath), preview(f.Body), f.ID)
	}
	_ = tw.Flush()
}

func printFeedbackDetail(out io.Writer, f api.Feedback) {
	_, _ = fmt.Fprintf(out, "Feedback %s\n", f.ID)
	_, _ = fmt.Fprintf(out, "  %-10s %s\n", "status:", f.Status)
	_, _ = fmt.Fprintf(out, "  %-10s %s\n", "captured:", dateOf(f.CreatedAt))
	_, _ = fmt.Fprintf(out, "  %-10s %s\n", "where:", orDash(f.ContextPath))
	_, _ = fmt.Fprintf(out, "  %-10s %s\n", "viewport:", viewportOf(f))
	if strings.TrimSpace(f.Body) != "" {
		_, _ = fmt.Fprintf(out, "\n%s\n", f.Body)
	}
}

// viewportOf renders the capture-time window size. It stays off the list table on
// purpose — triage scans that for what the feedback says, and the viewport only
// matters once you are reading one item and deciding what the layout was doing.
func viewportOf(f api.Feedback) string {
	if f.ViewportWidth == nil || f.ViewportHeight == nil {
		return "—"
	}
	return fmt.Sprintf("%d × %d", *f.ViewportWidth, *f.ViewportHeight)
}

// dateOf trims an RFC3339 timestamp to its calendar date for table display — the
// time of day is never what's being scanned for in a triage list.
func dateOf(timestamp string) string {
	if len(timestamp) >= 10 {
		return timestamp[:10]
	}
	return orDash(timestamp)
}
