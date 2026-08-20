package telemetryaudit

// The audit plane records audit log lines. It is NOT part of the applied event schema
// — the deployed database holds event, error, log, span and the metric tables and
// nothing else — but it is named under the one scheme so it lands in the event plane
// the day the tables are added rather than carrying a second scheme.
//
// A row of audit is one audited action. Its supporting tables are prefixed by the
// signal they support, which is what keeps audit_attribute apart from span_attribute
// and log_attribute now that one database holds them all.
const (
	DBName = "event"

	AuditLogsTableName      = "audit"
	AuditLogsLocalTableName = "audit"

	TagAttributesTableName      = "audit_attribute"
	TagAttributesLocalTableName = "audit_attribute"
	LogAttributeKeysTblName     = "audit_key"
	LogResourceKeysTblName      = "audit_resource_key"
	LogsResourceTableName       = "audit_resource"
)
