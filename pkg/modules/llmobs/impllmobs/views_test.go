package impllmobs

import (
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/types/llmobstypes"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
)

func TestResolveWindow(t *testing.T) {
	// explicit valid window is preserved
	if s, e := resolveWindow(100, 200); s != 100 || e != 200 {
		t.Fatalf("explicit window: got (%d,%d), want (100,200)", s, e)
	}
	// zero window defaults to last 24h and stays ordered
	s, e := resolveWindow(0, 0)
	if s >= e {
		t.Fatalf("zero window not ordered: (%d,%d)", s, e)
	}
	if got := e - s; got != uint64(defaultLookback.Milliseconds()) {
		t.Fatalf("zero window span: got %d ms, want %d", got, defaultLookback.Milliseconds())
	}
	// start >= end is invalid and falls back to a 24h lookback from end
	s, e = resolveWindow(500, 200)
	if s >= e || e != 200 {
		t.Fatalf("inverted window not corrected: (%d,%d)", s, e)
	}
}

func TestClampLimitOffset(t *testing.T) {
	cases := []struct{ in, want int }{{0, defaultViewLimit}, {-3, defaultViewLimit}, {30, 30}, {10000, maxViewLimit}}
	for _, c := range cases {
		if got := clampLimit(c.in); got != c.want {
			t.Errorf("clampLimit(%d)=%d want %d", c.in, got, c.want)
		}
	}
	if clampOffset(-5) != 0 || clampOffset(7) != 7 {
		t.Errorf("clampOffset mismatch")
	}
}

func TestGenAIFilter(t *testing.T) {
	// bare query yields only the marker, no variables
	expr, vars := genAIFilter(&llmobstypes.ViewQuery{})
	if expr != genAIMarker {
		t.Fatalf("bare filter = %q, want %q", expr, genAIMarker)
	}
	if len(vars) != 0 {
		t.Fatalf("bare filter should have no vars, got %d", len(vars))
	}

	// predicates become $N placeholders bound safely in the variables map
	q := &llmobstypes.ViewQuery{TraceID: "t1", Model: "gpt-4o"}
	expr, vars = genAIFilter(q)
	want := genAIMarker + " AND trace_id = $1 AND gen_ai.request.model = $2"
	if expr != want {
		t.Fatalf("filter = %q, want %q", expr, want)
	}
	if v, ok := vars["1"]; !ok || v.Value != "t1" || v.Type != qbtypes.DynamicVariableType {
		t.Fatalf("var 1 = %+v, want t1/dynamic", v)
	}
	if v, ok := vars["2"]; !ok || v.Value != "gpt-4o" {
		t.Fatalf("var 2 = %+v, want gpt-4o", v)
	}

	// requireExists injects EXISTS clauses after the marker
	expr, _ = genAIFilter(&llmobstypes.ViewQuery{}, llmobstypes.SessionID)
	if expr != genAIMarker+" AND "+llmobstypes.SessionID+" EXISTS" {
		t.Fatalf("requireExists filter = %q", expr)
	}
}

func TestBuildObservationsQuery(t *testing.T) {
	req := buildObservationsQuery(&llmobstypes.ViewQuery{Limit: 10, Offset: 5})
	if req.RequestType != qbtypes.RequestTypeRaw {
		t.Fatalf("requestType = %v, want raw", req.RequestType)
	}
	if req.Start >= req.End {
		t.Fatalf("window not ordered: (%d,%d)", req.Start, req.End)
	}
	spec, ok := req.CompositeQuery.Queries[0].Spec.(qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation])
	if !ok {
		t.Fatalf("spec type = %T", req.CompositeQuery.Queries[0].Spec)
	}
	if spec.Signal != telemetrytypes.SignalTraces {
		t.Fatalf("signal = %v, want traces", spec.Signal)
	}
	if spec.Limit != 10 || spec.Offset != 5 {
		t.Fatalf("limit/offset = %d/%d, want 10/5", spec.Limit, spec.Offset)
	}
	if len(spec.Aggregations) != 0 {
		t.Fatalf("raw query must not have aggregations, got %d", len(spec.Aggregations))
	}
	if !hasSelectField(spec.SelectFields, "trace_id") || !hasSelectField(spec.SelectFields, "gen_ai.request.model") {
		t.Fatalf("select fields missing required keys: %+v", spec.SelectFields)
	}
}

