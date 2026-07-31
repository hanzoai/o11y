package telemetrymetrics

import (
	"fmt"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types/metrictypes"
)

// Metrics live in the ONE event database beside the other signals, named for what
// they are: a row of metric is one sample, a row of series is one series. The
// fingerprint/series model is unchanged — samples carry a UInt64 fingerprint that
// every query INNER JOINs through the series dimension table, because rate() and
// increase() need WINDOW PARTITION BY fingerprint over physically fingerprint-ordered
// rows, which no time-ordered table can express.
//
// There are no Distributed wrappers on this deployment, so the distributed and local
// name of a table are the same string. The pair is kept because the query builders
// distinguish the two roles, and a sharded deployment would reintroduce the split in
// exactly one place: here.
const (
	DBName = "event"

	DescriptorTableName      = "descriptor"
	DescriptorLocalTableName = "descriptor"

	MetricTableName      = "metric"
	MetricLocalTableName = "metric"

	Metric5mTableName      = "metric_5m"
	Metric5mLocalTableName = "metric_5m"

	Metric30mTableName      = "metric_30m"
	Metric30mLocalTableName = "metric_30m"

	SeriesTableName      = "series"
	SeriesLocalTableName = "series"

	Series6hTableName      = "series_6h"
	Series6hLocalTableName = "series_6h"

	Series1dTableName      = "series_1d"
	Series1dLocalTableName = "series_1d"

	Series1wTableName      = "series_1w"
	Series1wLocalTableName = "series_1w"
)

// The names below are NOT part of the applied event schema — the deployed database
// holds metric, metric_5m, metric_30m, series, series_6h, series_1d, series_1w and
// descriptor, and nothing else. They name the tables the exponential-histogram,
// reduced-metric and metric-metadata code paths read, so those paths are wired to
// the event plane the day the tables are added rather than carrying a second naming
// scheme. Until then a query through them fails on a missing table, which is the
// honest outcome.
const (
	// histogram holds exponential-histogram sketch points, which do not decompose
	// into plain samples.
	HistogramTableName      = "histogram"
	HistogramLocalTableName = "histogram"

	// The buffers hold raw points for ~24h; the reduced tables hold 60s aggregates
	// of dropped-label series.
	MetricBufferTableName       = "metric_buffer"
	MetricBufferLocalTableName  = "metric_buffer"
	SeriesBufferTableName       = "series_buffer"
	SeriesBufferLocalTableName  = "series_buffer"
	MetricReducedLastTableName  = "metric_reduced_last"
	MetricReducedSumTableName   = "metric_reduced_sum"
	SeriesReducedTableName      = "series_reduced"
	SeriesReducedLocalTableName = "series_reduced"

	// metric_attribute is one attribute of one metric — the metadata dimension.
	AttributeTableName      = "metric_attribute"
	AttributeLocalTableName = "metric_attribute"

	// reduction is one label-reduction rule.
	ReductionTableName = "reduction"
)

var (
	oneHourInMilliseconds  = uint64(time.Hour.Milliseconds() * 1)
	sixHoursInMilliseconds = uint64(time.Hour.Milliseconds() * 6)
	oneDayInMilliseconds   = uint64(time.Hour.Milliseconds() * 24)
	oneWeekInMilliseconds  = uint64(oneDayInMilliseconds * 7)

	// when the query requests for almost 1 day, but not exactly 1 day, we need to add an offset to the end time
	// to make sure that we are using the correct table
	// this is because the start gets adjusted to the nearest step interval and uses the 5m table for 4m step interval
	// leading to time series that doesn't best represent the rate of change.
	offsetBucket = uint64(60 * time.Minute.Milliseconds())
)

