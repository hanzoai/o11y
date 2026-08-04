package o11y

// The TELEMETRY reads of the error-tracking product face — discover, logs,
// traces, one trace, stats — as TYPED ops.
//
// They were five of the routes that reached traffic only through the delegation
// wildcard, and a route behind a wildcard is in no document: no SDK method, no
// CLI command, no agent tool, no reference page. A customer could not reach
// their own error telemetry from anything but a hand-written HTTP call. Typing
// them is what puts the five operations in the document and therefore in every
// projection built from it.
//
// THE WIRE DOES NOT MOVE, and the way it does not move is that these ops do not
// re-implement the reads. Each hands the call to the SAME runtime handler the
// wildcard delegates to (see relay), so identity resolution, the org gate, the
// role check, the audit record, the timeout and the success envelope are all
// still the runtime's, executed in the order they always were. What is new here
// is the TYPE — the In the caller may send and the Out they get back — and the
// prose that goes with it.
//
// Registered ahead of the wildcard, and specific-beats-wildcard is what the
// router does regardless of registration order, so these five paths dispatch
// here and every other path under the prefix still reaches the runtime
// untouched (telemetry_test.go pins both halves).

//go:generate go run github.com/zap-proto/zip/cmd/zipdoc

import (
	"context"
	"net/http"
	"time"

	"github.com/zap-proto/zip"
)

// mountTelemetry registers the five typed telemetry ops on the native router.
// Collection routes register before the parameterised one so an id can never
// shadow a collection.
func mountTelemetry(app zip.Router) {
	g := app.Group(sentryRoot)
	zip.Post(g, "/discover", discover)
	zip.Get(g, "/logs", logs)
	zip.Get(g, "/traces", traces)
	zip.Get(g, "/traces/:id", trace)
	zip.Get(g, "/stats", stats)
}

// ── the five operations ───────────────────────────────────────────────────────

// discover aggregates a project's captured errors into a table — the caller
// names the filters, the groupings and the aggregations, and gets back the
// columns and rows they asked for.
//
// The project is mandatory and is checked against the caller's own org before it
// scopes anything, so a project id belonging to someone else reads as absent
// rather than as data.
func discover(ctx context.Context, in *O11yDiscoverIn) (*O11yDiscoverOut, error) {
	out := new(O11yDiscoverOut)
	return out, relay(ctx, http.MethodPost, sentryRoot+"/discover", nil, in, out)
}

// logs lists a project's captured error events, newest first, optionally
// narrowed to those whose message or exception text contains a search string.
func logs(ctx context.Context, in *O11yLogsIn) (*O11yLogsOut, error) {
	out := new(O11yLogsOut)
	return out, relay(ctx, http.MethodGet, sentryRoot+"/logs", query(
		"project", in.Project,
		"query", in.Query,
		"period", in.Period,
		"limit", in.Limit,
	), nil, out)
}

// traces lists the traces a project's captured errors reference, each with how
// many errors landed on it, when they started and stopped, and the latest
// message seen — the entry point for "which requests are failing".
func traces(ctx context.Context, in *O11yTracesIn) (*O11yTracesOut, error) {
	out := new(O11yTracesOut)
	return out, relay(ctx, http.MethodGet, sentryRoot+"/traces", query(
		"project", in.Project,
		"period", in.Period,
		"limit", in.Limit,
	), nil, out)
}

// trace returns one trace's captured errors for a project — every error event
// that carried the trace id, in the order the events plane holds them.
func trace(ctx context.Context, in *O11yTraceIn) (*O11yTraceOut, error) {
	out := new(O11yTraceOut)
	// The id goes on VERBATIM, as the segment the router matched. It arrived
	// percent-encoded or it did not, and re-encoding it here would hand the
	// runtime a different id than the caller named — for the one id spelling
	// that matters, an escaped slash, that is the difference between the answer
	// the face has always given and a new one.
	return out, relay(ctx, http.MethodGet, sentryRoot+"/traces/"+in.ID, query(
		"project", in.Project,
	), nil, out)
}

// stats returns a project's event-rate timeseries: one bucket per interval over
// the requested period, counting the events in it.
func stats(ctx context.Context, in *O11yStatsIn) (*O11yStatsOut, error) {
	out := new(O11yStatsOut)
	return out, relay(ctx, http.MethodGet, sentryRoot+"/stats", query(
		"project", in.Project,
		"field", in.Field,
		"period", in.Period,
	), nil, out)
}

// ── inputs ────────────────────────────────────────────────────────────────────

