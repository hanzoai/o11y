package o11y

// The METRICS face — the metrics-explorer reads and the volume-control
// (metric reduction) rules — as TYPED ops.
//
// These nineteen routes reached traffic only through the delegation wildcard,
// and a route behind a wildcard is in no document: no SDK method, no CLI
// command, no agent tool, no reference page. Typing them is what puts the
// metrics surface in the document and therefore in every projection built
// from it.
//
// THE WIRE DOES NOT MOVE, the same way it did not move for the telemetry
// face (telemetry.go): these ops do not re-implement anything. Each hands the
// call to the SAME runtime handler the wildcard delegates to (see
// metricsRelay), so identity resolution, the org gate, the ROLE CHECK the mux
// registration declared — ViewAccess on the reads, EditAccess on the metadata
// write, AdminAccess on every reduction-rule mutation — the audit record and
// the success envelope are all still the runtime's, executed in the order
// they always were. What is new is the TYPE and the prose that goes with it.
//
// Registered ahead of the wildcard; specific-beats-wildcard is what the
// router does regardless of registration order, so these nineteen paths
// dispatch here and every other path under the prefix reaches the runtime
// untouched (metrics_test.go pins both halves).

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/zap-proto/zip"
)

// metricsRoot is the face's path root, spelled ONCE: the group the ops
// register on, and the first segment metricsRelay rebuilds a call onto.
const metricsRoot = "/v1/o11y"

// mountMetrics registers the nineteen typed metrics ops on the native router.
// Collection routes register before the parameterised ones so an id can never
// shadow a collection.
func mountMetrics(app *zip.App) {
	g := app.Group(metricsRoot)

	zip.Get(g, "/metrics", listMetrics, zip.WithOperationID("ListMetrics"))
	zip.Post(g, "/metrics/stats", metricStats, zip.WithOperationID("GetMetricsStats"))
	zip.Post(g, "/metrics/treemap", metricTreemap, zip.WithOperationID("GetMetricsTreemap"))
	zip.Get(g, "/metrics/attributes", metricAttributes, zip.WithOperationID("GetMetricAttributes"))
	zip.Get(g, "/metrics/metadata", metricMetadata, zip.WithOperationID("GetMetricMetadata"))
	zip.Post(g, "/metrics/metadata", saveMetricMetadata, zip.WithOperationID("UpdateMetricMetadata"))
	zip.Get(g, "/metrics/highlights", metricHighlights, zip.WithOperationID("GetMetricHighlights"))
	zip.Get(g, "/metrics/alerts", metricAlerts, zip.WithOperationID("GetMetricAlerts"))
	zip.Get(g, "/metrics/dashboards", metricDashboards, zip.WithOperationID("GetMetricDashboardsV2"))
	zip.Post(g, "/metrics/inspect", inspectMetric, zip.WithOperationID("InspectMetrics"))
	zip.Get(g, "/metrics/onboarding", metricsOnboarding, zip.WithOperationID("GetMetricsOnboardingStatus"))

	zip.Get(g, "/metric_reduction_rules", listReductionRules, zip.WithOperationID("ListMetricReductionRules"))
	zip.Post(g, "/metric_reduction_rules", createReductionRule, zip.WithOperationID("CreateMetricReductionRule"), zip.WithStatus(http.StatusCreated))
	zip.Get(g, "/metric_reduction_rules/stats", reductionRuleStats, zip.WithOperationID("GetMetricReductionRuleStats"))
	zip.Get(g, "/metric_reduction_rules/timeseries", reductionRuleTimeseries, zip.WithOperationID("GetMetricReductionRuleTimeseries"))
	zip.Post(g, "/metric_reduction_rules/preview", previewReductionRule, zip.WithOperationID("PreviewMetricReductionRule"))
	zip.Get(g, "/metric_reduction_rules/:id", reductionRule, zip.WithOperationID("GetMetricReductionRuleByID"))
	zip.Put(g, "/metric_reduction_rules/:id", saveReductionRule, zip.WithOperationID("UpdateMetricReductionRuleByID"))
	zip.Delete(g, "/metric_reduction_rules/:id", deleteReductionRule, zip.WithOperationID("DeleteMetricReductionRuleByID"))
}

// ── the metrics-explorer operations ───────────────────────────────────────────

// listMetrics lists the distinct metric names seen in a time range, each with
// its description, type, unit, temporality and monotonicity.
func listMetrics(ctx context.Context, in *O11yMetricListIn) (*O11yMetricListOut, error) {
	out := new(O11yMetricListOut)
	return out, metricsRelay(ctx, http.MethodGet, "/metrics", query(
		"start", int(in.Start),
		"end", int(in.End),
		"limit", in.Limit,
		"searchText", in.SearchText,
		"source", in.Source,
	), nil, out)
}

// metricStats lists metrics with their sample and time-series counts for a
// time range — the volume view of the metrics explorer, pageable and sortable.
func metricStats(ctx context.Context, in *O11yMetricStatsIn) (*O11yMetricStatsOut, error) {
	out := new(O11yMetricStatsOut)
	return out, metricsRelay(ctx, http.MethodPost, "/metrics/stats", nil, in, out)
}

