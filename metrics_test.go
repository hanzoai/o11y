package o11y_test

// THE WIRE PROOF for the metrics face.
//
// The nineteen metrics ops are typed now, and a typed op that changes what a
// caller receives — or what the runtime is asked — is a break rather than a
// migration. So these tests do not assert that an answer "looks right": they
// pin the request the runtime is handed (its method, path, query, body and the
// caller's propagated identity), the status a caller gets back (the reads 200,
// the create 201, the delete 204, a refusal the runtime's own), and that the
// data the op re-emits is the data the runtime wrote, field for field. They
// also pin the two halves of the delegation contract — the nineteen typed
// paths win over the wildcard, and every other path under the prefix still
// reaches the runtime through it — and the point of the whole port: that the
// nineteen ops are in the document, each with its operation id and its prose.
//
// The shared harness (mounted, runtime, call, member, mustJSON) lives in
// telemetry_test.go; this file adds only what the metrics face needs.

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/types/metricreductionruletypes"
	"github.com/zap-proto/zip"
)

// norm canonicalises JSON so a comparison is about the DATA, not the byte order
// two encoders happened to choose: the runtime's answer travels through the op's
// typed Out and back out again, and a field the Out dropped or renamed moves the
// canonical form, while a field merely reordered does not.
func norm(t *testing.T, b []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("not JSON: %v (%s)", err, b)
	}
	return mustJSON(t, v)
}

// ── the request the runtime is handed ──────────────────────────────────────────

// A read carries the caller's query onto the runtime verbatim.
func TestListMetricsForwardsTheQuery(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, map[string]any{"metrics": []any{}})

	status, body := call(t, app, member(http.MethodGet,
		"/v1/o11y/metrics?start=1700000000000&end=1700003600000&limit=200&searchText=http&source=otel", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, body)
	}
	r := *asked
	if r.URL.Path != "/v1/o11y/metrics" {
		t.Fatalf("runtime asked %q, want /v1/o11y/metrics", r.URL.Path)
	}
	for k, want := range map[string]string{
		"start": "1700000000000", "end": "1700003600000",
		"limit": "200", "searchText": "http", "source": "otel",
	} {
		if got := r.URL.Query().Get(k); got != want {
			t.Errorf("runtime asked %s=%q, want %q", k, got, want)
		}
	}
}

// A body op hands the runtime the caller's body, field for field.
func TestMetricStatsForwardsTheBody(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, map[string]any{"metrics": []any{}, "total": 0})

	sent := `{"filter":{"expression":"service.name = 'api'"},"start":1700000000000,` +
		`"end":1700003600000,"limit":50,"offset":10,"orderBy":{"key":{"name":"samples"},"direction":"desc"}}`
	status, body := call(t, app, member(http.MethodPost, "/v1/o11y/metrics/stats", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, body)
	}
	forwarded, _ := io.ReadAll((*asked).Body)
	if norm(t, []byte(sent)) != norm(t, forwarded) {
		t.Fatalf("the op rewrote the request.\n caller:  %s\n runtime: %s", sent, forwarded)
	}
}

// A parameterised read carries the id onto the runtime as the segment the router
// matched — not re-encoded, not moved into the body.
func TestReductionRuleByIDForwardsTheID(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, map[string]any{"id": "rid-42"})

	status, body := call(t, app, member(http.MethodGet, "/v1/o11y/metric_reduction_rules/rid-42", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, body)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/metric_reduction_rules/rid-42" {
		t.Fatalf("runtime asked %q, want the id kept whole", r.URL.Path)
	}
}

// ── the data the op re-emits ────────────────────────────────────────────────────

// The stats read answers the runtime's numbers, field for field, inside the
// success envelope the face has always used.
func TestReductionRuleStatsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, _ := runtime(t, metricreductionruletypes.GettableReductionRuleStats{
		IngestedSeries: 100000, RetainedSeries: 25000,
		IngestedSamples: 5000000, RetainedSamples: 1200000,
		EstimatedMonthlySavingsUsd: 42.5,
	})

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/metric_reduction_rules/stats", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if norm(t, got) != norm(t, *wrote) {
		t.Fatalf("the op changed the numbers.\n runtime: %s\n op:      %s", *wrote, got)
	}
}

