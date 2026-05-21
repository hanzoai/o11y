package instrumentationtest

import (
	"log/slog"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/instrumentation"
	luxmetric "github.com/luxfi/metric"
	sdkmetric "go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

type noopInstrumentation struct {
	logger         *slog.Logger
	meterProvider  sdkmetric.MeterProvider
	tracerProvider sdktrace.TracerProvider
	metricsReg     luxmetric.Registry
}

func New() instrumentation.Instrumentation {
	return &noopInstrumentation{
		logger:         slog.New(slog.DiscardHandler),
		meterProvider:  noopmetric.NewMeterProvider(),
		tracerProvider: nooptrace.NewTracerProvider(),
		metricsReg:     luxmetric.NewRegistry(),
	}
}

func (i *noopInstrumentation) Logger() *slog.Logger {
	return i.logger
}

func (i *noopInstrumentation) MeterProvider() sdkmetric.MeterProvider {
	return i.meterProvider
}

func (i *noopInstrumentation) TracerProvider() sdktrace.TracerProvider {
	return i.tracerProvider
}

func (i *noopInstrumentation) MetricsRegisterer() luxmetric.Registerer {
	return i.metricsReg
}

func (i *noopInstrumentation) ToProviderSettings() factory.ProviderSettings {
	return factory.ProviderSettings{
		Logger:            i.Logger(),
		MeterProvider:     i.MeterProvider(),
		TracerProvider:    i.TracerProvider(),
		MetricsRegisterer: i.MetricsRegisterer(),
	}
}
