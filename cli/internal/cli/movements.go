package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"meso/cli/internal/api"
)

func newMovementsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "movements",
		Short: "Browse and manage the movement library",
		Long: "The unified library of exercises, stretches, and yoga poses. List and\n" +
			"filter, inspect a movement's how-to/cues/faults and muscles, and create,\n" +
			"update, or delete entries. Requires a logged-in session (`meso auth login`).",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(
		newMovementsListCommand(),
		newMovementsShowCommand(),
		newMovementsCreateCommand(),
		newMovementsUpdateCommand(),
		newMovementsDeleteCommand(),
		newMovementsExportCommand(),
	)
	return cmd
}

// movementFilterFlags binds the shared list/export filter flags onto a command
// and returns a builder that reads them back (favorite is tri-state via Changed).
func movementFilterFlags(cmd *cobra.Command) func() api.MovementFilter {
	var (
		kind, tag, equip, muscle, region, search string
		favorite                                 bool
	)
	f := cmd.Flags()
	f.StringVar(&kind, "kind", "", "Only this kind (exercise|stretch|yoga_pose)")
	f.StringVar(&tag, "tag", "", "Only movements carrying this tag")
	f.StringVar(&equip, "equipment", "", "Only movements using this equipment")
	f.StringVar(&muscle, "muscle", "", "Only movements hitting this muscle")
	f.StringVar(&region, "region", "", "Only movements in this muscle region (e.g. posterior)")
	f.StringVar(&search, "search", "", "Match name or tags, case-insensitively")
	f.BoolVar(&favorite, "favorite", false, "Only favorites (use --favorite=false for non-favorites)")

	return func() api.MovementFilter {
		filter := api.MovementFilter{
			Kind: kind, Tag: tag, Equipment: equip, Muscle: muscle, Region: region, Search: search,
		}
		if cmd.Flags().Changed("favorite") {
			filter.Favorite = &favorite
		}
		return filter
	}
}

func newMovementsListCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list [flags]",
		Short:   "List movements, optionally filtered",
		Example: "  meso movements list\n  meso movements list --kind stretch\n  meso movements list --region posterior --favorite --json",
		Args:    usageArgs(cobra.NoArgs),
	}
	readFilter := movementFilterFlags(cmd)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output movements as JSON to stdout")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd.Context())
		if err != nil {
			return handleAPIError(err)
		}
		movements, err := client.ListMovements(cmd.Context(), readFilter())
		if err != nil {
			return handleAPIError(err)
		}
		if asJSON {
			return encodeJSON(cmd.OutOrStdout(), movements)
		}
		printMovementsTable(cmd.OutOrStdout(), movements)
		return nil
	}
	return cmd
}

func newMovementsShowCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "show <id>",
		Short:   "Show a movement's full detail",
		Example: "  meso movements show 1\n  meso movements show 1 --json",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := api.ParseID(args[0])
			if err != nil {
				return usageError{err}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			movement, err := client.GetMovement(cmd.Context(), id)
			if err != nil {
				return handleAPIError(err)
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), movement)
			}
			printMovementDetail(cmd.OutOrStdout(), movement)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the movement as JSON to stdout")
	return cmd
}

// movementWriteFlags are the fields settable on create and update. Pointers stay
// nil unless the flag was changed, so update sends only what the user touched.
type movementWriteFlags struct {
	kind         string
	tags         []string
	equipment    []string
	muscles      []string
	howTo        string
	formCues     string
	commonFaults string
	sanskrit     string
	defaultReps  string
	sourceURL    string
	sourceName   string
	favorite     bool
	measurable   bool
	rating       int
	defaultSets  int
	defaultHold  int
}

