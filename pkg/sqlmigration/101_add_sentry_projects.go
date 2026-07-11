package sqlmigration

import (
	"context"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// addSentryProjects creates o11y_sentry_projects — the DSN-bearing unit of the Hanzo
// Sentry product face. It is a thin relational row: the DSN is NEVER stored (it is
// derived from the platform ingest secret + id + key_version), so rotation is a
// single-row bump. Raw events live on the columnar datastore plane and grouped-issue
// lifecycle stays in o11y_issues — this table only names projects and carries the DSN
// key watermark. The unique index on (org_id, slug) is the per-org name key; the index
// on (org_id) serves the org-scoped project list.
type addSentryProjects struct {
	sqlschema sqlschema.SQLSchema
	sqlstore  sqlstore.SQLStore
}

func NewAddSentryProjectsFactory(sqlstore sqlstore.SQLStore, sqlschema sqlschema.SQLSchema) factory.ProviderFactory[SQLMigration, Config] {
	return factory.NewProviderFactory(factory.MustNewName("add_sentry_projects"), func(_ context.Context, _ factory.ProviderSettings, _ Config) (SQLMigration, error) {
		return &addSentryProjects{sqlschema: sqlschema, sqlstore: sqlstore}, nil
	})
}

func (migration *addSentryProjects) Register(migrations *migrate.Migrations) error {
	return migrations.Register(migration.Up, migration.Down)
}

func (migration *addSentryProjects) Up(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	orgFK := &sqlschema.ForeignKeyConstraint{
		ReferencingColumnName: sqlschema.ColumnName("org_id"),
		ReferencedTableName:   sqlschema.TableName("organizations"),
		ReferencedColumnName:  sqlschema.ColumnName("id"),
	}

	sqls := migration.sqlschema.Operator().CreateTable(&sqlschema.Table{
		Name: "o11y_sentry_projects",
		Columns: []*sqlschema.Column{
			{Name: "id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "created_at", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "updated_at", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "org_id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "name", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "slug", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "platform", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "status", DataType: sqlschema.DataTypeText, Nullable: false, Default: "'active'"},
			{Name: "key_version", DataType: sqlschema.DataTypeBigInt, Nullable: false, Default: "1"},
		},
		PrimaryKeyConstraint:  &sqlschema.PrimaryKeyConstraint{ColumnNames: []sqlschema.ColumnName{"id"}},
		ForeignKeyConstraints: []*sqlschema.ForeignKeyConstraint{orgFK},
	})

	sqls = append(sqls,
		[]byte(`CREATE UNIQUE INDEX IF NOT EXISTS uq_o11y_sentry_projects_org_slug ON o11y_sentry_projects (org_id, slug)`),
		[]byte(`CREATE INDEX IF NOT EXISTS idx_o11y_sentry_projects_org ON o11y_sentry_projects (org_id)`),
	)

	for _, sql := range sqls {
		if _, err := tx.ExecContext(ctx, string(sql)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (migration *addSentryProjects) Down(context.Context, *bun.DB) error {
	return nil
}
