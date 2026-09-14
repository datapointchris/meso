package cli

import (
	"errors"

	"github.com/datapointchris/goclikit"
	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

// notFound is the classifier [goclikit.WithNotFound] calls: the API's 404, and
// the line naming what was missing.
//
// The subject is the server's own Message. Only the API knows which id was
// absent, and a verb here can take two — `meso workouts movements edit
// <workout-id> <entry-id>` — so a subject composed from the command line would
// have to guess which one the server refused.
//
// The "API request failed (404 Not Found)" prefix is dropped. That reports how
// the answer arrived, and a 404 is an answer rather than a failure to get one.
//
// # Why this never competes with resolveIDOrName
//
// `movements show`, `workouts show` and `cycles show` take <id-or-name> and go
// through [resolveIDOrName], which answers a miss with candidates it computed
// and returns an exitCode carrying no message. An exitCode is not an
// *api.APIError, so this declines and the menu is what the reader sees.
//
// That split is deliberate and is the better half getting the harder case:
// a computed list of real candidates beats a static command wherever one can be
// produced. These hints are what the other verbs get, where there is nothing to
// compute because the id was the only thing given.
func notFound(err error) (string, bool) {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || !apiErr.NotFound() || apiErr.Message == "" {
		return "", false
	}
	return apiErr.Message, true
}

// withNotFoundHints records on cmd the commands a 404 under it should name, and
// returns cmd so it can be attached inline in an AddCommand list.
//
// goclikit takes the nearest annotated ancestor rather than accumulating, so a
// nested noun names its own way in rather than inheriting its parent's.
func withNotFoundHints(cmd *cobra.Command, hints ...string) *cobra.Command {
	return goclikit.WithRecoveryHints(cmd, hints...)
}

// The way in for each noun.
//
// The two entry nouns have no list of their own: an entry exists only inside the
// workout or cycle that orders it, and both groups already document that their
// ids come from the parent's `show`.
const (
	hintMovements    = "List every movement: meso movements list"
	hintWorkouts     = "List every workout: meso workouts list"
	hintCycles       = "List every cycle: meso cycles list"
	hintMeasurements = "List every measurement: meso measurements list"
	hintSessions     = "List every session: meso sessions list"
	hintFeedback     = "List every feedback entry: meso admin feedback list"
	hintMetrics      = "List every metric: meso metrics list"
	hintLog          = "List every logged set: meso log list"

	hintWorkoutEntries = "Show the workout and its movement entries: meso workouts show <workout-id>"
	hintCycleEntries   = "Show the cycle and its workout entries: meso cycles show <cycle-id>"
)
