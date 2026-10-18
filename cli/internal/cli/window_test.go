package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/datapointchris/goclikit"
	"github.com/spf13/cobra"
)

func TestTimeWindow(t *testing.T) {
	loc := time.FixedZone("UTC-4", -4*60*60)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, loc) }

	cases := []struct {
		name         string
		sinceDefault string
		args         []string
		since, until time.Time
		err          string
	}{
		{name: "seconds", args: []string{"--since", "90s"}, since: now.Add(-90 * time.Second)},
		{name: "m is minutes", args: []string{"--since", "90m"}, since: now.Add(-90 * time.Minute)},
		{name: "hours", args: []string{"--since", "36h"}, since: now.Add(-36 * time.Hour)},
		{name: "days", args: []string{"--since", "3d"}, since: now.Add(-72 * time.Hour)},
		{name: "weeks", args: []string{"--since", "2w"}, since: now.Add(-14 * 24 * time.Hour)},
		{name: "a month unit is refused", args: []string{"--since", "6mo"}, err: "no month or year"},
		{
			name: "a day on each flag holds the whole --until day",
			args: []string{"--since", "2026-10-01", "--until", "2026-10-07"}, since: day(10, 1), until: day(10, 8),
		},
		{
			name: "an instant", args: []string{"--since", "2026-10-01T09:30:00Z"},
			since: time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC),
		},
		{name: "the default applies alone", sinceDefault: "30d", since: now.Add(-30 * 24 * time.Hour)},
		{
			name: "--until alone drops the default", sinceDefault: "30d",
			args: []string{"--until", "2026-10-07"}, until: day(10, 8),
		},
		{name: "an empty window is refused", args: []string{"--since", "1d", "--until", "2d"}, err: "is not before"},
		{name: "an unknown form is refused", args: []string{"--since", "last week"}, err: "want a span"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "read"}
			var w timeWindow
			w.bind(cmd, tc.sinceDefault)
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			got, err := w.resolve(cmd, now)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) || !errors.Is(err, goclikit.ErrUsage) {
					t.Fatalf("err = %v, want a usage error containing %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if !got.Since.Equal(tc.since) || !got.Until.Equal(tc.until) {
				t.Errorf("window = %v → %v, want %v → %v", got.Since, got.Until, tc.since, tc.until)
			}
		})
	}
}
