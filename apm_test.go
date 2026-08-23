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
	"github.com/hanzoai/o11y/pkg/query-service/app/integrations/messagingQueues/kafka"
	"github.com/hanzoai/o11y/pkg/query-service/app/integrations/messagingQueues/queues"
	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
	qbtypes "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/hanzoai/o11y/pkg/types/servicetypes/servicetypesv1"
	"github.com/hanzoai/o11y/pkg/types/thirdpartyapitypes"
	"github.com/zap-proto/zip"
)

// THE WIRE PROOF for the APM face — the service catalog, the messaging-queue
// views and the third-party API overview — by the same method telemetry_test.go
// and infra_test.go use: the bytes the RUNTIME wrote and the bytes the OP sent
// must be the same bytes, for payloads with every field populated, and the
// request the runtime receives must be the request the caller made — same path,
// same body fields. The helpers mounted/runtime/call/member/mustJSON live in
// telemetry_test.go; this file adds the APM payloads, routes and the one raw
// stand-in the two enveloped-less reads need.

// apmRuntimeRaw installs a stand-in that answers with raw bytes VERBATIM — for
// the two reads that answer without the {status,data} envelope (the service
// name catalog, the top-level-operations map), where render.Success would wrap
// bytes these ops have never wrapped. It reports what it was asked.
func apmRuntimeRaw(t *testing.T, raw string) (asked **http.Request) {
	t.Helper()
	var req *http.Request
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(read))
		req = r.Clone(r.Context())
		req.Body = io.NopCloser(bytes.NewReader(read))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, raw)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })
	return &req
}

// ── the service catalog ─────────────────────────────────────────────────────

// The service listing is the runtime's answer to the byte, and the body the
// runtime receives is the caller's own request — the legacy capitalized tag
// filter included.
func TestServicesAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, []servicetypesv1.ResponseItem{{
		ServiceName: "api", Percentile99: 1234.5, AvgDuration: 210.7,
		NumCalls: 900, CallRate: 15, NumErrors: 12, ErrorRate: 1.33,
		Num4XX: 4, FourXXRate: 0.44,
		DataWarning: servicetypesv1.DataWarning{TopLevelOps: []string{"GET /v1/thing", "POST /v1/thing"}},
	}})

	sent := `{"start":"1722400000000","end":"1722403600000",` +
		`"tags":[{"Key":"service.name","Operator":"in","StringValues":["api","web"],"NumberValues":[],"BoolValues":[],"TagType":"resource"}]}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/services", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/services" || r.Method != http.MethodPost {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
	assertBodyRoundTrips[servicetypesv1.Request](t, sent, *asked)
}

// The service name catalog answers a bare array — no envelope — and takes no
// input, so the op forwards a plain GET with no query.
func TestServiceNamesAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	raw := `["api","web","worker"]`
	asked := apmRuntimeRaw(t, raw)

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/services/list", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if string(got) != raw {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", raw, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/services/list" || r.Method != http.MethodGet || r.URL.RawQuery != "" {
		t.Fatalf("runtime was asked %s %s?%s, want a plain GET", r.Method, r.URL.Path, r.URL.RawQuery)
	}
}

// A service's operations profile is the runtime's answer, and the window and
// service the runtime receives are the caller's own.
func TestTopOperationsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, []servicetypesv1.OperationItem{
		{Name: "GET /v1/thing", P50: 100, P95: 200, P99: 300, NumCalls: 50, ErrorCount: 2},
		{Name: "POST /v1/thing", P50: 110, P95: 240, P99: 512, NumCalls: 8, ErrorCount: 0},
	})

	sent := `{"start":"1722400000000","end":"1722403600000","service":"api","tags":[],"limit":10}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/service/top_operations", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	assertBodyRoundTrips[servicetypesv1.OperationsRequest](t, sent, *asked)
}

