package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func newSessionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Log and review workout sessions — a workout performed on a date",
		Long: "A session is the tracked instance of a workout: the checkboxes, the real\n" +
			"weights and reps, and how it felt. Start one from a workout template (its\n" +
			"movements copy in, prescription seeded), or start it empty and add movements\n" +
			"as they happen — then `promote` that one into a reusable workout. Requires a\n" +
			"logged-in session (`meso auth login`).",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newSessionsLogCommand(),
		newSessionsListCommand(),
		newSessionsShowCommand(),
		newSessionsPromoteCommand(),
		newSessionsDeleteCommand(),
		newSessionMovementCommand(),
	)
	return cmd
}

// newSessionsPromoteCommand turns a performed ad-hoc session into a workout template.
// This is where unplanned training becomes repeatable: what got logged becomes the
// prescription, and the session is back-linked as the template's first instance.
func newSessionsPromoteCommand() *cobra.Command {
	var (
		in     api.SessionPromote
		theme  string
		tags   []string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "promote <session-id> --name <name> [flags]",
		Short: "Save an ad-hoc session as a reusable workout",
		Long: "Turn a session performed without a template into a workout: its logged\n" +
			"actuals become the prescription, in the order performed. Only an ad-hoc\n" +
			"session can be promoted — one started from a workout already has a template.",
		Example: "  meso sessions promote 018f... --name \"Cable pull day\"\n" +
			"  meso sessions promote 018f... --name \"Cable pull day\" --theme pull --tag upper",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if in.Name == "" {
				return usageError{fmt.Errorf("--name is required")}
			}
			if cmd.Flags().Changed("theme") {
				in.Theme = &theme
			}
			in.Tags = tags

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			workout, err := client.PromoteSession(cmd.Context(), args[0], in)
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), workout)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved session %s as workout %d %q (%d movement%s)\n",
				args[0], workout.ID, workout.Name, len(workout.Movements), plural(len(workout.Movements)))
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&in.Name, "name", "", "Name for the new workout (required, must be unique)")
	f.StringVar(&theme, "theme", "", "Workout theme (e.g. pull, lower + shoulder rehab)")
	f.StringArrayVar(&tags, "tag", nil, "A tag for the workout (repeatable)")
	f.StringVar(&in.Notes, "notes", "", "Workout notes (markdown)")
	f.BoolVar(&asJSON, "json", false, "Output the created workout as JSON")
	return cmd
}

// sessionWriteFlags are the session-level fields settable on log (and reused as the
// update shape).
type sessionWriteFlags struct {
	date     string
	felt     string
	notes    string
	duration int
}

func bindSessionWriteFlags(cmd *cobra.Command, s *sessionWriteFlags) {
	f := cmd.Flags()
	f.StringVar(&s.date, "date", "", "Date performed, YYYY-MM-DD (defaults to today)")
	f.StringVar(&s.felt, "felt", "", "How it felt (e.g. strong, tired, loose)")
	f.StringVar(&s.notes, "notes", "", "Overall session notes (markdown)")
	f.IntVar(&s.duration, "duration", 0, "Duration in minutes")
}

func newSessionsLogCommand() *cobra.Command {
	var (
		s           sessionWriteFlags
		fromWorkout int64
		asJSON      bool
	)
	cmd := &cobra.Command{
		Use:   "log [flags]",
		Short: "Start a session, optionally from a workout template",
		Long: "Start (log) a session. With --from-workout the workout's movements copy in\n" +
			"with their prescription seeded as the actuals, ready to check off. Without it,\n" +
			"an empty ad-hoc session is created — fill it with `sessions movement add` as\n" +
			"the workout happens, then `sessions promote` to keep it as a template.",
		Example: "  meso sessions log --from-workout 1\n" +
			"  meso sessions log --from-workout 1 --date 2026-07-24 --felt strong\n" +
			"  meso sessions log --felt strong   # ad-hoc, add movements as you go",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			in := api.SessionCreate{PerformedOn: s.date, OverallNotes: s.notes}
			f := cmd.Flags()
			if f.Changed("from-workout") {
				in.WorkoutID = &fromWorkout
			}
			if f.Changed("felt") {
				in.Felt = &s.felt
			}
			if f.Changed("duration") {
				in.DurationMinutes = &s.duration
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.CreateSession(cmd.Context(), in)
			if err != nil {
				return handleAPIError(err)
			}
			return echoSession(cmd.OutOrStdout(), session, asJSON, "Started")
		},
	}
	bindSessionWriteFlags(cmd, &s)
	cmd.Flags().Int64Var(&fromWorkout, "from-workout", 0, "Copy this workout's movements into the session")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the created session as JSON")
	return cmd
}

