package sqlitesqlstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/factory/factorytest"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/stretchr/testify/require"
)

// TestDriverRegisteredOnce proves the "sqlite" database/sql driver is registered
// exactly once — via this provider's modernc import — and opens cleanly. It is the
// guard for removing the now-redundant blank _ "modernc.org/sqlite" from
// pkg/query-service/app/http_handler.go: registration must survive that removal,
// and pointing that blank import at github.com/hanzoai/sqlite instead would
// double-register "sqlite" under CGO_ENABLED=1 (the fork's cgo backend Register()s
// mattn while modernc's init Register()s modernc) and panic. Opening the provider
// here — under whatever CGO mode the test runs — fails loudly if either regression
// returns. It also asserts the DSN pragmas take effect (journal_mode=wal,
// busy_timeout>0) through the driver.
func TestDriverRegisteredOnce(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "o11y.db")
	store, err := New(context.Background(), factorytest.NewSettings(), sqlstore.Config{
		Provider: "sqlite",
		Connection: sqlstore.ConnectionConfig{
			MaxOpenConns:    1,
			MaxConnLifetime: 0,
		},
		Sqlite: sqlstore.SqliteConfig{
			Path:            dbPath,
			Mode:            "wal",
			BusyTimeout:     5 * time.Second,
			TransactionMode: "deferred",
		},
	})
	require.NoError(t, err)

	sqldb := store.SQLDB()
	require.NoError(t, sqldb.Ping())

	var journalMode string
	require.NoError(t, sqldb.QueryRow("PRAGMA journal_mode").Scan(&journalMode))
	require.Equal(t, "wal", journalMode, "journal_mode not applied — modernc _pragma DSN form not honored")

	var busyTimeout int
	require.NoError(t, sqldb.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout))
	require.Greater(t, busyTimeout, 0, "busy_timeout not applied")

	t.Logf("journal_mode=%s busy_timeout=%d", journalMode, busyTimeout)
}
