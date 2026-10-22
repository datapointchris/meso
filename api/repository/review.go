package repository

import (
	"context"

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

// Review pulls the active cycles plus the sessions, measurements, and log entries
// dated from first through last, both inclusive "2006-01-02" days. An empty bound
// leaves that side open.
func (r *ReviewRepo) Review(ctx context.Context, first, last string) (models.Review, error) {
	activeCycles, err := r.cycles.List(ctx, models.CycleFilter{Status: "active"})
	if err != nil {
		return models.Review{}, err
	}
	sessions, err := r.sessions.List(ctx, models.WorkoutSessionFilter{From: first, To: last})
	if err != nil {
		return models.Review{}, err
	}
	measurements, err := r.measurements.ListMeasurements(ctx, models.MeasurementFilter{From: first, To: last})
	if err != nil {
		return models.Review{}, err
	}
	logEntries, err := r.log.List(ctx, models.FitnessLogEntryFilter{From: first, To: last})
	if err != nil {
		return models.Review{}, err
	}

	return models.Review{
		SchemaVersion: models.ReviewSchemaVersion,
		Since:         first,
		Until:         last,
		ActiveCycles:  activeCycles,
		Sessions:      sessions,
		Measurements:  measurements,
		LogEntries:    logEntries,
	}, nil
}
