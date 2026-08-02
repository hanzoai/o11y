package datastoretraces

import (
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/telemetrytraces"
)

// planeWriters is every INSERT this package issues, paired with the table it
// targets. Adding a write path means adding it here.
var planeWriters = []struct {
	table string
	tmpl  string
}{
	{telemetrytraces.SpanTableName, spanSQLTmpl},
	{telemetrytraces.SpanAttributeTableName, attributeSQLTmpl},
	{telemetrytraces.SpanKeyTableName, keySQLTmpl},
	{telemetrytraces.SpanResourceTableName, resourceSQLTmpl},
	{telemetrytraces.OperationTableName, operationSQLTmpl},
	{telemetrytraces.TraceTableName, traceSQLTmpl},
}

// TestPlaneWritersNeverBindIngestedAt is the guard for the rule this package
// broke: ingested_at is the event plane's retention anchor and must be set by
// the SERVER, never by a writer, and therefore never by whoever produced the
// batch.
//
// In event.span's DDL that one column is simultaneously
//
//	ReplacingMergeTree(ingested_at)     -- which duplicate wins a merge
//	PARTITION BY toDate(ingested_at)    -- which partition the row lands in
//	TTL toDateTime(ingested_at) + 30d   -- when the row is deleted
//
// so binding it lets a sender choose its own retention — keep data past the
// policy by dating it forward, or destroy it by dating it back. The column
// carries DEFAULT now64(3) precisely so no writer has to.
//
// cloud enforces the same rule downstream (TestRetentionIsNotARequestParameter,
// apps/analytics/capture_test.go). It lives here too because THIS repo owns the
// DDL, and a rule enforced only downstream is a rule the schema's own writers
// can still break.
func TestPlaneWritersNeverBindIngestedAt(t *testing.T) {
	for _, w := range planeWriters {
		if strings.Contains(w.tmpl, "ingested_at") {
			t.Errorf("%s writer binds ingested_at — the sender would control the "+
				"Replacing version, the partition and the TTL of its own rows; "+
				"drop the column and let DEFAULT now64(3) apply", w.table)
		}
	}
}

// TestPlaneTablesAreQualified keeps every INSERT database-qualified. The
// templates take the database as a parameter (WithDatabase), so an unqualified
// one would silently write to whatever database the connection happened to
// default to — a different plane, with the same table names.
func TestPlaneTablesAreQualified(t *testing.T) {
	for _, w := range planeWriters {
		if !strings.HasPrefix(w.tmpl, "INSERT INTO %s.%s (") {
			t.Errorf("%s writer is not database-qualified: %q", w.table, first(w.tmpl, 60))
		}
	}
}

func first(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