// metricTreemap returns the proportional distribution of metrics by sample
// count or time-series count, as the entries of a treemap.
func metricTreemap(ctx context.Context, in *O11yMetricTreemapIn) (*O11yMetricTreemapOut, error) {
	out := new(O11yMetricTreemapOut)
	return out, metricsRelay(ctx, http.MethodPost, "/metrics/treemap", nil, in, out)
}

// metricAttributes returns one metric's attribute keys, each with its unique
// values and their count.
func metricAttributes(ctx context.Context, in *O11yMetricAttributesIn) (*O11yMetricAttributesOut, error) {
	out := new(O11yMetricAttributesOut)
	return out, metricsRelay(ctx, http.MethodGet, "/metrics/attributes", query(
		"metricName", in.MetricName,
		"start", int(in.Start),
		"end", int(in.End),
	), nil, out)
}

// metricMetadata returns one metric's metadata: description, type, unit,
// temporality and monotonicity.
func metricMetadata(ctx context.Context, in *O11yMetricNameIn) (*O11yMetricMetadataOut, error) {
	out := new(O11yMetricMetadataOut)
	return out, metricsRelay(ctx, http.MethodGet, "/metrics/metadata", query(
		"metricName", in.MetricName,
	), nil, out)
}

// saveMetricMetadata updates one metric's metadata — description, type, unit,
// temporality, monotonicity — and answers with the bare success envelope.
func saveMetricMetadata(ctx context.Context, in *O11yMetricMetadataSaveIn) (*O11yMetricAckOut, error) {
	out := new(O11yMetricAckOut)
	return out, metricsRelay(ctx, http.MethodPost, "/metrics/metadata", nil, in, out)
}

// metricHighlights returns one metric's headline numbers: data points, total
// and active time series, and when it was last received.
func metricHighlights(ctx context.Context, in *O11yMetricNameIn) (*O11yMetricHighlightsOut, error) {
	out := new(O11yMetricHighlightsOut)
	return out, metricsRelay(ctx, http.MethodGet, "/metrics/highlights", query(
		"metricName", in.MetricName,
	), nil, out)
}

// metricAlerts lists the alert rules that reference a metric.
func metricAlerts(ctx context.Context, in *O11yMetricNameIn) (*O11yMetricAlertsOut, error) {
	out := new(O11yMetricAlertsOut)
	return out, metricsRelay(ctx, http.MethodGet, "/metrics/alerts", query(
		"metricName", in.MetricName,
	), nil, out)
}

// metricDashboards lists the dashboard panels that reference a metric.
func metricDashboards(ctx context.Context, in *O11yMetricNameIn) (*O11yMetricDashboardsOut, error) {
	out := new(O11yMetricDashboardsOut)
	return out, metricsRelay(ctx, http.MethodGet, "/metrics/dashboards", query(
		"metricName", in.MetricName,
	), nil, out)
}

// inspectMetric returns one metric's raw time series over a window of at most
// thirty minutes — each series with its labels and timestamp/value pairs.
func inspectMetric(ctx context.Context, in *O11yMetricInspectIn) (*O11yMetricInspectOut, error) {
	out := new(O11yMetricInspectOut)
	return out, metricsRelay(ctx, http.MethodPost, "/metrics/inspect", nil, in, out)
}

// metricsOnboarding reports whether any non-O11y metrics have been ingested —
// the lightweight check onboarding polls.
func metricsOnboarding(ctx context.Context, _ *struct{}) (*O11yMetricOnboardingOut, error) {
	out := new(O11yMetricOnboardingOut)
	return out, metricsRelay(ctx, http.MethodGet, "/metrics/onboarding", nil, nil, out)
}

// ── the metric reduction rule operations ──────────────────────────────────────

// listReductionRules lists the org's metric volume-control (label reduction)
// rules, pageable and sortable by name, volume or recency.
func listReductionRules(ctx context.Context, in *O11yReductionRuleListIn) (*O11yReductionRuleListOut, error) {
	out := new(O11yReductionRuleListOut)
	return out, metricsRelay(ctx, http.MethodGet, "/metric_reduction_rules", query(
		"orderBy", in.OrderBy,
		"order", in.Order,
		"search", in.Search,
		"metricName", in.MetricName,
		"offset", in.Offset,
		"limit", in.Limit,
	), nil, out)
}

// createReductionRule creates a volume-control rule for a metric and returns
// it with its id; a metric that already has a rule is refused.
func createReductionRule(ctx context.Context, in *O11yReductionRuleCreateIn) (*O11yReductionRuleOut, error) {
	out := new(O11yReductionRuleOut)
	return out, metricsRelay(ctx, http.MethodPost, "/metric_reduction_rules", nil, in, out)
}

// reductionRuleStats returns total ingested vs retained series and samples and
// the estimated monthly savings across all volume-control rules.
func reductionRuleStats(ctx context.Context, _ *struct{}) (*O11yReductionStatsOut, error) {
	out := new(O11yReductionStatsOut)
	return out, metricsRelay(ctx, http.MethodGet, "/metric_reduction_rules/stats", nil, nil, out)
}

