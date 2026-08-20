package utils

import (
	"time"

	"github.com/hanzoai/o11y/pkg/telemetrymetrics"
)

var (
	sixHoursInMilliseconds = time.Hour.Milliseconds() * 6
	oneDayInMilliseconds   = time.Hour.Milliseconds() * 24
	oneWeekInMilliseconds  = oneDayInMilliseconds * 7
)

func WhichTSTableToUse(start, end int64) (int64, int64, string, string) {

	var tableName string
	var localTableName string
	if end-start < sixHoursInMilliseconds {
		// adjust the start time to nearest 1 hour
		start = start - (start % (time.Hour.Milliseconds() * 1))
		tableName = telemetrymetrics.SeriesTableName
		localTableName = telemetrymetrics.SeriesLocalTableName
	} else if end-start < oneDayInMilliseconds {
		// adjust the start time to nearest 6 hours
		start = start - (start % (time.Hour.Milliseconds() * 6))
		tableName = telemetrymetrics.Series6hTableName
		localTableName = telemetrymetrics.Series6hLocalTableName
	} else if end-start < oneWeekInMilliseconds {
		// adjust the start time to nearest 1 day
		start = start - (start % (time.Hour.Milliseconds() * 24))
		tableName = telemetrymetrics.Series1dTableName
		localTableName = telemetrymetrics.Series1dLocalTableName
	} else {
		// adjust the start time to nearest 1 week
		start = start - (start % (time.Hour.Milliseconds() * 24 * 7))
		tableName = telemetrymetrics.Series1wTableName
		localTableName = telemetrymetrics.Series1wLocalTableName
	}

	return start, end, tableName, localTableName
}

func WhichSampleTableToUse(start, end int64) (string, string) {
	if end-start < oneDayInMilliseconds {
		return telemetrymetrics.MetricTableName, "count(*)"
	} else if end-start < oneWeekInMilliseconds {
		return telemetrymetrics.Metric5mTableName, "sum(count)"
	} else {
		return telemetrymetrics.Metric30mTableName, "sum(count)"
	}
}

func WhichAttributesTableToUse(start, end int64) (int64, int64, string, string) {
	if end-start < sixHoursInMilliseconds {
		start = start - (start % (time.Hour.Milliseconds() * 6))
	}
	return start, end, telemetrymetrics.AttributeTableName, telemetrymetrics.AttributeLocalTableName
}