func bindMovementWriteFlags(cmd *cobra.Command, w *movementWriteFlags) {
	f := cmd.Flags()
	f.StringVar(&w.kind, "kind", "", "Movement kind (exercise|stretch|yoga_pose)")
	f.StringArrayVar(&w.tags, "tag", nil, "A 'good for' tag (repeatable)")
	f.StringArrayVar(&w.equipment, "equipment", nil, "Equipment used (repeatable)")
	f.StringArrayVar(&w.muscles, "muscle", nil, "A muscle tag as name[:role] (role primary|secondary, default primary; repeatable)")
	f.StringVar(&w.howTo, "how-to", "", "How to perform it (markdown)")
	f.StringVar(&w.formCues, "form-cues", "", "Form cues (markdown)")
	f.StringVar(&w.commonFaults, "common-faults", "", "Common faults (markdown)")
	f.StringVar(&w.sanskrit, "sanskrit", "", "Sanskrit name (yoga poses)")
	f.StringVar(&w.defaultReps, "default-reps", "", "Default rep scheme (e.g. 4–6, AMRAP, 30s)")
	f.StringVar(&w.sourceURL, "source-url", "", "Where it came from (URL)")
	f.StringVar(&w.sourceName, "source-name", "", "Where it came from (name)")
	f.BoolVar(&w.favorite, "favorite", false, "Mark as a favorite")
	f.BoolVar(&w.measurable, "measurable-rom", false, "Its ROM is worth tracking as a measurement")
	f.IntVar(&w.rating, "rating", 0, "Rating 1–5")
	f.IntVar(&w.defaultSets, "default-sets", 0, "Default set count")
	f.IntVar(&w.defaultHold, "default-hold", 0, "Default hold in seconds (stretches/poses)")
}

func newMovementsCreateCommand() *cobra.Command {
	var w movementWriteFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "create <name> --kind <kind> [flags]",
		Short:   "Create a movement",
		Example: "  meso movements create \"Front Squat\" --kind exercise --tag legs --muscle quads:primary --muscle glutes:secondary",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if w.kind == "" {
				return usageError{fmt.Errorf("--kind is required")}
			}
			muscles, err := parseMuscleFlags(w.muscles)
			if err != nil {
				return usageError{err}
			}
			in := api.MovementCreate{
				Name:          args[0],
				MovementKind:  w.kind,
				Favorite:      w.favorite,
				MeasurableROM: w.measurable,
				Tags:          w.tags,
				Equipment:     w.equipment,
				HowTo:         w.howTo,
				FormCues:      w.formCues,
				CommonFaults:  w.commonFaults,
				Muscles:       muscles,
			}
			f := cmd.Flags()
			if f.Changed("rating") {
				in.Rating = &w.rating
			}
			if f.Changed("default-sets") {
				in.DefaultSets = &w.defaultSets
			}
			if f.Changed("default-hold") {
				in.DefaultHoldSeconds = &w.defaultHold
			}
			if f.Changed("default-reps") {
				in.DefaultReps = &w.defaultReps
			}
			if f.Changed("sanskrit") {
				in.SanskritName = &w.sanskrit
			}
			if f.Changed("source-url") {
				in.SourceURL = &w.sourceURL
			}
			if f.Changed("source-name") {
				in.SourceName = &w.sourceName
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			movement, err := client.CreateMovement(cmd.Context(), in)
			if err != nil {
				return handleAPIError(err)
			}
			return echoMovement(cmd.OutOrStdout(), movement, asJSON, "Created")
		},
	}
	bindMovementWriteFlags(cmd, &w)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the created movement as JSON")
	return cmd
}

func newMovementsUpdateCommand() *cobra.Command {
	var w movementWriteFlags
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "update <id> [flags]",
		Short:   "Update a movement (only the flags you pass change)",
		Example: "  meso movements update 3 --favorite\n  meso movements update 3 --tag mobility --muscle glutes:primary",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := api.ParseID(args[0])
			if err != nil {
				return usageError{err}
			}
			patch, err := buildMovementPatch(cmd, &w)
			if err != nil {
				return usageError{err}
			}
			if len(patch) == 0 {
				return usageError{fmt.Errorf("nothing to update — pass at least one field flag")}
			}

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			movement, err := client.UpdateMovement(cmd.Context(), id, patch)
			if err != nil {
				return handleAPIError(err)
			}
			return echoMovement(cmd.OutOrStdout(), movement, asJSON, "Updated")
		},
	}
	bindMovementWriteFlags(cmd, &w)
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the updated movement as JSON")
	return cmd
}

