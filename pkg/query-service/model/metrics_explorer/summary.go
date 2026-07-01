// Package metrics_explorer holds the request/response data model for the
// metrics explorer read paths (filter keys/values, summary list, treemap,
// related metrics and inspect). The types describe the shapes scanned from and
// returned by the datastore reader.
package metrics_explorer

import (
	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
)

// AvailableColumnFilterMap lists metric-summary columns that are real table
// columns (as opposed to JSON attribute keys) when building filter conditions.
var AvailableColumnFilterMap = map[string]struct{}{
	"metric_name": {},
	"description": {},
	"type":        {},
	"unit":        {},
	"temporality": {},
}

// FilterKeyRequest requests attribute keys available for metric filtering.
type FilterKeyRequest struct {
	Limit      int    `json:"limit"`
	SearchText string `json:"searchText"`
}

// FilterValueRequest requests attribute values for a given filter key.
type FilterValueRequest struct {
	FilterKey  string `json:"filterKey"`
	Limit      int    `json:"limit"`
	SearchText string `json:"searchText"`
}

// Attribute is a metric attribute key with a sampled value and its cardinality.
type Attribute struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	ValueCount uint64 `json:"valueCount"`
}

// SummaryListMetricsRequest requests a paged, filtered list of metric summaries.
type SummaryListMetricsRequest struct {
	Start   int64        `json:"start"`
	End     int64        `json:"end"`
	Limit   int          `json:"limit"`
	Offset  int          `json:"offset"`
	OrderBy v3.OrderBy   `json:"orderBy"`
	Filters v3.FilterSet `json:"filters"`
}

// MetricDetail is a single metric's summary row.
type MetricDetail struct {
	MetricName  string        `json:"metricName"`
	Description string        `json:"description"`
	MetricType  v3.MetricType `json:"metricType"`
	MetricUnit  string        `json:"metricUnit"`
	TimeSeries  uint64        `json:"timeSeries"`
	Samples     uint64        `json:"samples"`
}

// SummaryListMetricsResponse is a page of metric summaries.
type SummaryListMetricsResponse struct {
	Metrics []MetricDetail `json:"metrics"`
	Total   uint64         `json:"total"`
}

// TreeMapMetricsRequest requests the metric treemap for cardinality/sample share.
type TreeMapMetricsRequest struct {
	Start   int64        `json:"start"`
	End     int64        `json:"end"`
	Limit   int          `json:"limit"`
	Filters v3.FilterSet `json:"filters"`
}

// TreeMapResponseItem is one metric's contribution to the treemap total.
type TreeMapResponseItem struct {
	MetricName string  `json:"metricName"`
	TotalValue uint64  `json:"totalValue"`
	Percentage float64 `json:"percentage"`
}

// RelatedMetricsRequest requests metrics related to a given metric.
type RelatedMetricsRequest struct {
	CurrentMetricName string      `json:"currentMetricName"`
	Start             int64       `json:"start"`
	End               int64       `json:"end"`
	Filters           v3.FilterSet `json:"filters"`
}

// RelatedMetricsScore scores a related metric by name/attribute similarity.
type RelatedMetricsScore struct {
	NameSimilarity      float64        `json:"nameSimilarity"`
	AttributeSimilarity float64        `json:"attributeSimilarity"`
	Filters             [][]string     `json:"filters"`
	MetricType          v3.MetricType  `json:"metricType"`
	Temporality         v3.Temporality `json:"temporality"`
	IsMonotonic         bool           `json:"isMonotonic"`
}

// InspectMetricsRequest requests raw series for a single metric over a range.
type InspectMetricsRequest struct {
	MetricName string       `json:"metricName"`
	Start      int64        `json:"start"`
	End        int64        `json:"end"`
	Filters    v3.FilterSet `json:"filters"`
}

// InspectMetricsResponse holds the inspected series for a metric.
type InspectMetricsResponse struct {
	Series *[]v3.Series `json:"series"`
}
