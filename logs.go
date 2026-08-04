package o11y

// The LOGS product face — the recent-record read, the field catalog and its
// tuning write, the aggregate read, log parsing pipelines (preview, list,
// apply) and body-path promotion — as TYPED ops.
//
// These were nine of the routes that reached traffic only through the
// delegation wildcard, and a route behind a wildcard is in no document: no SDK
// method, no CLI command, no agent tool, no reference page. A customer could
// not reach their own logs from anything but a hand-written HTTP call. Typing
// them is what puts the nine operations in the document and therefore in every
// projection built from it.
//
// THE WIRE DOES NOT MOVE, and the way it does not move is that these ops do not
// re-implement the reads and writes. Each hands the call to the SAME runtime
// handler the wildcard delegates to (see relay.go), so identity resolution, the
// org gate, the role check (viewer on the reads, editor on the writes — the
// exact ViewAccess/EditAccess gates the runtime has always run), the audit
// record and the success envelope are all still the runtime's, executed in the
// order they always were. What is new here is the TYPE — the In the caller may
// send and the Out they get back — and the prose that goes with it.
//
// The TENTH route of this face, GET /v1/o11y/logs/livetail, is deliberately NOT
// typed: it is a live stream that never completes, and the relay buffers a
// whole answer before decoding it, so a typed livetail would hang forever on
// the first tail. It stays on the delegation wildcard, byte-identical to what
// it always was (logs_test.go pins that), and this comment is its escape-hatch
// record.
//
// Registered ahead of the wildcard, and specific-beats-wildcard is what the
// router does regardless of registration order, so these nine paths dispatch
// here and every other path under the prefix — livetail included — still
// reaches the runtime untouched (logs_test.go pins both halves).

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/zap-proto/zip"
)

// mountLogs registers the nine typed logs ops on the native router. The
// pipelines routes keep the mux tree's discipline — preview registers before
// the parameterised version read so a version can never shadow it.
func mountLogs(app *zip.App) {
	g := under{app, o11yRoot}
	opGet(g, "/logs", logRecords)
	opGet(g, "/logs/fields", logFields)
	opPost(g, "/logs/fields", logFieldUpdate)
	opGet(g, "/logs/aggregate", logAggregate)
	opPost(g, "/logs/pipelines/preview", logPipelinePreview)
	opGet(g, "/logs/pipelines/:version", logPipelines)
	opPost(g, "/logs/pipelines", logPipelineCreate)
	opPost(g, "/logs/promote_paths", logPromote, zip.WithStatus(http.StatusCreated))
	opGet(g, "/logs/promote_paths", logPromoted)
}

// ── the nine operations ───────────────────────────────────────────────────────

// logRecords returns the most recent log records in the query window, newest
// first — each record an open object carrying its nanosecond timestamp and
// whatever fields the record was ingested with.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func logRecords(ctx context.Context, in *O11yLogRecordsIn) (*O11yLogRecordsOut, error) {
	out := new(O11yLogRecordsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/logs", query(
		"limit", in.Limit,
		"timestampStart", nanos(in.TimestampStart),
		"timestampEnd", nanos(in.TimestampEnd),
	), nil, out)
}

// logFields returns the log field catalog: the fields already selected as
// indexed columns, and the interesting ones seen in the data that could be.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func logFields(ctx context.Context, _ *struct{}) (*O11yFieldCatalogOut, error) {
	out := new(O11yFieldCatalogOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/logs/fields", nil, nil, out)
}

// logFieldUpdate changes how one log field is stored — selects or deselects it
// as a materialized column and tunes its index — and echoes the setting back.
//
// Callers need the editor role; the runtime's own gate enforces it.
func logFieldUpdate(ctx context.Context, in *O11yFieldSetting) (*O11yFieldSetting, error) {
	out := new(O11yFieldSetting)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/logs/fields", nil, in, out)
}

// logAggregate returns the logs aggregate buckets for the query window. The
// runtime currently answers the empty set; the shape is the contract.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func logAggregate(ctx context.Context, _ *struct{}) (*O11yLogAggregateOut, error) {
	out := new(O11yLogAggregateOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/logs/aggregate", nil, nil, out)
}

