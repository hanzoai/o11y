package querybuildertypesv5

import "github.com/hanzoai/o11y/pkg/valuer"

type QueryType struct {
	valuer.String
}

var (
	QueryTypeUnknown       = QueryType{valuer.NewString("unknown")}
	QueryTypeBuilder       = QueryType{valuer.NewString("builder_query")}
	QueryTypeFormula       = QueryType{valuer.NewString("builder_formula")}
	QueryTypeSubQuery      = QueryType{valuer.NewString("builder_sub_query")}
	QueryTypeJoin          = QueryType{valuer.NewString("builder_join")}
	QueryTypeTraceOperator = QueryType{valuer.NewString("builder_trace_operator")}
	QueryTypeDatastoreSQL = QueryType{valuer.NewString("datastore_sql")}
	QueryTypePromQL        = QueryType{valuer.NewString("promql")}
)

// Enum returns the acceptable values for QueryType.
func (QueryType) Enum() []any {
	return []any{
		QueryTypeBuilder,
		QueryTypeFormula,
		// Not yet supported.
		// QueryTypeSubQuery,
		// QueryTypeJoin,
		QueryTypeTraceOperator,
		QueryTypeDatastoreSQL,
		QueryTypePromQL,
	}
}
