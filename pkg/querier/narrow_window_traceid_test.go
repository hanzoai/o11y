package querier

import (
	"context"
	"io"
	"log/slog"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	dsmock "github.com/hanzo-ds/mock"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/telemetrystoretest"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realTraceID is the live trace 9aa37a283590028516f27b45e3a4406a whose single
// gen_ai span (org=hanzo, model enso-flash, 211 tokens, $0.01) proved the read leg
// held the row all along — the detail view rendered "no observations" purely because
// the trace_id filter was a `$N` placeholder the trace-summary optimizer never
// resolved, short-circuiting the span query to empty before it ran.
const realTraceID = "9aa37a283590028516f27b45e3a4406a"

// llmobsDetailFilter is exactly what impllmobs.genAIFilter emits for an
// observations/traces DETAIL request: org bound as $1, trace_id bound as $2.
const llmobsDetailFilter = "gen_ai.system EXISTS AND gen_ai.hanzo.org_id = $1 AND trace_id = $2"

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newTraceStore(t *testing.T) *telemetrystoretest.Provider {
	t.Helper()
	return telemetrystoretest.New(telemetrystore.Config{}, sqlmock.QueryMatcherRegexp)
}

// TestNarrowWindowParameterizedTraceID is the regression test for the fleet-wide
// "trace shows, no observations" bug. The llmobs span views parameterize the
// trace_id predicate as `trace_id = $2` and carry the real id in the request
// Variables map; narrowWindowByTraceID reads the filter TEXT, so before the fix it
// looked up the literal "$2" in distributed_trace_summary, found nothing, and — for
// SignalTraces — short-circuited the whole span query to empty (overlap=false).
func TestNarrowWindowParameterizedTraceID(t *testing.T) {
	// The trace's nanosecond bounds; narrowWindowByTraceID pads by 1s and clamps the
	// ms window to them.
	const startNano = int64(1_700_000_000_000_000_000)
	const endNano = int64(1_700_000_001_200_000_000)
	fromMS := uint64(1_699_999_990_000)
	toMS := uint64(1_700_000_010_000)

	summaryCols := []dsmock.ColumnType{
		{Name: "count", Type: "UInt64"},
		{Name: "start", Type: "Int64"},
		{Name: "end", Type: "Int64"},
	}

	// A bound placeholder resolves to the real id, so the trace_summary lookup runs,
	// finds the trace, and the window is clamped to its bounds — the query is NOT
	// short-circuited. (Both org=$1 and trace_id=$2 are bound, as in production.)
	t.Run("bound placeholder runs the lookup and narrows the window", func(t *testing.T) {
		store := newTraceStore(t)
		store.Mock().ExpectQueryRow(`distributed_trace_summary`).
			WillReturnRow(dsmock.NewRow(summaryCols, []any{uint64(1), startNano, endNano}))

		q := &builderQuery[qbtypes.TraceAggregation]{
			logger:         discardLogger(),
			telemetryStore: store,
			spec: qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
				Signal: telemetrytypes.SignalTraces,
				Filter: &qbtypes.Filter{Expression: llmobsDetailFilter},
			},
			variables: map[string]qbtypes.VariableItem{
				"1": {Type: qbtypes.DynamicVariableType, Value: "hanzo"},
				"2": {Type: qbtypes.DynamicVariableType, Value: realTraceID},
			},
		}

		gotFrom, gotTo, overlap, warning := q.narrowWindowByTraceID(context.Background(), fromMS, toMS)

		require.True(t, overlap, "a resolvable parameterized trace_id must NOT short-circuit the span query")
		assert.Empty(t, warning)
		assert.Equal(t, uint64((startNano-1_000_000_000)/1_000_000), gotFrom, "from clamped to trace start")
		assert.Equal(t, uint64((endNano+1_000_000_000)/1_000_000), gotTo, "to clamped to trace end")
		require.NoError(t, store.Mock().ExpectationsWereMet(), "trace_summary lookup must run for a bound trace_id")
	})

	// An UNBOUND `$N` placeholder cannot be resolved to a concrete id. The fix skips
	// the optimization entirely (overlap=true → the fully-substituted main query
	// runs). Fail-before: the old code extracts the literal "$2", queries
	// trace_summary (count=0 here) and short-circuits to empty (overlap=false) — this
	// assertion fails. Pass-after: the lookup is skipped, so overlap=true and the mock
	// is never called.
	t.Run("unbound placeholder skips the optimization instead of short-circuiting", func(t *testing.T) {
		store := newTraceStore(t)
		// Only the pre-fix path reaches this; it simulates "$2" not existing in
		// trace_summary. We deliberately do NOT assert ExpectationsWereMet, since the
		// fixed path skips the query.
		store.Mock().ExpectQueryRow(`distributed_trace_summary`).
			WillReturnRow(dsmock.NewRow(summaryCols, []any{uint64(0), int64(0), int64(0)}))

		q := &builderQuery[qbtypes.TraceAggregation]{
			logger:         discardLogger(),
			telemetryStore: store,
			spec: qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
				Signal: telemetrytypes.SignalTraces,
				Filter: &qbtypes.Filter{Expression: llmobsDetailFilter},
			},
			variables: map[string]qbtypes.VariableItem{}, // trace_id $2 is NOT bound
		}

		_, _, overlap, _ := q.narrowWindowByTraceID(context.Background(), fromMS, toMS)
		require.True(t, overlap, "an unresolved $N placeholder must skip the optimization, never short-circuit to empty")
	})
}

// TestResolveTraceIDVars covers the placeholder-resolution helper directly: this is
// the seam the whole bug turned on — the extractor hands back "$2", and this resolves
// it against the bound variables into the real id that reaches trace_summary.
func TestResolveTraceIDVars(t *testing.T) {
	vars := map[string]qbtypes.VariableItem{
		"2":   {Type: qbtypes.DynamicVariableType, Value: realTraceID},
		"tid": {Type: qbtypes.DynamicVariableType, Value: "abc123"},
		"ids": {Type: qbtypes.DynamicVariableType, Value: []any{"a1", "b2"}},
	}
	q := &builderQuery[qbtypes.TraceAggregation]{variables: vars}

	tests := []struct {
		name     string
		in       []string
		want     []string
		resolved bool
	}{
		{name: "positional placeholder resolves to real id", in: []string{"$2"}, want: []string{realTraceID}, resolved: true},
		{name: "named placeholder resolves", in: []string{"$tid"}, want: []string{"abc123"}, resolved: true},
		{name: "list placeholder expands", in: []string{"$ids"}, want: []string{"a1", "b2"}, resolved: true},
		{name: "inline literal passes through", in: []string{"deadbeef"}, want: []string{"deadbeef"}, resolved: true},
		{name: "mixed literal and placeholder", in: []string{"lit", "$2"}, want: []string{"lit", realTraceID}, resolved: true},
		{name: "unbound placeholder skips optimization", in: []string{"$9"}, want: nil, resolved: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := q.resolveTraceIDVars(tt.in)
			assert.Equal(t, tt.resolved, ok)
			if tt.resolved {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
