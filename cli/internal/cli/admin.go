package cli

import (
	"github.com/datapointchris/goclikit"
	"github.com/spf13/cobra"
)

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
// The privilege lives on the server. The API refuses these routes to a caller outside
// the identity provider's admin group, read from the access token's groups claim, so
// the CLI only carries the door: anyone holding the binary can read this help and
// cannot run what it describes. Capturing feedback needs only a login, and its own
// help says so.
func newAdminCommand() *cobra.Command {
	cmd := goclikit.AsNamespace(&cobra.Command{
		Use:   "admin",
		Short: "Administer the app itself, as opposed to your training",
		Long: "Commands about meso rather than about training. Everything else in this CLI\n" +
			"operates your movement library, workouts, and history; these operate the\n" +
			"application. The server refuses administration to anyone outside the admin\n" +
			"group. A command open to every user says so in its own help.",
	})
	cmd.AddCommand(newAdminFeedbackCommand())
	return cmd
}
