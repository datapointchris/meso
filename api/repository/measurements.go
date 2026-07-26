package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/models"
)

type MeasurementRepo struct {
	pool *pgxpool.Pool
}

func NewMeasurementRepo(pool *pgxpool.Pool) *MeasurementRepo {
	return &MeasurementRepo{pool: pool}
}

// metricSelect is the shared read query; scanMetric stays in lockstep.
const metricSelect = `SELECT name, label, unit, direction, category, how_to_measure, created_at
FROM metric_definitions`

func scanMetric(row pgx.Row) (models.MetricDefinition, error) {
	var m models.MetricDefinition
	if err := row.Scan(&m.Name, &m.Label, &m.Unit, &m.Direction, &m.Category,
		&m.HowToMeasure, &m.CreatedAt); err != nil {
		return models.MetricDefinition{}, err
	}
	return m, nil
}

// ListMetrics returns the metric-definition vocabulary, grouped by category then
// name — the metrics a measurement can reference and the stats page groups by.
func (r *MeasurementRepo) ListMetrics(ctx context.Context) ([]models.MetricDefinition, error) {
	rows, err := r.pool.Query(ctx, metricSelect+" ORDER BY category, label")
	if err != nil {
		return nil, fmt.Errorf("listing metrics: %w", err)
	}
	defer rows.Close()

	metrics := []models.MetricDefinition{}
	for rows.Next() {
		m, err := scanMetric(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning metric: %w", err)
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// GetMetric fetches one metric definition by name, ErrNotFound when absent.
func (r *MeasurementRepo) GetMetric(ctx context.Context, name string) (models.MetricDefinition, error) {
	m, err := scanMetric(r.pool.QueryRow(ctx, metricSelect+" WHERE name = $1", name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.MetricDefinition{}, fmt.Errorf("metric %q: %w", name, ErrNotFound)
		}
		return models.MetricDefinition{}, fmt.Errorf("fetching metric: %w", err)
	}
	return m, nil
}

// DefineMetric inserts a metric definition. A duplicate name is ErrConflict (409); a
// direction/category outside its CHECK set is ErrInvalid (400, via mapWriteError's
// 23514 mapping). Metrics are the tracking vocabulary — POST creates, it does not
// silently overwrite an existing definition.
func (r *MeasurementRepo) DefineMetric(ctx context.Context, in models.MetricDefinitionCreate) (models.MetricDefinition, error) {
	label := in.Label
	if label == "" {
		label = models.DeriveMetricLabel(in.Name)
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO metric_definitions (name, label, unit, direction, category, how_to_measure)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		in.Name, label, in.Unit, in.Direction, in.Category, in.HowToMeasure)
	if err != nil {
		return models.MetricDefinition{}, mapWriteError("defining metric", err)
	}
	return r.GetMetric(ctx, in.Name)
}

// UpdateMetric applies a partial update to a definition, leaving nil fields alone. An
// unknown name is ErrNotFound; a direction/category outside its CHECK set is
// ErrInvalid. The name itself is immutable — it is the key measurements reference.
func (r *MeasurementRepo) UpdateMetric(ctx context.Context, name string, in models.MetricDefinitionUpdate) (models.MetricDefinition, error) {
	sets := []string{}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf(clause, len(args)))
	}
	if in.Label != nil {
		add("label = $%d", *in.Label)
	}
	if in.Unit != nil {
		add("unit = $%d", *in.Unit)
	}
	if in.Direction != nil {
		add("direction = $%d", *in.Direction)
	}
	if in.Category != nil {
		add("category = $%d", *in.Category)
	}
	if in.HowToMeasure != nil {
		add("how_to_measure = $%d", *in.HowToMeasure)
	}
	if len(sets) == 0 {
		return r.GetMetric(ctx, name)
	}

	args = append(args, name)
	query := fmt.Sprintf("UPDATE metric_definitions SET %s WHERE name = $%d",
		strings.Join(sets, ", "), len(args))
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return models.MetricDefinition{}, mapWriteError("updating metric", err)
	}
	if tag.RowsAffected() == 0 {
		return models.MetricDefinition{}, fmt.Errorf("metric %q: %w", name, ErrNotFound)
	}
	return r.GetMetric(ctx, name)
}

