package apiserver

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/config"
	"github.com/hanzoai/o11y/pkg/config/envprovider"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWithEnvProvider(t *testing.T) {
	t.Setenv("O11Y_APISERVER_TIMEOUT_DEFAULT", "70s")
	t.Setenv("O11Y_APISERVER_TIMEOUT_MAX", "700s")
	t.Setenv("O11Y_APISERVER_TIMEOUT_EXCLUDED__ROUTES", "/excluded1,/excluded2")
	t.Setenv("O11Y_APISERVER_LOGGING_EXCLUDED__ROUTES", "/v1/o11y/v1/health1")

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
	err = conf.Unmarshal("apiserver", actual)

	require.NoError(t, err)

	expected := &Config{
		Timeout: Timeout{
			Default: 70 * time.Second,
			Max:     700 * time.Second,
			ExcludedRoutes: []string{
				"/excluded1",
				"/excluded2",
			},
		},
		Logging: Logging{
			ExcludedRoutes: []string{
				"/v1/o11y/v1/health1",
			},
		},
	}

	assert.Equal(t, expected, actual)
}
