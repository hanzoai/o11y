package datastoreprometheus

import (
	"time"

	"github.com/hanzoai/o11y/pkg/telemetrymetrics"
	"github.com/hanzoai/o11y/pkg/telemetryplane"
)

// The names the PromQL adapter reads by. o11y_metrics was dropped by HIP-0132,
// so every PromQL panel — the whole alerting evaluation path included — answered
// `Code: 81 ... UNKNOWN_DATABASE`. Aliases of pkg/telemetrymetrics/tables.go:
// time_series_v4 IS event.series and samples_v4 IS event.metric.
const (
	databaseName                string = telemetryplane.DBName
	distributedTimeSeriesV4     string = telemetrymetrics.SeriesTableName
	distributedTimeSeriesV46hrs string = telemetrymetrics.Series6hTableName
	distributedTimeSeriesV41day string = telemetrymetrics.Series1dTableName
	distributedSamplesV4        string = telemetrymetrics.MetricTableName
)

var (
	sixHoursInMilliseconds = time.Hour.Milliseconds() * 6
	oneDayInMilliseconds   = time.Hour.Milliseconds() * 24
)

// Returns the start time, end time and the table name to use for the query.
//
//	If time range is less than 6 hours, we need to use the `time_series_v4` table
//	else if time range is less than 1 day and greater than 6 hours, we need to use the `time_series_v4_6hrs` table
//	else we need to use the `time_series_v4_1day` table
func getStartAndEndAndTableName(start, end int64) (int64, int64, string) {
	var tableName string

	if end-start <= sixHoursInMilliseconds {
		// adjust the start time to nearest 1 hour
		start = start - (start % (time.Hour.Milliseconds() * 1))
		tableName = distributedTimeSeriesV4
	} else if end-start <= oneDayInMilliseconds {
		// adjust the start time to nearest 6 hours
		start = start - (start % (time.Hour.Milliseconds() * 6))
		tableName = distributedTimeSeriesV46hrs
	} else {
		// adjust the start time to nearest 1 day
		start = start - (start % (time.Hour.Milliseconds() * 24))
		tableName = distributedTimeSeriesV41day
	}

	return start, end, tableName
}