// DeleteMetric removes a metric definition by name. A metric still referenced by a
// recorded measurement (FK RESTRICT) is ErrReferenced (409); an unknown name is
// ErrNotFound (404). A cycle that targets it is SET NULL by the FK, so deleting is
// not blocked by cycle references — the cycle simply loses its target.
func (r *MeasurementRepo) DeleteMetric(ctx context.Context, name string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM metric_definitions WHERE name = $1`, name)
	if err != nil {
		return mapWriteError("deleting metric", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("metric %q: %w", name, ErrNotFound)
	}
	return nil
}

// measurementSelect is the shared read query; scanMeasurement stays in lockstep.
const measurementSelect = `SELECT id, metric, value, measured_on, source, notes, created_at FROM measurements`

func scanMeasurement(row pgx.Row) (models.Measurement, error) {
	var m models.Measurement
	var measuredOn time.Time
	if err := row.Scan(&m.ID, &m.Metric, &m.Value, &measuredOn, &m.Source, &m.Notes, &m.CreatedAt); err != nil {
		return models.Measurement{}, err
	}
	m.MeasuredOn = measuredOn.Format(dateLayout)
	return m, nil
}

// ListMeasurements returns readings matching the filter, newest first. Filtering is
// in SQL so the CLI and web share one definition.
func (r *MeasurementRepo) ListMeasurements(ctx context.Context, f models.MeasurementFilter) ([]models.Measurement, error) {
	var (
		where []string
		args  []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.Metric != "" {
		add("metric = $%d", f.Metric)
	}
	if f.From != "" {
		from, err := parseDate(f.From)
		if err != nil {
			return nil, err
		}
		add("measured_on >= $%d", from)
	}
	if f.To != "" {
		to, err := parseDate(f.To)
		if err != nil {
			return nil, err
		}
		add("measured_on <= $%d", to)
	}

	query := measurementSelect
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY measured_on DESC, created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing measurements: %w", err)
	}
	defer rows.Close()

	measurements := []models.Measurement{}
	for rows.Next() {
		m, err := scanMeasurement(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning measurement: %w", err)
		}
		measurements = append(measurements, m)
	}
	return measurements, rows.Err()
}

func (r *MeasurementRepo) GetMeasurement(ctx context.Context, id int64) (models.Measurement, error) {
	m, err := scanMeasurement(r.pool.QueryRow(ctx, measurementSelect+" WHERE id = $1", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Measurement{}, fmt.Errorf("measurement %d: %w", id, ErrNotFound)
		}
		return models.Measurement{}, fmt.Errorf("fetching measurement: %w", err)
	}
	return m, nil
}

// RecordMeasurement appends a reading. measured_on defaults to today when omitted;
// source defaults to "manual". An unknown metric fails the FK -> ErrReferenced (409).
func (r *MeasurementRepo) RecordMeasurement(ctx context.Context, in models.MeasurementCreate) (models.Measurement, error) {
	measuredOn := time.Now()
	if in.MeasuredOn != "" {
		parsed, err := parseDate(in.MeasuredOn)
		if err != nil {
			return models.Measurement{}, err
		}
		measuredOn = parsed
	}
	source := in.Source
	if source == "" {
		source = "manual"
	}

	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO measurements (metric, value, measured_on, source, notes)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		in.Metric, in.Value, measuredOn, source, in.Notes).Scan(&id)
	if err != nil {
		return models.Measurement{}, mapWriteError("recording measurement", err)
	}
	return r.GetMeasurement(ctx, id)
}

func (r *MeasurementRepo) UpdateMeasurement(ctx context.Context, id int64, in models.MeasurementUpdate) (models.Measurement, error) {
	current, err := r.GetMeasurement(ctx, id)
	if err != nil {
		return models.Measurement{}, err
	}

	value := current.Value
	if in.Value != nil {
		value = *in.Value
	}
	measuredOn, err := parseDate(current.MeasuredOn)
	if err != nil {
		return models.Measurement{}, err
	}
	if in.MeasuredOn != nil {
		measuredOn, err = parseDate(*in.MeasuredOn)
		if err != nil {
			return models.Measurement{}, err
		}
	}
	notes := valueOr(in.Notes, current.Notes)

	_, err = r.pool.Exec(ctx,
		`UPDATE measurements SET value = $1, measured_on = $2, notes = $3 WHERE id = $4`,
		value, measuredOn, notes, id)
	if err != nil {
		return models.Measurement{}, mapWriteError("updating measurement", err)
	}
	return r.GetMeasurement(ctx, id)
}

func (r *MeasurementRepo) DeleteMeasurement(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM measurements WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting measurement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("measurement %d: %w", id, ErrNotFound)
	}
	return nil
}

// Trend returns one metric's full series (oldest-first, for a left-to-right chart)
// plus the summary numbers: first/latest readings and their delta. ErrNotFound when
// the metric is undefined. An optional date window bounds the series (From/To).
func (r *MeasurementRepo) Trend(ctx context.Context, metric string, f models.MeasurementFilter) (models.MetricTrend, error) {
	def, err := r.GetMetric(ctx, metric)
	if err != nil {
		return models.MetricTrend{}, err
	}

	where := []string{"metric = $1"}
	args := []any{metric}
	if f.From != "" {
		from, err := parseDate(f.From)
		if err != nil {
			return models.MetricTrend{}, err
		}
		args = append(args, from)
		where = append(where, fmt.Sprintf("measured_on >= $%d", len(args)))
	}
	if f.To != "" {
		to, err := parseDate(f.To)
		if err != nil {
			return models.MetricTrend{}, err
		}
		args = append(args, to)
		where = append(where, fmt.Sprintf("measured_on <= $%d", len(args)))
	}

	rows, err := r.pool.Query(ctx,
		`SELECT measured_on, value FROM measurements WHERE `+strings.Join(where, " AND ")+
			` ORDER BY measured_on, created_at`, args...)
	if err != nil {
		return models.MetricTrend{}, fmt.Errorf("loading trend: %w", err)
	}
	defer rows.Close()

	trend := trendFromDefinition(def)
	for rows.Next() {
		var measuredOn time.Time
		var value float64
		if err := rows.Scan(&measuredOn, &value); err != nil {
			return models.MetricTrend{}, fmt.Errorf("scanning trend point: %w", err)
		}
		trend.Points = append(trend.Points, models.TrendPoint{MeasuredOn: measuredOn.Format(dateLayout), Value: value})
	}
	if err := rows.Err(); err != nil {
		return models.MetricTrend{}, err
	}
	summarizeTrend(&trend)
	return trend, nil
}

// trendFromDefinition seeds a MetricTrend with its definition metadata and an empty,
// non-nil point slice (so JSON emits [] not null for an unmeasured metric).
func trendFromDefinition(def models.MetricDefinition) models.MetricTrend {
	return models.MetricTrend{
		Metric:       def.Name,
		Label:        def.Label,
		Unit:         def.Unit,
		Direction:    def.Direction,
		Category:     def.Category,
		HowToMeasure: def.HowToMeasure,
		Points:       []models.TrendPoint{},
	}
}

// summarizeTrend fills first/latest/change/count from the (oldest-first) points.
// Change is latest − first regardless of direction; the client colors it using
// Direction, so a smaller 5k time reads as improvement.
func summarizeTrend(t *models.MetricTrend) {
	t.Count = len(t.Points)
	if t.Count == 0 {
		return
	}
	first := t.Points[0].Value
	latest := t.Points[t.Count-1].Value
	change := latest - first
	t.First = &first
	t.Latest = &latest
	t.Change = &change
}
