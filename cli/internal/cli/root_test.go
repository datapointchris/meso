package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/datapointchris/goclikit"
	"github.com/datapointchris/goselfupdate/autoupdate"
)

// runLine drives the real tree through run, the path the shipped binary takes,
// with the version check suppressed. goclikit.Execute resolves the command
// from os.Args, so the line goes there rather than through SetArgs.
func runLine(t *testing.T, args ...string) error {
	t.Helper()
	original := os.Args
	os.Args = append([]string{"meso"}, args...)
	t.Cleanup(func() { os.Args = original })

	root := NewRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return run(root, autoupdate.Config{Suppress: true})
}

// A flag the leaf would have taken, typed after a word naming no subcommand,
// leaves the word as the mistake rather than the flag.
func TestAFlagAfterAnUnknownWordRefusesTheWord(t *testing.T) {
	for _, args := range [][]string{
		{"admin", "bogus", "--json"},
		{"sessions", "bogus", "--json"},
	} {
		err := runLine(t, args...)
		if err == nil || !strings.Contains(err.Error(), `unknown command "bogus"`) {
			t.Errorf("%v answered %v, want it to refuse \"bogus\"", args, err)
		}
		if !errors.Is(err, goclikit.ErrUsage) {
			t.Errorf("%v is not a usage error, so it would not exit 2: %v", args, err)
		}
	}
}

// A word one slip from a subcommand is answered with the subcommand, at the
// root and inside a group alike.
func TestAnUnknownSubcommandNamesTheNearOnes(t *testing.T) {
	for _, c := range []struct {
		args  []string
		meant string
	}{
		{[]string{"cyclea"}, "cycles"},
		{[]string{"sessions", "lisy"}, "list"},
	} {
		root := NewRootCommand()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs(c.args)
		err := root.Execute()
		if err == nil || !slices.Contains(strings.Fields(err.Error()), c.meant) {
			t.Errorf("%v answered %v, want it to name %q", c.args, err, c.meant)
		}
	}
}
