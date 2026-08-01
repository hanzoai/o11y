package o11y_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/cloud"
	"github.com/hanzoai/o11y"
	"github.com/zap-proto/zip"
)

// THE WIRE PROOF for the dashboards face — telemetry_test.go's discipline applied
// to the twenty-two typed dashboard ops. The reads take the bytes the RUNTIME
// wrote (through the SAME render.Success the handlers use) and the bytes the OP
// answered and demand they are the same bytes, for a payload built from the
// face's OWN types with every field populated. The writes prove the body the
// runtime receives is the caller's own, field for field — including the PATCH's
// bare RFC 6902 array and the void writes that keep their config body. The void
// routes answer a bodyless 204, the created share answers 201, and the routing
// and document proofs pin the whole face at once.
//
// The helpers (mounted, runtime, call, member, mustJSON, bracePath) are the ones
// telemetry_test.go and identity_test.go already carry; every typed face is
// proved with the one harness.

// dashboardOps is the face's routing table, spelled once: the twenty-two typed
// ops, their methods and their operation ids, as mountDashboards registers them.
// The routing proof reads it as native Fiber routes, the document proof as
// OpenAPI paths — one source, two projections.
var dashboardOps = []struct{ Method, Path, OpID string }{
	{"GET", "/dashboards", "ListDashboardsV2"},
	{"GET", "/users/me/dashboards", "ListDashboardsForUserV2"},
	{"POST", "/dashboards", "CreateDashboardV2"},
	{"POST", "/dashboards/:id/clone", "CloneDashboardV2"},
	{"GET", "/dashboards/:id", "GetDashboardV2"},
	{"PUT", "/dashboards/:id", "UpdateDashboardV2"},
	{"PATCH", "/dashboards/:id", "PatchDashboardV2"},
	{"DELETE", "/dashboards/:id", "DeleteDashboardV2"},
	{"PUT", "/dashboards/:id/lock", "LockDashboardV2"},
	{"DELETE", "/dashboards/:id/lock", "UnlockDashboardV2"},
	{"PUT", "/users/me/dashboards/:id/pins", "PinDashboardV2"},
	{"DELETE", "/users/me/dashboards/:id/pins", "UnpinDashboardV2"},
	{"GET", "/dashboard_views", "ListDashboardViews"},
	{"POST", "/dashboard_views", "CreateDashboardView"},
	{"PUT", "/dashboard_views/:id", "UpdateDashboardView"},
	{"DELETE", "/dashboard_views/:id", "DeleteDashboardView"},
	{"POST", "/dashboards/:id/public", "CreatePublicDashboard"},
	{"GET", "/dashboards/:id/public", "GetPublicDashboard"},
	{"PUT", "/dashboards/:id/public", "UpdatePublicDashboard"},
	{"DELETE", "/dashboards/:id/public", "DeletePublicDashboard"},
	{"GET", "/public/dashboards/:id", "GetPublicDashboardData"},
	{"GET", "/public/dashboards/:id/widgets/:idx/query_range", "GetPublicDashboardWidgetQueryRange"},
}

// THE ROUTES, exactly as they were: twenty-two paths dispatch to the native
// router, each at its full /v1/o11y path with the router's own :param spelling.
func TestDashboardRoutesAreTypedOps(t *testing.T) {
	app := mounted(t)
	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		if r.Method != http.MethodHead && r.Method != http.MethodOptions {
			got[r.Method+" "+r.Path] = true
		}
	}
	for _, op := range dashboardOps {
		key := op.Method + " /v1/o11y" + op.Path
		if !got[key] {
			t.Errorf("%s is not registered as a typed op", key)
		}
	}
}

// A page of dashboards is what the runtime wrote, to the byte — each row's every
// field populated, so a dropped or renamed field cannot hide behind a zero value
// — and the runtime is asked at the collection with the caller's own filter,
// sort, order and page bounds.
func TestDashboardListAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC)
	app := mounted(t)
	tags := []o11y.O11yDashboardTag{{Key: "team", Value: "core"}, {Key: "env", Value: "prod"}}
	wrote, asked := runtime(t, o11y.O11yDashboardList{
		Dashboards: []o11y.O11yDashboardListItem{{
			ID: "d1", CreatedAt: at, UpdatedAt: at.Add(time.Hour), CreatedBy: "z", UpdatedBy: "z",
			OrgID: "maxpower", Locked: true, Source: "user", SchemaVersion: "v6", Name: "cpu-dash",
			Image: "cover.png", Tags: tags,
			Spec: o11y.O11yDashboardListSpec{Display: o11y.O11yDashboardDisplay{Name: "CPU", Description: "cpu load"}},
		}},
		Total: 1,
		Tags:  tags,
	})

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/dashboards?query=name%3Acpu&sort=name&order=asc&limit=10&offset=5", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/dashboards" ||
		r.URL.Query().Get("query") != "name:cpu" || r.URL.Query().Get("sort") != "name" ||
		r.URL.Query().Get("order") != "asc" || r.URL.Query().Get("limit") != "10" ||
		r.URL.Query().Get("offset") != "5" {
		t.Fatalf("runtime was asked %s?%s, want the caller's own inputs", r.URL.Path, r.URL.RawQuery)
	}
}

