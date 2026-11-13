package postgressqlschema

import (
	"strings"

	"github.com/hanzoai/o11y/pkg/sqlschema"
)

type Formatter struct {
	sqlschema.Formatter
}

func (formatter Formatter) SQLDataTypeOf(dataType sqlschema.DataType) string {
	return strings.ToUpper(dataType.String())
}

// DataTypeOf reads information_schema.columns.data_type as well as DDL spellings.
func (formatter Formatter) DataTypeOf(dataType string) sqlschema.DataType {
	switch strings.ToUpper(dataType) {
	case "TIMESTAMP WITHOUT TIME ZONE":
		return sqlschema.DataTypeTimestamp
	case "DOUBLE PRECISION", "REAL":
		return sqlschema.DataTypeNumeric
	case "BYTEA":
		return sqlschema.DataTypeBytea
	}

	return formatter.Formatter.DataTypeOf(dataType)
}