// O11yDiscoverIn is one aggregation over a project's captured errors. Every
// type here carries the O11y qualifier because these names enter a fleet-wide
// document, where one name may have exactly one shape.
type O11yDiscoverIn struct {
	// Project is the project to read, as its id. Required.
	Project string `json:"project" validate:"required"`
	// Filters narrow the scan; each is a field, an operator (eq, neq, like) and
	// a value, and they combine with AND.
	Filters []O11yFilter `json:"filters,omitempty"`
	// Aggregations are the measures to compute per group. Empty means a count.
	Aggregations []string `json:"aggregations,omitempty"`
	// GroupBy are the columns to group the rows by.
	GroupBy []string `json:"groupBy,omitempty"`
	// Period is the window to read, relative to now — 1h, 24h, 7d, 14d, 30d.
	Period string `json:"period,omitempty"`
	// OrderBy is the column or aggregation to sort the rows on.
	OrderBy string `json:"orderBy,omitempty"`
	// OrderDir is asc or desc.
	OrderDir string `json:"orderDir,omitempty"`
	// Limit caps how many rows come back.
	Limit int `json:"limit,omitempty"`
}

// O11yFilter is one predicate on a column of the events plane.
type O11yFilter struct {
	// Field is the column to test.
	Field string `json:"field"`
	// Op is how to test it: eq, neq or like.
	Op string `json:"op"`
	// Value is what to test it against.
	Value string `json:"value"`
}

// O11yLogsIn selects a page of a project's captured error events.
type O11yLogsIn struct {
	// Project is the project to read, as its id. Required.
	Project string `json:"project" validate:"required"`
	// Query narrows the page to events whose text contains it.
	Query string `json:"query,omitempty"`
	// Period is the window to read, relative to now — 1h, 24h, 7d, 14d, 30d.
	Period string `json:"period,omitempty"`
	// Limit caps how many events come back.
	Limit int `json:"limit,omitempty"`
}

// O11yTracesIn selects a page of a project's error-carrying traces.
type O11yTracesIn struct {
	// Project is the project to read, as its id. Required.
	Project string `json:"project" validate:"required"`
	// Period is the window to read, relative to now — 1h, 24h, 7d, 14d, 30d.
	Period string `json:"period,omitempty"`
	// Limit caps how many traces come back.
	Limit int `json:"limit,omitempty"`
}

// O11yTraceIn names one trace to read, within one project.
type O11yTraceIn struct {
	// ID is the trace id.
	ID string `json:"id" validate:"required"`
	// Project is the project the trace's errors belong to. Required.
	Project string `json:"project" validate:"required"`
}

// O11yStatsIn selects a project's event-rate timeseries.
type O11yStatsIn struct {
	// Project is the project to read, as its id. Required.
	Project string `json:"project" validate:"required"`
	// Field is the dimension to count over. Empty counts all events.
	Field string `json:"field,omitempty"`
	// Period is the window to read, relative to now — 1h, 24h, 7d, 14d, 30d.
	Period string `json:"period,omitempty"`
}

// ── outputs ───────────────────────────────────────────────────────────────────
//
// Each Out is the runtime's answer NAMED, field for field, tag for tag —
// including the status/data envelope every read on this face has always
// answered with. Nothing embeds: the schema builder walks reflected fields and
// cannot name an embedded one, so an embedded shape would publish a schema
// missing every field it carries. Written flat, the document and the bytes
// agree, and telemetry_test.go re-encodes each Out and compares it byte for byte
// with what the runtime wrote.

// O11yDiscoverOut is an aggregation result.
type O11yDiscoverOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the table.
	Data O11yTable `json:"data,omitempty"`
}

// O11yTable is a tabular result: named columns, and rows of values in column
// order.
type O11yTable struct {
	// Columns names each position in a row.
	Columns []string `json:"columns"`
	// Rows are the result rows, each as long as Columns.
	Rows [][]any `json:"rows"`
}

// O11yLogsOut is a page of captured error events.
type O11yLogsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the events.
	Data O11yEvents `json:"data,omitempty"`
}

// O11yEvents is a page of error events.
type O11yEvents struct {
	// Items are the events, newest first.
	Items []O11yEvent `json:"items"`
}

// O11yTracesOut is a page of error-carrying traces.
type O11yTracesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the traces.
	Data O11yTraces `json:"data,omitempty"`
}

// O11yTraces is a page of trace summaries.
type O11yTraces struct {
	// Items are the traces, most recent first.
	Items []O11yTrace `json:"items"`
}

