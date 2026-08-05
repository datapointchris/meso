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
		Long: "A session is the tracked instance of a workout: what was actually performed,\n" +
			"set by set, and how it felt. Start one from a workout template (its movements\n" +
			"copy in as the target) or start it free-form and add movements as they happen —\n" +
			"then `promote` that one into a reusable workout. `finish` ends it. Requires a\n" +
			"logged-in session (`meso auth login`).",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newSessionsLogCommand(),
		newSessionsListCommand(),
		newSessionsShowCommand(),
		newSessionsUpdateCommand(),
		newSessionsFinishCommand(),
		newSessionsPromoteCommand(),
		newSessionsDeleteCommand(),
		newSessionMovementCommand(),
		newSessionSetCommand(),
	)
	return cmd
}

// newSessionsPromoteCommand turns a performed free-form session into a workout template.
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
		Short: "Save a free-form session as a reusable workout",
		Long: "Turn a session performed without a template into a workout: what was logged\n" +
			"becomes the prescription, in the order performed — as many sets as were done,\n" +
			"the reps most of them shared, and the load finished on. Only a free-form\n" +
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
		Long: "Start (log) a session. With --from-workout the workout's movements copy in as\n" +
			"the target, with nothing performed yet — `sessions set add` logs what actually\n" +
			"happens. Without it, an empty free-form session is created: fill it with\n" +
			"`sessions movement add` as the workout happens, then `sessions promote` to keep\n" +
			"it as a template.",
		Example: "  meso sessions log --from-workout 1\n" +
			"  meso sessions log --from-workout 1 --date 2026-07-24 --felt strong\n" +
			"  meso sessions log --felt strong   # free-form, add movements as you go",
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
		from, to           string
		workout            int64
		unfinished, asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "list [flags]",
		Short: "List sessions, newest first, optionally filtered by date or workout",
		Example: "  meso sessions list\n  meso sessions list --unfinished\n" +
			"  meso sessions list --from 2026-07-01 --to 2026-07-31 --json",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			filter := api.SessionFilter{From: from, To: to, Unfinished: unfinished}
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
	f.BoolVar(&unfinished, "unfinished", false, "Only sessions still in progress")
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

// newSessionsUpdateCommand edits the session-level fields after the fact. The API has
// always accepted this; there was simply no command reaching it, so a mistyped date or
// an unrecorded "felt" could only be fixed from the web app.
func newSessionsUpdateCommand() *cobra.Command {
	var (
		w      sessionWriteFlags
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "update <id> [flags]",
		Short: "Edit a session's date, felt, duration or notes (only the flags you pass change)",
		Example: "  meso sessions update 018f... --felt tired\n" +
			"  meso sessions update 018f... --date 2026-07-24 --duration 52",
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			patch := map[string]any{}
			if f.Changed("date") {
				patch["performed_on"] = w.date
			}
			if f.Changed("felt") {
				patch["felt"] = w.felt
			}
			if f.Changed("notes") {
				patch["overall_notes"] = w.notes
			}
			if f.Changed("duration") {
				patch["duration_minutes"] = w.duration
			}
			if len(patch) == 0 {
				return usageError{fmt.Errorf("nothing to update — pass at least one field flag")}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.UpdateSession(cmd.Context(), args[0], patch)
			if err != nil {
				return handleAPIError(err)
			}
			return echoSession(cmd.OutOrStdout(), session, asJSON, "Updated")
		},
	}
	bindSessionWriteFlags(cmd, &w)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated session as JSON")
	return cmd
}

func newSessionsFinishCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "finish <id>",
		Short: "End a session and fill in its duration",
		Long: "Mark training over. The duration is worked out from when the session started,\n" +
			"so it never has to be typed, and the session stops being the one offered to\n" +
			"resume. This says nothing about whether the plan was completed — a session\n" +
			"finished two movements in is a finished session. Safe to run twice.",
		Example: "  meso sessions finish 018f...",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.FinishSession(cmd.Context(), args[0])
			if err != nil {
				return handleAPIError(err)
			}
			return echoSession(cmd.OutOrStdout(), session, asJSON, "Finished")
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the finished session as JSON")
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
		Long: "Add movements to a session as they get performed, drop one added by mistake,\n" +
			"adjust this session's target, or swap an entry for an alternate mid-session\n" +
			"(the target and the logged sets carry over). What was actually performed is\n" +
			"logged with `sessions set`. Entry ids come from `sessions show`.",
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
		a      sessionTargetFlags
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "add <session-id> <movement-id> [flags]",
		Short: "Append a movement to a session",
		Long: "Add a movement to a session already underway, whether or not it came from a\n" +
			"template — doing something the plan did not call for is part of what happened.\n" +
			"Entries land in the order added. The --target flags record an intention; use\n" +
			"`sessions set add` to log what was performed.",
		Example: "  meso sessions movement add 018f... 37 --target-sets 3 --target-reps 12\n" +
			"  meso sessions movement add 018f... 85 --done",
		Args: usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			movementID, err := api.ParseID(args[1])
			if err != nil {
				return usageError{err}
			}
			in := api.SessionMovementInput{MovementID: movementID, Done: a.done, Notes: a.notes}
			f := cmd.Flags()
			if f.Changed("target-sets") {
				in.TargetSets = &a.sets
			}
			if f.Changed("target-reps") {
				in.TargetReps = &a.reps
			}
			if f.Changed("target-load") {
				in.TargetLoad = &a.load
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
	f.IntVar(&a.sets, "target-sets", 0, "Sets this session is aiming for")
	f.StringVar(&a.reps, "target-reps", "", "Reps this session is aiming for (e.g. 5, 8–10, AMRAP)")
	f.StringVar(&a.load, "target-load", "", "Load this session is aiming for (e.g. 100lb, 80% 1RM)")
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

// sessionTargetFlags are the per-entry plan fields settable on `movement add/update`.
// They are the target, not the result: what happened lives in the sets.
type sessionTargetFlags struct {
	reps     string
	load     string
	notes    string
	sets     int
	movement int64 // swap target
	done     bool
}

func newSessionMovementUpdateCommand() *cobra.Command {
	var (
		a      sessionTargetFlags
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "update <session-id> <entry-id> [flags]",
		Short: "Adjust the target, check off, or swap one entry (only the flags you pass change)",
		Example: "  meso sessions movement update 018f... 42 --done\n" +
			"  meso sessions movement update 018f... 42 --target-load 105lb\n" +
			"  meso sessions movement update 018f... 42 --movement 9   # swap, sets carry over",
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
	f.BoolVar(&a.done, "done", false, "Override the derived done flag (--done=false to un-check)")
	f.IntVar(&a.sets, "target-sets", 0, "Sets this session is aiming for")
	f.StringVar(&a.reps, "target-reps", "", "Reps this session is aiming for (e.g. 5, 8–10, AMRAP)")
	f.StringVar(&a.load, "target-load", "", "Load this session is aiming for (e.g. 100lb, 80% 1RM)")
	f.StringVar(&a.notes, "notes", "", "Per-entry notes")
	f.Int64Var(&a.movement, "movement", 0, "Swap the entry to this movement id (target and sets carry over)")
	f.BoolVar(&asJSON, "json", false, "Output the updated session as JSON")
	return cmd
}

func buildSessionMovementPatch(cmd *cobra.Command, a *sessionTargetFlags) map[string]any {
	f := cmd.Flags()
	patch := map[string]any{}
	if f.Changed("done") {
		patch["done"] = a.done
	}
	if f.Changed("movement") {
		patch["movement_id"] = a.movement
	}
	if f.Changed("target-sets") {
		patch["target_sets"] = a.sets
	}
	if f.Changed("target-reps") {
		patch["target_reps"] = a.reps
	}
	if f.Changed("target-load") {
		patch["target_load"] = a.load
	}
	if f.Changed("notes") {
		patch["notes"] = a.notes
	}
	return patch
}

// newSessionSetCommand groups logging what was actually performed. This is the most-used
// write in the app: `set add` with no flags repeats the last set, so a working set costs
// one command rather than a form.
func newSessionSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Log the sets actually performed against an entry",
		Long: "A set is one set as it happened. Adding one with no flags repeats the previous\n" +
			"set — same reps, same load — falling back to the entry's target for the first,\n" +
			"so only a set that differs needs describing. Reaching the target ticks the entry\n" +
			"off. Entry ids come from `sessions show`.",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newSessionSetAddCommand(),
		newSessionSetUpdateCommand(),
		newSessionSetRemoveCommand(),
	)
	return cmd
}

// sessionSetFlags are the per-set fields. Reps is an int because kind carries "AMRAP"
// and hold carries a timed hold, leaving nothing for prose.
type sessionSetFlags struct {
	load  string
	kind  string
	notes string
	reps  int
	hold  int
}

func bindSessionSetFlags(cmd *cobra.Command, v *sessionSetFlags) {
	f := cmd.Flags()
	f.IntVar(&v.reps, "reps", 0, "Reps performed")
	f.StringVar(&v.load, "load", "", "Load used (e.g. 100lb, 2 plates, bodyweight)")
	f.IntVar(&v.hold, "hold", 0, "Hold in seconds, for a stretch or a pose")
	f.StringVar(&v.kind, "kind", "", "Set kind: working (default), warmup, amrap, drop, failure")
	f.StringVar(&v.notes, "notes", "", "Notes for this set")
}

