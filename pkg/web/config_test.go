package web

import (
	"context"
	"testing"

	"github.com/hanzoai/o11y/pkg/config"
	"github.com/hanzoai/o11y/pkg/config/envprovider"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWithEnvProvider(t *testing.T) {
	t.Setenv("O11Y_WEB_ENABLED", "false")

	conf, err := config.New(
		context.Background(),
		config.ResolverConfig{
			Uris: []string{"env:"},
			ProviderFactories: []config.ProviderFactory{
				envprovider.NewFactory(),
			},
		},
		[]factory.ConfigFactory{
			NewConfigFactory(),
		},
	)
	require.NoError(t, err)

	actual := &Config{}
	err = conf.Unmarshal("web", actual)
	require.NoError(t, err)

	def := NewConfigFactory().New().(*Config)

	expected := &Config{
		Enabled:   false,
		Index:     def.Index,
		Directory: def.Directory,
		Settings:  def.Settings,
	}

	assert.Equal(t, expected, actual)
}

func TestSettingsConfigWithEnvProvider(t *testing.T) {
	testCases := []struct {
		name     string
		env      string
		value    string
		expected SettingsConfig
	}{
		{name: "sentry_enabled", env: "O11Y_WEB_SETTINGS_SENTRY_ENABLED", value: "true", expected: SettingsConfig{Sentry: SentryConfig{Enabled: true}}},
		{name: "sentry_dsn", env: "O11Y_WEB_SETTINGS_SENTRY_DSN", value: "https://examplePublicKey@o0.ingest.sentry.io/0", expected: SettingsConfig{Sentry: SentryConfig{DSN: "https://examplePublicKey@o0.ingest.sentry.io/0"}}},
		{name: "sentry_tunnel", env: "O11Y_WEB_SETTINGS_SENTRY_TUNNEL", value: "https://example.com/tunnel", expected: SettingsConfig{Sentry: SentryConfig{Tunnel: "https://example.com/tunnel"}}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(testCase.env, testCase.value)

			conf, err := config.New(
				context.Background(),
				config.ResolverConfig{
					Uris: []string{"env:"},
					ProviderFactories: []config.ProviderFactory{
						envprovider.NewFactory(),
					},
				},
				[]factory.ConfigFactory{
					NewConfigFactory(),
				},
			)
			require.NoError(t, err)

			actual := &Config{}
			err = conf.Unmarshal("web", actual)
			require.NoError(t, err)

			assert.Equal(t, testCase.expected, actual.Settings)
		})
	}
}
