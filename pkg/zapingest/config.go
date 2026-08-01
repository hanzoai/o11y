package zapingest

import (
	"net"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
)

// Config declares where the three ZAP signals are received. One address per
// signal, because each receiver runs its own zap.Node and therefore its own
// listener. 4317 is the span port zapreceiver already defaults to; logs and
// metrics take the next two.
type Config struct {
	Enabled bool   `mapstructure:"enabled"`
	Spans   string `mapstructure:"spans"`
	Logs    string `mapstructure:"logs"`
	Metrics string `mapstructure:"metrics"`
}

func NewConfigFactory() factory.ConfigFactory {
	return factory.NewConfigFactory(factory.MustNewName("zapingest"), newConfig)
}

func newConfig() factory.Config {
	return Config{
		Enabled: true,
		Spans:   "0.0.0.0:4317",
		Logs:    "0.0.0.0:4318",
		Metrics: "0.0.0.0:4319",
	}
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	for name, addr := range map[string]string{"spans": c.Spans, "logs": c.Logs, "metrics": c.Metrics} {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			return errors.WrapInvalidInputf(err, errors.CodeInvalidInput, "zapingest %s address %q is not host:port", name, addr)
		}
	}
	if c.Spans == c.Logs || c.Spans == c.Metrics || c.Logs == c.Metrics {
		return errors.NewInvalidInputf(errors.CodeInvalidInput, "zapingest needs one address per signal, got spans=%s logs=%s metrics=%s", c.Spans, c.Logs, c.Metrics)
	}
	return nil
}
