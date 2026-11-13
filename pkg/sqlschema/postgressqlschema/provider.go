// Package postgressqlschema inspects the PostgreSQL schema the sqlstore is bound to
// and renders DDL for it.
package postgressqlschema

import (
	"context"
	"regexp"
	"strings"

	"github.com/uptrace/bun"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
)

type provider struct {
	settings factory.ScopedProviderSettings
	fmter    sqlschema.SQLFormatter
	sqlstore sqlstore.SQLStore
	operator sqlschema.SQLOperator
}

func NewFactory(sqlstore sqlstore.SQLStore) factory.ProviderFactory[sqlschema.SQLSchema, sqlschema.Config] {
	return factory.NewProviderFactory(factory.MustNewName("postgres"), func(ctx context.Context, providerSettings factory.ProviderSettings, config sqlschema.Config) (sqlschema.SQLSchema, error) {
		return New(ctx, providerSettings, config, sqlstore)
	})
}

func New(ctx context.Context, providerSettings factory.ProviderSettings, config sqlschema.Config, sqlstore sqlstore.SQLStore) (sqlschema.SQLSchema, error) {
	settings := factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/sqlschema/postgressqlschema")
	fmter := Formatter{sqlschema.NewFormatter(sqlstore.BunDB().Dialect())}

	return &provider{
		fmter:    fmter,
		settings: settings,
		sqlstore: sqlstore,
		operator: sqlschema.NewOperator(fmter, sqlschema.OperatorSupport{
			SCreateAndDropConstraint:                        true,
			SAlterTableAddAndDropColumnIfNotExistsAndExists: true,
			SAlterTableAlterColumnSetAndDrop:                true,
		}),
	}, nil
}

func (provider *provider) Formatter() sqlschema.SQLFormatter { return provider.fmter }

func (provider *provider) Operator() sqlschema.SQLOperator { return provider.operator }

// castSuffix is the type postgres appends to a stored default: 'x'::text, 0::bigint.
var castSuffix = regexp.MustCompile(`::[a-zA-Z_][a-zA-Z0-9_ ]*(\[\])?$`)

func (provider *provider) GetTable(ctx context.Context, tableName sqlschema.TableName) (*sqlschema.Table, []*sqlschema.UniqueConstraint, error) {
	db := provider.sqlstore.BunDB()

	var cols []struct {
		Name     string  `bun:"column_name"`
		DataType string  `bun:"data_type"`
		Nullable string  `bun:"is_nullable"`
		Default  *string `bun:"column_default"`
	}
	if err := db.NewRaw(`SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ?
		ORDER BY ordinal_position`, string(tableName)).Scan(ctx, &cols); err != nil {
		return nil, nil, provider.sqlstore.WrapNotFoundErrf(err, errors.CodeNotFound, "table (%s) not found", tableName)
	}
	if len(cols) == 0 {
		return nil, nil, errors.Newf(errors.TypeNotFound, errors.CodeNotFound, "table (%s) not found", tableName)
	}

	table := &sqlschema.Table{Name: tableName}
	for _, c := range cols {
		column := &sqlschema.Column{
			Name:     sqlschema.ColumnName(c.Name),
			DataType: provider.fmter.DataTypeOf(c.DataType),
			Nullable: c.Nullable == "YES",
		}
		if c.Default != nil {
			column.Default = castSuffix.ReplaceAllString(*c.Default, "")
		}
		table.Columns = append(table.Columns, column)
	}

	var constraints []struct {
		Name       string `bun:"name"`
		Type       string `bun:"contype"`
		Columns    string `bun:"columns"`
		RefTable   string `bun:"ref_table"`
		RefColumns string `bun:"ref_columns"`
	}
	if err := db.NewRaw(`SELECT c.conname AS name, c.contype::text AS contype,
			COALESCE((SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord) JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k.attnum), '') AS columns,
			COALESCE(rt.relname, '') AS ref_table,
			COALESCE((SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(c.confkey) WITH ORDINALITY AS k(attnum, ord) JOIN pg_attribute a ON a.attrelid = c.confrelid AND a.attnum = k.attnum), '') AS ref_columns
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		LEFT JOIN pg_class rt ON rt.oid = c.confrelid
		WHERE t.relnamespace = current_schema()::regnamespace AND t.relname = ? AND c.contype IN ('p', 'f', 'u')
		ORDER BY c.oid`, string(tableName)).Scan(ctx, &constraints); err != nil {
		return nil, nil, err
	}

	uniques := []*sqlschema.UniqueConstraint{}
	for _, c := range constraints {
		names := columnNames(c.Columns)
		switch c.Type {
		case "p":
			pk := &sqlschema.PrimaryKeyConstraint{ColumnNames: names}
			if pk.Name(tableName) != c.Name {
				pk = pk.Named(c.Name).(*sqlschema.PrimaryKeyConstraint)
			}
			table.PrimaryKeyConstraint = pk
		case "f":
			fk := &sqlschema.ForeignKeyConstraint{
				ReferencingColumnName: names[0],
				ReferencedTableName:   sqlschema.TableName(c.RefTable),
				ReferencedColumnName:  columnNames(c.RefColumns)[0],
			}
			if fk.Name(tableName) != c.Name {
				fk = fk.Named(c.Name).(*sqlschema.ForeignKeyConstraint)
			}
			table.ForeignKeyConstraints = append(table.ForeignKeyConstraints, fk)
		case "u":
			uc := &sqlschema.UniqueConstraint{ColumnNames: names}
			if uc.Name(tableName) != c.Name {
				uc = uc.Named(c.Name).(*sqlschema.UniqueConstraint)
			}
			uniques = append(uniques, uc)
		}
	}

	return table, uniques, nil
}

