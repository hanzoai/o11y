package telemetryaudit

import "github.com/hanzoai/o11y/pkg/telemetryplane"

// DBName is NOT the event plane. o11y_audit holds the audit trail, which has a
// different retention and a different access gate from telemetry and was not in
// HIP-0132's scope. The name is spelled in pkg/telemetryplane with the plane and
// the other survivors.
const (
	DBName                      = telemetryplane.AuditDBName
	AuditLogsTableName          = "distributed_logs"
	AuditLogsLocalTableName     = "logs"
	TagAttributesTableName      = "distributed_tag_attributes"
	TagAttributesLocalTableName = "tag_attributes"
	LogAttributeKeysTblName     = "distributed_logs_attribute_keys"
	LogResourceKeysTblName      = "distributed_logs_resource_keys"
	LogsResourceTableName       = "distributed_logs_resource"
)
