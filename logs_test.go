package o11y_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/query-service/app/logparsingpipeline"
	"github.com/hanzoai/o11y/pkg/query-service/model"
	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/opamptypes"
	"github.com/hanzoai/o11y/pkg/types/pipelinetypes"
	"github.com/hanzoai/o11y/pkg/types/promotetypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/zap-proto/zip"
)

// THE WIRE PROOF for the logs face — telemetry_test.go's discipline applied to
// the nine typed logs ops. Each test takes the bytes the RUNTIME wrote —
// through the same construction the real handler uses: the bare marshal of
// WriteJSON, package app's {status,data} envelope, or render.Success — and the
// bytes the OP answered, and demands they are the same bytes, for a payload
// built from the runtime's OWN types with every field populated. A field the
// port failed to name, or named with a different tag, or ordered differently,
// shows up here as a diff. The helpers (mounted, call, member, mustJSON) are
// telemetry_test.go's; both faces are proved with one harness.

// logsRuntime installs a stand-in o11y runtime that answers every call with
// exactly the given status and bytes, and reports what it was asked.
func logsRuntime(t *testing.T, status int, body []byte) (asked **http.Request) {
	t.Helper()
	var req *http.Request
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, _ := io.ReadAll(r.Body)
		req = r.Clone(r.Context())
		req.Body = io.NopCloser(bytes.NewReader(read))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })
	return &req
}

// bare marshals a payload the way WriteJSON answers the record, field and
// aggregate reads: the payload itself, no envelope.
func bare(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// enveloped marshals a payload the way package app's Respond answers the
// pipelines routes: the {status,data} envelope, in that field order.
func enveloped(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(struct {
		Status string `json:"status"`
		Data   any    `json:"data,omitempty"`
	}{"success", v})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// rendered writes a payload through the SAME render.Success the promotion
// handlers use, and returns the bytes it produced.
func rendered(t *testing.T, status int, v any) []byte {
	t.Helper()
	rec := httptest.NewRecorder()
	render.Success(rec, status, v)
	return rec.Body.Bytes()
}

// A window of records is what the runtime wrote, to the byte — each record an
// open object, so the proof carries values of every JSON type. The inputs go
// on as the caller's own numbers, and an unset input stays ABSENT rather than
// arriving as zero.
func TestLogRecordsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote := bare(t, map[string]any{"results": []map[string]any{{
		"timestamp":         int64(1753900000000000000),
		"body":              "GET /v1/thing 200",
		"severity_text":     "INFO",
		"severity_number":   9,
		"attributes_string": map[string]string{"http.method": "GET"},
		"attributes_int":    map[string]int64{"http.status": 200},
		"handled":           true,
		"nested":            map[string]any{"deep": []any{1, "two", nil}},
	}}})
	asked := logsRuntime(t, http.StatusOK, wrote)

	status, got := call(t, app, member(http.MethodGet,
		"/v1/o11y/logs?limit=50&timestampStart=1753899000000000000&timestampEnd=1753900000000000000", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/logs" ||
		r.URL.Query().Get("limit") != "50" ||
		r.URL.Query().Get("timestampStart") != "1753899000000000000" ||
		r.URL.Query().Get("timestampEnd") != "1753900000000000000" {
		t.Fatalf("runtime was asked %s?%s, want the caller's own inputs", r.URL.Path, r.URL.RawQuery)
	}

	// No inputs, no parameters: the runtime reads its own defaults, exactly as
	// it always has, rather than a zero the caller never sent.
	if status, _ := call(t, app, member(http.MethodGet, "/v1/o11y/logs", nil)); status != http.StatusOK {
		t.Fatalf("status=%d, want 200", status)
	}
	if q := (*asked).URL.RawQuery; q != "" {
		t.Fatalf("an absent input reached the runtime as %q, want none", q)
	}
}

// The field catalog is what the runtime wrote, to the byte, through the
// runtime's own catalog type.
func TestLogFieldsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote := bare(t, model.GetFieldsResponse{
		Selected:    []model.Field{{Name: "trace_id", DataType: "string", Type: "attributes"}},
		Interesting: []model.Field{{Name: "user.tier", DataType: "string", Type: "resources"}},
	})
	asked := logsRuntime(t, http.StatusOK, wrote)

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/logs/fields", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/logs/fields" || r.Method != http.MethodGet {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
}

// The field write echoes what the runtime echoed — and the setting the runtime
// receives is the setting the caller sent, field for field, through the
// runtime's own UpdateField type.
func TestLogFieldUpdateEchoIsTheRuntimeEcho(t *testing.T) {
	app := mounted(t)
	setting := model.UpdateField{
		Name: "user.tier", DataType: "string", Type: "attributes",
		Selected: true, IndexType: "bloom_filter(0.01)", IndexGranularity: 64,
	}
	wrote := bare(t, setting)
	asked := logsRuntime(t, http.StatusOK, wrote)

	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/logs/fields",
		bytes.NewReader(mustBody(t, setting))))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", wrote, got)
	}

	forwarded, _ := io.ReadAll((*asked).Body)
	var have model.UpdateField
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, setting), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the request.\n caller: %s\n runtime: %s", a, b)
	}
}

