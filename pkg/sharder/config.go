package sharder

import (
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type Config struct {
	Provider string `mapstructure:"provider"`
	Single   Single `mapstructure:"single"`
}

type Single struct {
	OrgID valuer.UUID `mapstructure:"org_id"`
}

func NewConfigFactory() factory.ConfigFactory {
	return factory.NewConfigFactory(factory.MustNewName("sharder"), newConfig)
}

func newConfig() factory.Config {
	return &Config{
		Provider: "noop",
		Single: Single{
			OrgID: valuer.UUID{},
		},
	}
}

func (c Config) Validate() error {
	return nil
}