// reductionRuleTimeseries returns ingested vs retained series over time across
// all volume-control rules, in hourly buckets, in the query-range time-series
// response shape.
func reductionRuleTimeseries(ctx context.Context, _ *struct{}) (*O11yReductionSeriesOut, error) {
	out := new(O11yReductionSeriesOut)
	return out, metricsRelay(ctx, http.MethodGet, "/metric_reduction_rules/timeseries", nil, nil, out)
}

// previewReductionRule estimates the series reduction and the dashboards and
// alerts a candidate volume-control rule would touch, without persisting it.
func previewReductionRule(ctx context.Context, in *O11yReductionRulePreviewIn) (*O11yReductionRulePreviewOut, error) {
	out := new(O11yReductionRulePreviewOut)
	return out, metricsRelay(ctx, http.MethodPost, "/metric_reduction_rules/preview", nil, in, out)
}

// reductionRule returns one volume-control rule by its id.
func reductionRule(ctx context.Context, in *O11yReductionRuleRef) (*O11yReductionRuleOut, error) {
	out := new(O11yReductionRuleOut)
	// The id goes on VERBATIM, as the segment the router matched — re-encoding
	// it here would hand the runtime a different id than the caller named.
	return out, metricsRelay(ctx, http.MethodGet, "/metric_reduction_rules/"+in.ID, nil, nil, out)
}

// saveReductionRule updates the match type and labels of a volume-control rule
// by its id; the metric name is immutable.
func saveReductionRule(ctx context.Context, in *O11yReductionRuleSaveIn) (*O11yReductionRuleOut, error) {
	out := new(O11yReductionRuleOut)
	// The whole In rides as the body, id included. The runtime's decoder is
	// tolerant and binds only matchType and labels; the PATH is the addressing
	// authority at both hops, exactly as it was on the mux tree.
	return out, metricsRelay(ctx, http.MethodPut, "/metric_reduction_rules/"+in.ID, nil, in, out)
}

// deleteReductionRule deletes a volume-control rule by its id.
func deleteReductionRule(ctx context.Context, in *O11yReductionRuleRef) (*struct{}, error) {
	return nil, metricsRelay(ctx, http.MethodDelete, "/metric_reduction_rules/"+in.ID, nil, nil, nil)
}

// ── inputs ────────────────────────────────────────────────────────────────────
//
// Every type carries the O11y qualifier because these names enter a fleet-wide
// document, where one name may have exactly one shape. Each In names the wire
// the mux tree has always read — the same parameter names, the same JSON
// fields, in the same order.

// O11yMetricListIn selects a page of metric names.
type O11yMetricListIn struct {
	// Start is the start of the window as a Unix timestamp in milliseconds.
	Start int64 `json:"start,omitempty"`
	// End is the end of the window as a Unix timestamp in milliseconds.
	End int64 `json:"end,omitempty"`
	// Limit caps how many metrics come back; unset means 100, at most 5000.
	Limit int `json:"limit,omitempty"`
	// SearchText narrows the page to metric names containing it.
	SearchText string `json:"searchText,omitempty"`
	// Source narrows the page by ingestion source.
	Source string `json:"source,omitempty"`
}

// O11yMetricFilter narrows a metrics read with a filter expression.
type O11yMetricFilter struct {
	// Expression is the filter, in the query-builder filter syntax.
	Expression string `json:"expression"`
}

// O11yMetricField names a telemetry field: the column an order-by key or a
// series label refers to.
type O11yMetricField struct {
	// Name is the field's name.
	Name string `json:"name"`
	// Description describes the field.
	Description string `json:"description,omitempty"`
	// Unit is the field's unit.
	Unit string `json:"unit,omitempty"`
	// Signal is the telemetry signal the field belongs to.
	Signal string `json:"signal,omitempty"`
	// FieldContext is the context the field lives in, e.g. resource, attribute.
	FieldContext string `json:"fieldContext,omitempty"`
	// FieldDataType is the field's data type.
	FieldDataType string `json:"fieldDataType,omitempty"`
}

// O11yMetricOrder orders a metrics read.
type O11yMetricOrder struct {
	// Key is the field to order by.
	Key O11yMetricField `json:"key"`
	// Direction is asc or desc.
	Direction string `json:"direction"`
}

// O11yMetricStatsIn selects a page of metric volume statistics.
type O11yMetricStatsIn struct {
	// Filter narrows the metrics counted.
	Filter *O11yMetricFilter `json:"filter,omitempty"`
	// Start is the start of the window as a Unix timestamp in milliseconds. Required.
	Start int64 `json:"start" validate:"required"`
	// End is the end of the window as a Unix timestamp in milliseconds. Required.
	End int64 `json:"end" validate:"required"`
	// Limit caps how many metrics come back, between 1 and 5000. Required.
	Limit int `json:"limit" validate:"required"`
	// Offset is how many metrics to skip, for paging.
	Offset int `json:"offset"`
	// OrderBy sorts the page, by samples or timeseries.
	OrderBy *O11yMetricOrder `json:"orderBy,omitempty"`
}