// buildMovementPatch assembles the update body from only the flags the user
// changed, so omitted fields are left untouched server-side.
func buildMovementPatch(cmd *cobra.Command, w *movementWriteFlags) (map[string]any, error) {
	f := cmd.Flags()
	patch := map[string]any{}
	if f.Changed("kind") {
		patch["movement_kind"] = w.kind
	}
	if f.Changed("favorite") {
		patch["favorite"] = w.favorite
	}
	if f.Changed("measurable-rom") {
		patch["measurable_rom"] = w.measurable
	}
	if f.Changed("tag") {
		patch["tags"] = w.tags
	}
	if f.Changed("equipment") {
		patch["equipment"] = w.equipment
	}
	if f.Changed("how-to") {
		patch["how_to"] = w.howTo
	}
	if f.Changed("form-cues") {
		patch["form_cues"] = w.formCues
	}
	if f.Changed("common-faults") {
		patch["common_faults"] = w.commonFaults
	}
	if f.Changed("rating") {
		patch["rating"] = w.rating
	}
	if f.Changed("default-sets") {
		patch["default_sets"] = w.defaultSets
	}
	if f.Changed("default-hold") {
		patch["default_hold_seconds"] = w.defaultHold
	}
	if f.Changed("default-reps") {
		patch["default_reps"] = w.defaultReps
	}
	if f.Changed("sanskrit") {
		patch["sanskrit_name"] = w.sanskrit
	}
	if f.Changed("source-url") {
		patch["source_url"] = w.sourceURL
	}
	if f.Changed("source-name") {
		patch["source_name"] = w.sourceName
	}
	if f.Changed("muscle") {
		muscles, err := parseMuscleFlags(w.muscles)
		if err != nil {
			return nil, err
		}
		patch["muscles"] = muscles
	}
	return patch, nil
}

func newMovementsDeleteCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a movement",
		Example: "  meso movements delete 12\n  meso movements delete 12 --yes",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := api.ParseID(args[0])
			if err != nil {
				return usageError{err}
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return handleAPIError(err)
			}
			if !yes {
				movement, err := client.GetMovement(cmd.Context(), id)
				if err != nil {
					return handleAPIError(err)
				}
				if !confirm(cmd.InOrStdin(), cmd.OutOrStdout(), fmt.Sprintf("Delete %q (id %d)?", movement.Name, id)) {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}
			if err := client.DeleteMovement(cmd.Context(), id); err != nil {
				return handleAPIError(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted movement %d.\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}

func newMovementsExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "export [flags]",
		Short:   "Export movements as CSV to stdout",
		Long:    "Export the movement library (optionally filtered) as CSV — data portability from day one.",
		Example: "  meso movements export > movements.csv\n  meso movements export --kind exercise",
		Args:    usageArgs(cobra.NoArgs),
	}
	readFilter := movementFilterFlags(cmd)
	// --csv is accepted for the documented `export --csv` grammar; CSV is the only
	// format export emits, so the flag is a no-op affirmation.
	cmd.Flags().Bool("csv", false, "Emit CSV (the default and only format)")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd.Context())
		if err != nil {
			return handleAPIError(err)
		}
		movements, err := client.ListMovements(cmd.Context(), readFilter())
		if err != nil {
			return handleAPIError(err)
		}
		return writeMovementsCSV(cmd.OutOrStdout(), movements)
	}
	return cmd
}

// parseMuscleFlags turns "name[:role]" strings into muscle inputs, defaulting to
// the primary role and validating role values up front.
func parseMuscleFlags(raw []string) ([]api.MuscleInput, error) {
	out := make([]api.MuscleInput, 0, len(raw))
	for _, r := range raw {
		name := r
		role := "primary"
		if before, after, found := strings.Cut(r, ":"); found {
			name = before
			role = after
		}
		name = strings.TrimSpace(name)
		role = strings.TrimSpace(role)
		if name == "" {
			return nil, fmt.Errorf("empty muscle name in %q", r)
		}
		if role != "primary" && role != "secondary" {
			return nil, fmt.Errorf("invalid role %q in %q: want primary or secondary", role, r)
		}
		out = append(out, api.MuscleInput{Muscle: name, Role: role})
	}
	return out, nil
}