// The aggregate read is what the runtime wrote, to the byte, through the
// runtime's own aggregates type — buckets keyed by timestamp, values of the
// value's own type.
func TestLogAggregateAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote := bare(t, model.GetLogsAggregatesResponse{
		Items: map[int64]model.LogsAggregatesResponseItem{
			1753900000: {Timestamp: 1753900000, Value: 42.5, GroupBy: map[string]any{"service": "api"}},
			1753900060: {Timestamp: 1753900060, Value: 7},
		},
	})
	logsRuntime(t, http.StatusOK, wrote)

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/logs/aggregate", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", wrote, got)
	}
}

// fullPipeline is one stored pipeline with EVERY wire field populated — the
// audit trail, a filter with a fully-specified key, and processors exercising
// the trace-parser, router, flattening and severity-mapping keys — so a
// dropped or renamed field cannot hide behind a zero value.
func fullPipeline(at time.Time) pipelinetypes.GettablePipeline {
	return pipelinetypes.GettablePipeline{
		StoreablePipeline: pipelinetypes.StoreablePipeline{
			UserAuditable: types.UserAuditable{CreatedBy: "u1", UpdatedBy: "u2"},
			TimeAuditable: types.TimeAuditable{CreatedAt: at, UpdatedAt: at.Add(time.Minute)},
			Identifiable:  types.Identifiable{ID: valuer.MustNewUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8")},
			OrderID:       1, Enabled: true, Name: "nginx access", Alias: "nginx",
			Description: "parse access lines",
		},
		Filter: &v3.FilterSet{Operator: "AND", Items: []v3.FilterItem{{
			Key: v3.AttributeKey{
				Key: "source", DataType: v3.AttributeKeyDataTypeString,
				Type: v3.AttributeKeyTypeTag, IsColumn: true, IsJSON: false,
			},
			Value: "nginx", Operator: "=",
		}}},
		Config: []pipelinetypes.PipelineOperator{
			{
				Type: "grok_parser", ID: "p1", Output: "p2", OnError: "send", If: `source == "nginx"`,
				OrderId: 1, Enabled: true, Name: "grok",
				ParseTo: "attributes", Pattern: "%{COMMONAPACHELOG}", ParseFrom: "body",
				TraceParser: &pipelinetypes.TraceParser{
					TraceId:    &pipelinetypes.ParseFrom{ParseFrom: "attributes.tid"},
					SpanId:     &pipelinetypes.ParseFrom{ParseFrom: "attributes.sid"},
					TraceFlags: &pipelinetypes.ParseFrom{ParseFrom: "attributes.tf"},
				},
				Field: "attributes.env", Value: "prod", From: "attributes.a", To: "attributes.b",
			},
			{
				Type: "router", ID: "p2", OrderId: 2, Enabled: true, Name: "route",
				Expr:   `severity_text == "ERROR"`,
				Routes: &[]pipelinetypes.Route{{Output: "p3", Expr: `body contains "boom"`}},
				Fields: []string{"attributes.keep"}, Default: "end",
				Layout: "%Y-%m-%d", LayoutType: "strptime",
				EnableFlattening: true, EnablePaths: true, PathPrefix: "log",
				Mapping:               map[string][]string{"error": {"ERR", "FATAL"}},
				OverwriteSeverityText: true,
			},
		},
	}
}

// A pipelines config version is what the runtime wrote, to the byte — the
// deployment record inlined when a version exists, the pipelines, and the
// history — and the version the caller names is the segment the runtime is
// asked, verbatim.
func TestLogPipelinesAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	version := opamptypes.AgentConfigVersion{
		CreatedByName: "Zed",
		Identifiable:  types.Identifiable{ID: valuer.MustNewUUID("7ba7b810-9dad-11d1-80b4-00c04fd430c8")},
		TimeAuditable: types.TimeAuditable{CreatedAt: at, UpdatedAt: at.Add(time.Minute)},
		UserAuditable: types.UserAuditable{CreatedBy: "u1", UpdatedBy: "u2"},
		OrgID:         valuer.MustNewUUID("8ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		Version:       3, ElementType: opamptypes.ElementTypeLogPipelines,
		DeployStatus: opamptypes.Deployed, DeploySequence: 2,
		DeployResult: "deployed successfully", Hash: "abc123", Config: "receivers: {}",
	}
	app := mounted(t)
	wrote := enveloped(t, logparsingpipeline.PipelinesResponse{
		AgentConfigVersion: &version,
		Pipelines:          []pipelinetypes.GettablePipeline{fullPipeline(at)},
		History:            []opamptypes.AgentConfigVersion{version},
	})
	asked := logsRuntime(t, http.StatusOK, wrote)

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/logs/pipelines/latest", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/logs/pipelines/latest" {
		t.Fatalf("runtime was asked %s, want the caller's own version segment", r.URL.Path)
	}

	// A numbered version rides the same segment.
	call(t, app, member(http.MethodGet, "/v1/o11y/logs/pipelines/3", nil))
	if r := *asked; r.URL.Path != "/v1/o11y/logs/pipelines/3" {
		t.Fatalf("runtime was asked %s, want /v1/o11y/logs/pipelines/3", r.URL.Path)
	}

	// Before any version exists the deployment record is ABSENT, not zeroed —
	// the nil embed stays nil on the wire.
	empty := enveloped(t, logparsingpipeline.PipelinesResponse{
		Pipelines: []pipelinetypes.GettablePipeline{},
		History:   []opamptypes.AgentConfigVersion{},
	})
	logsRuntime(t, http.StatusOK, empty)
	_, got = call(t, app, member(http.MethodGet, "/v1/o11y/logs/pipelines/latest", nil))
	if !bytes.Equal(got, empty) {
		t.Fatalf("the op invented a deployment record.\n runtime: %s\n op:      %s", empty, got)
	}
}

// A dry run's answer is what the runtime wrote, to the byte, and the pipelines
// and sample records the runtime receives are the caller's own, field for
// field, through the runtime's own preview types.
func TestLogPipelinePreviewAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	record := model.O11yLog{
		Timestamp: 1753900000000000000, ID: "r1", TraceID: "t1", SpanID: "s1", TraceFlags: 1,
		SeverityText: "ERROR", SeverityNumber: 17, Body: "boom",
		Resources_string:   map[string]string{"service.name": "api"},
		Attributes_string:  map[string]string{"env": "prod"},
		Attributes_int64:   map[string]int64{"code": 500},
		Attributes_float64: map[string]float64{"latency": 0.25},
		Attributes_bool:    map[string]bool{"handled": false},
	}
	app := mounted(t)
	wrote := enveloped(t, logparsingpipeline.PipelinesPreviewResponse{
		OutputLogs:    []model.O11yLog{record},
		CollectorLogs: []string{"simulated 1 record"},
	})
	asked := logsRuntime(t, http.StatusOK, wrote)

	sent := logparsingpipeline.PipelinesPreviewRequest{
		Pipelines: []pipelinetypes.GettablePipeline{fullPipeline(at)},
		Logs:      []model.O11yLog{record},
	}
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/logs/pipelines/preview",
		bytes.NewReader(mustBody(t, sent))))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", wrote, got)
	}

	forwarded, _ := io.ReadAll((*asked).Body)
	var have logparsingpipeline.PipelinesPreviewRequest
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, sent), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the request.\n caller: %s\n runtime: %s", a, b)
	}
}

