package identn

import (
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
)

type Config struct {
	// Config for the Hanzo IAM session identN resolver (the sole human identity)
	IAM IAMConfig `mapstructure:"iam"`

	// Config for tokenizer identN resolver
	Tokenizer TokenizerConfig `mapstructure:"tokenizer"`

	// Config for apikey identN resolver
	APIKeyConfig APIKeyConfig `mapstructure:"apikey"`

	// Config for impersonation identN resolver
	Impersonation ImpersonationConfig `mapstructure:"impersonation"`
}

// IAMConfig toggles the Hanzo IAM session identN resolver. Identity is taken
// from the gateway-injected Hanzo IAM session headers (X-Org-Id/X-User-Id/
// X-User-Email); there are no headers to configure — they are a fixed contract.
type IAMConfig struct {
	// Toggles the identN resolver
	Enabled bool `mapstructure:"enabled"`
}

type ImpersonationConfig struct {
	// Toggles the identN resolver
	Enabled bool `mapstructure:"enabled"`
}

type TokenizerConfig struct {
	// Toggles the identN resolver
	Enabled bool `mapstructure:"enabled"`

	// Headers to extract from incoming requests
	Headers []string `mapstructure:"headers"`
}

type APIKeyConfig struct {
	// Toggles the identN resolver
	Enabled bool `mapstructure:"enabled"`

	// Headers to extract from incoming requests
	Headers []string `mapstructure:"headers"`
}

func NewConfigFactory() factory.ConfigFactory {
	return factory.NewConfigFactory(factory.MustNewName("identn"), newConfig)
}

func newConfig() factory.Config {
	return &Config{
		IAM: IAMConfig{
			Enabled: true,
		},
		Tokenizer: TokenizerConfig{
			Enabled: true,
			Headers: []string{"Authorization", "Sec-WebSocket-Protocol"},
		},
		APIKeyConfig: APIKeyConfig{
			Enabled: true,
			Headers: []string{"O11Y-API-KEY"},
		},
		Impersonation: ImpersonationConfig{
			Enabled: false,
		},
	}
}

func (c Config) Validate() error {
	if c.Impersonation.Enabled {
		if c.IAM.Enabled {
			return errors.New(errors.TypeInvalidInput, errors.CodeInvalidInput, "identn::impersonation cannot be enabled if identn::iam is enabled")
		}

		if c.Tokenizer.Enabled {
			return errors.New(errors.TypeInvalidInput, errors.CodeInvalidInput, "identn::impersonation cannot be enabled if identn::tokenizer is enabled")
		}

		if c.APIKeyConfig.Enabled {
			return errors.New(errors.TypeInvalidInput, errors.CodeInvalidInput, "identn::impersonation cannot be enabled if identn::apikey is enabled")
		}
	}

	return nil
}
