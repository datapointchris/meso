package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"meso/cli/internal/api"
)

func TestPrintFeedbackTable(t *testing.T) {
	var buf bytes.Buffer
	printFeedbackTable(&buf, []api.Feedback{
		{ID: "019f-a", Status: "open", Body: "Timer resets on sleep\nhappens every time", ContextPath: "/sessions/3", CreatedAt: "2026-07-25T14:02:11Z"},
		{ID: "019f-b", Status: "done", Body: "Cycle detail should show the week", CreatedAt: "2026-07-20T09:15:00Z"},
	})
	out := buf.String()
	for _, want := range []string{"STATUS", "CAPTURED", "WHERE", "open", "done", "2026-07-25", "/sessions/3", "Timer resets on sleep"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
	// The timestamp is trimmed to a date — the time of day is never what's scanned for.
	if strings.Contains(out, "14:02:11") {
		t.Errorf("table should show the date only:\n%s", out)
	}
	// The preview is the first line only.
	if strings.Contains(out, "happens every time") {
		t.Errorf("preview should show only the first body line:\n%s", out)
	}
	// Feedback with no route renders an em dash, not an empty column.
	if !strings.Contains(out, "—") {
		t.Errorf("missing context path should render an em dash:\n%s", out)
	}

	var empty bytes.Buffer
	printFeedbackTable(&empty, nil)
	if !strings.Contains(empty.String(), "No feedback matches") {
		t.Errorf("empty message missing: %q", empty.String())
	}
}

func TestPrintFeedbackDetail(t *testing.T) {
	var buf bytes.Buffer
	printFeedbackDetail(&buf, api.Feedback{
		ID: "019f-a", Status: "open", ContextPath: "/cycles/2",
		Body: "Week number is missing.\nHard to tell where I am.", CreatedAt: "2026-07-25T14:02:11Z",
	})
	out := buf.String()
	for _, want := range []string{"019f-a", "open", "2026-07-25", "/cycles/2", "Week number is missing.", "Hard to tell where I am."} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}
}

func TestPrintFeedbackDetailViewport(t *testing.T) {
	width := 390
	height := 844

	var phone bytes.Buffer
	printFeedbackDetail(&phone, api.Feedback{
		ID: "019f-a", Status: "open", ContextPath: "/movements/21",
		Body: "wall of text", CreatedAt: "2026-07-26T17:15:10Z",
		ViewportWidth: &width, ViewportHeight: &height,
	})
	if !strings.Contains(phone.String(), "390 × 844") {
		t.Errorf("detail missing the viewport:\n%s", phone.String())
	}

	// A CLI capture has no window to measure — an em dash, not a bogus 0 × 0.
	var terminal bytes.Buffer
	printFeedbackDetail(&terminal, api.Feedback{
		ID: "019f-b", Status: "open", Body: "filed from a shell", CreatedAt: "2026-07-26T17:15:10Z",
	})
	if !strings.Contains(terminal.String(), "viewport:  —") {
		t.Errorf("absent viewport should render an em dash:\n%s", terminal.String())
	}
}

func TestFeedbackDateOf(t *testing.T) {
	if got := dateOf("2026-07-25T14:02:11Z"); got != "2026-07-25" {
		t.Errorf("dateOf(timestamp) = %q", got)
	}
	if got := dateOf(""); got != "—" {
		t.Errorf("dateOf(empty) = %q, want an em dash", got)
	}
}

// The admin namespace exists to keep the top-level help a description of the training
// domain. If feedback ever gets registered at the root, that intent is silently lost.
func TestAdminNamespaceOwnsFeedback(t *testing.T) {
	root := NewRootCommand()

	for _, cmd := range root.Commands() {
		if cmd.Name() == "feedback" {
			t.Error("feedback is registered at the root; it belongs under `admin`")
		}
	}

	admin := findCommand(root.Commands(), "admin")
	if admin == nil {
		t.Fatal("admin command not registered on root")
	}
	if findCommand(admin.Commands(), "feedback") == nil {
		t.Error("feedback not registered under admin")
	}
}

func findCommand(commands []*cobra.Command, name string) *cobra.Command {
	for _, cmd := range commands {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func TestAdminFeedbackListRejectsConflictingFilters(t *testing.T) {
	root := NewRootCommand()
	root.SetArgs([]string{"admin", "feedback", "list", "--open", "--done"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected --open with --done to be rejected")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error: %v", err)
	}
}
