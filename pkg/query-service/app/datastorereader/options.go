package datastorereader

import (
	"time"

	"github.com/hanzoai/o11y/pkg/telemetrylogs"
	"github.com/hanzoai/o11y/pkg/telemetrymetadata"
	"github.com/hanzoai/o11y/pkg/telemetrytraces"

	"github.com/hanzo-ds/go"
)

type Encoding string

const (
	// EncodingJSON is used for spans encoded as JSON.
	EncodingJSON Encoding = "json"
	// EncodingProto is used for spans encoded as Protobuf.
	EncodingProto Encoding = "protobuf"
)

// These are this reader's DEFAULTS, not a second definition of the schema — each one
// resolves to the shared per-signal constant, so a table is named in exactly one place.
// This reader carried its own copy of every name, including several that predate the
// current span and log tables; there is one span table and one log table now, so the
// generations that used to be distinct all resolve to the same name.
const (
	defaultTraceDB                 string        = telemetrytraces.DBName
	defaultIndexTable              string        = telemetrytraces.SpanTableName
	defaultLocalIndexTable         string        = telemetrytraces.SpanLocalTableName
	defaultTopLevelOperationsTable string        = telemetrytraces.OperationTableName
	defaultSpanAttributeTableV2    string        = telemetrytraces.SpanAttributeTableName
	defaultSpanAttributeKeysTable  string        = telemetrytraces.SpanKeyTableName
	defaultLogsDB                  string        = telemetrylogs.DBName
	defaultLogsTable               string        = telemetrylogs.LogTableName
	defaultLogsLocalTable          string        = telemetrylogs.LogLocalTableName
	defaultLogAttributeKeysTable   string        = telemetrylogs.LogKeyTableName
	defaultLogResourceKeysTable    string        = telemetrylogs.LogResourceKeyTableName
	defaultLogTagAttributeTableV2  string        = telemetrylogs.LogAttributeTableName
	defaultLiveTailRefreshSeconds  int           = 5
	defaultWriteBatchDelay         time.Duration = 5 * time.Second
	defaultWriteBatchSize          int           = 10000
	defaultEncoding                Encoding      = EncodingJSON

	defaultLogsLocalTableV2         string = telemetrylogs.LogLocalTableName
	defaultLogsTableV2              string = telemetrylogs.LogTableName
	defaultLogsResourceLocalTableV2 string = telemetrylogs.LogResourceTableName
	defaultLogsResourceTableV2      string = telemetrylogs.LogResourceTableName

	defaultTraceIndexTableV3    string = telemetrytraces.SpanTableName
	defaultTraceLocalTableName  string = telemetrytraces.SpanLocalTableName
	defaultTraceResourceTableV3 string = telemetrytraces.SpanResourceTableName
	defaultTraceSummaryTable    string = telemetrytraces.TraceTableName

	defaultMetadataDB    string = telemetrymetadata.DBName
	defaultMetadataTable string = telemetrymetadata.AttributeTableName
)

// The tables below belong to this reader alone and have no successor in the applied
// event schema — the deployed database holds event, error, log, span and the metric
// tables and nothing else. They are named under the one scheme (a table that supports
// a signal is prefixed by that signal) so nothing carries a second naming scheme; a
// query through them fails on a missing table, which is the honest outcome.
const (
	defaultOperationsTable      string = "span_operation"
	defaultErrorTable           string = "span_error"
	defaultDurationTable        string = "span_duration"
	defaultUsageExplorerTable   string = "span_usage"
	defaultSpansTable           string = "span_raw"
	defaultDependencyGraphTable string = "dependency"
)

