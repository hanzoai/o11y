package sqlitesqlstore

import (
	"context"
	"database/sql"
	"log/slog"
	"net/url"
	"strconv"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	// hanzoai/sqlite is the one dual-backend driver: importing it registers the
	// "sqlite" database/sql driver (mattn/SQLCipher under cgo, modernc pure-Go
	// otherwise) AND exposes backend-neutral constraint-error classification, so
	// this package no longer imports modernc directly (which would double-register
	// "sqlite" in the cgo cloud binary).
	"github.com/hanzoai/sqlite"
)

type provider struct {
	settings  factory.ScopedProviderSettings
	sqldb     *sql.DB
	bundb     *sqlstore.BunDB
	dialect   *dialect
	formatter sqlstore.SQLFormatter
}

func NewFactory(hookFactories ...factory.ProviderFactory[sqlstore.SQLStoreHook, sqlstore.Config]) factory.ProviderFactory[sqlstore.SQLStore, sqlstore.Config] {
	return factory.NewProviderFactory(factory.MustNewName("sqlite"), func(ctx context.Context, providerSettings factory.ProviderSettings, config sqlstore.Config) (sqlstore.SQLStore, error) {
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

func New(ctx context.Context, providerSettings factory.ProviderSettings, config sqlstore.Config, hooks ...sqlstore.SQLStoreHook) (sqlstore.SQLStore, error) {
	settings := factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/sqlitesqlstore")

	// Build the DSN with backend-correct pragma syntax. hanzoai/sqlite.PragmaDSN
	// emits `_pragma=journal_mode(wal)` under the modernc backend and
	// `_journal_mode=wal` under the mattn/SQLCipher backend, so the pragmas
	// actually apply whichever backend the binary links — this package is built
	// CGO=0 standalone but CGO=1 inside the cloud binary, and a single hardcoded
	// syntax is silently dropped by the other backend. busy_timeout MUST lead
	// (WAL cannot be enabled while another connection holds the db). _txlock is a
	// driver connection param (not a pragma) that both backends honor; append it.
	dsn := sqlite.PragmaDSN(config.Sqlite.Path, []sqlite.Pragma{
		{Name: "busy_timeout", Value: strconv.FormatInt(config.Sqlite.BusyTimeout.Milliseconds(), 10)},
		{Name: "journal_mode", Value: config.Sqlite.Mode},
		{Name: "foreign_keys", Value: "1"},
	}) + "&_txlock=" + url.QueryEscape(config.Sqlite.TransactionMode)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	settings.Logger().InfoContext(ctx, "connected to sqlite", slog.String("path", config.Sqlite.Path))
	sqldb.SetMaxOpenConns(config.Connection.MaxOpenConns)
	sqldb.SetConnMaxLifetime(config.Connection.MaxConnLifetime)

	sqliteDialect := sqlitedialect.New()
	bunDB := sqlstore.NewBunDB(settings, sqldb, sqliteDialect, hooks)
	return &provider{
		settings:  settings,
		sqldb:     sqldb,
		bundb:     bunDB,
		dialect:   new(dialect),
		formatter: newFormatter(bunDB.Dialect()),
	}, nil
}

func (provider *provider) BunDB() *bun.DB {
	return provider.bundb.DB
}

func (provider *provider) SQLDB() *sql.DB {
	return provider.sqldb
}

func (provider *provider) Dialect() sqlstore.SQLDialect {
	return provider.dialect
}

func (provider *provider) Formatter() sqlstore.SQLFormatter {
	return provider.formatter
}

func (provider *provider) BunDBCtx(ctx context.Context) bun.IDB {
	return provider.bundb.BunDBCtx(ctx)
}

func (provider *provider) RunInTxCtx(ctx context.Context, opts *sql.TxOptions, cb func(ctx context.Context) error) error {
	return provider.bundb.RunInTxCtx(ctx, opts, cb)
}

func (provider *provider) WrapNotFoundErrf(err error, code errors.Code, format string, args ...any) error {
	if err == sql.ErrNoRows {
		return errors.Wrapf(err, errors.TypeNotFound, code, format, args...)
	}

	return err
}

func (provider *provider) WrapAlreadyExistsErrf(err error, code errors.Code, format string, args ...any) error {
	if sqlite.IsConstraintUnique(err) || sqlite.IsConstraintPrimaryKey(err) || sqlite.IsConstraintForeignKey(err) {
		return errors.Wrapf(err, errors.TypeAlreadyExists, code, format, args...)
	}

	return err
}
