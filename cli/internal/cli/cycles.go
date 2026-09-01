package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func newCyclesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cycles",
		Short: "Plan and manage cycles — ordered sequences of workouts toward a goal",
		Long: "A cycle (mesocycle) is a multi-week block of workouts aimed at a target — a\n" +
			"race date, a working weight, restored range of motion. List and filter, inspect\n" +
			"the sequence, create/update/delete cycles, and manage their workout list (add,\n" +
			"edit/swap, reorder, remove).",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newCyclesListCommand(),
		newCyclesShowCommand(),
		newCyclesCreateCommand(),
		newCyclesUpdateCommand(),
		newCyclesDeleteCommand(),
		newCycleWorkoutsCommand(),
	)
	return cmd
}

// cycleFilterFlags binds the list filter flags and returns a builder that reads them
// back.
func cycleFilterFlags(cmd *cobra.Command) func() api.CycleFilter {
	var status, search string
	f := cmd.Flags()
	f.StringVar(&status, "status", "", "Only cycles with this status (planned|active|paused|completed)")
	f.StringVar(&search, "search", "", "Match name or goal, case-insensitively")
	return func() api.CycleFilter {
		return api.CycleFilter{Status: status, Search: search}
	}
}

func newCyclesListCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list [flags]",
		Short:   "List cycles, optionally filtered",
		Example: "  meso cycles list\n  meso cycles list --status active --json",
		Args:    usageArgs(cobra.NoArgs),
	}
	readFilter := cycleFilterFlags(cmd)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output cycles as JSON to stdout")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd.Context())
		if err != nil {
			return handleAPIError(err)
		}
		cycles, err := client.ListCycles(cmd.Context(), readFilter())
		if err != nil {
			return handleAPIError(err)
		}
		if asJSON {
			return encodeJSON(cmd.OutOrStdout(), cycles)
		}
		printCyclesTable(cmd.OutOrStdout(), cycles)
		return nil
	}
	return cmd
}

// cycleResolver wires the cycle resource into resolveIDOrName.
func cycleResolver(client *api.Client) nameResolver[api.Cycle] {
	return nameResolver[api.Cycle]{
		noun: "cycle",
		get:  client.GetCycle,
		search: func(ctx context.Context, q string) ([]api.Cycle, error) {
			return client.ListCycles(ctx, api.CycleFilter{Search: q})
		},
		ref: func(c api.Cycle) nameRef { return nameRef{ID: c.ID, Name: c.Name} },
	}
}

func newCyclesShowCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <id-or-name>",
		Short: "Show a cycle and its ordered workout sequence, by id or by name",
		Long: "Show one cycle whole. The argument is an id, or a name to search for —\n" +
			"an exact name wins outright, and a partial name resolves when it matches\n" +
			"one cycle. A name matching several comes back as one ready-to-run\n" +
			"command per match; a name matching none offers to create it.\n\n" +
			"`update`, `delete` and the `workouts` verbs still take an id. A name is\n" +
			"accepted where a command reads and never where it writes, because a name\n" +
			"that narrows to the wrong cycle costs a wrong screen on a read and a\n" +
			"wrong row on a write.",
		Example: "  meso cycles show 1\n  meso cycles show \"winter strength\"\n  meso cycles show 1 --json",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			cycle, err := resolveIDOrName(cmd.Context(), cmd, args[0], asJSON, cycleResolver(client))
			if err != nil {
				return err
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), cycle)
			}
			printCycleDetail(cmd.OutOrStdout(), cycle)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the cycle as JSON to stdout")
	return cmd
}

// cycleWriteFlags are the cycle-level fields settable on create and update.
type cycleWriteFlags struct {
	goal         string
	targetMetric string
	targetValue  float64
	targetDate   string
	startDate    string
	status       string
	notes        string
}

func bindCycleWriteFlags(cmd *cobra.Command, c *cycleWriteFlags) {
	f := cmd.Flags()
	f.StringVar(&c.goal, "goal", "", "Goal summary (e.g. 12-week run return)")
	f.StringVar(&c.targetMetric, "target-metric", "", "Metric this cycle targets (a defined metric name)")
	f.Float64Var(&c.targetValue, "target-value", 0, "Target value to reach for the metric")
	f.StringVar(&c.targetDate, "target-date", "", "Target date, YYYY-MM-DD (a race-anchored build)")
	f.StringVar(&c.startDate, "start-date", "", "Start date, YYYY-MM-DD")
	f.StringVar(&c.status, "status", "", "planned | active | paused | completed (default planned)")
	f.StringVar(&c.notes, "notes", "", "Notes (markdown)")
}

