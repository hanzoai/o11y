package telemetrylogs

// THROWAWAY live-plane proof. Gated on O11Y_LIVE_PROOF=1 + O11Y_PROOF_DSN.
// Builds real statements through the log statement builder and EXECUTES them
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

func liveLogTelemetryStore(t *testing.T) telemetrystore.TelemetryStore {
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

func execLogCount(t *testing.T, ts telemetrystore.TelemetryStore, stmt qbtypes.Statement) int {
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

func TestLiveProofLogs(t *testing.T) {
	ts := liveLogTelemetryStore(t)

	releaseTime := time.Date(2025, 5, 6, 0, 0, 0, 0, time.UTC)
	fl := flaggertest.New(t)
	mockMetadataStore := telemetrytypestest.NewMockMetadataStore()
	mockMetadataStore.KeysMap = buildCompleteFieldKeyMap(releaseTime)
	fm := NewFieldMapper(fl)
	cb := NewConditionBuilder(fm, fl)
	aggExprRewriter := querybuilder.NewAggExprRewriter(instrumentationtest.New().ToProviderSettings(), nil, fm, cb, nil, fl)

	statementBuilder := NewLogQueryStatementBuilder(
		instrumentationtest.New().ToProviderSettings(),
		mockMetadataStore,
		fm,
		cb,
		aggExprRewriter,
		DefaultFullTextColumn,
		GetBodyJSONKey,
		fl,
		nil,
		false,
		100000,
	)

	start := uint64(1785540000000)
	end := uint64(1785560000000)

	t.Run("log search with resource filter CTE and body", func(t *testing.T) {
		q := qbtypes.QueryBuilderQuery[qbtypes.LogAggregation]{
			Signal: telemetrytypes.SignalLogs,
			Filter: &qbtypes.Filter{Expression: "service.name = 'o11y-writeside-proof' AND body CONTAINS 'boom' AND severity_text = 'error'"},
			Order:  []qbtypes.OrderBy{{Key: qbtypes.OrderByKey{TelemetryFieldKey: telemetrytypes.TelemetryFieldKey{Name: "time"}}, Direction: qbtypes.OrderDirectionDesc}},
			Limit:  10,
		}
		stmt, err := statementBuilder.Build(context.Background(), start, end, qbtypes.RequestTypeRaw, q, nil)
		require.NoError(t, err)
		n := execLogCount(t, ts, *stmt)
		t.Logf("log search rows=%d\nSQL: %s\nARGS: %v", n, stmt.Query, stmt.Args)
		require.Greater(t, n, 0, "expected proof log rows")
	})

	t.Run("log time series by severity", func(t *testing.T) {
		q := qbtypes.QueryBuilderQuery[qbtypes.LogAggregation]{
			Signal:       telemetrytypes.SignalLogs,
			StepInterval: qbtypes.Step{Duration: 60 * time.Second},
			Aggregations: []qbtypes.LogAggregation{{Expression: "count()"}},
			Filter:       &qbtypes.Filter{Expression: "service.name = 'o11y-writeside-proof'"},
			GroupBy:      []qbtypes.GroupByKey{{TelemetryFieldKey: telemetrytypes.TelemetryFieldKey{Name: "severity_text"}}},
			Limit:        10,
		}
		stmt, err := statementBuilder.Build(context.Background(), start, end, qbtypes.RequestTypeTimeSeries, q, nil)
		require.NoError(t, err)
		n := execLogCount(t, ts, *stmt)
		t.Logf("log timeseries rows=%d\nSQL: %s\nARGS: %v", n, stmt.Query, stmt.Args)
		require.Greater(t, n, 0)
	})

	t.Run("log list attribute and trace correlation", func(t *testing.T) {
		q := qbtypes.QueryBuilderQuery[qbtypes.LogAggregation]{
			Signal: telemetrytypes.SignalLogs,
			Filter: &qbtypes.Filter{Expression: "trace_id EXISTS"},
			Limit:  10,
		}
		stmt, err := statementBuilder.Build(context.Background(), start, end, qbtypes.RequestTypeRaw, q, nil)
		require.NoError(t, err)
		n := execLogCount(t, ts, *stmt)
		t.Logf("log trace_id exists rows=%d\nSQL: %s\nARGS: %v", n, stmt.Query, stmt.Args)
	})
}
