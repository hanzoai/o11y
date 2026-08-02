package o11y

// The QUERY-CORE face — the composite query engine and the reads that feed it:
// the v5 querier (query_range, its dry-run preview, variable substitution), the
// legacy metrics range read and the builder-format echo, attribute autocomplete
// for filter building, the field catalog, and the saved explorer views — as
// TYPED ops.
//
// These were sixteen of the routes that reached traffic only through the
// delegation wildcard, and a route behind a wildcard is in no document: no SDK
// method, no CLI command, no agent tool, no reference page. A customer could not
// run a query, complete a filter or open a saved view from anything but a
// hand-written HTTP call. Typing them is what puts the sixteen operations in the
// document and therefore in every projection built from it.
//
// THE WIRE DOES NOT MOVE, and the way it does not move is that these ops do not
// re-implement the reads and writes. Each hands the call to the SAME runtime
// handler the wildcard delegates to (see relayAt in logs.go), so identity
// resolution, the org gate, the role check — viewer on the reads and the query
// engine, editor on the saved-view writes, the exact ViewAccess/EditAccess gates
// the runtime has always run — the validation, the audit record and the success
// envelope are all still the runtime's, executed in the order they always were.
// What is new here is the TYPE — the In the caller may send and the Out they get
// back — and the prose that goes with it.
//
// THREE routes of this slice are deliberately NOT typed, and stay on the
// delegation wildcard byte-identical to what they always were:
//
//   - GET /v1/o11y/query_progress and GET /ws/query_progress both upgrade the
//     connection to a websocket (GetQueryProgressUpdates), and the relay buffers
//     a whole answer through an httptest recorder that cannot hijack a
//     connection — a typed progress op would never complete the handshake.
//   - POST /v1/o11y/export_raw_data streams a chunked CSV/JSONL download with a
//     Content-Disposition attachment and an X-Response-Complete trailer; it has
//     no JSON answer to name, and buffering an unbounded export to re-marshal it
//     would defeat the stream. It stays a binary download.
//
// Those three are the slice's escape hatches; this comment is their record.
//
// Registered ahead of the wildcard, and specific-beats-wildcard is what the
// router does regardless of registration order, so these sixteen paths dispatch
// here and every other path under the prefix — the three streams included —
// still reaches the runtime untouched.

import (
	"context"
	"encoding/json"
	"net/http"

	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/zap-proto/zip"
)

// mountQueryCore registers the sixteen typed query-core ops on the native
// router, all on the shared /v1/o11y group (o11yRoot, spelled once in logs.go).
// The saved-view collection routes register before the parameterised ones so a
// view id can never shadow the collection, exactly as the mux tree ordered them.
func mountQueryCore(app *zip.App) {
	g := app.Group(o11yRoot)

	// the v5 composite query engine
	zip.Post(g, "/query_range", querierQueryRange)
	zip.Post(g, "/query_range/preview", querierQueryRangePreview)
	zip.Post(g, "/substitute_vars", querierReplaceVariables)

	// the legacy metrics range read and the builder-format echo
	zip.Get(g, "/query_range", metricsQueryRange)
	zip.Post(g, "/query_range/format", queryRangeFormat)

	// attribute autocomplete for filter building
	zip.Get(g, "/autocomplete/aggregate_attributes", autocompleteAggregate)
	zip.Get(g, "/autocomplete/attribute_keys", autocompleteKeys)
	zip.Get(g, "/autocomplete/attribute_values", autocompleteValues)
	zip.Post(g, "/auto_complete/attribute_values", autocompleteValuesPost)

	// the field catalog
	zip.Get(g, "/fields/keys", fieldKeys)
	zip.Get(g, "/fields/values", fieldValues)

	// saved explorer views
	zip.Get(g, "/explorer/views", savedViewList)
	zip.Post(g, "/explorer/views", savedViewCreate)
	zip.Get(g, "/explorer/views/:viewId", savedViewGet)
	zip.Put(g, "/explorer/views/:viewId", savedViewUpdate)
	zip.Delete(g, "/explorer/views/:viewId", savedViewDelete)
}

// ── the composite query engine ─────────────────────────────────────────────────

