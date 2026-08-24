package o11y_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/query-service/model"
	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
	"github.com/hanzoai/o11y/pkg/types/inframonitoringtypes"
	"github.com/zap-proto/zip"
)

// THE WIRE PROOF for the infra face, by the same method telemetry_test.go
// uses: the bytes the RUNTIME wrote and the bytes the OP sent must be the same
// bytes, for payloads with every field populated, and the request the runtime
// receives must be the request the caller made — same path, same query, same
// body fields. The helpers (mounted, runtime, call, member) live in
// telemetry_test.go; this file only adds the infra payloads and routes.

// fullHosts is one host-list answer with EVERY field populated, wrapped the
// way the query-service runtime wraps it.
func fullHosts() *model.HostListResponse {
	return &model.HostListResponse{
		Type: model.ResponseTypeList,
		Records: []model.HostListRecord{{
			HostName: "node-7", Active: true, OS: "linux",
			CPU: 0.42, Memory: 0.73, Wait: 0.01, Load15: 1.5,
			Meta: map[string]string{"os.type": "linux", "region": "sfo"},
		}},
		Total:                    1,
		SentAnyHostMetricsData:   true,
		IsSendingK8SAgentMetrics: true,
		ClusterNames:             []string{"prod"},
		NodeNames:                []string{"node-7"},
		EndTimeBeforeRetention:   false,
	}
}

// A host list is the runtime's answer, to the byte, and the body the runtime
// receives is the caller's own request, field for field — including a filter
// whose value survives the typed round trip.
func TestHostListAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, fullHosts())

	sent := `{"start":1722400000000,"end":1722403600000,` +
		`"filters":{"op":"AND","items":[{"key":{"key":"os.type","dataType":"string","type":"resource","isColumn":false,"isJSON":false},"value":"linux","op":"="}]},` +
		`"groupBy":[{"key":"k8s.cluster.name","dataType":"string","type":"resource","isColumn":false,"isJSON":false}],` +
		`"orderBy":{"columnName":"cpu","order":"desc"},"offset":0,"limit":10}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/hosts/list", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/hosts/list" || r.Method != http.MethodPost {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}

	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have model.HostListRequest
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

// The attribute discovery reads carry their six query parameters through
// verbatim and answer with the runtime's bytes.
func TestAttributeKeysAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, &v3.FilterAttributeKeyResponse{AttributeKeys: []v3.AttributeKey{
		{Key: "os.type", DataType: v3.AttributeKeyDataTypeString, Type: v3.AttributeKeyTypeResource, IsColumn: false, IsJSON: false},
		{Key: "host.name", DataType: v3.AttributeKeyDataTypeString, Type: v3.AttributeKeyTypeResource, IsColumn: true},
	}})

	status, got := call(t, app, member(http.MethodGet,
		"/v1/o11y/hosts/attribute_keys?dataSource=metrics&aggregateOperator=noop&aggregateAttribute=system_cpu_load_average_15m&searchText=os&tagType=resource&limit=25", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	q := (*asked).URL.Query()
	for name, want := range map[string]string{
		"dataSource": "metrics", "aggregateOperator": "noop",
		"aggregateAttribute": "system_cpu_load_average_15m",
		"searchText":         "os", "tagType": "resource", "limit": "25",
	} {
		if q.Get(name) != want {
			t.Errorf("runtime was asked %s=%q, want %q", name, q.Get(name), want)
		}
	}
}

// The attribute values read adds the key being expanded and its data type.
func TestAttributeValuesAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, &v3.FilterAttributeValueResponse{
		StringAttributeValues: []string{"linux", "darwin"},
		NumberAttributeValues: []any{1, 2.5},
		BoolAttributeValues:   []bool{true},
	})

	status, got := call(t, app, member(http.MethodGet,
		"/v1/o11y/pods/attribute_values?dataSource=metrics&aggregateOperator=noop&attributeKey=os.type&filterAttributeKeyDataType=string&searchText=lin", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	q := (*asked).URL.Query()
	if q.Get("attributeKey") != "os.type" || q.Get("filterAttributeKeyDataType") != "string" {
		t.Fatalf("runtime was asked ?%s, want the caller's own inputs", (*asked).URL.RawQuery)
	}
}

// An infra_monitoring rollup answers with the runtime's bytes, and the body
// the runtime receives keeps every field — including the hosts filter, whose
// embedded expression must survive the typed round trip exactly (it is the
// one inlined shape on this face).
func TestInfraMonitoringHostsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, &inframonitoringtypes.Hosts{
		Type: inframonitoringtypes.ResponseTypeList,
		Records: []inframonitoringtypes.HostRecord{{
			HostName: "node-7", Status: inframonitoringtypes.HostStatusActive,
			ActiveHostCount: 1, InactiveHostCount: 0,
			CPU: 0.42, Memory: 0.73, Wait: 0.01, Load15: 1.5, DiskUsage: 0.61,
			Meta: map[string]string{"os.type": "linux"},
		}},
		Total:                  1,
		EndTimeBeforeRetention: false,
	})

	sent := `{"start":1722400000000,"end":1722403600000,` +
		`"filter":{"expression":"os.type = 'linux'","filterByStatus":"active"},` +
		`"groupBy":[{"name":"k8s.cluster.name"}],` +
		`"orderBy":{"key":{"name":"cpu"},"direction":"desc"},"offset":0,"limit":10}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/infra_monitoring/hosts", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}

	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have inframonitoringtypes.PostableHosts
	if err := json.Unmarshal([]byte(sent), &want); err != nil {
		t.Fatalf("unmarshal sent: %v", err)
	}
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, want), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the request.\n caller:  %s\n runtime: %s", a, b)
	}
	if have.Filter == nil || have.Filter.Expression != "os.type = 'linux'" {
		t.Fatalf("the inlined filter expression did not survive: %+v", have.Filter)
	}
}