func columnNames(joined string) []sqlschema.ColumnName {
	out := []sqlschema.ColumnName{}
	for _, name := range strings.Split(joined, ",") {
		if name != "" {
			out = append(out, sqlschema.ColumnName(name))
		}
	}
	return out
}

// GetIndices returns the unique indices of the table that no constraint owns.
func (provider *provider) GetIndices(ctx context.Context, tableName sqlschema.TableName) ([]sqlschema.Index, error) {
	var rows []struct {
		Name    string  `bun:"name"`
		Columns string  `bun:"columns"`
		Where   *string `bun:"pred"`
	}
	if err := provider.sqlstore.BunDB().NewRaw(`SELECT i.relname AS name,
			COALESCE((SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(x.indkey::int2[]) WITH ORDINALITY AS k(attnum, ord) JOIN pg_attribute a ON a.attrelid = x.indrelid AND a.attnum = k.attnum), '') AS columns,
			pg_get_expr(x.indpred, x.indrelid) AS pred
		FROM pg_index x
		JOIN pg_class t ON t.oid = x.indrelid
		JOIN pg_class i ON i.oid = x.indexrelid
		WHERE t.relnamespace = current_schema()::regnamespace AND t.relname = ?
			AND x.indisunique AND NOT x.indisprimary
			AND NOT EXISTS (SELECT 1 FROM pg_constraint c WHERE c.conindid = x.indexrelid)
		ORDER BY i.relname`, string(tableName)).Scan(ctx, &rows); err != nil {
		return nil, provider.sqlstore.WrapNotFoundErrf(err, errors.CodeNotFound, "no indices for table (%s) found", tableName)
	}

	indices := []sqlschema.Index{}
	for _, row := range rows {
		cols := columnNames(row.Columns)
		if row.Where != nil && *row.Where != "" {
			index := &sqlschema.PartialUniqueIndex{TableName: tableName, ColumnNames: cols, Where: *row.Where}
			if index.Name() == row.Name {
				indices = append(indices, index)
			} else {
				indices = append(indices, index.Named(row.Name))
			}
			continue
		}
		index := &sqlschema.UniqueIndex{TableName: tableName, ColumnNames: cols}
		if index.Name() == row.Name {
			indices = append(indices, index)
		} else {
			indices = append(indices, index.Named(row.Name))
		}
	}

	return indices, nil
}

// ToggleFKEnforcement is a no-op: postgres enforces foreign keys per constraint, not
// per session.
func (provider *provider) ToggleFKEnforcement(ctx context.Context, db bun.IDB, on bool) error {
	return nil
}
