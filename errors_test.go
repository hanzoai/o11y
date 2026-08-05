package o11y_test

// THE WIRE PROOF for the ERROR-READ family — the five legacy exception reads
// (listErrors, countErrors, errorFromErrorID, errorFromGroupID,
// nextPrevErrorIDs) that sentryerrors.go typed.
//
// They were the last routes of that face still without one. The slice landed
// with its ops and its zipdoc but no test, and an untested op is a claim rather
// than a fact: the source says the route is typed, and only a proof says the
// binary agrees. That gap is not hypothetical here — mount.go carried exactly
// it, and 83 typed ops in this package reached no router at all until
// route-table arithmetic caught them (see mount.go's note). A slice test is
// what keeps that from coming back one slice at a time.
//
// The method is the one telemetry_test.go, logs_test.go and apm_test.go use:
// not that the answer "looks right", but that the bytes the RUNTIME wrote and
// the bytes the OP sent are THE SAME BYTES, for payloads with every field
// populated, and that the request the runtime receives is the request the
// caller made. A field one of these Outs failed to name, or named with a
// different tag, shows up here as a diff.
//
// THESE FIVE ANSWER BARE. Every other slice's runtime stand-in writes through
// render.Success, because those routes answer inside the {status, data}
// envelope. These do not: they are the older face, written with
// APIHandler.WriteJSON, which marshals the payload and writes it with no
// envelope at all (pkg/query-service/app/http_handler.go). So the stand-in here
// mirrors WriteJSON rather than render.Success — using the wrong one would
// prove the ops against an envelope the routes have never sent, which is a test
// that passes while the surface is broken.
//
// The gates are the runtime's and stay there: all five are ViewAccess
// (pkg/query-service/app/routes_errors.go). Nothing here re-checks a role —
// what is pinned is that a refusal travels back with the runtime's own status
// and reason, and that with no runtime at all the ops fail closed on 503.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/query-service/model"
	"github.com/zap-proto/zip"
)

// errorsRuntime installs a stand-in for the o11y runtime that answers the way
// the legacy exceptions face has always answered — APIHandler.WriteJSON's plain
// marshal, no envelope, 200 implied — and reports what it wrote and what it was
// asked. Marshalling the RUNTIME'S OWN type is the load-bearing part: the bytes
// come from model.Error/model.ErrorWithSpan/model.NextPrevErrorIDs, so the
// comparison is against the wire the runtime really writes and not against a
// fixture that could drift with the op it is meant to check.
func errorsRuntime(t *testing.T, payload any) (wrote *[]byte, asked **http.Request) {
	t.Helper()
	var body []byte
	var req *http.Request
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(read))
		req = r.Clone(r.Context())
		req.Body = io.NopCloser(bytes.NewReader(read))

		marshalled, err := json.Marshal(payload)
		if err != nil {
			t.Errorf("marshal payload: %v", err)
			return
		}
		body = marshalled
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })
	return &body, &req
}

// seen is one moment every timestamp in this file is pinned to, so a dropped or
// reordered time field cannot hide behind two values that happen to match.
var seen = time.Date(2026, 7, 31, 12, 0, 0, 123456789, time.UTC)

// ── listErrors ──────────────────────────────────────────────────────────────