// logPipelinePreview runs the given log parsing pipelines over the given
// sample records without saving anything, and returns the transformed records
// plus whatever the collector logged while simulating them.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func logPipelinePreview(ctx context.Context, in *O11yLogPipelinePreviewIn) (*O11yLogPipelinePreviewOut, error) {
	out := new(O11yLogPipelinePreviewOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/logs/pipelines/preview", nil, in, out)
}

// logPipelines returns the caller's org's log parsing pipelines at one config
// version — "latest" for the newest — along with that version's deployment
// record and the recent version history.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func logPipelines(ctx context.Context, in *O11yLogPipelinesIn) (*O11yLogPipelinesOut, error) {
	out := new(O11yLogPipelinesOut)
	// The version goes on VERBATIM, as the segment the router matched, so the
	// runtime reads exactly the version the caller named.
	return out, relay(ctx, http.MethodGet, o11yRoot+"/logs/pipelines/"+in.Version, nil, nil, out)
}

// logPipelineCreate saves the given log parsing pipelines as the new config
// version for the caller's org and starts deploying it. The set REPLACES the
// current one: a pipeline left out of the request is dropped from the new
// version, and an empty set drops them all.
//
// Callers need the editor role; the runtime's own gate enforces it.
func logPipelineCreate(ctx context.Context, in *O11yLogPipelineCreateIn) (*O11yLogPipelinesOut, error) {
	out := new(O11yLogPipelinesOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/logs/pipelines", nil, in, out)
}

// logPromote promotes and indexes log body paths: each named path is lifted
// out of the JSON body into its own column, with the indexes the caller asked
// for. Paths must start with "body.".
//
// Callers need the editor role; the runtime's own gate enforces it.
func logPromote(ctx context.Context, in *O11yLogPromoteIn) (*O11yLogPromoteOut, error) {
	out := new(O11yLogPromoteOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/logs/promote_paths", nil, in, out)
}

// logPromoted lists the log body paths already promoted or indexed, with the
// indexes each carries.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func logPromoted(ctx context.Context, _ *struct{}) (*O11yLogPromotedOut, error) {
	out := new(O11yLogPromotedOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/logs/promote_paths", nil, nil, out)
}

// ── inputs ────────────────────────────────────────────────────────────────────

// O11yLogRecordsIn selects a window of recent log records. Every type here
// carries the O11yLog qualifier because these names enter a fleet-wide
// document, where one name may have exactly one shape.
type O11yLogRecordsIn struct {
	// Limit caps how many records come back. Zero means the default of 100.
	Limit int `json:"limit,omitempty"`
	// TimestampStart is the start of the window as a nanosecond epoch. Zero
	// means fifteen minutes before the end.
	TimestampStart int64 `json:"timestampStart,omitempty"`
	// TimestampEnd is the end of the window as a nanosecond epoch. Zero means
	// now.
	TimestampEnd int64 `json:"timestampEnd,omitempty"`
}

// O11yFieldSetting is one telemetry field's storage setting — the request a
// caller sends to change it, and the echo they get back. One name because it is
// one shape: the write is acknowledged with the setting it wrote. The logs face
// and the traces face tune the SAME catalog through it (see traces.go); the
// runtime has always used one type for both, so naming it per-signal here would
// have invented two names for one value.
type O11yFieldSetting struct {
	// Name is the field to tune. Required.
	Name string `json:"name" validate:"required"`
	// DataType is the field's data type, e.g. string, int64, float64, bool.
	// Required.
	DataType string `json:"dataType" validate:"required"`
	// Type is where the field lives: attributes or resources. Required.
	Type string `json:"type" validate:"required"`
	// Selected materializes the field as its own column when true.
	Selected bool `json:"selected"`
	// Index is the index expression to put on the column, e.g. minmax,
	// set(N), bloom_filter(P), tokenbf_v1(S,H,SEED). Empty keeps the default.
	Index string `json:"index"`
	// IndexGranularity is the index granularity in rows.
	IndexGranularity int `json:"indexGranularity"`
}

// O11yLogPipelinePreviewIn is one dry run: the pipelines to simulate and the
// sample records to run them over.
type O11yLogPipelinePreviewIn struct {
	// Pipelines are the pipelines to simulate, in order.
	Pipelines []O11yLogPipeline `json:"pipelines"`
	// Logs are the sample records to transform.
	Logs []O11yLogRecord `json:"logs"`
}

// O11yLogPipelinesIn names one pipelines config version to read.
type O11yLogPipelinesIn struct {
	// Version is the config version to read — a positive number, or "latest".
	Version string `json:"version" validate:"required"`
}

// O11yLogPipelineCreateIn is the full pipeline set to save as the new config
// version. It replaces the current set.
type O11yLogPipelineCreateIn struct {
	// Pipelines are the pipelines the new version holds, in order.
	Pipelines []O11yLogPostablePipeline `json:"pipelines"`
}

// O11yLogPromoteIn is the set of body paths to promote and index. The request
// body is this array itself, exactly as the face has always read it.
type O11yLogPromoteIn []O11yLogPromotePath

// O11yLogPostablePipeline is one pipeline as a caller writes it.
type O11yLogPostablePipeline struct {
	// ID is the pipeline's id. Empty on a new pipeline; the id it was listed
	// with to keep an existing one.
	ID string `json:"id"`
	// OrderID is the pipeline's 1-based position in the set.
	OrderID int `json:"orderId"`
	// Name is the pipeline's display name.
	Name string `json:"name"`
	// Alias is the pipeline's short name.
	Alias string `json:"alias"`
	// Description says what the pipeline is for.
	Description string `json:"description"`
	// Enabled turns the pipeline on.
	Enabled bool `json:"enabled"`
	// Filter selects which records the pipeline processes.
	Filter *O11yLogFilter `json:"filter"`
	// Config is the pipeline's processors, in order.
	Config []O11yLogPipelineOperator `json:"config"`
}

// ── outputs ───────────────────────────────────────────────────────────────────
//
// Each Out is the runtime's answer NAMED, field for field, tag for tag —
// including the envelope each route has always answered with: the record,
// field and aggregate reads answer their payload bare, the pipeline and
// promotion routes wrap theirs in the status/data envelope. Nothing embeds:
// the schema builder walks reflected fields and cannot name an embedded one.
// Written flat, the document and the bytes agree, and logs_test.go re-encodes
// each Out and compares it byte for byte with what the runtime wrote.

// O11yLogRecordsOut is a window of recent log records.
type O11yLogRecordsOut struct {
	// Results are the records, newest first.
	Results []O11yLogRow `json:"results"`
}

// O11yLogRow is one stored log record: its nanosecond timestamp plus whatever
// fields it was ingested with. Values stay exactly as stored — a log record is
// an open object, and renaming or renumbering its fields here would lie about
// the data.
type O11yLogRow map[string]json.RawMessage

// O11yFieldCatalogOut is a signal's field catalog — logs (here) and traces
// (traces.go) both answer with it, because on the runtime they are one type.
type O11yFieldCatalogOut struct {
	// Selected are the fields materialized as their own columns.
	Selected []O11yTelemetryField `json:"selected"`
	// Interesting are fields seen in the data that could be selected.
	Interesting []O11yTelemetryField `json:"interesting"`
}

// O11yTelemetryField is one field of a signal's field catalog.
type O11yTelemetryField struct {
	// Name is the field's name.
	Name string `json:"name"`
	// DataType is the field's data type, e.g. string, int64, float64, bool.
	DataType string `json:"dataType"`
	// Type is where the field lives: attributes or resources.
	Type string `json:"type"`
}

// O11yLogAggregateOut is the logs aggregate answer.
type O11yLogAggregateOut struct {
	// Items are the buckets, keyed by bucket timestamp.
	Items map[int64]O11yLogAggregateBucket `json:"items"`
}

// O11yLogAggregateBucket is one aggregate bucket.
type O11yLogAggregateBucket struct {
	// Timestamp is the start of the bucket.
	Timestamp int64 `json:"timestamp,omitempty"`
	// Value is the bucket's aggregated value.
	Value json.RawMessage `json:"value,omitempty"`
	// GroupBy carries the group's key values when the aggregate grouped.
	GroupBy map[string]json.RawMessage `json:"groupBy,omitempty"`
}

// O11yLogPipelinePreviewOut is a dry run's answer.
type O11yLogPipelinePreviewOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the dry run's result.
	Data O11yLogPipelinePreview `json:"data,omitempty"`
}

// O11yLogPipelinePreview is what a dry run produced.
type O11yLogPipelinePreview struct {
	// Logs are the sample records after the pipelines ran over them.
	Logs []O11yLogRecord `json:"logs"`
	// CollectorLogs is what the collector logged while simulating.
	CollectorLogs []string `json:"collectorLogs"`
}

// O11yLogPipelinesOut is a pipelines config version — the answer to both the
// read and the save, because the save answers with the version it created.
type O11yLogPipelinesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the config version.
	Data O11yLogPipelines `json:"data,omitempty"`
}

