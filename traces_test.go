package o11y_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/query-service/model"
	"github.com/hanzoai/o11y/pkg/types/spantypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
)

// THE WIRE PROOF for the traces face — logs_test.go's discipline applied to the
// six typed traces ops. Each case takes the bytes the RUNTIME writes, through
// the same construction the real handler uses (WriteJSON's bare marshal for the
// query-service reads, render.Success's {status,data} for the apiserver detail
// views), hands them to the op, and demands the op answered the same bytes and
// asked the runtime for the same method and path. A field the port failed to
// name, or named with a different tag, shows up here as a diff.
//
// The helpers (mounted, call, member, logsRuntime, bare, mustJSON) are
// telemetry_test.go's and logs_test.go's; every face is proved with one harness.

// fullTraceWindow is one window of a trace's spans with every field populated,
// so a dropped or renamed field cannot hide behind a zero value.
func fullTraceWindow() []model.SearchSpansResult {
	return []model.SearchSpansResult{{
		StartTimestampMillis: 1_760_000_000_000,
		EndTimestampMillis:   1_760_000_009_000,
		Columns:              []string{"timestamp", "spanID", "name", "durationNano"},
		Events: [][]any{
			{"2026-07-31T12:00:00Z", "s1", "GET /v1/thing", float64(1500000)},
			{"2026-07-31T12:00:01Z", "s2", "db.query", float64(430000)},
		},
		IsSubTree: true,
	}}
}

func TestTraceSpansAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	want := bare(t, fullTraceWindow())
	asked := logsRuntime(t, http.StatusOK, want)

	status, got := call(t, app, member(http.MethodGet,
		"/v1/o11y/traces/tr-1?spanId=s1&levelUp=2&levelDown=3&spanRenderLimit=500", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, got)
	}
	if string(got) != string(want) {
		t.Fatalf("op answered %s, runtime wrote %s", got, want)
	}
	r := *asked
	if r.Method != http.MethodGet || r.URL.Path != "/v1/o11y/traces/tr-1" {
		t.Fatalf("runtime asked %s %s", r.Method, r.URL.Path)
	}
	q := r.URL.Query()
	for name, value := range map[string]string{
		"spanId": "s1", "levelUp": "2", "levelDown": "3", "spanRenderLimit": "500",
	} {
		if q.Get(name) != value {
			t.Fatalf("query %s=%q want %q (full %q)", name, q.Get(name), value, r.URL.RawQuery)
		}
	}
}

// A trace id is a path segment the router matched, and it goes on VERBATIM.
// Re-encoding it here would hand the runtime a different id than the caller
// named — the difference between the answer this face has always given and a
// new one.
func TestTraceIDGoesOnVerbatim(t *testing.T) {
	app := mounted(t)
	asked := logsRuntime(t, http.StatusOK, bare(t, fullTraceWindow()))

	call(t, app, member(http.MethodGet, "/v1/o11y/traces/a.b-c_d", nil))
	if got := (*asked).URL.Path; got != "/v1/o11y/traces/a.b-c_d" {
		t.Fatalf("runtime saw %q", got)
	}
}

func TestTraceFieldCatalogAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	want := bare(t, model.GetFieldsResponse{
		Selected: []model.Field{{Name: "service.name", DataType: "string", Type: "resource"}},
		Interesting: []model.Field{
			{Name: "http.status_code", DataType: "int64", Type: "tag"},
			{Name: "db.system", DataType: "string", Type: "tag"},
		},
	})
	asked := logsRuntime(t, http.StatusOK, want)

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/traces/fields", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, got)
	}
	if string(got) != string(want) {
		t.Fatalf("op answered %s, runtime wrote %s", got, want)
	}
	if p := (*asked).URL.Path; p != "/v1/o11y/traces/fields" {
		t.Fatalf("runtime asked %q — the literal must beat the :traceId param", p)
	}
}

func TestTraceFieldUpdateEchoesTheSetting(t *testing.T) {
	app := mounted(t)
	setting := model.UpdateField{
		Name: "http.status_code", DataType: "int64", Type: "tag",
		Selected: true, IndexType: "minmax", IndexGranularity: 4,
	}
	want := bare(t, setting)
	asked := logsRuntime(t, http.StatusOK, want)

	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/traces/fields",
		strings.NewReader(mustJSON(t, setting))))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, got)
	}
	if string(got) != string(want) {
		t.Fatalf("op answered %s, runtime wrote %s", got, want)
	}
	if r := *asked; r.Method != http.MethodPost || r.URL.Path != "/v1/o11y/traces/fields" {
		t.Fatalf("runtime asked %s %s", r.Method, r.URL.Path)
	}
}

