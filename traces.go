package o11y

// The TRACES product face — one trace's spans, the trace field catalog and its
// tuning write, and the three trace-detail views (waterfall, flamegraph, span
// aggregations) — as TYPED ops.
//
// These were six of the routes that reached traffic only through the delegation
// wildcard, and a route behind a wildcard is in no document: no SDK method, no
// CLI command, no agent tool, no reference page. Distributed tracing is the
// product's headline read, and until now a customer could not reach a single
// trace from anything but a hand-written HTTP call. Typing them is what puts the
// six operations in the document and therefore in every projection built from it.
//
// THE WIRE DOES NOT MOVE, and the way it does not move is that these ops do not
// re-implement the reads. Each hands the call to the SAME runtime handler the
// wildcard delegates to (relay.go), so identity resolution, the org gate, the
// role check (viewer on the reads, editor on the field write — the exact
// ViewAccess/EditAccess gates the runtime has always run), the audit record and
// the success envelope are all still the runtime's, executed in the order they
// always were. What is new here is the TYPE — the In the caller may send and the
// Out they get back — and the prose that goes with it.
//
// TWO ENVELOPES, HONESTLY NAMED. The trace-detail views come from the
// apiserver, which answers {status, data}; the three query-service reads answer
// the bare value. The Out types below say which is which rather than papering
// over the difference, because the document has to be true of the wire.

//go:generate go run github.com/zap-proto/zip/cmd/zipdoc

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/types/spantypes"
	"github.com/zap-proto/zip"
)

// mountTraces registers the six typed traces ops on the native router. The
// literal /traces/fields routes register ahead of the parameterised trace read
// so a trace id can never shadow the catalog — the same defence the mux tree
// gives them by registering fields first.
func mountTraces(app *zip.App) {
	g := under{app, o11yRoot}
	zip.Get(g, "/traces/fields", traceFields, op("GetTraceFields"))
	zip.Post(g, "/traces/fields", traceFieldUpdate, op("UpdateTraceField"))
	zip.Get(g, "/traces/:traceId", traceSpans, op("SearchTraces"))
	zip.Post(g, "/traces/:traceId/waterfall", traceWaterfall, op("GetWaterfallV4"))
	zip.Post(g, "/traces/:traceId/flamegraph", traceFlamegraph, op("GetFlamegraph"))
	zip.Post(g, "/traces/:traceId/aggregations", traceAggregations, op("GetTraceAggregations"))
}

// ── the six operations ────────────────────────────────────────────────────────

// traceFields returns the trace field catalog: the span fields already selected
// as indexed columns, and the interesting ones seen in the data that could be.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func traceFields(ctx context.Context, _ *struct{}) (*O11yFieldCatalogOut, error) {
	out := new(O11yFieldCatalogOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/traces/fields", nil, nil, out)
}

// traceFieldUpdate changes how one span field is stored — selects or deselects
// it as a materialized column and tunes its index — and echoes the setting back.
//
// Callers need the editor role; the runtime's own gate enforces it.
func traceFieldUpdate(ctx context.Context, in *O11yFieldSetting) (*O11yFieldSetting, error) {
	out := new(O11yFieldSetting)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/traces/fields", nil, in, out)
}

// traceSpans returns one trace's spans as a column/row table, optionally
// centred on a span and walked a fixed number of levels up and down from it —
// the read the trace explorer opens a trace with.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func traceSpans(ctx context.Context, in *O11yTraceSpansIn) (*O11yTraceSpansOut, error) {
	out := new(O11yTraceSpansOut)
	// The trace id goes on VERBATIM, as the segment the router matched: it
	// arrived percent-encoded or it did not, and re-encoding it here would hand
	// the runtime a different id than the caller named.
	return out, relay(ctx, http.MethodGet, o11yRoot+"/traces/"+in.TraceID, query(
		"spanId", in.SpanID,
		"levelUp", in.LevelUp,
		"levelDown", in.LevelDown,
		"spanRenderLimit", in.SpanRenderLimit,
	), nil, out)
}

// traceWaterfall returns a trace's waterfall: every span when the trace is
// small enough, a capped window around the selected span when it is not, with
// the uncollapsed subtrees the caller asked to keep open.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func traceWaterfall(ctx context.Context, in *O11yTraceWaterfallIn) (*O11yTraceWaterfallOut, error) {
	out := new(O11yTraceWaterfallOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/traces/"+in.TraceID+"/waterfall", nil, in.PostableWaterfall, out)
}