// O11yLogPipelines is one pipelines config version: the version's deployment
// record (its fields appear together when the version exists, and not at all
// before any version has been saved), the pipelines it holds, and the recent
// version history.
type O11yLogPipelines struct {
	// CreatedByName is the display name of who created the version.
	CreatedByName *string `json:"createdByName,omitempty"`
	// ID is the version record's id.
	ID *string `json:"id,omitempty"`
	// CreatedAt is when the version was created.
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	// UpdatedAt is when the version last changed.
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	// CreatedBy is the id of who created the version.
	CreatedBy *string `json:"createdBy,omitempty"`
	// UpdatedBy is the id of who last changed it.
	UpdatedBy *string `json:"updatedBy,omitempty"`
	// OrgID is the org the version belongs to.
	OrgID *string `json:"orgId,omitempty"`
	// Version is the config version number.
	Version *int `json:"version,omitempty"`
	// ElementType is the config element this version carries — log_pipelines.
	ElementType *string `json:"elementType,omitempty"`
	// DeployStatus is where the deployment stands, e.g. dirty, deploying,
	// deployed, in_progress, failed, unknown.
	DeployStatus *string `json:"deployStatus,omitempty"`
	// DeploySequence orders this deployment among the version's deployments.
	DeploySequence *int `json:"deploySequence,omitempty"`
	// DeployResult is the deployment's outcome message.
	DeployResult *string `json:"deployResult,omitempty"`
	// LastHash is the deployed config's hash.
	LastHash *string `json:"lastHash,omitempty"`
	// Config is the rendered collector config the version deployed.
	Config *string `json:"config,omitempty"`
	// Pipelines are the version's pipelines, in order.
	Pipelines []O11yLogPipeline `json:"pipelines"`
	// History is the recent version history, newest first.
	History []O11yLogConfigVersion `json:"history"`
}

