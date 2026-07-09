package telemetrymetadata

const (
	DBName                           = "o11y_metadata"
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
