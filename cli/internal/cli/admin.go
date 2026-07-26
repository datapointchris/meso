package cli

import "github.com/spf13/cobra"

// newAdminCommand is the namespace for operating meso, as distinct from using it.
// Every other top-level command is a training noun — movements, workouts, sessions,
// cycles, metrics, log — and the top-level help reads as a description of the domain
// because of that. Commands about the software itself go here instead of diluting it.
//
// The convention is HashiCorp's (`vault operator`, `consul operator`, `nomad
// operator`), named `admin` rather than `operator` because "operator" in that world
// means cluster and consensus lifecycle — raft, seal, autopilot — and meso has no
// cluster. This is application administration.
//
// There is no separate privilege here today: meso is single-user and the real check
// is Authelia at the edge, the same one every other command passes. The namespace is
// where that check would go if it ever gained one.
func newAdminCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Administer the app itself, as opposed to your training",
		Long: "Commands about meso rather than about training. Everything else in this CLI\n" +
			"operates your movement library, workouts, and history; these operate the\n" +
			"application. Requires a logged-in session (`meso auth login`).",
		RunE: requireSubcommand,
	}
	cmd.AddCommand(newAdminFeedbackCommand())
	return cmd
}
