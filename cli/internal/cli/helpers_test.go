package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

// fakeLibrary drives resolveIDOrName without a client: get and search close over
// a fixed set of rows, and every call is counted so a test can assert that the
// numeric path costs one request and the name path does not fetch twice.
type fakeLibrary struct {
	rows     []api.Movement
	gets     int
	searches int
}

func (f *fakeLibrary) get(_ context.Context, id int64) (api.Movement, error) {
	f.gets++
	for _, m := range f.rows {
		if m.ID == id {
			return m, nil
		}
	}
	return api.Movement{}, &api.APIError{StatusCode: http.StatusNotFound}
}

// search applies the server's own rule — token-AND over a normalized name — so
// the fake and api/repository/movements.go agree about what a query matches.
func (f *fakeLibrary) search(_ context.Context, q string) ([]api.Movement, error) {
	f.searches++
	terms := searchTerms(q)
	if len(terms) == 0 {
		return f.rows, nil // a separator-only query is no filter at all
	}
	var out []api.Movement
	for _, m := range f.rows {
		if matchesAllTerms(m.Name, terms) {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeLibrary) resolver() nameResolver[api.Movement] {
	return nameResolver[api.Movement]{
		noun:        "movement",
		createFlags: "--kind exercise",
		get:         f.get,
		search:      f.search,
		ref:         func(m api.Movement) nameRef { return nameRef{ID: m.ID, Name: m.Name} },
	}
}

// showCommand returns a real `meso movements show` node, so the menus under test
// derive their command path from the tree the binary actually ships.
func showCommand(t *testing.T, stdout, stderr *bytes.Buffer) *cobra.Command {
	t.Helper()
	root := NewRootCommand()
	cmd, _, err := root.Find([]string{"movements", "show"})
	if err != nil {
		t.Fatalf("finding movements show: %v", err)
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd
}

func resolve(t *testing.T, lib *fakeLibrary, raw string, asJSON bool) (api.Movement, string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := showCommand(t, &stdout, &stderr)
	got, err := resolveIDOrName(context.Background(), cmd, raw, asJSON, lib.resolver())
	return got, stdout.String(), stderr.String(), err
}

func squatLibrary() *fakeLibrary {
	return &fakeLibrary{rows: []api.Movement{
		{ID: 7, Name: "Front Squat"},
		{ID: 12, Name: "Back Squat"},
		{ID: 19, Name: "Squat"},
	}}
}

func TestResolveIDOrName_Numeric(t *testing.T) {
	lib := squatLibrary()
	got, _, _, err := resolve(t, lib, "12", false)
	if err != nil {
		t.Fatalf("resolveIDOrName: %v", err)
	}
	if got.ID != 12 {
		t.Errorf("id = %d, want 12", got.ID)
	}
	if lib.searches != 0 {
		t.Errorf("a numeric id should not search, got %d searches", lib.searches)
	}
	if lib.gets != 1 {
		t.Errorf("want exactly 1 fetch, got %d", lib.gets)
	}
}

// TestResolveIDOrName_NumericName covers a record whose name is digits. The id
// lookup 404s and the search has to run anyway, because "300" is an ordinary
// workout name and "2026" an ordinary cycle name.
func TestResolveIDOrName_NumericName(t *testing.T) {
	lib := &fakeLibrary{rows: []api.Movement{{ID: 5, Name: "300"}}}
	got, _, _, err := resolve(t, lib, "300", false)
	if err != nil {
		t.Fatalf("a record named 300 was unreachable: %v", err)
	}
	if got.ID != 5 {
		t.Errorf("id = %d, want 5", got.ID)
	}
}

// TestResolveIDOrName_Punctuation is the mismatch the client's own narrowing
// used to introduce: the server matches "push up" against "Push-up" because both
// normalize to "pushup", and a raw-string comparison client-side does not.
func TestResolveIDOrName_Punctuation(t *testing.T) {
	lib := &fakeLibrary{rows: []api.Movement{
		{ID: 7, Name: "Push-up"},
		{ID: 12, Name: "Wide Push-up"},
	}}
	got, _, stderr, err := resolve(t, lib, "push up", false)
	if err != nil {
		t.Fatalf("exact name after normalization did not resolve: %v\n%s", err, stderr)
	}
	if got.ID != 7 {
		t.Errorf("id = %d, want 7", got.ID)
	}
}

// TestResolveIDOrName_Exact is why the exact test runs before the contains test:
// "Squat" is a substring of two longer names, and narrowing alone would refuse
// the name typed in full.
func TestResolveIDOrName_Exact(t *testing.T) {
	got, _, _, err := resolve(t, squatLibrary(), "squat", false)
	if err != nil {
		t.Fatalf("resolveIDOrName: %v", err)
	}
	if got.ID != 19 {
		t.Errorf("id = %d, want 19", got.ID)
	}
}

func TestResolveIDOrName_UniqueSubstring(t *testing.T) {
	got, _, _, err := resolve(t, squatLibrary(), "front", false)
	if err != nil {
		t.Fatalf("resolveIDOrName: %v", err)
	}
	if got.ID != 7 {
		t.Errorf("id = %d, want 7", got.ID)
	}
}

// TestResolveIDOrName_TokensInAnyOrder mirrors the server's token-AND rule, which
// is what lets "straight arm pulldown" reach "Eccentric Straight-Arm Pulldown".
func TestResolveIDOrName_TokensInAnyOrder(t *testing.T) {
	lib := &fakeLibrary{rows: []api.Movement{{ID: 4, Name: "Eccentric Straight-Arm Pulldown"}}}
	got, _, stderr, err := resolve(t, lib, "pulldown straight arm", false)
	if err != nil {
		t.Fatalf("token-AND did not reach the row: %v\n%s", err, stderr)
	}
	if got.ID != 4 {
		t.Errorf("id = %d, want 4", got.ID)
	}
}

// TestResolveIDOrName_Ambiguous is the presentation rule: an ambiguous name is a
// choice not yet made, not a mistake. Every candidate comes back as a runnable
// command, and the exit code is the not-found 1 rather than the usage-error 2,
// which stays with what usageArgs classifies.
func TestResolveIDOrName_Ambiguous(t *testing.T) {
	lib := &fakeLibrary{rows: []api.Movement{
		{ID: 7, Name: "Front Squat"},
		{ID: 12, Name: "Back Squat"},
	}}
	_, _, stderr, err := resolve(t, lib, "squat", false)

	var ec exitCode
	if !errors.As(err, &ec) || ec != 1 {
		t.Fatalf("want exitCode(1), got %T: %v", err, err)
	}
	for _, want := range []string{
		`2 movements match "squat". Show one:`,
		"meso movements show 7",
		"Front Squat",
		"meso movements show 12",
		"Back Squat",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("candidates missing %q:\n%s", want, stderr)
		}
	}
	assertNoReprimand(t, stderr)
	t.Logf("ambiguous:\n%s", stderr)
}

// TestResolveIDOrName_NoMatch checks the dead end has two ways out: browse what
// exists, or create what does not, pre-filled and quoted with the name typed.
func TestResolveIDOrName_NoMatch(t *testing.T) {
	_, _, stderr, err := resolve(t, squatLibrary(), "burpee", false)

	var ec exitCode
	if !errors.As(err, &ec) || ec != 1 {
		t.Fatalf("want exitCode(1), got %T: %v", err, err)
	}
	for _, want := range []string{
		`No movement matches "burpee". From here:`,
		"meso movements list",
		"meso movements create burpee --kind exercise",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("no-match guidance missing %q:\n%s", want, stderr)
		}
	}
	assertNoReprimand(t, stderr)
	t.Logf("no match:\n%s", stderr)
}

// TestResolveIDOrName_UnsearchableQuery covers an argument that normalizes to
// nothing. The server drops such a search and returns the whole table, so
// reporting those rows as matches would claim every record matched "".
func TestResolveIDOrName_UnsearchableQuery(t *testing.T) {
	for _, raw := range []string{"", "-", "  "} {
		t.Run("query="+raw, func(t *testing.T) {
			lib := squatLibrary()
			_, _, stderr, err := resolve(t, lib, raw, false)

			var ec exitCode
			if !errors.As(err, &ec) || ec != 1 {
				t.Fatalf("want exitCode(1), got %T: %v", err, err)
			}
			if strings.Contains(stderr, "match") && strings.Contains(stderr, "Show one") {
				t.Errorf("an unfiltered response was reported as matches:\n%s", stderr)
			}
			// Creating a record named "-" is nonsense, so no create line is offered.
			if strings.Contains(stderr, "create") {
				t.Errorf("offered to create an unsearchable name:\n%s", stderr)
			}
			if lib.searches != 0 {
				t.Errorf("a query with nothing to search on should not reach the API")
			}
		})
	}
}

// TestResolveIDOrName_SingleRowUnsearchable is the worst shape of the same bug:
// one row in the table used to make any unsearchable argument resolve to it,
// silently, with no output at all.
func TestResolveIDOrName_SingleRowUnsearchable(t *testing.T) {
	lib := &fakeLibrary{rows: []api.Movement{{ID: 42, Name: "Bench Press"}}}
	got, _, _, err := resolve(t, lib, "-", false)
	if err == nil {
		t.Fatalf("an unsearchable argument resolved to movement %d", got.ID)
	}
}

// TestResolveIDOrName_TagOnlyHits covers a query the server matched on tags: no
// candidate name carries it, so all of them are offered rather than the search
// reporting nothing.
func TestResolveIDOrName_TagOnlyHits(t *testing.T) {
	lib := &fakeLibrary{rows: []api.Movement{{ID: 7, Name: "Front Squat"}, {ID: 12, Name: "Leg Press"}}}
	// A tag search returns rows whose names do not carry the query.
	lib.rows = lib.rows[:2]
	tagResolver := lib.resolver()
	tagResolver.search = func(context.Context, string) ([]api.Movement, error) { return lib.rows, nil }

	var stdout, stderr bytes.Buffer
	cmd := showCommand(t, &stdout, &stderr)
	_, err := resolveIDOrName(context.Background(), cmd, "legs", false, tagResolver)
	if err == nil {
		t.Fatal("two tag hits resolved to one movement")
	}
	if !strings.Contains(stderr.String(), "Front Squat") || !strings.Contains(stderr.String(), "Leg Press") {
		t.Errorf("tag-only hits dropped from the candidates:\n%s", stderr.String())
	}
}

// TestResolveIDOrName_JSONCandidates covers the caller who asked for JSON: a menu
// of prose is unusable to them, so the candidates come back as data on stdout.
func TestResolveIDOrName_JSONCandidates(t *testing.T) {
	lib := &fakeLibrary{rows: []api.Movement{
		{ID: 7, Name: "Front Squat"},
		{ID: 12, Name: "Back Squat"},
	}}
	_, stdout, stderr, err := resolve(t, lib, "squat", true)
	if err == nil {
		t.Fatal("ambiguous name resolved")
	}
	if stderr != "" {
		t.Errorf("a --json caller got prose on stderr:\n%s", stderr)
	}
	for _, want := range []string{`"query": "squat"`, `"id": 7`, `"name": "Front Squat"`, `"id": 12`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("JSON candidates missing %q:\n%s", want, stdout)
		}
	}
	t.Logf("json candidates:\n%s", stdout)
}

