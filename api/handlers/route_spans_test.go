package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"meso/api/tracing"
)

// A caller's traceparent and baggage reach the API's server span: the span is
// the caller's child, is named for the route, and carries the caller's baggage
// and the record the path names.
func TestServerSpanContinuesTheCallersTrace(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := tracing.NewProvider(resource.Empty(), sdktrace.WithSyncer(exporter))
	handler := tracing.Handler(buildTestMux(setupTestDB(t)), provider)

	const traceID, callerSpanID = "4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7"
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/movements/41", http.NoBody)
	req.Header.Set("traceparent", "00-"+traceID+"-"+callerSpanID+"-01")
	req.Header.Set("baggage", "caller.flow.id=flow-7")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	span := spans[0]
	require.Equal(t, traceID, span.SpanContext.TraceID().String())
	require.Equal(t, callerSpanID, span.Parent.SpanID().String())
	require.Equal(t, "GET /api/v1/movements/{id}", span.Name)

	attributes := map[attribute.Key]string{}
	for _, kv := range span.Attributes {
		attributes[kv.Key] = kv.Value.String()
	}
	require.Equal(t, "flow-7", attributes["caller.flow.id"])
	require.Equal(t, "/api/v1/movements/{id}", attributes["http.route"])
	require.Equal(t, "41", attributes["meso.movement.id"])
}