func TestBuildScalarQueries(t *testing.T) {
	sessions := buildSessionsQuery(&llmobstypes.ViewQuery{})
	if sessions.RequestType != qbtypes.RequestTypeScalar {
		t.Fatalf("sessions requestType = %v, want scalar", sessions.RequestType)
	}
	sSpec := sessions.CompositeQuery.Queries[0].Spec.(qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation])
	if !hasGroupBy(sSpec.GroupBy, llmobstypes.SessionID) {
		t.Fatalf("sessions must group by session.id: %+v", sSpec.GroupBy)
	}
	if len(sSpec.Aggregations) == 0 {
		t.Fatalf("scalar query must have aggregations")
	}
	if sSpec.Filter.Expression != genAIMarker+" AND "+llmobstypes.SessionID+" EXISTS" {
		t.Fatalf("sessions filter = %q", sSpec.Filter.Expression)
	}

	users := buildUsersQuery(&llmobstypes.ViewQuery{})
	uSpec := users.CompositeQuery.Queries[0].Spec.(qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation])
	if !hasGroupBy(uSpec.GroupBy, llmobstypes.UserID) {
		t.Fatalf("users must group by user.id")
	}

	traces := buildTracesQuery(&llmobstypes.ViewQuery{})
	tSpec := traces.CompositeQuery.Queries[0].Spec.(qbtypes.QueryBuilderQuery[qbtypes.TraceAggregation])
	if !hasGroupBy(tSpec.GroupBy, "trace_id") {
		t.Fatalf("traces must group by trace_id")
	}
}

func TestMapObservations(t *testing.T) {
	ts := time.Unix(1700000000, 0)
	resp := rawResp(&qbtypes.RawRow{Timestamp: ts, Data: map[string]any{
		"span_id":                    "s1",
		"trace_id":                   "t1",
		"parent_span_id":             "p1",
		"name":                       "chat",
		"duration_nano":              uint64(2_000_000), // 2ms
		"response_status_code":       "200",
		"gen_ai.request.model":       "gpt-4o-mini",
		"gen_ai.response.model":      "gpt-4o-2024",
		"gen_ai.system":              "openai",
		"gen_ai.operation.name":      "chat",
		"gen_ai.usage.input_tokens":  float64(100),
		"gen_ai.usage.output_tokens": float64(50),
		"_signoz.gen_ai.total_cost":  float64(0.0025),
		"session.id":                 "sess1",
		"user.id":                    "user1",
		"service.name":               "app",
	}})

	got := mapObservations(resp)
	if len(got) != 1 {
		t.Fatalf("got %d observations, want 1", len(got))
	}
	o := got[0]
	if o.ID != "s1" || o.TraceID != "t1" || o.ParentID != "p1" || o.Name != "chat" {
		t.Errorf("identity fields wrong: %+v", o)
	}
	if o.Model != "gpt-4o-2024" { // response.model wins over request.model
		t.Errorf("model = %q, want response model gpt-4o-2024", o.Model)
	}
	if o.Provider != "openai" || o.Type != "CHAT" {
		t.Errorf("provider/type = %q/%q", o.Provider, o.Type)
	}
	if o.PromptTokens != 100 || o.CompletionTokens != 50 || o.TotalTokens != 150 {
		t.Errorf("tokens = %d/%d/%d", o.PromptTokens, o.CompletionTokens, o.TotalTokens)
	}
	if o.TotalCost != 0.0025 || o.LatencyMs != 2.0 {
		t.Errorf("cost/latency = %v/%v", o.TotalCost, o.LatencyMs)
	}
	if o.SessionID != "sess1" || o.UserID != "user1" || o.ServiceName != "app" || o.StatusCode != "200" {
		t.Errorf("attrs = %+v", o)
	}
	if !o.StartTime.Equal(ts) {
		t.Errorf("startTime = %v, want %v", o.StartTime, ts)
	}
}

func TestMapObservationsModelFallback(t *testing.T) {
	// when response.model is absent, request.model is used
	resp := rawResp(&qbtypes.RawRow{Data: map[string]any{
		"span_id":              "s2",
		"gen_ai.request.model": "claude-4",
	}})
	got := mapObservations(resp)
	if got[0].Model != "claude-4" {
		t.Fatalf("model fallback = %q, want claude-4", got[0].Model)
	}
	if got[0].Type != "GENERATION" { // no operation.name -> default type
		t.Fatalf("default type = %q, want GENERATION", got[0].Type)
	}
}

