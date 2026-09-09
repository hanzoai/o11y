package telemetrymetadata

import "github.com/hanzoai/o11y/pkg/telemetryplane"

// DBName is NOT the event plane, and that is deliberate.
//
// HIP-0132 unified the three SIGNAL databases — o11y_logs, o11y_traces,
// o11y_metrics — into `event`, and dropped them. It did not touch o11y_metadata,
// which the otel-collector metadata exporter still WRITES on every flush. A
// reader repointed at `event` would stop reading a table that is being fed,
// which is the same defect as reading a database that was dropped, pointed the
// other way.
//
// The name is spelled in pkg/telemetryplane with the plane and the other three
// survivors, so "which databases does this binary name?" has one answer.
const (
	DBName                           = telemetryplane.MetadataDBName
	AttributesMetadataTableName      = "distributed_attributes_metadata"
	AttributesMetadataLocalTableName = "attributes_metadata"
	ColumnEvolutionMetadataTableName = "distributed_column_evolution_metadata"
	// FieldKeysTable is the distributed field-keys table the otel-collector
	// metadata exporter writes to (o11y_metadata.distributed_field_keys). The
	// bare table-name constant was dropped from the collector's public
	// constants package in v0.144.7, so it is pinned locally here.
	FieldKeysTable = "distributed_field_keys"
	// Column Evolution table stores promoted paths as (signal, column_name, field_context, field_name); see otel-collector metadata_migrations.
	PromotedPathsTableName = "distributed_column_evolution_metadata"
	SkipIndexTableName     = "system.data_skipping_indices"
)
