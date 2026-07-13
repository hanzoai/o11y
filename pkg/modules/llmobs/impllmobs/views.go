package impllmobs

import (
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/o11y/pkg/types/llmobstypes"
	"github.com/hanzoai/o11y/pkg/types/llmpricingruletypes"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
)

// This file is the whole span-view engine: pure builders that turn a ViewQuery
// into a QueryRangeRequest and pure mappers that turn the QueryRangeResponse
// back into observation-shaped DTOs. Keeping them free of the querier makes them
// unit-testable without Datastore. Observations are raw gen_ai spans; traces,
// sessions and users are scalar aggregations grouped by trace_id / session.id /
// user.id.

const (
	defaultViewLimit = 50
	maxViewLimit     = 200
	defaultLookback  = 24 * time.Hour
	nanosPerMilli    = 1e6
)

// genAIMarker is the canonical "this span is an LLM call" filter.
var genAIMarker = llmobstypes.GenAISystem + " EXISTS"

// noOrgSentinel is a tenant value no real span carries. When a span-view query
// reaches genAIFilter with an empty org slug (a caller/plumbing bug — the handler
// already fails closed), the org predicate binds this sentinel so the query matches
// ZERO rows instead of every tenant's spans. Fail closed, never fail open.
const noOrgSentinel = "\x00-no-org-\x00"

func spanField(name string) telemetrytypes.TelemetryFieldKey {
	return telemetrytypes.TelemetryFieldKey{Name: name, FieldContext: telemetrytypes.FieldContextSpan}
}

func attrStr(name string) telemetrytypes.TelemetryFieldKey {
	return telemetrytypes.TelemetryFieldKey{Name: name, FieldContext: telemetrytypes.FieldContextAttribute, FieldDataType: telemetrytypes.FieldDataTypeString}
}

func attrNum(name string) telemetrytypes.TelemetryFieldKey {
	return telemetrytypes.TelemetryFieldKey{Name: name, FieldContext: telemetrytypes.FieldContextAttribute, FieldDataType: telemetrytypes.FieldDataTypeFloat64}
}

func resourceField(name string) telemetrytypes.TelemetryFieldKey {
	return telemetrytypes.TelemetryFieldKey{Name: name, FieldContext: telemetrytypes.FieldContextResource, FieldDataType: telemetrytypes.FieldDataTypeString, Materialized: true}
}

func groupBy(key telemetrytypes.TelemetryFieldKey) qbtypes.GroupByKey {
	return qbtypes.GroupByKey{TelemetryFieldKey: key}
}

// resolveWindow converts a ViewQuery's unix-ms window into the uint64 ms bounds
// QueryRange expects, defaulting to the last 24h when unset. Guarantees start < end.
func resolveWindow(startMs, endMs int64) (uint64, uint64) {
	if endMs <= 0 {
		endMs = time.Now().UnixMilli()
	}
	if startMs <= 0 || startMs >= endMs {
		startMs = endMs - defaultLookback.Milliseconds()
	}
	if startMs < 0 {
		startMs = 0
	}
	return uint64(startMs), uint64(endMs)
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultViewLimit
	}
	if limit > maxViewLimit {
		return maxViewLimit
	}
	return limit
}

func clampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

// genAIFilter assembles the filter expression (marker + optional equality
// predicates) plus a variables map that binds user values safely as $N
// placeholders. requireExists adds `<key> EXISTS` clauses (used to drop rows
// without the grouping key).
func genAIFilter(q *llmobstypes.ViewQuery, requireExists ...string) (string, map[string]qbtypes.VariableItem) {
	parts := []string{genAIMarker}
	for _, k := range requireExists {
		parts = append(parts, k+" EXISTS")
	}

	vars := map[string]qbtypes.VariableItem{}
	next := 1
	// bind adds an unconditional `key = $N` predicate with the value bound as a
	// query variable (never string-interpolated).
	bind := func(key, val string) {
		id := strconv.Itoa(next)
		vars[id] = qbtypes.VariableItem{Type: qbtypes.DynamicVariableType, Value: val}
		parts = append(parts, key+" = $"+id)
		next++
	}
	// eq is the optional narrower: skipped when the value is empty.
	eq := func(key, val string) {
		if val == "" {
			return
		}
		bind(key, val)
	}

	// MANDATORY tenant boundary, added FIRST. Every observations/traces/sessions/
	// users row must belong to the caller's validated org: the span views have no
	// org column other than the gen_ai.hanzo.org_id the ai emit path tags, so
	// without this AND any authenticated tenant reads every other tenant's spans.
	// The slug is server-set by the handler from the validated X-Org-Id (never
	// client input); an empty value binds a sentinel that matches nothing (fail
	// closed). Because it is ANDed before the optional narrowers, a caller supplying
	// a FOREIGN traceId/sessionId still gets zero rows — the foreign row's org
	// differs — closing the compose-path cross-tenant read.
	org := q.OrgSlug
	if org == "" {
		org = noOrgSentinel
	}
	bind(llmobstypes.GenAIHanzoOrgID, org)

	eq("trace_id", q.TraceID)
	eq(llmobstypes.SessionID, q.SessionID)
	eq(llmobstypes.UserID, q.UserID)
	eq("name", q.Name)
	eq(llmpricingruletypes.GenAIRequestModel, q.Model)

	return strings.Join(parts, " AND "), vars
}

