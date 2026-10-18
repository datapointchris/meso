package cli

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/datapointchris/goclikit"
	"github.com/spf13/cobra"

	"github.com/datapointchris/meso/cli/internal/api"
)

const windowForms = "a span back from now (90m, 36h, 2w), a day (2026-07-01) or an RFC 3339 instant"

var (
	spanPattern = regexp.MustCompile(`^(\d+)([A-Za-z]+)$`)
	spanUnits   = map[string]time.Duration{
		"s": time.Second,
		"m": time.Minute,
		"h": time.Hour,
		"d": 24 * time.Hour,
		"w": 7 * 24 * time.Hour,
	}
)

// timeWindow is a read's --since and --until.
type timeWindow struct {
	since, until string
}

// bind registers both flags. sinceDefault is the read's default window, shown in help.
func (w *timeWindow) bind(cmd *cobra.Command, sinceDefault string) {
	f := cmd.Flags()
	f.StringVar(&w.since, "since", sinceDefault, "Start of the window: "+windowForms)
	f.StringVar(&w.until, "until", "", "End of the window, the whole day when it is a day: "+windowForms)
}

// resolve turns both flags into instants against now, in now's zone. --until alone runs
// from the earliest row, so it drops --since's default.
func (w *timeWindow) resolve(cmd *cobra.Command, now time.Time) (api.Window, error) {
	since := w.since
	if cmd.Flags().Changed("until") && !cmd.Flags().Changed("since") {
		since = ""
	}
	var out api.Window
	var err error
	if since != "" {
		if out.Since, err = resolveBound("since", since, now, false); err != nil {
			return api.Window{}, err
		}
	}
	if w.until != "" {
		if out.Until, err = resolveBound("until", w.until, now, true); err != nil {
			return api.Window{}, err
		}
	}
	if !out.Since.IsZero() && !out.Until.IsZero() && !out.Since.Before(out.Until) {
		return api.Window{}, goclikit.UsageError(fmt.Errorf(
			"--since %s is not before --until %s, so the window holds nothing", since, w.until))
	}
	return out, nil
}

// resolveBound reads one bound. A day on --until is the next day's first instant, so
// the whole day is inside the window.
func resolveBound(flag, raw string, now time.Time, isUntil bool) (time.Time, error) {
	if m := spanPattern.FindStringSubmatch(raw); m != nil {
		unit, ok := spanUnits[m[2]]
		if !ok {
			return time.Time{}, goclikit.UsageError(fmt.Errorf(
				"--%s %s: %q is not a unit; a span takes s, m (minutes), h, d or w. "+
					"There is no month or year, so write 26w, or a day", flag, raw, m[2]))
		}
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil || n > math.MaxInt64/int64(unit) {
			return time.Time{}, goclikit.UsageError(fmt.Errorf("--%s %s reaches back further than a time can hold", flag, raw))
		}
		return now.Add(-time.Duration(n) * unit), nil
	}
	if day, err := time.Parse(time.DateOnly, raw); err == nil {
		y, mo, d := day.Date()
		if isUntil {
			d++
		}
		return time.Date(y, mo, d, 0, 0, 0, 0, now.Location()), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	return time.Time{}, goclikit.UsageError(fmt.Errorf("--%s %q: want %s", flag, raw, windowForms))
}
