package tracefunnel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/telemetryplane"
	"github.com/hanzoai/o11y/pkg/telemetrytraces"
)

// All six funnel queries named `o11y_traces.distributed_o11y_index_v3` — a
// database HIP-0132 dropped — and the tests in this package stayed green through
// all of it, because every one of them asserts on the WITH clause and the step
// predicates and none of them ever looked at the FROM.
//
// A test that checks everything except the one thing that breaks is not
// coverage. This checks the FROM.
func TestEveryFunnelQueryReadsThePlane(t *testing.T) {
	type step2 = struct {
		ServiceName   string
		SpanName      string
		ContainsError int
		Clause        string
	}
	type stepLatency = struct {
		ServiceName    string
		SpanName       string
		ContainsError  int
		LatencyPointer string
		Clause         string
	}
	type stepLatencyTyped = struct {
		ServiceName    string
		SpanName       string
		ContainsError  int
		LatencyPointer string
		LatencyType    string
		Clause         string
	}

	plain := []step2{
		{ServiceName: "a", SpanName: "s1", ContainsError: 0, Clause: ""},
		{ServiceName: "b", SpanName: "s2", ContainsError: 0, Clause: ""},
	}
	latency := []stepLatency{
		{ServiceName: "a", SpanName: "s1", ContainsError: 0, LatencyPointer: "start", Clause: ""},
		{ServiceName: "b", SpanName: "s2", ContainsError: 0, LatencyPointer: "start", Clause: ""},
	}
	latencyTyped := []stepLatencyTyped{
		{ServiceName: "a", SpanName: "s1", ContainsError: 0, LatencyPointer: "start", LatencyType: "p99", Clause: ""},
		{ServiceName: "b", SpanName: "s2", ContainsError: 0, LatencyPointer: "start", LatencyType: "p99", Clause: ""},
	}

	const start, end = int64(1700000000000000000), int64(1700003600000000000)

	queries := map[string]string{
		"validation":       BuildFunnelValidationQuery(plain, start, end),
		"overview":         BuildFunnelOverviewQuery(latency, start, end),
		"count":            BuildFunnelCountQuery(plain, start, end),
		"stepOverview":     BuildFunnelStepOverviewQuery(latencyTyped, start, end, 1, 2),
		"topSlowTraces":    BuildFunnelTopSlowTracesQuery(0, 0, start, end, "a", "s1", "b", "s2", "", "", "start", "start"),
		"topSlowErrTraces": BuildFunnelTopSlowErrorTracesQuery(0, 1, start, end, "a", "s1", "b", "s2", "", "", "start", "start"),
	}

	wantTable := telemetryplane.DBName + "." + telemetrytraces.SpanTableName

	for name, sql := range queries {
		t.Run(name, func(t *testing.T) {
			require.NotEmpty(t, sql)
			assert.Containsf(t, sql, wantTable,
				"funnel query must read the event plane's span table, got:\n%s", sql)
			for _, gone := range []string{"o11y_traces", "o11y_logs", "o11y_metrics"} {
				assert.NotContainsf(t, sql, gone,
					"funnel query names %s, a database HIP-0132 dropped", gone)
			}
		})
	}
}
