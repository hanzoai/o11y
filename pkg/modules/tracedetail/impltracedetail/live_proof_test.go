package impltracedetail

// THROWAWAY live-plane proof. Gated on O11Y_LIVE_PROOF=1 + O11Y_PROOF_DSN.
// Runs the ACTUAL trace-detail store methods (span lookup) against the live
// event plane end to end. Deleted after the proof.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/datastoretelemetrystore"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/stretchr/testify/require"
)

func TestLiveProofSpanLookup(t *testing.T) {
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

	store := NewTraceStore(ts)
	ctx := context.Background()
	const traceID = "prooftrace01"

	summary, err := store.GetTraceSummary(ctx, traceID)
	require.NoError(t, err)
	require.Equal(t, traceID, summary.TraceID)
	require.Greater(t, summary.NumSpans, uint64(0))
	t.Logf("summary: start=%s end=%s spans=%d", summary.Start, summary.End, summary.NumSpans)

	spans, err := store.GetTraceSpans(ctx, traceID, summary)
	require.NoError(t, err)
	require.NotEmpty(t, spans)
	t.Logf("GetTraceSpans: %d spans, first: id=%s svc=%s kind=%d kindStr=%s err=%v attrs=%d",
		len(spans), spans[0].SpanID, spans[0].ServiceName, spans[0].Kind, spans[0].SpanKind, spans[0].HasError, len(spans[0].AttributesString))

	minimal, err := store.GetMinimalSpans(ctx, traceID, summary.Start, summary.End)
	require.NoError(t, err)
	require.NotEmpty(t, minimal)
	t.Logf("GetMinimalSpans: %d", len(minimal))

	flame, err := store.GetFlamegraphSpans(ctx, traceID, summary.Start, summary.End, nil)
	require.NoError(t, err)
	require.NotEmpty(t, flame)
	t.Logf("GetFlamegraphSpans: %d, first resources=%v", len(flame), flame[0].ResourcesString)

	byIDs, err := store.GetTraceSpansByIDs(ctx, traceID, summary.Start, summary.End, []string{spans[0].SpanID})
	require.NoError(t, err)
	require.NotEmpty(t, byIDs)
	t.Logf("GetTraceSpansByIDs: %d", len(byIDs))

	counts, err := store.GetSpanCountByField(ctx, traceID, summary, telemetrytypes.TelemetryFieldKey{
		Name: "service.name", FieldContext: telemetrytypes.FieldContextResource,
	})
	require.NoError(t, err)
	t.Logf("GetSpanCountByField(service.name): %v", counts)

	durations, err := store.GetSpanDurationByField(ctx, traceID, summary, telemetrytypes.TelemetryFieldKey{
		Name: "service.name", FieldContext: telemetrytypes.FieldContextResource,
	})
	require.NoError(t, err)
	t.Logf("GetSpanDurationByField(service.name): %v", durations)
}