// The entry-point operations read shares the operations request and answer, so
// one wire proof pins both it and top_operations to the runtime's shapes.
func TestEntryPointOperationsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, _ := runtime(t, []servicetypesv1.OperationItem{
		{Name: "consume orders", P50: 5, P95: 9, P99: 14, NumCalls: 4200, ErrorCount: 3},
	})

	sent := `{"start":"1","end":"2","service":"api","tags":[]}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/service/entry_point_operations", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
}

// The top-level-operations map answers bare — no envelope — and its keys are
// service names, which Go marshals in sorted order; the map Out has to
// reproduce that exactly.
func TestTopLevelOperationsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	raw := `{"api":["GET /v1/a","POST /v1/a"],"web":["GET /v1/b"]}`
	asked := apmRuntimeRaw(t, raw)

	sent := `{"service":"api","start":"1722400000000","end":"1722403600000"}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/service/top_level_operations", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if string(got) != raw {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", raw, got)
	}
	var have struct {
		Service, Start, End string
	}
	forwarded, _ := io.ReadAll((*asked).Body)
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if have.Service != "api" || have.Start != "1722400000000" || have.End != "1722403600000" {
		t.Fatalf("the op rewrote the request: %s", forwarded)
	}
}

// ── the messaging-queue surface ─────────────────────────────────────────────

// The queue overview is the runtime's list answer to the byte, and the body the
// runtime receives keeps every field — the span-attribute filter whose value
// survives the typed round trip included.
func TestQueueOverviewAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	wrote, asked := runtime(t, []*v3.Row{{
		Timestamp: at,
		Data:      map[string]interface{}{"service_name": "api", "p99": 512.5, "throughput": 1200},
	}})

	sent := `{"start":1722400000000,"end":1722403600000,` +
		`"filters":{"op":"AND","items":[{"key":{"key":"messaging.system","dataType":"string","type":"tag","isColumn":false,"isJSON":false},"value":"kafka","op":"="}]},` +
		`"limit":10}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/messaging-queues/queue-overview", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	assertBodyRoundTrips[queues.QueueListRequest](t, sent, *asked)
}

// An onboarding check is the runtime's answer to the byte — error_message key
// and all — and the window and Kafka variables reach the runtime intact.
func TestOnboardingAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, []kafka.OnboardingResponse{
		{Attribute: "messaging.system", Message: "", Status: "1"},
		{Attribute: "kind", Message: "check if your producer spans has kind=4 as attribute", Status: "0"},
	})

	sent := `{"start":1722400000000,"end":1722403600000,"variables":{"partition":"0","topic":"orders"}}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/messaging-queues/kafka/onboarding/producers", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	assertBodyRoundTrips[kafka.MessagingQueue](t, sent, *asked)
}

// The Kafka views all answer with the query-range shape, whose series values
// the runtime renders as decimal STRINGS — this pins that shape (and the eleven
// ops that share it) to the runtime's bytes, and the window and variables to
// the caller's own body.
func TestKafkaViewAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, v3.QueryRangeResponse{
		ResultType: "series",
		Result: []*v3.Result{{
			QueryName: "producer",
			Series: []*v3.Series{{
				Labels:      map[string]string{"partition": "0", "topic": "orders"},
				LabelsArray: []map[string]string{{"topic": "orders"}, {"partition": "0"}},
				Points:      []v3.Point{{Timestamp: 1722400000000, Value: 1.5}, {Timestamp: 1722400060000, Value: 2}},
			}},
		}},
	})

	sent := `{"start":1722400000000,"end":1722403600000,"variables":{"partition":"0","topic":"orders"}}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/messaging-queues/kafka/partition-latency/overview", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	assertBodyRoundTrips[kafka.MessagingQueue](t, sent, *asked)
}

// ── the third-party API overview ────────────────────────────────────────────

// A domain list is the runtime's v5 answer to the byte — the polymorphic
// results pass through as raw bytes — and the window, the show_ip flag, the
// filter expression and the group-by reach the runtime as the caller wrote them.
func TestDomainAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, &qbtypes.QueryRangeResponse{
		Type: qbtypes.RequestTypeTimeSeries,
		Data: qbtypes.QueryData{Results: []any{json.RawMessage(`{"domain":"api.stripe.com","rate":42}`)}},
		Meta: qbtypes.ExecStats{RowsScanned: 12, BytesScanned: 4096, DurationMS: 7},
	})

	sent := `{"start":1722400000000,"end":1722403600000,"show_ip":true,"domain":"api.stripe.com",` +
		`"filter":{"expression":"http.status_code >= 500"},"groupBy":[{"name":"http.url"}]}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/third-party-apis/overview/list", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	assertBodyRoundTrips[thirdpartyapitypes.ThirdPartyApiRequest](t, sent, *asked)
}

// ── census, document, identity, refusal, fail-closed ────────────────────────

