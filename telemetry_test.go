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
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/zap-proto/zip"
)

// THE WIRE PROOF.
//
// The five telemetry reads are typed ops now, and a typed op that changes what a
// caller receives is a break rather than a migration. So these tests do not
// assert that the answer "looks right" — they take the bytes the RUNTIME wrote
// and the bytes the OP sent and demand they are the same bytes, for a payload
// with every field of every type populated. A field this port failed to name, or
// named with a different tag, or ordered differently, shows up here as a diff.

// mounted builds the app the way the embedding host does: the telemetry ops on
// the native router, and the delegation wildcard behind them.
func mounted(t *testing.T) *zip.App {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := o11y.Mount(app); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	return app
}

// runtime installs a stand-in for the o11y runtime that answers every call with
// the success envelope the real one renders — through the SAME render.Success
// the handlers use — and reports what it wrote and what it was asked.
func runtime(t *testing.T, payload any) (wrote *[]byte, asked **http.Request) {
	t.Helper()
	var body []byte
	var req *http.Request
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(read))
		req = r.Clone(r.Context())
		req.Body = io.NopCloser(bytes.NewReader(read))

		rec := httptest.NewRecorder()
		render.Success(rec, http.StatusOK, payload)
		body = rec.Body.Bytes()
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(body)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })
	return &body, &req
}

// call runs one request through the app and returns the status and body a caller
// would receive.
func call(t *testing.T, app *zip.App, r *http.Request) (int, []byte) {
	t.Helper()
	resp, err := app.Test(r)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, got
}

// member is the request the gateway mints for a signed-in org member — the
// identity assertion these ops propagate and never mint.
func member(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.Header.Set(zip.HeaderOrg, "maxpower")
	r.Header.Set(zip.HeaderUser, "z")
	r.Header.Set(zip.HeaderUserEmail, "z@hanzo.ai")
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

// full is one event with EVERY field populated, so a dropped or renamed field
// cannot hide behind a zero value.
func full() *sentrytypes.Event {
	at := time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC)
	return &sentrytypes.Event{
		OrgID: "maxpower", ProjectID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		EventID: "e1", Timestamp: at, ReceivedAt: at.Add(time.Second),
		Level: "error", Type: "TypeError", Value: "x is not a function",
		Message: "x is not a function", Culprit: "web/handler.go",
		Fingerprint: "fp1", Platform: "go", Environment: "prod", Release: "v1.2.3",
		ServiceName: "api", Transaction: "GET /v1/thing", TraceID: "t1", SpanID: "s1",
		ServerName: "node-7", UserID: "u1", UserEmail: "user@example.com", UserIP: "203.0.113.7",
		Handled: true,
		Tags:    map[string]string{"region": "sfo", "tier": "gold"},
		Frames: []sentrytypes.Frame{
			{Function: "main.serve", File: "main.go", Line: 42, Column: 7, Own: true},
			{Function: "runtime.goexit", File: "asm.s", Line: 1650},
		},
	}
}

// A page of events is what the runtime wrote, to the byte.
func TestLogsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, map[string]any{"items": []*sentrytypes.Event{full()}})

	status, got := call(t, app, member(http.MethodGet, "/v1/sentinel/logs?project=p1&query=is+not&period=24h&limit=50", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/sentinel/logs" ||
		r.URL.Query().Get("project") != "p1" || r.URL.Query().Get("query") != "is not" ||
		r.URL.Query().Get("period") != "24h" || r.URL.Query().Get("limit") != "50" {
		t.Fatalf("runtime was asked %s?%s, want the caller's own inputs", r.URL.Path, r.URL.RawQuery)
	}
}

func TestTracesAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 30, 9, 30, 0, 0, time.UTC)
	app := mounted(t)
	wrote, asked := runtime(t, map[string]any{"items": []*sentrytypes.TraceSummary{
		{TraceID: "t1", Count: 12, FirstSeen: at, LastSeen: at.Add(time.Hour), Message: "boom"},
		{TraceID: "t2", Count: 1, FirstSeen: at, LastSeen: at},
	}})

	status, got := call(t, app, member(http.MethodGet, "/v1/sentinel/traces?project=p1&period=7d&limit=25", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/sentinel/traces" || r.URL.Query().Get("limit") != "25" {
		t.Fatalf("runtime was asked %s?%s", r.URL.Path, r.URL.RawQuery)
	}
}