// querierQueryRange executes a composite query over a time range: builder
// queries over traces, logs and metrics, formulas, trace operators, PromQL and
// Datastore SQL, answering time series, scalars or raw records as the request
// type asks.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func querierQueryRange(ctx context.Context, in *qbtypes.QueryRangeRequest) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/query_range", nil, in, out)
}

// querierQueryRangePreview validates a composite query and renders the Datastore
// statements it would run WITHOUT executing it — a dry run for agentic and
// tooling use. verbose=false trades the rendered SQL and EXPLAIN for a
// lightweight per-query verdict.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func querierQueryRangePreview(ctx context.Context, in *O11yQueryRangePreviewIn) (*O11yQueryRangePreviewOut, error) {
	out := new(O11yQueryRangePreviewOut)
	// verbose rides the query string, the request rides the body — the same
	// split the runtime reads.
	return out, relay(ctx, http.MethodPost, o11yRoot+"/query_range/preview", query("verbose", in.Verbose), &in.QueryRangeRequest, out)
}

// querierReplaceVariables substitutes a query's variables and returns the
// resolved request, without running it — what a dashboard does before it
// queries.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func querierReplaceVariables(ctx context.Context, in *qbtypes.QueryRangeRequest) (*O11ySubstituteVarsOut, error) {
	out := new(O11ySubstituteVarsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/substitute_vars", nil, in, out)
}

// metricsQueryRange runs a Prometheus-style range query over metrics — the
// legacy read that predates the v5 querier — and returns the matrix, vector or
// scalar the query resolved to.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func metricsQueryRange(ctx context.Context, in *O11yMetricsQueryRangeIn) (*O11yMetricsQueryRangeOut, error) {
	out := new(O11yMetricsQueryRangeOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/query_range", query(
		"start", in.Start,
		"end", in.End,
		"step", in.Step,
		"query", in.Query,
		"stats", in.Stats,
		"timeout", in.Timeout,
	), nil, out)
}

// queryRangeFormat parses a builder query and echoes it back normalized to the
// v3 shape — the endpoint the UI uses to canonicalize a query without running
// it.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func queryRangeFormat(ctx context.Context, in *v3.QueryRangeParamsV3) (*O11yQueryRangeFormatOut, error) {
	out := new(O11yQueryRangeFormatOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/query_range/format", nil, in, out)
}

// ── attribute autocomplete ─────────────────────────────────────────────────────

// autocompleteAggregate lists the attributes usable as an aggregate target for
// the given telemetry and operator — what a filter builder offers after the
// aggregation is chosen.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func autocompleteAggregate(ctx context.Context, in *O11yAggregateAttributesIn) (*O11yAggregateAttributesOut, error) {
	out := new(O11yAggregateAttributesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/autocomplete/aggregate_attributes", query(
		"aggregateOperator", in.AggregateOperator,
		"dataSource", in.DataSource,
		"searchText", in.SearchText,
		"limit", in.Limit,
	), nil, out)
}

// autocompleteKeys lists the attribute keys available for filtering the given
// telemetry, each with its data type and whether it is a materialized column.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func autocompleteKeys(ctx context.Context, in *O11yAttributeKeysIn) (*O11yAttributeKeysOut, error) {
	out := new(O11yAttributeKeysOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/autocomplete/attribute_keys", query(
		"dataSource", in.DataSource,
		"aggregateOperator", in.AggregateOperator,
		"aggregateAttribute", in.AggregateAttribute,
		"searchText", in.SearchText,
		"tagType", in.TagType,
		"limit", in.Limit,
	), nil, out)
}

// autocompleteValues lists the values one attribute key has taken — string,
// number and bool values in their own lists — for completing a filter.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func autocompleteValues(ctx context.Context, in *O11yAttributeValuesIn) (*O11yAttributeValuesOut, error) {
	out := new(O11yAttributeValuesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/autocomplete/attribute_values", query(
		"dataSource", in.DataSource,
		"aggregateOperator", in.AggregateOperator,
		"aggregateAttribute", in.AggregateAttribute,
		"attributeKey", in.AttributeKey,
		"filterAttributeKeyDataType", in.FilterAttributeKeyDataType,
		"searchText", in.SearchText,
		"tagType", in.TagType,
		"limit", in.Limit,
	), nil, out)
}