// traceFlamegraph returns a trace's flamegraph: spans bucketed by depth level,
// each level ordered as it is drawn, around the selected span.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func traceFlamegraph(ctx context.Context, in *O11yTraceFlamegraphIn) (*O11yTraceFlamegraphOut, error) {
	out := new(O11yTraceFlamegraphOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/traces/"+in.TraceID+"/flamegraph", nil, in.PostableFlamegraph, out)
}

// traceAggregations computes span aggregations over one trace — span count,
// duration or share of execution time — grouped by the resource field each
// aggregation names.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func traceAggregations(ctx context.Context, in *O11yTraceAggregationsIn) (*O11yTraceAggregationsOut, error) {
	out := new(O11yTraceAggregationsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/traces/"+in.TraceID+"/aggregations", nil, in.PostableTraceAggregations, out)
}

// ── inputs ────────────────────────────────────────────────────────────────────

// O11yTraceSpansIn names the trace to read and how much of it to walk.
type O11yTraceSpansIn struct {
	// TraceID is the trace to read. Required.
	TraceID string `json:"-" url:"traceId" validate:"required"`
	// SpanID centres the read on one span. Empty reads from the trace roots.
	SpanID string `json:"-" url:"spanId"`
	// LevelUp is how many ancestor levels above the selected span to include.
	LevelUp int `json:"-" url:"levelUp"`
	// LevelDown is how many descendant levels below the selected span to
	// include.
	LevelDown int `json:"-" url:"levelDown"`
	// SpanRenderLimit caps how many spans come back. Zero takes the runtime's
	// own default.
	SpanRenderLimit int `json:"-" url:"spanRenderLimit"`
}

// O11yTraceWaterfallIn names the trace and the waterfall's view state. The
// body is the runtime's own PostableWaterfall, embedded rather than restated so
// the two cannot drift.
type O11yTraceWaterfallIn struct {
	// TraceID is the trace to draw. Required.
	TraceID string `json:"-" url:"traceId" validate:"required"`

	spantypes.PostableWaterfall
}

// O11yTraceFlamegraphIn names the trace and the flamegraph's view state.
type O11yTraceFlamegraphIn struct {
	// TraceID is the trace to draw. Required.
	TraceID string `json:"-" url:"traceId" validate:"required"`

	spantypes.PostableFlamegraph
}

// O11yTraceAggregationsIn names the trace and the aggregations to compute.
type O11yTraceAggregationsIn struct {
	// TraceID is the trace to aggregate. Required.
	TraceID string `json:"-" url:"traceId" validate:"required"`

	spantypes.PostableTraceAggregations
}

// ── answers ───────────────────────────────────────────────────────────────────

// O11yTraceSpansOut is one trace's spans as the explorer reads them: a set of
// windows, each a column list and the rows under it. It is an ARRAY on the wire
// — the runtime writes the bare value with no envelope — and the type says so
// rather than wrapping it into an object the wire does not have.
type O11yTraceSpansOut []O11yTraceSpanWindow

// O11yTraceSpanWindow is one contiguous window of a trace's spans.
type O11yTraceSpanWindow struct {
	// StartTimestampMillis is when the window opens.
	StartTimestampMillis uint64 `json:"startTimestampMillis"`
	// EndTimestampMillis is when it closes.
	EndTimestampMillis uint64 `json:"endTimestampMillis"`
	// Columns names the fields each row carries, in row order.
	Columns []string `json:"columns"`
	// Events are the rows, each positionally matching Columns.
	Events [][]any `json:"events"`
	// IsSubTree says the window is a subtree of the trace rather than the whole
	// of it.
	IsSubTree bool `json:"isSubTree"`
}

// O11yTraceWaterfallOut is a trace's waterfall view.
type O11yTraceWaterfallOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the waterfall.
	Data spantypes.GettableWaterfallTrace `json:"data"`
}

// O11yTraceFlamegraphOut is a trace's flamegraph view.
type O11yTraceFlamegraphOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the flamegraph.
	Data spantypes.GettableFlamegraphTrace `json:"data"`
}

// O11yTraceAggregationsOut is a trace's computed span aggregations.
type O11yTraceAggregationsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds one result per aggregation asked for.
	Data spantypes.GettableTraceAggregations `json:"data"`
}
