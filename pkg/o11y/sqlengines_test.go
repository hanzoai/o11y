package o11y

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/factory/factorytest"
	"github.com/hanzoai/o11y/pkg/modules/user/impluser"
	"github.com/hanzoai/o11y/pkg/sqlmigration"
	"github.com/hanzoai/o11y/pkg/sqlmigrator"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlschema/postgressqlschema"
	"github.com/hanzoai/o11y/pkg/sqlschema/sqlitesqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/postgressqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlitesqlstore"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/telemetrystoretest"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// engine opens a fresh, empty control-plane store on one engine and answers it with its
// schema inspector. The postgres leg needs O11Y_TEST_POSTGRES_DSN (a database the test
// may create schemas in) and is skipped without it, except under CI.
type engine struct {
	name string
	open func(t *testing.T) (sqlstore.SQLStore, sqlschema.SQLSchema)
}

func engines() []engine {
	return []engine{
		{"sqlite", func(t *testing.T) (sqlstore.SQLStore, sqlschema.SQLSchema) {
			settings := factorytest.NewSettings()
			store, err := sqlitesqlstore.New(context.Background(), settings, sqlstore.Config{
				Provider:   "sqlite",
				Connection: sqlstore.ConnectionConfig{MaxOpenConns: 100},
				Sqlite:     sqlstore.SqliteConfig{Path: filepath.Join(t.TempDir(), "o11y.db"), Mode: "wal", BusyTimeout: 5 * time.Second, TransactionMode: "deferred"},
			})
			require.NoError(t, err)
			schema, err := sqlitesqlschema.New(context.Background(), settings, sqlschema.Config{}, store)
			require.NoError(t, err)
			return store, schema
		}},
		{"postgres", func(t *testing.T) (sqlstore.SQLStore, sqlschema.SQLSchema) {
			dsn := os.Getenv("O11Y_TEST_POSTGRES_DSN")
			if dsn == "" {
				if os.Getenv("CI") != "" {
					t.Fatal("O11Y_TEST_POSTGRES_DSN is not set")
				}
				t.Skip("O11Y_TEST_POSTGRES_DSN is not set")
			}
			admin, err := sql.Open("pgx", dsn)
			require.NoError(t, err)
			name := fmt.Sprintf("t_%d", time.Now().UnixNano())
			_, err = admin.Exec(`CREATE SCHEMA "` + name + `"`)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, _ = admin.Exec(`DROP SCHEMA "` + name + `" CASCADE`)
				_ = admin.Close()
			})

			u, err := url.Parse(dsn)
			require.NoError(t, err)
			q := u.Query()
			q.Set("search_path", name)
			u.RawQuery = q.Encode()

			settings := factorytest.NewSettings()
			store, err := postgressqlstore.New(context.Background(), settings, sqlstore.Config{
				Provider:   "postgres",
				Connection: sqlstore.ConnectionConfig{MaxOpenConns: 8},
				Postgres:   sqlstore.PostgresConfig{DSN: u.String()},
			})
			require.NoError(t, err)
			t.Cleanup(func() { _ = store.SQLDB().Close() })
			schema, err := postgressqlschema.New(context.Background(), settings, sqlschema.Config{}, store)
			require.NoError(t, err)
			return store, schema
		}},
	}
}

func migrate(t *testing.T, store sqlstore.SQLStore, schema sqlschema.SQLSchema) {
	t.Helper()
	ctx := context.Background()
	settings := factorytest.NewSettings()
	var telemetry telemetrystore.TelemetryStore = telemetrystoretest.New(telemetrystore.Config{}, sqlmock.QueryMatcherRegexp)
	migrations, err := sqlmigration.New(ctx, settings, sqlmigration.Config{}, NewSQLMigrationProviderFactories(store, schema, telemetry, settings))
	require.NoError(t, err)
	cfg := sqlmigrator.Config{Lock: sqlmigrator.Lock{Timeout: 10 * time.Second, Interval: 100 * time.Millisecond}}
	m := sqlmigrator.New(ctx, settings, store, migrations, cfg)
	require.NoError(t, m.Migrate(ctx))
	// a second run is a no-op
	require.NoError(t, m.Migrate(ctx))
	require.NoError(t, m.Check(ctx))
}

