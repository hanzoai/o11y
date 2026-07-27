package sqlmigration

import (
	"context"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// addErrorTracking creates the one net-new table backing error/crash tracking:
// o11y_issues (grouped-error lifecycle). Occurrences stay in the telemetry store
// (o11y_traces / o11y_logs); only non-derivable lifecycle state lives here. The
// unique index on (org_id, fingerprint) is the grouping key the ingest upsert
// conflicts on; (org_id, last_seen) serves the default list ordering.
type addErrorTracking struct {
	sqlschema sqlschema.SQLSchema
	sqlstore  sqlstore.SQLStore
}

func NewAddErrorTrackingFactory(sqlstore sqlstore.SQLStore, sqlschema sqlschema.SQLSchema) factory.ProviderFactory[SQLMigration, Config] {
	return factory.NewProviderFactory(factory.MustNewName("add_error_tracking"), func(_ context.Context, _ factory.ProviderSettings, _ Config) (SQLMigration, error) {
		return &addErrorTracking{sqlschema: sqlschema, sqlstore: sqlstore}, nil
	})
}

func (migration *addErrorTracking) Register(migrations *migrate.Migrations) error {
	return migrations.Register(migration.Up, migration.Down)
}

func (migration *addErrorTracking) Up(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	orgFK := func(col string) *sqlschema.ForeignKeyConstraint {
		return &sqlschema.ForeignKeyConstraint{
			ReferencingColumnName: sqlschema.ColumnName(col),
			ReferencedTableName:   sqlschema.TableName("organizations"),
			ReferencedColumnName:  sqlschema.ColumnName("id"),
		}
	}

	sqls := migration.sqlschema.Operator().CreateTable(&sqlschema.Table{
		Name: "o11y_issues",
		Columns: []*sqlschema.Column{
			{Name: "id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "created_at", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "updated_at", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "org_id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "fingerprint", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "type", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "value", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "culprit", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "level", DataType: sqlschema.DataTypeText, Nullable: false, Default: "'error'"},
			{Name: "platform", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "status", DataType: sqlschema.DataTypeText, Nullable: false, Default: "'unresolved'"},
			{Name: "assignee", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "first_seen", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "last_seen", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "count", DataType: sqlschema.DataTypeBigInt, Nullable: false, Default: "0"},
			{Name: "resolved_at", DataType: sqlschema.DataTypeTimestamp, Nullable: true},
			{Name: "regressed", DataType: sqlschema.DataTypeBoolean, Nullable: false, Default: "false"},
			{Name: "environment", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "release", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "service_name", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "version", DataType: sqlschema.DataTypeBigInt, Nullable: false, Default: "0"},
			{Name: "sample_event", DataType: sqlschema.DataTypeText, Nullable: true},
		},
		PrimaryKeyConstraint:  &sqlschema.PrimaryKeyConstraint{ColumnNames: []sqlschema.ColumnName{"id"}},
		ForeignKeyConstraints: []*sqlschema.ForeignKeyConstraint{orgFK("org_id")},
	})

	// o11y_ingest_revocations: the per-org DSN-key rotation watermark. A DSN key is
	// "<version>:<hmac>"; raising min_version for ONE org revokes only that org's
	// below-min DSNs — isolated rotation without a global secret roll.
	sqls = append(sqls, migration.sqlschema.Operator().CreateTable(&sqlschema.Table{
		Name: "o11y_ingest_revocations",
		Columns: []*sqlschema.Column{
			{Name: "org_id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "min_version", DataType: sqlschema.DataTypeBigInt, Nullable: false, Default: "0"},
			{Name: "updated_at", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
		},
		PrimaryKeyConstraint: &sqlschema.PrimaryKeyConstraint{ColumnNames: []sqlschema.ColumnName{"org_id"}},
	})...)

	// The grouping key (ingest upserts ON CONFLICT here) and the list-ordering index.
	sqls = append(sqls,
		[]byte(`CREATE UNIQUE INDEX IF NOT EXISTS uq_o11y_issues_org_fingerprint ON o11y_issues (org_id, fingerprint)`),
		[]byte(`CREATE INDEX IF NOT EXISTS idx_o11y_issues_org_last_seen ON o11y_issues (org_id, last_seen)`),
	)

	for _, sql := range sqls {
		if _, err := tx.ExecContext(ctx, string(sql)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (migration *addErrorTracking) Down(context.Context, *bun.DB) error {
	return nil
}
