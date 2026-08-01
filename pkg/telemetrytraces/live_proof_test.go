package telemetrytraces

// THROWAWAY live-plane proof. Gated on O11Y_LIVE_PROOF=1 + O11Y_PROOF_DSN.
// Builds real statements through the trace statement builder and EXECUTES them
// against the live event plane with real arg binding. Deleted after the proof.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/flagger/flaggertest"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/querybuilder"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/datastoretelemetrystore"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes/telemetrytypestest"
	"github.com/stretchr/testify/require"
)

func liveTelemetryStore(t *testing.T) telemetrystore.TelemetryStore {
	t.Helper()
	if os.Getenv("O11Y_LIVE_PROOF") != "1" {
		t.Skip("live proof disabled")
	}
	dsn := os.Getenv("O11Y_PROOF_DSN")
	require.NotEmpty(t, dsn, "O11Y_PROOF_DSN required")
	ts, err := datastoretelemetrystore.New(
		context.Background(),
		instrumentationtest.New().ToProviderSettings(),
		telemetrystore.Config{
			Provider:   "datastore",
			Connection: telemetrystore.ConnectionConfig{MaxOpenConns: 2, MaxIdleConns: 1, DialTimeout: 5 * time.Second},
			Datastore:  telemetrystore.DatastoreConfig{DSN: dsn},
		},
	)
	require.NoError(t, err)
	return ts
}

func execCount(t *testing.T, ts telemetrystore.TelemetryStore, stmt qbtypes.Statement) int {
	t.Helper()
	rows, err := ts.Datastore().Query(context.Background(), stmt.Query, stmt.Args...)
	require.NoError(t, err, "EXEC FAILED\nSQL: %s\nARGS: %v", stmt.Query, stmt.Args)
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	require.NoError(t, rows.Err())
	return n
}

func TestLiveProofTraces(t *testing.T) {
	ts := liveTelemetryStore(t)

	releaseTime := time.Date(2025, 5, 22, 22, 0, 0, 0, time.UTC)
	fm := NewFieldMapper()
	cb := NewConditionBuilder(fm)
	mockMetadataStore := telemetrytypestest.NewMockMetadataStore()
	mockMetadataStore.KeysMap = buildCompleteFieldKeyMap(releaseTime)
	fl := flaggertest.New(t)
	aggExprRewriter := querybuilder.NewAggExprRewriter(instrumentationtest.New().ToProviderSettings(), nil, fm, cb, nil, fl)

	statementBuilder := NewTraceQueryStatementBuilder(
		instrumentationtest.New().ToProviderSettings(),
		mockMetadataStore,
		fm,
		cb,
		aggExprRewriter,
		nil,
		fl,
		false,
		100000,
	)

	// window covering the seeded proof rows (bucket 1785544200)
	start := uint64(1785540000000)
	end := uint64(1785560000000)

	t.Run("span list with resource filter CTE", func(t *testing.T) {
		q := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
			Signal: telemetrytypes.SignalTraces,
			Filter: &qbtypes.Filter{Expression: "service.name = 'o11y-writeside-proof' AND http.request.method = 'GET'"},
			SelectFields: []telemetrytypes.TelemetryFieldKey{
				{Name: "service.name", FieldContext: telemetrytypes.FieldContextResource},
				{Name: "name", FieldContext: telemetrytypes.FieldContextSpan},
				{Name: "duration_nano", FieldContext: telemetrytypes.FieldContextSpan},
				{Name: "response_status_code", FieldContext: telemetrytypes.FieldContextSpan},
			},
			Order: []qbtypes.OrderBy{{Key: qbtypes.OrderByKey{TelemetryFieldKey: telemetrytypes.TelemetryFieldKey{Name: "time"}}, Direction: qbtypes.OrderDirectionDesc}},
			Limit: 10,
		}
		stmt, err := statementBuilder.Build(context.Background(), start, end, qbtypes.RequestTypeRaw, q, nil)
		require.NoError(t, err)
		n := execCount(t, ts, *stmt)
		t.Logf("span list rows=%d\nSQL: %s\nARGS: %v", n, stmt.Query, stmt.Args)
		require.Greater(t, n, 0, "expected proof spans to match")
	})

	t.Run("span list intrinsic filters", func(t *testing.T) {
		q := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
			Signal: telemetrytypes.SignalTraces,
			Filter: &qbtypes.Filter{Expression: "kind_string = 'client' AND has_error = true AND duration_nano > 1000"},
			Limit:  10,
		}
		stmt, err := statementBuilder.Build(context.Background(), start, end, qbtypes.RequestTypeRaw, q, nil)
		require.NoError(t, err)
		n := execCount(t, ts, *stmt)
		t.Logf("intrinsic rows=%d\nSQL: %s\nARGS: %v", n, stmt.Query, stmt.Args)
		require.Greater(t, n, 0, "expected error client span")
	})

	t.Run("time series group by service.name", func(t *testing.T) {
		q := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
			Signal:       telemetrytypes.SignalTraces,
			StepInterval: qbtypes.Step{Duration: 60 * time.Second},
			Aggregations: []qbtypes.TraceAggregation{{Expression: "count()"}},
			Filter:       &qbtypes.Filter{Expression: "service.name = 'o11y-writeside-proof'"},
			GroupBy:      []qbtypes.GroupByKey{{TelemetryFieldKey: telemetrytypes.TelemetryFieldKey{Name: "service.name"}}},
			Limit:        10,
		}
		stmt, err := statementBuilder.Build(context.Background(), start, end, qbtypes.RequestTypeTimeSeries, q, nil)
		require.NoError(t, err)
		n := execCount(t, ts, *stmt)
		t.Logf("timeseries rows=%d\nSQL: %s\nARGS: %v", n, stmt.Query, stmt.Args)
		require.Greater(t, n, 0)
	})

	t.Run("trace query root spans", func(t *testing.T) {
		q := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
			Signal: telemetrytypes.SignalTraces,
			Filter: &qbtypes.Filter{Expression: "service.name = 'o11y-writeside-proof'"},
			Limit:  10,
		}
		stmt, err := statementBuilder.Build(context.Background(), start, end, qbtypes.RequestTypeTrace, q, nil)
		require.NoError(t, err)
		n := execCount(t, ts, *stmt)
		t.Logf("trace query rows=%d\nSQL: %s\nARGS: %v", n, stmt.Query, stmt.Args)
		require.Greater(t, n, 0)
	})

	t.Run("span scope isroot", func(t *testing.T) {
		q := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
			Signal: telemetrytypes.SignalTraces,
			Filter: &qbtypes.Filter{Expression: "isroot = true"},
			Limit:  5,
		}
		stmt, err := statementBuilder.Build(context.Background(), start, end, qbtypes.RequestTypeRaw, q, nil)
		require.NoError(t, err)
		n := execCount(t, ts, *stmt)
		t.Logf("isroot rows=%d\nSQL: %s\nARGS: %v", n, stmt.Query, stmt.Args)
		require.Greater(t, n, 0)
	})

	t.Run("span scope isentrypoint", func(t *testing.T) {
		q := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
			Signal: telemetrytypes.SignalTraces,
			Filter: &qbtypes.Filter{Expression: "isentrypoint = true"},
			Limit:  5,
		}
		stmt, err := statementBuilder.Build(context.Background(), start, end, qbtypes.RequestTypeRaw, q, nil)
		require.NoError(t, err)
		n := execCount(t, ts, *stmt)
		t.Logf("isentrypoint rows=%d\nSQL: %s\nARGS: %v", n, stmt.Query, stmt.Args)
	})
}
