package v4

import (
	"strings"
	"testing"

	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
	"github.com/stretchr/testify/require"
)

// The v3/v4 builders are the LIVE path for the older query API — querier.go
// wires tracesV4.PrepareTracesQuery — and every caller value in an ORDER BY
// reached ClickHouse raw:
//
//   - the DIRECTION is a bare `Direction string` off the request body,
//     interpolated next to the column, so it needed neither a quote nor a
//     backtick to become an extra ordering term;
//   - the COLUMN is wrapped in backticks that it can simply close;
//   - the attribute key is wrapped in single quotes it can close, which is a
//     WHERE-clause injection rather than an ordering one.
//
// The allowlist that looks like it gates the column (`tagLookup`) is built from
// the caller's own GroupBy, so a name repeated in both fields validates itself.
//
// Assertions are on the emitted SQL and structural: the payload may appear as
// the inert name of a tag nobody has, but never in the executable skeleton.
func TestOrderByCannotEscapeItsQuoting(t *testing.T) {
	for _, c := range []struct {
		name           string
		mq             *v3.BuilderQuery
		mustNotExecute string
	}{
		{
			name: "direction is a whole extra ordering term",
			mq: &v3.BuilderQuery{
				QueryName: "A", StepInterval: 60, DataSource: v3.DataSourceTraces,
				AggregateOperator: v3.AggregateOperatorCount, Expression: "A",
				GroupBy: []v3.AttributeKey{{Key: "name", DataType: v3.AttributeKeyDataTypeString, Type: v3.AttributeKeyTypeTag}},
				OrderBy: []v3.OrderBy{{ColumnName: "name", Order: v3.Direction("asc,(select/**/count()/**/from/**/system.tables)")}},
			},
			mustNotExecute: "system.tables",
		},
		{
			name: "column closes its backticks",
			mq: &v3.BuilderQuery{
				QueryName: "A", StepInterval: 60, DataSource: v3.DataSourceTraces,
				AggregateOperator: v3.AggregateOperatorCount, Expression: "A",
				GroupBy: []v3.AttributeKey{{Key: "n`,(select/**/grouparray(name)/**/from/**/system.tables)/**/as/**/`x", DataType: v3.AttributeKeyDataTypeString, Type: v3.AttributeKeyTypeTag}},
				OrderBy: []v3.OrderBy{{ColumnName: "n`,(select/**/grouparray(name)/**/from/**/system.tables)/**/as/**/`x", Order: v3.DirectionAsc}},
			},
			mustNotExecute: "system.tables",
		},
		{
			name: "attribute key closes the literal in the WHERE clause",
			mq: &v3.BuilderQuery{
				QueryName: "A", StepInterval: 60, DataSource: v3.DataSourceTraces,
				AggregateOperator: v3.AggregateOperatorCount, Expression: "A",
				GroupBy: []v3.AttributeKey{{Key: "a'),(select/**/1))/**/or/**/('1'='1", DataType: v3.AttributeKeyDataTypeString, Type: v3.AttributeKeyTypeTag}},
			},
			mustNotExecute: "or/**/(",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			q, err := PrepareTracesQuery(1700000000000000000, 1700003600000000000, v3.PanelTypeTable, c.mq, v3.QBOptions{})
			require.NoError(t, err)
			skel, ok := skeleton(q)
			require.True(t, ok, "a quoted region was left open: %s", q)
			require.NotContains(t, skel, c.mustNotExecute, "payload became executable SQL.\nskeleton: %s\nfull: %s", skel, q)
		})
	}
}

// skeleton strips every quoted region, honouring backslash escapes, leaving the
// text the server executes as syntax. ok is false if a region is still open.
func skeleton(sql string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		if c != '`' && c != '\'' {
			b.WriteByte(c)
			continue
		}
		closed := false
		for i++; i < len(sql); i++ {
			if sql[i] == '\\' {
				i++
				continue
			}
			if sql[i] == c {
				closed = true
				break
			}
		}
		if !closed {
			return b.String(), false
		}
		b.WriteString("<quoted>")
	}
	return b.String(), true
}