// The grouped-exception listing is the runtime's answer to the byte — a BARE
// array, not an envelope — and the body the runtime receives is the caller's
// own request, tag predicates included.
func TestListErrorsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := errorsRuntime(t, &[]model.Error{{
		ExceptionType: "TypeError", ExceptionMsg: "x is not a function",
		ExceptionCount: 42, LastSeen: seen, FirstSeen: seen.Add(-time.Hour),
		ServiceName: "api", GroupID: "g1",
	}, {
		ExceptionType: "ValueError", ExceptionMsg: "bad input",
		ExceptionCount: 7, LastSeen: seen.Add(-time.Minute), FirstSeen: seen.Add(-2 * time.Hour),
		ServiceName: "worker", GroupID: "g2",
	}})

	sent := `{"start":"1753899000000000000","end":"1753900000000000000","limit":50,` +
		`"orderParam":"exceptionCount","order":"descending","offset":10,` +
		`"serviceName":"api","exceptionType":"TypeError",` +
		`"tags":[{"key":"deployment.environment","tagType":"ResourceAttribute",` +
		`"stringValues":["prod","staging"],"boolValues":[true],"numberValues":[1.5],"operator":"In"}]}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/listErrors", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/listErrors" || r.Method != http.MethodPost {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
	assertBodyRoundTrips[model.ListErrorsParams](t, sent, *asked)
}

// ── countErrors ─────────────────────────────────────────────────────────────

// The count answers a BARE number — no envelope, no wrapper object — and the op
// carries it through untouched. A scalar Out is the case an envelope-shaped
// port would silently break, so it gets its own proof.
func TestCountErrorsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := errorsRuntime(t, uint64(4217))

	sent := `{"start":"1753899000000000000","end":"1753900000000000000",` +
		`"serviceName":"api","exceptionType":"TypeError",` +
		`"tags":[{"key":"service.name","tagType":"ResourceAttribute",` +
		`"stringValues":["api"],"boolValues":[],"numberValues":[],"operator":"In"}]}`
	status, got := call(t, app, member(http.MethodPost, "/v1/o11y/countErrors", strings.NewReader(sent)))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	if r := *asked; r.URL.Path != "/v1/o11y/countErrors" || r.Method != http.MethodPost {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
	assertBodyRoundTrips[model.CountErrorsParams](t, sent, *asked)
}

// ── the three instance lookups ──────────────────────────────────────────────

// One exception instance and the span it happened on, by error id. These reads
// take their inputs on the QUERY, not in a body — parseGetErrorRequest reads
// r.URL.Query() — so what is pinned is that all three named inputs arrive
// there, spelled as the caller spelled them.
func TestErrorFromErrorIDAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := errorsRuntime(t, &model.ErrorWithSpan{
		ErrorID: "e1", ExceptionType: "TypeError",
		ExceptionStacktrace: "main.serve\n\tmain.go:42", ExceptionEscaped: true,
		ExceptionMsg: "x is not a function", Timestamp: seen,
		SpanID: "s1", TraceID: "t1", ServiceName: "api", GroupID: "g1",
	})

	status, got := call(t, app, member(http.MethodGet,
		"/v1/o11y/errorFromErrorID?timestamp=1753899000000000000&groupID=g1&errorID=e1", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	r := *asked
	if r.URL.Path != "/v1/o11y/errorFromErrorID" || r.Method != http.MethodGet {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
	for name, want := range map[string]string{
		"timestamp": "1753899000000000000", "groupID": "g1", "errorID": "e1",
	} {
		if got := r.URL.Query().Get(name); got != want {
			t.Errorf("%s reached the runtime as %q, want %q (asked %s?%s)",
				name, got, want, r.URL.Path, r.URL.RawQuery)
		}
	}
}

// The group's representative instance. Its prose says the error id is UNUSED on
// this route, and an input the caller left unset must stay unset rather than
// arrive as an empty string — the runtime reads errorID off the query, so a
// blank one is not the same as an absent one. This is what pins that.
func TestErrorFromGroupIDLeavesTheUnusedErrorIDOff(t *testing.T) {
	app := mounted(t)
	wrote, asked := errorsRuntime(t, &model.ErrorWithSpan{
		ErrorID: "e9", ExceptionType: "ValueError",
		ExceptionStacktrace: "worker.run\n\trun.go:7", ExceptionMsg: "bad input",
		Timestamp: seen, SpanID: "s9", TraceID: "t9",
		ServiceName: "worker", GroupID: "g2",
	})

	status, got := call(t, app, member(http.MethodGet,
		"/v1/o11y/errorFromGroupID?timestamp=1753899000000000000&groupID=g2", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	r := *asked
	if r.URL.Path != "/v1/o11y/errorFromGroupID" {
		t.Fatalf("runtime was asked %s", r.URL.Path)
	}
	if got := r.URL.Query().Get("groupID"); got != "g2" {
		t.Errorf("groupID reached the runtime as %q, want %q", got, "g2")
	}
	if r.URL.Query().Has("errorID") {
		t.Errorf("an unset errorID was invented on the wire: %s?%s", r.URL.Path, r.URL.RawQuery)
	}
}

// The paging cursor the error detail view walks — next and previous instance
// within the group, each with its time.
func TestNextPrevErrorIDsAnswerIsTheRuntimeAnswer(t *testing.T) {
	app := mounted(t)
	wrote, asked := errorsRuntime(t, &model.NextPrevErrorIDs{
		NextErrorID: "e2", NextTimestamp: seen.Add(time.Minute),
		PrevErrorID: "e0", PrevTimestamp: seen.Add(-time.Minute),
		GroupID: "g1",
	})

	status, got := call(t, app, member(http.MethodGet,
		"/v1/o11y/nextPrevErrorIDs?timestamp=1753899000000000000&groupID=g1&errorID=e1", nil))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", status, got)
	}
	if !bytes.Equal(got, *wrote) {
		t.Fatalf("the op changed the bytes.\n runtime: %s\n op:      %s", *wrote, got)
	}
	r := *asked
	if r.URL.Path != "/v1/o11y/nextPrevErrorIDs" || r.Method != http.MethodGet {
		t.Fatalf("runtime was asked %s %s", r.Method, r.URL.Path)
	}
	for name, want := range map[string]string{
		"timestamp": "1753899000000000000", "groupID": "g1", "errorID": "e1",
	} {
		if got := r.URL.Query().Get(name); got != want {
			t.Errorf("%s reached the runtime as %q, want %q (asked %s?%s)",
				name, got, want, r.URL.Path, r.URL.RawQuery)
		}
	}
}

// ── the document ────────────────────────────────────────────────────────────

// THE POINT OF THE PORT: the five error reads are in the document, each with an
// operation id and its prose. A route behind the delegation wildcard is in no
// document — no SDK method, no CLI command, no agent tool, no reference page —
// so this is the assertion that says the conversion actually happened, and the
// one that goes red if these ops ever stop reaching the router.
//
// The paths are the camelCase legacy literals ON PURPOSE. They are the public
// contract this face has always answered on; only the Go identifiers behind
// them follow house style. Renaming a path here would be a break dressed as a
// tidy-up.
func TestErrorReadsReachTheDocument(t *testing.T) {
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
		"/v1/o11y/listErrors":       "post",
		"/v1/o11y/countErrors":      "post",
		"/v1/o11y/errorFromErrorID": "get",
		"/v1/o11y/errorFromGroupID": "get",
		"/v1/o11y/nextPrevErrorIDs": "get",
	} {
		op, ok := spec.Paths[path][method]
		if !ok {
			t.Errorf("%s %s is not in the document", strings.ToUpper(method), path)
			continue
		}
		if op.OperationID == "" {
			t.Errorf("%s %s has no operation id, so nothing can name it", strings.ToUpper(method), path)
		}
		if len(op.Summary) < 20 {
			t.Errorf("%s %s has no prose in the document: %q", strings.ToUpper(method), path, op.Summary)
		}
	}
}

// ── the gates, carried ──────────────────────────────────────────────────────

// All five are ViewAccess, and the gate stays the runtime's. What the op owes
// the caller is the runtime's own answer: when the gate refuses, the status and
// the reason are the ones the runtime chose, not a status invented at the relay.
func TestErrorReadsRefusalKeepsTheRuntimeStatus(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(o11y.Whole(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"status":"error","errorType":"forbidden","error":"user is not authorized"}`)
	})))
	t.Cleanup(func() { o11y.SetRuntime(nil) })

	for _, probe := range []struct {
		method, target string
		body           io.Reader
	}{
		{http.MethodPost, "/v1/o11y/listErrors", strings.NewReader(`{"limit":10}`)},
		{http.MethodPost, "/v1/o11y/countErrors", strings.NewReader(`{}`)},
		{http.MethodGet, "/v1/o11y/errorFromErrorID?timestamp=1&groupID=g1&errorID=e1", nil},
		{http.MethodGet, "/v1/o11y/errorFromGroupID?timestamp=1&groupID=g1", nil},
		{http.MethodGet, "/v1/o11y/nextPrevErrorIDs?timestamp=1&groupID=g1&errorID=e1", nil},
	} {
		status, got := call(t, app, member(probe.method, probe.target, probe.body))
		if status != http.StatusForbidden {
			t.Errorf("%s %s: status=%d body=%s, want 403 (the runtime's own)",
				probe.method, probe.target, status, got)
			continue
		}
		if !strings.Contains(string(got), "user is not authorized") {
			t.Errorf("%s %s: the reason was lost: %s", probe.method, probe.target, got)
		}
	}
}