// NamespaceConfig is Datastore's internal configuration data
type namespaceConfig struct {
	namespace               string
	Enabled                 bool
	Datasource              string
	TraceDB                 string
	OperationsTable         string
	IndexTable              string
	LocalIndexTable         string
	DurationTable           string
	UsageExplorerTable      string
	SpansTable              string
	ErrorTable              string
	SpanAttributeTableV2    string
	SpanAttributeKeysTable  string
	DependencyGraphTable    string
	TopLevelOperationsTable string
	LogsDB                  string
	LogsTable               string
	LogsLocalTable          string
	LogsAttributeKeysTable  string
	LogsResourceKeysTable   string
	LogsTagAttributeTableV2 string
	LiveTailRefreshSeconds  int
	WriteBatchDelay         time.Duration
	WriteBatchSize          int
	Encoding                Encoding
	Connector               Connector

	LogsLocalTableV2         string
	LogsTableV2              string
	LogsResourceLocalTableV2 string
	LogsResourceTableV2      string

	TraceIndexTableV3     string
	TraceLocalTableNameV3 string
	TraceResourceTableV3  string
	TraceSummaryTable     string
	MetadataDB            string
	MetadataTable         string
}

// Connecto defines how to connect to the database
type Connector func(cfg *namespaceConfig) (datastore.Conn, error)

// Options store storage plugin related configs
type Options struct {
	primary *namespaceConfig

	others map[string]*namespaceConfig
}

// NewOptions creates a new Options struct.
func NewOptions(
	primaryNamespace string,
	otherNamespaces ...string,
) *Options {
	options := &Options{
		primary: &namespaceConfig{
			namespace:               primaryNamespace,
			Enabled:                 true,
			TraceDB:                 defaultTraceDB,
			OperationsTable:         defaultOperationsTable,
			IndexTable:              defaultIndexTable,
			LocalIndexTable:         defaultLocalIndexTable,
			ErrorTable:              defaultErrorTable,
			DurationTable:           defaultDurationTable,
			UsageExplorerTable:      defaultUsageExplorerTable,
			SpansTable:              defaultSpansTable,
			SpanAttributeTableV2:    defaultSpanAttributeTableV2,
			SpanAttributeKeysTable:  defaultSpanAttributeKeysTable,
			DependencyGraphTable:    defaultDependencyGraphTable,
			TopLevelOperationsTable: defaultTopLevelOperationsTable,
			LogsDB:                  defaultLogsDB,
			LogsTable:               defaultLogsTable,
			LogsLocalTable:          defaultLogsLocalTable,
			LogsAttributeKeysTable:  defaultLogAttributeKeysTable,
			LogsResourceKeysTable:   defaultLogResourceKeysTable,
			LogsTagAttributeTableV2: defaultLogTagAttributeTableV2,
			LiveTailRefreshSeconds:  defaultLiveTailRefreshSeconds,
			WriteBatchDelay:         defaultWriteBatchDelay,
			WriteBatchSize:          defaultWriteBatchSize,
			Encoding:                defaultEncoding,

			LogsTableV2:              defaultLogsTableV2,
			LogsLocalTableV2:         defaultLogsLocalTableV2,
			LogsResourceTableV2:      defaultLogsResourceTableV2,
			LogsResourceLocalTableV2: defaultLogsResourceLocalTableV2,

			TraceIndexTableV3:     defaultTraceIndexTableV3,
			TraceLocalTableNameV3: defaultTraceLocalTableName,
			TraceResourceTableV3:  defaultTraceResourceTableV3,
			TraceSummaryTable:     defaultTraceSummaryTable,
			MetadataDB:            defaultMetadataDB,
			MetadataTable:         defaultMetadataTable,
		},
		others: make(map[string]*namespaceConfig, len(otherNamespaces)),
	}

	for _, namespace := range otherNamespaces {
		if namespace == archiveNamespace {
			options.others[namespace] = &namespaceConfig{
				namespace:              namespace,
				TraceDB:                "",
				OperationsTable:        "",
				IndexTable:             "",
				ErrorTable:             "",
				LogsDB:                 "",
				LogsTable:              "",
				LogsLocalTable:         "",
				LogsAttributeKeysTable: "",
				LogsResourceKeysTable:  "",
				LiveTailRefreshSeconds: defaultLiveTailRefreshSeconds,
				WriteBatchDelay:        defaultWriteBatchDelay,
				WriteBatchSize:         defaultWriteBatchSize,
				Encoding:               defaultEncoding,
			}
		} else {
			options.others[namespace] = &namespaceConfig{namespace: namespace}
		}
	}

	return options
}

// GetPrimary returns the primary namespace configuration
func (opt *Options) getPrimary() *namespaceConfig {
	return opt.primary
}