// THE ROUTES, exactly the twenty-one that were behind the wildcard: five for
// the service catalog, fourteen for the messaging queues, two for the
// third-party overview. No more (the face grew no route) and no fewer (every
// route reached the document).
func TestAPMRoutesAreTheSameTwentyOne(t *testing.T) {
	app := mounted(t)
	want := map[string]bool{
		"POST /v1/o11y/services":                       true,
		"GET /v1/o11y/services/list":                   true,
		"POST /v1/o11y/service/top_operations":         true,
		"POST /v1/o11y/service/top_level_operations":   true,
		"POST /v1/o11y/service/entry_point_operations": true,

		"POST /v1/o11y/messaging-queues/queue-overview":                          true,
		"POST /v1/o11y/messaging-queues/kafka/onboarding/producers":              true,
		"POST /v1/o11y/messaging-queues/kafka/onboarding/consumers":              true,
		"POST /v1/o11y/messaging-queues/kafka/onboarding/kafka":                  true,
		"POST /v1/o11y/messaging-queues/kafka/partition-latency/overview":        true,
		"POST /v1/o11y/messaging-queues/kafka/partition-latency/consumer":        true,
		"POST /v1/o11y/messaging-queues/kafka/consumer-lag/producer-details":     true,
		"POST /v1/o11y/messaging-queues/kafka/consumer-lag/consumer-details":     true,
		"POST /v1/o11y/messaging-queues/kafka/consumer-lag/network-latency":      true,
		"POST /v1/o11y/messaging-queues/kafka/topic-throughput/producer":         true,
		"POST /v1/o11y/messaging-queues/kafka/topic-throughput/producer-details": true,
		"POST /v1/o11y/messaging-queues/kafka/topic-throughput/consumer":         true,
		"POST /v1/o11y/messaging-queues/kafka/topic-throughput/consumer-details": true,
		"POST /v1/o11y/messaging-queues/kafka/span/evaluation":                   true,

		"POST /v1/o11y/third-party-apis/overview/list":   true,
		"POST /v1/o11y/third-party-apis/overview/domain": true,
	}
	if len(want) != 21 {
		t.Fatalf("the census itself is wrong: %d", len(want))
	}

	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		if r.Method == http.MethodHead || r.Method == http.MethodOptions {
			continue
		}
		if !strings.HasPrefix(r.Path, "/v1/o11y/") || strings.HasSuffix(r.Path, "*") {
			continue
		}
		// "/service" alone also matches /service_accounts, which is the ACCESS
		// face (access.go), not APM's. Each slice census must count only its own
		// routes or every slice fails the moment another one converts — the
		// substring was safe only while APM was the sole owner of that prefix.
		// cloud_integrations owns /…/services too (integrations.go). Same lesson as
		// /service_accounts: a substring is only unambiguous while one face owns it.
		if (strings.Contains(r.Path, "/service") &&
			!strings.Contains(r.Path, "/service_accounts") &&
			!strings.Contains(r.Path, "cloud_integrations") &&
			!strings.Contains(r.Path, "cloud-integrations")) ||
			strings.Contains(r.Path, "/messaging-queues/") ||
			strings.Contains(r.Path, "/third-party-apis/") {
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
			t.Errorf("%s is registered and was not before — the face grew a route", route)
		}
	}
}

// THE POINT OF THE PORT: all twenty-one reads are in the document, each with an
// operation id and its prose. A route behind the wildcard had none of that.
func TestAPMReachesTheDocument(t *testing.T) {
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

	for path, method := range map[string]string{
		"/v1/o11y/services":                                                 "post",
		"/v1/o11y/services/list":                                            "get",
		"/v1/o11y/service/top_operations":                                   "post",
		"/v1/o11y/service/top_level_operations":                             "post",
		"/v1/o11y/service/entry_point_operations":                           "post",
		"/v1/o11y/messaging-queues/queue-overview":                          "post",
		"/v1/o11y/messaging-queues/kafka/onboarding/producers":              "post",
		"/v1/o11y/messaging-queues/kafka/onboarding/consumers":              "post",
		"/v1/o11y/messaging-queues/kafka/onboarding/kafka":                  "post",
		"/v1/o11y/messaging-queues/kafka/partition-latency/overview":        "post",
		"/v1/o11y/messaging-queues/kafka/partition-latency/consumer":        "post",
		"/v1/o11y/messaging-queues/kafka/consumer-lag/producer-details":     "post",
		"/v1/o11y/messaging-queues/kafka/consumer-lag/consumer-details":     "post",
		"/v1/o11y/messaging-queues/kafka/consumer-lag/network-latency":      "post",
		"/v1/o11y/messaging-queues/kafka/topic-throughput/producer":         "post",
		"/v1/o11y/messaging-queues/kafka/topic-throughput/producer-details": "post",
		"/v1/o11y/messaging-queues/kafka/topic-throughput/consumer":         "post",
		"/v1/o11y/messaging-queues/kafka/topic-throughput/consumer-details": "post",
		"/v1/o11y/messaging-queues/kafka/span/evaluation":                   "post",
		"/v1/o11y/third-party-apis/overview/list":                           "post",
		"/v1/o11y/third-party-apis/overview/domain":                         "post",
	} {
		op, ok := spec.Paths[path][method]
		if !ok {
			t.Errorf("%s %s is not in the document", strings.ToUpper(method), path)
			continue
		}
		if op.OperationID == "" {
			t.Errorf("%s has no operation id, so nothing can name it", path)
		}
		if len(op.Summary) < 20 {
			t.Errorf("%s has no prose in the document: %q", path, op.Summary)
		}
	}
}