func newRequest(start, end uint64, reqType qbtypes.RequestType, vars map[string]qbtypes.VariableItem, spec qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]) *qbtypes.QueryRangeRequest {
	return &qbtypes.QueryRangeRequest{
		Start:       start,
		End:         end,
		RequestType: reqType,
		Variables:   vars,
		CompositeQuery: qbtypes.CompositeQuery{
			Queries: []qbtypes.QueryEnvelope{
				{Type: qbtypes.QueryTypeBuilder, Spec: spec},
			},
		},
	}
}

// --- observations (raw gen_ai spans) ---

func buildObservationsQuery(q *llmobstypes.ViewQuery) *qbtypes.QueryRangeRequest {
	start, end := resolveWindow(q.Start, q.End)
	filter, vars := genAIFilter(q)

	spec := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
		Name:   "A",
		Signal: telemetrytypes.SignalTraces,
		Filter: &qbtypes.Filter{Expression: filter},
		SelectFields: []telemetrytypes.TelemetryFieldKey{
			spanField("timestamp"),
			spanField("trace_id"),
			spanField("span_id"),
			spanField("parent_span_id"),
			spanField("name"),
			spanField("duration_nano"),
			spanField("response_status_code"),
			attrStr(llmpricingruletypes.GenAIRequestModel),
			attrStr(llmobstypes.GenAIResponseModel),
			attrStr(llmobstypes.GenAISystem),
			attrStr(llmobstypes.GenAIOperationName),
			attrNum(llmpricingruletypes.GenAIUsageInputTokens),
			attrNum(llmpricingruletypes.GenAIUsageOutputTokens),
			attrNum(llmpricingruletypes.O11yGenAITotalCost),
			attrStr(llmobstypes.SessionID),
			attrStr(llmobstypes.UserID),
			resourceField(llmobstypes.ServiceName),
		},
		Order: []qbtypes.OrderBy{
			{Key: qbtypes.OrderByKey{TelemetryFieldKey: spanField("timestamp")}, Direction: qbtypes.OrderDirectionDesc},
		},
		Limit:  clampLimit(q.Limit),
		Offset: clampOffset(q.Offset),
	}

	return newRequest(start, end, qbtypes.RequestTypeRaw, vars, spec)
}

func mapObservations(resp *qbtypes.QueryRangeResponse) []*llmobstypes.Observation {
	out := []*llmobstypes.Observation{}
	rd := rawData(resp)
	if rd == nil {
		return out
	}
	for _, row := range rd.Rows {
		d := row.Data
		o := &llmobstypes.Observation{
			ID:               asString(d["span_id"]),
			TraceID:          asString(d["trace_id"]),
			ParentID:         asString(d["parent_span_id"]),
			Type:             observationType(asString(d[llmobstypes.GenAIOperationName])),
			Name:             asString(d["name"]),
			StartTime:        row.Timestamp,
			LatencyMs:        asFloat(d["duration_nano"]) / nanosPerMilli,
			Model:            firstNonEmpty(asString(d[llmobstypes.GenAIResponseModel]), asString(d[llmpricingruletypes.GenAIRequestModel])),
			Provider:         asString(d[llmobstypes.GenAISystem]),
			PromptTokens:     int64(asFloat(d[llmpricingruletypes.GenAIUsageInputTokens])),
			CompletionTokens: int64(asFloat(d[llmpricingruletypes.GenAIUsageOutputTokens])),
			TotalCost:        asFloat(d[llmpricingruletypes.O11yGenAITotalCost]),
			SessionID:        asString(d[llmobstypes.SessionID]),
			UserID:           asString(d[llmobstypes.UserID]),
			ServiceName:      asString(d[llmobstypes.ServiceName]),
			StatusCode:       asString(d["response_status_code"]),
		}
		o.TotalTokens = o.PromptTokens + o.CompletionTokens
		out = append(out, o)
	}
	return out
}

// --- traces / sessions / users (scalar aggregations) ---