// O11yMetricTreemapIn selects a treemap of metric volume.
type O11yMetricTreemapIn struct {
	// Filter narrows the metrics counted.
	Filter *O11yMetricFilter `json:"filter,omitempty"`
	// Start is the start of the window as a Unix timestamp in milliseconds. Required.
	Start int64 `json:"start" validate:"required"`
	// End is the end of the window as a Unix timestamp in milliseconds. Required.
	End int64 `json:"end" validate:"required"`
	// Limit caps how many entries come back, between 1 and 5000. Required.
	Limit int `json:"limit" validate:"required"`
	// Mode picks the measure: timeseries or samples. Required.
	Mode string `json:"mode" validate:"required"`
}

// O11yMetricNameIn names one metric.
type O11yMetricNameIn struct {
	// MetricName is the metric's name; it may contain slashes, e.g.
	// run.googleapis.com/request_latencies. Required.
	MetricName string `json:"metricName" validate:"required"`
}

// O11yMetricAttributesIn selects one metric's attribute keys and values.
type O11yMetricAttributesIn struct {
	// MetricName is the metric's name; it may contain slashes. Required.
	MetricName string `json:"metricName" validate:"required"`
	// Start is the start of the window as a Unix timestamp in milliseconds.
	Start int64 `json:"start,omitempty"`
	// End is the end of the window as a Unix timestamp in milliseconds.
	End int64 `json:"end,omitempty"`
}

// O11yMetricMetadataSaveIn is one metric's metadata, whole — the write
// replaces every field it names.
type O11yMetricMetadataSaveIn struct {
	// MetricName is the metric to update. Required.
	MetricName string `json:"metricName" validate:"required"`
	// Type is the metric type, e.g. gauge, sum, histogram.
	Type string `json:"type"`
	// Description describes the metric.
	Description string `json:"description"`
	// Unit is the metric's unit.
	Unit string `json:"unit"`
	// Temporality is delta or cumulative.
	Temporality string `json:"temporality"`
	// IsMonotonic marks a sum that only ever increases.
	IsMonotonic bool `json:"isMonotonic"`
}

// O11yMetricInspectIn selects one metric's raw series over a short window.
type O11yMetricInspectIn struct {
	// MetricName is the metric to inspect. Required.
	MetricName string `json:"metricName" validate:"required"`
	// Start is the start of the window as a Unix timestamp in milliseconds. Required.
	Start int64 `json:"start" validate:"required"`
	// End is the end of the window as a Unix timestamp in milliseconds, at most
	// thirty minutes after start. Required.
	End int64 `json:"end" validate:"required"`
	// Filter narrows the series returned.
	Filter *O11yMetricFilter `json:"filter,omitempty"`
}

// O11yReductionRuleListIn selects a page of volume-control rules.
type O11yReductionRuleListIn struct {
	// OrderBy sorts the page: metric, ingested_volume, reduced_volume or
	// last_updated. Unset means ingested_volume.
	OrderBy string `json:"orderBy,omitempty"`
	// Order is asc or desc. Unset means desc.
	Order string `json:"order,omitempty"`
	// Search narrows the page to rules whose metric name contains it.
	Search string `json:"search,omitempty"`
	// MetricName narrows the page to one metric's rule.
	MetricName string `json:"metricName,omitempty"`
	// Offset is how many rules to skip, for paging.
	Offset int `json:"offset,omitempty"`
	// Limit caps how many rules come back, at most 1000. Unset means 10.
	Limit int `json:"limit,omitempty"`
}

// O11yReductionRuleCreateIn is a volume-control rule to create.
type O11yReductionRuleCreateIn struct {
	// MetricName is the metric the rule governs; one rule per metric. Required.
	MetricName string `json:"metricName" validate:"required"`
	// MatchType is drop or keep: drop the named labels, or keep only them. Required.
	MatchType string `json:"matchType" validate:"required"`
	// Labels are the label names the rule matches. Required, at least one.
	Labels []string `json:"labels" validate:"required"`
}

// O11yReductionRulePreviewIn is a candidate volume-control rule to estimate.
type O11yReductionRulePreviewIn struct {
	// MetricName is the metric the rule would govern. Required.
	MetricName string `json:"metricName" validate:"required"`
	// MatchType is drop or keep. Required.
	MatchType string `json:"matchType" validate:"required"`
	// Labels are the label names the rule would match. Required, at least one.
	Labels []string `json:"labels" validate:"required"`
	// LookbackMs is how far back to sample when estimating.
	LookbackMs int64 `json:"lookbackMs,omitempty"`
}

// O11yReductionRuleRef names one volume-control rule.
type O11yReductionRuleRef struct {
	// ID is the rule's id.
	ID string `json:"id" validate:"required"`
}