// autocompleteValuesPost reads the attribute-value request from the body rather
// than off the query string — the spelling the newer builder uses to send its
// filters alongside the request.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func autocompleteValuesPost(ctx context.Context, in *v3.FilterAttributeValueRequest) (*O11yAttributeValuesOut, error) {
	out := new(O11yAttributeValuesOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/auto_complete/attribute_values", nil, in, out)
}

// ── the field catalog ──────────────────────────────────────────────────────────

// fieldKeys returns the telemetry field keys matching the selector — the
// signal's fields grouped by name, and whether the catalog is complete.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func fieldKeys(ctx context.Context, in *O11yFieldKeysIn) (*O11yFieldKeysOut, error) {
	out := new(O11yFieldKeysOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/fields/keys", query(
		"signal", in.Signal,
		"source", in.Source,
		"limit", in.Limit,
		"startUnixMilli", nanos(in.StartUnixMilli),
		"endUnixMilli", nanos(in.EndUnixMilli),
		"fieldContext", in.FieldContext,
		"fieldDataType", in.FieldDataType,
		"metricName", in.MetricName,
		"metricNamespace", in.MetricNamespace,
		"searchText", in.SearchText,
	), nil, out)
}

// fieldValues returns the values one telemetry field has taken — string, bool,
// number and related values — and whether the value list is complete.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func fieldValues(ctx context.Context, in *O11yFieldValuesIn) (*O11yFieldValuesOut, error) {
	out := new(O11yFieldValuesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/fields/values", query(
		"signal", in.Signal,
		"source", in.Source,
		"limit", in.Limit,
		"startUnixMilli", nanos(in.StartUnixMilli),
		"endUnixMilli", nanos(in.EndUnixMilli),
		"fieldContext", in.FieldContext,
		"fieldDataType", in.FieldDataType,
		"metricName", in.MetricName,
		"metricNamespace", in.MetricNamespace,
		"searchText", in.SearchText,
		"name", in.Name,
		"existingQuery", in.ExistingQuery,
	), nil, out)
}

// ── saved explorer views ───────────────────────────────────────────────────────

// savedViewList lists the caller's org's saved explorer views, optionally
// narrowed to one source page, name or category.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func savedViewList(ctx context.Context, in *O11ySavedViewListIn) (*O11ySavedViewListOut, error) {
	out := new(O11ySavedViewListOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/explorer/views", query(
		"sourcePage", in.SourcePage,
		"name", in.Name,
		"category", in.Category,
	), nil, out)
}

