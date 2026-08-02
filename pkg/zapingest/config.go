package zapingest

import (
	"fmt"
	"net"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
)

// Config declares where the three ZAP signals are received. One address per
// signal, because each receiver runs its own zap.Node and therefore its own
// listener.
//
// THE ADDRESS IS THE SWITCH. An empty address means that signal is not
// received in this process. There is no separate `enabled` boolean, because a
// boolean beside three addresses is a second way to state one decision and the
// two can disagree — stating where spans arrive IS the act of deciding that
// spans arrive here.
//
// Every address therefore defaults to EMPTY, and that default is load-bearing:
// linking this package binds nothing. The previous default (enabled, with
// 0.0.0.0:4317-4319 baked in) meant importing the o11y module seized three
// well-known ports as a side effect of being linked, which is how a host
// process and the library it embeds came to race for the same sockets. The
// loser dropped telemetry silently and log ingest fell ~200x. A library must
// not take a port because someone linked it.
//
// The ports a deployment usually wants are the fleet's conventional ones:
// spans 4317, logs 4318, metrics 4319. They are documented here rather than
// defaulted, so the choice is made once, visibly, by the deployment that also
// publishes them.
type Config struct {
	Spans   string `mapstructure:"spans"`
	Logs    string `mapstructure:"logs"`
	Metrics string `mapstructure:"metrics"`
}

func NewConfigFactory() factory.ConfigFactory {
	return factory.NewConfigFactory(factory.MustNewName("zapingest"), newConfig)
}

func newConfig() factory.Config {
	return Config{}
}

// signal is one receivable signal and the address it is received on.
type signal struct {
	name string
	addr string
}

// signals lists the signals this process receives, in a fixed order, omitting
// the ones it does not. It is the ONE place that knows "configured" means
// "non-empty address".
func (c Config) signals() []signal {
	out := make([]signal, 0, 3)
	for _, s := range []signal{{"spans", c.Spans}, {"logs", c.Logs}, {"metrics", c.Metrics}} {
		if s.addr != "" {
			out = append(out, s)
		}
	}
	return out
}

// Validate reports a bad or duplicated address. The message is folded into the
// CAUSE rather than passed to Wrap*f: errors.base.Error() renders the wrapped
// cause and drops the wrapper's own message, so "which signal" would otherwise
// be kept on the value and never printed.
func (c Config) Validate() error {
	configured := c.signals()
	for _, s := range configured {
		if _, _, err := net.SplitHostPort(s.addr); err != nil {
			cause := fmt.Errorf("zapingest %s address %q is not host:port: %w", s.name, s.addr, err)
			return errors.WrapInvalidInputf(cause, errors.CodeInvalidInput, "%s", cause)
		}
	}
	for i, a := range configured {
		for _, b := range configured[i+1:] {
			if a.addr == b.addr {
				return errors.NewInvalidInputf(errors.CodeInvalidInput, "zapingest needs one address per signal, but %s and %s both use %s", a.name, b.name, a.addr)
			}
		}
	}
	return nil
}