// costTokenAggs are the numeric rollups shared by every grouped view.
func costTokenAggs() []qbtypes.TraceAggregation {
	return []qbtypes.TraceAggregation{
		{Expression: "sum(" + llmpricingruletypes.GenAIUsageInputTokens + ")", Alias: "promptTokens"},
		{Expression: "sum(" + llmpricingruletypes.GenAIUsageOutputTokens + ")", Alias: "completionTokens"},
		{Expression: "sum(" + llmpricingruletypes.O11yGenAITotalCost + ")", Alias: "totalCost"},
	}
}

func buildTracesQuery(q *llmobstypes.ViewQuery) *qbtypes.QueryRangeRequest {
	start, end := resolveWindow(q.Start, q.End)
	filter, vars := genAIFilter(q)

	aggs := append([]qbtypes.TraceAggregation{{Expression: "count()", Alias: "observations"}}, costTokenAggs()...)
	aggs = append(aggs, qbtypes.TraceAggregation{Expression: "max(duration_nano)", Alias: "latency"})

	spec := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
		Name:   "A",
		Signal: telemetrytypes.SignalTraces,
		Filter: &qbtypes.Filter{Expression: filter},
		GroupBy: []qbtypes.GroupByKey{
			groupBy(spanField("trace_id")),
			groupBy(attrStr(llmobstypes.SessionID)),
			groupBy(attrStr(llmobstypes.UserID)),
			groupBy(resourceField(llmobstypes.ServiceName)),
		},
		Aggregations: aggs,
		Order:        []qbtypes.OrderBy{{Key: qbtypes.OrderByKey{TelemetryFieldKey: telemetrytypes.TelemetryFieldKey{Name: "observations"}}, Direction: qbtypes.OrderDirectionDesc}},
		Limit:        clampLimit(q.Limit),
	}

	return newRequest(start, end, qbtypes.RequestTypeScalar, vars, spec)
}

func mapTraces(resp *qbtypes.QueryRangeResponse) []*llmobstypes.Trace {
	out := []*llmobstypes.Trace{}
	sd := scalarData(resp)
	if sd == nil {
		return out
	}
	groups, aggs := indexColumns(sd)
	for _, row := range sd.Data {
		t := &llmobstypes.Trace{
			ID:               groupString(row, groups, "trace_id"),
			SessionID:        groupString(row, groups, llmobstypes.SessionID),
			UserID:           groupString(row, groups, llmobstypes.UserID),
			ServiceName:      groupString(row, groups, llmobstypes.ServiceName),
			Observations:     int64(aggFloat(row, aggs, 0)),
			PromptTokens:     int64(aggFloat(row, aggs, 1)),
			CompletionTokens: int64(aggFloat(row, aggs, 2)),
			TotalCost:        aggFloat(row, aggs, 3),
			LatencyMs:        aggFloat(row, aggs, 4) / nanosPerMilli,
		}
		t.TotalTokens = t.PromptTokens + t.CompletionTokens
		out = append(out, t)
	}
	return out
}

func buildSessionsQuery(q *llmobstypes.ViewQuery) *qbtypes.QueryRangeRequest {
	start, end := resolveWindow(q.Start, q.End)
	filter, vars := genAIFilter(q, llmobstypes.SessionID)

	aggs := append([]qbtypes.TraceAggregation{
		{Expression: "count_distinct(trace_id)", Alias: "traces"},
		{Expression: "count()", Alias: "observations"},
	}, costTokenAggs()...)

	spec := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
		Name:   "A",
		Signal: telemetrytypes.SignalTraces,
		Filter: &qbtypes.Filter{Expression: filter},
		GroupBy: []qbtypes.GroupByKey{
			groupBy(attrStr(llmobstypes.SessionID)),
			groupBy(attrStr(llmobstypes.UserID)),
		},
		Aggregations: aggs,
		Order:        []qbtypes.OrderBy{{Key: qbtypes.OrderByKey{TelemetryFieldKey: telemetrytypes.TelemetryFieldKey{Name: "observations"}}, Direction: qbtypes.OrderDirectionDesc}},
		Limit:        clampLimit(q.Limit),
	}

	return newRequest(start, end, qbtypes.RequestTypeScalar, vars, spec)
}

func mapSessions(resp *qbtypes.QueryRangeResponse) []*llmobstypes.Session {
	out := []*llmobstypes.Session{}
	sd := scalarData(resp)
	if sd == nil {
		return out
	}
	groups, aggs := indexColumns(sd)
	for _, row := range sd.Data {
		s := &llmobstypes.Session{
			ID:               groupString(row, groups, llmobstypes.SessionID),
			UserID:           groupString(row, groups, llmobstypes.UserID),
			Traces:           int64(aggFloat(row, aggs, 0)),
			Observations:     int64(aggFloat(row, aggs, 1)),
			PromptTokens:     int64(aggFloat(row, aggs, 2)),
			CompletionTokens: int64(aggFloat(row, aggs, 3)),
			TotalCost:        aggFloat(row, aggs, 4),
		}
		s.TotalTokens = s.PromptTokens + s.CompletionTokens
		out = append(out, s)
	}
	return out
}

