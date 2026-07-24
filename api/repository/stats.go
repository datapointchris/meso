package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/models"
)

type StatsRepo struct {
	pool *pgxpool.Pool
}

func NewStatsRepo(pool *pgxpool.Pool) *StatsRepo {
	return &StatsRepo{pool: pool}
}

// sessionWeekWindow bounds the by-week session-frequency series so the chart stays
// a readable recent window rather than the whole history.
const sessionWeekWindow = 12

// Summary assembles the whole stats-page payload: every measured metric's trend,
// the library counts, and the session-frequency summary. Each part is one aggregate
// query — no per-metric N+1.
func (r *StatsRepo) Summary(ctx context.Context) (models.StatsSummary, error) {
	metrics, err := r.metricTrends(ctx)
	if err != nil {
		return models.StatsSummary{}, err
	}
	library, err := r.libraryStats(ctx)
	if err != nil {
		return models.StatsSummary{}, err
	}
	sessions, err := r.sessionStats(ctx)
	if err != nil {
		return models.StatsSummary{}, err
	}
	return models.StatsSummary{Metrics: metrics, Library: library, Sessions: sessions}, nil
}

// metricTrends returns a trend per metric that has at least one reading, points
// oldest-first. One query joins every measurement to its definition, ordered so the
// grouping below is a single linear pass; a defined-but-unmeasured metric is omitted
// (an empty chart is noise — the full vocabulary is at /metrics).
func (r *StatsRepo) metricTrends(ctx context.Context) ([]models.MetricTrend, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.name, d.unit, d.direction, d.category, m.measured_on, m.value
		FROM measurements m
		JOIN metric_definitions d ON d.name = m.metric
		ORDER BY d.category, d.name, m.measured_on, m.created_at`)
	if err != nil {
		return nil, fmt.Errorf("loading metric trends: %w", err)
	}
	defer rows.Close()

	trends := []models.MetricTrend{}
	byName := map[string]int{}
	for rows.Next() {
		var name, unit, direction, category string
		var measuredOn time.Time
		var value float64
		if err := rows.Scan(&name, &unit, &direction, &category, &measuredOn, &value); err != nil {
			return nil, fmt.Errorf("scanning metric trend: %w", err)
		}
		idx, ok := byName[name]
		if !ok {
			idx = len(trends)
			byName[name] = idx
			trends = append(trends, trendFromDefinition(models.MetricDefinition{
				Name: name, Unit: unit, Direction: direction, Category: category,
			}))
		}
		trends[idx].Points = append(trends[idx].Points, models.TrendPoint{
			MeasuredOn: measuredOn.Format(dateLayout), Value: value,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range trends {
		summarizeTrend(&trends[i])
	}
	return trends, nil
}

// libraryStats counts the movement library: total, favorites, and the by-kind mix.
func (r *StatsRepo) libraryStats(ctx context.Context) (models.LibraryStats, error) {
	stats := models.LibraryStats{ByKind: []models.KindCount{}}
	err := r.pool.QueryRow(ctx,
		`SELECT count(*), count(*) FILTER (WHERE favorite) FROM movements`).
		Scan(&stats.TotalMovements, &stats.Favorites)
	if err != nil {
		return models.LibraryStats{}, fmt.Errorf("counting movements: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT movement_kind, count(*) FROM movements GROUP BY movement_kind ORDER BY movement_kind`)
	if err != nil {
		return models.LibraryStats{}, fmt.Errorf("counting by kind: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kc models.KindCount
		if err := rows.Scan(&kc.Kind, &kc.Count); err != nil {
			return models.LibraryStats{}, fmt.Errorf("scanning kind count: %w", err)
		}
		stats.ByKind = append(stats.ByKind, kc)
	}
	return stats, rows.Err()
}

// sessionStats counts logged sessions overall, in the trailing 30 days, and by week
// over the recent window (Monday-anchored, oldest-first).
func (r *StatsRepo) sessionStats(ctx context.Context) (models.SessionStats, error) {
	stats := models.SessionStats{ByWeek: []models.WeekCount{}}
	err := r.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE performed_on >= CURRENT_DATE - INTERVAL '30 days')
		FROM workout_sessions`).
		Scan(&stats.Total, &stats.Last30Days)
	if err != nil {
		return models.SessionStats{}, fmt.Errorf("counting sessions: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT date_trunc('week', performed_on)::date AS week_start, count(*)
		FROM workout_sessions
		WHERE performed_on >= date_trunc('week', CURRENT_DATE) - make_interval(weeks => $1)
		GROUP BY week_start
		ORDER BY week_start`, sessionWeekWindow)
	if err != nil {
		return models.SessionStats{}, fmt.Errorf("counting by week: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var weekStart time.Time
		var count int
		if err := rows.Scan(&weekStart, &count); err != nil {
			return models.SessionStats{}, fmt.Errorf("scanning week count: %w", err)
		}
		stats.ByWeek = append(stats.ByWeek, models.WeekCount{WeekStart: weekStart.Format(dateLayout), Count: count})
	}
	return stats, rows.Err()
}