func newSessionSetAddCommand() *cobra.Command {
	var (
		v      sessionSetFlags
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "add <session-id> <entry-id> [flags]",
		Short: "Log one set, repeating the last one unless told otherwise",
		Example: "  meso sessions set add 018f... 42                      # another like the last\n" +
			"  meso sessions set add 018f... 42 --reps 8 --load 100lb\n" +
			"  meso sessions set add 018f... 42 --load 85lb --kind drop",
		Args: usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			entryID, err := api.ParseID(args[1])
			if err != nil {
				return usageError{err}
			}
			in := api.SessionSetInput{SetKind: v.kind, Notes: v.notes}
			f := cmd.Flags()
			if f.Changed("reps") {
				in.Reps = &v.reps
			}
			if f.Changed("load") {
				in.Load = &v.load
			}
			if f.Changed("hold") {
				in.HoldSeconds = &v.hold
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.AddSessionSet(cmd.Context(), args[0], entryID, in)
			if err != nil {
				return handleAPIError(err)
			}
			return echoSession(cmd.OutOrStdout(), session, asJSON, "Logged a set on")
		},
	}
	bindSessionSetFlags(cmd, &v)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated session as JSON")
	return cmd
}

func newSessionSetUpdateCommand() *cobra.Command {
	var (
		v      sessionSetFlags
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:     "update <session-id> <entry-id> <set-id> [flags]",
		Short:   "Fix one logged set (only the flags you pass change)",
		Example: "  meso sessions set update 018f... 42 7 --reps 6",
		Args:    usageArgs(cobra.ExactArgs(3)),
		RunE: func(cmd *cobra.Command, args []string) error {
			entryID, setID, err := parseEntryAndSetIDs(args[1], args[2])
			if err != nil {
				return usageError{err}
			}
			f := cmd.Flags()
			patch := map[string]any{}
			if f.Changed("reps") {
				patch["reps"] = v.reps
			}
			if f.Changed("load") {
				patch["load"] = v.load
			}
			if f.Changed("hold") {
				patch["hold_seconds"] = v.hold
			}
			if f.Changed("kind") {
				patch["set_kind"] = v.kind
			}
			if f.Changed("notes") {
				patch["notes"] = v.notes
			}
			if len(patch) == 0 {
				return usageError{fmt.Errorf("nothing to update — pass at least one field flag")}
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.UpdateSessionSet(cmd.Context(), args[0], entryID, setID, patch)
			if err != nil {
				return handleAPIError(err)
			}
			return echoSession(cmd.OutOrStdout(), session, asJSON, "Updated")
		},
	}
	bindSessionSetFlags(cmd, &v)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated session as JSON")
	return cmd
}

func newSessionSetRemoveCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "rm <session-id> <entry-id> <set-id>",
		Short:   "Drop one logged set",
		Example: "  meso sessions set rm 018f... 42 7",
		Args:    usageArgs(cobra.ExactArgs(3)),
		RunE: func(cmd *cobra.Command, args []string) error {
			entryID, setID, err := parseEntryAndSetIDs(args[1], args[2])
			if err != nil {
				return usageError{err}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			session, err := client.RemoveSessionSet(cmd.Context(), args[0], entryID, setID)
			if err != nil {
				return handleAPIError(err)
			}
			return echoSession(cmd.OutOrStdout(), session, asJSON, "Updated")
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated session as JSON")
	return cmd
}

func parseEntryAndSetIDs(entryArg, setArg string) (int64, int64, error) {
	entryID, err := api.ParseID(entryArg)
	if err != nil {
		return 0, 0, err
	}
	setID, err := api.ParseID(setArg)
	if err != nil {
		return 0, 0, err
	}
	return entryID, setID, nil
}

func echoSession(out io.Writer, s api.Session, asJSON bool, verb string) error {
	if asJSON {
		return encodeJSON(out, s)
	}
	// What happened, not how much of the plan was hit. A session is a record, and a
	// score against the plan is the wrong thing to read back after every write.
	_, _ = fmt.Fprintf(out, "%s session %s on %s (%d movement%s · %d set%s)\n",
		verb, s.ID, s.PerformedOn, len(s.Movements), plural(len(s.Movements)),
		totalSets(s), plural(totalSets(s)))
	return nil
}

func totalSets(s api.Session) int {
	n := 0
	for _, m := range s.Movements {
		n += len(m.Sets)
	}
	return n
}

func printSessionsTable(out io.Writer, sessions []api.Session) {
	if len(sessions) == 0 {
		_, _ = fmt.Fprintln(out, "No sessions match.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATE\tWORKOUT\tMOVEMENTS\tSETS\tSTATUS\tFELT\tID")
	for _, s := range sessions {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\t%s\t%s\n",
			s.PerformedOn, orDashPtr(s.WorkoutName), len(s.Movements), totalSets(s),
			sessionStatus(s), orDashPtr(s.Felt), s.ID)
	}
	_ = tw.Flush()
}

func printSessionDetail(out io.Writer, s api.Session) {
	_, _ = fmt.Fprintf(out, "Session on %s  (%s)\n", s.PerformedOn, s.ID)
	row := func(label, value string) { _, _ = fmt.Fprintf(out, "  %-16s %s\n", label+":", value) }
	row("workout", orDashPtr(s.WorkoutName))
	row("status", sessionStatus(s))
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
	_, _ = fmt.Fprintln(tw, "  #\tENTRY\tDONE\tMOVEMENT\tPERFORMED\tTARGET\tPREVIOUS")
	for _, m := range s.Movements {
		_, _ = fmt.Fprintf(tw, "  %d\t%d\t%s\t%s\t%s\t%s\t%s\n",
			m.Position, m.ID, doneGlyph(m.Done), m.MovementName,
			formatPerformed(m), formatTarget(m), formatPrevious(m.Previous))
	}
	_ = tw.Flush()

	printSessionSets(out, s)
}

// printSessionSets lists the sets under each entry that has any. Set ids have to be
// reachable for `sessions set update/rm`, and the per-set numbers are the actual record
// — the entry row above only summarizes them.
func printSessionSets(out io.Writer, s api.Session) {
	for _, m := range s.Movements {
		if len(m.Sets) == 0 {
			continue
		}
		_, _ = fmt.Fprintf(out, "\n%s (entry %d):\n", m.MovementName, m.ID)
		tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "  SET\tID\tREPS\tLOAD\tHOLD\tKIND\tNOTES")
		for _, set := range m.Sets {
			hold := "—"
			if set.HoldSeconds != nil {
				hold = strconv.Itoa(*set.HoldSeconds) + "s"
			}
			_, _ = fmt.Fprintf(tw, "  %d\t%d\t%s\t%s\t%s\t%s\t%s\n",
				set.Position, set.ID, orDashIntPtr(set.Reps), orDashPtr(set.Load),
				hold, set.SetKind, orDash(set.Notes))
		}
		_ = tw.Flush()
	}
}

// formatPerformed summarizes an entry's sets as "3 × 8 · 100lb", collapsing the reps and
// load when every set shared them and spelling out the range when they did not.
func formatPerformed(m api.SessionMovement) string {
	if len(m.Sets) == 0 {
		return "—"
	}
	reps := distinctReps(m.Sets)
	loads := distinctLoads(m.Sets)

	out := strconv.Itoa(len(m.Sets))
	if reps != "" {
		out += " × " + reps
	}
	if loads != "" {
		out += " · " + loads
	}
	return out
}

func distinctReps(sets []api.SessionSet) string {
	seen := []string{}
	for _, set := range sets {
		v := "?"
		if set.Reps != nil {
			v = strconv.Itoa(*set.Reps)
		} else if set.HoldSeconds != nil {
			v = strconv.Itoa(*set.HoldSeconds) + "s"
		}
		seen = appendUnique(seen, v)
	}
	if len(seen) == 1 && seen[0] == "?" {
		return ""
	}
	return strings.Join(seen, "/")
}

func distinctLoads(sets []api.SessionSet) string {
	seen := []string{}
	for _, set := range sets {
		if set.Load != nil && *set.Load != "" {
			seen = appendUnique(seen, *set.Load)
		}
	}
	return strings.Join(seen, "/")
}

func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}

