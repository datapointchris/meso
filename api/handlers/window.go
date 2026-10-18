package handlers

import (
	"fmt"
	"net/url"
	"time"
)

// dayLayout is the calendar-date form the repositories bound performed_on,
// measured_on and entry_date by.
const dayLayout = "2006-01-02"

// readWindow reads ?since= and ?until=, each an RFC 3339 instant, into the first and
// last days a dated row may fall on, both inclusive. A row carries a day and no zone,
// so each instant is read as a day in its own offset, which is the caller's. until is
// exclusive, so the last day is the one holding the instant just before it. A bound
// the caller left out stays empty, which the repositories read as open.
func readWindow(q url.Values) (first, last string, err error) {
	since, hasSince, err := readInstant(q, "since")
	if err != nil {
		return "", "", err
	}
	until, hasUntil, err := readInstant(q, "until")
	if err != nil {
		return "", "", err
	}
	if hasSince && hasUntil && !since.Before(until) {
		return "", "", fmt.Errorf("since %s is not before until %s, so the window holds nothing", q.Get("since"), q.Get("until"))
	}
	if hasSince {
		first = since.Format(dayLayout)
	}
	if hasUntil {
		last = until.Add(-time.Nanosecond).Format(dayLayout)
	}
	return first, last, nil
}

func readInstant(q url.Values, key string) (time.Time, bool, error) {
	raw := q.Get(key)
	if raw == "" {
		return time.Time{}, false, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("invalid %s %q: want an RFC 3339 instant, such as 2026-10-01T00:00:00-04:00", key, raw)
	}
	return t, true, nil
}
