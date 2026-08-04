package o11y

// The APM reads of the /v1/o11y product face — the service catalog, the
// messaging-queue views and the third-party API overview — as TYPED ops.
//
// These twenty-one routes reached traffic only through the delegation wildcard,
// and a route behind a wildcard is in no document: no SDK method, no CLI
// command, no agent tool, no reference page. Typing them is what puts the
// operations in the document and therefore in every projection built from it.
//
// THE WIRE DOES NOT MOVE, the same way telemetry.go's five ops do not move it:
// each op hands the call to the SAME runtime handler the wildcard delegates to
// (see relay.go), so identity resolution, the org gate, the role check —
// every one of these routes has always answered behind the runtime's
// ViewAccess gate — the audit record and the success envelope are all still
// the runtime's, executed in the order they always were. What is new here is
// the TYPE — the In the caller may send and the Out they get back — and the
// prose that goes with it.
//
// Registered ahead of the wildcard, and specific-beats-wildcard is what the
// router does regardless of registration order, so these twenty-one paths
// dispatch here and every other path under the prefix still reaches the
// runtime untouched (apm_test.go pins both halves).
//
// Where an answer is polymorphic by construction — a query-range result's
// cells, the v5 result union — the field is json.RawMessage rather than a
// pretend schema: the runtime's bytes pass through untouched (large integer
// cells would not survive a float64 round trip), and the prose says what the
// bytes are. Nothing here is a re-implementation and nothing is invented.

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/zap-proto/zip"
)

// mountAPM registers the twenty-one typed APM ops on the native router — the
// service catalog first, then the messaging-queue groups in the order the mux
// tree declares them, then the third-party API overview.
func mountAPM(app zip.Router) {
	g := app.Group(o11yRoot)

	// The service catalog: /services collection, /service/* breakdowns.
	zip.Post(g, "/services", services)
	zip.Get(g, "/services/list", serviceNames)
	zip.Post(g, "/service/top_operations", topOperations)
	zip.Post(g, "/service/top_level_operations", topLevelOperations)
	zip.Post(g, "/service/entry_point_operations", entryPointOperations)

	// The messaging-queue surface: the queue overview, then the Kafka
	// onboarding, partition-latency, consumer-lag, topic-throughput and
	// span-evaluation groups.
	zip.Post(g, "/messaging-queues/queue-overview", queueOverview)
	zip.Post(g, "/messaging-queues/kafka/onboarding/producers", producersOnboarding)
	zip.Post(g, "/messaging-queues/kafka/onboarding/consumers", consumersOnboarding)
	zip.Post(g, "/messaging-queues/kafka/onboarding/kafka", kafkaOnboarding)
	zip.Post(g, "/messaging-queues/kafka/partition-latency/overview", partitionLatency)
	zip.Post(g, "/messaging-queues/kafka/partition-latency/consumer", consumerPartitionLatency)
	zip.Post(g, "/messaging-queues/kafka/consumer-lag/producer-details", producerLagDetails)
	zip.Post(g, "/messaging-queues/kafka/consumer-lag/consumer-details", consumerLagDetails)
	zip.Post(g, "/messaging-queues/kafka/consumer-lag/network-latency", consumerLagNetwork)
	zip.Post(g, "/messaging-queues/kafka/topic-throughput/producer", producerThroughput)
	zip.Post(g, "/messaging-queues/kafka/topic-throughput/producer-details", producerThroughputDetails)
	zip.Post(g, "/messaging-queues/kafka/topic-throughput/consumer", consumerThroughput)
	zip.Post(g, "/messaging-queues/kafka/topic-throughput/consumer-details", consumerThroughputDetails)
	zip.Post(g, "/messaging-queues/kafka/span/evaluation", spanEvaluation)

	// The third-party API overview: external domains and one domain's detail.
	zip.Post(g, "/third-party-apis/overview/list", domainList)
	zip.Post(g, "/third-party-apis/overview/domain", domainInfo)
}

// ── the service catalog ───────────────────────────────────────────────────────

