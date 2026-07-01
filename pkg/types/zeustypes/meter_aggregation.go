package zeustypes

import "github.com/hanzoai/o11y/pkg/valuer"

type MeterAggregation struct {
	valuer.String
}

var (
	MeterAggregationSum = MeterAggregation{valuer.NewString("sum")}
	MeterAggregationMax = MeterAggregation{valuer.NewString("max")}
)