// One trace's events — the shape the runtime renders is a map, whose keys Go
// marshals in sorted order. The Out has to be declared in that order or the
// bytes move, which is exactly what this pins.
func TestTraceAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, map[string]any{"traceId": "t1/2", "events": []*sentrytypes.Event{full()}})

	status, got := call(t, app, member(http.MethodGet, "/v1/sentinel/traces/t1%2F2?project=p1", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	// The trace id is a path segment, so it reaches the runtime escaped exactly
	// as the caller wrote it.
	if r := *asked; r.URL.EscapedPath() != "/v1/sentinel/traces/t1%2F2" {
		t.Fatalf("runtime was asked %q, want the id kept whole", r.URL.EscapedPath())
	}
}

func TestStatsAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	app := mounted(t)
	wrote, asked := runtime(t, map[string]any{"items": []sentrytypes.StatsPoint{
		{Time: at, Value: 9},
		{Time: at.Add(time.Hour), Value: 0},
	}})

	status, got := call(t, app, member(http.MethodGet, "/v1/sentinel/stats?project=p1&field=level&period=24h", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Query().Get("field") != "level" || r.URL.Query().Get("period") != "24h" {
		t.Fatalf("runtime was asked %s?%s", r.URL.Path, r.URL.RawQuery)
	}
}

// Discover is the one that carries a body. The body the runtime receives has to
// be the request the caller sent, field for field.
func TestDiscoverAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, &sentrytypes.DiscoverResult{
		Columns: []string{"level", "count"},
		Rows:    [][]any{{"error", 12}, {"warning", 3}},
	})

	sent := `{"project":"p1","filters":[{"field":"level","op":"eq","value":"error"}],` +
		`"aggregations":["count"],"groupBy":["level"],"period":"24h","orderBy":"count","orderDir":"desc","limit":10}`
	status, got := call(t, app, member(http.MethodPost, "/v1/sentinel/discover", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}

	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have sentrytypes.DiscoverRequest
	if err := json.Unmarshal([]byte(sent), &want); err != nil {
		t.Fatalf("unmarshal sent: %v", err)
	}
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, want), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the request.\n caller: %s\n runtime: %s", a, b)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// The caller's identity travels on as the gateway asserted it — propagated, not
// minted, and not invented when there is none.
func TestIdentityIsPropagated(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, map[string]any{"items": []*sentrytypes.Event{}})

	r := member(http.MethodGet, "/v1/sentinel/logs?project=p1", nil)
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

// A refusal keeps the runtime's status and its reason. Both shapes this face
// answers with are read: the runtime's own error envelope and the org gate's.
func TestRefusalKeepsTheRuntimeStatus(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"gate", `{"status":"error","msg":"an org-scoped principal is required"}`, "an org-scoped principal is required", http.StatusForbidden},
		{"runtime", `{"status":"error","error":{"code":"sentry_invalid_input","message":"a valid project query param is required"}}`, "a valid project query param is required", http.StatusBadRequest},
		{"notfound", `{"status":"error","error":{"code":"sentry_not_found","message":"event not found"}}`, "event not found", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := mounted(t)
			o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})))
			t.Cleanup(func() { o11y.SetRuntime(nil) })

			status, got := call(t, app, member(http.MethodGet, "/v1/sentinel/logs?project=p1", nil))
			if status != tc.status {
				t.Fatalf("status=%d, want %d (the runtime's own)", status, tc.status)
			}
			if !strings.Contains(string(got), tc.want) {
				t.Fatalf("the reason was lost: %s", got)
			}
		})
	}
}

// No runtime, no answer: the ops fail closed with the same 503 the delegation
// wildcard gives when nothing has been registered yet.
func TestTelemetryFailsClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(nil)

	for _, target := range []string{
		"/v1/sentinel/logs?project=p1",
		"/v1/sentinel/traces?project=p1",
		"/v1/sentinel/traces/t1?project=p1",
		"/v1/sentinel/stats?project=p1",
	} {
		if status, body := call(t, app, member(http.MethodGet, target, nil)); status != http.StatusServiceUnavailable {
			t.Errorf("%s: status=%d body=%s, want 503", target, status, body)
		}
	}
}

// A missing project is refused before anything is read — the same 400 the
// runtime answers, from the one place the input is now declared mandatory.
func TestProjectIsMandatory(t *testing.T) {
	app := mounted(t)
	runtime(t, map[string]any{"items": []*sentrytypes.Event{}})

	if status, body := call(t, app, member(http.MethodGet, "/v1/sentinel/logs", nil)); status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", status, body)
	}
}