// O11yReductionRuleSaveIn updates one volume-control rule; the metric name is
// immutable.
type O11yReductionRuleSaveIn struct {
	// ID is the rule's id.
	ID string `json:"id" validate:"required"`
	// MatchType is drop or keep. Required.
	MatchType string `json:"matchType" validate:"required"`
	// Labels are the label names the rule matches. Required, at least one.
	Labels []string `json:"labels" validate:"required"`
}

// ── outputs ───────────────────────────────────────────────────────────────────
//
// Each Out is the runtime's answer NAMED, field for field, tag for tag —
// including the status/data envelope every route on this face has always
// answered with. Nothing embeds: the schema builder walks reflected fields
// and cannot name an embedded one. metrics_test.go re-encodes each Out and
// compares it byte for byte with what the runtime wrote.

// O11yMetricListOut is a page of metric names.
type O11yMetricListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the metrics.
	Data O11yMetricList `json:"data,omitempty"`
}

// O11yMetricList is a page of metrics.
type O11yMetricList struct {
	// Metrics are the metrics, with their metadata.
	Metrics []O11yMetricSummary `json:"metrics"`
}

// O11yMetricSummary is one metric and its metadata.
type O11yMetricSummary struct {
	// MetricName is the metric's name.
	MetricName string `json:"metricName"`
	// Description describes the metric.
	Description string `json:"description"`
	// Type is the metric type, e.g. gauge, sum, histogram.
	Type string `json:"type"`
	// Unit is the metric's unit.
	Unit string `json:"unit"`
	// Temporality is delta or cumulative.
	Temporality string `json:"temporality"`
	// IsMonotonic marks a sum that only ever increases.
	IsMonotonic bool `json:"isMonotonic"`
}

// O11yMetricStatsOut is a page of metric volume statistics.
type O11yMetricStatsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the statistics.
	Data O11yMetricStats `json:"data,omitempty"`
}

// O11yMetricStats is a page of per-metric volume counts.
type O11yMetricStats struct {
	// Metrics are the counted metrics.
	Metrics []O11yMetricStat `json:"metrics"`
	// Total is how many metrics matched, across all pages.
	Total uint64 `json:"total"`
}

// O11yMetricStat is one metric's volume in the window.
type O11yMetricStat struct {
	// MetricName is the metric's name.
	MetricName string `json:"metricName"`
	// Description describes the metric.
	Description string `json:"description"`
	// Type is the metric type.
	Type string `json:"type"`
	// Unit is the metric's unit.
	Unit string `json:"unit"`
	// TimeSeries is how many time series the metric had.
	TimeSeries uint64 `json:"timeseries"`
	// Samples is how many samples the metric had.
	Samples uint64 `json:"samples"`
}

// O11yMetricTreemapOut is a treemap of metric volume.
type O11yMetricTreemapOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the treemap.
	Data O11yMetricTreemap `json:"data,omitempty"`
}

// O11yMetricTreemap is the treemap's entries, one list per measure.
type O11yMetricTreemap struct {
	// TimeSeries are the entries when measuring by time-series count.
	TimeSeries []O11yTreemapEntry `json:"timeseries"`
	// Samples are the entries when measuring by sample count.
	Samples []O11yTreemapEntry `json:"samples"`
}

// O11yTreemapEntry is one metric's share of the whole.
type O11yTreemapEntry struct {
	// MetricName is the metric's name.
	MetricName string `json:"metricName"`
	// Percentage is the metric's share, in percent.
	Percentage float64 `json:"percentage"`
	// TotalValue is the metric's absolute count.
	TotalValue uint64 `json:"totalValue"`
}

// O11yMetricAttributesOut is one metric's attribute keys and values.
type O11yMetricAttributesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the attributes.
	Data O11yMetricAttributes `json:"data,omitempty"`
}

// O11yMetricAttributes is a metric's attribute keys.
type O11yMetricAttributes struct {
	// Attributes are the keys, each with its values.
	Attributes []O11yMetricAttribute `json:"attributes"`
	// TotalKeys is how many keys the metric has.
	TotalKeys int64 `json:"totalKeys"`
}

// O11yMetricAttribute is one attribute key and its values.
type O11yMetricAttribute struct {
	// Key is the attribute's name.
	Key string `json:"key"`
	// Values are the attribute's distinct values.
	Values []string `json:"values"`
	// ValueCount is how many distinct values the attribute has.
	ValueCount uint64 `json:"valueCount"`
}

// O11yMetricMetadataOut is one metric's metadata.
type O11yMetricMetadataOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the metadata.
	Data O11yMetricMetadata `json:"data,omitempty"`
}

// O11yMetricMetadata is a metric's metadata.
type O11yMetricMetadata struct {
	// Description describes the metric.
	Description string `json:"description"`
	// Type is the metric type, e.g. gauge, sum, histogram.
	Type string `json:"type"`
	// Unit is the metric's unit.
	Unit string `json:"unit"`
	// Temporality is delta or cumulative.
	Temporality string `json:"temporality"`
	// IsMonotonic marks a sum that only ever increases.
	IsMonotonic bool `json:"isMonotonic"`
}

// O11yMetricAckOut is the bare success envelope a write answers with.
type O11yMetricAckOut struct {
	// Status is "success".
	Status string `json:"status"`
}

