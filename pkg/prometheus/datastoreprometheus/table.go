package datastoreprometheus

import (
	"github.com/hanzoai/o11y/pkg/telemetrymetrics"
)

// PromQL reads the same two tables every other metrics query reads, so it reads
// their names from telemetrymetrics and nowhere else. This package used to keep
// its own copy — o11y_metrics.distributed_time_series_v4 and friends — and that
// is how it went on addressing a database that stopped existing when metrics
// moved into the one event plane: a copy has no reason to change when the
// original does. A second spelling of a physical name is a second thing to
// migrate, and the one the migration does not read is the one that rots.
const (
	databaseName     = telemetrymetrics.DBName
	samplesTableName = telemetrymetrics.MetricTableName
)

// getStartAndEndAndTableName returns the start time, end time and the series
// table to read, picking the coarsest rollup the requested window can afford.
//
// The choice is telemetrymetrics.WhichTSTableToUse — the same function every
// other metrics query builder calls, so one window resolves to one table across
// the whole service. PromQL has no read buffer and no table hints to offer it.
func getStartAndEndAndTableName(start, end int64) (int64, int64, string) {
	adjustedStart, adjustedEnd, tableName, _ := telemetrymetrics.WhichTSTableToUse(
		uint64(start), uint64(end), false, nil,
	)
	return int64(adjustedStart), int64(adjustedEnd), tableName
}
