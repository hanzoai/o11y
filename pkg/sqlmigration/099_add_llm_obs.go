package sqlmigration

import (
	"context"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// addLLMObs creates the two net-new tables backing the native LLM-observability
// API: llm_scores (eval scores + human feedback) and llm_annotations (review
// notes/queues). Observations/traces/sessions/users are span views and need no
// tables.
type addLLMObs struct {
	sqlschema sqlschema.SQLSchema
	sqlstore  sqlstore.SQLStore
}

func NewAddLLMObsFactory(sqlstore sqlstore.SQLStore, sqlschema sqlschema.SQLSchema) factory.ProviderFactory[SQLMigration, Config] {
	return factory.NewProviderFactory(factory.MustNewName("add_llm_obs"), func(_ context.Context, _ factory.ProviderSettings, _ Config) (SQLMigration, error) {
		return &addLLMObs{sqlschema: sqlschema, sqlstore: sqlstore}, nil
	})
}

func (migration *addLLMObs) Register(migrations *migrate.Migrations) error {
	return migrations.Register(migration.Up, migration.Down)
}

func (migration *addLLMObs) Up(ctx context.Context, db *bun.DB) error {
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

	sqls := [][]byte{}

	sqls = append(sqls, migration.sqlschema.Operator().CreateTable(&sqlschema.Table{
		Name: "llm_scores",
		Columns: []*sqlschema.Column{
			{Name: "id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "created_at", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "updated_at", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "org_id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "trace_id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "observation_id", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "name", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "value", DataType: sqlschema.DataTypeNumeric, Nullable: false, Default: "0"},
			{Name: "string_value", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "data_type", DataType: sqlschema.DataTypeText, Nullable: false, Default: "'NUMERIC'"},
			{Name: "comment", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "source", DataType: sqlschema.DataTypeText, Nullable: false, Default: "'API'"},
			{Name: "timestamp", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "created_by", DataType: sqlschema.DataTypeText, Nullable: true},
		},
		PrimaryKeyConstraint:  &sqlschema.PrimaryKeyConstraint{ColumnNames: []sqlschema.ColumnName{"id"}},
		ForeignKeyConstraints: []*sqlschema.ForeignKeyConstraint{orgFK("org_id")},
	})...)

	sqls = append(sqls, migration.sqlschema.Operator().CreateTable(&sqlschema.Table{
		Name: "llm_annotations",
		Columns: []*sqlschema.Column{
			{Name: "id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "created_at", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "updated_at", DataType: sqlschema.DataTypeTimestamp, Nullable: false},
			{Name: "org_id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "trace_id", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "observation_id", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "queue", DataType: sqlschema.DataTypeText, Nullable: true},
			{Name: "content", DataType: sqlschema.DataTypeText, Nullable: false},
			{Name: "status", DataType: sqlschema.DataTypeText, Nullable: false, Default: "'PENDING'"},
			{Name: "author", DataType: sqlschema.DataTypeText, Nullable: true},
		},
		PrimaryKeyConstraint:  &sqlschema.PrimaryKeyConstraint{ColumnNames: []sqlschema.ColumnName{"id"}},
		ForeignKeyConstraints: []*sqlschema.ForeignKeyConstraint{orgFK("org_id")},
	})...)

	for _, sql := range sqls {
		if _, err := tx.ExecContext(ctx, string(sql)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (migration *addLLMObs) Down(context.Context, *bun.DB) error {
	return nil
}
