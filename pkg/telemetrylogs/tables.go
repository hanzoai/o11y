package telemetrylogs

// Logs live in the ONE event database beside the other signals, as event.log:
// singular, because a row is one log line. It carries the same 15-column envelope as
// event.event / event.error / event.span, and is sorted (org, service, time) — the
// order a log read actually wants, with the tenant leading.
//
// There are no Distributed wrappers on this deployment, so the distributed and local
// name of a table are the same string. The pair is kept because the query builders
// distinguish the two roles.
const (
	DBName = "event"

	LogTableName      = "log"
	LogLocalTableName = "log"

	// SkipIndexTableName is the datastore's own introspection table, not ours.
	SkipIndexTableName = "system.data_skipping_indices"
)

// The names below are NOT part of the applied event schema, which holds event, error,
// log, span and the metric tables and nothing else. They name the supporting tables
// the log query plane reads — attribute autocomplete, resource fingerprints, JSON path
// types — under the one naming scheme, so those paths land in the event plane the day
// the tables are added rather than carrying a second scheme. Until then a query
// through them fails on a missing table, which is the honest outcome.
//
// The scheme: a table that SUPPORTS a signal is prefixed by that signal, which is what
// keeps log_attribute and span_attribute apart now that both planes share one database.
const (
	LogAttributeTableName      = "log_attribute"
	LogAttributeLocalTableName = "log_attribute"
	LogKeyTableName            = "log_key"
	LogResourceKeyTableName    = "log_resource_key"
	LogResourceTableName       = "log_resource"
	LogPathTableName           = "log_path"
	LogPromotedTableName       = "log_promoted"
)
