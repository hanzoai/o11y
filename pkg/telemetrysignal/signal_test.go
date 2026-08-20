package telemetrysignal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsed(t *testing.T) {
	cases := []struct {
		name                  string
		sql                   string
		metrics, logs, traces bool
	}{
		{"metric samples", "SELECT count() FROM event.metric WHERE org = 'a'", true, false, false},
		{"series rollup", "SELECT * FROM event.series_1w", true, false, false},
		{"logs", "SELECT body FROM event.log WHERE service = 'api'", false, true, false},
		{"spans", "SELECT name FROM event.span WHERE trace_id = 'x'", false, false, true},
		{"unqualified", "SELECT * FROM span", false, false, true},
		{"backquoted", "SELECT * FROM `event`.`log`", false, true, false},
		{"lowercase from", "select * from event.metric", true, false, false},
		{"join pulls in a second signal", "SELECT * FROM event.metric m JOIN event.series s USING fingerprint", true, false, false},
		{"span joined to log", "SELECT * FROM event.span JOIN event.log USING trace_id", false, true, true},
		{"a sharded name still resolves", "SELECT * FROM event.distributed_span", false, false, true},
		{"no telemetry table", "SELECT 1", false, false, false},
		{"an unrelated table is not a signal", "SELECT * FROM event.event", false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, l, tr := Used(c.sql)
			assert.Equal(t, c.metrics, m, "metrics")
			assert.Equal(t, c.logs, l, "logs")
			assert.Equal(t, c.traces, tr, "traces")
		})
	}
}

// The reason this is a table match and not a substring search: span is a prefix of
// span_resource, and both "span" and "log" appear inside ordinary column names. A
// substring search reports a signal that the query never reads.
func TestUsedDoesNotMatchSubstrings(t *testing.T) {
	m, l, tr := Used("SELECT span_id, log_level FROM event.metric")
	assert.True(t, m, "the one table it reads is a metric table")
	assert.False(t, l, "a column called log_level is not a read of the log table")
	assert.False(t, tr, "a column called span_id is not a read of the span table")
}

// A supporting table counts as its signal — span_resource is part of reading traces.
func TestUsedCountsSupportingTables(t *testing.T) {
	_, _, tr := Used("SELECT fingerprint FROM event.span_resource")
	assert.True(t, tr)

	_, l, _ := Used("SELECT DISTINCT name FROM event.log_attribute")
	assert.True(t, l)
}
