package telemetrytraces

// Spans live in the ONE event database beside the other signals, as event.span:
// singular, because a row is one span. It carries the same 15-column envelope as
// event.event / event.error / event.log, so correlating a span with an error is a
// UNION ALL rather than a translation layer, and org is the FIRST column of the sort
// key rather than a map lookup — spans are tenant-scopable by construction.
//
// There are no Distributed wrappers on this deployment, so the distributed and local
// name of a table are the same string. The pair is kept because the query builders
// distinguish the two roles.
const (
	DBName = "event"

	SpanTableName      = "span"
	SpanLocalTableName = "span"
)

// The names below are NOT part of the applied event schema, which holds event, error,
// log, span and the metric tables and nothing else. They name the supporting tables
// the trace query plane reads — attribute autocomplete, resource fingerprints,
// service-operation rollups — under the one naming scheme, so those paths land in the
// event plane the day the tables are added rather than carrying a second scheme.
// Until then a query through them fails on a missing table, which is the honest
// outcome.
//
// The scheme: a table that SUPPORTS a signal is prefixed by that signal, which is what
// keeps span_attribute and log_attribute apart now that both planes share one database.
const (
	SpanAttributeTableName      = "span_attribute"
	SpanAttributeLocalTableName = "span_attribute"
	SpanKeyTableName            = "span_key"
	SpanResourceTableName       = "span_resource"

	// operation is one (service, operation) pair; trace is one whole trace's summary.
	OperationTableName = "operation"
	TraceTableName     = "trace"
)
