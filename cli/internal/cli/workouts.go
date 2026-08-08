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

func newWorkoutsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workouts",
		Short: "Build and manage workouts — ordered compositions of movements",
		Long: "A workout is an ordered, themed list of prescribed movements. List and\n" +
			"filter, inspect the composition, create/update/delete workouts, and manage\n" +
			"their movement list (add, edit/swap, reorder, remove).",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newWorkoutsListCommand(),
		newWorkoutsShowCommand(),
		newWorkoutsCreateCommand(),
		newWorkoutsUpdateCommand(),
		newWorkoutsDeleteCommand(),
		newWorkoutMovementsCommand(),
	)
	return cmd
}

// workoutFilterFlags binds the list filter flags and returns a builder that reads
// them back (favorite is tri-state via Changed).
func workoutFilterFlags(cmd *cobra.Command) func() api.WorkoutFilter {
	var (
		theme, tag, search string
		favorite           bool
	)
	f := cmd.Flags()
	f.StringVar(&theme, "theme", "", "Only workouts with this theme")
	f.StringVar(&tag, "tag", "", "Only workouts carrying this tag")
	f.StringVar(&search, "search", "", "Match name, theme, or tags, case-insensitively")
	f.BoolVar(&favorite, "favorite", false, "Only favorites (use --favorite=false for non-favorites)")

	return func() api.WorkoutFilter {
		filter := api.WorkoutFilter{Theme: theme, Tag: tag, Search: search}
		if cmd.Flags().Changed("favorite") {
			filter.Favorite = &favorite
		}
		return filter
	}
}

func newWorkoutsListCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list [flags]",
		Short:   "List workouts, optionally filtered",
		Example: "  meso workouts list\n  meso workouts list --theme push --favorite --json",
		Args:    usageArgs(cobra.NoArgs),
	}
	readFilter := workoutFilterFlags(cmd)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output workouts as JSON to stdout")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd.Context())
		if err != nil {
			return handleAPIError(err)
		}
		workouts, err := client.ListWorkouts(cmd.Context(), readFilter())
		if err != nil {
			return handleAPIError(err)
		}
		if asJSON {
			return encodeJSON(cmd.OutOrStdout(), workouts)
		}
		printWorkoutsTable(cmd.OutOrStdout(), workouts)
		return nil
	}
	return cmd
}

func newWorkoutsShowCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "show <id>",
		Short:   "Show a workout and its ordered movements",
		Example: "  meso workouts show 1\n  meso workouts show 1 --json",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := api.ParseWorkoutID(args[0])
			if err != nil {
				return usageError{err}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			workout, err := client.GetWorkout(cmd.Context(), id)
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), workout)
			}
			printWorkoutDetail(cmd.OutOrStdout(), workout)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the workout as JSON to stdout")
	return cmd
}

// workoutWriteFlags are the workout-level fields settable on create and update.
type workoutWriteFlags struct {
	theme     string
	tags      []string
	notes     string
	favorite  bool
	estimated int
}

func bindWorkoutWriteFlags(cmd *cobra.Command, w *workoutWriteFlags) {
	f := cmd.Flags()
	f.StringVar(&w.theme, "theme", "", "Theme (e.g. push, lower + shoulder rehab)")
	f.StringArrayVar(&w.tags, "tag", nil, "A tag (repeatable)")
	f.StringVar(&w.notes, "notes", "", "Notes (markdown)")
	f.BoolVar(&w.favorite, "favorite", false, "Mark as a favorite")
	f.IntVar(&w.estimated, "estimated-minutes", 0, "Estimated duration in minutes")
}

