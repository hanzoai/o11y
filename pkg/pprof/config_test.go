package pprof

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
	t.Setenv("SIGNOZ_PPROF_ENABLED", "false")
	t.Setenv("SIGNOZ_PPROF_ADDRESS", "127.0.0.1:6061")

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

	actual := Config{}
	err = conf.Unmarshal("pprof", &actual)
	require.NoError(t, err)

	expected := Config{
		Enabled: false,
		Address: "127.0.0.1:6061",
	}

	assert.Equal(t, expected, actual)
}
