package tracing_test

import (
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"meso/api/tracing"
)

// An install whose environment names no OTLP endpoint gets no provider, so it
// exports nothing and serves as it would with no tracing built in.
func TestStartInstallsNothingWithoutAnEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")

	provider, err := tracing.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if provider != nil {
		t.Fatal("Start built a provider with no endpoint set")
	}
	if _, installed := otel.GetTracerProvider().(*sdktrace.TracerProvider); installed {
		t.Fatal("Start installed an SDK provider globally with no endpoint set")
	}
}