// O11yLogConfigVersion is one entry of the pipelines version history.
type O11yLogConfigVersion struct {
	// CreatedByName is the display name of who created the version.
	CreatedByName string `json:"createdByName"`
	// ID is the version record's id.
	ID string `json:"id"`
	// CreatedAt is when the version was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the version last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// CreatedBy is the id of who created the version.
	CreatedBy string `json:"createdBy"`
	// UpdatedBy is the id of who last changed it.
	UpdatedBy string `json:"updatedBy"`
	// OrgID is the org the version belongs to.
	OrgID string `json:"orgId"`
	// Version is the config version number.
	Version int `json:"version"`
	// ElementType is the config element the version carries — log_pipelines.
	ElementType string `json:"elementType"`
	// DeployStatus is where the deployment stands, e.g. dirty, deploying,
	// deployed, in_progress, failed, unknown.
	DeployStatus string `json:"deployStatus"`
	// DeploySequence orders this deployment among the version's deployments.
	DeploySequence int `json:"deploySequence"`
	// DeployResult is the deployment's outcome message.
	DeployResult string `json:"deployResult"`
	// LastHash is the deployed config's hash.
	LastHash string `json:"lastHash"`
	// Config is the rendered collector config the version deployed.
	Config string `json:"config"`
}

// O11yLogPipeline is one log parsing pipeline as the runtime stores it.
type O11yLogPipeline struct {
	// CreatedBy is the id of who created the pipeline.
	CreatedBy string `json:"createdBy"`
	// UpdatedBy is the id of who last changed it.
	UpdatedBy string `json:"updatedBy"`
	// CreatedAt is when the pipeline was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the pipeline last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// ID is the pipeline's id.
	ID string `json:"id"`
	// OrderID is the pipeline's 1-based position in the set.
	OrderID int `json:"orderId"`
	// Enabled says whether the pipeline is on.
	Enabled bool `json:"enabled"`
	// Name is the pipeline's display name.
	Name string `json:"name"`
	// Alias is the pipeline's short name.
	Alias string `json:"alias"`
	// Description says what the pipeline is for.
	Description string `json:"description"`
	// Filter selects which records the pipeline processes.
	Filter *O11yLogFilter `json:"filter"`
	// Config is the pipeline's processors, in order.
	Config []O11yLogPipelineOperator `json:"config"`
}

