package datastoreprometheus

import (
	"time"

	"github.com/hanzoai/o11y/pkg/telemetrymetrics"
)

// The metric plane is named in ONE place. This package used to keep its own copy of
// the database and table names, which is how a rename leaves half a codebase pointing
// at a table that no longer exists.
const (
	databaseName = telemetrymetrics.DBName
	metricTable  = telemetrymetrics.MetricTableName
)

var (
	sixHoursInMilliseconds = time.Hour.Milliseconds() * 6
	oneDayInMilliseconds   = time.Hour.Milliseconds() * 24
)

// Returns the start time, end time and the table name to use for the query.
//
// Pick the coarsest series rollup the window can afford: series under 6 hours,
// series_6h under a day, series_1d beyond that.
func getStartAndEndAndTableName(start, end int64) (int64, int64, string) {
	var tableName string

	if end-start <= sixHoursInMilliseconds {
		// adjust the start time to nearest 1 hour
		start = start - (start % (time.Hour.Milliseconds() * 1))
		tableName = telemetrymetrics.SeriesTableName
	} else if end-start <= oneDayInMilliseconds {
		// adjust the start time to nearest 6 hours
		start = start - (start % (time.Hour.Milliseconds() * 6))
		tableName = telemetrymetrics.Series6hTableName
	} else {
		// adjust the start time to nearest 1 day
		start = start - (start % (time.Hour.Milliseconds() * 24))
		tableName = telemetrymetrics.Series1dTableName
	}

	return start, end, tableName
}
