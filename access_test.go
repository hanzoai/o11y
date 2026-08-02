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
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/zap-proto/zip"
)

// THE WIRE PROOF, for the access-control face.
//
// The twenty roles / service-account / authz routes are typed ops now, and a
// typed op that changes what a caller receives is a break rather than a
// migration. So these tests do not assert the answer "looks right" — they take
// the bytes the RUNTIME wrote and the bytes the OP sent and demand they are the
// same bytes, they demand the twenty routes are exactly the twenty that were
// there, and they demand the operations reached the document that a route
// behind the wildcard never could. The harness (mounted, runtime, call, member)
// is telemetry_test.go's; this file adds only what the access face needs.

// answers installs a stand-in for the o11y runtime that replies with a fixed
// status and body — for the create/delete/refusal shapes render.Success's
// always-200 stand-in cannot express — and reports the request it was handed,
// so a test can prove the caller's path and body reached it unrewritten.
func answers(t *testing.T, status int, body string) **http.Request {
	t.Helper()
	var req *http.Request
	o11y.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(read))
		req = r.Clone(r.Context())
		req.Body = io.NopCloser(bytes.NewReader(read))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != "" {
			_, _ = io.WriteString(w, body)
		}
	}))
	t.Cleanup(func() { o11y.SetHandler(nil) })
	return &req
}

// A page of roles is what the runtime wrote, to the byte — every field of the
// role type populated so a dropped or renamed one cannot hide behind a zero
// value. The op's O11yRole is a NAMING of authtypes.Role, and this is what pins
// the two to the same bytes.
func TestAccessRolesListIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	app := mounted(t)
	role := &authtypes.Role{
		Identifiable:  types.Identifiable{ID: valuer.MustNewUUID("11111111-1111-4111-8111-111111111111")},
		TimeAuditable: types.TimeAuditable{CreatedAt: at, UpdatedAt: at.Add(time.Hour)},
		Name:          "admin",
		Description:   "full access to everything",
		Type:          valuer.NewString("managed"),
		OrgID:         valuer.MustNewUUID("22222222-2222-4222-8222-222222222222"),
	}
	wrote, asked := runtime(t, []*authtypes.Role{role})

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/roles", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/roles" || r.Method != http.MethodGet {
		t.Fatalf("runtime was asked %s %s, want GET /v1/o11y/roles", r.Method, r.URL.Path)
	}
}