// O11yLogFilter selects log records: predicates over their fields, combined
// with the operator.
type O11yLogFilter struct {
	// Op combines the items: AND or OR.
	Op string `json:"op,omitempty"`
	// Items are the predicates.
	Items []O11yLogFilterItem `json:"items"`
}

// O11yLogFilterItem is one predicate over one log field.
type O11yLogFilterItem struct {
	// Key is the field the predicate tests.
	Key O11yLogFilterKey `json:"key"`
	// Value is what it tests against, in the value's own JSON type.
	Value json.RawMessage `json:"value"`
	// Op is the comparison, e.g. =, !=, in, contains.
	Op string `json:"op"`
}

// O11yLogFilterKey names one log field for a filter.
type O11yLogFilterKey struct {
	// Key is the field's name.
	Key string `json:"key"`
	// DataType is the field's data type, e.g. string, int64, float64, bool.
	DataType string `json:"dataType"`
	// Type is where the field lives: tag or resource.
	Type string `json:"type"`
	// IsColumn marks a field materialized as its own column.
	IsColumn bool `json:"isColumn"`
	// IsJSON marks a path into the record's JSON body.
	IsJSON bool `json:"isJSON"`
}

// O11yLogPipelineOperator is one processor of a pipeline. Which keys apply
// depends on Type; the runtime validates the combination.
type O11yLogPipelineOperator struct {
	// Type is the processor type, e.g. grok_parser, regex_parser, json_parser,
	// trace_parser, time_parser, severity_parser, add, remove, move, copy.
	Type string `json:"type"`
	// ID is the processor's id, unique within the pipeline.
	ID string `json:"id,omitempty"`
	// Output is the id of the processor that runs next.
	Output string `json:"output,omitempty"`
	// OnError says what to do when the processor fails, e.g. send, drop.
	OnError string `json:"on_error,omitempty"`
	// If gates the processor on an expression.
	If string `json:"if,omitempty"`
	// OrderID is the processor's 1-based position in the pipeline.
	OrderID int `json:"orderId"`
	// Enabled turns the processor on.
	Enabled bool `json:"enabled"`
	// Name is the processor's display name.
	Name string `json:"name,omitempty"`
	// ParseTo is where a parser writes its result.
	ParseTo string `json:"parse_to,omitempty"`
	// Pattern is a grok parser's pattern.
	Pattern string `json:"pattern,omitempty"`
	// Regex is a regex parser's expression.
	Regex string `json:"regex,omitempty"`
	// ParseFrom is where a parser reads from.
	ParseFrom string `json:"parse_from,omitempty"`
	// TraceID says where a trace parser reads the trace id from.
	TraceID *O11yLogParseFrom `json:"trace_id,omitempty"`
	// SpanID says where a trace parser reads the span id from.
	SpanID *O11yLogParseFrom `json:"span_id,omitempty"`
	// TraceFlags says where a trace parser reads the trace flags from.
	TraceFlags *O11yLogParseFrom `json:"trace_flags,omitempty"`
	// Field is the field an add/remove processor works on.
	Field string `json:"field,omitempty"`
	// Value is the value an add processor writes.
	Value string `json:"value,omitempty"`
	// From is the source field of a move or copy.
	From string `json:"from,omitempty"`
	// To is the destination field of a move or copy.
	To string `json:"to,omitempty"`
	// Expr is a router route's expression.
	Expr string `json:"expr,omitempty"`
	// Routes are a router processor's routes.
	Routes *[]O11yLogPipelineRoute `json:"routes,omitempty"`
	// Fields are the fields a retain processor keeps.
	Fields []string `json:"fields,omitempty"`
	// Default is the id of the processor a router falls through to.
	Default string `json:"default,omitempty"`
	// Layout is a time parser's layout.
	Layout string `json:"layout,omitempty"`
	// LayoutType is the layout's kind, e.g. strptime, gotime, epoch.
	LayoutType string `json:"layout_type,omitempty"`
	// EnableFlattening flattens parsed JSON one level when true.
	EnableFlattening bool `json:"enable_flattening,omitempty"`
	// EnablePaths keeps the JSON path in flattened keys when true.
	EnablePaths bool `json:"enable_paths,omitempty"`
	// PathPrefix prefixes flattened keys.
	PathPrefix string `json:"path_prefix,omitempty"`
	// Mapping maps severity levels (or flattened keys) to the values that
	// mean them.
	Mapping map[string][]string `json:"mapping,omitempty"`
	// OverwriteSeverityText rewrites the severity text alongside the number
	// when true.
	OverwriteSeverityText bool `json:"overwrite_text,omitempty"`
}

