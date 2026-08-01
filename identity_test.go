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

// THE WIRE PROOF for the identity face — telemetry_test.go's discipline applied
// to the forty-five typed identity ops. The reads take the bytes the RUNTIME
// wrote (through the SAME render.Success the handlers use) and the bytes the OP
// answered and demand they are the same bytes, for a payload built from the
// face's OWN types with every field populated. The writes prove the body the
// runtime receives is the caller's own, field for field. A field this port
// failed to name, or named with a different tag, or ordered differently, shows
// up here as a diff. The routing and document proofs pin the whole face at once.
//
// The helpers (mounted, runtime, call, member, mustJSON) are telemetry_test.go's;
// every typed face is proved with the one harness.

// identityOps is the face's routing table, spelled once: the forty-five typed
// ops, their methods and their operation ids, as mountIdentity registers them.
// The routing proof reads it as native Fiber routes, the document proof as
// OpenAPI paths — one source, two projections.
var identityOps = []struct{ Method, Path, OpID string }{
	{"POST", "/invite", "CreateInvite"},
	{"POST", "/invite/bulk", "CreateBulkInvite"},
	{"GET", "/user", "ListUsersDeprecated"},
	{"GET", "/users", "ListUsers"},
	{"GET", "/user/me", "GetMyUserDeprecated"},
	{"GET", "/users/me", "GetMyUser"},
	{"POST", "/users", "CreateUser"},
	{"PUT", "/users/me", "UpdateMyUserV2"},
	{"GET", "/user/:id", "GetUserDeprecated"},
	{"GET", "/users/:id", "GetUser"},
	{"PUT", "/user/:id", "UpdateUserDeprecated"},
	{"PUT", "/users/:id", "UpdateUser"},
	{"DELETE", "/user/:id", "DeleteUserDeprecated"},
	{"DELETE", "/users/:id", "DeleteUser"},
	{"GET", "/getResetPasswordToken/:id", "GetResetPasswordTokenDeprecated"},
	{"GET", "/users/:id/reset_password_tokens", "GetResetPasswordToken"},
	{"PUT", "/users/:id/reset_password_tokens", "CreateResetPasswordToken"},
	{"POST", "/reset_password_tokens/verify", "VerifyResetPasswordToken"},
	{"POST", "/resetPassword", "ResetPassword"},
	{"PUT", "/users/me/factor_password", "UpdateMyPassword"},
	{"POST", "/factor_password/forgot", "ForgotPassword"},
	{"GET", "/users/:id/roles", "GetRolesByUserID"},
	{"POST", "/users/:id/roles", "SetRoleByUserID"},
	{"DELETE", "/users/:id/roles/:roleId", "RemoveUserRoleByUserIDAndRoleID"},
	{"GET", "/roles/:id/users", "GetUsersByRoleID"},
	{"POST", "/sessions/email_password", "CreateSessionByEmailPassword"},
	{"GET", "/sessions/context", "GetSessionContext"},
	{"POST", "/sessions/rotate", "RotateSession"},
	{"DELETE", "/sessions", "DeleteSession"},
	{"GET", "/domains", "ListAuthDomains"},
	{"POST", "/domains", "CreateAuthDomain"},
	{"GET", "/domains/:id", "GetAuthDomain"},
	{"PUT", "/domains/:id", "UpdateAuthDomain"},
	{"DELETE", "/domains/:id", "DeleteAuthDomain"},
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

// THE ROUTES, exactly as the mux tree declared them: forty-five typed paths on
// the native router, and the three /complete/* sign-in callbacks NOT among them —
// they answer with 303 redirects, so they stay on the delegation wildcard where
// a typed 2xx-JSON contract cannot lie about them.
func TestIdentityRoutesAreTheSameFortyFive(t *testing.T) {
	if len(identityOps) != 45 {
		t.Fatalf("identityOps has %d entries, want 45", len(identityOps))
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
	for _, cb := range []string{
		"GET /v1/o11y/complete/google",
		"POST /v1/o11y/complete/saml",
		"GET /v1/o11y/complete/oidc",
	} {
		if got[cb] {
			t.Errorf("%s is registered natively; it must stay on the wildcard (it answers 303)", cb)
		}
	}
}

// The three SSO callbacks still reach the runtime through the host's wildcard,
// and the typed identity paths take precedence over that same wildcard however
// the host ordered its mounts — the escape hatch is honest and the ops win.
func TestCompleteCallbacksStayOnTheWildcard(t *testing.T) {
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
	runtime(t, []any{})

	for method, target := range map[string]string{
		http.MethodGet:  "/v1/o11y/complete/google",
		http.MethodPost: "/v1/o11y/complete/saml",
	} {
		if _, body := call(t, app, member(method, target, nil)); !strings.Contains(string(body), `"door":"wildcard"`) {
			t.Errorf("%s no longer reaches the runtime through the wildcard: %s", target, body)
		}
	}
	if _, body := call(t, app, member(http.MethodGet, "/v1/o11y/complete/oidc", nil)); !strings.Contains(string(body), `"door":"wildcard"`) {
		t.Errorf("/v1/o11y/complete/oidc no longer reaches the wildcard: %s", body)
	}
	// ...and the typed op wins over that wildcard.
	if _, body := call(t, app, member(http.MethodGet, "/v1/o11y/users", nil)); strings.Contains(string(body), `"door":"wildcard"`) {
		t.Fatalf("the typed users op did not take precedence: %s", body)
	}
}

// A page of org members is what the runtime wrote, to the byte — each member's
// every field populated, so a dropped or renamed field cannot hide behind a
// zero value — and the runtime is asked at the member collection, verbatim.
func TestUsersAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC)
	app := mounted(t)
	wrote, asked := runtime(t, []o11y.O11yUser{
		{ID: "u1", DisplayName: "Ada", Email: "ada@example.com", OrgID: "maxpower", IsRoot: true, Status: "active", CreatedAt: at, UpdatedAt: at.Add(time.Hour)},
		{ID: "u2", DisplayName: "Grace", Email: "grace@example.com", OrgID: "maxpower", Status: "pending_invite", CreatedAt: at, UpdatedAt: at},
	})

	status, got := call(t, app, member(http.MethodGet, "/v1/o11y/users", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/users" {
		t.Fatalf("runtime was asked %q, want /v1/o11y/users", r.URL.Path)
	}
}

// The calling user and every role they hold is what the runtime wrote, to the
// byte — a nested role assignment carrying its own role, so a member's whole
// role graph is proved, not just the scalar fields.
func TestUserWithRolesAnswerIsTheRuntimeAnswer(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC)
	app := mounted(t)
	role := &o11y.O11yRole{ID: "r1", CreatedAt: at, UpdatedAt: at, Name: "ADMIN", Description: "full access", Type: "managed", OrgID: "maxpower"}
	wrote, asked := runtime(t, o11y.O11yUserWithRoles{
		ID: "u1", DisplayName: "Ada", Email: "ada@example.com", OrgID: "maxpower", IsRoot: true, Status: "active",
		CreatedAt: at, UpdatedAt: at,
		UserRoles: []o11y.O11yUserRole{{ID: "ur1", UserID: "u1", RoleID: "r1", CreatedAt: at, UpdatedAt: at, Role: role}},
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

// The create forwards the caller's postable user through the runtime's own type,
// field for field, and answers with the runtime's 201 — the created status the
// mux registration always carried.
func TestCreateUserForwardsTheBody(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, o11y.O11yCreated{ID: "u9"})

	sent := `{"displayName":"Ada","email":"ada@example.com","frontendBaseUrl":"https://console.example.com",` +
		`"userRoles":[{"id":"r1"},{"id":"r2"}]}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/users", strings.NewReader(sent)))
	if status != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", status, got)
	}

	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have o11y.O11yPostableUser
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

// Sign-in answers the runtime's token pair, to the byte, and the runtime receives
// the caller's own credentials — the body a session is minted from is not
// rewritten on the way through.
func TestSessionTokenAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := runtime(t, o11y.O11yToken{TokenType: "bearer", AccessToken: "acc.jwt", RefreshToken: "ref.jwt", ExpiresIn: 3600})

	sent := `{"email":"ada@example.com","password":"hunter2","orgId":"maxpower"}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/sessions/email_password", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	forwarded, _ := io.ReadAll((*asked).Body)
	var want, have o11y.O11yEmailPasswordSessionIn
	if err := json.Unmarshal([]byte(sent), &want); err != nil {
		t.Fatalf("unmarshal sent: %v", err)
	}
	if err := json.Unmarshal(forwarded, &have); err != nil {
		t.Fatalf("the runtime was sent something it cannot read: %v (%s)", err, forwarded)
	}
	if a, b := mustJSON(t, want), mustJSON(t, have); a != b {
		t.Fatalf("the op rewrote the credentials.\n caller: %s\n runtime: %s", a, b)
	}
}

// The caller's identity travels on as the gateway asserted it — propagated, not
// minted, and not invented when there is none.
func TestIdentityPropagatesTheCaller(t *testing.T) {
	app := mounted(t)
	_, asked := runtime(t, []any{})

	r := member(http.MethodGet, "/v1/o11y/users", nil)
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
		{http.MethodGet, "/v1/o11y/users"},
		{http.MethodGet, "/v1/o11y/users/me"},
		{http.MethodGet, "/v1/o11y/domains"},
		{http.MethodGet, "/v1/o11y/orgs/me"},
		{http.MethodGet, "/v1/o11y/user/preferences"},
	} {
		if status, body := call(t, app, member(tc.method, tc.target, nil)); status != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status=%d body=%s, want 503", tc.method, tc.target, status, body)
		}
	}
}

// THE POINT OF THE PORT: every one of the forty-five ops is in the document now,
// each with its operation id and its prose. A route behind the wildcard had none
// of that — no SDK method, no command, no agent tool, no reference page — which
// is what made a tenant's own membership, sign-in and preferences unreachable
// from anything but a hand-written call.
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
	// The three redirect callbacks are deliberately absent from the document.
	for _, p := range []string{"/v1/o11y/complete/google", "/v1/o11y/complete/saml", "/v1/o11y/complete/oidc"} {
		if _, there := spec.Paths[p]; there {
			t.Errorf("%s entered the document as a typed op — it answers a 303 redirect, and the document would lie about it", p)
		}
	}
}
