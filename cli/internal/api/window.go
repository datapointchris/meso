package api

import (
	"net/url"
	"time"
)

// Window bounds a read in time. Since is inclusive, Until exclusive, and a zero side
// is open. Both go on the wire as RFC 3339 in the machine's offset, because the API
// maps each instant to a day in the offset it carries.
type Window struct {
	Since time.Time
	Until time.Time
}

func (w Window) set(q url.Values) {
	if !w.Since.IsZero() {
		q.Set("since", w.Since.Local().Format(time.RFC3339))
	}
	if !w.Until.IsZero() {
		q.Set("until", w.Until.Local().Format(time.RFC3339))
	}
}
