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

// The supporting tables of the logs signal — attribute and key autocomplete, resource
// fingerprints. Their shapes are defined in schema.sql beside this file, and the first
// four below are applied to the event database.
//
// The scheme: a table that SUPPORTS a signal is prefixed by that signal, which is what
// keeps log_attribute and span_attribute apart now that both planes share one database.
const (
	LogAttributeTableName      = "log_attribute"
	LogAttributeLocalTableName = "log_attribute"
	LogKeyTableName            = "log_key"
	LogResourceKeyTableName    = "log_resource_key"
	LogResourceTableName       = "log_resource"
)

// These two are declared but NOT applied, because nothing reads them: log_path was
// distributed_json_path_types and log_promoted was distributed_json_promoted_paths,
// and neither had a SELECT against it before the rename either. The JSON-body
// promotion path reads telemetrymetadata.PromotedPathsTableName — a same-named
// constant in a different package, which is why a naive grep looks like this one has
// readers. Creating them would mean guessing a column set; a query through a missing
// table fails loudly, while a table with the wrong columns fails silently and corrupts
// the lens reading it. They are created the day a reader defines their shape.
const (
	LogPathTableName     = "log_path"
	LogPromotedTableName = "log_promoted"
)
