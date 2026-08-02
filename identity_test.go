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
	"github.com/zap-proto/zip"
)

// THE WIRE PROOF for what is LEFT of the identity face. The reads take the bytes
// the RUNTIME wrote (through the SAME render.Success the handlers use) and the
// bytes the OP answered and demand they are the same bytes, for a payload built
// from the face's OWN types with every field populated. A field this port failed
// to name, or named with a different tag, or ordered differently, shows up here
// as a diff.
//
// It also proves the DELETIONS, which is the harder half: thirty-three ops and
// three redirect hatches are gone, and a deletion that leaves the door standing
// is not a deletion. TestDeletedIdentityRoutesAre404 asks the router for each of
// them by name.
//
// The helpers (mounted, runtime, call, member, mustJSON) are telemetry_test.go's;
// every typed face is proved with the one harness.

// identityOps is the face's routing table, spelled once: the twelve typed ops,
// their methods and their operation ids, as mountIdentity registers them. The
// routing proof reads it as native Fiber routes, the document proof as OpenAPI
// paths — one source, two projections.
var identityOps = []struct{ Method, Path, OpID string }{
	{"GET", "/users/me", "GetMyUser"},
	{"GET", "/orgs/me", "GetMyOrganization"},
	{"PUT", "/orgs/me", "UpdateMyOrganization"},
	{"GET", "/orgs/me/filters", "GetQuickFilters"},
	{"GET", "/orgs/me/filters/:signal", "GetSignalFilters"},
	{"PUT", "/orgs/me/filters", "UpdateQuickFilters"},
	{"GET", "/user/preferences", "ListUserPreferences"},
	{"GET", "/user/preferences/:name", "GetUserPreference"},
	{"PUT", "/user/preferences/:name", "UpdateUserPreference"},
	{"GET", "/org/preferences", "ListOrgPreferences"},
	{"GET", "/org/preferences/:name", "GetOrgPreference"},
	{"PUT", "/org/preferences/:name", "UpdateOrgPreference"},
}

// deletedIdentityRoutes is every method+path the identity face used to answer
// and must not answer any more — the credential surfaces (a), the member
// administration (c), and the three sign-in callbacks that were the last
// authentication hatches. Spelled out rather than counted, because the failure
// this guards against is ONE of them surviving.
var deletedIdentityRoutes = []struct{ Method, Path string }{
	// invites
	{"POST", "/v1/o11y/invite"},
	{"POST", "/v1/o11y/invite/bulk"},
	// member administration
	{"GET", "/v1/o11y/user"},
	{"GET", "/v1/o11y/users"},
	{"POST", "/v1/o11y/users"},
	{"GET", "/v1/o11y/user/me"},
	{"PUT", "/v1/o11y/users/me"},
	{"GET", "/v1/o11y/user/u1"},
	{"GET", "/v1/o11y/users/u1"},
	{"PUT", "/v1/o11y/user/u1"},
	{"PUT", "/v1/o11y/users/u1"},
	{"DELETE", "/v1/o11y/user/u1"},
	{"DELETE", "/v1/o11y/users/u1"},
	// passwords and their reset tokens
	{"GET", "/v1/o11y/getResetPasswordToken/u1"},
	{"GET", "/v1/o11y/users/u1/reset_password_tokens"},
	{"PUT", "/v1/o11y/users/u1/reset_password_tokens"},
	{"POST", "/v1/o11y/reset_password_tokens/verify"},
	{"POST", "/v1/o11y/resetPassword"},
	{"PUT", "/v1/o11y/users/me/factor_password"},
	{"POST", "/v1/o11y/factor_password/forgot"},
	// roles on users
	{"GET", "/v1/o11y/users/u1/roles"},
	{"POST", "/v1/o11y/users/u1/roles"},
	{"DELETE", "/v1/o11y/users/u1/roles/r1"},
	{"GET", "/v1/o11y/roles/r1/users"},
	// sessions
	{"POST", "/v1/o11y/sessions/email_password"},
	{"GET", "/v1/o11y/sessions/context"},
	{"POST", "/v1/o11y/sessions/rotate"},
	{"DELETE", "/v1/o11y/sessions"},
	// SSO domains
	{"GET", "/v1/o11y/domains"},
	{"POST", "/v1/o11y/domains"},
	{"GET", "/v1/o11y/domains/d1"},
	{"PUT", "/v1/o11y/domains/d1"},
	{"DELETE", "/v1/o11y/domains/d1"},
	// the sign-in callbacks
	{"GET", "/v1/o11y/complete/google"},
	{"POST", "/v1/o11y/complete/saml"},
	{"GET", "/v1/o11y/complete/oidc"},
	// self-registration
	{"POST", "/v1/o11y/register"},
}

// bracePath rewrites a zip route's ":seg" parameters into the OpenAPI document's
// "{seg}" form, so identityOps can address both the router and the spec.
func bracePath(zipPath string) string {
	segs := strings.Split(zipPath, "/")
	for i, s := range segs {
		if strings.HasPrefix(s, ":") {
			segs[i] = "{" + s[1:] + "}"
		}
	}
	return strings.Join(segs, "/")
}

