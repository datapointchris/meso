// Package cli wires the meso command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/datapointchris/goselfupdate/autoupdate"
	"github.com/datapointchris/goselfupdate/cobracmd"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags.
var version = "dev"

// usageError marks an invocation mistake (bad flag/args) so Execute can return
// exit code 2, distinct from a runtime failure (1). Per CLI conventions.
type usageError struct{ err error }

func (u usageError) Error() string { return u.err.Error() }

// exitCode lets a command choose the process exit code without Execute printing
// an "error:" line — used by `auth status` to report "not logged in" (exit 1)
// as a valid state, not a failure.
type exitCode int

func (e exitCode) Error() string { return "" }

// requireSubcommand is the RunE for group commands (root, auth) that have no
// action of their own: a bare invocation shows help (exit 0), but an unknown
// subcommand is a usage error (exit 2) rather than cobra's default of silently
// showing help.
func requireSubcommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	return usageError{fmt.Errorf("unknown command %q for %q\nRun '%s --help' for usage",
		args[0], cmd.CommandPath(), cmd.CommandPath())}
}

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "meso",
		Short: "meso — a mobile-first training CLI",
		Long: "meso is the command-line client for the meso training app. Authenticate\n" +
			"once with `meso auth login`, then run commands against the API as yourself.",
		Version:       version,
		SilenceUsage:  true, // usage is shown deliberately, not on every runtime error
		SilenceErrors: true, // Execute prints errors itself, to stderr
		RunE:          requireSubcommand,
	}
	// Flag mistakes become usageError → exit 2. Inherited by subcommands.
	// cobracmd.Execute composes with this rather than replacing it, and keeping
	// it here is what makes the tree self-classifying for anything driving
	// NewRootCommand directly.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })

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
	root := NewRootCommand()
	err := cobracmd.Execute(context.Background(), root, autoupdate.Config{Update: updateConfig()})
	if err == nil {
		return 0
	}

	var ec exitCode
	if errors.As(err, &ec) {
		return int(ec)
	}

	// `update` writes its own ✓/✗ line, so printing here would report the same
	// failure twice.
	if errors.Is(err, cobracmd.ErrReported) {
		return 1
	}

	fmt.Fprintln(os.Stderr, "error:", err)

	var usageErr usageError
	if errors.As(err, &usageErr) {
		return 2
	}
	// The library's classification, for a usage mistake cobra rejects before
	// any RunE runs and the tree therefore never marks itself.
	if errors.Is(err, cobracmd.ErrUsage) {
		return 2
	}
	return 1
}