// The create is the one that answers 201: it hands the runtime the caller's rule
// and gives back the created rule under the op's declared status.
func TestCreateReductionRuleAnswers201(t *testing.T) {
	app := mounted(t)
	// The runtime answers a WHOLE rule — every required field of
	// GettableReductionRule, none omitempty — so the op that re-emits it must
	// carry every one back. A partial answer here would only prove the op keeps
	// the fields the stand-in happened to send.
	wrote, asked := runtime(t, map[string]any{
		"id":              "rid-1",
		"createdAt":       "2026-07-31T12:00:00Z",
		"updatedAt":       "2026-07-31T12:05:00Z",
		"createdBy":       "z",
		"updatedBy":       "z",
		"metricName":      "http.server.duration",
		"matchType":       "drop",
		"labels":          []string{"host", "pod"},
		"effectiveFrom":   "2026-07-31T12:00:00Z",
		"active":          true,
		"ingestedSeries":  100000,
		"retainedSeries":  25000,
		"ingestedSamples": 5000000,
		"retainedSamples": 1200000,
	})

	sent := `{"metricName":"http.server.duration","matchType":"drop","labels":["host","pod"]}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/metric_reduction_rules", strings.NewReader(sent)))
	if status != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", status, got)
	}
	if forwarded, _ := io.ReadAll((*asked).Body); norm(t, []byte(sent)) != norm(t, forwarded) {
		t.Fatalf("the op rewrote the create.\n caller:  %s\n runtime: %s", sent, forwarded)
	}
	if norm(t, got) != norm(t, *wrote) {
		t.Fatalf("the op changed the created rule.\n runtime: %s\n op: %s", *wrote, got)
	}
}

// The delete carries no body and answers 204 — the status zip gives an op whose
// handler returns a nil Out, which is the status the mux registration declared.
func TestDeleteReductionRuleAnswers204(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, nil)

	status, got := call(t, app, member(http.MethodDelete, "/v1/o11y/metric_reduction_rules/rid-9", nil))
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}
	if r := *asked; r.Method != http.MethodDelete || r.URL.Path != "/v1/o11y/metric_reduction_rules/rid-9" {
		t.Fatalf("runtime asked %s %s, want DELETE the id", r.Method, r.URL.Path)
	}
}

// ── identity, refusal, fail-closed ──────────────────────────────────────────────

// The caller's identity travels on as the gateway asserted it — propagated, not
// minted at this hop.
func TestMetricsIdentityIsPropagated(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, map[string]any{"metrics": []any{}})

	r := member(http.MethodGet, "/v1/o11y/metrics?start=1&end=2", nil)
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

// An admin-only mutation the runtime refuses keeps the runtime's status and its
// reason: the gate is the runtime's, and this op is a naming of the wire, not a
// second policy.
func TestMetricsRefusalKeepsTheRuntimeStatus(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"status":"error","error":{"code":"forbidden","message":"admin access is required"}}`)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })

	sent := `{"metricName":"m","matchType":"drop","labels":["host"]}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/metric_reduction_rules", strings.NewReader(sent)))
	if status != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 (the runtime's own)", status)
	}
	if !strings.Contains(string(got), "admin access is required") {
		t.Fatalf("the reason was lost: %s", got)
	}
}

// No runtime, no answer: the ops fail closed with the same 503 the delegation
// wildcard gives when nothing has been registered yet.
func TestMetricsFailClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(nil)

	for _, tc := range []struct {
		method, target, body string
	}{
		{http.MethodGet, "/v1/o11y/metrics?start=1&end=2", ""},
		// A well-formed body: the point under test is the nil-runtime guard, which
		// sits AFTER the typed op's own required-field validation — so the body has
		// to satisfy that validation to reach the guard and prove it answers 503.
		{http.MethodPost, "/v1/o11y/metrics/stats", `{"start":1,"end":2,"limit":1}`},
		{http.MethodGet, "/v1/o11y/metric_reduction_rules", ""},
		{http.MethodGet, "/v1/o11y/metric_reduction_rules/rid-1", ""},
		{http.MethodDelete, "/v1/o11y/metric_reduction_rules/rid-1", ""},
	} {
		var body io.Reader
		if tc.body != "" {
			body = strings.NewReader(tc.body)
		}
		if status, got := call(t, app, member(tc.method, tc.target, body)); status != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status=%d body=%s, want 503", tc.method, tc.target, status, got)
		}
	}
}

// ── the routes, and the wildcard behind them ────────────────────────────────────

// THE ROUTES, exactly the nineteen. The router's own spelling of a path
// parameter (:id) differs from the mux tree's ({id}); the wire path does not.
func TestMetricsRoutesAreTheSameNineteen(t *testing.T) {
	app := mounted(t)
	want := map[string]bool{
		"GET /v1/o11y/metrics":                           true,
		"POST /v1/o11y/metrics/stats":                    true,
		"POST /v1/o11y/metrics/treemap":                  true,
		"GET /v1/o11y/metrics/attributes":                true,
		"GET /v1/o11y/metrics/metadata":                  true,
		"POST /v1/o11y/metrics/metadata":                 true,
		"GET /v1/o11y/metrics/highlights":                true,
		"GET /v1/o11y/metrics/alerts":                    true,
		"GET /v1/o11y/metrics/dashboards":                true,
		"POST /v1/o11y/metrics/inspect":                  true,
		"GET /v1/o11y/metrics/onboarding":                true,
		"GET /v1/o11y/metric_reduction_rules":            true,
		"POST /v1/o11y/metric_reduction_rules":           true,
		"GET /v1/o11y/metric_reduction_rules/stats":      true,
		"GET /v1/o11y/metric_reduction_rules/timeseries": true,
		"POST /v1/o11y/metric_reduction_rules/preview":   true,
		"GET /v1/o11y/metric_reduction_rules/:id":        true,
		"PUT /v1/o11y/metric_reduction_rules/:id":        true,
		"DELETE /v1/o11y/metric_reduction_rules/:id":     true,
	}
	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		if !strings.HasPrefix(r.Path, "/v1/o11y/metric") || strings.Contains(r.Path, "*") {
			continue
		}
		if r.Method == http.MethodHead || r.Method == http.MethodOptions {
			continue
		}
		got[r.Method+" "+r.Path] = true
	}
	for route := range want {
		if !got[route] {
			t.Errorf("%s is not registered", route)
		}
	}
	for route := range got {
		if !want[route] {
			// platform.go owns the OLDER /metric/metric_metadata route (legacy
			// path, service-scoped input) — a different op from this face's
			// /metrics/metadata. Another slice's route is not this face growing one.
			if route == "GET /v1/o11y/metric/metric_metadata" {
				continue
			}
			t.Errorf("%s is registered and was not declared — the face grew a route", route)
		}
	}
}

// The rest of the metrics face is untouched: a path with no typed op still
// reaches the runtime through the host's wildcard, and the typed paths win over
// that wildcard however the host ordered its mounts.
func TestMetricsRestStillReachesTheWildcard(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	// The host registers its /v1/o11y wildcard BEFORE the module mounts, which is
	// the order the composed binary uses.
	app.All("/v1/o11y/*", zip.AdaptNetHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"endpoint":"wildcard","path":"`+r.URL.Path+`"}`)
	})))
	if err := o11y.Mount(app); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	runtime(t, map[string]any{"metrics": []any{}})

	// A metrics path with no typed op keeps reaching the runtime.
	if _, body := call(t, app, member(http.MethodGet, "/v1/o11y/metrics/samples", nil)); !strings.Contains(string(body), `"endpoint":"wildcard"`) {
		t.Errorf("an untyped metrics path no longer reaches the runtime: %s", body)
	}
	// ...and a typed path wins over the wildcard.
	if _, body := call(t, app, member(http.MethodGet, "/v1/o11y/metrics?start=1&end=2", nil)); strings.Contains(string(body), `"endpoint":"wildcard"`) {
		t.Fatalf("the typed op did not take precedence: %s", body)
	}
}

