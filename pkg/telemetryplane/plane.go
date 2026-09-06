// Package telemetryplane is the single answer to "which databases does this
// binary name?".
//
// WHY IT EXISTS. HIP-0132 unified the three SIGNAL databases — o11y_logs,
// o11y_traces, o11y_metrics — into one `event` database and dropped them. The
// commit that restored the reads (5be9208c4e, "o11y: read the event plane")
// pointed THREE files at `event` and said so in its own message:
//
//	NOTE, deliberately left as-is: DBName is still a compile-time constant. It
//	is now a CORRECT hardcode rather than a wrong one, but one name in three
//	files is still three places to change.
//
// It was thirty files, not three. The other twenty-seven kept naming databases
// that had been dropped, so every read through them answered
// `Code: 81 ... UNKNOWN_DATABASE` — and the views on top of them rendered an
// error boundary, or worse a FALSE EMPTY that told the customer to instrument an
// already-instrumented application.
//
// This is that cut. A database name is a VALUE, so it has ONE address, and every
// layer can reach it because this package imports nothing. The signal packages,
// the legacy v3/v4 query builders, the Prometheus adapter and the query-service
// constants sit at four different depths and some of them are imported BY
// telemetrystore, so the constants cannot live there without a cycle.
//
// plane_test.go asserts that no OTHER file in the tree spells a database name.
// That assertion is the point of the package: a new hardcode fails the build
// instead of failing a customer's dashboard six weeks later.
package telemetryplane

// DBName is the telemetry plane: ONE database holding every signal —
// event.log, event.span, event.error and the metric tables. Every table in
// every signal is qualified by this and nothing else.
//
// It is a constant, not config: a deployment that pointed half its readers at a
// second database would be two planes wearing one name, and the failure mode is
// the silent one — half the rows.
const DBName = "event"

// THE DATABASES THAT ARE NOT THE PLANE.
//
// Listing them here is not an invitation to add more. It is what makes the
// question answerable: "the plane, and these four" is checkable, while a name
// spelled in whichever file happened to need it is not. Each has a live WRITER,
// which is the whole reason it was out of HIP-0132's scope — repointing a reader
// at `event` when the rows are still landing somewhere else is the same defect as
// reading a dropped database, pointed the other way.
//
// Each name is aliased by exactly one package, which owns its table names too:
//
//	MetadataDBName  -> telemetrymetadata   (otel-collector metadata exporter writes it)
//	MeterDBName     -> telemetrymeter      (metering/usage counters)
//	AuditDBName     -> telemetryaudit      (audit trail; separate retention and gate)
//	AnalyticsDBName -> implrulestatehistory (AddRuleStateHistory writes it)
//
// A name leaves this block by being MIGRATED — a writer moves, the rows move,
// the alias becomes DBName — never by being deleted.
const (
	MetadataDBName  = "o11y_metadata"
	MeterDBName     = "o11y_meter"
	AuditDBName     = "o11y_audit"
	AnalyticsDBName = "o11y_analytics"
)
