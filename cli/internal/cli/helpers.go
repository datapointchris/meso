package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// usageArgs wraps a positional-args validator so a violation (wrong count, etc.)
// surfaces as a usageError → exit 2, matching how flag errors are classified.
// Cobra's built-in validators return plain errors that would otherwise exit 1.
func usageArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			return usageError{err}
		}
		return nil
	}
}

// encodeJSON writes v as indented JSON — the scripting-friendly output shared by
// every resource command's --json flag.
func encodeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// orDash renders an empty string as an em dash so blank table columns read
// cleanly.
func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// orDashPtr renders a nil/empty string pointer as an em dash.
func orDashPtr(s *string) string {
	if s == nil || *s == "" {
		return "—"
	}
	return *s
}

// orDashIntPtr renders a nil int pointer as an em dash.
func orDashIntPtr(n *int) string {
	if n == nil {
		return "—"
	}
	return strconv.Itoa(*n)
}

// yesNo renders a bool as a compact table glyph.
func yesNo(b bool) string {
	if b {
		return "★"
	}
	return ""
}

// confirm prompts for a yes/no answer on the given reader, defaulting to no. Used
// by destructive commands unless --yes is passed.
func confirm(in io.Reader, out io.Writer, prompt string) bool {
	fmt.Fprintf(out, "%s [y/N]: ", prompt)
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}
