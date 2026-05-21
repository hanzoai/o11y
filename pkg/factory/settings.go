package factory

import (
	"log/slog"

	luxmetric "github.com/luxfi/metric"
	sdkmetric "go.opentelemetry.io/otel/metric"
	sdktrace "go.opentelemetry.io/otel/trace"
)

// ProviderSettings is the dependency bundle every o11y provider receives.
//
// MetricsRegisterer replaces the previous PrometheusRegisterer field —
// luxfi/metric is the canonical Lux metrics library, scrape-compatible,
// no google.golang.org/protobuf in the dep graph.
type ProviderSettings struct {
	// Logger is the slog logger.
	Logger *slog.Logger
	// MeterProvider is the OpenTelemetry meter provider (kept for OTel-instrumented libraries).
	MeterProvider sdkmetric.MeterProvider
	// TracerProvider is the tracer provider.
	TracerProvider sdktrace.TracerProvider
	// MetricsRegisterer is the luxfi/metric registerer for application metrics.
	MetricsRegisterer luxmetric.Registerer
}

type ScopedProviderSettings interface {
	Logger() *slog.Logger
	Meter() sdkmetric.Meter
	Tracer() sdktrace.Tracer
	MetricsRegisterer() luxmetric.Registerer
}

type scoped struct {
	logger            *slog.Logger
	meter             sdkmetric.Meter
	tracer            sdktrace.Tracer
	metricsRegisterer luxmetric.Registerer
}

func NewScopedProviderSettings(settings ProviderSettings, pkgName string) *scoped {
	return &scoped{
		logger:            settings.Logger.With("logger", pkgName),
		meter:             settings.MeterProvider.Meter(pkgName),
		tracer:            settings.TracerProvider.Tracer(pkgName),
		metricsRegisterer: settings.MetricsRegisterer,
	}
}

func (s *scoped) Logger() *slog.Logger {
	return s.logger
}

func (s *scoped) Meter() sdkmetric.Meter {
	return s.meter
}

func (s *scoped) Tracer() sdktrace.Tracer {
	return s.tracer
}

func (s *scoped) MetricsRegisterer() luxmetric.Registerer {
	return s.metricsRegisterer
}
