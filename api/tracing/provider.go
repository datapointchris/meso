// Package tracing gives each API request a server span that continues the
// caller's trace, and exports the spans over OTLP where the environment names an
// endpoint. Everything it reads is the OpenTelemetry SDK's own OTEL_* variables.
package tracing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/processors/baggagecopy"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// ServiceName is service.name on every span, unless OTEL_SERVICE_NAME names another.
const ServiceName = "meso"

// Propagator reads W3C trace context and baggage, the default of OTEL_PROPAGATORS.
var Propagator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

// Start installs a tracer provider exporting over OTLP/HTTP, configured by the
// OTEL_* variables, and returns it so the caller can flush it at shutdown. It
// returns nil and installs nothing when neither OTEL_EXPORTER_OTLP_ENDPOINT nor
// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT is set, so an install that names no
// collector serves exactly as it would with no tracing built in.
func Start(ctx context.Context) (*sdktrace.TracerProvider, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		return nil, nil
	}
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("OTLP trace exporter: %w", err)
	}
	// Detection that finds only part of the host, such as no container id
	// outside a container, still returns what it found.
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(ServiceName)),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithContainer(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
	)
	if err != nil && !errors.Is(err, resource.ErrPartialResource) {
		return nil, fmt.Errorf("trace resource: %w", err)
	}
	provider := NewProvider(res, sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(Propagator)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Warn("trace export", "err", err)
	}))
	return provider, nil
}

// NewProvider builds a tracer provider that hands its spans to export and copies
// every baggage member the request carries onto each span it starts, so a
// caller's flow id reaches the API's spans without the API naming it.
func NewProvider(res *resource.Resource, export sdktrace.TracerProviderOption) *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(baggagecopy.NewSpanProcessor(baggagecopy.AllowAllMembers)),
		export,
	)
}

// Handler wraps next so each request gets a server span from provider, the child
// of the span its traceparent header names. Docker's health probe gets none,
// because it arrives every thirty seconds and carries no caller.
func Handler(next http.Handler, provider trace.TracerProvider) http.Handler {
	return otelhttp.NewHandler(next, ServiceName,
		otelhttp.WithTracerProvider(provider),
		otelhttp.WithPropagators(Propagator),
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/health" }),
	)
}
