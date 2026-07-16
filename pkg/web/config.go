package web

import (
	"github.com/hanzoai/o11y/pkg/factory"
)

// Config holds the configuration for web.
type Config struct {
	// Whether the web package is enabled.
	Enabled bool `mapstructure:"enabled"`

	// The name of the index file to serve.
	Index string `mapstructure:"index"`

	// The directory from which to serve the web files.
	Directory string `mapstructure:"directory"`

	// Web settings configuration.
	Settings SettingsConfig `mapstructure:"settings"`
}

// SettingsConfig holds the configuration for web settings.
//
// Third-party trackers (product analytics, onboarding tours, support chat) are
// NOT part of Hanzo o11y — see settings.go. Analytics is Hanzo Insights; support
// chat is Hanzo Chat. Only Sentry remains, pointed at our own fork and opt-in.
type SettingsConfig struct {
	Sentry SentryConfig `mapstructure:"sentry"`
}

type SentryConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	DSN     string `mapstructure:"dsn"`
	Tunnel  string `mapstructure:"tunnel"`
}

func NewConfigFactory() factory.ConfigFactory {
	return factory.NewConfigFactory(factory.MustNewName("web"), newConfig)
}

func newConfig() factory.Config {
	return &Config{
		Enabled:   true,
		Index:     "index.html",
		Directory: "/etc/o11y/web",
		Settings: SettingsConfig{
			Sentry: SentryConfig{
				Enabled: false,
			},
		},
	}
}

func (c Config) Validate() error {
	return nil
}

func (c Config) Provider() string {
	if c.Enabled {
		return "router"
	}

	return "noop"
}
