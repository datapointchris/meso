package api

import (
	"context"
	"net/http"
	"net/url"
)

// MetricDefinition mirrors the API's metric-definition JSON — the tracked-stat
// vocabulary. Name is the slug-shaped key every command addresses it by; Label is
// the display string. Direction ∈ higher_better|lower_better records which way
// improves.
type MetricDefinition struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Unit      string `json:"unit"`
	Direction string `json:"direction"`
	Category  string `json:"category"`
}

// MetricDefinitionCreate is the body for POST /api/v1/metrics. An omitted Label is
// derived from Name server-side.
type MetricDefinitionCreate struct {
	Name      string `json:"name"`
	Label     string `json:"label,omitempty"`
	Unit      string `json:"unit"`
	Direction string `json:"direction"`
	Category  string `json:"category"`
}

// MetricDefinitionUpdate is the body for PUT /api/v1/metrics/{name} — a partial
// update where an omitted field is left unchanged. Name is immutable.
type MetricDefinitionUpdate struct {
	Label     *string `json:"label,omitempty"`
	Unit      *string `json:"unit,omitempty"`
	Direction *string `json:"direction,omitempty"`
	Category  *string `json:"category,omitempty"`
}

// Measurement mirrors one dated reading in the API's measurement JSON.
type Measurement struct {
	Metric     string  `json:"metric"`
	MeasuredOn string  `json:"measured_on"`
	Source     string  `json:"source"`
	Notes      string  `json:"notes"`
	Value      float64 `json:"value"`
	ID         int64   `json:"id"`
}

// MeasurementCreate is the body for POST /api/v1/measurements. MeasuredOn defaults
// to today server-side when empty; Source defaults to "manual".
type MeasurementCreate struct {
	Metric     string  `json:"metric"`
	MeasuredOn string  `json:"measured_on,omitempty"`
	Notes      string  `json:"notes,omitempty"`
	Value      float64 `json:"value"`
}

// TrendPoint is one (date, value) in a metric's series, oldest-first.
type TrendPoint struct {
	MeasuredOn string  `json:"measured_on"`
	Value      float64 `json:"value"`
}

// MetricTrend mirrors GET /api/v1/metrics/{name}/trend — a metric's series plus its
// summary numbers. First/Latest/Change are nil for an unmeasured metric.
type MetricTrend struct {
	First     *float64     `json:"first"`
	Latest    *float64     `json:"latest"`
	Change    *float64     `json:"change"`
	Metric    string       `json:"metric"`
	Label     string       `json:"label"`
	Unit      string       `json:"unit"`
	Direction string       `json:"direction"`
	Category  string       `json:"category"`
	Points    []TrendPoint `json:"points"`
	Count     int          `json:"count"`
}

// KindCount, WeekCount, LibraryStats, SessionStats, Stats mirror the stats payload.
type KindCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

type WeekCount struct {
	WeekStart string `json:"week_start"`
	Count     int    `json:"count"`
}

type LibraryStats struct {
	ByKind         []KindCount `json:"by_kind"`
	TotalMovements int         `json:"total_movements"`
	Favorites      int         `json:"favorites"`
}

type SessionStats struct {
	ByWeek     []WeekCount `json:"by_week"`
	Total      int         `json:"total"`
	Last30Days int         `json:"last_30_days"`
}

// Stats mirrors GET /api/v1/stats — the whole stats-page payload.
type Stats struct {
	Metrics  []MetricTrend `json:"metrics"`
	Library  LibraryStats  `json:"library"`
	Sessions SessionStats  `json:"sessions"`
}

// MeasurementFilter carries the list-endpoint query params.
type MeasurementFilter struct {
	Metric string
	From   string
	To     string
}

func (f MeasurementFilter) query() string {
	q := url.Values{}
	if f.Metric != "" {
		q.Set("metric", f.Metric)
	}
	if f.From != "" {
		q.Set("from", f.From)
	}
	if f.To != "" {
		q.Set("to", f.To)
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// ListMetrics returns the metric-definition vocabulary (GET /api/v1/metrics).
func (c *Client) ListMetrics(ctx context.Context) ([]MetricDefinition, error) {
	var metrics []MetricDefinition
	if err := c.get(ctx, "/api/v1/metrics", &metrics); err != nil {
		return nil, err
	}
	return metrics, nil
}

// DefineMetric defines a metric (POST /api/v1/metrics).
func (c *Client) DefineMetric(ctx context.Context, in MetricDefinitionCreate) (MetricDefinition, error) {
	var m MetricDefinition
	if err := c.send(ctx, http.MethodPost, "/api/v1/metrics", in, &m); err != nil {
		return MetricDefinition{}, err
	}
	return m, nil
}

// UpdateMetric applies a partial update to a definition (PUT /api/v1/metrics/{name}).
func (c *Client) UpdateMetric(ctx context.Context, name string, in MetricDefinitionUpdate) (MetricDefinition, error) {
	var m MetricDefinition
	if err := c.send(ctx, http.MethodPut, "/api/v1/metrics/"+url.PathEscape(name), in, &m); err != nil {
		return MetricDefinition{}, err
	}
	return m, nil
}

// DeleteMetric removes a metric definition (DELETE /api/v1/metrics/{name}). A metric
// still referenced by a recorded measurement is a 409.
func (c *Client) DeleteMetric(ctx context.Context, name string) error {
	return c.send(ctx, http.MethodDelete, "/api/v1/metrics/"+url.PathEscape(name), nil, nil)
}

// Trend returns one metric's series + summary (GET /api/v1/metrics/{name}/trend),
// optionally windowed by from/to.
func (c *Client) Trend(ctx context.Context, metric string, f MeasurementFilter) (MetricTrend, error) {
	q := url.Values{}
	if f.From != "" {
		q.Set("from", f.From)
	}
	if f.To != "" {
		q.Set("to", f.To)
	}
	path := "/api/v1/metrics/" + url.PathEscape(metric) + "/trend"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var t MetricTrend
	if err := c.get(ctx, path, &t); err != nil {
		return MetricTrend{}, err
	}
	return t, nil
}

// ListMeasurements returns readings matching the filter (GET /api/v1/measurements).
func (c *Client) ListMeasurements(ctx context.Context, f MeasurementFilter) ([]Measurement, error) {
	var measurements []Measurement
	if err := c.get(ctx, "/api/v1/measurements"+f.query(), &measurements); err != nil {
		return nil, err
	}
	return measurements, nil
}

// RecordMeasurement appends a reading (POST /api/v1/measurements).
func (c *Client) RecordMeasurement(ctx context.Context, in MeasurementCreate) (Measurement, error) {
	var m Measurement
	if err := c.send(ctx, http.MethodPost, "/api/v1/measurements", in, &m); err != nil {
		return Measurement{}, err
	}
	return m, nil
}

// Stats returns the aggregated stats-page payload (GET /api/v1/stats).
func (c *Client) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	if err := c.get(ctx, "/api/v1/stats", &s); err != nil {
		return Stats{}, err
	}
	return s, nil
}
