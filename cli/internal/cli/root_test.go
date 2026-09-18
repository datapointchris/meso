package cli

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

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
