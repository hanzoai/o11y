package datastorereader

import (
	"time"

	"github.com/hanzo-ds/go"

	"github.com/hanzoai/o11y/pkg/telemetrylogs"
	"github.com/hanzoai/o11y/pkg/telemetrymetadata"
	"github.com/hanzoai/o11y/pkg/telemetryplane"
	"github.com/hanzoai/o11y/pkg/telemetrytraces"
)

type Encoding string

const (
	// EncodingJSON is used for spans encoded as JSON.
	EncodingJSON Encoding = "json"
	// EncodingProto is used for spans encoded as Protobuf.
	EncodingProto Encoding = "protobuf"
)

// The defaults this reader is constructed with.
//
// EVERY NAME IS AN ALIAS. Both databases this file used to spell — o11y_traces
// and o11y_logs — were dropped by HIP-0132 and unified into the event plane, so
// every default here named something that no longer existed. The spellings live
// in pkg/telemetryplane and the table names in pkg/telemetry{traces,logs}/
// tables.go, which is the canonical statement of the plane's shape; this file
// now only says WHICH of them each role takes.
//
// THE V2-ERA NAMES ARE NOT ALIASED, and the second block below is where they
// stay. operations, index_v2, error_index_v2, durationSort, usage_explorer,
// o11y_spans and dependency_graph_minutes_v2 were dropped WITH o11y_traces and
// the plane has no successor with their columns. Pointing one at an event-plane
// table would swap `UNKNOWN_TABLE`, which names the retired table and is true,
// for `THERE_IS_NO_COLUMN`, which names a live table and reads like the plane is
// broken. The engine's error is the honest reason and it names the exact table;
// this comment is what an operator finds when they grep for it.
const (
	defaultTraceDB                 string        = telemetryplane.DBName
	defaultTopLevelOperationsTable string        = telemetrytraces.OperationTableName
	defaultSpanAttributeTableV2    string        = telemetrytraces.SpanAttributeTableName
	defaultSpanAttributeKeysTable  string        = telemetrytraces.SpanKeyTableName
	defaultLogsDB                  string        = telemetryplane.DBName
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
	defaultMetadataTable string = telemetrymetadata.AttributesMetadataTableName
)

// RETIRED — the v2-era tables of the old trace schema. Every one of them was
// dropped with o11y_traces and NONE of them has an event-plane successor that
// carries its columns: the exceptions pages read errorID/groupID/exceptionType,
// the service map reads a precomputed edge table, and the usage explorer reads a
// per-hour count. The event plane stores the SPANS those were derived from and
// nothing else.
//
// They are kept as literals, in one block, so that the read that reaches one
// fails with `Code: 60 UNKNOWN_TABLE: event.distributed_o11y_error_index_v2` —
// a message that names the retired table, which is the true reason. The
// alternative is a false empty, and a customer told "no errors" by a page that
// cannot see the table burns a day before opening a ticket.
const (
	defaultOperationsTable      string = "distributed_o11y_operations"
	defaultIndexTable           string = "distributed_o11y_index_v2"
	defaultLocalIndexTable      string = "o11y_index_v2"
	defaultErrorTable           string = "distributed_o11y_error_index_v2"
	defaultDurationTable        string = "distributed_durationSort"
	defaultUsageExplorerTable   string = "distributed_usage_explorer"
	defaultSpansTable           string = "distributed_o11y_spans"
	defaultDependencyGraphTable string = "distributed_dependency_graph_minutes_v2"
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