func newWorkoutsCreateCommand() *cobra.Command {
	var w workoutWriteFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "create <name> [flags]",
		Short:   "Create a workout (add movements with `workouts movements add`)",
		Example: "  meso workouts create \"Push Day\" --theme push --tag upper",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := api.WorkoutCreate{
				Name:     args[0],
				Favorite: w.favorite,
				Notes:    w.notes,
				Tags:     w.tags,
			}
			f := cmd.Flags()
			if f.Changed("theme") {
				in.Theme = &w.theme
			}
			if f.Changed("estimated-minutes") {
				in.EstimatedMinutes = &w.estimated
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			workout, err := client.CreateWorkout(cmd.Context(), in)
			if err != nil {
				return handleAPIError(err)
			}
			return echoWorkout(cmd.OutOrStdout(), workout, asJSON, "Created")
		},
	}
	bindWorkoutWriteFlags(cmd, &w)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the created workout as JSON")
	return cmd
}

func newWorkoutsUpdateCommand() *cobra.Command {
	var w workoutWriteFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "update <id> [flags]",
		Short:   "Update a workout's fields (only the flags you pass change)",
		Example: "  meso workouts update 3 --favorite\n  meso workouts update 3 --theme \"lower + rehab\"",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := api.ParseWorkoutID(args[0])
			if err != nil {
				return usageError{err}
			}
			patch := buildWorkoutPatch(cmd, &w)
			if len(patch) == 0 {
				return usageError{fmt.Errorf("nothing to update — pass at least one field flag")}
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			workout, err := client.UpdateWorkout(cmd.Context(), id, patch)
			if err != nil {
				return handleAPIError(err)
			}
			return echoWorkout(cmd.OutOrStdout(), workout, asJSON, "Updated")
		},
	}
	bindWorkoutWriteFlags(cmd, &w)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated workout as JSON")
	return cmd
}

func buildWorkoutPatch(cmd *cobra.Command, w *workoutWriteFlags) map[string]any {
	f := cmd.Flags()
	patch := map[string]any{}
	if f.Changed("theme") {
		patch["theme"] = w.theme
	}
	if f.Changed("tag") {
		patch["tags"] = w.tags
	}
	if f.Changed("notes") {
		patch["notes"] = w.notes
	}
	if f.Changed("favorite") {
		patch["favorite"] = w.favorite
	}
	if f.Changed("estimated-minutes") {
		patch["estimated_minutes"] = w.estimated
	}
	return patch
}

func newWorkoutsDeleteCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a workout",
		Example: "  meso workouts delete 12 --yes",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := api.ParseWorkoutID(args[0])
			if err != nil {
				return usageError{err}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			if !yes {
				workout, err := client.GetWorkout(cmd.Context(), id)
				if err != nil {
					return handleAPIError(err)
				}
				ok, confirmErr := confirm(cmd, fmt.Sprintf("Delete %q (id %d)?", workout.Name, id))
				if confirmErr != nil {
					return confirmErr
				}
				if !ok {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Aborted.")
					return nil
				}
			}
			if err := client.DeleteWorkout(cmd.Context(), id); err != nil {
				return handleAPIError(err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted workout %d.\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}

// newWorkoutMovementsCommand groups the composition sub-commands: add / update /
// reorder / rm operate on a workout's ordered movement list.
func newWorkoutMovementsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "movements",
		Short: "Manage a workout's ordered movement list",
		Long: "Add a movement to a workout, edit or swap an entry's prescription,\n" +
			"reorder the list, or remove an entry. Entry ids come from `workouts show`.",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newWorkoutMovementsAddCommand(),
		newWorkoutMovementsUpdateCommand(),
		newWorkoutMovementsReorderCommand(),
		newWorkoutMovementsRemoveCommand(),
	)
	return cmd
}

// prescriptionFlags binds the per-entry prescription flags shared by add/update.
type prescriptionFlags struct {
	reps     string
	load     string
	superset string
	notes    string
	sets     int
	rest     int
	movement int64 // swap target, update only
}