// services lists the instrumented services seen in the window, each with the
// request profile of its entry-point spans: p99 and average latency, call and
// error rates, and the entry-point operations the numbers were computed over.
func services(ctx context.Context, in *O11yServicesIn) (*O11yServicesOut, error) {
	out := new(O11yServicesOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/services", nil, in, out)
}

// serviceNames lists the name of every service the trace store holds, with no
// window applied — the complete catalog, for pickers and autocomplete.
func serviceNames(ctx context.Context, in *O11yServiceNamesIn) (*O11yServiceNames, error) {
	out := new(O11yServiceNames)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/services/list", nil, nil, out)
}

// topOperations returns one service's heaviest operations in the window, each
// with p50/p95/p99 latency, how often it ran and how often it errored.
func topOperations(ctx context.Context, in *O11yOperationsIn) (*O11yOperationsOut, error) {
	out := new(O11yOperationsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/service/top_operations", nil, in, out)
}

// topLevelOperations maps each service to its entry-point span names — for the
// one service named in the request, or for every service when none is.
func topLevelOperations(ctx context.Context, in *O11yTopLevelOpsIn) (*O11yServiceOperations, error) {
	out := new(O11yServiceOperations)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/service/top_level_operations", nil, in, out)
}

// entryPointOperations returns one service's entry-point operations with the
// same latency and error profile topOperations reports.
func entryPointOperations(ctx context.Context, in *O11yOperationsIn) (*O11yOperationsOut, error) {
	out := new(O11yOperationsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/service/entry_point_operations", nil, in, out)
}

// ── the messaging-queue surface ───────────────────────────────────────────────

// queueOverview lists the messaging destinations observed in the window — one
// row per queue/destination/service combination with its throughput and
// latency columns. Filters narrow by queue system, destination, service or any
// span attribute.
func queueOverview(ctx context.Context, in *O11yQueueListIn) (*O11yQueueRowsOut, error) {
	out := new(O11yQueueRowsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/queue-overview", nil, in, out)
}

// producersOnboarding checks whether the spans the Kafka producer views need
// are arriving — one row per required span attribute, with a pass/fail status
// and, on failure, what is missing from the instrumentation.
func producersOnboarding(ctx context.Context, in *O11yQueueIn) (*O11yQueueChecksOut, error) {
	out := new(O11yQueueChecksOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/onboarding/producers", nil, in, out)
}

// consumersOnboarding checks whether the spans the Kafka consumer views need
// are arriving, row for row like producersOnboarding.
func consumersOnboarding(ctx context.Context, in *O11yQueueIn) (*O11yQueueChecksOut, error) {
	out := new(O11yQueueChecksOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/onboarding/consumers", nil, in, out)
}

// kafkaOnboarding checks whether Kafka's own metrics — consumer lag and
// partition telemetry — are arriving, so the lag views can be lit up.
func kafkaOnboarding(ctx context.Context, in *O11yQueueIn) (*O11yQueueChecksOut, error) {
	out := new(O11yQueueChecksOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/onboarding/kafka", nil, in, out)
}

// partitionLatency returns the per-partition latency overview for the window —
// each topic/partition with its throughput and latency profile.
func partitionLatency(ctx context.Context, in *O11yQueueIn) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/partition-latency/overview", nil, in, out)
}

// consumerPartitionLatency returns the consumer-group latency detail for the
// topic and partition named in the request's variables.
func consumerPartitionLatency(ctx context.Context, in *O11yQueueIn) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/partition-latency/consumer", nil, in, out)
}

// producerLagDetails returns the producer side of a consumer-lag view: the
// producers writing to the topic/partition named in variables, with their
// throughput and latency over the window.
func producerLagDetails(ctx context.Context, in *O11yQueueIn) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/consumer-lag/producer-details", nil, in, out)
}

// consumerLagDetails returns the consumer side of a consumer-lag view: the
// consumer groups reading the topic/partition named in variables, with their
// throughput and latency over the window.
func consumerLagDetails(ctx context.Context, in *O11yQueueIn) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/consumer-lag/consumer-details", nil, in, out)
}

// consumerLagNetwork returns consumer network latency correlated per client:
// a throughput pass over the window finds the consumer clients, then their
// fetch latency joins in as a latency column per client/instance/service.
func consumerLagNetwork(ctx context.Context, in *O11yQueueIn) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/consumer-lag/network-latency", nil, in, out)
}

// producerThroughput returns the producer topic-throughput overview for the
// window — what each producer service wrote, per topic.
func producerThroughput(ctx context.Context, in *O11yQueueIn) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/topic-throughput/producer", nil, in, out)
}

// producerThroughputDetails breaks one producer topic's throughput down using
// the topic and service named in variables.
func producerThroughputDetails(ctx context.Context, in *O11yQueueIn) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/topic-throughput/producer-details", nil, in, out)
}