// The caller's identity travels to the runtime on the APM seam exactly as it
// does on the others — propagated, never minted.
func TestAPMIdentityIsPropagated(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, []servicetypesv1.ResponseItem{})

	r := member(http.MethodPost, "/v1/o11y/services", strings.NewReader(`{"start":"1","end":"2"}`))
	r.Header.Set(zip.HeaderUserAdmin, "true")
	if status, body := call(t, app, r); status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	got := (*asked).Header
	for header, want := range map[string]string{
		zip.HeaderOrg:       "maxpower",
		zip.HeaderUser:      "z",
		zip.HeaderUserEmail: "z@hanzo.ai",
		zip.HeaderUserAdmin: "true",
	} {
		if got.Get(header) != want {
			t.Errorf("%s reached the runtime as %q, want %q", header, got.Get(header), want)
		}
	}
}

// A refusal keeps the runtime's status and reason — the query-service face's
// legacy {status, errorType, error} answer read through apmRefusal.
func TestAPMRefusalKeepsTheRuntimeStatus(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"status":"error","errorType":"forbidden","error":"API Key is not allowed"}`)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })

	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/services", strings.NewReader(`{"start":"1","end":"2"}`)))
	if status != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 (the runtime's own)", status)
	}
	if !strings.Contains(string(got), "API Key is not allowed") {
		t.Fatalf("the reason was lost: %s", got)
	}
}

// No runtime, no answer: the APM ops fail closed with the same 503 the
// delegation wildcard gives before a handler is registered.
func TestAPMFailsClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(nil)

	for _, probe := range []struct{ method, target string }{
		{http.MethodGet, "/v1/o11y/services/list"},
		{http.MethodPost, "/v1/o11y/services"},
		{http.MethodPost, "/v1/o11y/messaging-queues/queue-overview"},
		{http.MethodPost, "/v1/o11y/messaging-queues/kafka/onboarding/producers"},
		{http.MethodPost, "/v1/o11y/third-party-apis/overview/list"},
	} {
		var body io.Reader
		if probe.method == http.MethodPost {
			// An EMPTY object binds to every op's In — the service catalog reads
			// start/end as strings, the queue and domain reads as numbers, and
			// {} is the one body valid for all of them — so the request reaches
			// the relay and fails THERE. That is what this pins: no runtime, a
			// 503 from the seam, not a 400 the binder would give a typed body
			// that fits one op and not another.
			body = strings.NewReader(`{}`)
		}
		if status, got := call(t, app, member(probe.method, probe.target, body)); status != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status=%d body=%s, want 503", probe.method, probe.target, status, got)
		}
	}
}

// assertBodyRoundTrips decodes both the caller's sent JSON and the body the
// runtime received into the runtime's OWN request type and demands they are the
// same request — the op is a naming of the wire, so a field it drops, renames or
// retypes shows up here as a diff.
func assertBodyRoundTrips[T any](t *testing.T, sent string, asked *http.Request) {
	t.Helper()
	forwarded, _ := io.ReadAll(asked.Body)
	var want, have T
	if err := json.Unmarshal([]byte(sent), &want); err != nil {
		t.Fatalf("unmarshal sent: %v", err)
	}
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, want), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the request.\n caller:  %s\n runtime: %s", a, b)
	}
}