// savedViewCreate saves a new explorer view for the caller's org and returns its
// id.
//
// Callers need the editor role; the runtime's own gate enforces it.
func savedViewCreate(ctx context.Context, in *v3.SavedView) (*O11ySavedViewCreateOut, error) {
	out := new(O11ySavedViewCreateOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/explorer/views", nil, in, out)
}

// savedViewGet returns one saved explorer view by id.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func savedViewGet(ctx context.Context, in *O11ySavedViewRef) (*O11ySavedViewOut, error) {
	out := new(O11ySavedViewOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/explorer/views/"+in.ViewID, nil, nil, out)
}

// savedViewUpdate replaces one saved explorer view by id with the given view and
// echoes it back.
//
// Callers need the editor role; the runtime's own gate enforces it.
func savedViewUpdate(ctx context.Context, in *O11ySavedViewUpdateIn) (*O11ySavedViewOut, error) {
	out := new(O11ySavedViewOut)
	// The view id addresses the target from the URL; the body carries the new
	// view, relayed without the id so the URL stays the one authority.
	return out, relay(ctx, http.MethodPut, o11yRoot+"/explorer/views/"+in.ViewID, nil, &in.SavedView, out)
}

// savedViewDelete deletes one saved explorer view by id.
//
// Callers need the editor role; the runtime's own gate enforces it.
func savedViewDelete(ctx context.Context, in *O11ySavedViewRef) (*O11ySavedViewDeleteOut, error) {
	out := new(O11ySavedViewDeleteOut)
	return out, relay(ctx, http.MethodDelete, o11yRoot+"/explorer/views/"+in.ViewID, nil, nil, out)
}

// ── inputs ──────────────────────────────────────────────────────────────────────
//
// The query engine's three write-shaped ops take the runtime's OWN request types
// — qbtypes.QueryRangeRequest and v3.QueryRangeParamsV3 — verbatim: they are the
// canonical bodies the runtime already parses, so naming them here is DRY rather
// than a second definition free to drift. The GET reads carry a flat input whose
// json tags ARE the query keys the runtime reads, so zip binds the caller's query
// string onto the input and the op renders the same keys back onto the wire.

// O11yQueryRangePreviewIn is a dry-run request: the composite query in the body,
// plus the verbose flag off the query string.
type O11yQueryRangePreviewIn struct {
	// QueryRangeRequest is the composite query to render, embedded so the request
	// body stays the bare query the execute endpoint takes.
	qbtypes.QueryRangeRequest
	// Verbose selects the answer's depth. Empty or "true" renders the underlying
	// Datastore SQL with EXPLAIN and granule analysis; "false" returns only the
	// per-query valid/error/warnings verdict with no Datastore round trips.
	Verbose string `json:"verbose,omitempty"`
}

// O11yMetricsQueryRangeIn selects a Prometheus-style metrics range query. Every
// type here carries the O11y qualifier because these names enter a fleet-wide
// document, where one name may have exactly one shape.
type O11yMetricsQueryRangeIn struct {
	// Start is the window start — a unix timestamp (seconds, with optional
	// fraction) or an RFC 3339 time. Required.
	Start string `json:"start" validate:"required"`
	// End is the window end, in the same form as Start, and not before it.
	// Required.
	End string `json:"end" validate:"required"`
	// Step is the query resolution, e.g. 60s, 1m, 1h — a positive duration.
	// Required.
	Step string `json:"step" validate:"required"`
	// Query is the PromQL expression to evaluate. Required.
	Query string `json:"query" validate:"required"`
	// Stats, when "all", asks for query statistics alongside the result.
	Stats string `json:"stats,omitempty"`
	// Timeout caps how long the query may run, e.g. 30s, 1m — a positive
	// duration. Absent means the server default.
	Timeout string `json:"timeout,omitempty"`
}

// O11yAggregateAttributesIn selects which aggregate-target attributes to list.
type O11yAggregateAttributesIn struct {
	// DataSource is the telemetry the attributes come from — traces, logs,
	// metrics or meter. The runtime requires it.
	DataSource string `json:"dataSource"`
	// AggregateOperator is the aggregation the attribute will be used under, e.g.
	// count, avg, sum. The runtime requires it for non-metrics sources.
	AggregateOperator string `json:"aggregateOperator"`
	// SearchText narrows the attributes to those containing it.
	SearchText string `json:"searchText"`
	// Limit caps how many attributes come back. Absent means 50.
	Limit int `json:"limit"`
}

// O11yAttributeKeysIn selects which attribute keys to list.
type O11yAttributeKeysIn struct {
	// DataSource is the telemetry the keys come from — traces, logs, metrics or
	// meter. The runtime requires it.
	DataSource string `json:"dataSource"`
	// AggregateOperator is the aggregation the keys will be used under. The
	// runtime requires it for non-metrics sources.
	AggregateOperator string `json:"aggregateOperator"`
	// AggregateAttribute is the metric the keys must appear on.
	AggregateAttribute string `json:"aggregateAttribute"`
	// SearchText narrows the keys to those containing it.
	SearchText string `json:"searchText"`
	// TagType narrows the keys to one kind — tag or resource. Empty means all;
	// an invalid value reads as empty.
	TagType string `json:"tagType"`
	// Limit caps how many keys come back. Absent means 50.
	Limit int `json:"limit"`
}

// O11yAttributeValuesIn selects which values of one attribute key to list.
type O11yAttributeValuesIn struct {
	// DataSource is the telemetry the values come from — traces, logs or
	// metrics. The runtime requires it.
	DataSource string `json:"dataSource"`
	// AggregateOperator is the aggregation the values will be used under. The
	// runtime requires it for non-metrics sources.
	AggregateOperator string `json:"aggregateOperator"`
	// AggregateAttribute is the metric the values must appear on.
	AggregateAttribute string `json:"aggregateAttribute"`
	// AttributeKey is the key whose values to list.
	AttributeKey string `json:"attributeKey"`
	// FilterAttributeKeyDataType is the key's data type — string, int64, float64
	// or bool. Empty means unspecified.
	FilterAttributeKeyDataType string `json:"filterAttributeKeyDataType"`
	// SearchText narrows the values to those containing it.
	SearchText string `json:"searchText"`
	// TagType narrows the search to one kind of key — tag or resource.
	TagType string `json:"tagType"`
	// Limit caps how many values come back. Absent means 50.
	Limit int `json:"limit"`
}

// O11yFieldKeysIn selects which telemetry field keys to list. Its json tags are
// the query keys the field catalog reads.
type O11yFieldKeysIn struct {
	// Signal is the telemetry to read the fields of — traces, logs or metrics.
	Signal string `json:"signal"`
	// Source narrows the fields to one source within the signal.
	Source string `json:"source"`
	// Limit caps how many keys come back.
	Limit int `json:"limit"`
	// StartUnixMilli is the window start as a unix millisecond epoch. Zero reads
	// as unset.
	StartUnixMilli int64 `json:"startUnixMilli"`
	// EndUnixMilli is the window end as a unix millisecond epoch. Zero reads as
	// unset.
	EndUnixMilli int64 `json:"endUnixMilli"`
	// FieldContext narrows the keys to one context — resource, scope, attribute,
	// span, log or metric.
	FieldContext string `json:"fieldContext"`
	// FieldDataType narrows the keys to one data type.
	FieldDataType string `json:"fieldDataType"`
	// MetricName narrows the keys to those on one metric.
	MetricName string `json:"metricName"`
	// MetricNamespace narrows the keys to one metric namespace.
	MetricNamespace string `json:"metricNamespace"`
	// SearchText narrows the keys to those containing it.
	SearchText string `json:"searchText"`
}

// O11yFieldValuesIn selects which telemetry field values to list. It carries the
// field-keys selector plus the field to read and the query it appears in.
type O11yFieldValuesIn struct {
	// Signal is the telemetry to read the field of — traces, logs or metrics.
	Signal string `json:"signal"`
	// Source narrows the field to one source within the signal.
	Source string `json:"source"`
	// Limit caps how many values come back.
	Limit int `json:"limit"`
	// StartUnixMilli is the window start as a unix millisecond epoch. Zero reads
	// as unset.
	StartUnixMilli int64 `json:"startUnixMilli"`
	// EndUnixMilli is the window end as a unix millisecond epoch. Zero reads as
	// unset.
	EndUnixMilli int64 `json:"endUnixMilli"`
	// FieldContext narrows the field to one context.
	FieldContext string `json:"fieldContext"`
	// FieldDataType narrows the field to one data type.
	FieldDataType string `json:"fieldDataType"`
	// MetricName narrows the field to one metric.
	MetricName string `json:"metricName"`
	// MetricNamespace narrows the field to one metric namespace.
	MetricNamespace string `json:"metricNamespace"`
	// SearchText narrows the values to those containing it.
	SearchText string `json:"searchText"`
	// Name is the field whose values to read.
	Name string `json:"name"`
	// ExistingQuery is the query the field appears in, so related values can be
	// suggested for it.
	ExistingQuery string `json:"existingQuery"`
}

// O11ySavedViewListIn narrows the saved-view listing.
type O11ySavedViewListIn struct {
	// SourcePage narrows the views to one source page, e.g. traces, logs.
	SourcePage string `json:"sourcePage"`
	// Name narrows the views to one name.
	Name string `json:"name"`
	// Category narrows the views to one category.
	Category string `json:"category"`
}

// O11ySavedViewRef names one saved explorer view to read or delete.
type O11ySavedViewRef struct {
	// ViewID is the view's id.
	ViewID string `json:"viewId" validate:"required"`
}

// O11ySavedViewUpdateIn is one view update: the id from the URL and the new view
// in the body.
type O11ySavedViewUpdateIn struct {
	// SavedView is the new view content, embedded so the request body stays the
	// bare view the create endpoint takes.
	v3.SavedView
	// ViewID is the id of the view to replace, taken from the URL.
	ViewID string `json:"viewId"`
}

// ── outputs ──────────────────────────────────────────────────────────────────────
//
// Each Out names the runtime's answer: the {status, data} envelope every read
// and write on this face has always answered with (render.Success and package
// app's Respond write the same shape), around the runtime's own response type.
// Status is declared first because the runtime writes it first, and a typed op
// that reorders or drops a byte is a break, not a migration.
//
// Where the runtime's answer is a concrete type it is REUSED as Data — DRY, and
// byte-identical because the bytes were marshaled from that same type. Where the
// answer is polymorphic — the query engine's result carries a result set that is
// one of time-series, scalar or raw, PromQL's value is one of matrix/vector/
// scalar, a preview embeds a rendered-error union — Data is a json.RawMessage
// that carries those bytes VERBATIM, so the op re-answers exactly what the
// runtime wrote rather than round-tripping a union through a Go interface and
// re-sorting its keys.

// O11yQueryRangeOut is a composite query result. Data is verbatim: its result set
// is a union of time-series, scalar and raw shapes the request type chooses.
type O11yQueryRangeOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the query result — the result type, the result set, execution stats
	// and any warning — as the engine rendered it.
	Data json.RawMessage `json:"data,omitempty"`
}

