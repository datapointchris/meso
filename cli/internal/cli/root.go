// Package cli wires the meso command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/datapointchris/goclikit"
	"github.com/datapointchris/goselfupdate/autoupdate"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags.
var version = "dev"

// noInput is bound to the persistent --no-input flag, which forces the
// non-interactive path from a terminal. Package-level because every destructive
// verb consults it through confirm; pflag rewrites it to the default on each
// NewRootCommand, so a test never inherits the previous one's value.
var noInput bool

// usageError marks an invocation mistake (bad flag/args) so Execute can return
// exit code 2, distinct from a runtime failure (1). Per CLI conventions.
type usageError struct{ err error }

func (u usageError) Error() string { return u.err.Error() }

// exitCode lets a command choose the process exit code without Execute printing
// an "error:" line — used by `auth status` to report "not logged in" (exit 1)
// as a valid state, not a failure.
type exitCode int

func (e exitCode) Error() string { return "" }

// asNamespace marks a group with no action of its own, the root included. A
// bare one shows help and exits 0. A word naming none of its subcommands exits
// 2 naming the ones near it, with or without a flag after the word.
//
// Here rather than in each command file so this package's files import
// goclikit in one place.
func asNamespace(cmd *cobra.Command) *cobra.Command {
	return goclikit.AsNamespace(cmd)
}

func NewRootCommand() *cobra.Command {
	root := asNamespace(&cobra.Command{
		Use:   "meso",
		Short: "meso — a mobile-first training CLI",
		Long: "meso is a training log — the movement library, the workouts composed from\n" +
			"it, the cycles that sequence those, the sessions where they were actually\n" +
			"performed, and the measurements and journal alongside.\n" +
			"\n" +
			"The noun comes first and the verb last, so moving from reading to acting\n" +
			"changes only the final word: `meso workouts list` becomes\n" +
			"`meso workouts edit`. Every top-level noun is a training noun — anything\n" +
			"about the app rather than the training lives under `meso admin`. Every\n" +
			"list and show takes --json.\n" +
			"\n" +
			"Run any partial command with no arguments or --help to see what comes\n" +
			"next. Authenticate once with `meso auth login`.",
		Version:       version,
		SilenceUsage:  true, // usage is shown deliberately, not on every runtime error
		SilenceErrors: true, // Execute prints errors itself, to stderr
	})
	// Flag mistakes become usageError → exit 2. Inherited by subcommands.
	// goclikit.Execute composes with this rather than replacing it, and keeping
	// it here is what makes the tree self-classifying for anything driving
	// NewRootCommand directly.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })

	root.PersistentFlags().BoolVar(&noInput, "no-input", false,
		"Never prompt; fail naming the flag that would have answered")

	// Free -v for a future --verbose flag: cobra's auto version flag claims -v,
	// but the CLI convention reserves -v for verbose and -V/--version for
	// version. Drop the auto shorthand so --version stays long-only for now.
	root.InitDefaultVersionFlag()
	if f := root.Flags().Lookup("version"); f != nil {
		f.Shorthand = ""
	}

	root.AddCommand(newAuthCommand())
	root.AddCommand(newMovementsCommand())
	root.AddCommand(newWorkoutsCommand())
	root.AddCommand(newSessionsCommand())
	root.AddCommand(newMetricsCommand())
	root.AddCommand(newMeasurementsCommand())
	root.AddCommand(newStatsCommand())
	root.AddCommand(newLogCommand())
	root.AddCommand(newCyclesCommand())
	root.AddCommand(newReviewCommand())
	root.AddCommand(newAdminCommand())
	root.AddCommand(newUpdateCommand())
	return root
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

	// `update` writes its own ✓/✗ line, so printing here would report the same
	// failure twice.
	if errors.Is(err, goclikit.ErrReported) {
		return 1
	}

	fmt.Fprintln(os.Stderr, "error:", err)

	var usageErr usageError
	if errors.As(err, &usageErr) {
		return 2
	}
	// The library's classification, for a usage mistake cobra rejects before
	// any RunE runs and the tree therefore never marks itself.
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
