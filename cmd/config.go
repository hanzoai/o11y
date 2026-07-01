package cmd

import (
	"context"
	"log/slog"

	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/config"
	"github.com/hanzoai/o11y/pkg/config/envprovider"
	"github.com/hanzoai/o11y/pkg/config/fileprovider"
)

func NewHanzoO11yConfig(ctx context.Context, logger *slog.Logger, configFiles []string, flags o11y.DeprecatedFlags) (o11y.Config, error) {
	uris := make([]string, 0, len(configFiles)+1)
	for _, f := range configFiles {
		uris = append(uris, "file:"+f)
	}
	uris = append(uris, "env:")

	config, err := o11y.NewConfig(
		ctx,
		logger,
		config.ResolverConfig{
			Uris: uris,
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