// O11yMetricHighlightsOut is one metric's headline numbers.
type O11yMetricHighlightsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the highlights.
	Data O11yMetricHighlights `json:"data,omitempty"`
}

// O11yMetricHighlights is a metric's headline numbers.
type O11yMetricHighlights struct {
	// DataPoints is how many data points the metric has.
	DataPoints uint64 `json:"dataPoints"`
	// LastReceived is when the metric last arrived, as a Unix timestamp in
	// milliseconds.
	LastReceived uint64 `json:"lastReceived"`
	// TotalTimeSeries is how many time series the metric has ever had.
	TotalTimeSeries uint64 `json:"totalTimeSeries"`
	// ActiveTimeSeries is how many of them are active.
	ActiveTimeSeries uint64 `json:"activeTimeSeries"`
}

// O11yMetricAlertsOut is the alert rules that reference a metric.
type O11yMetricAlertsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the alerts.
	Data O11yMetricAlerts `json:"data,omitempty"`
}

// O11yMetricAlerts is a list of alert rules.
type O11yMetricAlerts struct {
	// Alerts are the alert rules referencing the metric.
	Alerts []O11yMetricAlert `json:"alerts"`
}

// O11yMetricAlert is one alert rule referencing a metric.
type O11yMetricAlert struct {
	// AlertName is the alert rule's name.
	AlertName string `json:"alertName"`
	// AlertID is the alert rule's id.
	AlertID string `json:"alertId"`
}

// O11yMetricDashboardsOut is the dashboard panels that reference a metric.
type O11yMetricDashboardsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the panels.
	Data O11yMetricDashboards `json:"data,omitempty"`
}

// O11yMetricDashboards is a list of dashboard panels.
type O11yMetricDashboards struct {
	// Dashboards are the panels referencing the metric.
	Dashboards []O11yMetricPanel `json:"dashboards"`
}

// O11yMetricPanel is one dashboard panel referencing a metric.
type O11yMetricPanel struct {
	// DashboardID is the dashboard's id.
	DashboardID string `json:"dashboardId"`
	// DashboardName is the dashboard's name.
	DashboardName string `json:"dashboardName"`
	// PanelID is the panel's id.
	PanelID string `json:"panelId"`
	// PanelName is the panel's name.
	PanelName string `json:"panelName"`
	// GroupBy are the labels the panel groups the metric by.
	GroupBy []string `json:"groupBy,omitempty"`
	// FilterBy are the labels the panel filters the metric by.
	FilterBy []string `json:"filterBy,omitempty"`
}

// O11yMetricInspectOut is one metric's raw series.
type O11yMetricInspectOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the series.
	Data O11yMetricSeriesSet `json:"data,omitempty"`
}

// O11yMetricSeriesSet is a set of raw time series.
type O11yMetricSeriesSet struct {
	// Series are the time series.
	Series []O11yMetricSeries `json:"series"`
}

// O11yMetricSeries is one time series: its labels and its points.
type O11yMetricSeries struct {
	// Labels identify the series.
	Labels []O11yMetricLabel `json:"labels,omitempty"`
	// Values are the series' points, in time order.
	Values []O11yMetricPoint `json:"values"`
}

// O11yMetricLabel is one label of a time series.
type O11yMetricLabel struct {
	// Key is the label's field.
	Key O11yMetricField `json:"key"`
	// Value is the label's value.
	Value any `json:"value"`
}

// O11yMetricPoint is one point of a time series.
type O11yMetricPoint struct {
	// Timestamp is the point's time as a Unix timestamp in milliseconds.
	Timestamp int64 `json:"timestamp"`
	// Value is the point's value.
	Value float64 `json:"value"`
	// Partial marks a point whose bucket the window only partly covers.
	Partial bool `json:"partial,omitempty"`
	// Values carries the bucket values of a heatmap point.
	Values []float64 `json:"values,omitempty"`
}

// O11yMetricOnboardingOut is the onboarding check's answer.
type O11yMetricOnboardingOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the flag.
	Data O11yMetricOnboarding `json:"data,omitempty"`
}

// O11yMetricOnboarding says whether any non-O11y metrics have arrived.
type O11yMetricOnboarding struct {
	// HasMetrics is true once any non-O11y metric has been ingested.
	HasMetrics bool `json:"hasMetrics"`
}

// O11yReductionRuleListOut is a page of volume-control rules.
type O11yReductionRuleListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rules.
	Data O11yReductionRules `json:"data,omitempty"`
}

// O11yReductionRules is a page of volume-control rules.
type O11yReductionRules struct {
	// Rules are the rules.
	Rules []O11yReductionRule `json:"rules"`
	// Total is how many rules matched, across all pages.
	Total int `json:"total"`
}

