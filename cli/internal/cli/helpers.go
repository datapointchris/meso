package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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

// confirm asks prompt and reports whether the answer approved. Destructive verbs
// call it unless --yes bypasses it.
//
// A prompt is only ever offered on an interactive stdin. Prompting a
// non-interactive caller blocks on a stdin that never closes, leaving it with no
// output and no exit code — the one failure a caller cannot recover from, and
// the reason the gate lives in here rather than at the seven call sites. A
// closed stdin was worse than a hang before this: the scanner returned false, so
// the delete reported itself aborted rather than saying --yes was needed.
func confirm(cmd *cobra.Command, prompt string) (bool, error) {
	if !interactive(cmd) {
		return false, fmt.Errorf("refusing to prompt without an interactive terminal; pass --yes to confirm")
	}
	return readConfirmation(cmd.ErrOrStderr(), cmd.InOrStdin(), prompt)
}

// interactive reports whether the command may prompt: --no-input never may, and
// otherwise stdin has to be a terminal. A reader a test substituted is not an
// *os.File, so it reads as non-interactive — which is what makes the gate
// testable without a pty.
func interactive(cmd *cobra.Command) bool {
	if noInput {
		return false
	}
	file, ok := cmd.InOrStdin().(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

// readConfirmation prints prompt to out and reads a yes/no answer from in. Only
// "y"/"yes" (case-insensitive) approves; EOF or anything else declines. Split
// from confirm so the parsing stays testable on plain buffers.
func readConfirmation(out io.Writer, in io.Reader, prompt string) (bool, error) {
	_, _ = fmt.Fprintf(out, "%s [y/N]: ", prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