func newSessionsListCommand() *cobra.Command {
	var (
		from, to string
		workout  int64
		asJSON   bool
	)
	cmd := &cobra.Command{
		Use:     "list [flags]",
		Short:   "List sessions, newest first, optionally filtered by date or workout",
		Example: "  meso sessions list\n  meso sessions list --from 2026-07-01 --to 2026-07-31 --json",
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			filter := api.SessionFilter{From: from, To: to}
			if cmd.Flags().Changed("workout") {
				filter.WorkoutID = &workout
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			sessions, err := client.ListSessions(cmd.Context(), filter)
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), sessions)
			}
			printSessionsTable(cmd.OutOrStdout(), sessions)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&from, "from", "", "Only sessions on or after this date (YYYY-MM-DD)")
	f.StringVar(&to, "to", "", "Only sessions on or before this date (YYYY-MM-DD)")
	f.Int64Var(&workout, "workout", 0, "Only sessions of this workout")
	f.BoolVar(&asJSON, "json", false, "Output sessions as JSON to stdout")
	return cmd
}

func newSessionsShowCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "show <id>",
		Short:   "Show a session and its logged movements",
		Example: "  meso sessions show 018f...  --json",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.GetSession(cmd.Context(), args[0])
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), session)
			}
			printSessionDetail(cmd.OutOrStdout(), session)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the session as JSON to stdout")
	return cmd
}

func newSessionsDeleteCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a session and its logged movements",
		Example: "  meso sessions delete 018f... --yes",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			if !yes {
				session, err := client.GetSession(cmd.Context(), args[0])
				if err != nil {
					return handleAPIError(err)
				}
				if !confirm(cmd.InOrStdin(), cmd.OutOrStdout(),
					fmt.Sprintf("Delete the session on %s?", session.PerformedOn)) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}
			if err := client.DeleteSession(cmd.Context(), args[0]); err != nil {
				return handleAPIError(err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted session %s.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}

// newSessionMovementCommand groups the per-entry sub-commands: composing the session
// (add / rm) and logging against it (done, update).
func newSessionMovementCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "movement",
		Short: "Compose and log the movements of a session",
		Long: "Add movements to a session as they get performed (the ad-hoc path), drop one\n" +
			"added by mistake, check off a set, record the real sets/reps/load, or swap an\n" +
			"entry for an alternate mid-session (actuals carry over). Entry ids come from\n" +
			"`sessions show`.",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newSessionMovementAddCommand(),
		newSessionMovementRemoveCommand(),
		newSessionMovementDoneCommand(),
		newSessionMovementUpdateCommand(),
	)
	return cmd
}

func newSessionMovementAddCommand() *cobra.Command {
	var (
		a      sessionActualFlags
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "add <session-id> <movement-id> [flags]",
		Short: "Append a movement to a session, with its actuals",
		Long: "Add a movement to a session already underway — the ad-hoc path, where the\n" +
			"session starts empty (`sessions log` with no --from-workout) and is filled in\n" +
			"as the workout happens. Entries land in the order added.",
		Example: "  meso sessions movement add 018f... 37 --sets 3 --reps 12 --load 60lb\n" +
			"  meso sessions movement add 018f... 85 --done --reps 15",
		Args: usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			movementID, err := api.ParseID(args[1])
			if err != nil {
				return usageError{err}
			}
			in := api.SessionMovementInput{MovementID: movementID, Done: a.done, Notes: a.notes}
			f := cmd.Flags()
			if f.Changed("sets") {
				in.ActualSets = &a.sets
			}
			if f.Changed("reps") {
				in.ActualReps = &a.reps
			}
			if f.Changed("load") {
				in.ActualLoad = &a.load
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.AddSessionMovement(cmd.Context(), args[0], in)
			if err != nil {
				return handleAPIError(err)
			}
			return echoSession(cmd.OutOrStdout(), session, asJSON, "Updated")
		},
	}
	f := cmd.Flags()
	f.BoolVar(&a.done, "done", false, "Mark it done as it is added")
	f.IntVar(&a.sets, "sets", 0, "Actual sets performed")
	f.StringVar(&a.reps, "reps", "", "Actual reps (e.g. 5, 8, 30s)")
	f.StringVar(&a.load, "load", "", "Actual load (e.g. 100lb, 2 plates, bodyweight)")
	f.StringVar(&a.notes, "notes", "", "Per-entry notes")
	f.BoolVar(&asJSON, "json", false, "Output the updated session as JSON")
	return cmd
}

func newSessionMovementRemoveCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "rm <session-id> <entry-id>",
		Short:   "Drop one entry from a session",
		Example: "  meso sessions movement rm 018f... 42",
		Args:    usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			entryID, err := api.ParseID(args[1])
			if err != nil {
				return usageError{err}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.RemoveSessionMovement(cmd.Context(), args[0], entryID)
			if err != nil {
				return handleAPIError(err)
			}
			return echoSession(cmd.OutOrStdout(), session, asJSON, "Updated")
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated session as JSON")
	return cmd
}

func newSessionMovementDoneCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "done <session-id> <entry-id>",
		Short:   "Mark one entry done (shortcut for `update --done`)",
		Example: "  meso sessions movement done 018f... 42",
		Args:    usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			entryID, err := api.ParseID(args[1])
			if err != nil {
				return usageError{err}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.UpdateSessionMovement(cmd.Context(), args[0], entryID, map[string]any{"done": true})
			if err != nil {
				return handleAPIError(err)
			}
			return echoSession(cmd.OutOrStdout(), session, asJSON, "Updated")
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated session as JSON")
	return cmd
}

// sessionActualFlags are the per-entry actuals settable on `movement update`.
type sessionActualFlags struct {
	reps     string
	load     string
	notes    string
	sets     int
	movement int64 // swap target
	done     bool
}

func newSessionMovementUpdateCommand() *cobra.Command {
	var (
		a      sessionActualFlags
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "update <session-id> <entry-id> [flags]",
		Short: "Record actuals, check off, or swap one entry (only the flags you pass change)",
		Example: "  meso sessions movement update 018f... 42 --done --load 100lb\n" +
			"  meso sessions movement update 018f... 42 --movement 9   # swap, actuals carry over",
		Args: usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			entryID, err := api.ParseID(args[1])
			if err != nil {
				return usageError{err}
			}
			patch := buildSessionMovementPatch(cmd, &a)
			if len(patch) == 0 {
				return usageError{fmt.Errorf("nothing to update — pass at least one field flag")}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.UpdateSessionMovement(cmd.Context(), args[0], entryID, patch)
			if err != nil {
				return handleAPIError(err)
			}
			return echoSession(cmd.OutOrStdout(), session, asJSON, "Updated")
		},
	}
	f := cmd.Flags()
	f.BoolVar(&a.done, "done", false, "Mark the entry done (--done=false to un-check)")
	f.IntVar(&a.sets, "sets", 0, "Actual sets performed")
	f.StringVar(&a.reps, "reps", "", "Actual reps (e.g. 5, 8, 30s)")
	f.StringVar(&a.load, "load", "", "Actual load (e.g. 100lb, 2 plates, bodyweight)")
	f.StringVar(&a.notes, "notes", "", "Per-entry notes")
	f.Int64Var(&a.movement, "movement", 0, "Swap the entry to this movement id (actuals carry over)")
	f.BoolVar(&asJSON, "json", false, "Output the updated session as JSON")
	return cmd
}