// No runtime, no answer: the error reads fail closed with the same 503 the
// delegation wildcard gives before a handler is registered. The GETs carry
// their required lookup inputs so the request reaches the relay and fails
// THERE — a 400 from the binder would prove nothing about the seam.
func TestErrorReadsFailClosedWithoutARuntime(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(nil)

	for _, probe := range []struct {
		method, target string
		body           io.Reader
	}{
		{http.MethodPost, "/v1/o11y/listErrors", strings.NewReader(`{"limit":10}`)},
		{http.MethodPost, "/v1/o11y/countErrors", strings.NewReader(`{}`)},
		{http.MethodGet, "/v1/o11y/errorFromErrorID?timestamp=1&groupID=g1&errorID=e1", nil},
		{http.MethodGet, "/v1/o11y/errorFromGroupID?timestamp=1&groupID=g1", nil},
		{http.MethodGet, "/v1/o11y/nextPrevErrorIDs?timestamp=1&groupID=g1&errorID=e1", nil},
	} {
		if status, got := call(t, app, member(probe.method, probe.target, probe.body)); status != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status=%d body=%s, want 503", probe.method, probe.target, status, got)
		}
	}
}

// The caller's identity travels to the runtime on this seam exactly as it does
// on the others — propagated as the gateway asserted it, never minted here.
// Without it the runtime's ViewAccess gate has nobody to check, so an invented
// identity would be the one way these ops could hand out another org's errors.
func TestErrorReadsIdentityIsPropagated(t *testing.T) {
	app := mounted(t)
	_, asked := errorsRuntime(t, &[]model.Error{})

	r := member(http.MethodPost, "/v1/o11y/listErrors", strings.NewReader(`{"limit":10}`))
	r.Header.Set(zip.HeaderUserName, "Z")
	if status, body := call(t, app, r); status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	got := (*asked).Header
	for header, want := range map[string]string{
		zip.HeaderOrg:       "maxpower",
		zip.HeaderUser:      "z",
		zip.HeaderUserEmail: "z@hanzo.ai",
		zip.HeaderUserName:  "Z",
	} {
		if got.Get(header) != want {
			t.Errorf("%s reached the runtime as %q, want %q", header, got.Get(header), want)
		}
	}
}