// consumerThroughput returns the consumer topic-throughput overview for the
// window — what each consumer group read, per topic.
func consumerThroughput(ctx context.Context, in *O11yQueueIn) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/topic-throughput/consumer", nil, in, out)
}

// consumerThroughputDetails breaks one consumer topic's throughput down using
// the topic and service named in variables.
func consumerThroughputDetails(ctx context.Context, in *O11yQueueIn) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/topic-throughput/consumer-details", nil, in, out)
}

// spanEvaluation correlates producer and consumer spans over the evaluation
// window (eval_time bounds the scan) and returns the pairings with their
// end-to-end delay — the check that messages produced are being consumed.
func spanEvaluation(ctx context.Context, in *O11yQueueIn) (*O11yQueryRangeOut, error) {
	out := new(O11yQueryRangeOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/messaging-queues/kafka/span/evaluation", nil, in, out)
}

// ── the third-party API overview ──────────────────────────────────────────────

// domainList lists the external domains the instrumented services call, with
// request rate, error percentage and latency per domain. Rows whose domain is
// a bare IP address are dropped unless show_ip asks for them.
func domainList(ctx context.Context, in *O11yDomainsIn) (*O11yDomainsOut, error) {
	out := new(O11yDomainsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/third-party-apis/overview/list", nil, in, out)
}

// domainInfo returns one external domain's endpoint-level breakdown — each
// endpoint with its rate, error and latency columns over the window.
func domainInfo(ctx context.Context, in *O11yDomainsIn) (*O11yDomainsOut, error) {
	out := new(O11yDomainsOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/third-party-apis/overview/domain", nil, in, out)
}

// ── inputs ────────────────────────────────────────────────────────────────────
//
// Every type carries the O11y qualifier because these names enter a fleet-wide
// document, where one name may have exactly one shape. Each In mirrors the
// runtime's own request type tag for tag, so the body relay sends is the
// body the runtime has always decoded — validation stays the runtime's, one
// layer in, exactly where it has always run.

// O11yServicesIn selects the window and filters a service listing is computed
// over.
type O11yServicesIn struct {
	// Start is the window's start, epoch nanoseconds as a string.
	Start string `json:"start"`
	// End is the window's end, epoch nanoseconds as a string.
	End string `json:"end"`
	// Tags narrow the spans counted, each a span-attribute predicate.
	Tags []O11yServiceTag `json:"tags"`
}

// O11yServiceTag is one span-attribute predicate of the service catalog's
// legacy filter shape — its key spellings are capitalized on the wire.
type O11yServiceTag struct {
	// Key is the span attribute to test.
	Key string `json:"Key"`
	// Operator is how to test it, e.g. in, not_in.
	Operator string `json:"Operator"`
	// StringValues are the string operands, when the attribute is a string.
	StringValues []string `json:"StringValues"`
	// NumberValues are the numeric operands, when the attribute is a number.
	NumberValues []float64 `json:"NumberValues"`
	// BoolValues are the boolean operands, when the attribute is a bool.
	BoolValues []bool `json:"BoolValues"`
	// TagType says which plane the attribute lives on, e.g. tag or resource.
	TagType string `json:"TagType"`
}

// O11yServiceNamesIn is empty: the catalog listing takes no parameters.
type O11yServiceNamesIn struct{}

// O11yOperationsIn names the service whose operations are read, and the window
// to read them over.
type O11yOperationsIn struct {
	// Start is the window's start, epoch nanoseconds as a string.
	Start string `json:"start"`
	// End is the window's end, epoch nanoseconds as a string.
	End string `json:"end"`
	// Service is the service whose operations are read.
	Service string `json:"service"`
	// Tags narrow the spans counted, each a span-attribute predicate.
	Tags []O11yServiceTag `json:"tags"`
	// Limit caps how many operations come back.
	Limit int `json:"limit,omitempty"`
}

// O11yTopLevelOpsIn optionally narrows the entry-point listing to one service
// and a window; every field may be left empty for the full map.
type O11yTopLevelOpsIn struct {
	// Service narrows the map to one service when set.
	Service string `json:"service"`
	// Start is the window's start, epoch nanoseconds as a string; empty means
	// unbounded.
	Start string `json:"start"`
	// End is the window's end, epoch nanoseconds as a string; empty means
	// unbounded.
	End string `json:"end"`
}

// O11yQueueIn is the shared request of the Kafka views: a window, and the
// view's own variables.
type O11yQueueIn struct {
	// Start is the window's start, epoch nanoseconds.
	Start int64 `json:"start"`
	// End is the window's end, epoch nanoseconds.
	End int64 `json:"end"`
	// EvalTime bounds the span-evaluation scan, nanoseconds; only the
	// span/evaluation view reads it.
	EvalTime int64 `json:"eval_time,omitempty"`
	// Variables name what the view drills into — topic, partition, service,
	// consumer_group — keyed by the name the view expects.
	Variables map[string]string `json:"variables,omitempty"`
}

// O11yQueueListIn selects the window, filters and page size of the queue
// overview.
type O11yQueueListIn struct {
	// Start is the window's start, epoch nanoseconds.
	Start int64 `json:"start"`
	// End is the window's end, epoch nanoseconds.
	End int64 `json:"end"`
	// Filters narrow the rows by span attribute; null means all rows.
	Filters *O11yQueueFilterSet `json:"filters"`
	// Limit caps how many rows come back.
	Limit int `json:"limit"`
}

// O11yQueueFilterSet combines attribute predicates with one operator.
type O11yQueueFilterSet struct {
	// Op combines the items: AND or OR.
	Op string `json:"op,omitempty"`
	// Items are the predicates.
	Items []O11yQueueFilterRule `json:"items"`
}

// O11yQueueFilterRule is one predicate on a span attribute.
type O11yQueueFilterRule struct {
	// Key names the attribute the predicate tests.
	Key O11yQueueFilterKey `json:"key"`
	// Value is the operand; its JSON type follows the attribute's dataType.
	Value any `json:"value"`
	// Op is the comparison, e.g. =, !=, in, contains.
	Op string `json:"op"`
}

// O11yQueueFilterKey names a span attribute and how it is stored.
type O11yQueueFilterKey struct {
	// Key is the attribute name.
	Key string `json:"key"`
	// DataType is the attribute's type: string, int64, float64 or bool.
	DataType string `json:"dataType"`
	// Type says which plane the attribute lives on: tag or resource.
	Type string `json:"type"`
	// IsColumn marks an attribute materialized as its own store column.
	IsColumn bool `json:"isColumn"`
	// IsJSON marks an attribute read out of the span's JSON body.
	IsJSON bool `json:"isJSON"`
}

// O11yDomainsIn selects the window and scope of a third-party API read.
type O11yDomainsIn struct {
	// Start is the window's start, epoch milliseconds.
	Start uint64 `json:"start"`
	// End is the window's end, epoch milliseconds.
	End uint64 `json:"end"`
	// ShowIP keeps rows whose domain is a bare IP address; they are dropped
	// otherwise.
	ShowIP bool `json:"show_ip,omitempty"`
	// Domain narrows the read to one external domain (the domain view requires
	// it).
	Domain string `json:"domain,omitempty"`
	// Endpoint narrows the domain view to one endpoint.
	Endpoint string `json:"endpoint,omitempty"`
	// Filter is an additional predicate in the query-builder filter syntax.
	Filter *O11yDomainFilter `json:"filter,omitempty"`
	// GroupBy adds grouping columns to the result.
	GroupBy []O11yDomainGroupBy `json:"groupBy,omitempty"`
}

// O11yDomainFilter is a filter expression in the query-builder syntax.
type O11yDomainFilter struct {
	// Expression is the predicate, e.g. `http.status_code >= 500`.
	Expression string `json:"expression"`
}

// O11yDomainGroupBy names one telemetry field to group by.
type O11yDomainGroupBy struct {
	// Name is the field's name. Required.
	Name string `json:"name"`
	// Description describes the field, when known.
	Description string `json:"description,omitempty"`
	// Unit is the field's unit, when known.
	Unit string `json:"unit,omitempty"`
	// Signal is the telemetry signal the field belongs to, e.g. traces.
	Signal string `json:"signal,omitempty"`
	// FieldContext says which plane the field lives on, e.g. attribute,
	// resource, span.
	FieldContext string `json:"fieldContext,omitempty"`
	// FieldDataType is the field's type: string, int64, float64 or bool.
	FieldDataType string `json:"fieldDataType,omitempty"`
}

// ── outputs ───────────────────────────────────────────────────────────────────
//
// Each Out is the runtime's answer NAMED, field for field, tag for tag —
// including the status/data envelope every enveloped read on this face has
// always answered with, and NO envelope on the two reads that never had one.
// Where a cell's shape depends on the query that produced it, the cell is
// json.RawMessage: the runtime's bytes pass through verbatim rather than
// surviving (or not) a float64 round trip. apm_test.go re-encodes each Out and
// compares it byte for byte with what the runtime wrote.

// O11yServicesOut is the service catalog listing.
type O11yServicesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds one entry per service.
	Data []O11yService `json:"data"`
}

// O11yService is one instrumented service's request profile over the window.
type O11yService struct {
	// ServiceName is the service.
	ServiceName string `json:"serviceName"`
	// Percentile99 is the p99 latency of its entry-point spans, nanoseconds.
	Percentile99 float64 `json:"p99"`
	// AvgDuration is their average latency, nanoseconds.
	AvgDuration float64 `json:"avgDuration"`
	// NumCalls is how many entry-point spans landed in the window.
	NumCalls uint64 `json:"numCalls"`
	// CallRate is calls per second over the window.
	CallRate float64 `json:"callRate"`
	// NumErrors is how many of the calls errored.
	NumErrors uint64 `json:"numErrors"`
	// ErrorRate is the percentage of calls that errored.
	ErrorRate float64 `json:"errorRate"`
	// Num4XX is how many of the calls answered 4xx.
	Num4XX uint64 `json:"num4XX"`
	// FourXXRate is the percentage of calls that answered 4xx.
	FourXXRate float64 `json:"fourXXRate"`
	// DataWarning carries the entry-point operations the numbers were computed
	// over.
	DataWarning O11yServiceWarning `json:"dataWarning"`
}

// O11yServiceWarning qualifies a service's numbers.
type O11yServiceWarning struct {
	// TopLevelOps are the entry-point operations the profile was computed over.
	TopLevelOps []string `json:"topLevelOps"`
}

// O11yServiceNames is the catalog of service names, as the bare array this
// route has always answered with — no envelope.
type O11yServiceNames []string

// O11yOperationsOut is an operation listing for one service.
type O11yOperationsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds one entry per operation.
	Data []O11yOperation `json:"data"`
}

// O11yOperation is one operation's latency and error profile.
type O11yOperation struct {
	// Name is the operation (span name).
	Name string `json:"name"`
	// P50 is its median latency, nanoseconds.
	P50 float64 `json:"p50"`
	// P95 is its p95 latency, nanoseconds.
	P95 float64 `json:"p95"`
	// P99 is its p99 latency, nanoseconds.
	P99 float64 `json:"p99"`
	// NumCalls is how many times it ran in the window.
	NumCalls uint64 `json:"numCalls"`
	// ErrorCount is how many of those runs errored.
	ErrorCount uint64 `json:"errorCount"`
}

// O11yServiceOperations maps each service to its entry-point operation names,
// as the bare object this route has always answered with — no envelope.
type O11yServiceOperations map[string][]string

// O11yQueueChecksOut is an onboarding check result.
type O11yQueueChecksOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds one check per required attribute, sorted by attribute.
	Data []O11yQueueCheck `json:"data"`
}