// The pipeline save forwards the caller's set through the runtime's own
// postable type and answers with the version the runtime created.
func TestLogPipelineCreateForwardsThePipelines(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	app := mounted(t)
	wrote := enveloped(t, logparsingpipeline.PipelinesResponse{
		Pipelines: []pipelinetypes.GettablePipeline{fullPipeline(at)},
		History:   []opamptypes.AgentConfigVersion{},
	})
	asked := logsRuntime(t, http.StatusOK, wrote)

	sent := struct {
		Pipelines []pipelinetypes.PostablePipeline `json:"pipelines"`
	}{[]pipelinetypes.PostablePipeline{{
		ID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", OrderID: 1,
		Name: "nginx access", Alias: "nginx", Description: "parse access lines", Enabled: true,
		Filter: &v3.FilterSet{Operator: "AND", Items: []v3.FilterItem{{
			Key:   v3.AttributeKey{Key: "source", DataType: v3.AttributeKeyDataTypeString, Type: v3.AttributeKeyTypeTag},
			Value: "nginx", Operator: "=",
		}}},
		Config: []pipelinetypes.PipelineOperator{{
			Type: "grok_parser", ID: "p1", OrderId: 1, Enabled: true, Name: "grok",
			ParseTo: "attributes", Pattern: "%{COMMONAPACHELOG}", ParseFrom: "body",
		}},
	}}}
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/logs/pipelines",
		bytes.NewReader(mustBody(t, sent))))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", wrote, got)
	}

	forwarded, _ := io.ReadAll((*asked).Body)
	var have struct {
		Pipelines []pipelinetypes.PostablePipeline `json:"pipelines"`
	}
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, sent), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the request.\n caller: %s\n runtime: %s", a, b)
	}
}