// WhichTSTableToUse returns adjusted start, adjusted end, distributed table name, local table name
// in that order.
func WhichTSTableToUse(
	start, end uint64,
	useBuffer bool,
	tableHints *metrictypes.MetricTableHints,
) (uint64, uint64, string, string) {
	// the buffer holds the recent raw window for reduced metrics and has the same
	// shape as series; round the start to the hour like series does.
	if useBuffer {
		start = start - (start % (oneHourInMilliseconds))
		return start, end, SeriesBufferTableName, SeriesBufferLocalTableName
	}

	// if we have a hint for the table, we need to use it
	// the hint will be used to override the default table selection logic
	if tableHints != nil {
		if tableHints.TimeSeriesTableName != "" {
			var distributedTableName string
			switch tableHints.TimeSeriesTableName {
			case SeriesLocalTableName:
				// adjust the start time to nearest 1 hour
				start = start - (start % (oneHourInMilliseconds))
				distributedTableName = SeriesTableName
			case Series6hLocalTableName:
				// adjust the start time to nearest 6 hours
				start = start - (start % (sixHoursInMilliseconds))
				distributedTableName = Series6hTableName
			case Series1dLocalTableName:
				// adjust the start time to nearest 1 day
				start = start - (start % (oneDayInMilliseconds))
				distributedTableName = Series1dTableName
			case Series1wLocalTableName:
				// adjust the start time to nearest 1 week
				start = start - (start % (oneWeekInMilliseconds))
				distributedTableName = Series1wTableName
			}
			return start, end, distributedTableName, tableHints.TimeSeriesTableName
		}
	}

	// Pick the coarsest rollup the window can afford: series under 6 hours, then
	// series_6h under a day, series_1d under a week, series_1w beyond that.
	var distributedTableName string
	var localTableName string
	if end-start < sixHoursInMilliseconds {
		// adjust the start time to nearest 1 hour
		start = start - (start % (oneHourInMilliseconds))
		distributedTableName = SeriesTableName
		localTableName = SeriesLocalTableName
	} else if end-start < oneDayInMilliseconds {
		// adjust the start time to nearest 6 hours
		start = start - (start % (sixHoursInMilliseconds))
		distributedTableName = Series6hTableName
		localTableName = Series6hLocalTableName
	} else if end-start < oneWeekInMilliseconds {
		// adjust the start time to nearest 1 day
		start = start - (start % (oneDayInMilliseconds))
		distributedTableName = Series1dTableName
		localTableName = Series1dLocalTableName
	} else {
		// adjust the start time to nearest 1 week
		start = start - (start % (oneWeekInMilliseconds))
		distributedTableName = Series1wTableName
		localTableName = Series1wLocalTableName
	}

	return start, end, distributedTableName, localTableName
}

// CountExpressionForSamplesTable returns the count expression for a given samples table name.
// For the raw tables (metric, histogram) it returns "count(*)"; for the rollups
// (metric_5m, metric_30m), whose rows already carry a count, it returns "sum(count)".
func CountExpressionForSamplesTable(tableName string) string {
	// Non-aggregated tables use count(*)
	if tableName == MetricTableName ||
		tableName == MetricLocalTableName ||
		tableName == HistogramTableName ||
		tableName == HistogramLocalTableName {
		return "count(*)"
	}
	// Aggregated tables use sum(count)
	return "sum(count)"
}

// ValueColumnForSamplesTable returns the column name holding the sample value:
// "last" for the 5m/30m aggregated tables, "value" otherwise.
// note all the other columns in the aggregated samples tables are nothing but aggregations.
// and so "last" is the value column for these tables.
func ValueColumnForSamplesTable(tableName string) string {
	if tableName == Metric5mTableName || tableName == Metric30mTableName {
		return "last"
	}
	return "value"
}

