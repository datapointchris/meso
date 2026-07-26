package models

import (
	"strings"
	"time"
	"unicode"
)

// MetricDefinition names a tracked metric and carries the metadata a chart needs:
// its unit, which direction counts as improvement (a heavier deadlift and a faster
// 5k both improve, with opposite signs), and a coarse category for grouping. It is
// the lookup a Measurement references. Name is the slug-shaped natural key every
// other surface addresses it by; Label is what a human reads.
//
// HowToMeasure is the protocol that produces the number, as markdown. Without it a
// metric is a label over a series nobody can reproduce, and a reading taken a
// different way silently ruins the trend.
type MetricDefinition struct {
	CreatedAt    time.Time `json:"created_at"`
	Name         string    `json:"name"`
	Label        string    `json:"label"`
	Unit         string    `json:"unit"`
	Direction    string    `json:"direction"`
	Category     string    `json:"category"`
	HowToMeasure string    `json:"how_to_measure"`
}

// MetricDefinitionCreate defines a metric. Direction ∈ higher_better|lower_better,
// category ∈ strength|cardio|mobility|body — the DB CHECKs enforce both, so an
// unknown value is a 400, not a silent write. An empty Label is derived from Name;
// an empty HowToMeasure stays empty, since there is nothing to derive a protocol from.
type MetricDefinitionCreate struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	Unit         string `json:"unit"`
	Direction    string `json:"direction"`
	Category     string `json:"category"`
	HowToMeasure string `json:"how_to_measure"`
}

// MetricDefinitionUpdate is a partial update of a definition: a nil field is left
// unchanged. Name is the natural key and is fixed — every measurement, cycle target,
// and CLI invocation addresses the metric by it, so renaming is delete + redefine.
type MetricDefinitionUpdate struct {
	Label        *string `json:"label"`
	Unit         *string `json:"unit"`
	Direction    *string `json:"direction"`
	Category     *string `json:"category"`
	HowToMeasure *string `json:"how_to_measure"`
}

// DeriveMetricLabel turns a slug-shaped metric name into a display label:
// "back-squat-working-weight" → "Back Squat Working Weight". It is the default a
// define with no explicit label gets, and it matches the SQL backfill in migration
// 00008 (initcap over the de-hyphenated name), so a metric defined before and after
// that migration reads the same.
func DeriveMetricLabel(name string) string {
	words := strings.Split(name, "-")
	for i, word := range words {
		if word == "" {
			continue
		}
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// Measurement is one dated reading of a metric — a point in the time series. Value
// is a plain number on the wire (JSON has no decimal type); the column is NUMERIC
// for storage. MeasuredOn is a calendar date ("2006-01-02"). Source is manual in
// v1; 'session' is reserved for future auto-derivation from logged sessions.
type Measurement struct {
	CreatedAt  time.Time `json:"created_at"`
	Metric     string    `json:"metric"`
	MeasuredOn string    `json:"measured_on"`
	Source     string    `json:"source"`
	Notes      string    `json:"notes"`
	Value      float64   `json:"value"`
	ID         int64     `json:"id"`
}

// MeasurementCreate records a reading. MeasuredOn defaults to today when empty;
// Source defaults to "manual" server-side.
type MeasurementCreate struct {
	Metric     string  `json:"metric"`
	MeasuredOn string  `json:"measured_on"`
	Source     string  `json:"source"`
	Notes      string  `json:"notes"`
	Value      float64 `json:"value"`
}

// MeasurementUpdate is a partial update of a reading: a nil field is left unchanged.
// The metric a reading belongs to is fixed (re-point by delete + re-record).
type MeasurementUpdate struct {
	Value      *float64 `json:"value"`
	MeasuredOn *string  `json:"measured_on"`
	Notes      *string  `json:"notes"`
}

// MeasurementFilter carries the list-endpoint query params. Metric scopes to one
// series; From/To bound measured_on (inclusive) as "2006-01-02" strings.
type MeasurementFilter struct {
	Metric string
	From   string
	To     string
}

// TrendPoint is one (date, value) in a metric's series, ordered oldest-first so a
// chart plots left-to-right in time.
type TrendPoint struct {
	MeasuredOn string  `json:"measured_on"`
	Value      float64 `json:"value"`
}

// MetricTrend is a metric's full series plus the summary numbers a card shows: the
// first and latest readings and the change between them. Direction lets the client
// color the change as good or bad without re-deriving the sign. Points is empty for
// a defined-but-unmeasured metric.
type MetricTrend struct {
	Metric       string       `json:"metric"`
	Label        string       `json:"label"`
	Unit         string       `json:"unit"`
	Direction    string       `json:"direction"`
	Category     string       `json:"category"`
	HowToMeasure string       `json:"how_to_measure"`
	Points       []TrendPoint `json:"points"`
	First        *float64     `json:"first"`
	Latest       *float64     `json:"latest"`
	Change       *float64     `json:"change"`
	Count        int          `json:"count"`
}

// KindCount is one bar of the movements-by-kind mix (exercise|stretch|yoga_pose).
type KindCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// WeekCount is sessions logged in the week beginning WeekStart (a Monday), the
// session-frequency series.
type WeekCount struct {
	WeekStart string `json:"week_start"`
	Count     int    `json:"count"`
}

// LibraryStats summarizes the movement library for the stats page's cards.
type LibraryStats struct {
	ByKind         []KindCount `json:"by_kind"`
	TotalMovements int         `json:"total_movements"`
	Favorites      int         `json:"favorites"`
}

// SessionStats summarizes logged sessions: the total, the trailing-30-day count,
// and a by-week frequency series for the recent window.
type SessionStats struct {
	ByWeek     []WeekCount `json:"by_week"`
	Total      int         `json:"total"`
	Last30Days int         `json:"last_30_days"`
}

// StatsSummary is the whole stats-page payload: every metric's trend, plus library
// and session summaries. One GET renders the page.
type StatsSummary struct {
	Metrics  []MetricTrend `json:"metrics"`
	Library  LibraryStats  `json:"library"`
	Sessions SessionStats  `json:"sessions"`
}