// O11yLogParseFrom names the field a trace parser reads one value from.
type O11yLogParseFrom struct {
	// ParseFrom is the field to read.
	ParseFrom string `json:"parse_from"`
}

// O11yLogPipelineRoute is one route of a router processor.
type O11yLogPipelineRoute struct {
	// Output is the id of the processor the route sends to.
	Output string `json:"output"`
	// Expr is the expression that selects the route.
	Expr string `json:"expr"`
}

// O11yLogRecord is one structured log record, as pipeline previews read and
// return them.
type O11yLogRecord struct {
	// Timestamp is the record's time as a nanosecond epoch.
	Timestamp uint64 `json:"timestamp"`
	// ID is the record's id.
	ID string `json:"id"`
	// TraceID is the trace the record belongs to.
	TraceID string `json:"trace_id"`
	// SpanID is the span the record belongs to.
	SpanID string `json:"span_id"`
	// TraceFlags are the record's trace flags.
	TraceFlags uint32 `json:"trace_flags"`
	// SeverityText is the record's severity as text, e.g. ERROR.
	SeverityText string `json:"severity_text"`
	// SeverityNumber is the record's severity as a number.
	SeverityNumber uint8 `json:"severity_number"`
	// Body is the record's body.
	Body string `json:"body"`
	// ResourcesString are the record's string resource attributes.
	ResourcesString map[string]string `json:"resources_string"`
	// AttributesString are the record's string attributes.
	AttributesString map[string]string `json:"attributes_string"`
	// AttributesInt are the record's integer attributes.
	AttributesInt map[string]int64 `json:"attributes_int"`
	// AttributesFloat are the record's float attributes.
	AttributesFloat map[string]float64 `json:"attributes_float"`
	// AttributesBool are the record's boolean attributes.
	AttributesBool map[string]bool `json:"attributes_bool"`
}

// O11yLogPromotePath is one log body path and its index settings.
type O11yLogPromotePath struct {
	// Path is the body path, e.g. body.user.id on the way in; listed without
	// the body. prefix.
	Path string `json:"path"`
	// Promote lifts the path into its own column when true.
	Promote bool `json:"promote,omitempty"`
	// Indexes are the indexes to put on the path.
	Indexes []O11yLogPromoteIndex `json:"indexes,omitempty"`
}

// O11yLogPromoteIndex is one index on a promoted path.
type O11yLogPromoteIndex struct {
	// FieldDataType is the path's data type, e.g. string, number, bool.
	FieldDataType string `json:"fieldDataType"`
	// Type is the index type, e.g. minmax, set(N), bloom_filter(P).
	Type string `json:"type"`
	// Granularity is the index granularity in rows.
	Granularity int `json:"granularity"`
}

// O11yLogPromoteOut acknowledges a promotion.
type O11yLogPromoteOut struct {
	// Status is "success".
	Status string `json:"status"`
}

// O11yLogPromotedOut lists the promoted and indexed paths.
type O11yLogPromotedOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the paths.
	Data []O11yLogPromotePath `json:"data,omitempty"`
}

// ── the seam ──────────────────────────────────────────────────────────────────