// O11yReductionRule is one volume-control rule and its measured effect.
type O11yReductionRule struct {
	// ID is the rule's id.
	ID string `json:"id"`
	// CreatedAt is when the rule was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the rule last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// CreatedBy is who created it.
	CreatedBy string `json:"createdBy"`
	// UpdatedBy is who last changed it.
	UpdatedBy string `json:"updatedBy"`
	// MetricName is the metric the rule governs.
	MetricName string `json:"metricName"`
	// MatchType is drop or keep.
	MatchType string `json:"matchType"`
	// Labels are the label names the rule matches.
	Labels []string `json:"labels"`
	// EffectiveFrom is when the rule took effect.
	EffectiveFrom time.Time `json:"effectiveFrom"`
	// Active says whether the rule is in force.
	Active bool `json:"active"`
	// IngestedSeries is how many series arrived while the rule was active.
	IngestedSeries uint64 `json:"ingestedSeries"`
	// RetainedSeries is how many of them were kept.
	RetainedSeries uint64 `json:"retainedSeries"`
	// IngestedSamples is how many samples arrived while the rule was active.
	IngestedSamples uint64 `json:"ingestedSamples"`
	// RetainedSamples is how many of them were kept.
	RetainedSamples uint64 `json:"retainedSamples"`
}

// O11yReductionRuleOut is one volume-control rule.
type O11yReductionRuleOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the rule.
	Data O11yReductionRule `json:"data,omitempty"`
}

// O11yReductionStatsOut is the aggregate effect of all volume-control rules.
type O11yReductionStatsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the totals.
	Data O11yReductionStats `json:"data,omitempty"`
}

// O11yReductionStats is the aggregate volume-control summary.
type O11yReductionStats struct {
	// IngestedSeries is how many series arrived across all rules.
	IngestedSeries uint64 `json:"ingestedSeries"`
	// RetainedSeries is how many of them were kept.
	RetainedSeries uint64 `json:"retainedSeries"`
	// IngestedSamples is how many samples arrived across all rules.
	IngestedSamples uint64 `json:"ingestedSamples"`
	// RetainedSamples is how many of them were kept.
	RetainedSamples uint64 `json:"retainedSamples"`
	// EstimatedMonthlySavingsUsd is the estimated monthly savings, in USD.
	EstimatedMonthlySavingsUsd float64 `json:"estimatedMonthlySavingsUsd"`
}

// O11yReductionSeriesOut is the volume-control effect over time.
type O11yReductionSeriesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the query-range answer.
	Data O11yQueryRange `json:"data,omitempty"`
}

// O11yQueryRange is a query-range answer carrying time-series results.
type O11yQueryRange struct {
	// Type is the result kind; time_series here.
	Type string `json:"type"`
	// Data holds the results.
	Data O11yQueryRangeData `json:"data"`
	// Meta reports what the query scanned.
	Meta O11yQueryStats `json:"meta"`
	// Warning carries a non-fatal warning, when the query raised one.
	Warning *O11yQueryWarning `json:"warning,omitempty"`
}

// O11yQueryRangeData is the results of a query-range answer.
type O11yQueryRangeData struct {
	// Results are the per-query results, each a set of aggregated series.
	Results []O11yReductionSeriesResult `json:"results"`
}

// O11yReductionSeriesResult is one query's aggregated series.
type O11yReductionSeriesResult struct {
	// QueryName names the query the result answers.
	QueryName string `json:"queryName"`
	// Aggregations are the query's aggregation buckets.
	Aggregations []O11yAggregation `json:"aggregations"`
}

// O11yAggregation is one aggregation bucket and its series.
type O11yAggregation struct {
	// Index is the aggregation's position in the query.
	Index int `json:"index"`
	// Alias is the aggregation's alias.
	Alias string `json:"alias"`
	// Meta describes the aggregation.
	Meta O11yAggregationMeta `json:"meta,omitempty"`
	// Series are the aggregated time series.
	Series []O11yMetricSeries `json:"series"`
	// PredictedSeries are forecast overlays, when the query asked for them.
	PredictedSeries []O11yMetricSeries `json:"predictedSeries,omitempty"`
	// UpperBoundSeries are forecast upper bounds.
	UpperBoundSeries []O11yMetricSeries `json:"upperBoundSeries,omitempty"`
	// LowerBoundSeries are forecast lower bounds.
	LowerBoundSeries []O11yMetricSeries `json:"lowerBoundSeries,omitempty"`
	// AnomalyScores are anomaly overlays.
	AnomalyScores []O11yMetricSeries `json:"anomalyScores,omitempty"`
}

// O11yAggregationMeta describes an aggregation.
type O11yAggregationMeta struct {
	// Unit is the aggregation's unit.
	Unit string `json:"unit,omitempty"`
}

// O11yQueryStats reports what a query scanned.
type O11yQueryStats struct {
	// RowsScanned is how many rows the query read.
	RowsScanned uint64 `json:"rowsScanned"`
	// BytesScanned is how many bytes the query read.
	BytesScanned uint64 `json:"bytesScanned"`
	// DurationMS is how long the query took, in milliseconds.
	DurationMS uint64 `json:"durationMs"`
	// StepIntervals is the step used per query, in seconds.
	StepIntervals map[string]uint64 `json:"stepIntervals,omitempty"`
}