// WhichSamplesTableToUse returns the distributed and local samples table names
// (in that order) appropriate for the given window, metric type, and time aggregation.
//
// start and end are in milliseconds. We have three tables for samples:
//  1. metric
//  2. metric_5m — for queries with time range >= 1 day and < 1 week
//  3. metric_30m — for queries with time range >= 1 week
//
// If the `timeAggregation` is `count_distinct` we can't use the aggregated tables
// because they don't support it.
func WhichSamplesTableToUse(
	start, end uint64,
	metricType metrictypes.Type,
	timeAggregation metrictypes.TimeAggregation,
	useBuffer bool,
	tableHints *metrictypes.MetricTableHints,
) (string, string) {
	// the buffer holds the recent raw window for reduced metrics; same shape as metric
	if useBuffer {
		return MetricBufferTableName, MetricBufferLocalTableName
	}

	// if we have a hint for the table, we need to use it
	// the hint will be used to override the default table selection logic.
	// SamplesTableName is the distributed name; derive the local via switch.
	if tableHints != nil && tableHints.SamplesTableName != "" {
		switch tableHints.SamplesTableName {
		case MetricTableName, MetricBufferTableName:
			return MetricTableName, MetricLocalTableName
		case Metric5mTableName:
			return Metric5mTableName, Metric5mLocalTableName
		case Metric30mTableName:
			return Metric30mTableName, Metric30mLocalTableName
		case HistogramTableName:
			return HistogramTableName, HistogramLocalTableName
		}
		return tableHints.SamplesTableName, tableHints.SamplesTableName
	}

	// we don't have any aggregated table for sketches (yet)
	if metricType == metrictypes.ExpHistogramType {
		return HistogramTableName, HistogramLocalTableName
	}

	// if the time aggregation is count_distinct, we need to use the raw metric table
	// because the aggregated tables don't support count_distinct
	if timeAggregation == metrictypes.TimeAggregationCountDistinct {
		return MetricTableName, MetricLocalTableName
	}

	if end-start < oneDayInMilliseconds+offsetBucket {
		return MetricTableName, MetricLocalTableName
	} else if end-start < oneWeekInMilliseconds+offsetBucket {
		return Metric5mTableName, Metric5mLocalTableName
	}
	return Metric30mTableName, Metric30mLocalTableName
}

