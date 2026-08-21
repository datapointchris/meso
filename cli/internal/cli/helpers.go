package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/datapointchris/meso/cli/internal/api"
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
	Name string `json:"name"`
	ID   int64  `json:"id"`
}

// searchKey mirrors searchNormalize in api/repository/movements.go: lowercase,
// then drop everything that is not a letter or digit. The server compares a
// normalized query against a column normalized this way, so a client that
// narrows the result with raw text disagrees with the search that produced it —
// "push up" would fail to match the row "Push-up" that the server did match.
func searchKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// searchTerms mirrors searchTokens in the same file: the query splits on
// whitespace and each field normalizes to a term that has to appear in the name.
// A query of only separators yields none, which is the server reading it as no
// filter at all rather than as a query that matched everything.
func searchTerms(query string) []string {
	terms := []string{}
	for _, field := range strings.Fields(query) {
		if key := searchKey(field); key != "" {
			terms = append(terms, key)
		}
	}
	return terms
}

// nameResolver is the per-resource half of resolving an <id-or-name> argument.
// The judgement below is shared; what differs by noun is how to fetch one record,
// how to search by name, how to read a candidate's id and name back out, and what
// `create` needs beyond the name.
type nameResolver[T any] struct {
	noun        string
	createFlags string
	get         func(context.Context, int64) (T, error)
	search      func(context.Context, string) ([]T, error)
	ref         func(T) nameRef
}

// resolveIDOrName returns the record a `show`-style <id-or-name> argument names.
//
// A numeric argument is tried as an id first, and a 404 on that falls through to
// the name search rather than surfacing: bare integers are ordinary names in this
// domain, so a workout called "300" and a cycle called "2026" both have to be
// reachable. Everything else searches server-side, then narrows the result using
// the server's own matching rule — an exact name wins outright, so a name that is
// also a substring of longer ones still resolves.
//
// A query that lands on several records, or on none, is not a mistake the caller
// made; they typed a command the CLI invited. So nothing here is phrased as a
// failure. They get the commands that carry on from where they already are, and
// the returned exitCode stops the run without Execute printing an "error:" line
// above them. Under --json those commands are emitted as data instead, because a
// caller who asked for JSON cannot use a menu.
func resolveIDOrName[T any](ctx context.Context, cmd *cobra.Command, raw string, asJSON bool, r nameResolver[T]) (T, error) {
	var zero T
	if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
		record, getErr := r.get(ctx, id)
		if getErr == nil {
			return record, nil
		}
		var apiErr *api.APIError
		if !errors.As(getErr, &apiErr) || !apiErr.NotFound() {
			return zero, handleAPIError(getErr)
		}
	}

	terms := searchTerms(raw)
	if len(terms) == 0 {
		printNoNameMatch(cmd, raw, false, asJSON, r)
		return zero, exitCode(1)
	}

	rows, err := r.search(ctx, raw)
	if err != nil {
		return zero, handleAPIError(err)
	}
	candidates := make([]nameRef, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, r.ref(row))
	}

	id, err := narrowToOne(cmd, raw, terms, candidates, asJSON, r)
	if err != nil {
		return zero, err
	}
	// Refetch rather than returning the list row: a list omits what only the
	// detail endpoint attaches, and movements' `related` is the live case.
	record, err := r.get(ctx, id)
	if err != nil {
		return zero, handleAPIError(err)
	}
	return record, nil
}

// narrowToOne picks the single candidate the query meant, or writes what to run
// next and returns the code to stop on.
func narrowToOne[T any](cmd *cobra.Command, raw string, terms []string, candidates []nameRef, asJSON bool, r nameResolver[T]) (int64, error) {
	key := searchKey(raw)
	for _, c := range candidates {
		if searchKey(c.Name) == key {
			return c.ID, nil
		}
	}

	byName := make([]nameRef, 0, len(candidates))
	for _, c := range candidates {
		if matchesAllTerms(c.Name, terms) {
			byName = append(byName, c)
		}
	}
	// The server searches tags as well as names, so a query that hit only tags
	// leaves this empty. Those rows really did match, so they are offered.
	if len(byName) == 0 {
		byName = candidates
	}

	switch len(byName) {
	case 1:
		return byName[0].ID, nil
	case 0:
		printNoNameMatch(cmd, raw, true, asJSON, r)
		return 0, exitCode(1)
	default:
		printNameCandidates(cmd, raw, byName, asJSON, r)
		return 0, exitCode(1)
	}
}

// matchesAllTerms applies the server's token-AND rule to one name.
func matchesAllTerms(name string, terms []string) bool {
	key := searchKey(name)
	for _, t := range terms {
		if !strings.Contains(key, t) {
			return false
		}
	}
	return true
}

// shellArg renders a value for a command the caller is invited to paste back.
// Anything outside the safe subset is quoted, because a name with a space in it
// splits into two arguments and the pasted command then fails on its arity.
// A value already safe is left bare — quoting `1` would read as the tool not
// knowing an id from a name.
func shellArg(s string) string {
	if s == "" {
		return `""`
	}
	for _, r := range s {
		safe := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' || r == '/'
		if !safe {
			return strconv.Quote(s)
		}
	}
	return s
}

// showPath and groupPath come from the command tree rather than a literal, so a
// renamed group cannot leave the menus printing a command that no longer exists.
func showPath(cmd *cobra.Command) string { return cmd.CommandPath() }

func groupPath(cmd *cobra.Command) string {
	if parent := cmd.Parent(); parent != nil {
		return parent.CommandPath()
	}
	return cmd.CommandPath()
}

// printNameCandidates offers one runnable command per candidate, so narrowing an
// ambiguous name costs a copy rather than a fresh search. Every value the caller
// supplied is quoted, because a name with a space in it has to survive the shell.
func printNameCandidates[T any](cmd *cobra.Command, raw string, refs []nameRef, asJSON bool, r nameResolver[T]) {
	if asJSON {
		_ = encodeJSON(cmd.OutOrStdout(), map[string]any{"query": raw, "matches": refs})
		return
	}
	out := cmd.ErrOrStderr()
	_, _ = fmt.Fprintf(out, "%d %ss match %q. Show one:\n\n", len(refs), r.noun, raw)
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	for _, ref := range refs {
		_, _ = fmt.Fprintf(tw, "  %s %d\t%s\n", showPath(cmd), ref.ID, ref.Name)
	}
	_ = tw.Flush()
}

// printNoNameMatch offers the two things worth doing when a name is absent:
// browse what does exist, or add the thing that does not. searchable is false
// when the argument held nothing to search on, where offering to create it by
// that name would be offering nonsense.
func printNoNameMatch[T any](cmd *cobra.Command, raw string, searchable bool, asJSON bool, r nameResolver[T]) {
	if asJSON {
		_ = encodeJSON(cmd.OutOrStdout(), map[string]any{"query": raw, "matches": []nameRef{}})
		return
	}
	out := cmd.ErrOrStderr()
	_, _ = fmt.Fprintf(out, "No %s matches %q. From here:\n\n", r.noun, raw)
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "  %s list\tbrowse every %s\n", groupPath(cmd), r.noun)
	if searchable {
		create := fmt.Sprintf("  %s create %s", groupPath(cmd), shellArg(raw))
		if r.createFlags != "" {
			create += " " + r.createFlags
		}
		_, _ = fmt.Fprintf(tw, "%s\tadd it\n", create)
	}
	_ = tw.Flush()
}
