package cli

import (
	"github.com/datapointchris/goclikit"
	"github.com/datapointchris/goselfupdate"
	"github.com/spf13/cobra"
)

// newUpdateCommand replaces this binary with the newest published release.
//
// It sits at the root rather than under `admin`, even though updating is about
// the software rather than the training. The daily update notice is a fixed
// sentence in goselfupdate/autoupdate — "meso vX available (running vY) — run
// `meso update`" — built from the binary name, with no way to name a nested
// command. Under `admin` the notice would direct the user to a command that
// does not exist.
//
// TagPrefix is what makes it resolve the right thing. The CLI is a nested
// module released under cli/v1.2.3 so its tags never collide with the app's,
// and GitHub's "latest release" endpoint is repository-wide — without the
// prefix it would return whatever meso released most recently, which is not
// this program.
func newUpdateCommand() *cobra.Command {
	return goclikit.UpdateCommand(updateConfig())
}

func updateConfig() goselfupdate.Config {
	return goselfupdate.Config{
		Owner:     "datapointchris",
		Repo:      "meso",
		Binary:    "meso",
		Version:   version,
		TagPrefix: "cli/",
		// No Token or TokenFunc. goselfupdate resolves the credential itself,
		// only when a request goes out: $GITHUB_TOKEN, then $GH_TOKEN, then
		// $GITHUB_TOKEN_COMMAND, which defaults to `gh auth token` and runs
		// under the library's own timeout.
	}
}