// fullPromotePath is one promoted path with every wire field populated.
func fullPromotePath() promotetypes.PromotePath {
	return promotetypes.PromotePath{
		Path: "body.user.id", Promote: true,
		Indexes: []promotetypes.WrappedIndex{{
			FieldDataType: telemetrytypes.FieldDataTypeString,
			Type:          "bloom_filter(0.01)", Granularity: 64,
		}},
	}
}

// A promotion answers the runtime's own 201 and its own acknowledgement, and
// the paths the runtime receives are the caller's, field for field, as the
// bare array this route has always read.
func TestLogPromoteAnswersTheRuntime201(t *testing.T) {
	app := mounted(t)
	wrote := rendered(t, http.StatusCreated, nil)
	asked := logsRuntime(t, http.StatusCreated, wrote)

	sent := []promotetypes.PromotePath{fullPromotePath()}
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/logs/promote_paths",
		bytes.NewReader(mustBody(t, sent))))
	if status != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201 (the runtime's own)", status, got)
	}
	if !bytes.Equal(got, wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", wrote, got)
	}

	forwarded, _ := io.ReadAll((*asked).Body)
	var have []promotetypes.PromotePath
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, sent), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the request.\n caller: %s\n runtime: %s", a, b)
	}
}

// The promoted-paths list is what the runtime wrote, to the byte, through the
// SAME render.Success the handler uses.
func TestLogPromotedAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote := rendered(t, http.StatusOK, []promotetypes.PromotePath{fullPromotePath()})
	logsRuntime(t, http.StatusOK, wrote)

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/logs/promote_paths", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", wrote, got)
	}
}

// THE ROUTES, exactly as they were: nine typed paths under /v1/o11y/logs, and
// not a tenth — livetail is deliberately NOT among them.
func TestLogsRoutesAreTheSameNine(t *testing.T) {
	app := mounted(t)
	want := map[string]bool{
		"GET /v1/o11y/logs":                    true,
		"GET /v1/o11y/logs/fields":             true,
		"POST /v1/o11y/logs/fields":            true,
		"GET /v1/o11y/logs/aggregate":          true,
		"POST /v1/o11y/logs/pipelines/preview": true,
		"GET /v1/o11y/logs/pipelines/:version": true,
		"POST /v1/o11y/logs/pipelines":         true,
		"POST /v1/o11y/logs/promote_paths":     true,
		"GET /v1/o11y/logs/promote_paths":      true,

		// The tenth door: a NAMED hatch, not a typed op and not a fall-through.
		// It is in this census precisely because it is registered — a hatch that
		// stops being registered is a 404 now, and this is what catches that.
		"GET /v1/o11y/logs/livetail": true,
	}
	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		if strings.HasPrefix(r.Path, "/v1/o11y/logs") && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			got[r.Method+" "+r.Path] = true
		}
	}
	for route := range want {
		if !got[route] {
			t.Errorf("%s is not registered", route)
		}
	}
	for route := range got {
		if !want[route] {
			t.Errorf("%s is registered and was not before — the face grew a door", route)
		}
	}
}

// Live tail is the tenth route and one of the ten honest escape hatches: a
// stream that never completes cannot be a buffered typed op. It reaches the
// runtime through its OWN named route now, bytes untouched — a typed relay would
// have choked on a non-JSON stream frame, so verbatim arrival IS the proof of
// the door it went through.
func TestLivetailIsANamedHatch(t *testing.T) {
	app := mounted(t)
	frame := "event: log\ndata: {\"body\":\"boom\"}\n\n"
	var askedPath string
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		askedPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, frame)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/logs/livetail?q=service%3Dapi", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if string(got) != frame {
		t.Fatalf("the stream no longer passes through whole.\n runtime: %q\n caller:  %q", frame, got)
	}
	if askedPath != "/v1/o11y/logs/livetail" {
		t.Fatalf("runtime was asked %s, want /v1/o11y/logs/livetail", askedPath)
	}
}