// O11yQueueCheck is one onboarding check.
type O11yQueueCheck struct {
	// Attribute is the span attribute or telemetry the check looked for.
	Attribute string `json:"attribute"`
	// Message says what is missing when the check fails; empty on a pass. Its
	// wire key is error_message.
	Message string `json:"error_message"`
	// Status is "1" when the telemetry is present, "0" when it is not.
	Status string `json:"status"`
}

// O11yQueueRowsOut is the queue overview listing.
type O11yQueueRowsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds one row per messaging destination.
	Data []O11yQueueRow `json:"data"`
}

// O11yQueueRow is one row of a list-shaped result: a timestamp and the row's
// columns.
type O11yQueueRow struct {
	// Timestamp anchors the row in time.
	Timestamp time.Time `json:"timestamp"`
	// Data holds the row's cells keyed by column name; each cell's JSON type is
	// the column's own, so the bytes pass through verbatim.
	Data map[string]json.RawMessage `json:"data"`
}

// O11yQueryResult is one query's result: series, rows or a table, whichever
// the view produced.
type O11yQueryResult struct {
	// QueryName names the query within the view.
	QueryName string `json:"queryName,omitempty"`
	// Series are the timeseries, for series-shaped results.
	Series []O11ySeries `json:"series,omitempty"`
	// PredictedSeries are model-predicted values, when the view predicts.
	PredictedSeries []O11ySeries `json:"predictedSeries,omitempty"`
	// UpperBoundSeries bound the prediction from above.
	UpperBoundSeries []O11ySeries `json:"upperBoundSeries,omitempty"`
	// LowerBoundSeries bound the prediction from below.
	LowerBoundSeries []O11ySeries `json:"lowerBoundSeries,omitempty"`
	// AnomalyScores score each point's deviation, when the view scores.
	AnomalyScores []O11ySeries `json:"anomalyScores,omitempty"`
	// List holds row-shaped results.
	List []O11yQueueRow `json:"list,omitempty"`
	// Table holds table-shaped results.
	Table *O11yQueryTable `json:"table,omitempty"`
}