func AggregationColumnForSamplesTable(
	tableName string,
	temporality metrictypes.Temporality,
	timeAggregation metrictypes.TimeAggregation,
) (string, error) {
	var aggregationColumn string
	switch temporality {
	case metrictypes.Delta:
		// for delta metrics, we only support `RATE`/`INCREASE` both of which are sum
		// although it doesn't make sense to use anyLast, avg, min, max, count on delta metrics,
		// we are keeping it here to make sure that query will not be invalid
		switch tableName {
		case MetricTableName, MetricBufferTableName:
			switch timeAggregation {
			case metrictypes.TimeAggregationLatest:
				aggregationColumn = "anyLast(value)"
			case metrictypes.TimeAggregationSum:
				aggregationColumn = "sum(value)"
			case metrictypes.TimeAggregationAvg:
				aggregationColumn = "avg(value)"
			case metrictypes.TimeAggregationMin:
				aggregationColumn = "min(value)"
			case metrictypes.TimeAggregationMax:
				aggregationColumn = "max(value)"
			case metrictypes.TimeAggregationCount:
				aggregationColumn = "count(value)"
			case metrictypes.TimeAggregationCountDistinct:
				aggregationColumn = "countDistinct(value)"
			case metrictypes.TimeAggregationRate, metrictypes.TimeAggregationIncrease: // only these two options give meaningful results
				aggregationColumn = "sum(value)"
			}
		case Metric5mTableName, Metric30mTableName:
			switch timeAggregation {
			case metrictypes.TimeAggregationLatest:
				aggregationColumn = "anyLast(last)"
			case metrictypes.TimeAggregationSum:
				aggregationColumn = "sum(sum)"
			case metrictypes.TimeAggregationAvg:
				aggregationColumn = "sum(sum) / sum(count)"
			case metrictypes.TimeAggregationMin:
				aggregationColumn = "min(min)"
			case metrictypes.TimeAggregationMax:
				aggregationColumn = "max(max)"
			case metrictypes.TimeAggregationCount:
				aggregationColumn = "sum(count)"
			// count_distinct is not supported in aggregated tables
			case metrictypes.TimeAggregationRate, metrictypes.TimeAggregationIncrease: // only these two options give meaningful results
				aggregationColumn = "sum(sum)"
			}
		}
	case metrictypes.Cumulative:
		// for cumulative metrics, we only support `RATE`/`INCREASE`. The max value in window is
		// used to calculate the sum which is then divided by the window size to get the rate
		switch tableName {
		case MetricTableName, MetricBufferTableName:
			switch timeAggregation {
			case metrictypes.TimeAggregationLatest:
				aggregationColumn = "anyLast(value)"
			case metrictypes.TimeAggregationSum:
				aggregationColumn = "sum(value)"
			case metrictypes.TimeAggregationAvg:
				aggregationColumn = "avg(value)"
			case metrictypes.TimeAggregationMin:
				aggregationColumn = "min(value)"
			case metrictypes.TimeAggregationMax:
				aggregationColumn = "max(value)"
			case metrictypes.TimeAggregationCount:
				aggregationColumn = "count(value)"
			case metrictypes.TimeAggregationCountDistinct:
				aggregationColumn = "countDistinct(value)"
			case metrictypes.TimeAggregationRate, metrictypes.TimeAggregationIncrease: // only these two options give meaningful results
				aggregationColumn = "max(value)"
			}
		case Metric5mTableName, Metric30mTableName:
			switch timeAggregation {
			case metrictypes.TimeAggregationLatest:
				aggregationColumn = "anyLast(last)"
			case metrictypes.TimeAggregationSum:
				aggregationColumn = "sum(sum)"
			case metrictypes.TimeAggregationAvg:
				aggregationColumn = "sum(sum) / sum(count)"
			case metrictypes.TimeAggregationMin:
				aggregationColumn = "min(min)"
			case metrictypes.TimeAggregationMax:
				aggregationColumn = "max(max)"
			case metrictypes.TimeAggregationCount:
				aggregationColumn = "sum(count)"
			// count_distinct is not supported in aggregated tables
			case metrictypes.TimeAggregationRate, metrictypes.TimeAggregationIncrease: // only these two options give meaningful results
				aggregationColumn = "max(max)"
			}
		}
	case metrictypes.Unspecified:
		switch tableName {
		case MetricTableName, MetricBufferTableName:
			switch timeAggregation {
			case metrictypes.TimeAggregationLatest:
				aggregationColumn = "anyLast(value)"
			case metrictypes.TimeAggregationSum:
				aggregationColumn = "sum(value)"
			case metrictypes.TimeAggregationAvg:
				aggregationColumn = "avg(value)"
			case metrictypes.TimeAggregationMin:
				aggregationColumn = "min(value)"
			case metrictypes.TimeAggregationMax:
				aggregationColumn = "max(value)"
			case metrictypes.TimeAggregationCount:
				aggregationColumn = "count(value)"
			case metrictypes.TimeAggregationCountDistinct:
				aggregationColumn = "countDistinct(value)"
			case metrictypes.TimeAggregationRate, metrictypes.TimeAggregationIncrease: // ideally, this should never happen
				aggregationColumn = "sum(value)"
			}
		case Metric5mTableName, Metric30mTableName:
			switch timeAggregation {
			case metrictypes.TimeAggregationLatest:
				aggregationColumn = "anyLast(last)"
			case metrictypes.TimeAggregationSum:
				aggregationColumn = "sum(sum)"
			case metrictypes.TimeAggregationAvg:
				aggregationColumn = "sum(sum) / sum(count)"
			case metrictypes.TimeAggregationMin:
				aggregationColumn = "min(min)"
			case metrictypes.TimeAggregationMax:
				aggregationColumn = "max(max)"
			case metrictypes.TimeAggregationCount:
				aggregationColumn = "sum(count)"
			// count_distinct is not supported in aggregated tables
			case metrictypes.TimeAggregationRate, metrictypes.TimeAggregationIncrease: // ideally, this should never happen
				aggregationColumn = "sum(sum)"
			}
		}
	}
	if aggregationColumn == "" {
		return "", errors.NewInvalidInputf(
			errors.CodeInvalidInput,
			"invalid time aggregation, should be one of the following: [`latest`, `sum`, `avg`, `min`, `max`, `count`, `rate`, `increase`]",
		)
	}
	return aggregationColumn, nil
}

// WhichReducedSamplesTableToUse returns the 60s reduced samples table for a metric
// type: the last_60s table for gauge-like series, the sum_60s table for counters
// and histograms.
func WhichReducedSamplesTableToUse(metricType metrictypes.Type) string {
	if metricType == metrictypes.SumType || metricType == metrictypes.HistogramType {
		return MetricReducedSumTableName
	}
	return MetricReducedLastTableName
}

