package user

import (
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Config is what remains once o11y stopped holding credentials: WHICH row the
// local single-user mode stands in for, and nothing about how anyone proves who
// they are. The password reset and invite token lifetimes went with the tokens.
type Config struct {
	Root RootConfig `mapstructure:"root"`
}

// RootConfig seeds the one row that impersonationidentn resolves to in local
// single-user mode. It carries NO password: nothing authenticates against this
// row — impersonation matches every request unconditionally, which is why
// identn::impersonation refuses to boot alongside any real resolver.
type RootConfig struct {
	Enabled bool         `mapstructure:"enabled"`
	Email   valuer.Email `mapstructure:"email"`
	Org     OrgConfig    `mapstructure:"org"`
}

type OrgConfig struct {
	ID   valuer.UUID `mapstructure:"id"`
	Name string      `mapstructure:"name"`
}

func NewConfigFactory() factory.ConfigFactory {
	return factory.NewConfigFactory(factory.MustNewName("user"), newConfig)
}

func newConfig() factory.Config {
	return &Config{
		Root: RootConfig{
			Enabled: false,
			Org: OrgConfig{
				Name: "default",
			},
		},
	}
}

func (c Config) Validate() error {
	if c.Root.Enabled && c.Root.Email.IsZero() {
		return errors.New(errors.TypeInvalidInput, errors.CodeInvalidInput, "user::root::email is required when root user is enabled")
	}

	return nil
}
