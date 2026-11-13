package postgressqlstore

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/uptrace/bun"
)

const (
	Identity string = "id"
	Text     string = "TEXT"
)

const (
	Org                string = "org"
	User               string = "user"
	UserNoCascade      string = "user_no_cascade"
	FactorPassword     string = "factor_password"
	CloudIntegration   string = "cloud_integration"
	AgentConfigVersion string = "agent_config_version"
)

const (
	OrgReference                string = `("org_id") REFERENCES "organizations" ("id")`
	UserReference               string = `("user_id") REFERENCES "users" ("id") ON DELETE CASCADE ON UPDATE CASCADE`
	UserNoCascadeReference      string = `("user_id") REFERENCES "users" ("id")`
	FactorPasswordReference     string = `("password_id") REFERENCES "factor_password" ("id")`
	CloudIntegrationReference   string = `("cloud_integration_id") REFERENCES "cloud_integration" ("id") ON DELETE CASCADE`
	AgentConfigVersionReference string = `("version_id") REFERENCES "agent_config_version" ("id")`
)

const OrgField string = "org_id"

var references = map[string]string{
	Org:                OrgReference,
	User:               UserReference,
	UserNoCascade:      UserNoCascadeReference,
	FactorPassword:     FactorPasswordReference,
	CloudIntegration:   CloudIntegrationReference,
	AgentConfigVersion: AgentConfigVersionReference,
}

type dialect struct{}

// current is the schema the connection's search_path resolves first, which is the
// store's own: a host binds one schema per store.
const current = "table_schema = current_schema()"

// GetColumnType answers the type the way sqlite spells it (INTEGER, TEXT, ...), the
// vocabulary the callers compare against.
func (dialect *dialect) GetColumnType(ctx context.Context, bun bun.IDB, table string, column string) (string, error) {
	var columnType string

	err := bun.
		NewSelect().
		ColumnExpr("data_type").
		TableExpr("information_schema.columns").
		Where(current).
		Where("table_name = ?", table).
		Where("column_name = ?", column).
		Scan(ctx, &columnType)
	if err != nil {
		return "", err
	}

	switch strings.ToLower(columnType) {
	case "timestamp without time zone":
		return "TIMESTAMP", nil
	case "timestamp with time zone":
		return "TIMESTAMPTZ", nil
	default:
		return strings.ToUpper(columnType), nil
	}
}

func (dialect *dialect) IntToTimestamp(ctx context.Context, bun bun.IDB, table string, column string) error {
	columnType, err := dialect.GetColumnType(ctx, bun, table, column)
	if err != nil {
		return err
	}

	if columnType != "INTEGER" && columnType != "BIGINT" {
		return nil
	}

	_, err = bun.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %q ALTER COLUMN %q DROP DEFAULT, ALTER COLUMN %q TYPE TIMESTAMP USING to_timestamp(%q) AT TIME ZONE 'UTC'`, table, column, column, column))
	return err
}

func (dialect *dialect) IntToBoolean(ctx context.Context, bun bun.IDB, table string, column string) error {
	columnExists, err := dialect.ColumnExists(ctx, bun, table, column)
	if err != nil {
		return err
	}
	if !columnExists {
		return nil
	}

	columnType, err := dialect.GetColumnType(ctx, bun, table, column)
	if err != nil {
		return err
	}

	if columnType != "INTEGER" && columnType != "BIGINT" {
		return nil
	}

	_, err = bun.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %q ALTER COLUMN %q DROP DEFAULT, ALTER COLUMN %q TYPE BOOLEAN USING (%q = 1)`, table, column, column, column))
	return err
}

