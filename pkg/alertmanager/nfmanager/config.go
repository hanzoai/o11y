package nfmanager

import "github.com/hanzoai/o11y/pkg/factory"

type Config struct {
}

func NewConfigFactory() factory.ConfigFactory {
	return factory.NewConfigFactory(factory.MustNewName("nfmanager"), newConfig)
}

func newConfig() factory.Config {
	return Config{}
}

func (c Config) Validate() error {
	return nil
}
