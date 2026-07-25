package api

import (
	"context"
	"net/http"
	"testing"
)

func TestListMeasurements_QueryAndDecode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `[
		{"id":2,"metric":"deadlift-working-weight","value":225,"measured_on":"2026-07-22","source":"manual","notes":""},
		{"id":1,"metric":"deadlift-working-weight","value":202.5,"measured_on":"2026-07-15","source":"manual","notes":"RPE 8"}
	]`)
	client := New(srv.URL, staticTokenClient("t"))

	measurements, err := client.ListMeasurements(context.Background(),
		MeasurementFilter{Metric: "deadlift-working-weight", From: "2026-07-01"})
	if err != nil {
		t.Fatalf("ListMeasurements: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/api/v1/measurements" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	for _, want := range []string{"metric=deadlift-working-weight", "from=2026-07-01"} {
		if !containsParam(got.query, want) {
			t.Errorf("query %q missing %q", got.query, want)
		}
	}
	if len(measurements) != 2 || measurements[1].Value != 202.5 {
		t.Fatalf("decoded = %+v", measurements)
	}
}

func TestRecordMeasurement_SendsBody(t *testing.T) {
	srv, got := recordingServer(t, http.StatusCreated,
		`{"id":3,"metric":"5k-time","value":1650,"measured_on":"2026-07-27","source":"manual","notes":""}`)
	client := New(srv.URL, staticTokenClient("t"))

	if _, err := client.RecordMeasurement(context.Background(),
		MeasurementCreate{Metric: "5k-time", Value: 1650, MeasuredOn: "2026-07-27"}); err != nil {
		t.Fatalf("RecordMeasurement: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/measurements" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["metric"] != "5k-time" || got.body["value"] != float64(1650) {
		t.Errorf("body = %v", got.body)
	}
}

func TestDefineMetric_SendsBody(t *testing.T) {
	srv, got := recordingServer(t, http.StatusCreated,
		`{"name":"toe-reach","unit":"cm","direction":"higher_better","category":"mobility"}`)
	client := New(srv.URL, staticTokenClient("t"))

	if _, err := client.DefineMetric(context.Background(), MetricDefinitionCreate{
		Name: "toe-reach", Unit: "cm", Direction: "higher_better", Category: "mobility",
	}); err != nil {
		t.Fatalf("DefineMetric: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/metrics" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body["name"] != "toe-reach" || got.body["direction"] != "higher_better" {
		t.Errorf("body = %v", got.body)
	}
}

func TestDeleteMetric_SendsDelete(t *testing.T) {
	srv, got := recordingServer(t, http.StatusNoContent, "")
	client := New(srv.URL, staticTokenClient("t"))

	if err := client.DeleteMetric(context.Background(), "continuous-easy-run"); err != nil {
		t.Fatalf("DeleteMetric: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/api/v1/metrics/continuous-easy-run" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
}

func TestTrend_QueryPathAndDecode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{
		"metric":"5k-time","unit":"seconds","direction":"lower_better","category":"cardio",
		"points":[{"measured_on":"2026-07-01","value":1800},{"measured_on":"2026-07-27","value":1650}],
		"first":1800,"latest":1650,"change":-150,"count":2
	}`)
	client := New(srv.URL, staticTokenClient("t"))

	trend, err := client.Trend(context.Background(), "5k-time", MeasurementFilter{From: "2026-07-01"})
	if err != nil {
		t.Fatalf("Trend: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/api/v1/metrics/5k-time/trend" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if !containsParam(got.query, "from=2026-07-01") {
		t.Errorf("query %q missing from", got.query)
	}
	if trend.Count != 2 || trend.Change == nil || *trend.Change != -150 {
		t.Fatalf("decoded = %+v", trend)
	}
}

func TestStats_Decode(t *testing.T) {
	srv, got := recordingServer(t, http.StatusOK, `{
		"metrics":[{"metric":"deadlift-working-weight","unit":"lb","direction":"higher_better","category":"strength",
			"points":[{"measured_on":"2026-07-01","value":185}],"first":185,"latest":225,"change":40,"count":2}],
		"library":{"by_kind":[{"kind":"exercise","count":3}],"total_movements":3,"favorites":1},
		"sessions":{"by_week":[{"week_start":"2026-07-20","count":2}],"total":2,"last_30_days":2}
	}`)
	client := New(srv.URL, staticTokenClient("t"))

	stats, err := client.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got.path != "/api/v1/stats" {
		t.Errorf("path = %s", got.path)
	}
	if stats.Library.TotalMovements != 3 || stats.Sessions.Total != 2 || len(stats.Metrics) != 1 {
		t.Fatalf("decoded = %+v", stats)
	}
}
