package alertmanager

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/config"
	"github.com/hanzoai/o11y/pkg/config/envprovider"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const prefix = "O11Y_"

// clearEnv unsets all existing O11Y_* env vars for the duration of the test.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, prefix) {
			key := strings.SplitN(kv, "=", 2)[0]
			orig, _ := os.LookupEnv(key)
			_ = os.Unsetenv(key)
			t.Cleanup(func() { _ = os.Setenv(key, orig) })
		}
	}
}

func TestNewWithEnvProvider(t *testing.T) {
	clearEnv(t)
	t.Setenv("O11Y_ALERTMANAGER_PROVIDER", "o11y")
	t.Setenv("O11Y_ALERTMANAGER_LEGACY_API__URL", "http://localhost:9093/api")
	t.Setenv("O11Y_ALERTMANAGER_O11Y_ROUTE_REPEAT__INTERVAL", "5m")
	t.Setenv("O11Y_ALERTMANAGER_O11Y_EXTERNAL__URL", "https://example.com/test")
	t.Setenv("O11Y_ALERTMANAGER_O11Y_GLOBAL_RESOLVE__TIMEOUT", "10s")

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
		Provider: "o11y",
		O11y:     def.O11y,
	}

	assert.Equal(t, expected, actual)
	assert.NoError(t, actual.Validate())
}
