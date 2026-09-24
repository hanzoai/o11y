package telemetrylogs

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

// testTenant is the org the statement tests read as; every event.log read
// they build binds it as `org = ?`.
const testTenant = "acme"

func tenantCtx() context.Context {
	return authtypes.NewContextWithTenant(context.Background(), testTenant)
}

func newTenantTestBuilder(t *testing.T) qbtypes.StatementBuilder[qbtypes.LogAggregation] {
	fl := flaggertest.New(t)
	store := telemetrytypestest.NewMockMetadataStore()
	store.KeysMap = buildCompleteFieldKeyMap(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))
	fm := NewFieldMapper(fl)
	cb := NewConditionBuilder(fm, fl)
	rewriter := querybuilder.NewAggExprRewriter(instrumentationtest.New().ToProviderSettings(), nil, fm, cb, nil, fl)
	return NewLogQueryStatementBuilder(instrumentationtest.New().ToProviderSettings(), store, fm, cb, rewriter,
		DefaultFullTextColumn, GetBodyJSONKey, fl, nil, false, 100000)
}

// Every request type reads event.log for exactly one org, and none reads it
// for no org.
func TestLogReadsAreScopedToTheTenant(t *testing.T) {
	b := newTenantTestBuilder(t)
	query := qbtypes.QueryBuilderQuery[qbtypes.LogAggregation]{
		Signal:       telemetrytypes.SignalLogs,
		StepInterval: qbtypes.Step{Duration: 30e9},
		Aggregations: []qbtypes.LogAggregation{{Expression: "count()"}},
		Limit:        10,
	}
	for _, rt := range []qbtypes.RequestType{qbtypes.RequestTypeRaw, qbtypes.RequestTypeTimeSeries, qbtypes.RequestTypeScalar} {
		t.Run(rt.StringValue(), func(t *testing.T) {
			stmt, err := b.Build(tenantCtx(), 1747947419000, 1747983448000, rt, query, nil)
			require.NoError(t, err)
			require.Contains(t, stmt.Query, "org = ?")
			require.Contains(t, stmt.Args, testTenant)

			_, err = b.Build(context.Background(), 1747947419000, 1747983448000, rt, query, nil)
			require.Error(t, err)
			require.True(t, errors.Ast(err, errors.TypeForbidden), "a read with no tenant is refused as forbidden, got %v", err)
		})
	}
}