func newCyclesCreateCommand() *cobra.Command {
	var c cycleWriteFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "create <name> [flags]",
		Short:   "Create a cycle (add workouts with `cycles workouts add`)",
		Example: "  meso cycles create \"Return to 5k\" --goal \"12-week run return\" --status active --target-date 2026-10-24",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := api.CycleCreate{Name: args[0], GoalSummary: c.goal, Notes: c.notes, Status: c.status}
			f := cmd.Flags()
			if f.Changed("target-metric") {
				in.TargetMetric = &c.targetMetric
			}
			if f.Changed("target-value") {
				in.TargetValue = &c.targetValue
			}
			if f.Changed("target-date") {
				in.TargetDate = &c.targetDate
			}
			if f.Changed("start-date") {
				in.StartDate = &c.startDate
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			cycle, err := client.CreateCycle(cmd.Context(), in)
			if err != nil {
				return handleAPIError(err)
			}
			return echoCycle(cmd.OutOrStdout(), cycle, asJSON, "Created")
		},
	}
	bindCycleWriteFlags(cmd, &c)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the created cycle as JSON")
	return cmd
}

func newCyclesUpdateCommand() *cobra.Command {
	var c cycleWriteFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "update <id> [flags]",
		Short:   "Update a cycle's fields (only the flags you pass change)",
		Example: "  meso cycles update 3 --status active\n  meso cycles update 3 --start-date \"\"   # clear the start date",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := api.ParseCycleID(args[0])
			if err != nil {
				return usageError{err}
			}
			patch := buildCyclePatch(cmd, &c)
			if len(patch) == 0 {
				return usageError{fmt.Errorf("nothing to update — pass at least one field flag")}
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			cycle, err := client.UpdateCycle(cmd.Context(), id, patch)
			if err != nil {
				return handleAPIError(err)
			}
			return echoCycle(cmd.OutOrStdout(), cycle, asJSON, "Updated")
		},
	}
	bindCycleWriteFlags(cmd, &c)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated cycle as JSON")
	return cmd
}

func buildCyclePatch(cmd *cobra.Command, c *cycleWriteFlags) map[string]any {
	f := cmd.Flags()
	patch := map[string]any{}
	if f.Changed("goal") {
		patch["goal_summary"] = c.goal
	}
	if f.Changed("target-metric") {
		patch["target_metric"] = c.targetMetric
	}
	if f.Changed("target-value") {
		patch["target_value"] = c.targetValue
	}
	if f.Changed("target-date") {
		patch["target_date"] = c.targetDate
	}
	if f.Changed("start-date") {
		patch["start_date"] = c.startDate
	}
	if f.Changed("status") {
		patch["status"] = c.status
	}
	if f.Changed("notes") {
		patch["notes"] = c.notes
	}
	return patch
}

func newCyclesDeleteCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a cycle",
		Example: "  meso cycles delete 12 --yes",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := api.ParseCycleID(args[0])
			if err != nil {
				return usageError{err}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			if !yes {
				cycle, err := client.GetCycle(cmd.Context(), id)
				if err != nil {
					return handleAPIError(err)
				}
				ok, confirmErr := confirm(cmd, fmt.Sprintf("Delete %q (id %d)?", cycle.Name, id))
				if confirmErr != nil {
					return confirmErr
				}
				if !ok {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Aborted.")
					return nil
				}
			}
			if err := client.DeleteCycle(cmd.Context(), id); err != nil {
				return handleAPIError(err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted cycle %d.\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}

// newCycleWorkoutsCommand groups the sequence sub-commands: add / update / reorder /
// rm operate on a cycle's ordered workout list.
func newCycleWorkoutsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workouts",
		Short: "Manage a cycle's ordered workout sequence",
		Long: "Add a workout to a cycle, edit or swap an entry's prescription (week/phase/\n" +
			"frequency/intensity/conditions), reorder the sequence, or remove an entry.\n" +
			"Entry ids come from `cycles show`.",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newCycleWorkoutsAddCommand(),
		newCycleWorkoutsUpdateCommand(),
		newCycleWorkoutsReorderCommand(),
		newCycleWorkoutsRemoveCommand(),
	)
	return cmd
}

