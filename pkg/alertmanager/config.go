package alertmanager

import (
	"net/url"
	"strings"
	"time"

	"github.com/hanzoai/o11y/pkg/alertmanager/alertmanagerserver"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
)

type Config struct {
	// Provider is the provider for the alertmanager service.
	Provider string `mapstructure:"provider"`

	// Internal is the internal alertmanager configuration.
	O11y O11y `mapstructure:"observe" yaml:"observe"`
}

type O11y struct {
	// PollInterval is the interval at which the alertmanager is synced.
	PollInterval time.Duration `mapstructure:"poll_interval"`

	// Config is the config for the alertmanager server.
	alertmanagerserver.Config `mapstructure:",squash" yaml:",squash"`
}

type Legacy struct {
	// ApiURL is the URL of the legacy o11y alertmanager.
	ApiURL *url.URL `mapstructure:"api_url"`
}

func NewConfigFactory() factory.ConfigFactory {
	return factory.NewConfigFactory(factory.MustNewName("alertmanager"), newConfig)
}

func newConfig() factory.Config {
	return Config{
		Provider: "observe",
		O11y: O11y{
			PollInterval: 1 * time.Minute,
			Config:       alertmanagerserver.NewConfig(),
		},
	}
}

func (c Config) Validate() error {
	if c.Provider != "observe" {
		return errors.Newf(errors.TypeInvalidInput, errors.CodeInvalidInput, "provider must be one of [%s], got %s", strings.Join([]string{"observe"}, ", "), c.Provider)
	}

	return nil
}
