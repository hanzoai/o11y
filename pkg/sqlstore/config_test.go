package sqlstore

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/config"
	"github.com/hanzoai/o11y/pkg/config/envprovider"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWithEnvProvider(t *testing.T) {
	t.Setenv("O11Y_SQLSTORE_PROVIDER", "sqlite")
	t.Setenv("O11Y_SQLSTORE_SQLITE_PATH", "/tmp/test.db")
	t.Setenv("O11Y_SQLSTORE_SQLITE_MODE", "wal")
	t.Setenv("O11Y_SQLSTORE_SQLITE_BUSY__TIMEOUT", "5s")
	t.Setenv("O11Y_SQLSTORE_SQLITE_TRANSACTION__MODE", "immediate")
	t.Setenv("O11Y_SQLSTORE_MAX__OPEN__CONNS", "50")
	t.Setenv("O11Y_SQLSTORE_MAX__CONN__LIFETIME", "3h")

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

	actual := &Config{}
	err = conf.Unmarshal("sqlstore", actual)
	require.NoError(t, err)

	expected := &Config{
		Provider: "sqlite",
		Connection: ConnectionConfig{
			MaxOpenConns:    50,
			MaxConnLifetime: time.Hour * 3,
		},
		Sqlite: SqliteConfig{
			Path:            "/tmp/test.db",
			Mode:            "wal",
			BusyTimeout:     5 * time.Second,
			TransactionMode: "immediate",
		},
	}

	assert.Equal(t, expected, actual)
	assert.NoError(t, actual.Validate())
}