// periodizationFlags binds the per-entry prescription flags shared by add/update.
type periodizationFlags struct {
	phase      string
	frequency  string
	intensity  string
	conditions string
	week       int
	workout    int64 // swap target, update only
}

func bindPeriodizationFlags(cmd *cobra.Command, p *periodizationFlags, withSwap bool) {
	f := cmd.Flags()
	f.IntVar(&p.week, "week", 0, "Which week of the cycle this block belongs to")
	f.StringVar(&p.phase, "phase", "", "Phase label (e.g. base, build, taper)")
	f.StringVar(&p.frequency, "frequency", "", "How often (e.g. 2×/week)")
	f.StringVar(&p.intensity, "intensity", "", "Effort target (e.g. easy / Zone 2)")
	f.StringVar(&p.conditions, "conditions", "", "Readiness condition to advance (e.g. when knee-to-wall symmetric)")
	if withSwap {
		f.Int64Var(&p.workout, "workout", 0, "Swap the entry to this workout id (prescription carries over)")
	}
}

func newCycleWorkoutsAddCommand() *cobra.Command {
	var p periodizationFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "add <cycle-id> <workout-id> [flags]",
		Short:   "Append a workout to a cycle with its periodization",
		Example: "  meso cycles workouts add 1 7 --week 1 --phase base --frequency \"3×/week\" --intensity \"easy / Zone 2\"",
		Args:    usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			cycleID, err := api.ParseCycleID(args[0])
			if err != nil {
				return usageError{err}
			}
			workoutID, err := api.ParseWorkoutID(args[1])
			if err != nil {
				return usageError{err}
			}
			in := api.CycleWorkoutInput{WorkoutID: workoutID}
			f := cmd.Flags()
			if f.Changed("week") {
				in.Week = &p.week
			}
			if f.Changed("phase") {
				in.Phase = &p.phase
			}
			if f.Changed("frequency") {
				in.Frequency = &p.frequency
			}
			if f.Changed("intensity") {
				in.Intensity = &p.intensity
			}
			if f.Changed("conditions") {
				in.Conditions = &p.conditions
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			cycle, err := client.AddCycleWorkout(cmd.Context(), cycleID, in)
			if err != nil {
				return handleAPIError(err)
			}
			return echoCycle(cmd.OutOrStdout(), cycle, asJSON, "Updated")
		},
	}
	bindPeriodizationFlags(cmd, &p, false)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated cycle as JSON")
	return cmd
}

func newCycleWorkoutsUpdateCommand() *cobra.Command {
	var p periodizationFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "update <cycle-id> <entry-id> [flags]",
		Short:   "Edit or swap one entry (only the flags you pass change)",
		Example: "  meso cycles workouts update 1 4 --phase taper\n  meso cycles workouts update 1 4 --workout 9   # swap, prescription carries over",
		Args:    usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			cycleID, err := api.ParseCycleID(args[0])
			if err != nil {
				return usageError{err}
			}
			entryID, err := api.ParseCycleID(args[1])
			if err != nil {
				return usageError{err}
			}
			patch := buildPeriodizationPatch(cmd, &p)
			if len(patch) == 0 {
				return usageError{fmt.Errorf("nothing to update — pass at least one field flag")}
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			cycle, err := client.UpdateCycleWorkout(cmd.Context(), cycleID, entryID, patch)
			if err != nil {
				return handleAPIError(err)
			}
			return echoCycle(cmd.OutOrStdout(), cycle, asJSON, "Updated")
		},
	}
	bindPeriodizationFlags(cmd, &p, true)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated cycle as JSON")
	return cmd
}

func buildPeriodizationPatch(cmd *cobra.Command, p *periodizationFlags) map[string]any {
	f := cmd.Flags()
	patch := map[string]any{}
	if f.Changed("workout") {
		patch["workout_id"] = p.workout
	}
	if f.Changed("week") {
		patch["week"] = p.week
	}
	if f.Changed("phase") {
		patch["phase"] = p.phase
	}
	if f.Changed("frequency") {
		patch["frequency"] = p.frequency
	}
	if f.Changed("intensity") {
		patch["intensity"] = p.intensity
	}
	if f.Changed("conditions") {
		patch["conditions"] = p.conditions
	}
	return patch
}

func newCycleWorkoutsReorderCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "reorder <cycle-id> <entry-id>...",
		Short:   "Set the workout order (list every entry id in the desired order)",
		Example: "  meso cycles workouts reorder 1 4 2 3",
		Args:    usageArgs(cobra.MinimumNArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			cycleID, err := api.ParseCycleID(args[0])
			if err != nil {
				return usageError{err}
			}
			entryIDs := make([]int64, 0, len(args)-1)
			for _, raw := range args[1:] {
				id, err := api.ParseCycleID(raw)
				if err != nil {
					return usageError{err}
				}
				entryIDs = append(entryIDs, id)
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			cycle, err := client.ReorderCycleWorkouts(cmd.Context(), cycleID, entryIDs)
			if err != nil {
				return handleAPIError(err)
			}
			return echoCycle(cmd.OutOrStdout(), cycle, asJSON, "Reordered")
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the reordered cycle as JSON")
	return cmd
}

func newCycleWorkoutsRemoveCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "rm <cycle-id> <entry-id>",
		Short:   "Remove one entry from a cycle",
		Example: "  meso cycles workouts rm 1 4",
		Args:    usageArgs(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			cycleID, err := api.ParseCycleID(args[0])
			if err != nil {
				return usageError{err}
			}
			entryID, err := api.ParseCycleID(args[1])
			if err != nil {
				return usageError{err}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			cycle, err := client.RemoveCycleWorkout(cmd.Context(), cycleID, entryID)
			if err != nil {
				return handleAPIError(err)
			}
			return echoCycle(cmd.OutOrStdout(), cycle, asJSON, "Updated")
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated cycle as JSON")
	return cmd
}

func echoCycle(out io.Writer, c api.Cycle, asJSON bool, verb string) error {
	if asJSON {
		return encodeJSON(out, c)
	}
	_, _ = fmt.Fprintf(out, "%s cycle %d: %s [%s] (%d workout%s)\n",
		verb, c.ID, c.Name, c.Status, len(c.Workouts), plural(len(c.Workouts)))
	return nil
}

func printCyclesTable(out io.Writer, cycles []api.Cycle) {
	if len(cycles) == 0 {
		_, _ = fmt.Fprintln(out, "No cycles match.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tSTATUS\tTARGET\tWORKOUTS\tGOAL")
	for _, c := range cycles {
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%d\t%s\n",
			c.ID, c.Name, c.Status, cycleTargetLabel(c), len(c.Workouts), orDash(c.GoalSummary))
	}
	_ = tw.Flush()
}

func printCycleDetail(out io.Writer, c api.Cycle) {
	_, _ = fmt.Fprintf(out, "%s  (#%d)\n", c.Name, c.ID)
	row := func(label, value string) { _, _ = fmt.Fprintf(out, "  %-14s %s\n", label+":", value) }
	row("status", c.Status)
	row("goal", orDash(c.GoalSummary))
	row("target", cycleTargetLabel(c))
	row("start", orDashPtr(c.StartDate))
	row("target date", orDashPtr(c.TargetDate))
	if strings.TrimSpace(c.Notes) != "" {
		_, _ = fmt.Fprintf(out, "\nNotes:\n%s\n", c.Notes)
	}

	if len(c.Workouts) == 0 {
		_, _ = fmt.Fprintln(out, "\nNo workouts yet — add one with `meso cycles workouts add`.")
		return
	}
	_, _ = fmt.Fprintln(out, "\nWorkouts:")
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "  ENTRY\tWORKOUT\tWEEK\tPHASE\tFREQ\tINTENSITY\tCONDITIONS")
	for _, cw := range c.Workouts {
		_, _ = fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			cw.ID, fmt.Sprintf("%s (#%d)", cw.WorkoutName, cw.WorkoutID),
			orDashIntPtr(cw.Week), orDashPtr(cw.Phase), orDashPtr(cw.Frequency),
			orDashPtr(cw.Intensity), orDashPtr(cw.Conditions))
	}
	_ = tw.Flush()
}

// cycleTargetLabel renders the metric target compactly (e.g. "deadlift-working-weight → 315"),
// an em dash when the cycle has no numeric target.
func cycleTargetLabel(c api.Cycle) string {
	if c.TargetMetric == nil {
		return "—"
	}
	if c.TargetValue == nil {
		return *c.TargetMetric
	}
	return fmt.Sprintf("%s → %s", *c.TargetMetric, formatValue(*c.TargetValue))
}
