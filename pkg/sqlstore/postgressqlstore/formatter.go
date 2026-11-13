package postgressqlstore

import (
	"strings"

	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/uptrace/bun/schema"
)

// The control plane stores JSON documents in TEXT columns, as it does on sqlite,
// so every expression casts to jsonb before it reads inside one.
type formatter struct {
	bunf schema.Formatter
}

func newFormatter(dialect schema.Dialect) sqlstore.SQLFormatter {
	return &formatter{bunf: schema.NewFormatter(dialect)}
}

// pathOf turns a sqlite-style JSON path ($.a.b[0]) into a postgres text[] literal.
func pathOf(path string) string {
	path = strings.TrimPrefix(path, "$")
	var keys []string
	var key strings.Builder
	flush := func() {
		if key.Len() > 0 {
			keys = append(keys, key.String())
			key.Reset()
		}
	}
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '.', '[', ']':
			flush()
		case '"':
			// a quoted key keeps its dots
			if j := strings.IndexByte(path[i+1:], '"'); j >= 0 {
				key.WriteString(path[i+1 : i+1+j])
				i += j + 1
			}
		default:
			key.WriteByte(path[i])
		}
	}
	flush()
	return "{" + strings.Join(keys, ",") + "}"
}

func (f *formatter) col(column string) []byte {
	sql := f.bunf.AppendIdent(nil, column)
	return append(sql, "::jsonb"...)
}

func (f *formatter) at(column, path string) []byte {
	sql := f.col(column)
	sql = append(sql, " #> "...)
	sql = schema.Append(f.bunf, sql, pathOf(path))
	return append(sql, "::text[]"...)
}

func (f *formatter) JSONExtractString(column, path string) []byte {
	sql := []byte{'('}
	sql = append(sql, f.col(column)...)
	sql = append(sql, " #>> "...)
	sql = schema.Append(f.bunf, sql, pathOf(path))
	return append(sql, "::text[])"...)
}

func (f *formatter) JSONType(column, path string) []byte {
	sql := append([]byte("jsonb_typeof("), f.at(column, path)...)
	return append(sql, ')')
}

func (f *formatter) JSONIsArray(column, path string) []byte {
	sql := f.JSONType(column, path)
	sql = append(sql, " = "...)
	return schema.Append(f.bunf, sql, "array")
}

func (f *formatter) elements(fn, column, path, alias, col string) ([]byte, []byte) {
	sql := append([]byte(fn+"("), f.at(column, path)...)
	sql = append(sql, ") AS "...)
	sql = f.bunf.AppendIdent(sql, alias)
	sql = append(sql, "("+col+")"...)
	return sql, append([]byte(alias), ("." + col)...)
}

func (f *formatter) JSONArrayElements(column, path, alias string) ([]byte, []byte) {
	return f.elements("jsonb_array_elements", column, path, alias, "value")
}

func (f *formatter) JSONArrayOfStrings(column, path, alias string) ([]byte, []byte) {
	return f.elements("jsonb_array_elements_text", column, path, alias, "value")
}

func (f *formatter) JSONKeys(column, path, alias string) ([]byte, []byte) {
	return f.elements("jsonb_object_keys", column, path, alias, "key")
}

func (f *formatter) JSONArrayAgg(expression string) []byte {
	return []byte("jsonb_agg(" + expression + ")")
}

func (f *formatter) JSONArrayLiteral(values ...string) []byte {
	sql := []byte("jsonb_build_array(")
	for idx, value := range values {
		if idx > 0 {
			sql = append(sql, ", "...)
		}
		sql = schema.Append(f.bunf, sql, value)
		sql = append(sql, "::text"...)
	}
	return append(sql, ')')
}

func (f *formatter) TextToJsonColumn(column string) []byte {
	return f.col(column)
}

func (f *formatter) LowerExpression(expression string) []byte {
	return []byte("lower(" + expression + ")")
}

func (f *formatter) EscapeLikePattern(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