// The caller's identity travels to the runtime as the gateway asserted it —
// relayAt propagates, never mints.
func TestLogsIdentityIsPropagated(t *testing.T) {
	app := mounted(t)
	asked := logsRuntime(t, http.StatusOK, bare(t, map[string]any{"results": []any{}}))

	r := member(http.MethodGet, "/v1/o11y/logs", nil)
	r.Header.Set(zip.HeaderUserAdmin, "true")
	r.Header.Set(zip.HeaderProject, "proj-9")
	if status, body := call(t, app, r); status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	got := (*asked).Header
	for header, want := range map[string]string{
		zip.HeaderOrg:       "maxpower",
		zip.HeaderUser:      "z",
		zip.HeaderUserEmail: "z@hanzo.ai",
		zip.HeaderUserAdmin: "true",
		zip.HeaderProject:   "proj-9",
	} {
		if got.Get(header) != want {
			t.Errorf("%s reached the runtime as %q, want %q", header, got.Get(header), want)
		}
	}
}

// A refusal keeps the runtime's status and its reason, whichever of the two
// refusal shapes this face answers with.
func TestLogsRefusalKeepsTheRuntimeStatus(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"gate", `{"status":"error","msg":"an org-scoped principal is required"}`, "an org-scoped principal is required", http.StatusForbidden},
		{"runtime", `{"status":"error","error":{"code":"invalid_input","message":"index granularity must be greater than 0"}}`, "index granularity must be greater than 0", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := mounted(t)
			logsRuntime(t, tc.status, []byte(tc.body))

			status, got := call(t, app, member(http.MethodGet, "/v1/o11y/logs/promote_paths", nil))
			if status != tc.status {
				t.Fatalf("status=%d, want %d (the runtime's own)", status, tc.status)
			}
			if !strings.Contains(string(got), tc.want) {
				t.Fatalf("the reason was lost: %s", got)
			}
		})
	}
}

// No runtime, no answer: typed ops and the wildcard both fail closed with the
// same 503.
func TestLogsFailClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(nil)

	for _, target := range []string{
		"/v1/o11y/logs",
		"/v1/o11y/logs/fields",
		"/v1/o11y/logs/pipelines/latest",
		"/v1/o11y/logs/promote_paths",
		"/v1/o11y/logs/livetail", // the wildcard's own refusal
	} {
		if status, body := call(t, app, member(http.MethodGet, target, nil)); status != http.StatusServiceUnavailable {
			t.Errorf("%s: status=%d body=%s, want 503", target, status, body)
		}
	}
}

// THE POINT OF THE PORT: the nine ops are in the document now, each with its
// operation id and its prose — and livetail, the deliberate escape hatch, is
// NOT, because a stream the document cannot describe honestly does not belong
// in it.
func TestLogsReachTheDocument(t *testing.T) {
	app := mounted(t)
	raw, err := json.Marshal(app.OpenAPISpec())
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Summary     string `json:"summary"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("spec: %v", err)
	}

	for path, methods := range map[string][]string{
		"/v1/o11y/logs":                     {"get"},
		"/v1/o11y/logs/fields":              {"get", "post"},
		"/v1/o11y/logs/aggregate":           {"get"},
		"/v1/o11y/logs/pipelines/preview":   {"post"},
		"/v1/o11y/logs/pipelines/{version}": {"get"},
		"/v1/o11y/logs/pipelines":           {"post"},
		"/v1/o11y/logs/promote_paths":       {"get", "post"},
	} {
		for _, method := range methods {
			op, ok := spec.Paths[path][method]
			if !ok {
				t.Errorf("%s %s is not in the document", strings.ToUpper(method), path)
				continue
			}
			if op.OperationID == "" {
				t.Errorf("%s %s has no operation id, so nothing can name it", method, path)
			}
			if len(op.Summary) < 20 {
				t.Errorf("%s %s has no prose in the document: %q", method, path, op.Summary)
			}
		}
	}
	if _, there := spec.Paths["/v1/o11y/logs/livetail"]; there {
		t.Error("livetail entered the document as a typed op — it is a stream, and the document would lie about it")
	}
}

// mustBody marshals a request body or fails the test.
func mustBody(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return b
}
