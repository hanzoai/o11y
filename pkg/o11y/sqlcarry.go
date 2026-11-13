package o11y

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// carrySkip are the tables a carry leaves alone. The two migration tables record what
// each engine ran. The rest is the authorization store: sqlite keeps the first layout
// of it, postgres the one migration 054 rewrites it to, and the in-process authorizer
// rebuilds the grants from the role assignments, so nothing is lost by leaving it.
var carrySkip = map[string]bool{
	"migration": true, "migration_lock": true,
	"tuple": true, "authorization_model": true, "changelog": true, "assertion": true, "store": true,
}

// Carried is what a carry copied: rows per table, as counted on the target.
type Carried map[string]int64

// CarrySQLite copies every control-plane row from a sqlite database to a postgres one,
// in one transaction on the target, then checks each table's row count against the
// source before it commits. Both databases must already be at the current version
// ([MigrateSQL]): a row is copied under the column names both sides have. The target's
// tables are emptied first, so rows the migrations seeded there do not collide with
// the ones carried.
func CarrySQLite(ctx context.Context, from, to *sql.DB) (Carried, error) {
	source, err := tablesOf(ctx, from, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, fmt.Errorf("list the source tables: %w", err)
	}
	target, err := tablesOf(ctx, to, `SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'`)
	if err != nil {
		return nil, fmt.Errorf("list the target tables: %w", err)
	}

	var tables []string
	for _, name := range source {
		if !carrySkip[name] && contains(target, name) {
			tables = append(tables, name)
		}
	}
	sort.Strings(tables)

	// A single connection holds the transaction, the deferred constraints and the
	// count that follows them.
	tx, err := to.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Foreign keys are checked at COMMIT: no copy order satisfies a cycle or a self
	// reference, and the source was consistent.
	if err := deferForeignKeys(ctx, tx); err != nil {
		return nil, err
	}
	if len(tables) > 0 {
		quoted := make([]string, len(tables))
		for i, name := range tables {
			quoted[i] = quote(name)
		}
		if _, err := tx.ExecContext(ctx, "TRUNCATE "+strings.Join(quoted, ", ")); err != nil {
			return nil, fmt.Errorf("empty the target: %w", err)
		}
	}

	carried := Carried{}
	for _, table := range tables {
		n, err := carryTable(ctx, from, tx, table)
		if err != nil {
			return nil, fmt.Errorf("carry %s: %w", table, err)
		}
		carried[table] = n
	}

	// Every identity column continues after the largest id carried.
	if err := resyncIdentities(ctx, tx, tables); err != nil {
		return nil, err
	}

	for _, table := range tables {
		var want, got int64
		if err := from.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quote(table)).Scan(&want); err != nil {
			return nil, err
		}
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quote(table)).Scan(&got); err != nil {
			return nil, err
		}
		if want != got {
			return nil, fmt.Errorf("carry %s: the source holds %d rows and the target %d", table, want, got)
		}
		carried[table] = got
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit the carry: %w", err)
	}
	return carried, nil
}

