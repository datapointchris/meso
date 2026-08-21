package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func squatCandidates() []nameRef {
	return []nameRef{
		{ID: 7, Name: "Front Squat"},
		{ID: 12, Name: "Back Squat"},
		{ID: 19, Name: "Squat"},
	}
}

var movementLookup = nameLookup{noun: "movement", createFlags: "--kind exercise"}

// TestResolveNameArg_Exact is why an exact match short-circuits: "Squat" is a
// substring of two longer names, so a contains-only rule would call it ambiguous
// and refuse the one name the caller typed in full.
func TestResolveNameArg_Exact(t *testing.T) {
	id, err := resolveNameArg(io.Discard, "squat", squatCandidates(), movementLookup)
	if err != nil {
		t.Fatalf("resolveNameArg: %v", err)
	}
	if id != 19 {
		t.Errorf("id = %d, want 19", id)
	}
}

func TestResolveNameArg_UniqueSubstring(t *testing.T) {
	id, err := resolveNameArg(io.Discard, "front", squatCandidates(), movementLookup)
	if err != nil {
		t.Fatalf("resolveNameArg: %v", err)
	}
	if id != 7 {
		t.Errorf("id = %d, want 7", id)
	}
}

// TestResolveNameArg_Ambiguous is the whole point of the presentation: an
// ambiguous name is a choice the caller has not made yet, not a mistake they
// made. Every candidate comes back as a command to run, and nothing on screen
// tells them they did something wrong.
func TestResolveNameArg_Ambiguous(t *testing.T) {
	var buf bytes.Buffer
	candidates := []nameRef{{ID: 7, Name: "Front Squat"}, {ID: 12, Name: "Back Squat"}}
	_, err := resolveNameArg(&buf, "squat", candidates, movementLookup)

	var ec exitCode
	if !errors.As(err, &ec) || ec != 2 {
		t.Fatalf("want exitCode(2) so Execute prints no error line, got %T: %v", err, err)
	}
	out := buf.String()
	for _, want := range []string{
		`2 movements match "squat". Show one:`,
		"meso movements show 7",
		"Front Squat",
		"meso movements show 12",
		"Back Squat",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("candidates missing %q:\n%s", want, out)
		}
	}
	assertNoReprimand(t, out)
	t.Logf("ambiguous:\n%s", out)
}

// TestResolveNameArg_NoMatch checks the dead end has two ways out: browse what
// exists, or create what does not. The create line is pre-filled with the name
// they typed, so adding it is a copy rather than a retype.
func TestResolveNameArg_NoMatch(t *testing.T) {
	var buf bytes.Buffer
	_, err := resolveNameArg(&buf, "burpee", nil, movementLookup)

	var ec exitCode
	if !errors.As(err, &ec) || ec != 1 {
		t.Fatalf("want exitCode(1), got %T: %v", err, err)
	}
	out := buf.String()
	for _, want := range []string{
		`No movement matches "burpee". From here:`,
		"meso movements list",
		`meso movements create "burpee" --kind exercise`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no-match guidance missing %q:\n%s", want, out)
		}
	}
	assertNoReprimand(t, out)
	t.Logf("no match:\n%s", out)
}

// TestResolveNameArg_NoCreateFlags covers a noun whose create takes the name
// alone, so no stray flags are appended.
func TestResolveNameArg_NoCreateFlags(t *testing.T) {
	var buf bytes.Buffer
	_, _ = resolveNameArg(&buf, "leg day", nil, nameLookup{noun: "workout"})
	if !strings.Contains(buf.String(), `meso workouts create "leg day"`+"\t") &&
		!strings.Contains(buf.String(), `meso workouts create "leg day"  `) {
		t.Errorf("workout create line malformed:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "--kind") {
		t.Errorf("movement-only flags leaked onto a workout:\n%s", buf.String())
	}
}

// TestResolveNameArg_TagOnlyHits covers a query the server matched on tags: no
// candidate name contains it, so all of them are offered rather than the search
// reporting nothing.
func TestResolveNameArg_TagOnlyHits(t *testing.T) {
	var buf bytes.Buffer
	candidates := []nameRef{{ID: 7, Name: "Front Squat"}, {ID: 12, Name: "Leg Press"}}
	_, err := resolveNameArg(&buf, "legs", candidates, movementLookup)
	if err == nil {
		t.Fatal("two tag hits resolved to one movement")
	}
	if !strings.Contains(buf.String(), "Front Squat") || !strings.Contains(buf.String(), "Leg Press") {
		t.Errorf("tag-only hits dropped from the candidates:\n%s", buf.String())
	}
}

func TestResolveNameArg_SingleTagHit(t *testing.T) {
	id, err := resolveNameArg(io.Discard, "posterior-chain", []nameRef{{ID: 3, Name: "Romanian Deadlift"}}, movementLookup)
	if err != nil {
		t.Fatalf("resolveNameArg: %v", err)
	}
	if id != 3 {
		t.Errorf("id = %d, want 3", id)
	}
}

// assertNoReprimand guards the design cue this presentation exists for: a caller
// who typed a valid command is never told they erred, and is never left without
// a command to run next.
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
