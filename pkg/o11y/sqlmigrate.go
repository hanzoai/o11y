package o11y

import (
	"context"
	"database/sql"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/hanzoai/o11y/pkg/factory"
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
)

// MigrateSQL brings the control-plane tables in db to the current version. A host that
// owns the pool (its connection budget, its schema) calls it ahead of New to carry rows
// between engines before the runtime starts. postgres names the engine db speaks.
//
// The two migrations that read the telemetry store for old query shapes are given one
// that answers nothing: they skip what they cannot read, which on a store with no
// rules or dashboards of that age is everything.
func MigrateSQL(ctx context.Context, settings factory.ProviderSettings, db *sql.DB, postgres bool) error {
	var (
		store  sqlstore.SQLStore
		schema sqlschema.SQLSchema
		err    error
	)
	if postgres {
		store = postgressqlstore.NewWithDB(ctx, settings, db)
		schema, err = postgressqlschema.New(ctx, settings, sqlschema.Config{}, store)
	} else {
		store = sqlitesqlstore.NewWithDB(ctx, settings, db)
		schema, err = sqlitesqlschema.New(ctx, settings, sqlschema.Config{}, store)
	}
	if err != nil {
		return err
	}

	var telemetry telemetrystore.TelemetryStore = telemetrystoretest.New(telemetrystore.Config{}, sqlmock.QueryMatcherRegexp)
	migrations, err := sqlmigration.New(ctx, settings, sqlmigration.Config{}, NewSQLMigrationProviderFactories(store, schema, telemetry, settings))
	if err != nil {
		return err
	}

	cfg := sqlmigrator.Config{Lock: sqlmigrator.Lock{Timeout: time.Minute, Interval: time.Second}}
	return sqlmigrator.New(ctx, settings, store, migrations, cfg).Migrate(ctx)
}
