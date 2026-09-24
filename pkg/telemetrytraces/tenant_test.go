package telemetrytraces

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/flagger/flaggertest"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/querybuilder"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes/telemetrytypestest"
	"github.com/stretchr/testify/require"
)

// testTenant is the org the statement tests read as; every event.span read
// they build binds it as `org = ?`.
const testTenant = "acme"

func tenantCtx() context.Context {
	return authtypes.NewContextWithTenant(context.Background(), testTenant)
}

// Every request type reads event.span for exactly one org, and none reads it
// for no org. The trace request type reads spans twice (matching traces and
// their root spans), and both reads are scoped.
func TestSpanReadsAreScopedToTheTenant(t *testing.T) {
	fl := flaggertest.New(t)
	store := telemetrytypestest.NewMockMetadataStore()
	store.KeysMap = buildCompleteFieldKeyMap(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	fm := NewFieldMapper()
	cb := NewConditionBuilder(fm)
	rewriter := querybuilder.NewAggExprRewriter(instrumentationtest.New().ToProviderSettings(), nil, fm, cb, nil, fl)
	b := NewTraceQueryStatementBuilder(instrumentationtest.New().ToProviderSettings(), store, fm, cb, rewriter, nil, fl, false, 100000)

	query := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
		Signal:       telemetrytypes.SignalTraces,
		StepInterval: qbtypes.Step{Duration: 30e9},
		Aggregations: []qbtypes.TraceAggregation{{Expression: "count()"}},
		Limit:        10,
	}
	cases := map[qbtypes.RequestType]int{
		qbtypes.RequestTypeRaw:        1,
		qbtypes.RequestTypeTimeSeries: 1,
		qbtypes.RequestTypeScalar:     1,
		qbtypes.RequestTypeTrace:      2,
	}
	for rt, reads := range cases {
		t.Run(rt.StringValue(), func(t *testing.T) {
			stmt, err := b.Build(tenantCtx(), 1747947419000, 1747983448000, rt, query, nil)
			require.NoError(t, err)
			require.GreaterOrEqual(t, countOf(stmt.Args, testTenant), reads)

			_, err = b.Build(context.Background(), 1747947419000, 1747983448000, rt, query, nil)
			require.Error(t, err)
			require.True(t, errors.Ast(err, errors.TypeForbidden), "a read with no tenant is refused as forbidden, got %v", err)
		})
	}
}

func countOf(args []any, v any) int {
	n := 0
	for _, a := range args {
		if a == v {
			n++
		}
	}
	return n
}