// O11yQueryWarning is a non-fatal warning a query raised.
type O11yQueryWarning struct {
	// Message is the warning.
	Message string `json:"message"`
	// URL points at the relevant documentation.
	URL string `json:"url,omitempty"`
	// Warnings carries additional notes.
	Warnings []O11yQueryWarningNote `json:"warnings,omitempty"`
}

// O11yQueryWarningNote is one additional warning note.
type O11yQueryWarningNote struct {
	// Message is the note.
	Message string `json:"message"`
}

// O11yReductionRulePreviewOut is the estimated effect of a candidate rule.
type O11yReductionRulePreviewOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the estimate.
	Data O11yReductionRulePreview `json:"data,omitempty"`
}

// O11yReductionRulePreview is the estimated effect of a candidate rule.
type O11yReductionRulePreview struct {
	// IngestedSeries is how many series the metric ingests today.
	IngestedSeries uint64 `json:"ingestedSeries"`
	// CurrentRetainedSeries is how many survive the rules in force today.
	CurrentRetainedSeries uint64 `json:"currentRetainedSeries"`
	// RetainedSeries is how many would survive with the candidate rule.
	RetainedSeries uint64 `json:"retainedSeries"`
	// ReductionPercent is the estimated reduction, in percent.
	ReductionPercent float64 `json:"reductionPercent"`
	// DroppedLabels are the labels the rule would drop.
	DroppedLabels []string `json:"droppedLabels"`
	// AffectedAssets are the dashboards and alerts the rule would touch.
	AffectedAssets []O11yAffectedAsset `json:"affectedAssets"`
	// EffectiveFrom is when the rule would take effect.
	EffectiveFrom time.Time `json:"effectiveFrom"`
}

// O11yAffectedAsset is one dashboard or alert a candidate rule would touch.
type O11yAffectedAsset struct {
	// Type is dashboard or alert_rule.
	Type string `json:"type"`
	// ID is the asset's id.
	ID string `json:"id"`
	// Name is the asset's name.
	Name string `json:"name"`
	// Widget is the affected panel, for a dashboard.
	Widget *O11yAffectedWidget `json:"widget,omitempty"`
	// ImpactedLabels are the rule labels the asset uses.
	ImpactedLabels []string `json:"impactedLabels"`
}

// O11yAffectedWidget is the affected panel of a dashboard.
type O11yAffectedWidget struct {
	// ID is the panel's id.
	ID string `json:"id"`
	// Name is the panel's name.
	Name string `json:"name"`
}

// ── the seam ──────────────────────────────────────────────────────────────────

// metricsRelay hands a typed op's call to the o11y runtime and decodes the
// answer into the op's Out — the same seam relay (telemetry.go) is for the
// error-tracking face, rebuilt on this face's own root.
//
// It is what keeps these nineteen ops a NAMING of the wire rather than a
// second implementation of it. The handler it calls is the one SetHandler
// registered — the same value the delegation wildcard forwards to — so the
// request runs the whole chain it always ran: identity resolution, the org
// gate, the ROLE CHECK each mux registration declared (ViewAccess, EditAccess
// or AdminAccess, unchanged per route), the handler, the envelope. There is
// no policy here.
//
// Identity is PROPAGATED, never minted: the gateway's assertion travels on as
// the same headers it arrived on. A context with no request behind it carries
// none, so the runtime's gate refuses it — the honest answer rather than an
// identity invented at this hop.
//
// A nil out skips the decode: a DELETE answers 204 and its body, if any, is
// not part of the contract. A non-2xx becomes an error carrying the runtime's
// own status and reason, so the status a caller sees is the status the
// runtime chose.
func metricsRelay(ctx context.Context, method, path string, params url.Values, body, out any) error {
	h := getHandler()
	if h == nil {
		return zip.Errorf(http.StatusServiceUnavailable, "o11y runtime not initialized")
	}

	target := metricsRoot + path
	if q := params.Encode(); q != "" {
		target += "?" + q
	}

	payload := io.Reader(http.NoBody)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return zip.ErrBadRequest(err.Error())
		}
		payload = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return zip.ErrBadRequest(err.Error())
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	caller := zip.CallerOf(ctx)
	for header, value := range map[string]string{
		zip.HeaderOrg:       caller.Org,
		zip.HeaderProject:   caller.Project,
		zip.HeaderUser:      caller.User,
		zip.HeaderUserName:  caller.Name,
		zip.HeaderUserEmail: caller.Email,
		zip.HeaderUserOwner: caller.Owner,
		zip.HeaderRequestID: caller.RequestID,
	} {
		if value != "" {
			req.Header.Set(header, value)
		}
	}
	if caller.Admin {
		req.Header.Set(zip.HeaderUserAdmin, "true")
	}
	if caller.OrgAdmin {
		req.Header.Set(zip.HeaderUserOrgAdmin, "true")
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code < http.StatusOK || rec.Code >= http.StatusMultipleChoices {
		code, reason := refusal(rec.Body.Bytes())
		return &zip.HTTPError{Status: rec.Code, Code: code, Msg: reason}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		return zip.ErrInternal("cannot read the runtime's answer: " + err.Error())
	}
	return nil
}
