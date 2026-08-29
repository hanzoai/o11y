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

// TestDriverRegisteredOnce opens the provider and drives the "sqlite" driver name
// end to end, under whichever CGO mode go test runs.
//
// It catches the two ways this provider regresses. A duplicate
// sql.Register("sqlite") panics during init, so the binary cannot reach the body
// below if anything in the graph claims that name twice — which is what importing
// a backend directly alongside the hanzoai/sqlite facade does under CGO_ENABLED=1,
// where the facade already registers the name against csqlite. And the pragma
// assertions read back through the driver, so a DSN in the other backend's dialect
// is caught here rather than silently ignored at open.
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
	require.Equal(t, "wal", journalMode, "journal_mode not applied — PragmaDSN emitted a form the linked backend ignores")

	var busyTimeout int
	require.NoError(t, sqldb.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout))
	require.Greater(t, busyTimeout, 0, "busy_timeout not applied")

	t.Logf("journal_mode=%s busy_timeout=%d", journalMode, busyTimeout)
}
