package cli

import (
	"bytes"
	"strings"
	"testing"

	"meso/cli/internal/api"
)

func strptr(s string) *string { return &s }

func TestPrintLogTable(t *testing.T) {
	var buf bytes.Buffer
	printLogTable(&buf, []api.LogEntry{
		{ID: "018f-a", EntryDate: "2026-07-25", Body: "Mobility work\nsecond line", Tags: []string{"mobility"}},
		{ID: "018f-b", EntryDate: "2026-07-20", Body: "Deadlifts moved well", Tags: []string{"strength", "knee"}, Mood: strptr("focused")},
	})
	out := buf.String()
	for _, want := range []string{"DATE", "MOOD", "TAGS", "ENTRY", "2026-07-25", "mobility", "strength, knee", "focused", "Deadlifts moved well"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
	// The preview is the first line only — the second line must not appear.
	if strings.Contains(out, "second line") {
		t.Errorf("preview should show only the first body line:\n%s", out)
	}

	var empty bytes.Buffer
	printLogTable(&empty, nil)
	if !strings.Contains(empty.String(), "No log entries match") {
		t.Errorf("empty message missing: %q", empty.String())
	}
}

func TestPrintLogDetail(t *testing.T) {
	var buf bytes.Buffer
	printLogDetail(&buf, api.LogEntry{
		ID: "018f-b", EntryDate: "2026-07-20",
		Body: "Deadlifts moved well.\nKnee-to-wall symmetric.", Tags: []string{"strength"}, Mood: strptr("focused"),
	})
	out := buf.String()
	for _, want := range []string{"2026-07-20", "018f-b", "focused", "strength", "Deadlifts moved well.", "Knee-to-wall symmetric."} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}

	// An entry with no mood/tags renders em dashes, not "<nil>" or an empty gap.
	var bare bytes.Buffer
	printLogDetail(&bare, api.LogEntry{ID: "018f-c", EntryDate: "2026-07-24", Body: "note"})
	if !strings.Contains(bare.String(), "—") {
		t.Errorf("bare entry should show em dashes:\n%s", bare.String())
	}
}

func TestLogPreview(t *testing.T) {
	if got := preview("first line\nsecond line"); got != "first line" {
		t.Errorf("preview multi-line = %q", got)
	}
	if got := preview("   "); got != "—" {
		t.Errorf("preview blank = %q", got)
	}
	long := strings.Repeat("a", 80)
	got := preview(long)
	if r := []rune(got); len(r) != 60 || r[59] != '…' {
		t.Errorf("preview long = %q (len %d)", got, len([]rune(got)))
	}
}
