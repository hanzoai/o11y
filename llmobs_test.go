package o11y_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y"
)

// THE WIRE PROOF for the LLM-observability face — telemetry_test.go's discipline
// applied to the fourteen typed llmobs ops. The reads take the bytes the RUNTIME
// wrote (through the SAME render.Success the handlers use) and the bytes the OP
// answered and demand they are the same bytes, for a payload built from the
// face's OWN types with every field populated. The writes prove the body the
// runtime receives is the caller's own, field for field, and that a status-only
// write answers the empty 204 it always has. The routing and document proofs pin
// the whole face at once. The helpers (mounted, runtime, call, member, mustJSON)
// are telemetry_test.go's; every typed face is proved with the one harness.

// llmObsOps is the face's routing table, spelled once: the fourteen typed ops,
// their methods and their operation ids, as mountLLMObs registers them.
var llmObsOps = []struct{ Method, Path, OpID string }{
	{"GET", "/llm/observations", "ListLLMObservations"},
	{"GET", "/llm/traces", "ListLLMTraces"},
	{"GET", "/llm/sessions", "ListLLMSessions"},
	{"GET", "/llm/users", "ListLLMUsers"},
	{"GET", "/llm/scores", "ListLLMScores"},
	{"POST", "/llm/scores", "CreateLLMScore"},
	{"GET", "/llm/score/:id", "GetLLMScore"},
	{"DELETE", "/llm/score/:id", "DeleteLLMScore"},
	{"GET", "/llm/annotation", "ListLLMAnnotations"},
	{"POST", "/llm/annotation", "CreateLLMAnnotation"},
	{"GET", "/llm_pricing_rules", "ListLLMPricingRules"},
	{"PUT", "/llm_pricing_rules", "CreateOrUpdateLLMPricingRules"},
	{"GET", "/llm_pricing_rules/:id", "GetLLMPricingRule"},
	{"DELETE", "/llm_pricing_rules/:id", "DeleteLLMPricingRule"},
}

// braceLLMPath rewrites a zip route's ":seg" parameters into the OpenAPI
// document's "{seg}" form, so llmObsOps can address both the router and the spec.
func braceLLMPath(zipPath string) string {
	segs := strings.Split(zipPath, "/")
	for i, s := range segs {
		if strings.HasPrefix(s, ":") {
			segs[i] = "{" + s[1:] + "}"
		}
	}
	return strings.Join(segs, "/")
}

// THE ROUTES, exactly as the mux tree declared them: fourteen typed paths on the
// native router, each taking precedence over the delegation wildcard.
func TestLLMObsRoutesAreTheSameFourteen(t *testing.T) {
	if len(llmObsOps) != 14 {
		t.Fatalf("llmObsOps has %d entries, want 14", len(llmObsOps))
	}
	app := mounted(t)
	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		got[r.Method+" "+r.Path] = true
	}
	for _, op := range llmObsOps {
		key := op.Method + " /v1/o11y" + op.Path
		if !got[key] {
			t.Errorf("%s is not registered as a typed op", key)
		}
	}
}

// A page of observations is what the runtime wrote, to the byte — every field of
// the observation populated, so a dropped or renamed field cannot hide behind a
// zero value — and the runtime is asked at the observations collection with the
// window carried through as query params.
func TestObservationsAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC)
	app := mounted(t)
	wrote, asked := runtime(t, o11y.O11yLLMObservationsPage{
		Items: []o11y.O11yLLMObservation{{
			ID: "obs1", TraceID: "tr1", ParentID: "obs0", Type: "chat", Name: "generate",
			StartTime: at, LatencyMs: 812.5, Model: "gpt-4o", Provider: "openai",
			PromptTokens: 120, CompletionTokens: 48, TotalTokens: 168, TotalCost: 0.0031,
			SessionID: "sess1", UserID: "user1", ServiceName: "checkout", StatusCode: "OK",
		}},
		Offset: 0, Limit: 50,
	})

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/llm/observations?start=1&end=2&model=gpt-4o&limit=50", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/llm/observations" {
		t.Fatalf("runtime was asked %q, want /v1/o11y/llm/observations", r.URL.Path)
	}
	if q := (*asked).URL.Query(); q.Get("start") != "1" || q.Get("end") != "2" || q.Get("model") != "gpt-4o" || q.Get("limit") != "50" {
		t.Fatalf("the view window did not reach the runtime: %s", (*asked).URL.RawQuery)
	}
}

