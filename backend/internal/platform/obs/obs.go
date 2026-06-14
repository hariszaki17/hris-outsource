// Package obs wires OpenTelemetry: distributed tracing (OTLP/HTTP exporter) and
// Prometheus metrics. Tracing is a no-op when OTEL_EXPORTER_OTLP_ENDPOINT is
// empty, so it's free in environments that don't collect traces. Metrics are
// always available at /metrics.
//
// HTTP handlers are instrumented with otelhttp at the server edge; the
// MetricsHandler is mounted on the metrics route.
package obs

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	prometheusexp "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Providers holds the shutdown hook for the OTel pipelines.
type Providers struct {
	shutdowns []func(context.Context) error
}

// Setup configures global tracer + meter providers. Call Shutdown on exit.
func Setup(ctx context.Context, serviceName, otlpEndpoint string) (*Providers, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	p := &Providers{}

	// --- Tracing (optional) ---
	if otlpEndpoint != "" {
		exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(otlpEndpoint))
		if err != nil {
			return nil, fmt.Errorf("otlp trace exporter: %w", err)
		}
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tp)
		p.shutdowns = append(p.shutdowns, tp.Shutdown)
	}

	// --- Metrics (always on; scraped at /metrics) ---
	metricExp, err := prometheusexp.New()
	if err != nil {
		return nil, fmt.Errorf("prometheus exporter: %w", err)
	}
	mp := metric.NewMeterProvider(
		metric.WithReader(metricExp),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)
	p.shutdowns = append(p.shutdowns, mp.Shutdown)

	return p, nil
}

// MetricsHandler serves the Prometheus exposition format.
func (p *Providers) MetricsHandler() http.Handler { return promhttp.Handler() }

// Shutdown flushes and stops all pipelines.
func (p *Providers) Shutdown(ctx context.Context) error {
	for _, fn := range p.shutdowns {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}
