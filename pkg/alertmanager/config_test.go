package alertmanager

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/config"
	"github.com/hanzoai/o11y/pkg/config/envprovider"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWithEnvProvider(t *testing.T) {
	t.Setenv("HANZO_ALERTMANAGER_PROVIDER", "observe")
	t.Setenv("HANZO_ALERTMANAGER_LEGACY_API__URL", "http://localhost:9093/api")
	t.Setenv("HANZO_ALERTMANAGER_HANZO_ROUTE_REPEAT__INTERVAL", "5m")
	t.Setenv("HANZO_ALERTMANAGER_HANZO_EXTERNAL__URL", "https://example.com/test")
	t.Setenv("HANZO_ALERTMANAGER_HANZO_GLOBAL_RESOLVE__TIMEOUT", "10s")

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
	err = conf.Unmarshal("alertmanager", actual, "yaml")
	require.NoError(t, err)

	def := NewConfigFactory().New().(Config)
	def.O11y.Global.ResolveTimeout = model.Duration(10 * time.Second)
	def.O11y.Route.RepeatInterval = 5 * time.Minute
	def.O11y.ExternalURL = &url.URL{
		Scheme: "https",
		Host:   "example.com",
		Path:   "/test",
	}

	expected := &Config{
		Provider: "observe",
		O11y:   def.O11y,
	}

	assert.Equal(t, expected, actual)
	assert.NoError(t, actual.Validate())
}