func echoMovement(out io.Writer, m api.Movement, asJSON bool, verb string) error {
	if asJSON {
		return encodeJSON(out, m)
	}
	fmt.Fprintf(out, "%s movement %d: %s\n", verb, m.ID, m.Name)
	return nil
}

func printMovementsTable(out io.Writer, movements []api.Movement) {
	if len(movements) == 0 {
		fmt.Fprintln(out, "No movements match.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tKIND\tFAV\tPRIMARY MUSCLES\tTAGS")
	for _, m := range movements {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
			m.ID, m.Name, m.MovementKind, yesNo(m.Favorite),
			orDash(strings.Join(m.PrimaryMuscles(), ", ")), orDash(strings.Join(m.Tags, ", ")))
	}
	_ = tw.Flush()
}

func printMovementDetail(out io.Writer, m api.Movement) {
	fmt.Fprintf(out, "%s  (#%d)\n", m.Name, m.ID)
	row := func(label, value string) { fmt.Fprintf(out, "  %-15s %s\n", label+":", value) }
	row("kind", m.MovementKind)
	row("favorite", map[bool]string{true: "yes", false: "no"}[m.Favorite])
	if m.Rating != nil {
		row("rating", strconv.Itoa(*m.Rating)+"/5")
	}
	row("tags", orDash(strings.Join(m.Tags, ", ")))
	row("equipment", orDash(strings.Join(m.Equipment, ", ")))
	if len(m.Muscles) > 0 {
		parts := make([]string, 0, len(m.Muscles))
		for _, mm := range m.Muscles {
			parts = append(parts, fmt.Sprintf("%s (%s, %s)", mm.Muscle, mm.Role, mm.Region))
		}
		row("muscles", strings.Join(parts, ", "))
	}
	if m.SanskritName != nil {
		row("sanskrit", *m.SanskritName)
	}
	if m.DefaultSets != nil || m.DefaultReps != nil {
		row("default scheme", fmt.Sprintf("%s sets × %s", orDashIntPtr(m.DefaultSets), orDashPtr(m.DefaultReps)))
	}
	if m.DefaultHoldSeconds != nil {
		row("default hold", strconv.Itoa(*m.DefaultHoldSeconds)+"s")
	}
	if m.MeasurableROM {
		row("measurable ROM", "yes")
	}
	if m.SourceName != nil || m.SourceURL != nil {
		row("source", strings.TrimSpace(orDashPtr(m.SourceName)+" "+orDashPtr(m.SourceURL)))
	}
	section := func(title, body string) {
		if strings.TrimSpace(body) == "" {
			return
		}
		fmt.Fprintf(out, "\n%s:\n%s\n", title, body)
	}
	section("How to", m.HowTo)
	section("Form cues", m.FormCues)
	section("Common faults", m.CommonFaults)
}

func writeMovementsCSV(out io.Writer, movements []api.Movement) error {
	w := csv.NewWriter(out)
	header := []string{"id", "name", "kind", "favorite", "rating", "tags", "equipment",
		"primary_muscles", "secondary_muscles", "default_sets", "default_reps",
		"default_hold_seconds", "sanskrit_name", "measurable_rom", "source_name", "source_url"}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, m := range movements {
		var secondary []string
		for _, mm := range m.Muscles {
			if mm.Role == "secondary" {
				secondary = append(secondary, mm.Muscle)
			}
		}
		record := []string{
			strconv.FormatInt(m.ID, 10), m.Name, m.MovementKind, strconv.FormatBool(m.Favorite),
			orDashIntPtr(m.Rating), strings.Join(m.Tags, "; "), strings.Join(m.Equipment, "; "),
			strings.Join(m.PrimaryMuscles(), "; "), strings.Join(secondary, "; "),
			orDashIntPtr(m.DefaultSets), orDashPtr(m.DefaultReps), orDashIntPtr(m.DefaultHoldSeconds),
			orDashPtr(m.SanskritName), strconv.FormatBool(m.MeasurableROM),
			orDashPtr(m.SourceName), orDashPtr(m.SourceURL),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