var _ factory.ProviderSettings

// Every migration, in order, on each engine.
func TestSQLMigrationsRunOnEveryEngine(t *testing.T) {
	for _, e := range engines() {
		t.Run(e.name, func(t *testing.T) {
			store, schema := e.open(t)
			migrate(t, store, schema)
		})
	}
}

var authzTables = map[string]bool{"tuple": true, "authorization_model": true, "changelog": true, "assertion": true, "store": true}

// columnsOf lists "table.column" for every table the migrations leave, the way each
// engine spells its catalog.
func columnsOf(t *testing.T, name string, store sqlstore.SQLStore) map[string]bool {
	t.Helper()
	q := `SELECT m.name || '.' || p.name FROM sqlite_master m, pragma_table_info(m.name) p WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'`
	if name == "postgres" {
		q = `SELECT table_name || '.' || column_name FROM information_schema.columns WHERE table_schema = current_schema()`
	}
	rows, err := store.SQLDB().Query(q)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		// The authorization store takes its own layout per engine (migration 054
		// rewrites it on postgres), so it is not expected to match.
		if table, _, _ := strings.Cut(s, "."); authzTables[table] {
			continue
		}
		out[s] = true
	}
	return out
}

// The two engines must leave the same tables and columns.
func TestSQLEnginesLeaveTheSameSchema(t *testing.T) {
	got := map[string]map[string]bool{}
	for _, e := range engines() {
		store, schema := e.open(t)
		migrate(t, store, schema)
		got[e.name] = columnsOf(t, e.name, store)
	}
	require.NotEmpty(t, got["sqlite"])
	for c := range got["sqlite"] {
		require.True(t, got["postgres"][c], "postgres lacks %s", c)
	}
	for c := range got["postgres"] {
		require.True(t, got["sqlite"][c], "sqlite lacks %s", c)
	}
}

// A primary key that other tables point at moves in one transaction on each engine.
func TestRekeyOnEveryEngine(t *testing.T) {
	for _, e := range engines() {
		t.Run(e.name, func(t *testing.T) {
			ctx := context.Background()
			store, schema := e.open(t)
			migrate(t, store, schema)

			org := types.NewOrganization("hanzo", "hanzo")
			_, err := store.BunDB().NewInsert().Model(org).Exec(ctx)
			require.NoError(t, err)
			from, to := valuer.GenerateUUID(), valuer.GenerateUUID()
			u, err := types.NewUserWithID(from, "z", valuer.MustNewEmail("z@hanzo.ai"), org.ID, types.UserStatusActive)
			require.NoError(t, err)
			_, err = store.BunDB().NewInsert().Model(u).Exec(ctx)
			require.NoError(t, err)
			_, err = store.BunDB().NewRaw(`INSERT INTO user_preference (id, name, value, user_id) VALUES (?, 'theme', 'dark', ?)`, valuer.GenerateUUID(), from).Exec(ctx)
			require.NoError(t, err)

			users := impluser.NewStore(store, factorytest.NewSettings())
			require.NoError(t, store.RunInTxCtx(ctx, nil, func(ctx context.Context) error {
				return users.Rekey(ctx, org.ID, from, to)
			}))

			for table, column := range map[string]string{"users": "id", "user_preference": "user_id"} {
				var moved, stale int
				require.NoError(t, store.BunDB().NewRaw("SELECT COUNT(*) FROM "+table+" WHERE "+column+" = ?", to).Scan(ctx, &moved))
				require.NoError(t, store.BunDB().NewRaw("SELECT COUNT(*) FROM "+table+" WHERE "+column+" = ?", from).Scan(ctx, &stale))
				require.Equal(t, 1, moved, table)
				require.Equal(t, 0, stale, table)
			}
		})
	}
}