// The per-user list carries the pin state and floats pinned rows — the shape the
// runtime renders, to the byte, and the runtime is asked at the personalized
// collection.
func TestDashboardListForUserAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	app := mounted(t)
	row := o11y.O11yDashboardListItem{
		ID: "d1", CreatedAt: at, UpdatedAt: at, CreatedBy: "z", UpdatedBy: "z",
		OrgID: "maxpower", Locked: false, Source: "user", SchemaVersion: "v6", Name: "cpu-dash",
		Tags: []o11y.O11yDashboardTag{{Key: "team", Value: "core"}},
		Spec: o11y.O11yDashboardListSpec{Display: o11y.O11yDashboardDisplay{Name: "CPU"}},
	}
	wrote, asked := runtime(t, o11y.O11yDashboardListForUser{
		Dashboards: []o11y.O11yDashboardListItemForUser{{O11yDashboardListItem: row, Pinned: true}},
		Total:      1,
		Tags:       []o11y.O11yDashboardTag{{Key: "team", Value: "core"}},
	})

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/users/me/dashboards", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/users/me/dashboards" {
		t.Fatalf("runtime was asked %q, want /v1/o11y/users/me/dashboards", r.URL.Path)
	}
}

// The anonymous public read answers the sanitized dashboard and its share config,
// to the byte — the widget data carried verbatim as open JSON, so the query
// internals the runtime stripped stay exactly as it left them.
func TestPublicDashboardDataAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	app := mounted(t)
	wrote, asked := runtime(t, o11y.O11yPublicDashboardData{
		Dashboard: &o11y.O11yPublicDashboardV1{
			CreatedAt: at, UpdatedAt: at, CreatedBy: "z", UpdatedBy: "z", ID: "d1",
			Data: json.RawMessage(`{"widgets":[{"panelTypes":"graph"}]}`), Locked: false,
			OrgID: "maxpower", Source: "user",
		},
		PublicDashboard: &o11y.O11yPublicDashboard{TimeRangeEnabled: true, DefaultTimeRange: "1h", PublicPath: "/public/dashboards/share-1"},
	})

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/public/dashboards/share-1", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/public/dashboards/share-1" {
		t.Fatalf("runtime was asked %q, want the public read path", r.URL.Path)
	}
}