// A create answers 201 with the new id, and the create body reaches the runtime
// as the caller wrote it — the status the mux route always chose, carried by the
// op's declared WithStatus, and the request forwarded whole.
func TestAccessCreateReturns201(t *testing.T) {
	app := mounted(t)
	envelope := `{"status":"success","data":{"id":"11111111-1111-4111-8111-111111111111"}}`
	asked := answers(t, http.StatusCreated, envelope)

	sent := `{"name":"reader","description":"read only","transactionGroups":[]}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/roles", strings.NewReader(sent)))
	if status != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", status, got)
	}
	if string(got) != envelope {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", envelope, got)
	}
	forwarded, _ := io.ReadAll((*asked).Body)
	if !strings.Contains(string(forwarded), `"name":"reader"`) {
		t.Fatalf("the runtime was sent %s, want the caller's create body", forwarded)
	}
}

// A delete answers 204 with no content — the same bytes (none) a caller has
// always received through the wildcard — and the id reaches the runtime on the
// path.
func TestAccessDeleteReturns204(t *testing.T) {
	app := mounted(t)
	asked := answers(t, http.StatusNoContent, "")

	status, got := call(t, app, member(http.MethodDelete, "/v1/o11y/roles/11111111-1111-4111-8111-111111111111", nil))
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}
	if len(got) != 0 {
		t.Fatalf("a 204 carried a body: %s", got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/roles/11111111-1111-4111-8111-111111111111" {
		t.Fatalf("runtime was asked %s, want the id on the path", r.URL.Path)
	}
}

// The caller's identity travels on as the gateway asserted it — propagated, not
// minted.
func TestAccessIdentityIsPropagated(t *testing.T) {
	app := mounted(t)
	asked := answers(t, http.StatusOK, `{"status":"success","data":[]}`)

	r := member(http.MethodGet, "/v1/o11y/roles", nil)
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

// A refusal keeps the runtime's status and its reason — both shapes this face
// answers with, the org gate's {msg} and the runtime's own error envelope.
func TestAccessRefusalKeepsTheRuntimeStatus(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"gate", `{"status":"error","msg":"an org-scoped principal is required"}`, "an org-scoped principal is required", http.StatusForbidden},
		{"notfound", `{"status":"error","error":{"code":"role_not_found","message":"role not found"}}`, "role not found", http.StatusNotFound},
		{"conflict", `{"status":"error","error":{"code":"role_conflict","message":"a role with this name already exists"}}`, "already exists", http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := mounted(t)
			answers(t, tc.status, tc.body)

			status, got := call(t, app, member(http.MethodGet, "/v1/o11y/roles", nil))
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
func TestAccessFailsClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetHandler(nil)

	for _, tc := range []struct {
		method, target, body string
	}{
		{http.MethodGet, "/v1/o11y/roles", ""},
		{http.MethodPost, "/v1/o11y/roles", `{"name":"x"}`},
		{http.MethodGet, "/v1/o11y/service_accounts", ""},
		{http.MethodGet, "/v1/o11y/service_accounts/me", ""},
		{http.MethodPost, "/v1/o11y/authz/check", `[]`},
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

// THE ROUTES, exactly as they were. Twenty paths across the three files; the
// router's own spelling of a parameter differs from the mux tree's, the wire
// path does not, and no door was added or dropped.
func TestAccessRoutesAreTheSameTwenty(t *testing.T) {
	app := mounted(t)
	want := map[string]bool{
		"POST /v1/o11y/roles":                             true,
		"GET /v1/o11y/roles":                              true,
		"GET /v1/o11y/roles/:id":                          true,
		"PUT /v1/o11y/roles/:id":                          true,
		"DELETE /v1/o11y/roles/:id":                       true,
		"POST /v1/o11y/service_accounts":                  true,
		"GET /v1/o11y/service_accounts":                   true,
		"GET /v1/o11y/service_accounts/me":                true,
		"PUT /v1/o11y/service_accounts/me":                true,
		"GET /v1/o11y/service_accounts/:id":               true,
		"PUT /v1/o11y/service_accounts/:id":               true,
		"DELETE /v1/o11y/service_accounts/:id":            true,
		"GET /v1/o11y/service_accounts/:id/roles":         true,
		"POST /v1/o11y/service_accounts/:id/roles":        true,
		"DELETE /v1/o11y/service_accounts/:id/roles/:rid": true,
		"POST /v1/o11y/service_accounts/:id/keys":         true,
		"GET /v1/o11y/service_accounts/:id/keys":          true,
		"PUT /v1/o11y/service_accounts/:id/keys/:fid":     true,
		"DELETE /v1/o11y/service_accounts/:id/keys/:fid":  true,
		"POST /v1/o11y/authz/check":                       true,
	}
	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		if r.Method == http.MethodHead || r.Method == http.MethodOptions {
			continue
		}
		if strings.HasPrefix(r.Path, "/v1/o11y/roles") ||
			strings.HasPrefix(r.Path, "/v1/o11y/service_accounts") ||
			strings.HasPrefix(r.Path, "/v1/o11y/authz") {
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
			// identity.go owns the role<->user join (roles/:id/users); this census
			// counts the ACCESS face only. A door another slice converted is not
			// this face growing one.
			if route == "GET /v1/o11y/roles/:id/users" {
				continue
			}
			t.Errorf("%s is registered and was not before — the face grew a door", route)
		}
	}
}

// THE POINT OF THE PORT: the twenty operations are in the document now, each
// under the operation id the face has always published and each with its prose.
// A route behind the wildcard had none of that — no SDK method, no command, no
// agent tool, no reference page — which is what kept an operator from granting a
// role or minting a key from anything but a hand-written call.
func TestAccessReachesTheDocument(t *testing.T) {
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

	want := map[string]map[string]string{
		"/v1/o11y/roles":                             {"post": "CreateRole", "get": "ListRoles"},
		"/v1/o11y/roles/{id}":                        {"get": "GetRole", "put": "UpdateRole", "delete": "DeleteRole"},
		"/v1/o11y/service_accounts":                  {"post": "CreateServiceAccount", "get": "ListServiceAccounts"},
		"/v1/o11y/service_accounts/me":               {"get": "GetMyServiceAccount", "put": "UpdateMyServiceAccount"},
		"/v1/o11y/service_accounts/{id}":             {"get": "GetServiceAccount", "put": "UpdateServiceAccount", "delete": "DeleteServiceAccount"},
		"/v1/o11y/service_accounts/{id}/roles":       {"get": "GetServiceAccountRoles", "post": "CreateServiceAccountRole"},
		"/v1/o11y/service_accounts/{id}/roles/{rid}": {"delete": "DeleteServiceAccountRole"},
		"/v1/o11y/service_accounts/{id}/keys":        {"post": "CreateServiceAccountKey", "get": "ListServiceAccountKeys"},
		"/v1/o11y/service_accounts/{id}/keys/{fid}":  {"put": "UpdateServiceAccountKey", "delete": "RevokeServiceAccountKey"},
		"/v1/o11y/authz/check":                       {"post": "AuthzCheck"},
	}
	for path, methods := range want {
		for method, opID := range methods {
			op, ok := spec.Paths[path][method]
			if !ok {
				t.Errorf("%s %s is not in the document", strings.ToUpper(method), path)
				continue
			}
			if op.OperationID != opID {
				t.Errorf("%s %s has operation id %q, want %q — the published id did not survive the move", strings.ToUpper(method), path, op.OperationID, opID)
			}
			if op.Summary == "" {
				t.Errorf("%s %s has no prose in the document — the doc comment never left the source", strings.ToUpper(method), path)
			}
		}
	}
}

// The REST of the o11y face is untouched: a path no typed op claims still reaches
// the runtime through the host's wildcard, and the typed access paths take
// precedence over it however the host ordered its mounts.
func TestAccessTheRestOfTheFaceStillReachesTheRuntime(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	// The host registers its /v1/o11y wildcard BEFORE the module mounts, which is
	// the order the composed binary uses.
	app.All("/v1/o11y/*", zip.AdaptNetHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"door":"wildcard","path":"`+r.URL.Path+`"}`)
	})))
	if err := o11y.Mount(app); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	answers(t, http.StatusOK, `{"status":"success","data":[]}`)

	// /v1/o11y/dashboards was in this list until the DASHBOARDS slice typed it —
	// it now dispatches off the mux tree ahead of the wildcard, which is the
	// migration succeeding, not the access face breaking. Its own slice test
	// proves it. /alerts left this list for the SAME reason once mountRulesAlerts
	// was actually wired into Mount — it had only appeared wildcarded because that
	// slice was DARK (its file landed, nothing called it). There is no
	// still-wildcarded path left to sample from this face, so the fallthrough
	// guard now lives where the hatches do: mount_test.go.
	// ...and the typed access path wins over that wildcard.
	if _, body := call(t, app, member(http.MethodGet, "/v1/o11y/roles", nil)); strings.Contains(string(body), `"door":"wildcard"`) {
		t.Fatalf("the typed op did not take precedence: %s", body)
	}
}