func (dialect *dialect) ColumnExists(ctx context.Context, bun bun.IDB, table string, column string) (bool, error) {
	var count int
	err := bun.NewSelect().
		ColumnExpr("COUNT(*)").
		TableExpr("information_schema.columns").
		Where(current).
		Where("table_name = ?", table).
		Where("column_name = ?", column).
		Scan(ctx, &count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (dialect *dialect) AddColumn(ctx context.Context, bun bun.IDB, table string, column string, columnExpr string) error {
	exists, err := dialect.ColumnExists(ctx, bun, table, column)
	if err != nil {
		return err
	}
	if !exists {
		_, err = bun.
			NewAddColumn().
			Table(table).
			ColumnExpr(column + " " + columnExpr).
			Exec(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (dialect *dialect) RenameColumn(ctx context.Context, bun bun.IDB, table string, oldColumnName string, newColumnName string) (bool, error) {
	oldColumnExists, err := dialect.ColumnExists(ctx, bun, table, oldColumnName)
	if err != nil {
		return false, err
	}

	newColumnExists, err := dialect.ColumnExists(ctx, bun, table, newColumnName)
	if err != nil {
		return false, err
	}

	if newColumnExists {
		return true, nil
	}

	if !oldColumnExists {
		return false, errors.Newf(errors.TypeInvalidInput, errors.CodeInvalidInput, "old column: %s doesn't exist", oldColumnName)
	}

	if _, err = bun.ExecContext(ctx, "ALTER TABLE "+table+" RENAME COLUMN "+oldColumnName+" TO "+newColumnName); err != nil {
		return false, err
	}
	return true, nil
}

func (dialect *dialect) DropColumn(ctx context.Context, bun bun.IDB, table string, column string) error {
	exists, err := dialect.ColumnExists(ctx, bun, table, column)
	if err != nil {
		return err
	}
	if exists {
		_, err = bun.
			NewDropColumn().
			Table(table).
			Column(column).
			Exec(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (dialect *dialect) TableExists(ctx context.Context, bun bun.IDB, table interface{}) (bool, error) {
	count := 0
	err := bun.
		NewSelect().
		ColumnExpr("count(*)").
		TableExpr("information_schema.tables").
		Where(current).
		Where("table_name = ?", bun.Dialect().Tables().Get(reflect.TypeOf(table)).Name).
		Scan(ctx, &count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (dialect *dialect) RenameTableAndModifyModel(ctx context.Context, bun bun.IDB, oldModel interface{}, newModel interface{}, refs []string, cb func(context.Context) error) error {
	if len(refs) == 0 {
		return errors.Newf(errors.TypeInvalidInput, errors.CodeInvalidInput, "cannot run migration without reference")
	}
	exists, err := dialect.TableExists(ctx, bun, newModel)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	var fkReferences []string
	for _, reference := range refs {
		if fk, ok := references[reference]; ok && !slices.Contains(fkReferences, fk) {
			fkReferences = append(fkReferences, fk)
		}
	}

	createTable := bun.
		NewCreateTable().
		IfNotExists().
		Model(newModel)

	for _, fk := range fkReferences {
		createTable = createTable.ForeignKey(fk)
	}

	if _, err = createTable.Exec(ctx); err != nil {
		return err
	}

	if err = cb(ctx); err != nil {
		return err
	}

	_, err = bun.
		NewDropTable().
		IfExists().
		Model(oldModel).
		Exec(ctx)
	return err
}

func (dialect *dialect) AddNotNullDefaultToColumn(ctx context.Context, bun bun.IDB, table string, column, columnType, defaultValue string) error {
	if _, err := bun.NewAddColumn().Table(table).ColumnExpr(fmt.Sprintf("%s_new %s NOT NULL DEFAULT %s ", column, columnType, defaultValue)).Exec(ctx); err != nil {
		return err
	}

	if _, err := bun.NewUpdate().Table(table).Set(fmt.Sprintf("%s_new = %s", column, column)).Where("1=1").Exec(ctx); err != nil {
		return err
	}

	if _, err := bun.NewDropColumn().Table(table).ColumnExpr(column).Exec(ctx); err != nil {
		return err
	}

	_, err := bun.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s_new TO %s", table, column, column))
	return err
}

func (dialect *dialect) rebuild(ctx context.Context, bun bun.IDB, oldModel interface{}, newModel interface{}, reference string, cb func(context.Context) error) error {
	oldTableName := bun.Dialect().Tables().Get(reflect.TypeOf(oldModel)).Name
	newTableName := bun.Dialect().Tables().Get(reflect.TypeOf(newModel)).Name

	fkReference := ""
	switch reference {
	case Org:
		fkReference = OrgReference
	case User:
		fkReference = UserReference
	}

	if _, err := bun.
		NewCreateTable().
		IfNotExists().
		Model(newModel).
		ForeignKey(fkReference).
		Exec(ctx); err != nil {
		return err
	}

	if err := cb(ctx); err != nil {
		return err
	}

	if _, err := bun.
		NewDropTable().
		IfExists().
		Model(oldModel).
		Exec(ctx); err != nil {
		return err
	}

	_, err := bun.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", newTableName, oldTableName))
	return err
}

func (dialect *dialect) UpdatePrimaryKey(ctx context.Context, bun bun.IDB, oldModel interface{}, newModel interface{}, reference string, cb func(context.Context) error) error {
	if reference == "" {
		return errors.Newf(errors.TypeInvalidInput, errors.CodeInvalidInput, "cannot run migration without reference")
	}
	oldTableName := bun.Dialect().Tables().Get(reflect.TypeOf(oldModel)).Name

	columnType, err := dialect.GetColumnType(ctx, bun, oldTableName, Identity)
	if err != nil {
		return err
	}
	if columnType == Text {
		return nil
	}

	return dialect.rebuild(ctx, bun, oldModel, newModel, reference, cb)
}

func (dialect *dialect) AddPrimaryKey(ctx context.Context, bun bun.IDB, oldModel interface{}, newModel interface{}, reference string, cb func(context.Context) error) error {
	if reference == "" {
		return errors.Newf(errors.TypeInvalidInput, errors.CodeInvalidInput, "cannot run migration without reference")
	}
	oldTableName := bun.Dialect().Tables().Get(reflect.TypeOf(oldModel)).Name

	identityExists, err := dialect.ColumnExists(ctx, bun, oldTableName, Identity)
	if err != nil {
		return err
	}
	if identityExists {
		return nil
	}

	return dialect.rebuild(ctx, bun, oldModel, newModel, reference, cb)
}

// DropColumnWithForeignKeyConstraint: postgres drops the foreign keys on a column
// with the column, so there is no table to rebuild.
func (dialect *dialect) DropColumnWithForeignKeyConstraint(ctx context.Context, bunIDB bun.IDB, model interface{}, column string) error {
	existingTable := bunIDB.Dialect().Tables().Get(reflect.TypeOf(model))
	columnExists, err := dialect.ColumnExists(ctx, bunIDB, existingTable.Name, column)
	if err != nil {
		return err
	}
	if !columnExists {
		return nil
	}

	_, err = bunIDB.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %q DROP COLUMN %q`, existingTable.Name, column))
	return err
}

// ToggleForeignKeyConstraint is a no-op: postgres has no session switch, and a
// migration that needs a foreign key deferred declares it deferrable.
func (dialect *dialect) ToggleForeignKeyConstraint(ctx context.Context, bun *bun.DB, enable bool) error {
	return nil
}
