// Package telemetrysignal answers one question: which signals does a raw SQL query
// read?
//
// It used to be answered by sniffing the DATABASE name, back when each signal had a
// database of its own. Every signal now lives in the one event database, so the
// database says nothing and the TABLE is the only thing that does. That is the honest
// key anyway: a query reads metrics because it reads metric or series, not because of
// the schema it happens to be qualified by.
package telemetrysignal

import (
	"regexp"
	"strings"

	"github.com/hanzoai/o11y/pkg/telemetrylogs"
	"github.com/hanzoai/o11y/pkg/telemetrymetrics"
	"github.com/hanzoai/o11y/pkg/telemetrytraces"
)

// Used reports which signals a raw SQL query reads.
func Used(sql string) (metrics, logs, traces bool) {
	tables := tablesIn(sql)
	return anyOf(tables, metricTables), anyOf(tables, logTables), anyOf(tables, traceTables)
}

var (
	metricTables = []string{
		telemetrymetrics.MetricTableName,
		telemetrymetrics.Metric5mTableName,
		telemetrymetrics.Metric30mTableName,
		telemetrymetrics.HistogramTableName,
		telemetrymetrics.SeriesTableName,
		telemetrymetrics.Series6hTableName,
		telemetrymetrics.Series1dTableName,
		telemetrymetrics.Series1wTableName,
		telemetrymetrics.SeriesReducedTableName,
		telemetrymetrics.DescriptorTableName,
		telemetrymetrics.AttributeTableName,
	}
	logTables = []string{
		telemetrylogs.LogTableName,
		telemetrylogs.LogAttributeTableName,
		telemetrylogs.LogKeyTableName,
		telemetrylogs.LogResourceTableName,
		telemetrylogs.LogResourceKeyTableName,
	}
	traceTables = []string{
		telemetrytraces.SpanTableName,
		telemetrytraces.SpanAttributeTableName,
		telemetrytraces.SpanKeyTableName,
		telemetrytraces.SpanResourceTableName,
		telemetrytraces.OperationTableName,
		telemetrytraces.TraceTableName,
	}
)

// tableRef matches what FROM and JOIN name: an optionally qualified identifier, with
// each segment independently quotable (`event`.`log` quotes both, not the pair). The
// table is the last segment, so "event.span", "span" and the quoted forms all reduce
// to the same word — which is what makes a bare substring search wrong, since "span"
// appears inside "span_resource" and inside plenty of column names.
var tableRef = regexp.MustCompile(`(?i)\b(?:from|join)\s+(?:` + "`?" + `(\w+)` + "`?" + `\s*\.\s*)?` + "`?" + `(\w+)` + "`?")

func tablesIn(sql string) map[string]bool {
	out := map[string]bool{}
	for _, m := range tableRef.FindAllStringSubmatch(sql, -1) {
		out[strings.ToLower(strings.TrimPrefix(m[2], "distributed_"))] = true
	}
	return out
}

func anyOf(tables map[string]bool, names []string) bool {
	for _, n := range names {
		if tables[n] {
			return true
		}
	}
	return false
}
