package o11yglobal

import (
	"context"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/global"
)

type provider struct {
	config       global.Config
	identNConfig identn.Config
	settings     factory.ScopedProviderSettings
}

func NewFactory() factory.ProviderFactory[global.Global, global.Config] {
	return factory.NewProviderFactory(factory.MustNewName("observe"), func(ctx context.Context, providerSettings factory.ProviderSettings, config global.Config) (global.Global, error) {
		return newProvider(ctx, providerSettings, config)
	})
}

func newProvider(_ context.Context, providerSettings factory.ProviderSettings, config global.Config) (global.Global, error) {
	settings := factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/global/o11yglobal")
	return &provider{
		config:       config,
		identNConfig: identNConfig,
		settings:     settings,
	}, nil
}

func (provider *provider) GetConfig(context.Context) *globaltypes.Config {
	var mcpURL *string
	if provider.config.MCPURL != nil {
		s := provider.config.MCPURL.String()
		mcpURL = &s
	}

	var aiAssistantURL *string
	if provider.config.AIAssistantURL != nil {
		s := provider.config.AIAssistantURL.String()
		aiAssistantURL = &s
	}

	return globaltypes.NewConfig(
		globaltypes.NewEndpoint(
			provider.config.ExternalURL.String(),
			provider.config.IngestionURL.String(),
			mcpURL,
			aiAssistantURL,
		),
		globaltypes.NewIdentNConfig(
			globaltypes.TokenizerConfig{Enabled: provider.identNConfig.Tokenizer.Enabled},
			globaltypes.APIKeyConfig{Enabled: provider.identNConfig.APIKeyConfig.Enabled},
			globaltypes.ImpersonationConfig{Enabled: provider.identNConfig.Impersonation.Enabled},
		),
	)
}