// The create forwards the caller's postable dashboard through the face's own
// type, field for field — the Perses spec carried verbatim — and answers with the
// runtime's 201, the created status the mux registration always carried.
func TestCreateDashboardForwardsTheBody(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, o11y.O11yDashboard{ID: "d9"})

	sent := `{"schemaVersion":"v6","generateName":true,"tags":[{"key":"team","value":"core"}],` +
		`"spec":{"display":{"name":"CPU"},"panels":{"p1":{"kind":"TimeSeriesChart"}}}}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/dashboards", strings.NewReader(sent)))
	if status != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", status, got)
	}

	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have o11y.O11yDashboardPostable
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

// The patch forwards the caller's bare RFC 6902 operations array as the runtime's
// patch decoder reads it — a bare array, not an object — and the dashboard it
// patches is the one the URL named, not one the body could smuggle.
func TestPatchForwardsTheBareOpsArray(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, o11y.O11yDashboard{ID: "d1"})

	sent := `[{"op":"replace","path":"/spec/display/name","value":"New Name"},` +
		`{"op":"remove","path":"/tags/0"},{"op":"copy","from":"/spec/panels/p1","path":"/spec/panels/p2"}]`
	status, got := call(t, app, member(http.MethodPatch, "/v1/o11y/dashboards/d1", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/dashboards/d1" {
		t.Fatalf("runtime was asked %q, want the id from the path", r.URL.Path)
	}
	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have []o11y.O11yDashboardPatchOp
	if err := json.Unmarshal([]byte(sent), &want); err != nil {
		t.Fatalf("unmarshal sent: %v", err)
	}
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read as a JSON Patch: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, want), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the patch.\n caller: %s\n runtime: %s", a, b)
	}
}

// A delete answers a bodyless 204 — the No Content the route has always given —
// and the runtime is asked to DELETE the dashboard the URL named.
func TestDeleteDashboardIs204(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, map[string]any{"status": "success"})

	status, got := call(t, app, member(http.MethodDelete, "/v1/o11y/dashboards/d1", nil))
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}
	if len(got) != 0 {
		t.Fatalf("204 carried a body: %s", got)
	}
	if r := *asked; r.Method != http.MethodDelete || r.URL.Path != "/v1/o11y/dashboards/d1" {
		t.Fatalf("runtime was asked %s %q, want DELETE /v1/o11y/dashboards/d1", r.Method, r.URL.Path)
	}
}

// The public-config write answers 204 and still forwards its config body — a void
// answer does not mean a void request — to the dashboard the URL named.
func TestUpdatePublicForwardsTheConfigAnd204s(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, map[string]any{"status": "success"})

	sent := `{"timeRangeEnabled":true,"defaultTimeRange":"30m"}`
	status, got := call(t, app, member(http.MethodPut, "/v1/o11y/dashboards/d1/public", strings.NewReader(sent)))
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}
	if len(got) != 0 {
		t.Fatalf("204 carried a body: %s", got)
	}
	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have o11y.O11yPublicDashboardWrite
	if err := json.Unmarshal([]byte(sent), &want); err != nil {
		t.Fatalf("unmarshal sent: %v", err)
	}
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, want), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the config.\n caller: %s\n runtime: %s", a, b)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/dashboards/d1/public" {
		t.Fatalf("runtime was asked %q, want /v1/o11y/dashboards/d1/public", r.URL.Path)
	}
}

// The pin is a viewer bookmark: it forwards the caller's own pin write to the
// personalized path and answers 204, the same No Content the route has given.
func TestPinDashboardIs204(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, map[string]any{"status": "success"})

	status, got := call(t, app, member(http.MethodPut, "/v1/o11y/users/me/dashboards/d1/pins", nil))
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}
	if r := *asked; r.Method != http.MethodPut || r.URL.Path != "/v1/o11y/users/me/dashboards/d1/pins" {
		t.Fatalf("runtime was asked %s %q, want the caller's own pin path", r.Method, r.URL.Path)
	}
}

// The caller's identity travels on as the gateway asserted it — propagated, not
// minted, and not invented when there is none.
func TestDashboardIdentityIsPropagated(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, o11y.O11yDashboardList{})

	r := member(http.MethodGet, "/v1/o11y/dashboards", nil)
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

// No runtime, no answer: the ops fail closed with the same 503 the delegation
// wildcard gives when nothing has been registered yet.
func TestDashboardFailsClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetHandler(nil)

	for _, tc := range []struct{ method, target string }{
		{http.MethodGet, "/v1/o11y/dashboards"},
		{http.MethodGet, "/v1/o11y/users/me/dashboards"},
		{http.MethodGet, "/v1/o11y/dashboard_views"},
		{http.MethodGet, "/v1/o11y/dashboards/d1"},
		{http.MethodDelete, "/v1/o11y/dashboards/d1"},
		{http.MethodGet, "/v1/o11y/public/dashboards/share-1"},
	} {
		if status, body := call(t, app, member(tc.method, tc.target, nil)); status != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status=%d body=%s, want 503", tc.method, tc.target, status, body)
		}
	}
}

// The typed dashboard paths take precedence over the host's /v1/o11y wildcard
// however the host ordered its mounts, and every OTHER path under the prefix — a
// deeper dashboard subpath none of these ops model — still reaches the runtime
// through that same wildcard.
func TestDashboardTypedPathsWinOverWildcard(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	// The host registers its /v1/o11y wildcard BEFORE the module mounts, which is
	// the order the composed binary uses.
	app.All("/v1/o11y/*", zip.AdaptNetHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"door":"wildcard","path":"`+r.URL.Path+`"}`)
	})))
	if err := o11y.Mount(app, cloud.Deps{}); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	runtime(t, o11y.O11yDashboardList{})

	// A deeper, unmodeled dashboard subpath still reaches the wildcard.
	if _, body := call(t, app, member(http.MethodGet, "/v1/o11y/dashboards/d1/history", nil)); !strings.Contains(string(body), `"door":"wildcard"`) {
		t.Errorf("an unmodeled dashboard subpath no longer reaches the runtime through the wildcard: %s", body)
	}
	// ...and the typed list op wins over that wildcard.
	if _, body := call(t, app, member(http.MethodGet, "/v1/o11y/dashboards", nil)); strings.Contains(string(body), `"door":"wildcard"`) {
		t.Fatalf("the typed dashboards op did not take precedence: %s", body)
	}
}

// THE POINT OF THE PORT: every one of the twenty-two ops is in the document now,
// each with its operation id and its prose. A route behind the wildcard had none
// of that — no SDK method, no command, no agent tool, no reference page — which is
// what made a customer's own dashboards unreachable from anything but a
// hand-written call.
func TestDashboardReachesTheDocument(t *testing.T) {
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

	for _, op := range dashboardOps {
		path := "/v1/o11y" + bracePath(op.Path)
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