func quote(ident string) string { return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"` }

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func tablesOf(ctx context.Context, db *sql.DB, query string) ([]string, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// deferForeignKeys makes every foreign key of the schema deferrable (transactional DDL:
// a rollback leaves them as they were) and holds the checks until COMMIT.
func deferForeignKeys(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT t.relname, c.conname FROM pg_constraint c JOIN pg_class t ON t.oid = c.conrelid
		WHERE c.contype = 'f' AND c.connamespace = current_schema()::regnamespace`)
	if err != nil {
		return err
	}
	type fk struct{ table, name string }
	var fks []fk
	for rows.Next() {
		var f fk
		if err := rows.Scan(&f.table, &f.name); err != nil {
			rows.Close()
			return err
		}
		fks = append(fks, f)
	}
	rows.Close()
	for _, f := range fks {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE "+quote(f.table)+" ALTER CONSTRAINT "+quote(f.name)+" DEFERRABLE INITIALLY IMMEDIATE"); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "SET CONSTRAINTS ALL DEFERRED")
	return err
}

type column struct {
	name     string
	dataType string
}

// carryTable copies the columns the two sides share, converting each value to the type
// of the target column.
func carryTable(ctx context.Context, from *sql.DB, to *sql.Tx, table string) (int64, error) {
	cols, err := targetColumns(ctx, to, table)
	if err != nil {
		return 0, err
	}
	have, err := tablesOf(ctx, from, `SELECT name FROM pragma_table_info('`+strings.ReplaceAll(table, "'", "''")+`')`)
	if err != nil {
		return 0, err
	}
	var shared []column
	for _, c := range cols {
		if contains(have, c.name) {
			shared = append(shared, c)
		}
	}
	if len(shared) == 0 {
		return 0, nil
	}

	names := make([]string, len(shared))
	marks := make([]string, len(shared))
	for i, c := range shared {
		names[i] = quote(c.name)
		marks[i] = "$" + strconv.Itoa(i+1)
	}
	rows, err := from.QueryContext(ctx, "SELECT "+strings.Join(names, ", ")+" FROM "+quote(table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	insert, err := to.PrepareContext(ctx, "INSERT INTO "+quote(table)+" ("+strings.Join(names, ", ")+") VALUES ("+strings.Join(marks, ", ")+")")
	if err != nil {
		return 0, err
	}
	defer insert.Close()

	var n int64
	for rows.Next() {
		raw := make([]any, len(shared))
		ptrs := make([]any, len(shared))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return n, err
		}
		args := make([]any, len(shared))
		for i, c := range shared {
			if args[i], err = convert(raw[i], c.dataType); err != nil {
				return n, fmt.Errorf("row %d, column %s: %w", n+1, c.name, err)
			}
		}
		if _, err := insert.ExecContext(ctx, args...); err != nil {
			return n, fmt.Errorf("row %d: %w", n+1, err)
		}
		n++
	}
	return n, rows.Err()
}

func targetColumns(ctx context.Context, tx *sql.Tx, table string) ([]column, error) {
	rows, err := tx.QueryContext(ctx, `SELECT column_name, data_type FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = $1 ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []column
	for rows.Next() {
		var c column
		if err := rows.Scan(&c.name, &c.dataType); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

var timeLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00", "2006-01-02T15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02",
}

// convert reads a sqlite value the way the target column wants it. sqlite keeps what it
// was given, so a boolean may be 0/1 or 'true', a timestamp text or an integer.
func convert(v any, dataType string) (any, error) {
	if v == nil {
		return nil, nil
	}
	if b, ok := v.([]byte); ok && dataType != "bytea" {
		v = string(b)
	}

	switch dataType {
	case "boolean":
		switch x := v.(type) {
		case bool:
			return x, nil
		case int64:
			return x != 0, nil
		case float64:
			return x != 0, nil
		case string:
			b, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(x)))
			return b, err
		}
	case "smallint", "integer", "bigint":
		switch x := v.(type) {
		case int64:
			return x, nil
		case bool:
			if x {
				return int64(1), nil
			}
			return int64(0), nil
		case float64:
			return int64(x), nil
		case string:
			return strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		}
	case "real", "double precision", "numeric":
		switch x := v.(type) {
		case float64:
			return x, nil
		case int64:
			return float64(x), nil
		case string:
			return strconv.ParseFloat(strings.TrimSpace(x), 64)
		}
	case "timestamp without time zone", "timestamp with time zone":
		switch x := v.(type) {
		case time.Time:
			return x, nil
		case int64:
			return time.Unix(x, 0).UTC(), nil
		case string:
			for _, layout := range timeLayouts {
				if t, err := time.Parse(layout, strings.TrimSpace(x)); err == nil {
					return t, nil
				}
			}
			return nil, fmt.Errorf("unreadable timestamp %q", x)
		}
	case "bytea":
		switch x := v.(type) {
		case []byte:
			return x, nil
		case string:
			return []byte(x), nil
		}
	default: // text, json, jsonb, character varying, uuid, ...
		switch x := v.(type) {
		case string:
			if strings.ContainsRune(x, 0) {
				return nil, fmt.Errorf("a text value holds a NUL, which postgres cannot store")
			}
			return x, nil
		case int64:
			return strconv.FormatInt(x, 10), nil
		case float64:
			return strconv.FormatFloat(x, 'g', -1, 64), nil
		case bool:
			return strconv.FormatBool(x), nil
		case time.Time:
			return x.Format(time.RFC3339Nano), nil
		}
	}
	return nil, fmt.Errorf("cannot read %T as %s", v, dataType)
}

// resyncIdentities moves each identity/serial column past the largest id carried.
func resyncIdentities(ctx context.Context, tx *sql.Tx, tables []string) error {
	for _, table := range tables {
		rows, err := tx.QueryContext(ctx, `SELECT column_name FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = $1
			AND (is_identity = 'YES' OR column_default LIKE 'nextval(%')`, table)
		if err != nil {
			return err
		}
		var cols []string
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				rows.Close()
				return err
			}
			cols = append(cols, c)
		}
		rows.Close()
		for _, c := range cols {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(
				`SELECT setval(pg_get_serial_sequence(%s, %s), COALESCE(MAX(%s), 1), MAX(%s) IS NOT NULL) FROM %s`,
				literal(quote(table)), literal(c), quote(c), quote(c), quote(table))); err != nil {
				return fmt.Errorf("resync %s.%s: %w", table, c, err)
			}
		}
	}
	return nil
}

func literal(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