// O11yQueryRangePreviewOut is a dry-run result. Data is verbatim: each query's
// entry carries a rendered-error union the runtime encodes.
type O11yQueryRangePreviewOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds one preview per query, keyed by query name — the rendered
	// statements with optional EXPLAIN and granule analysis, or the per-query
	// verdict.
	Data json.RawMessage `json:"data,omitempty"`
}

// O11ySubstituteVarsOut is a variable-substituted query. Data is verbatim: it is
// a composite query, whose builder queries are a polymorphic set.
type O11ySubstituteVarsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the resolved composite query, with its variables substituted.
	Data json.RawMessage `json:"data,omitempty"`
}

// O11yMetricsQueryRangeOut is a Prometheus-style range result. Data is verbatim:
// the value is a matrix, a vector or a scalar.
type O11yMetricsQueryRangeOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the result — its result type, the value, and query stats when
	// asked.
	Data json.RawMessage `json:"data,omitempty"`
}

// O11yQueryRangeFormatOut is a normalized builder query. Data is verbatim: a
// composite query whose queries are a polymorphic set.
type O11yQueryRangeFormatOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the query, normalized to the v3 shape.
	Data json.RawMessage `json:"data,omitempty"`
}

// O11yAggregateAttributesOut is a page of aggregate-target attributes.
type O11yAggregateAttributesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the attribute keys.
	Data v3.AggregateAttributeResponse `json:"data,omitempty"`
}