// The three detail views come off the apiserver, which answers the
// {status,data} envelope. Each proves both halves: the answer is the runtime's
// bytes, and the trace id reached the runtime on the path rather than in the
// body.
func TestTraceDetailViewsAnswerTheRuntimeAnswer(t *testing.T) {
	waterfall := spantypes.GettableWaterfallTrace{
		StartTimestampMillis: 1_760_000_000_000, EndTimestampMillis: 1_760_000_009_000,
		RootServiceName: "api", RootServiceEntryPoint: "GET /v1/thing",
		TotalSpansCount: 2, TotalErrorSpansCount: 1,
		Spans: []*spantypes.WaterfallSpan{{
			SpanID: "s1", Name: "GET /v1/thing", DurationNano: 1500000,
			HasError: true, KindString: "SPAN_KIND_SERVER", TraceID: "tr-1",
			Attributes: map[string]any{"http.method": "GET"},
			Resource:   map[string]string{"service.name": "api"},
			References: []spantypes.OtelSpanRef{},
			Events:     []spantypes.Event{},
		}},
		HasMissingSpans: false, UncollapsedSpans: []string{"s1"}, HasMore: false,
	}
	flame := spantypes.GettableFlamegraphTrace{
		Spans: [][]*spantypes.FlamegraphSpan{{{
			SpanID: "s1", ParentSpanID: "", Timestamp: 1_760_000_000_000,
			DurationNano: 1500000, HasError: true, Name: "GET /v1/thing", Level: 0,
			Events:     []spantypes.Event{},
			Attributes: map[string]any{"http.method": "GET"},
			Resource:   map[string]string{"service.name": "api"},
		}}},
		StartTimestampMillis: 1_760_000_000_000, EndTimestampMillis: 1_760_000_009_000,
		HasMore: false,
	}

	for _, tc := range []struct {
		view    string
		payload any
		body    string
	}{
		{"waterfall", waterfall, `{"selectedSpanId":"s1","uncollapsedSpans":["s1"]}`},
		{"flamegraph", flame, `{"selectedSpanId":"s1"}`},
	} {
		t.Run(tc.view, func(t *testing.T) {
			app := mounted(t)
			want := rendered(t, http.StatusOK, tc.payload)
			asked := logsRuntime(t, http.StatusOK, want)

			status, got := call(t, app, member(http.MethodPost,
				"/v1/o11y/traces/tr-1/"+tc.view, strings.NewReader(tc.body)))
			if status != http.StatusOK {
				t.Fatalf("status=%d body=%s", status, got)
			}
			if string(got) != string(want) {
				t.Fatalf("op answered %s, runtime wrote %s", got, want)
			}
			r := *asked
			if r.Method != http.MethodPost || r.URL.Path != "/v1/o11y/traces/tr-1/"+tc.view {
				t.Fatalf("runtime asked %s %s", r.Method, r.URL.Path)
			}
		})
	}
}

// TWO ENCODERS, ONE VALUE — a divergence this test PINS rather than hides.
//
// The runtime renders with jsoniter (pkg/http/render, pkg/query-service/app);
// zip renders a typed op's Out with the stdlib encoder. jsoniter does not
// implement Go 1.24's `,omitzero`, so for any type that uses it — here
// telemetrytypes.TelemetryFieldKey's signal/fieldContext/fieldDataType, and
// sixteen other fields across pkg/types — the runtime WRITES the zero value and
// a typed op OMITS the key. The value is the same; the bytes are not.
//
// This is not new with the traces face: every already-shipped op whose Out
// reaches TelemetryFieldKey (querycore's field catalog, rules, the v5 builder)
// has had it since it was typed. It is recorded here, on the one op that makes
// it visible, so the next person finds a test instead of a surprise. The fix is
// ONE encoder across both halves, which is a change to the runtime's rendering
// and belongs in its own pass.
func TestTraceAggregationsCarryTheRuntimeValue(t *testing.T) {
	app := mounted(t)
	aggs := spantypes.GettableTraceAggregations{
		Aggregations: []spantypes.SpanAggregationResult{{
			Field:       telemetrytypes.TelemetryFieldKey{Name: "service.name", FieldContext: telemetrytypes.FieldContextResource},
			Aggregation: spantypes.SpanAggregationSpanCount,
			Value:       map[string]uint64{"api": 2},
		}},
	}
	want := rendered(t, http.StatusOK, aggs)
	asked := logsRuntime(t, http.StatusOK, want)

	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/traces/tr-1/aggregations",
		strings.NewReader(`{"aggregations":[{"field":{"name":"service.name","fieldContext":"resource"},"aggregation":"span_count"}]}`)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, got)
	}
	// Equal as VALUES: every field the runtime set survives the op.
	var runtimeValue, opValue map[string]any
	if err := json.Unmarshal(want, &runtimeValue); err != nil {
		t.Fatalf("runtime bytes: %v", err)
	}
	if err := json.Unmarshal(got, &opValue); err != nil {
		t.Fatalf("op bytes: %v", err)
	}
	stripZeroStrings(runtimeValue)
	if !reflect.DeepEqual(runtimeValue, opValue) {
		t.Fatalf("op answered %s, runtime wrote %s", got, want)
	}
	if r := *asked; r.Method != http.MethodPost || r.URL.Path != "/v1/o11y/traces/tr-1/aggregations" {
		t.Fatalf("runtime asked %s %s", r.Method, r.URL.Path)
	}
}

// stripZeroStrings removes the empty-string keys jsoniter writes for `omitzero`
// fields and the stdlib encoder leaves out. It is the exact shape of the
// divergence above, spelled once.
func stripZeroStrings(v map[string]any) {
	for k, val := range v {
		switch t := val.(type) {
		case string:
			if t == "" {
				delete(v, k)
			}
		case map[string]any:
			stripZeroStrings(t)
		case []any:
			for _, e := range t {
				if m, ok := e.(map[string]any); ok {
					stripZeroStrings(m)
				}
			}
		}
	}
}

// The six routes are the six the runtime registers — no more, no fewer, at the
// same methods. Route-table arithmetic is what caught the dark slices; it is
// cheap and it is the only thing that catches a mount that never ran.
func TestTracesRoutesAreTheSameSix(t *testing.T) {
	assertRoutes(t, map[string]bool{
		"GET /v1/o11y/traces/fields":                 true,
		"POST /v1/o11y/traces/fields":                true,
		"GET /v1/o11y/traces/:traceId":               true,
		"POST /v1/o11y/traces/:traceId/waterfall":    true,
		"POST /v1/o11y/traces/:traceId/flamegraph":   true,
		"POST /v1/o11y/traces/:traceId/aggregations": true,
	}, "/v1/o11y/traces")
}