// THE POINT OF THE PORT: the nineteen ops are in the document now, each with its
// operation id and its prose. A route behind the wildcard had none of that — no
// SDK method, no command, no agent tool, no reference page.
func TestMetricsReachTheDocument(t *testing.T) {
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

	// path+method → the operation id the mux registration declared and this port
	// carries forward.
	for _, tc := range []struct{ method, path, id string }{
		{"get", "/v1/o11y/metrics", "ListMetrics"},
		{"post", "/v1/o11y/metrics/stats", "GetMetricsStats"},
		{"post", "/v1/o11y/metrics/treemap", "GetMetricsTreemap"},
		{"get", "/v1/o11y/metrics/attributes", "GetMetricAttributes"},
		{"get", "/v1/o11y/metrics/metadata", "GetMetricMetadata"},
		{"post", "/v1/o11y/metrics/metadata", "UpdateMetricMetadata"},
		{"get", "/v1/o11y/metrics/highlights", "GetMetricHighlights"},
		{"get", "/v1/o11y/metrics/alerts", "GetMetricAlerts"},
		{"get", "/v1/o11y/metrics/dashboards", "GetMetricDashboardsV2"},
		{"post", "/v1/o11y/metrics/inspect", "InspectMetrics"},
		{"get", "/v1/o11y/metrics/onboarding", "GetMetricsOnboardingStatus"},
		{"get", "/v1/o11y/metric_reduction_rules", "ListMetricReductionRules"},
		{"post", "/v1/o11y/metric_reduction_rules", "CreateMetricReductionRule"},
		{"get", "/v1/o11y/metric_reduction_rules/stats", "GetMetricReductionRuleStats"},
		{"get", "/v1/o11y/metric_reduction_rules/timeseries", "GetMetricReductionRuleTimeseries"},
		{"post", "/v1/o11y/metric_reduction_rules/preview", "PreviewMetricReductionRule"},
		{"get", "/v1/o11y/metric_reduction_rules/{id}", "GetMetricReductionRuleByID"},
		{"put", "/v1/o11y/metric_reduction_rules/{id}", "UpdateMetricReductionRuleByID"},
		{"delete", "/v1/o11y/metric_reduction_rules/{id}", "DeleteMetricReductionRuleByID"},
	} {
		op, ok := spec.Paths[tc.path][tc.method]
		if !ok {
			t.Errorf("%s %s is not in the document", strings.ToUpper(tc.method), tc.path)
			continue
		}
		if op.OperationID != tc.id {
			t.Errorf("%s %s has operation id %q, want %q", strings.ToUpper(tc.method), tc.path, op.OperationID, tc.id)
		}
		if len(op.Summary) < 20 {
			t.Errorf("%s %s has no prose in the document: %q", strings.ToUpper(tc.method), tc.path, op.Summary)
		}
	}
}