// THE ROUTES, exactly as they were. Five paths, five methods; the router's own
// spelling of a parameter differs from the mux tree's, the wire path does not.
func TestTelemetryRoutesAreTheSameFive(t *testing.T) {
	app := mounted(t)
	want := map[string]bool{
		"POST /v1/sentinel/discover":  true,
		"GET /v1/sentinel/logs":       true,
		"GET /v1/sentinel/traces":     true,
		"GET /v1/sentinel/traces/:id": true,
		"GET /v1/sentinel/stats":      true,

		// The two ingest doors are NAMED hatches (mount.go mountHatches), and they
		// land in this census because it counts what is registered. They were not
		// here before for a reason worth recording: the composed binary only ever
		// wildcarded /v1/o11y/*, so these two /v1/sentinel paths reached NOTHING —
		// a Sentry SDK pointed at the clean root got a 404. Naming every route is
		// what surfaced it.
		"POST /v1/sentinel/:project/envelope/": true,
		"POST /v1/sentinel/:project/store/":    true,
	}
	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		// /v1/sentinel is shared: THIS face owns the telemetry reads (discover, logs,
		// traces, trace/:id, stats); sentryerrors.go owns projects and issues/*.
		// Counting the whole prefix made this census fail the moment
		// mountSentryErrors was wired in — the doors were always defined, just dark.
		if strings.HasPrefix(r.Path, "/v1/sentinel") &&
			!strings.Contains(r.Path, "/projects") && !strings.Contains(r.Path, "/issues") &&
			!strings.Contains(r.Path, "/events/") &&
			r.Method != http.MethodHead && r.Method != http.MethodOptions {
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

// The ingest door reaches the runtime through its OWN named route, path
// untouched, while the typed reads next to it dispatch as ops.
//
// This test used to install a host wildcard and assert the ingest paths fell
// into it. That framing hid a live defect: the composed binary's only wildcard
// was /v1/o11y/*, so nothing in the real deployment ever answered
// /v1/sentinel/<project>/envelope/ — the test passed because the test itself had
// registered the door. Naming every route removed both the wildcard and the
// illusion.
func TestTheIngestDoorIsANamedHatch(t *testing.T) {
	app := mounted(t)

	var saw string
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"e1"}`)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })

	const target = "/v1/sentinel/6ba7b810-9dad-11d1-80b4-00c04fd430c8/envelope/"
	if status, body := call(t, app, member(http.MethodPost, target, strings.NewReader("{}"))); status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if saw != target {
		t.Fatalf("runtime received %q, want %q verbatim", saw, target)
	}

	// ...and the typed reads next to it still dispatch as ops.
	runtime(t, map[string]any{"items": []*sentrytypes.Event{}})
	if status, body := call(t, app, member(http.MethodGet, "/v1/sentinel/logs?project=p1", nil)); status != http.StatusOK {
		t.Fatalf("the typed op did not answer: status=%d %s", status, body)
	}
}

// THE POINT OF THE PORT: the five reads are in the document now, each with its
// operation id, its prose and its inputs. A route behind the wildcard had none
// of that — no SDK method, no command, no agent tool, no reference page — which
// is what made a customer's own error telemetry unreachable from anything but a
// hand-written call.
func TestTelemetryReachesTheDocument(t *testing.T) {
	app := mounted(t)
	raw, err := json.Marshal(app.OpenAPISpec())
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Summary     string `json:"summary"`
			Parameters  []struct {
				Name     string `json:"name"`
				In       string `json:"in"`
				Required bool   `json:"required"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("spec: %v", err)
	}

	for path, method := range map[string]string{
		"/v1/sentinel/discover":    "post",
		"/v1/sentinel/logs":        "get",
		"/v1/sentinel/traces":      "get",
		"/v1/sentinel/traces/{id}": "get",
		"/v1/sentinel/stats":       "get",
	} {
		op, ok := spec.Paths[path][method]
		if !ok {
			t.Errorf("%s %s is not in the document", strings.ToUpper(method), path)
			continue
		}
		if op.OperationID == "" {
			t.Errorf("%s has no operation id, so nothing can name it", path)
		}
		// The summary is the FIRST SENTENCE of the handler's doc comment, lifted
		// by zipdoc. An empty one means the prose never left the source.
		if len(op.Summary) < 20 {
			t.Errorf("%s has no prose in the document: %q", path, op.Summary)
		}
		if method == "get" {
			var required bool
			for _, p := range op.Parameters {
				if p.Name == "project" && p.In == "query" && p.Required {
					required = true
				}
			}
			if !required {
				t.Errorf("%s does not declare its mandatory project input", path)
			}
		}
	}
}
