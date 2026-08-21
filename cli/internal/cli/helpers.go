package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

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

// nameRef is one candidate for resolving a name argument: the id a caller types
// and the name they searched for.
type nameRef struct {
	Name string
	ID   int64
}

// nameLookup is what a name argument needs beyond its candidates: the singular
// noun for prose, and any flags `create` requires, so a query that matched
// nothing can offer to add what is missing.
type nameLookup struct {
	noun        string
	createFlags string
}

// namespace is the command group the noun owns, which is its plural.
func (l nameLookup) namespace() string { return "meso " + l.noun + "s" }

// resolveNameArg turns a non-numeric <id-or-name> argument into an id. An exact
// case-insensitive name wins outright, so a name that is also a prefix of longer
// ones still resolves; otherwise the candidates whose name contains the query
// have to narrow to one.
//
// A query that lands on several records, or on none, is not a mistake the caller
// made — they typed a command the CLI invited. So nothing here is phrased as a
// failure: out gets the commands that carry on from where they already are, one
// per candidate or the way to create what is missing, and the returned exitCode
// stops the run without Execute printing an "error:" line above them.
//
// candidates comes from the server-side search, which matches tags too. A query
// that hit only tags leaves the contains-filter empty, and those hits are then
// offered rather than reported as no match.
func resolveNameArg(out io.Writer, raw string, candidates []nameRef, l nameLookup) (int64, error) {
	query := strings.ToLower(strings.TrimSpace(raw))
	for _, c := range candidates {
		if strings.ToLower(c.Name) == query {
			return c.ID, nil
		}
	}

	byName := make([]nameRef, 0, len(candidates))
	for _, c := range candidates {
		if strings.Contains(strings.ToLower(c.Name), query) {
			byName = append(byName, c)
		}
	}
	if len(byName) == 0 {
		byName = candidates
	}

	switch len(byName) {
	case 1:
		return byName[0].ID, nil
	case 0:
		printNoNameMatch(out, raw, l)
		return 0, exitCode(1)
	default:
		printNameCandidates(out, raw, byName, l)
		return 0, exitCode(2)
	}
}

// printNameCandidates offers one runnable command per candidate, so narrowing an
// ambiguous name costs a copy rather than a fresh search.
func printNameCandidates(out io.Writer, raw string, refs []nameRef, l nameLookup) {
	_, _ = fmt.Fprintf(out, "%d %ss match %q. Show one:\n\n", len(refs), l.noun, raw)
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	for _, r := range refs {
		_, _ = fmt.Fprintf(tw, "  %s show %d\t%s\n", l.namespace(), r.ID, r.Name)
	}
	_ = tw.Flush()
}

// printNoNameMatch offers the two things worth doing when a name is absent:
// browse what does exist, or add the thing that does not.
func printNoNameMatch(out io.Writer, raw string, l nameLookup) {
	_, _ = fmt.Fprintf(out, "No %s matches %q. From here:\n\n", l.noun, raw)
	create := fmt.Sprintf("  %s create %q", l.namespace(), raw)
	if l.createFlags != "" {
		create += " " + l.createFlags
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "  %s list\tbrowse every %s\n", l.namespace(), l.noun)
	_, _ = fmt.Fprintf(tw, "%s\tadd it\n", create)
	_ = tw.Flush()
}