// O11yTraceOut is one trace's captured errors.
type O11yTraceOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the trace and its events.
	Data O11yTraceDetail `json:"data,omitempty"`
}

// O11yTraceDetail is one trace and the error events that referenced it.
type O11yTraceDetail struct {
	// Events are the error events carrying the trace id.
	Events []O11yEvent `json:"events"`
	// TraceID is the trace that was read.
	TraceID string `json:"traceId"`
}

// O11yStatsOut is an event-rate timeseries.
type O11yStatsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the buckets.
	Data O11yStats `json:"data,omitempty"`
}

// O11yStats is a timeseries of event counts.
type O11yStats struct {
	// Items are the buckets, oldest first.
	Items []O11yStat `json:"items"`
}

// O11yStat is one bucket of an event-rate timeseries.
type O11yStat struct {
	// Time is the start of the bucket.
	Time time.Time `json:"time"`
	// Value is how many events fell in it.
	Value uint64 `json:"value"`
}

// O11yTrace is a trace the project's errors referenced.
type O11yTrace struct {
	// TraceID is the trace id.
	TraceID string `json:"traceId"`
	// Count is how many captured errors carried it.
	Count uint64 `json:"count"`
	// FirstSeen is when the earliest of them was recorded.
	FirstSeen time.Time `json:"firstSeen"`
	// LastSeen is when the latest was.
	LastSeen time.Time `json:"lastSeen"`
	// Message is the latest error message seen on the trace.
	Message string `json:"message,omitempty"`
}

// O11yEvent is one captured error occurrence.
type O11yEvent struct {
	// OrgID is the org that owns it.
	OrgID string `json:"orgId"`
	// ProjectID is the project it was captured for.
	ProjectID string `json:"projectId"`
	// EventID is its own id.
	EventID string `json:"eventId"`
	// Timestamp is when the error happened, as the reporter recorded it.
	Timestamp time.Time `json:"timestamp"`
	// ReceivedAt is when it arrived here.
	ReceivedAt time.Time `json:"receivedAt"`
	// Level is its severity, e.g. error, warning, info.
	Level string `json:"level"`
	// Type is the exception type.
	Type string `json:"type"`
	// Value is the exception value.
	Value string `json:"value"`
	// Message is the human-readable message.
	Message string `json:"message"`
	// Culprit is where it came from — the function or route blamed for it.
	Culprit string `json:"culprit"`
	// Fingerprint is the grouping key that puts like errors in one issue.
	Fingerprint string `json:"fingerprint"`
	// Platform is the reporting runtime, e.g. go, python, javascript.
	Platform string `json:"platform,omitempty"`
	// Environment is the deployment it happened in.
	Environment string `json:"environment,omitempty"`
	// Release is the version that produced it.
	Release string `json:"release,omitempty"`
	// ServiceName is the service that reported it.
	ServiceName string `json:"serviceName,omitempty"`
	// Transaction is the operation it happened in.
	Transaction string `json:"transaction,omitempty"`
	// TraceID is the trace it belonged to.
	TraceID string `json:"traceId,omitempty"`
	// SpanID is the span it belonged to.
	SpanID string `json:"spanId,omitempty"`
	// ServerName is the host that reported it.
	ServerName string `json:"serverName,omitempty"`
	// UserID identifies the affected end user, when the reporter attached one.
	UserID string `json:"userId,omitempty"`
	// UserEmail is that user's email, when attached.
	UserEmail string `json:"userEmail,omitempty"`
	// UserIP is that user's address, when attached.
	UserIP string `json:"userIp,omitempty"`
	// Handled says whether the application caught it.
	Handled bool `json:"handled"`
	// Tags are the reporter's own key/value labels.
	Tags map[string]string `json:"tags,omitempty"`
	// Frames are the stack, innermost first.
	Frames []O11yFrame `json:"frames,omitempty"`
}

// O11yFrame is one stack frame of a captured error.
type O11yFrame struct {
	// Function is the function the frame is in.
	Function string `json:"function,omitempty"`
	// File is the file it is in.
	File string `json:"file,omitempty"`
	// Line is the line number.
	Line uint32 `json:"line,omitempty"`
	// Column is the column number.
	Column uint32 `json:"column,omitempty"`
	// Own marks a frame in the reporting application's own code rather than in a
	// dependency or the runtime.
	Own bool `json:"own,omitempty"`
}

// ── the seam ──────────────────────────────────────────────────────────────────