// A rollup request the runtime would refuse is refused at the op with the
// SAME 400 — the Postable types validate on decode, so the judgment moved a
// layer out but neither the status nor the reason changed class.
func TestInfraMonitoringValidatesLikeTheRuntime(t *testing.T) {
	app := mounted(t)
	runtime(t, &inframonitoringtypes.Hosts{})

	// limit outside 1..5000, the shape PostableHosts.Validate refuses.
	sent := `{"start":1722400000000,"end":1722403600000,"limit":0}`
	status, body := call(t, app, member(http.MethodPost, "/v1/o11y/infra_monitoring/hosts", strings.NewReader(sent)))
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", status, body)
	}
	if !strings.Contains(string(body), "limit must be between 1 and 5000") {
		t.Fatalf("the runtime's own reason was lost: %s", body)
	}
}

// The setup checks read carries its one selector through and answers with the
// runtime's bytes.
func TestChecksAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, &inframonitoringtypes.Checks{
		Type:  inframonitoringtypes.CheckTypeHosts,
		Ready: true,
		PresentDefaultEnabledMetrics: []inframonitoringtypes.MetricsComponentEntry{{
			Metrics: []string{"system.cpu.load_average.15m"},
			AssociatedComponent: inframonitoringtypes.AssociatedComponent{
				Type: inframonitoringtypes.CheckComponentTypeReceiver, Name: "hostmetricsreceiver",
			},
		}},
		PresentOptionalMetrics:       []inframonitoringtypes.MetricsComponentEntry{},
		PresentRequiredAttributes:    []inframonitoringtypes.AttributesComponentEntry{},
		MissingDefaultEnabledMetrics: []inframonitoringtypes.MissingMetricsComponentEntry{},
		MissingOptionalMetrics:       []inframonitoringtypes.MissingMetricsComponentEntry{},
		MissingRequiredAttributes:    []inframonitoringtypes.MissingAttributesComponentEntry{},
	})

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/infra_monitoring/checks?type=hosts", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if q := (*asked).URL.Query(); q.Get("type") != "hosts" {
		t.Fatalf("runtime was asked ?%s", (*asked).URL.RawQuery)
	}
}

