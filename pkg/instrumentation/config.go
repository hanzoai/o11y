package instrumentation

import (
	"log/slog"

	"github.com/hanzoai/o11y/pkg/factory"
)

// Config holds the configuration for all instrumentation components.
type Config struct {
	Logs     LogsConfig    `mapstructure:"logs"`
	Traces   TracesConfig  `mapstructure:"traces"`
	Metrics  MetricsConfig `mapstructure:"metrics"`
	Resource Resource      `mapstructure:"resource"`
}

// Resource defines the configuration for OpenTelemetry resource attributes.
// Values are stringified — see sdk.go's toString helper.
type Resource struct {
	Attributes map[string]any `mapstructure:"attributes"`
}

// LogsConfig holds the configuration for the logging component.
type LogsConfig struct {
	Level slog.Level `mapstructure:"level"`
}

// TracesConfig holds the configuration for the tracing component.
//
// When Enabled is true the SDK creates an in-process TracerProvider with
// no exporter — wire luxfi/trace.New(...) with Type=ZAP to ship spans.
type TracesConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// MetricsConfig holds the configuration for the application metrics endpoint.
//
// Backend is luxfi/metric — the Lux scrape-compatible text format library.
// No protobuf, no OTel-to-prometheus bridge. The /metrics endpoint is
// served from the host process via metric.NewHTTPHandler(registry, opts).
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
}

func NewConfigFactory() factory.ConfigFactory {
	return factory.NewConfigFactory(factory.MustNewName("instrumentation"), newConfig)
}

func newConfig() factory.Config {
	return Config{
		Logs: LogsConfig{
			Level: slog.LevelInfo,
		},
		Traces: TracesConfig{
			Enabled: false,
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Host:    "0.0.0.0",
			Port:    9090,
		},
	}
}

func (c Config) Validate() error {
	return nil
}