// TestOfferedCommandsResolve is the check that could not exist while the command
// path was a string literal: every command a menu offers is looked up in the real
// tree, so renaming a group turns these red instead of leaving the menus pointing
// at a command that no longer exists.
func TestOfferedCommandsResolve(t *testing.T) {
	// Two rows and no exact match, so "squat" produces the candidate menu.
	ambiguousLib := &fakeLibrary{rows: []api.Movement{
		{ID: 7, Name: "Front Squat"},
		{ID: 12, Name: "Back Squat"},
	}}
	var offered []string

	_, _, ambiguous, _ := resolve(t, ambiguousLib, "squat", false)
	_, _, missing, _ := resolve(t, squatLibrary(), "burpee", false)
	for _, out := range []string{ambiguous, missing} {
		for _, line := range strings.Split(out, "\n") {
			if fields := strings.Fields(line); len(fields) > 0 && fields[0] == "meso" {
				offered = append(offered, line)
			}
		}
	}
	if len(offered) < 3 {
		t.Fatalf("expected the menus to offer commands, found %d:\n%s%s", len(offered), ambiguous, missing)
	}

	root := NewRootCommand()
	for _, line := range offered {
		path := commandWords(strings.Fields(line)[1:])
		if _, _, err := root.Find(path); err != nil {
			t.Errorf("offered command %q does not resolve: %v", strings.TrimSpace(line), err)
		}
	}
}