// ReducedValueColumn returns the reduced value column (and the avg-denominator
// weight) for a space aggregation. The reduced columns are pre-aggregated across
// the original series, so the space aggregation picks the underlying value; the
// sum table only has `sum`, so min/max across series have no column (ok=false).
func ReducedValueColumn(metricType metrictypes.Type, space metrictypes.SpaceAggregation) (value, weight string, ok bool) {
	if metricType == metrictypes.SumType || metricType == metrictypes.HistogramType {
		switch space {
		case metrictypes.SpaceAggregationSum:
			return "`sum`", "", true
		case metrictypes.SpaceAggregationAvg:
			return "`sum`", "`count_series`", true
		}
		return "", "", false
	}
	switch space {
	case metrictypes.SpaceAggregationSum:
		return "`sum_last`", "", true
	case metrictypes.SpaceAggregationAvg:
		return "`sum_last`", "`count_series`", true
	case metrictypes.SpaceAggregationMin:
		return "`min`", "", true
	case metrictypes.SpaceAggregationMax:
		return "`max`", "", true
	}
	return "", "", false
}

// ReducedTimeAggregationColumn applies the time aggregation to the reduced `value`
// column over the step's 60s buckets. latest uses argMax over the bucket timestamp
// (the buckets have no read order); rate divides the per-step sum by the step.
func ReducedTimeAggregationColumn(timeAggregation metrictypes.TimeAggregation, stepSec int64) string {
	switch timeAggregation {
	case metrictypes.TimeAggregationLatest:
		return "argMax(value, unix_milli)"
	case metrictypes.TimeAggregationAvg:
		return "avg(value)"
	case metrictypes.TimeAggregationMin:
		return "min(value)"
	case metrictypes.TimeAggregationMax:
		return "max(value)"
	case metrictypes.TimeAggregationCount:
		return "count(value)"
	case metrictypes.TimeAggregationRate:
		return fmt.Sprintf("sum(value) / %d", stepSec)
	default: // sum, increase
		return "sum(value)"
	}
}

func AggregationQueryForHistogramCountWithParams(param *metrictypes.ComparisonSpaceAggregationParam) (string, error) {
	if param == nil {
		return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "no aggregation param provided for histogram count")
	}
	histogramCountThreshold := param.Threshold

	switch param.Operater {
	case "<=":
		return fmt.Sprintf("argMaxIf(value, toFloat64(le), toFloat64(le) <= %f) + (argMinIf(value, toFloat64(le), toFloat64(le) > %f) - argMaxIf(value, toFloat64(le), toFloat64(le) <= %f)) * (%f - maxIf(toFloat64(le), toFloat64(le) <= %f)) / (minIf(toFloat64(le), toFloat64(le) > %f) - maxIf(toFloat64(le), toFloat64(le) <= %f)) AS value", histogramCountThreshold, histogramCountThreshold, histogramCountThreshold, histogramCountThreshold, histogramCountThreshold, histogramCountThreshold, histogramCountThreshold), nil
	case ">":
		return fmt.Sprintf("argMax(value, toFloat64(le)) - (argMaxIf(value, toFloat64(le), toFloat64(le) <= %f) + (argMinIf(value, toFloat64(le), toFloat64(le) > %f) - argMaxIf(value, toFloat64(le), toFloat64(le) <= %f)) * (%f - maxIf(toFloat64(le), toFloat64(le) <= %f)) / (minIf(toFloat64(le), toFloat64(le) > %f) - maxIf(toFloat64(le), toFloat64(le) <= %f))) AS value", histogramCountThreshold, histogramCountThreshold, histogramCountThreshold, histogramCountThreshold, histogramCountThreshold, histogramCountThreshold, histogramCountThreshold), nil
	default:
		return "", errors.NewInvalidInputf(errors.CodeInvalidInput, "invalid space aggregation operator, should be one of the following: [`<=`, `>`]")
	}

}