func TestMapSessions(t *testing.T) {
	sd := &qbtypes.ScalarData{
		Columns: []*qbtypes.ColumnDescriptor{
			groupCol(llmobstypes.SessionID),
			groupCol(llmobstypes.UserID),
			aggCol(0), aggCol(1), aggCol(2), aggCol(3), aggCol(4),
		},
		Data: [][]any{{"sess1", "user1", uint64(3), uint64(9), float64(300), float64(150), float64(0.05)}},
	}
	got := mapSessions(scalarResp(sd))
	if len(got) != 1 {
		t.Fatalf("got %d sessions", len(got))
	}
	s := got[0]
	if s.ID != "sess1" || s.UserID != "user1" {
		t.Errorf("session identity = %+v", s)
	}
	if s.Traces != 3 || s.Observations != 9 {
		t.Errorf("counts = %d/%d, want 3/9", s.Traces, s.Observations)
	}
	if s.PromptTokens != 300 || s.CompletionTokens != 150 || s.TotalTokens != 450 || s.TotalCost != 0.05 {
		t.Errorf("rollups = %+v", s)
	}
}

func TestMapTraces(t *testing.T) {
	sd := &qbtypes.ScalarData{
		Columns: []*qbtypes.ColumnDescriptor{
			groupCol("trace_id"), groupCol(llmobstypes.SessionID), groupCol(llmobstypes.UserID), groupCol(llmobstypes.ServiceName),
			aggCol(0), aggCol(1), aggCol(2), aggCol(3), aggCol(4),
		},
		Data: [][]any{{"t1", "sess1", "user1", "app", uint64(2), float64(80), float64(40), float64(0.02), uint64(5_000_000)}},
	}
	got := mapTraces(scalarResp(sd))
	tr := got[0]
	if tr.ID != "t1" || tr.SessionID != "sess1" || tr.UserID != "user1" || tr.ServiceName != "app" {
		t.Errorf("trace identity = %+v", tr)
	}
	if tr.Observations != 2 || tr.TotalTokens != 120 || tr.TotalCost != 0.02 || tr.LatencyMs != 5.0 {
		t.Errorf("trace rollups = %+v", tr)
	}
}

func TestMapUsers(t *testing.T) {
	sd := &qbtypes.ScalarData{
		Columns: []*qbtypes.ColumnDescriptor{
			groupCol(llmobstypes.UserID),
			aggCol(0), aggCol(1), aggCol(2), aggCol(3), aggCol(4), aggCol(5),
		},
		Data: [][]any{{"user1", uint64(4), uint64(12), uint64(40), float64(1000), float64(500), float64(0.9)}},
	}
	got := mapUsers(scalarResp(sd))
	u := got[0]
	if u.ID != "user1" || u.Sessions != 4 || u.Traces != 12 || u.Observations != 40 {
		t.Errorf("user counts = %+v", u)
	}
	if u.TotalTokens != 1500 || u.TotalCost != 0.9 {
		t.Errorf("user rollups = %+v", u)
	}
}

func TestMapEmptyResponses(t *testing.T) {
	if got := mapObservations(nil); len(got) != 0 {
		t.Errorf("nil observations should be empty")
	}
	if got := mapSessions(&qbtypes.QueryRangeResponse{}); len(got) != 0 {
		t.Errorf("empty sessions should be empty")
	}
}

func TestCoercions(t *testing.T) {
	str := "x"
	if asString("a") != "a" || asString(&str) != "x" || asString([]byte("b")) != "b" || asString(nil) != "" {
		t.Errorf("asString coercion wrong")
	}
	f := 3.5
	u := uint64(7)
	if asFloat(1.5) != 1.5 || asFloat(&f) != 3.5 || asFloat(uint64(2)) != 2 || asFloat(&u) != 7 || asFloat(int64(4)) != 4 || asFloat(nil) != 0 {
		t.Errorf("asFloat coercion wrong")
	}
}

// --- helpers ---

func rawResp(rows ...*qbtypes.RawRow) *qbtypes.QueryRangeResponse {
	return &qbtypes.QueryRangeResponse{Data: qbtypes.QueryData{Results: []any{&qbtypes.RawData{Rows: rows}}}}
}

func scalarResp(sd *qbtypes.ScalarData) *qbtypes.QueryRangeResponse {
	return &qbtypes.QueryRangeResponse{Data: qbtypes.QueryData{Results: []any{sd}}}
}

func groupCol(name string) *qbtypes.ColumnDescriptor {
	return &qbtypes.ColumnDescriptor{TelemetryFieldKey: telemetrytypes.TelemetryFieldKey{Name: name}, Type: qbtypes.ColumnTypeGroup}
}

func aggCol(index int64) *qbtypes.ColumnDescriptor {
	return &qbtypes.ColumnDescriptor{AggregationIndex: index, Type: qbtypes.ColumnTypeAggregation}
}

func hasSelectField(fields []telemetrytypes.TelemetryFieldKey, name string) bool {
	for _, f := range fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

func hasGroupBy(keys []qbtypes.GroupByKey, name string) bool {
	for _, k := range keys {
		if k.Name == name {
			return true
		}
	}
	return false
}
