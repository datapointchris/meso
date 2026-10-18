// Package cli wires the meso command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/datapointchris/goclikit"
	"github.com/datapointchris/goselfupdate/autoupdate"
	"github.com/datapointchris/gotermstyle/cobrahelp"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags.
var version = "dev"

// noInput is bound to the persistent --no-input flag, which forces the
// non-interactive path from a terminal. Package-level because every destructive
// verb consults it through confirm; pflag rewrites it to the default on each
// NewRootCommand, so a test never inherits the previous one's value.
var noInput bool

// apiOverride is bound to --api, the one value that outranks the config file.
// Package-level for the reason noInput is.
var apiOverride string

// exitCode lets a command choose the process exit code without Execute printing
// an "error:" line — used by `auth status` to report "not logged in" (exit 1)
// as a valid state, not a failure.
type exitCode int

func (e exitCode) Error() string { return "" }

func NewRootCommand() *cobra.Command {
	root := goclikit.AsNamespace(&cobra.Command{
		Use:   "meso",
		Short: "A mobile-first training log",
		Long: "meso is a training log — the movement library, the workouts composed from\n" +
			"it, the cycles that sequence those, the sessions where they were actually\n" +
			"performed, and the measurements and journal alongside.\n" +
			"\n" +
			"A training day starts with `meso sessions log --from-workout <id>`, which\n" +
			"copies the workout's movements in as targets; `meso workouts list` finds\n" +
			"the id. `meso sessions set add` logs each set as it happens, and\n" +
			"`meso sessions finish` closes the session. `meso review` reads the recent\n" +
			"weeks back in one payload.\n" +
			"\n" +
			"`meso sessions log` starts a training session; `meso log add` writes a\n" +
			"journal entry. A workout is the plan and a session is what was performed,\n" +
			"so changing a session's sets never changes the workout it came from.\n" +
			"\n" +
			"The noun comes first and the verb last, so moving from reading to acting\n" +
			"changes only the verb: `meso workouts show 3` becomes\n" +
			"`meso workouts edit 3`. Every top-level noun is a training noun — anything\n" +
			"about the app rather than the training lives under `meso admin`. Every\n" +
			"list and show takes --json. Authenticate once with `meso auth login`.",
		Example: "  meso workouts list                    the workouts a session can start from\n" +
			"  meso sessions log --from-workout 1    start a session from workout 1\n" +
			"  meso sessions set add 018f... 42      log a set like the last one\n" +
			"  meso review --since 12w               the last twelve weeks in one read",
		Version:       version,
		SilenceUsage:  true, // usage is shown deliberately, not on every runtime error
		SilenceErrors: true, // Execute prints errors itself, to stderr
	})
	// Flag mistakes are marked goclikit.ErrUsage, so they exit 2. Inherited by subcommands.
	// goclikit.Execute composes with this rather than replacing it, and keeping
	// it here is what makes the tree self-classifying for anything driving
	// NewRootCommand directly.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return goclikit.UsageError(err) })

	root.PersistentFlags().BoolVar(&noInput, "no-input", false,
		"Never prompt; fail naming the flag that would have answered")

	// Free -v for a future --verbose flag: cobra's auto version flag claims -v,
	// but the CLI convention reserves -v for verbose and -V/--version for
	// version. Drop the auto shorthand so --version stays long-only for now.
	root.InitDefaultVersionFlag()
	if f := root.Flags().Lookup("version"); f != nil {
		f.Shorthand = ""
		f.Usage = "Print which release of meso this is"
	}

	cobrahelp.Install(root)

	// Planning comes first because the library and its compositions are what a
	// session draws on, and everything about the app rather than the training
	// sits in the last section.
	root.AddGroup(
		&cobra.Group{ID: groupPlanning, Title: "Planning commands:"},
		&cobra.Group{ID: groupRecord, Title: "Training record commands:"},
		&cobra.Group{ID: groupReview, Title: "Review commands:"},
		&cobra.Group{ID: groupApp, Title: "App commands:"},
	)
	root.SetCompletionCommandGroupID(groupApp)
	root.AddCommand(
		inGroup(groupPlanning, withAPIFlag(newMovementsCommand())),
		inGroup(groupPlanning, withAPIFlag(newWorkoutsCommand())),
		inGroup(groupPlanning, withAPIFlag(newCyclesCommand())),
		inGroup(groupRecord, withAPIFlag(newSessionsCommand())),
		inGroup(groupRecord, withAPIFlag(newLogCommand())),
		inGroup(groupRecord, withAPIFlag(newMeasurementsCommand())),
		inGroup(groupRecord, withAPIFlag(newMetricsCommand())),
		inGroup(groupReview, withAPIFlag(newStatsCommand())),
		inGroup(groupReview, withAPIFlag(newReviewCommand())),
		inGroup(groupApp, withAPIFlag(newAdminCommand())),
		inGroup(groupApp, newAuthCommand()),
		inGroup(groupApp, newConfigCommand()),
		inGroup(groupApp, newUpdateCommand()),
	)
	return root
}

// The root's help sections.
const (
	groupPlanning = "planning"
	groupRecord   = "record"
	groupReview   = "review"
	groupApp      = "app"
)

// inGroup puts cmd under the root's section id.
func inGroup(id string, cmd *cobra.Command) *cobra.Command {
	cmd.GroupID = id
	return cmd
}

// withAPIFlag declares --api on cmd and everything under it. Only the commands
// that reach the API carry it, so one that never reads it does not offer it.
func withAPIFlag(cmd *cobra.Command) *cobra.Command {
	cmd.PersistentFlags().StringVar(&apiOverride, "api", "",
		"Use this API instead of the config file's; it must trust the file's login server")
	return cmd
}

// Execute runs the command tree and returns the process exit code.
func Execute() int {
	err := run(NewRootCommand(), autoupdate.Config{Update: updateConfig()})
	if err == nil {
		return 0
	}

	var ec exitCode
	if errors.As(err, &ec) {
		return int(ec)
	}

	// The failure is already on screen: `update` writes its own ✓/✗ line, and a
	// command given none of its arguments prints its help. Printing here would
	// report it twice.
	if !errors.Is(err, goclikit.ErrReported) {
		fmt.Fprintln(os.Stderr, "error:", err)
	}

	// A usage mistake is marked either by goclikit, for a line cobra rejects
	// before any RunE runs, or by the RunE that refused it.
	if errors.Is(err, goclikit.ErrUsage) {
		return 2
	}
	return 1
}

// run drives root through the shared bootstrap and returns its error.
//
// Separate from Execute, and taking the update config, so a test drives the
// real tree with the version check suppressed and reads the error rather than
// an exit code.
func run(root *cobra.Command, config autoupdate.Config) error {
	return goclikit.Execute(context.Background(), root, config, goclikit.WithNotFound(notFound))
}
