package telemetrymetadata

// The metadata plane is the CROSS-SIGNAL catalog of which fields exist and how they
// are typed — what powers attribute autocomplete. It is NOT part of the applied event
// schema — the deployed database holds event, error, log, span and the metric tables
// and nothing else — but it is named under the one scheme so it lands in the event
// plane the day the tables are added rather than carrying a second scheme.
//
// A row of field is one field key; a row of promotion is one field promoted out of the
// attribute map into its own column.
const (
	DBName = "event"

	AttributeTableName      = "field"
	AttributeLocalTableName = "field"

	// FieldKeysTable is the field-keys table the otel-collector metadata exporter
	// writes to. The bare table-name constant was dropped from the collector's public
	// constants package in v0.144.7, so it is pinned locally here.
	FieldKeysTable = "field"

	// Promotion stores promoted paths as (signal, column_name, field_context,
	// field_name); see the otel-collector metadata migrations.
	ColumnEvolutionMetadataTableName = "promotion"
	PromotedPathsTableName           = "promotion"

	// SkipIndexTableName is the datastore's own introspection table, not ours.
	SkipIndexTableName = "system.data_skipping_indices"
)
