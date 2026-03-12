package cmd

import (
	"context"
	"log/slog"

	"github.com/hanzoai/o11y/pkg/config"
	"github.com/hanzoai/o11y/pkg/config/envprovider"
	"github.com/hanzoai/o11y/pkg/config/fileprovider"
	"github.com/hanzoai/o11y/pkg/o11y"
)

func NewHanzoO11yConfig(ctx context.Context, logger *slog.Logger, flags o11y.DeprecatedFlags) (o11y.Config, error) {
	config, err := o11y.NewConfig(
		ctx,
		logger,
		config.ResolverConfig{
			Uris: []string{"env:"},
			ProviderFactories: []config.ProviderFactory{
				envprovider.NewFactory(),
				fileprovider.NewFactory(),
			},
		},
		flags,
	)
	if err != nil {
		return o11y.Config{}, err
	}

	return config, nil
}
