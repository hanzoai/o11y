package configtest

import (
	"github.com/hanzoai/o11y/pkg/config"
	"github.com/hanzoai/o11y/pkg/config/envprovider"
)

func NewResolverConfig() config.ResolverConfig {
	return config.ResolverConfig{
		Uris:              []string{"env:"},
		ProviderFactories: []config.ProviderFactory{envprovider.NewFactory()},
	}
}