// O11ySeries is one labeled timeseries.
type O11ySeries struct {
	// Labels key the series, e.g. topic, partition, service_name.
	Labels map[string]string `json:"labels"`
	// LabelsArray is the same labels in the store's array form.
	LabelsArray []map[string]string `json:"labelsArray"`
	// Points are the series' points, under the wire key "values".
	Points []O11ySeriesPoint `json:"values"`
}

// O11ySeriesPoint is one point of a timeseries. The wire carries the value as
// a decimal string — that is this face's own encoding, kept verbatim.
type O11ySeriesPoint struct {
	// Timestamp is the point's time, epoch milliseconds.
	Timestamp int64 `json:"timestamp"`
	// Value is the point's value, as a decimal string.
	Value string `json:"value"`
}

// O11yQueryTable is a table-shaped result.
type O11yQueryTable struct {
	// Columns name and type the table's columns.
	Columns []O11yQueryColumn `json:"columns"`
	// Rows are the table's rows.
	Rows []O11yQueryTableRow `json:"rows"`
}

// O11yQueryColumn is one table column.
type O11yQueryColumn struct {
	// Name is the column's name.
	Name string `json:"name"`
	// QueryName is the query the column came from.
	QueryName string `json:"queryName"`
	// IsValueColumn marks the column carrying the plotted value.
	IsValueColumn bool `json:"isValueColumn"`
}

