package authz

import (
	"github.com/hanzoai/o11y/pkg/factory"
)

// Config configures o11y's authorization seam. Hanzo IAM is the sole
// authorization provider — every authz decision is delegated to the Hanzo IAM
// Casbin enforce endpoint (see pkg/authz/iamauthz). There is no pluggable
// provider selection: one way to do authz, and it is Hanzo IAM.
type Config struct {
	// Provider names the authorization provider. Retained for telemetry and
	// config-shape stability; the only supported value is "iam".
	Provider string `mapstructure:"provider"`

	// IAM configures the Hanzo IAM integration.
	IAM IAMConfig `mapstructure:"iam"`
}

// IAMConfig configures the Hanzo IAM integration. The client credentials are
// secrets and are read from the environment (O11Y_IAM_CLIENT_ID and
// O11Y_IAM_CLIENT_SECRET) rather than from config files, per the house rule to
// keep secrets out of config and in KMS/env.
type IAMConfig struct {
	// URL is the base URL of the Hanzo IAM API. The enforce endpoints live under
	// {URL}/v1/iam/enforce and {URL}/v1/iam/batch-enforce; policy endpoints under
	// {URL}/v1/iam/{add,remove,get}-poli{cy,cies}.
	URL string `mapstructure:"url"`

	// EnforcerID is the IAM enforcer id (in "owner/name" form) that holds o11y's
	// authorization model. It scopes both enforce calls (as enforcerId) and
	// policy calls (as id).
	EnforcerID string `mapstructure:"enforcer_id"`
}

func NewConfigFactory() factory.ConfigFactory {
	return factory.NewConfigFactory(factory.MustNewName("authz"), newConfig)
}

func newConfig() factory.Config {
	return &Config{
		Provider: "iam",
		IAM: IAMConfig{
			URL:        "https://iam.hanzo.ai",
			EnforcerID: "hanzo/o11y",
		},
	}
}

func (c Config) Validate() error {
	return nil
}
