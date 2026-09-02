package cli

import (
	"os"
	"os/exec"
	"strings"

	"github.com/datapointchris/goclikit"
	"github.com/datapointchris/goselfupdate"
	"github.com/spf13/cobra"
)

// newUpdateCommand replaces this binary with the newest published release.
//
// It sits at the root rather than under `admin`, where it started, even though
// updating is about the software rather than the training. The daily update
// notice is a fixed sentence in goselfupdate/autoupdate — "meso vX available
// (running vY) — run `meso update`" — built from the binary name, with no way
// to name a nested command. Under `admin` the notice would direct the user to
// a command that does not exist. It also puts all four product CLIs on one
// spelling, which is worth something on its own.
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
		// TokenFunc, not Token: Execute builds this config on every invocation
		// to run the daily update check's gate, and that gate is designed to
		// cost nothing. Calling githubToken() here would put a `gh auth token`
		// subprocess in front of every `meso` command, including the ~364 out
		// of 365 that decline to check at all.
		TokenFunc: githubToken,
	}
}

// githubToken resolves a GitHub credential the way the dotfiles installer does:
// the environment first, then the gh CLI's stored token.
//
// goselfupdate reads $GITHUB_TOKEN and $GH_TOKEN on its own, so this only adds
// the third source. meso's repository is public, so this is not load-bearing
// here — it lifts the 60-requests-per-hour unauthenticated rate limit, and it
// keeps the four product CLIs resolving credentials identically rather than
// each behaving differently the day one of them changes visibility.
func githubToken() string {
	for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if token := os.Getenv(name); token != "" {
			return token
		}
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
