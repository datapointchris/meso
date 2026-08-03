package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"meso/api/models"
)

// ReviewRepo assembles the capstone read by composing the existing resource repos,
// so `meso review` reuses their SQL filter definitions rather than re-implementing
// "recent sessions / measurements / log". It owns no queries of its own beyond
// resolving the window — the reasoning about the next cycle happens in-conversation,
// not here.
type ReviewRepo struct {
	sessions     *SessionRepo
	measurements *MeasurementRepo
	log          *LogRepo
	cycles       *CycleRepo
}

func NewReviewRepo(sessions *SessionRepo, measurements *MeasurementRepo, log *LogRepo, cycles *CycleRepo) *ReviewRepo {
	return &ReviewRepo{sessions: sessions, measurements: measurements, log: log, cycles: cycles}
}

// defaultReviewWindow is the look-back used when the caller passes no `since`.
const defaultReviewWindow = "30d"

// Review pulls the active cycles plus the sessions, measurements, and log entries
// within the window into one payload. since is a relative duration ("30d", "12w",
// "6m"); empty means the default window. An unparsable since is ErrInvalid (400).
func (r *ReviewRepo) Review(ctx context.Context, since string) (models.Review, error) {
	if since == "" {
		since = defaultReviewWindow
	}
	from, err := windowStart(since)
	if err != nil {
		return models.Review{}, err
	}
	fromStr := from.Format(dateLayout)

	activeCycles, err := r.cycles.List(ctx, models.CycleFilter{Status: "active"})
	if err != nil {
		return models.Review{}, err
	}
	sessions, err := r.sessions.List(ctx, models.WorkoutSessionFilter{From: fromStr})
	if err != nil {
		return models.Review{}, err
	}
	measurements, err := r.measurements.ListMeasurements(ctx, models.MeasurementFilter{From: fromStr})
	if err != nil {
		return models.Review{}, err
	}
	logEntries, err := r.log.List(ctx, models.FitnessLogEntryFilter{From: fromStr})
	if err != nil {
		return models.Review{}, err
	}

	return models.Review{
		Since:        fromStr,
		ActiveCycles: activeCycles,
		Sessions:     sessions,
		Measurements: measurements,
		LogEntries:   logEntries,
	}, nil
}

// windowStart resolves a relative-duration string ("30d", "12w", "6m") to the start
// date of the review window, counting back from today. Only whole days/weeks/months
// are accepted — a review window is coarse by nature. A malformed value is ErrInvalid.
func windowStart(since string) (time.Time, error) {
	since = strings.TrimSpace(since)
	if len(since) < 2 {
		return time.Time{}, fmt.Errorf("invalid since %q: want a count and a unit, e.g. 30d: %w", since, ErrInvalid)
	}
	unit := since[len(since)-1]
	n, err := strconv.Atoi(since[:len(since)-1])
	if err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("invalid since %q: want a count and a unit, e.g. 30d: %w", since, ErrInvalid)
	}

	now := time.Now()
	switch unit {
	case 'd':
		return now.AddDate(0, 0, -n), nil
	case 'w':
		return now.AddDate(0, 0, -7*n), nil
	case 'm':
		return now.AddDate(0, -n, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid since unit in %q: want d, w, or m: %w", since, ErrInvalid)
	}
}
