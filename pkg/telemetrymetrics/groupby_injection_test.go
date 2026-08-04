package telemetrymetrics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/types/metrictypes"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes/telemetrytypestest"
	"github.com/stretchr/testify/require"
)

// A group-by key name is caller text: it arrives as JSON on /api/v5/query_range
// and is replayed out of stored dashboards and alert rules — the rule evaluator
// builds a QueryRangeRequest and calls the querier directly, so a name on that
// path never meets the request validator at all.
//
// It lands in TWO syntactic positions and a bound parameter can occupy NEITHER:
// a backtick-quoted identifier (the SELECT alias, the GROUP BY term, the ORDER
// BY term) and a single-quoted literal (JSONExtractString(labels, '<name>')).
//
// Metrics is the signal that proves the guard cannot live in the validator: a
// metric label is free-form, so an unrecognised name falls back to the labels
// column BY DESIGN and there is no key allowlist for it to fail against.
//
// The assertion is on the SQL the real builder emits, and it is structural
// rather than textual: the payload is allowed to APPEAR — quoted, as the inert
// name of a label nobody has — but it must never appear in the EXECUTABLE
// SKELETON, the text left after every quoted region is removed. That is the
// property an injection breaks, and it fails if any layer underneath stops
// escaping.
func TestGroupByKeyCannotEscapeItsQuoting(t *testing.T) {
	fm := NewFieldMapper()
	cb := NewConditionBuilder(fm)
	fl, err := flagger.New(context.Background(), instrumentationtest.New().ToProviderSettings(), flagger.Config{}, flagger.MustNewRegistry())
	require.NoError(t, err)
	sb := NewMetricQueryStatementBuilder(instrumentationtest.New().ToProviderSettings(), telemetrytypestest.NewMockMetadataStore(), fm, cb, fl)

	// Every payload is all-lowercase and spells its spaces "/**/": the two things
	// that let a payload survive a builder that folds case or strips whitespace.
	for _, c := range []struct{ name, payload, mustNotExecute string }{
		{
			name:           "backtick closes the identifier and appends a column",
			payload:        "le`,(select/**/grouparray(name)/**/from/**/system.tables)/**/as/**/`x",
			mustNotExecute: "system.tables",
		},
		{
			name:           "quote closes the literal in the labels access",
			payload:        "le'),(select/**/1))/**/union/**/all/**/select/**/1,'x",
			mustNotExecute: "union/**/all",
		},
		{
			name:           "trailing backslash eats the closing delimiter",
			payload:        `le\`,
			mustNotExecute: "",
		},
		{
			name:           "comment truncates the rest of the statement",
			payload:        "le`--",
			mustNotExecute: "--",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			q := qbtypes.QueryBuilderQuery[qbtypes.MetricAggregation]{
				Signal:       telemetrytypes.SignalMetrics,
				StepInterval: qbtypes.Step{Duration: 5 * time.Minute},
				Aggregations: []qbtypes.MetricAggregation{{
					MetricName:       "test.metric",
					Type:             metrictypes.GaugeType,
					Temporality:      metrictypes.Unspecified,
					TimeAggregation:  metrictypes.TimeAggregationLatest,
					SpaceAggregation: metrictypes.SpaceAggregationSum,
				}},
				GroupBy: []qbtypes.GroupByKey{
					{TelemetryFieldKey: telemetrytypes.TelemetryFieldKey{Name: c.payload}},
				},
			}
			got, err := sb.Build(context.Background(), uint64(1747000000000), uint64(1747172800000), qbtypes.RequestTypeTimeSeries, q, nil)
			require.NoError(t, err)

			skeleton, ok := executableSkeleton(got.Query)
			require.True(t, ok, "a quoted region was left open, so the rest of the statement is attacker-controlled: %s", got.Query)
			if c.mustNotExecute != "" {
				require.NotContains(t, skeleton, c.mustNotExecute,
					"payload escaped its quoting and became executable SQL.\nskeleton: %s\nfull: %s", skeleton, got.Query)
			}
		})
	}
}

// A legitimate label name must still render, and unchanged — the escaper is a
// no-op on every real attribute key, which is a dotted identifier.
func TestGroupByKeyOrdinaryNamesAreUnchanged(t *testing.T) {
	fm := NewFieldMapper()
	cb := NewConditionBuilder(fm)
	fl, err := flagger.New(context.Background(), instrumentationtest.New().ToProviderSettings(), flagger.Config{}, flagger.MustNewRegistry())
	require.NoError(t, err)
	sb := NewMetricQueryStatementBuilder(instrumentationtest.New().ToProviderSettings(), telemetrytypestest.NewMockMetadataStore(), fm, cb, fl)

	for _, name := range []string{"le", "service.name", "http.status_code", "k8s.pod.name"} {
		q := qbtypes.QueryBuilderQuery[qbtypes.MetricAggregation]{
			Signal:       telemetrytypes.SignalMetrics,
			StepInterval: qbtypes.Step{Duration: 5 * time.Minute},
			Aggregations: []qbtypes.MetricAggregation{{
				MetricName:       "test.metric",
				Type:             metrictypes.GaugeType,
				Temporality:      metrictypes.Unspecified,
				TimeAggregation:  metrictypes.TimeAggregationLatest,
				SpaceAggregation: metrictypes.SpaceAggregationSum,
			}},
			GroupBy: []qbtypes.GroupByKey{
				{TelemetryFieldKey: telemetrytypes.TelemetryFieldKey{Name: name}},
			},
		}
		got, err := sb.Build(context.Background(), uint64(1747000000000), uint64(1747172800000), qbtypes.RequestTypeTimeSeries, q, nil)
		require.NoError(t, err)
		require.Contains(t, got.Query, "`"+name+"`", "ordinary name lost its plain rendering")
		require.Contains(t, got.Query, "JSONExtractString(labels, '"+name+"')", "ordinary name lost its plain literal")
	}
}

// executableSkeleton returns the SQL with every backtick-quoted identifier and
// every single-quoted literal replaced by a placeholder, honouring backslash
// escapes exactly as the datastore does. What is left is the text the server
// will EXECUTE as syntax. ok is false if a region is still open at the end,
// which by itself means the statement was broken out of.
func executableSkeleton(sql string) (string, bool) {
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
				i++ // the escaped byte is data, never a delimiter
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