func buildUsersQuery(q *llmobstypes.ViewQuery) *qbtypes.QueryRangeRequest {
	start, end := resolveWindow(q.Start, q.End)
	filter, vars := genAIFilter(q, llmobstypes.UserID)

	aggs := append([]qbtypes.TraceAggregation{
		{Expression: "count_distinct(" + llmobstypes.SessionID + ")", Alias: "sessions"},
		{Expression: "count_distinct(trace_id)", Alias: "traces"},
		{Expression: "count()", Alias: "observations"},
	}, costTokenAggs()...)

	spec := qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation]{
		Name:   "A",
		Signal: telemetrytypes.SignalTraces,
		Filter: &qbtypes.Filter{Expression: filter},
		GroupBy: []qbtypes.GroupByKey{
			groupBy(attrStr(llmobstypes.UserID)),
		},
		Aggregations: aggs,
		Order:        []qbtypes.OrderBy{{Key: qbtypes.OrderByKey{TelemetryFieldKey: telemetrytypes.TelemetryFieldKey{Name: "observations"}}, Direction: qbtypes.OrderDirectionDesc}},
		Limit:        clampLimit(q.Limit),
	}

	return newRequest(start, end, qbtypes.RequestTypeScalar, vars, spec)
}

func mapUsers(resp *qbtypes.QueryRangeResponse) []*llmobstypes.User {
	out := []*llmobstypes.User{}
	sd := scalarData(resp)
	if sd == nil {
		return out
	}
	groups, aggs := indexColumns(sd)
	for _, row := range sd.Data {
		u := &llmobstypes.User{
			ID:               groupString(row, groups, llmobstypes.UserID),
			Sessions:         int64(aggFloat(row, aggs, 0)),
			Traces:           int64(aggFloat(row, aggs, 1)),
			Observations:     int64(aggFloat(row, aggs, 2)),
			PromptTokens:     int64(aggFloat(row, aggs, 3)),
			CompletionTokens: int64(aggFloat(row, aggs, 4)),
			TotalCost:        aggFloat(row, aggs, 5),
		}
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
		out = append(out, u)
	}
	return out
}

// --- shared response helpers ---

func rawData(resp *qbtypes.QueryRangeResponse) *qbtypes.RawData {
	if resp == nil || len(resp.Data.Results) == 0 {
		return nil
	}
	rd, _ := resp.Data.Results[0].(*qbtypes.RawData)
	return rd
}

func scalarData(resp *qbtypes.QueryRangeResponse) *qbtypes.ScalarData {
	if resp == nil || len(resp.Data.Results) == 0 {
		return nil
	}
	sd, _ := resp.Data.Results[0].(*qbtypes.ScalarData)
	return sd
}

// indexColumns splits a scalar result's columns into group-name→index and
// aggregation-index→row-index maps, mirroring the services module.
func indexColumns(sd *qbtypes.ScalarData) (map[string]int, map[int]int) {
	groups := map[string]int{}
	aggs := map[int]int{}
	for i, c := range sd.Columns {
		switch c.Type {
		case qbtypes.ColumnTypeGroup:
			groups[c.Name] = i
		case qbtypes.ColumnTypeAggregation:
			aggs[int(c.AggregationIndex)] = i
		}
	}
	return groups, aggs
}

func groupString(row []any, groups map[string]int, name string) string {
	if idx, ok := groups[name]; ok {
		return asString(row[idx])
	}
	return ""
}

func aggFloat(row []any, aggs map[int]int, aggIndex int) float64 {
	if idx, ok := aggs[aggIndex]; ok {
		return asFloat(row[idx])
	}
	return 0
}

func observationType(op string) string {
	if op == "" {
		return "GENERATION"
	}
	return strings.ToUpper(op)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// asString coerces a scanned cell (string, *string, []byte, nil) to a string.
func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case *string:
		if x != nil {
			return *x
		}
	case []byte:
		return string(x)
	}
	return ""
}

// asFloat coerces any numeric scan result (incl. pointers) to float64.
func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case *float64:
		if x != nil {
			return *x
		}
	case float32:
		return float64(x)
	case uint64:
		return float64(x)
	case int64:
		return float64(x)
	case uint32:
		return float64(x)
	case int32:
		return float64(x)
	case int:
		return float64(x)
	case *uint64:
		if x != nil {
			return float64(*x)
		}
	case *int64:
		if x != nil {
			return float64(*x)
		}
	}
	return 0
}