// A page of pricing rules is what the runtime wrote, to the byte — the richest
// type in the face, carrying audit columns, a nested per-unit cost with its
// cache sub-cost, a string slice and an optional synced_at, all populated.
func TestPricingRulesAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC)
	synced := at.Add(time.Hour)
	app := mounted(t)
	wrote, asked := runtime(t, o11y.O11yLLMPricingRulesPage{
		Items: []o11y.O11yLLMPricingRule{{
			ID: "rule1", CreatedAt: at, UpdatedAt: at, CreatedBy: "z@hanzo.ai", UpdatedBy: "z@hanzo.ai",
			OrgID: "maxpower", SourceID: "src1", Model: "gpt-4o", Provider: "openai",
			ModelPattern: []string{"gpt-4o", "gpt-4o-*"}, Unit: "per_million_tokens",
			Pricing:    o11y.O11yLLMRulePricing{Input: 2.5, Output: 10, Cache: &o11y.O11yLLMPricingCacheCosts{Mode: "additive", Read: 0.25, Write: 3.75}},
			IsOverride: true, SyncedAt: &synced, Enabled: true,
		}},
		Total: 1, Offset: 0, Limit: 20,
	})

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/llm_pricing_rules?q=gpt&isOverride=true&limit=20", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if q := (*asked).URL.Query(); q.Get("q") != "gpt" || q.Get("isOverride") != "true" || q.Get("limit") != "20" {
		t.Fatalf("the pricing filter did not reach the runtime: %s", (*asked).URL.RawQuery)
	}
}

// The score create forwards the caller's ingest through the runtime's own type,
// field for field, and answers with the runtime's 201 — the created status the
// mux registration always carried.
func TestScoreCreateForwardsTheBody(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC)
	app := mounted(t)
	wrote, asked := runtime(t, o11y.O11yLLMScore{
		ID: "sc1", CreatedAt: at, UpdatedAt: at, TraceID: "tr1", Name: "helpfulness",
		Value: 0.9, DataType: "NUMERIC", Source: "API", Timestamp: at, CreatedBy: "z@hanzo.ai",
	})

	sent := `{"traceId":"tr1","observationId":"obs1","name":"helpfulness","value":0.9,"source":"API"}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/llm/scores", strings.NewReader(sent)))
	if status != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have o11y.O11yLLMIngestScore
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

// The bulk pricing write forwards the caller's batch verbatim and answers the
// empty 204 the write has always carried — a status, no body.
func TestUpsertPricingRulesForwardsTheBodyAndIsVoid(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, map[string]string{"status": "success"})

	sent := `{"rules":[{"sourceId":"src1","modelName":"gpt-4o","provider":"openai","modelPattern":["gpt-4o"],"unit":"per_million_tokens","pricing":{"input":2.5,"output":10},"enabled":true}]}`
	status, got := call(t, app, member(http.MethodPut, "/v1/o11y/llm_pricing_rules", strings.NewReader(sent)))
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}
	if len(got) != 0 {
		t.Fatalf("a 204 must have no body, got %s", got)
	}
	if r := *asked; r.Method != http.MethodPut || r.URL.Path != "/v1/o11y/llm_pricing_rules" {
		t.Fatalf("runtime was asked %s %s, want PUT /v1/o11y/llm_pricing_rules", r.Method, r.URL.Path)
	}
	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have o11y.O11yLLMUpdatablePricingRules
	if err := json.Unmarshal([]byte(sent), &want); err != nil {
		t.Fatalf("unmarshal sent: %v", err)
	}
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, want), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the batch.\n caller: %s\n runtime: %s", a, b)
	}
}

// A score delete addresses its target with the URL, carries no body, and answers
// the empty 204 — the same status-only contract the mux registration held.
func TestScoreDeleteIsVoidAndAddressesByID(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, map[string]string{"status": "success"})

	status, got := call(t, app, member(http.MethodDelete, "/v1/o11y/llm/score/sc1", nil))
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}
	if len(got) != 0 {
		t.Fatalf("a 204 must have no body, got %s", got)
	}
	if r := *asked; r.Method != http.MethodDelete || r.URL.Path != "/v1/o11y/llm/score/sc1" {
		t.Fatalf("runtime was asked %s %s, want DELETE /v1/o11y/llm/score/sc1", r.Method, r.URL.Path)
	}
}

// No runtime, no answer: the ops fail closed with the same 503 the delegation
// wildcard gives when nothing has been registered yet.
func TestLLMObsFailsClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetHandler(nil)

	for _, tc := range []struct{ method, target string }{
		{http.MethodGet, "/v1/o11y/llm/observations"},
		{http.MethodGet, "/v1/o11y/llm/scores"},
		{http.MethodGet, "/v1/o11y/llm_pricing_rules"},
	} {
		if status, body := call(t, app, member(tc.method, tc.target, nil)); status != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status=%d body=%s, want 503", tc.method, tc.target, status, body)
		}
	}
}

// THE POINT OF THE PORT: every one of the fourteen ops is in the document now,
// each with its operation id and its prose. A route behind the wildcard had none
// of that — no SDK method, no command, no agent tool, no reference page — which
// is what made a tenant's own LLM telemetry and token pricing unreachable from
// anything but a hand-written call.
func TestLLMObsReachesTheDocument(t *testing.T) {
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
	for _, op := range llmObsOps {
		path := "/v1/o11y" + braceLLMPath(op.Path)
		method := strings.ToLower(op.Method)
		doc, ok := spec.Paths[path][method]
		if !ok {
			t.Errorf("%s %s is not in the document", op.Method, path)
			continue
		}
		if doc.OperationID != op.OpID {
			t.Errorf("%s %s has operation id %q, want %q", op.Method, path, doc.OperationID, op.OpID)
		}
		if len(doc.Summary) < 20 {
			t.Errorf("%s %s has no prose in the document: %q", op.Method, path, doc.Summary)
		}
	}
}
