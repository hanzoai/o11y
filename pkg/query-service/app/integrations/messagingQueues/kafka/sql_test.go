package kafka

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hanzoai/o11y/pkg/telemetryplane"
	"github.com/hanzoai/o11y/pkg/telemetrytraces"
)

// This package had NO test at all, and thirteen SQL templates in it named
// `o11y_traces` — a database HIP-0132 dropped. Every messaging/Kafka panel
// answered `Code: 81 UNKNOWN_DATABASE` and nothing in the tree noticed, because
// nothing in the tree ever looked at the SQL these functions produce.
//
// This is that look. It does not pin the whole query — a golden that pins 40
// lines of SQL fails on every unrelated edit and gets regenerated without being
// read. It pins the ONE thing that was wrong and is invisible until production:
// which database and table the read names.
func TestEveryGeneratedQueryNamesThePlane(t *testing.T) {
	const start, end = int64(1700000000000000000), int64(1700003600000000000)

	queries := map[string]string{
		"consumer":                    generateConsumerSQL(start, end, "topic", "0", "group", "kafka"),
		"partitionLatency":            generatePartitionLatencySQL(start, end, "kafka"),
		"consumerPartitionLatency":    generateConsumerPartitionLatencySQL(start, end, "topic", "0", "kafka"),
		"producerPartitionThroughput": generateProducerPartitionThroughputSQL(start, end, "kafka"),
		"producerTopicLatency":        generateProducerTopicLatencySQL(start, end, "topic", "svc", "kafka"),
		"consumerLatency":             generateConsumerLatencySQL(start, end, "kafka"),
		"consumerServiceLatency":      generateConsumerServiceLatencySQL(start, end, "topic", "svc", "kafka"),
		"producerConsumerEval":        generateProducerConsumerEvalSQL(start, end, "kafka", 100),
		"producer":                    generateProducerSQL(start, end, "topic", "0", "kafka"),
		"networkLatencyThroughput":    generateNetworkLatencyThroughputSQL(start, end, "group", "0", "kafka"),
	}

	wantTable := telemetryplane.DBName + "." + telemetrytraces.SpanTableName

	for name, sql := range queries {
		t.Run(name, func(t *testing.T) {
			require.NotEmpty(t, sql)
			assert.Containsf(t, sql, wantTable,
				"query must read the event plane's span table, got:\n%s", sql)
			for _, gone := range telemetryplane.Dropped {
				assert.NotContainsf(t, sql, gone,
					"query names %s, a database HIP-0132 dropped", gone)
			}
			// A qualifier with no database is the other way to reach the wrong
			// plane: it resolves against the connection's default database.
			assert.Falsef(t, strings.Contains(sql, " FROM "+telemetrytraces.SpanTableName+" "),
				"query reads an UNQUALIFIED table; it must name the plane explicitly")
		})
	}
}