// formatTarget renders the plan the entry was performed against, or an em dash when
// there was none — a free-form entry is not measured against anything.
func formatTarget(m api.SessionMovement) string {
	parts := []string{}
	if m.TargetSets != nil || m.TargetReps != nil {
		sets, reps := "?", "?"
		if m.TargetSets != nil {
			sets = strconv.Itoa(*m.TargetSets)
		}
		if m.TargetReps != nil {
			reps = *m.TargetReps
		}
		parts = append(parts, sets+" × "+reps)
	}
	if m.TargetLoad != nil && *m.TargetLoad != "" {
		parts = append(parts, *m.TargetLoad)
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " · ")
}

// sessionStatus is the one thing about a session that changes what to do next: whether
// it is still open.
func sessionStatus(s api.Session) string {
	if s.FinishedAt == nil {
		return "in progress"
	}
	return "finished"
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
	if p.Sets > 0 {
		sets = strconv.Itoa(p.Sets)
	}
	reps := ""
	if p.Reps != nil {
		reps = strconv.Itoa(*p.Reps)
	}

	parts := []string{}
	if sets != "" || reps != "" {
		parts = append(parts, unknown(sets)+" × "+unknown(reps))
	}
	if p.Load != nil && *p.Load != "" {
		parts = append(parts, *p.Load)
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