// O11yQueryTableRow is one table row.
type O11yQueryTableRow struct {
	// Data holds the row's cells keyed by column name; each cell's JSON type is
	// the column's own, so the bytes pass through verbatim.
	Data map[string]json.RawMessage `json:"data"`
}

// O11yDomainsOut is a third-party API answer.
type O11yDomainsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the result.
	Data *O11yDomainsAnswer `json:"data"`
}

// O11yDomainsAnswer is the v5 query result the third-party views answer with.
//
// FIELD ORDER mirrors qbtypes.QueryRangeResponse.MarshalJSON, not that type's
// own field order: the marshaler embeds the value and re-declares data as an
// OUTER field, and encoding/json sorts by index path — the shallower outer
// data sorts after the embedded meta and warning. So the runtime emits
// type, meta, warning?, data, and this Out reproduces the runtime's bytes,
// so it reproduces that order (apm_test.go compares them byte for byte).
type O11yDomainsAnswer struct {
	// Type names the result shape: scalar, time_series or raw.
	Type string `json:"type"`
	// Meta reports what the read cost.
	Meta O11yQueryStats `json:"meta"`
	// Warning carries the store's warning for this read, when it raised one.
	Warning *O11yQueryWarning `json:"warning,omitempty"`
	// Data holds the per-query results, emitted LAST — see the field-order note
	// on the type.
	Data O11yDomainsData `json:"data"`
}

// O11yDomainsData holds the result union.
type O11yDomainsData struct {
	// Results is one entry per query. Each entry's shape follows the answer's
	// type — time-series data, scalar data or raw rows — so the bytes pass
	// through verbatim.
	Results []json.RawMessage `json:"results"`
}

// ── the seam ──────────────────────────────────────────────────────────────────

// apmRefusal reads a refusal for its code and reason. The query-service face
// answers refusals in one more shape than the sentry face does — the legacy
// {status, errorType, error} — so that shape is read here and every other one
// falls through to the shared refusal reader, which never invents a reason and
// never loses one.
func apmRefusal(body []byte) (code, reason string) {
	var refused struct {
		ErrorType string `json:"errorType"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &refused); err == nil && refused.Error != "" {
		return refused.ErrorType, refused.Error
	}
	return refusal(body)
}