// THE ROUTES, exactly as mountIdentity declares them.
func TestIdentityRoutesAreTheSameTwelve(t *testing.T) {
	if len(identityOps) != 12 {
		t.Fatalf("identityOps has %d entries, want 12", len(identityOps))
	}
	app := mounted(t)
	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		got[r.Method+" "+r.Path] = true
	}
	for _, op := range identityOps {
		key := op.Method + " /v1/o11y" + op.Path
		if !got[key] {
			t.Errorf("%s is not registered as a typed op", key)
		}
	}
}

// THE DELETION, on the router the binary serves.
//
// A registered runtime answers 200 to EVERYTHING, so a route that still exists
// answers 200 here and a route that is gone answers 404. That is the whole
// discrimination, and it is why the runtime is registered rather than left nil:
// with no runtime every path returns 503, and 503 would hide a surviving door.
func TestDeletedIdentityRoutesAre404(t *testing.T) {
	app := mounted(t)
	runtime(t, []any{})

	for _, r := range deletedIdentityRoutes {
		// 404 (no such path) or 405 (the path survives for another method, as
		// /users/me does for GET) both mean this method+path is not served. A
		// 2xx, a 4xx from a handler, or the 503 of an unwired hatch would not.
		status, body := call(t, app, member(r.Method, r.Path, strings.NewReader("{}")))
		if status != http.StatusNotFound && status != http.StatusMethodNotAllowed {
			t.Errorf("%s %s answers %d (%s) — it must be gone, not merely unrouted", r.Method, r.Path, status, body)
		}
	}
}

// The caller's own identity is what the runtime wrote, to the byte — every field
// populated, so a dropped or renamed field cannot hide behind a zero value.
func TestMyUserAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC)
	app := mounted(t)
	wrote, asked := runtime(t, o11y.O11yUser{
		ID: "u1", DisplayName: "ada@example.com", Email: "ada@example.com", OrgID: "maxpower",
		IsRoot: false, Status: "active", CreatedAt: at, UpdatedAt: at,
	})

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/users/me", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/users/me" {
		t.Fatalf("runtime was asked %q, want /v1/o11y/users/me", r.URL.Path)
	}
}

// The org update forwards the caller's body through the face's own type, field
// for field.
func TestUpdateMyOrgForwardsTheBody(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, struct{}{})

	sent := `{"displayName":"Max Power","name":"maxpower","alias":"mp"}`
	if status, got := call(t, app, member(http.MethodPut, "/v1/o11y/orgs/me", strings.NewReader(sent))); status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s, want 204", status, got)
	}

	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have o11y.O11yOrganization
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

// The caller's identity travels on as the gateway asserted it — propagated, not
// minted, and not invented when there is none. This is the ONLY way a caller is
// identified now, which is why it is proved on the identity face itself.
func TestIdentityPropagatesTheCaller(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, o11y.O11yUser{ID: "u1"})

	r := member(http.MethodGet, "/v1/o11y/users/me", nil)
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
func TestIdentityFailsClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetHandler(nil)

	for _, tc := range []struct{ method, target string }{
		{http.MethodGet, "/v1/o11y/users/me"},
		{http.MethodGet, "/v1/o11y/orgs/me"},
		{http.MethodGet, "/v1/o11y/user/preferences"},
	} {
		if status, body := call(t, app, member(tc.method, tc.target, nil)); status != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status=%d body=%s, want 503", tc.method, tc.target, status, body)
		}
	}
}

// Every surviving op is in the document, with its operation id and its prose —
// and every deleted one is NOT, because a document that still advertises a
// password reset is a client that will call one.
func TestIdentityReachesTheDocument(t *testing.T) {
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

	for _, op := range identityOps {
		path := "/v1/o11y" + bracePath(op.Path)
		method := strings.ToLower(op.Method)
		doc, ok := spec.Paths[path][method]
		if !ok {
			t.Errorf("%s %s is not in the document", op.Method, path)
			continue
		}
		if doc.OperationID == "" {
			t.Errorf("%s %s has no operation id, so nothing can name it", op.Method, path)
		}
		if len(doc.Summary) < 20 {
			t.Errorf("%s %s has no prose in the document: %q", op.Method, path, doc.Summary)
		}
	}

	for _, p := range []string{
		"/v1/o11y/invite", "/v1/o11y/users", "/v1/o11y/users/{id}", "/v1/o11y/sessions",
		"/v1/o11y/sessions/rotate", "/v1/o11y/sessions/email_password", "/v1/o11y/domains",
		"/v1/o11y/resetPassword", "/v1/o11y/factor_password/forgot",
		"/v1/o11y/users/me/factor_password", "/v1/o11y/complete/google",
	} {
		if _, there := spec.Paths[p]; there {
			t.Errorf("%s is still in the document — a generated client would still call it", p)
		}
	}
}