// commandWords keeps the leading bare words of an offered command — the command
// path — and drops the arguments, flags and trailing description after it.
func commandWords(fields []string) []string {
	var path []string
	for _, f := range fields {
		if strings.HasPrefix(f, "-") || strings.HasPrefix(f, `"`) || isAllDigits(f) {
			break
		}
		path = append(path, f)
	}
	return path
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// TestShellArg covers the values a menu interpolates. A name with a space is the
// case that made a printed command fail on its arity when pasted back.
func TestShellArg(t *testing.T) {
	cases := map[string]string{
		"1":            "1",
		"burpee":       "burpee",
		"heel-raise-l": "heel-raise-l",
		"push day":     `"push day"`,
		"a;rm -rf /":   `"a;rm -rf /"`,
		`say"what`:     `"say\"what"`,
		"":             `""`,
	}
	for in, want := range cases {
		if got := shellArg(in); got != want {
			t.Errorf("shellArg(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestSearchKey(t *testing.T) {
	cases := map[string]string{
		"Push-up":                         "pushup",
		"push up":                         "pushup",
		"Eccentric Straight-Arm Pulldown": "eccentricstraightarmpulldown",
		"-":                               "",
		"":                                "",
	}
	for in, want := range cases {
		if got := searchKey(in); got != want {
			t.Errorf("searchKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSearchTerms(t *testing.T) {
	if got := searchTerms("straight arm  pull-down"); !strings.EqualFold(strings.Join(got, ","), "straight,arm,pulldown") {
		t.Errorf("searchTerms = %v", got)
	}
	// A separator-only query is the server reading it as no filter at all.
	for _, raw := range []string{"", "  ", "-", " / "} {
		if got := searchTerms(raw); len(got) != 0 {
			t.Errorf("searchTerms(%q) = %v, want none", raw, got)
		}
	}
}

// assertNoReprimand guards the design cue this presentation exists for: a caller
// who typed a valid command is never told they erred, and is never left without a
// command to run next.
func assertNoReprimand(t *testing.T, out string) {
	t.Helper()
	for _, banned := range []string{"error", "invalid", "must ", "failed", "cannot", "--help"} {
		if strings.Contains(strings.ToLower(out), banned) {
			t.Errorf("output reprimands with %q:\n%s", banned, out)
		}
	}
	if !strings.Contains(out, "meso ") {
		t.Errorf("output offers no command to run next:\n%s", out)
	}
}