func buildSessionMovementPatch(cmd *cobra.Command, a *sessionActualFlags) map[string]any {
	f := cmd.Flags()
	patch := map[string]any{}
	if f.Changed("done") {
		patch["done"] = a.done
	}
	if f.Changed("movement") {
		patch["movement_id"] = a.movement
	}
	if f.Changed("sets") {
		patch["actual_sets"] = a.sets
	}
	if f.Changed("reps") {
		patch["actual_reps"] = a.reps
	}
	if f.Changed("load") {
		patch["actual_load"] = a.load
	}
	if f.Changed("notes") {
		patch["notes"] = a.notes
	}
	return patch
}

func echoSession(out io.Writer, s api.Session, asJSON bool, verb string) error {
	if asJSON {
		return encodeJSON(out, s)
	}
	done := 0
	for _, m := range s.Movements {
		if m.Done {
			done++
		}
	}
	_, _ = fmt.Fprintf(out, "%s session %s on %s (%d/%d movement%s done)\n",
		verb, s.ID, s.PerformedOn, done, len(s.Movements), plural(len(s.Movements)))
	return nil
}

func printSessionsTable(out io.Writer, sessions []api.Session) {
	if len(sessions) == 0 {
		_, _ = fmt.Fprintln(out, "No sessions match.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATE\tWORKOUT\tDONE\tFELT\tID")
	for _, s := range sessions {
		done := 0
		for _, m := range s.Movements {
			if m.Done {
				done++
			}
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d/%d\t%s\t%s\n",
			s.PerformedOn, orDashPtr(s.WorkoutName), done, len(s.Movements), orDashPtr(s.Felt), s.ID)
	}
	_ = tw.Flush()
}

func printSessionDetail(out io.Writer, s api.Session) {
	_, _ = fmt.Fprintf(out, "Session on %s  (%s)\n", s.PerformedOn, s.ID)
	row := func(label, value string) { _, _ = fmt.Fprintf(out, "  %-16s %s\n", label+":", value) }
	row("workout", orDashPtr(s.WorkoutName))
	row("felt", orDashPtr(s.Felt))
	if s.DurationMinutes != nil {
		row("duration", strconv.Itoa(*s.DurationMinutes)+" min")
	}
	if strings.TrimSpace(s.OverallNotes) != "" {
		_, _ = fmt.Fprintf(out, "\nNotes:\n%s\n", s.OverallNotes)
	}

	if len(s.Movements) == 0 {
		_, _ = fmt.Fprintln(out, "\nNo movements logged.")
		return
	}
	_, _ = fmt.Fprintln(out, "\nMovements:")
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "  #\tENTRY\tDONE\tMOVEMENT\tSETS\tREPS\tLOAD\tPREVIOUS")
	for _, m := range s.Movements {
		_, _ = fmt.Fprintf(tw, "  %d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			m.Position, m.ID, doneGlyph(m.Done), m.MovementName,
			orDashIntPtr(m.ActualSets), orDashPtr(m.ActualReps), orDashPtr(m.ActualLoad),
			formatPrevious(m.Previous))
	}
	_ = tw.Flush()
}

// formatPrevious renders the last performed result as one compact cell — the number to
// beat, alongside when it was set.
func formatPrevious(p *api.PreviousActuals) string {
	if p == nil {
		return "—"
	}
	// A "?" rather than an em dash here: within a result that exists, a missing half of
	// "sets × reps" reads as unrecorded, not as absent.
	unknown := func(s string) string {
		if s == "" {
			return "?"
		}
		return s
	}
	sets := ""
	if p.ActualSets != nil {
		sets = strconv.Itoa(*p.ActualSets)
	}
	reps := ""
	if p.ActualReps != nil {
		reps = *p.ActualReps
	}

	parts := []string{}
	if sets != "" || reps != "" {
		parts = append(parts, unknown(sets)+" × "+unknown(reps))
	}
	if p.ActualLoad != nil && *p.ActualLoad != "" {
		parts = append(parts, *p.ActualLoad)
	}
	if len(parts) == 0 {
		return p.PerformedOn
	}
	return strings.Join(parts, " · ") + " (" + p.PerformedOn + ")"
}

// doneGlyph renders the checkbox state compactly for the detail table.
func doneGlyph(done bool) string {
	if done {
		return "✓"
	}
	return "·"
}