// O11yAttributeKeysOut is a page of attribute keys.
type O11yAttributeKeysOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the attribute keys.
	Data v3.FilterAttributeKeyResponse `json:"data,omitempty"`
}

// O11yAttributeValuesOut is a page of one attribute key's values.
type O11yAttributeValuesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the values, split by data type.
	Data v3.FilterAttributeValueResponse `json:"data,omitempty"`
}

// O11yFieldKeysOut is a page of telemetry field keys.
type O11yFieldKeysOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the field keys grouped by name, and whether the catalog is
	// complete.
	Data telemetrytypes.GettableFieldKeys `json:"data,omitempty"`
}

// O11yFieldValuesOut is a page of one telemetry field's values.
type O11yFieldValuesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the values by data type, and whether the value list is complete.
	Data telemetrytypes.GettableFieldValues `json:"data,omitempty"`
}

// O11ySavedViewListOut is the saved explorer views. Data has no omitempty: the
// runtime answers a null data for no views and an empty array for none matched,
// and the op keeps that distinction.
type O11ySavedViewListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the views.
	Data []v3.SavedView `json:"data"`
}

// O11ySavedViewOut is one saved explorer view.
type O11ySavedViewOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the view.
	Data v3.SavedView `json:"data,omitempty"`
}

// O11ySavedViewCreateOut acknowledges a saved-view creation with its id.
type O11ySavedViewCreateOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the new view's id.
	Data valuer.UUID `json:"data,omitempty"`
}

// O11ySavedViewDeleteOut acknowledges a saved-view deletion.
type O11ySavedViewDeleteOut struct {
	// Status is "success".
	Status string `json:"status"`
}
