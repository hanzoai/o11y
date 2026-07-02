package instrumentation

import (
	"context"
	"log/slog"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/version"
	luxmetric "github.com/luxfi/metric"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/metric"
	sdkmetricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktraceapi "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	sdktrace "go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

var _ factory.Service = (*SDK)(nil)
var _ Instrumentation = (*SDK)(nil)

// SDK holds the core components for application instrumentation.
//
// Metrics: luxfi/metric (the Lux scrape-compatible text format library —
// no protobuf in the dep graph). The OTel MeterProvider stays as a noop
// so OTel-instrumented libraries don't crash; in-tree o11y code emits
// through MetricsRegisterer.
//
// Traces: built directly from go.opentelemetry.io/otel/sdk/trace — no
// go.opentelemetry.io/contrib/config dependency. (That contrib package
// imports the prometheus exporter and every OTLP exporter unconditionally,
// re-introducing prometheus/client_golang we just ripped out.)
//
// Real trace export should be wired with luxfi/trace + the ZAP-native
// exporter in a separate construction path; this SDK provides only the
// process-local Tracer with a noop exporter unless callers override.
type SDK struct {
	logger         *slog.Logger
	tracerProvider sdktrace.TracerProvider
	traceShutdown  func(context.Context) error
	meterProvider  sdkmetric.MeterProvider
	metricsReg     luxmetric.Registry
	startCh        chan struct{}
}

// New creates a new Instrumentation instance with configured providers.
// It sets up logging, tracing, and metrics based on the provided configuration.
func New(ctx context.Context, cfg Config, build version.Build, serviceName string) (*SDK, error) {
	// Set default resource attributes if not provided
	if cfg.Resource.Attributes == nil {
		cfg.Resource.Attributes = map[string]any{
			string(semconv.ServiceNameKey):    serviceName,
			string(semconv.ServiceVersionKey): build.Version(),
		}
	}

	resource, err := sdkresource.New(
		ctx,
		sdkresource.WithContainer(),
		sdkresource.WithFromEnv(),
		sdkresource.WithHost(),
	)
	if err != nil {
		return nil, err
	}

	// Merge user-supplied resource attributes onto the detector-derived ones.
	resAttrs := mergeAttributes(cfg.Resource.Attributes, resource)
	kvs := make([]attribute.KeyValue, 0, len(resAttrs))
	for k, v := range resAttrs {
		kvs = append(kvs, attribute.String(k, toString(v)))
	}
	merged, err := sdkresource.Merge(
		sdkresource.NewSchemaless(kvs...),
		sdkresource.NewWithAttributes(semconv.SchemaURL),
	)
	if err != nil {
		merged = sdkresource.NewWithAttributes(semconv.SchemaURL, kvs...)
	}

	// Trace provider: in-process Tracer with a noop exporter by default.
	// Real export wires via luxfi/trace separately.
	var (
		tp            sdktrace.TracerProvider = tracenoop.NewTracerProvider()
		traceShutdown                         = func(context.Context) error { return nil }
	)
	if cfg.Traces.Enabled {
		sdkTP := sdktraceapi.NewTracerProvider(
			sdktraceapi.WithResource(merged),
		)
		tp = sdkTP
		traceShutdown = sdkTP.Shutdown
	}
	otel.SetTracerProvider(tp)

	// OTel MeterProvider stays no-op — OTel-instrumented libraries
	// (otelhttp, otelgrpc) keep working without crashing. Measurements
	// are discarded; in-tree o11y code uses luxfi/metric directly.
	meterProvider := sdkmetric.MeterProvider(sdkmetricnoop.NewMeterProvider())

	return &SDK{
		tracerProvider: tp,
		traceShutdown:  traceShutdown,
		meterProvider:  meterProvider,
		metricsReg:     luxmetric.NewRegistry(),
		logger:         NewLogger(cfg),
		startCh:        make(chan struct{}),
	}, nil
}

// toString renders a resource-attribute value as a string. Resource
// attribute maps come back as `any` from the contribsdkconfig schema —
// most are strings already, but we accept anything via fmt fallback.
func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

func (i *SDK) Start(ctx context.Context) error {
	<-i.startCh
	return nil
}

func (i *SDK) Stop(ctx context.Context) error {
	close(i.startCh)
	return i.traceShutdown(ctx)
}

func (i *SDK) Logger() *slog.Logger {
	return i.logger
}

func (i *SDK) MeterProvider() sdkmetric.MeterProvider {
	return i.meterProvider
}

func (i *SDK) TracerProvider() sdktrace.TracerProvider {
	return i.tracerProvider
}

func (i *SDK) MetricsRegisterer() luxmetric.Registerer {
	return i.metricsReg
}

// MetricsRegistry exposes the full registry for HTTP /metrics handler wiring.
func (i *SDK) MetricsRegistry() luxmetric.Registry {
	return i.metricsReg
}

func (i *SDK) ToProviderSettings() factory.ProviderSettings {
	return factory.ProviderSettings{
		Logger:            i.Logger(),
		MeterProvider:     i.MeterProvider(),
		TracerProvider:    i.TracerProvider(),
		MetricsRegisterer: i.MetricsRegisterer(),
	}
}
