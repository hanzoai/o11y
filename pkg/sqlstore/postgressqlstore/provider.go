// Package postgressqlstore is the PostgreSQL provider of the sqlstore: the same
// control-plane tables on a server, so a deployment keeps no state on a local disk.
package postgressqlstore

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	o11yerrors "github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlstore"
)

type provider struct {
	settings  factory.ScopedProviderSettings
	sqldb     *sql.DB
	bundb     *sqlstore.BunDB
	dialect   *dialect
	formatter sqlstore.SQLFormatter
}

func NewFactory(hookFactories ...factory.ProviderFactory[sqlstore.SQLStoreHook, sqlstore.Config]) factory.ProviderFactory[sqlstore.SQLStore, sqlstore.Config] {
	return factory.NewProviderFactory(factory.MustNewName("postgres"), func(ctx context.Context, providerSettings factory.ProviderSettings, config sqlstore.Config) (sqlstore.SQLStore, error) {
		hooks := make([]sqlstore.SQLStoreHook, len(hookFactories))
		for i, hookFactory := range hookFactories {
			hook, err := hookFactory.New(ctx, providerSettings, config)
			if err != nil {
				return nil, err
			}
			hooks[i] = hook
		}

		return New(ctx, providerSettings, config, hooks...)
	})
}

var pool atomic.Pointer[sql.DB]

// UsePool hands the provider a pool the host owns (its connection budget, a
// search_path bound to the store's schema). New wraps it instead of dialling
// config.Postgres.DSN, and nothing here closes it. A process builds one runtime, so
// one pool is the whole of it.
func UsePool(db *sql.DB) { pool.Store(db) }

// New opens the server named by config.Postgres.DSN, or wraps the pool given to
// UsePool. Put the store's schema in the DSN (search_path) to keep it apart from other
// tenants of the database.
func New(ctx context.Context, providerSettings factory.ProviderSettings, config sqlstore.Config, hooks ...sqlstore.SQLStoreHook) (sqlstore.SQLStore, error) {
	if db := pool.Load(); db != nil {
		return NewWithDB(ctx, providerSettings, db, hooks...), nil
	}
	if config.Postgres.DSN == "" {
		return nil, o11yerrors.NewInvalidInputf(o11yerrors.CodeInvalidInput, "sqlstore.postgres.dsn is required for the postgres provider")
	}
	sqldb, err := sql.Open("pgx", config.Postgres.DSN)
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(config.Connection.MaxOpenConns)
	sqldb.SetConnMaxLifetime(config.Connection.MaxConnLifetime)
	if err := sqldb.PingContext(ctx); err != nil {
		_ = sqldb.Close()
		return nil, err
	}

	return NewWithDB(ctx, providerSettings, sqldb, hooks...), nil
}

// NewWithDB wraps a pool the host already owns (its connection budget, its
// search_path). The store never closes it.
func NewWithDB(ctx context.Context, providerSettings factory.ProviderSettings, sqldb *sql.DB, hooks ...sqlstore.SQLStoreHook) sqlstore.SQLStore {
	settings := factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/postgressqlstore")
	settings.Logger().InfoContext(ctx, "connected to postgres", slog.String("driver", "pgx"))

	bunDB := sqlstore.NewBunDB(settings, sqldb, pgdialect.New(), hooks)
	return &provider{
		settings:  settings,
		sqldb:     sqldb,
		bundb:     bunDB,
		dialect:   new(dialect),
		formatter: newFormatter(bunDB.Dialect()),
	}
}

func (provider *provider) BunDB() *bun.DB                   { return provider.bundb.DB }
func (provider *provider) SQLDB() *sql.DB                   { return provider.sqldb }
func (provider *provider) Dialect() sqlstore.SQLDialect     { return provider.dialect }
func (provider *provider) Formatter() sqlstore.SQLFormatter { return provider.formatter }
func (provider *provider) BunDBCtx(ctx context.Context) bun.IDB {
	return provider.bundb.BunDBCtx(ctx)
}

func (provider *provider) RunInTxCtx(ctx context.Context, opts *sql.TxOptions, cb func(ctx context.Context) error) error {
	return provider.bundb.RunInTxCtx(ctx, opts, cb)
}

func (provider *provider) WrapNotFoundErrf(err error, code o11yerrors.Code, format string, args ...any) error {
	if err == sql.ErrNoRows {
		return o11yerrors.Wrapf(err, o11yerrors.TypeNotFound, code, format, args...)
	}

	return err
}

// SQLSTATE 23505 is a unique or primary-key violation and 23503 a foreign-key one;
// the sqlite provider counts all three as "already exists".
func (provider *provider) WrapAlreadyExistsErrf(err error, code o11yerrors.Code, format string, args ...any) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23503") {
		return o11yerrors.Wrapf(err, o11yerrors.TypeAlreadyExists, code, format, args...)
	}

	return err
}