// The checks selector is mandatory, as the runtime has always held.
func TestChecksTypeIsMandatory(t *testing.T) {
	app := mounted(t)
	runtime(t, &inframonitoringtypes.Checks{})

	if status, body := call(t, app, member(http.MethodGet, "/v1/o11y/infra_monitoring/checks", nil)); status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", status, body)
	}
}

// No runtime, no answer: the infra ops fail closed with the same 503 the
// delegation wildcard gives before a handler is registered.
func TestInfraFailsClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(nil)

	for _, probe := range []struct{ method, target string }{
		{http.MethodGet, "/v1/o11y/hosts/attribute_keys?dataSource=metrics"},
		{http.MethodPost, "/v1/o11y/pods/list"},
		{http.MethodPost, "/v1/o11y/infra_monitoring/nodes"},
		{http.MethodGet, "/v1/o11y/infra_monitoring/checks?type=hosts"},
	} {
		var body io.Reader
		if probe.method == http.MethodPost {
			body = strings.NewReader(`{"start":1,"end":2,"limit":10}`)
		}
		if status, got := call(t, app, member(probe.method, probe.target, body)); status != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status=%d body=%s, want 503", probe.method, probe.target, status, got)
		}
	}
}

// THE ROUTES, exactly the forty-four that were behind the wildcard: three per
// resource face, ten rollups, one checks read. No more (the face grew no
// route) and no fewer (every route reached the document).
func TestInfraRoutesAreTheSameFortyFour(t *testing.T) {
	app := mounted(t)
	want := map[string]bool{}
	for _, r := range []string{
		"hosts", "processes", "pods", "pvcs", "nodes", "namespaces",
		"clusters", "deployments", "daemonsets", "statefulsets", "jobs",
	} {
		want["GET /v1/o11y/"+r+"/attribute_keys"] = true
		want["GET /v1/o11y/"+r+"/attribute_values"] = true
		want["POST /v1/o11y/"+r+"/list"] = true
	}
	for _, r := range []string{
		"hosts", "pods", "nodes", "namespaces", "clusters", "pvcs",
		"deployments", "statefulsets", "jobs", "daemonsets",
	} {
		want["POST /v1/o11y/infra_monitoring/"+r] = true
	}
	want["GET /v1/o11y/infra_monitoring/checks"] = true
	if len(want) != 44 {
		t.Fatalf("the census itself is wrong: %d", len(want))
	}

	// The /list heuristic must stay inside infra's OWN resources: other faces
	// carry /list routes too (the service catalog's /services/list, the
	// third-party overview's .../overview/list), and a bare "/list" suffix
	// would miscount those as infra routes. Scope it to the eleven resources the
	// want-set is built from, so any other face's /list route is ignored here.
	infraList := map[string]bool{}
	for _, r := range []string{
		"hosts", "processes", "pods", "pvcs", "nodes", "namespaces",
		"clusters", "deployments", "daemonsets", "statefulsets", "jobs",
	} {
		infraList["/v1/o11y/"+r+"/list"] = true
	}
	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		if r.Method == http.MethodHead || r.Method == http.MethodOptions {
			continue
		}
		if strings.HasPrefix(r.Path, "/v1/o11y/") && !strings.HasSuffix(r.Path, "*") &&
			// query-core's autocomplete/auto_complete routes carry attribute_* in
			// their paths too, and they are NOT infra routes — without this exclusion
			// the census counts them and the 44-route assertion drifts.
			!strings.Contains(r.Path, "auto") &&
			// "/list" alone also matches /services/list (apm face) and
			// /third-party-apis/overview/list (apm face). Each slice census counts
			// only its OWN routes — a bare suffix was safe only while infra was the
			// sole owner of it. The explicit infraList literal is the real anchor.
			!strings.HasPrefix(r.Path, "/v1/o11y/services") &&
			!strings.Contains(r.Path, "third-party-apis") &&
			// trace-funnels owns a /list too (tracefunnels.go). Same lesson the
			// APM census learned about /service: a suffix is only unambiguous
			// while one face owns it.
			!strings.HasPrefix(r.Path, "/v1/o11y/trace-funnels") &&
			(strings.Contains(r.Path, "attribute_") || infraList[r.Path] ||
				strings.HasSuffix(r.Path, "/list") || strings.Contains(r.Path, "infra_monitoring")) {
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

// A path no slice names is NOT served — there is no wildcard behind these ops
// any more — while a typed path reaches the runtime with the path untouched.
// Both halves in one test: the interesting failure is a typed op that quietly
// stops dispatching and gets answered by something else.
func TestUnnamedPathsAreNotServedButTypedOnesReachTheRuntime(t *testing.T) {
	app := mounted(t)
	var askedPath string
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		askedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"endpoint":"runtime","path":"`+r.URL.Path+`"}}`)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })

	// A path no slice names: 404, and the runtime is never asked. Under the old
	// catch-all this same request was answered, which is exactly why an unmounted
	// slice could go unnoticed for a release.
	askedPath = ""
	if status, body := call(t, app, member(http.MethodGet, "/v1/o11y/never-a-typed-op/at-all", nil)); status != http.StatusNotFound {
		t.Fatalf("status=%d want 404: %s", status, body)
	}
	if askedPath != "" {
		t.Fatalf("an unnamed path still reached the runtime as %q", askedPath)
	}
	// A typed path: the op dispatches, and what reaches the runtime is the
	// SAME path — the op is a naming of the route, not a move of it.
	if status, body := call(t, app, member(http.MethodGet, "/v1/o11y/hosts/attribute_keys?dataSource=metrics", nil)); status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if askedPath != "/v1/o11y/hosts/attribute_keys" {
		t.Fatalf("the typed op asked the runtime for %q, want the caller's own path", askedPath)
	}
}

// THE POINT OF THE PORT: all forty-four reads are in the document, each with
// an operation id and its prose.
func TestInfraReachesTheDocument(t *testing.T) {
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

	checks := map[string]string{
		"/v1/o11y/hosts/attribute_keys":      "get",
		"/v1/o11y/hosts/attribute_values":    "get",
		"/v1/o11y/hosts/list":                "post",
		"/v1/o11y/statefulsets/list":         "post",
		"/v1/o11y/infra_monitoring/hosts":    "post",
		"/v1/o11y/infra_monitoring/pvcs":     "post",
		"/v1/o11y/infra_monitoring/checks":   "get",
		"/v1/o11y/daemonsets/attribute_keys": "get",
	}
	for path, method := range checks {
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
	// The whole face, counted: 44 infra operations under the prefix.
	count := 0
	for path, methods := range spec.Paths {
		if strings.HasPrefix(path, "/v1/o11y/") {
			for range methods {
				count++
			}
		}
	}
	if count < 44 {
		t.Errorf("only %d /v1/o11y operations reached the document, want at least the 44 infra reads", count)
	}
}

// The identity assertion travels to the runtime on the infra seam exactly as
// it does on the telemetry seam — propagated, never minted.
func TestInfraIdentityIsPropagated(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, &v3.FilterAttributeKeyResponse{})

	r := member(http.MethodGet, "/v1/o11y/hosts/attribute_keys?dataSource=metrics", nil)
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

// A refusal keeps the runtime's status and reason on this face too — the mux
// half's legacy {status, errorType, error} answer included.
func TestInfraRefusalKeepsTheRuntimeStatus(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"status":"error","errorType":"forbidden","error":"API Key is not allowed"}`)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })

	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/hosts/list", strings.NewReader(`{"start":1,"end":2}`)))
	if status != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 (the runtime's own)", status)
	}
	if !strings.Contains(string(got), "API Key is not allowed") {
		t.Fatalf("the reason was lost: %s", got)
	}
}
