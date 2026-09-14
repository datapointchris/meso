package cli

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/datapointchris/goclikit"
	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

func TestNotFoundCarriesTheServersMessage(t *testing.T) {
	t.Parallel()

	err := &api.APIError{
		StatusCode: http.StatusNotFound,
		Status:     "404 Not Found",
		Message:    "workout 12 not found",
	}

	subject, ok := notFound(err)
	if !ok {
		t.Fatal("notFound() = false, want true for a 404 carrying a message")
	}
	if subject != "workout 12 not found" {
		t.Errorf("subject = %q, want the server's message", subject)
	}
	if strings.Contains(subject, "API request failed") {
		t.Errorf("subject = %q, carries the transport prefix", subject)
	}
}

func TestNotFoundDeclinesA404WithNoMessage(t *testing.T) {
	t.Parallel()

	err := &api.APIError{StatusCode: http.StatusNotFound, Status: "404 Not Found"}
	if _, ok := notFound(err); ok {
		t.Error("notFound() = true for a 404 with no message")
	}
}

func TestNotFoundDeclinesOtherStatuses(t *testing.T) {
	t.Parallel()

	unauthorized := &api.APIError{StatusCode: http.StatusUnauthorized, Message: "not logged in"}
	if _, ok := notFound(unauthorized); ok {
		t.Error("notFound() = true for a 401")
	}
	if _, ok := notFound(errors.New("dial tcp: connection refused")); ok {
		t.Error("notFound() = true for a transport error")
	}
}

// TestTheResolverMenuIsNeverReplacedByAHint is the boundary this work had to
// settle. `movements show`, `workouts show` and `cycles show` answer a miss with
// candidates resolveIDOrName computed; the hints are for everything else. Both
// firing on one failure, or one failure answered two ways depending on the verb,
// is the thing being prevented.
func TestTheResolverMenuIsNeverReplacedByAHint(t *testing.T) {
	t.Parallel()

	// resolveIDOrName returns an exitCode once it has printed its menu. An
	// exitCode is not an *api.APIError, so the classifier declines and goclikit
	// leaves the run alone.
	if _, ok := notFound(exitCode(1)); ok {
		t.Error("notFound() = true for the resolver's exitCode, which would print over its menu")
	}
}

// findLeaf resolves a command path and fails if it did not land on the command
// named. cobra's Find returns the deepest *matching* command with the rest as
// arguments, so a misspelled verb silently resolves to its group -- which makes
// a hint assertion pass against the wrong command.
func findLeaf(t *testing.T, root *cobra.Command, path []string) *cobra.Command {
	t.Helper()
	command, _, err := root.Find(path)
	if err != nil {
		t.Fatalf("Find(%v) = %v", path, err)
	}
	if want := path[len(path)-1]; command.Name() != want {
		t.Fatalf("Find(%v) landed on %q, not %q", path, command.Name(), want)
	}
	return command
}

// hintsFor mirrors goclikit's lookup: nearest annotated ancestor wins, and it
// replaces the outer set rather than adding to it.
func hintsFor(cmd *cobra.Command) string {
	for current := cmd; current != nil; current = current.Parent() {
		if joined := current.Annotations[goclikit.RecoveryHintsAnnotation]; joined != "" {
			return joined
		}
	}
	return ""
}

func TestEveryNounNamesItsOwnWayIn(t *testing.T) {
	t.Parallel()

	root := NewRootCommand()
	for _, tc := range []struct {
		path []string
		want string
	}{
		{[]string{"movements", "update"}, hintMovements},
		{[]string{"workouts", "update"}, hintWorkouts},
		{[]string{"cycles", "update"}, hintCycles},
		{[]string{"measurements", "list"}, hintMeasurements},
		{[]string{"sessions", "list"}, hintSessions},
		// feedback is registered under admin, not on the root.
		{[]string{"admin", "feedback", "show"}, hintFeedback},
		{[]string{"metrics", "show"}, hintMetrics},
		{[]string{"log", "list"}, hintLog},
	} {
		command := findLeaf(t, root, tc.path)
		if got := hintsFor(command); !strings.Contains(got, tc.want) {
			t.Errorf("%v hints = %q, want it to name %q", tc.path, got, tc.want)
		}
	}
}

func TestAnEntryPointsAtItsParentsShow(t *testing.T) {
	t.Parallel()

	root := NewRootCommand()
	for _, tc := range []struct {
		path []string
		want string
	}{
		// An entry id exists only inside its parent and is listed nowhere else.
		{[]string{"workouts", "movements", "rm"}, hintWorkoutEntries},
		{[]string{"workouts", "movements", "update"}, hintWorkoutEntries},
		{[]string{"cycles", "workouts", "rm"}, hintCycleEntries},
		{[]string{"cycles", "workouts", "update"}, hintCycleEntries},
	} {
		command := findLeaf(t, root, tc.path)
		if got := hintsFor(command); !strings.Contains(got, tc.want) {
			t.Errorf("%v hints = %q, want it to name %q", tc.path, got, tc.want)
		}
	}
}

func TestAnAddVerbNamesTheLibraryItDrawsFrom(t *testing.T) {
	t.Parallel()

	root := NewRootCommand()
	for _, tc := range []struct {
		path []string
		want []string
	}{
		{[]string{"workouts", "movements", "add"}, []string{hintWorkouts, hintMovements}},
		{[]string{"cycles", "workouts", "add"}, []string{hintCycles, hintWorkouts}},
	} {
		command := findLeaf(t, root, tc.path)
		got := hintsFor(command)
		for _, want := range tc.want {
			if !strings.Contains(got, want) {
				t.Errorf("%v hints = %q, want it to name %q", tc.path, got, want)
			}
		}
	}
}