func bindPrescriptionFlags(cmd *cobra.Command, p *prescriptionFlags, withSwap bool) {
	f := cmd.Flags()
	f.IntVar(&p.sets, "sets", 0, "Prescribed set count")
	f.StringVar(&p.reps, "reps", "", "Prescribed reps (e.g. 4–6, AMRAP, 30s)")
	f.StringVar(&p.load, "load", "", "Prescribed load (e.g. 80% 1RM, 2 plates, bodyweight)")
	f.IntVar(&p.rest, "rest", 0, "Rest between sets, seconds")
	f.StringVar(&p.superset, "superset", "", "Superset group label")
	f.StringVar(&p.notes, "notes", "", "Per-entry notes")
	if withSwap {
		f.Int64Var(&p.movement, "movement", 0, "Swap the entry to this movement id (prescription carries over)")
	}
}

func newWorkoutMovementsAddCommand() *cobra.Command {
	var p prescriptionFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "add <workout-id> <movement-id> [flags]",
		Short:   "Append a movement to a workout with its prescription",
		Example: "  meso workouts movements add 1 7 --sets 5 --reps 5 --load \"80% 1RM\"",
		Args:    usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			workoutID, err := api.ParseWorkoutID(args[0])
			if err != nil {
				return usageError{err}
			}
			movementID, err := api.ParseID(args[1])
			if err != nil {
				return usageError{err}
			}
			in := api.WorkoutMovementInput{MovementID: movementID, Notes: p.notes}
			f := cmd.Flags()
			if f.Changed("sets") {
				in.Sets = &p.sets
			}
			if f.Changed("reps") {
				in.Reps = &p.reps
			}
			if f.Changed("load") {
				in.Load = &p.load
			}
			if f.Changed("rest") {
				in.RestSeconds = &p.rest
			}
			if f.Changed("superset") {
				in.SupersetGroup = &p.superset
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			workout, err := client.AddWorkoutMovement(cmd.Context(), workoutID, in)
			if err != nil {
				return handleAPIError(err)
			}
			return echoWorkout(cmd.OutOrStdout(), workout, asJSON, "Updated")
		},
	}
	bindPrescriptionFlags(cmd, &p, false)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated workout as JSON")
	return cmd
}

func newWorkoutMovementsUpdateCommand() *cobra.Command {
	var p prescriptionFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "update <workout-id> <entry-id> [flags]",
		Short:   "Edit or swap one entry (only the flags you pass change)",
		Example: "  meso workouts movements update 1 4 --load \"85% 1RM\"\n  meso workouts movements update 1 4 --movement 9   # swap, target carries over",
		Args:    usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			workoutID, err := api.ParseWorkoutID(args[0])
			if err != nil {
				return usageError{err}
			}
			entryID, err := api.ParseWorkoutID(args[1])
			if err != nil {
				return usageError{err}
			}
			patch := buildPrescriptionPatch(cmd, &p)
			if len(patch) == 0 {
				return usageError{fmt.Errorf("nothing to update — pass at least one field flag")}
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			workout, err := client.UpdateWorkoutMovement(cmd.Context(), workoutID, entryID, patch)
			if err != nil {
				return handleAPIError(err)
			}
			return echoWorkout(cmd.OutOrStdout(), workout, asJSON, "Updated")
		},
	}
	bindPrescriptionFlags(cmd, &p, true)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated workout as JSON")
	return cmd
}

func buildPrescriptionPatch(cmd *cobra.Command, p *prescriptionFlags) map[string]any {
	f := cmd.Flags()
	patch := map[string]any{}
	if f.Changed("movement") {
		patch["movement_id"] = p.movement
	}
	if f.Changed("sets") {
		patch["sets"] = p.sets
	}
	if f.Changed("reps") {
		patch["reps"] = p.reps
	}
	if f.Changed("load") {
		patch["load"] = p.load
	}
	if f.Changed("rest") {
		patch["rest_seconds"] = p.rest
	}
	if f.Changed("superset") {
		patch["superset_group"] = p.superset
	}
	if f.Changed("notes") {
		patch["notes"] = p.notes
	}
	return patch
}

func newWorkoutMovementsReorderCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "reorder <workout-id> <entry-id>...",
		Short:   "Set the movement order (list every entry id in the desired order)",
		Example: "  meso workouts movements reorder 1 4 2 3",
		Args:    usageArgs(cobra.MinimumNArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			workoutID, err := api.ParseWorkoutID(args[0])
			if err != nil {
				return usageError{err}
			}
			entryIDs := make([]int64, 0, len(args)-1)
			for _, raw := range args[1:] {
				id, err := api.ParseWorkoutID(raw)
				if err != nil {
					return usageError{err}
				}
				entryIDs = append(entryIDs, id)
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			workout, err := client.ReorderWorkoutMovements(cmd.Context(), workoutID, entryIDs)
			if err != nil {
				return handleAPIError(err)
			}
			return echoWorkout(cmd.OutOrStdout(), workout, asJSON, "Reordered")
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the reordered workout as JSON")
	return cmd
}

func newWorkoutMovementsRemoveCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "rm <workout-id> <entry-id>",
		Short:   "Remove one entry from a workout",
		Example: "  meso workouts movements rm 1 4",
		Args:    usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			workoutID, err := api.ParseWorkoutID(args[0])
			if err != nil {
				return usageError{err}
			}
			entryID, err := api.ParseWorkoutID(args[1])
			if err != nil {
				return usageError{err}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			workout, err := client.RemoveWorkoutMovement(cmd.Context(), workoutID, entryID)
			if err != nil {
				return handleAPIError(err)
			}
			return echoWorkout(cmd.OutOrStdout(), workout, asJSON, "Updated")
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated workout as JSON")
	return cmd
}

func echoWorkout(out io.Writer, w api.Workout, asJSON bool, verb string) error {
	if asJSON {
		return encodeJSON(out, w)
	}
	_, _ = fmt.Fprintf(out, "%s workout %d: %s (%d movement%s)\n", verb, w.ID, w.Name, len(w.Movements), plural(len(w.Movements)))
	return nil
}

func printWorkoutsTable(out io.Writer, workouts []api.Workout) {
	if len(workouts) == 0 {
		_, _ = fmt.Fprintln(out, "No workouts match.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tTHEME\tFAV\tMOVES\tTAGS")
	for _, w := range workouts {
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%d\t%s\n",
			w.ID, w.Name, orDashPtr(w.Theme), yesNo(w.Favorite), len(w.Movements), orDash(strings.Join(w.Tags, ", ")))
	}
	_ = tw.Flush()
}

func printWorkoutDetail(out io.Writer, w api.Workout) {
	_, _ = fmt.Fprintf(out, "%s  (#%d)\n", w.Name, w.ID)
	row := func(label, value string) { _, _ = fmt.Fprintf(out, "  %-16s %s\n", label+":", value) }
	row("theme", orDashPtr(w.Theme))
	row("favorite", map[bool]string{true: "yes", false: "no"}[w.Favorite])
	row("tags", orDash(strings.Join(w.Tags, ", ")))
	if w.EstimatedMinutes != nil {
		row("estimated", strconv.Itoa(*w.EstimatedMinutes)+" min")
	}
	if strings.TrimSpace(w.Notes) != "" {
		_, _ = fmt.Fprintf(out, "\nNotes:\n%s\n", w.Notes)
	}

	if len(w.Movements) == 0 {
		_, _ = fmt.Fprintln(out, "\nNo movements yet — add one with `meso workouts movements add`.")
		return
	}
	_, _ = fmt.Fprintln(out, "\nMovements:")
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "  #\tENTRY\tMOVEMENT\tSETS\tREPS\tLOAD\tREST\tSUPERSET")
	for _, m := range w.Movements {
		_, _ = fmt.Fprintf(tw, "  %d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			m.Position, m.ID, m.MovementName,
			orDashIntPtr(m.Sets), orDashPtr(m.Reps), orDashPtr(m.Load),
			restLabel(m.RestSeconds), orDashPtr(m.SupersetGroup))
	}
	_ = tw.Flush()
}

// restLabel renders a nil rest as an em dash, else "<n>s".
func restLabel(n *int) string {
	if n == nil {
		return "—"
	}
	return strconv.Itoa(*n) + "s"
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
