package o11y

// THE SEAM. One function, one place: every typed op in this package reaches the
// o11y runtime through relay and through nothing else.
//
// It arrived here by collapse. The conversion grew face by face, and each face
// carried its own copy of this function — relay (telemetry.go), relayAt
// (logs.go), relayO11y (identity.go), relayAPM (apm.go), send (access.go) —
// five bodies that differed only in which prefix constant they concatenated and
// whether they took query parameters. Alongside them sat NINE names for one
// value: identityPrefix, infraPrefix, integrationsPrefix, metricsRoot,
// apmPrefix, accessPrefix, rulesAlertsPrefix and o11yRoot were all the string
// "/v1/o11y", and prefix was "/v1/sentry". A name per place instead of a name
// per value — so a change to the seam had five sites to find, and a change to
// the root had eight.
//
// The fix is the obvious one once seen: the path a caller reaches is a VALUE,
// so ops pass the whole public path (o11yRoot+"/logs") and relay concatenates
// nothing. Which face an op belongs to is a fact about the file it lives in,
// not a parameter of the transport.
//
// What relay is: it hands a typed op's call to the handler SetHandler
// registered — the same value the delegation wildcard forwards to — so the
// request runs the whole chain it always ran, in order. The auth middleware
// resolves the same identity, the SAME ViewAccess/EditAccess/AdminAccess gate
// the route has always had refuses the same callers, the audit record is
// written where it always was, and the bytes it answers with are the bytes the
// runtime wrote. There is no policy here: no tenant is resolved, no role is
// checked, nothing is scoped. Those all happen where they already do, one layer
// in. That is what keeps a typed op a NAMING of the wire rather than a second
// implementation of it.
//
// Identity is PROPAGATED, never minted: the gateway's assertion about the
// caller travels on as the same headers it arrived on (zip.CallerOf reads
// exactly the set the call plane forwards). A context with no request behind it
// — a command, an agent call — carries no assertion, so the runtime's gate
// refuses it, which is the honest answer rather than an identity invented at
// this hop.
//
// A non-2xx becomes an error carrying the runtime's own status and reason, so
// the status a caller sees is the status the runtime chose.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"

	"github.com/zap-proto/zip"
)

// The two public roots this package answers on, each spelled ONCE.
//
//   - o11yRoot is the observability face: metrics, traces, logs, dashboards,
//     alerts, identity, access, infra, LLM observability — everything the
//     console calls.
//   - sentryRoot is the error-tracking face's own contract, kept separate
//     because a Sentry SDK reaches it with a DSN key rather than a session.
const (
	o11yRoot   = "/v1/o11y"
	sentryRoot = "/v1/sentry"
)

// under is the OpTarget that registers on app at a ROOT — the one place this
// package's two public roots become route prefixes.
//
// It replaces app.Group(o11yRoot), and the difference is the operation ids.
// zip v1.23 qualifies an op's id by the prefix of the OCCURRENCE it answers
// under (walk.go occurrenceID), because a definition included TWICE declares one
// id and produces two operations, and an OpenAPI document cannot hold two
// operations under one operationId. Sound rule — and o11yRoot is not that kind
// of prefix. It is not a composition point a host chose; it is this service's
// own address, fixed by HIP-0106 and spelled above. Reached through a Group it
// read as one, and all 353 published ids became "v1.o11y.CreateRole" — renaming
// every MCP tool, OpenAPI operationId, CLI command and generated SDK method in a
// patch release. There is no opt-out: an explicit WithOperationID is qualified
// too.
//
// An occurrence at the ROOT prefix is unqualified, so this registers the ops on
// app itself and lets OpScope.Prefix do what it documents — prepend to the op's
// path. Same method, same full path, same middleware; the id is the one the
// declaration wrote. That is also what mount.go's PATH UNTOUCHED already claimed
// and what mountHealth already does by hand, so the Group was the odd one out.
//
// The middleware comes from app's own scope rather than being zeroed: Group
// inherits the app's wrap, and dropping it here would silently unwrap every
// typed op.
type under struct {
	app  *zip.App
	root string
}

func (u under) OpScope() zip.OpScope {
	s := u.app.OpScope()
	s.Prefix = u.root
	return s
}

// relay sends one typed op's call to the runtime at the FULL public path and
// decodes the answer into the op's Out. Pass out == nil for the operations
// whose answer is a 204 with no body.
func relay(ctx context.Context, method, path string, params url.Values, body, out any) error {
	h := getHandler()
	if h == nil {
		return zip.Errorf(http.StatusServiceUnavailable, "o11y runtime not initialized")
	}

	target := path
	if q := params.Encode(); q != "" {
		target += "?" + q
	}

	payload := io.Reader(http.NoBody)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return zip.ErrBadRequest(err.Error())
		}
		payload = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return zip.ErrBadRequest(err.Error())
	}
	// A handler is a SERVER seam, so the request handed to it has to be
	// server-SHAPED. http.NewRequest builds a CLIENT request and deliberately
	// leaves RequestURI empty — net/http fills the request-target from URL at
	// write time, and documents that a client must not set it. Nothing writes
	// this request to a wire: it goes straight into an http.Handler, and a
	// handler is entitled to read the field the server would have populated.
	//
	// It is not cosmetic. When the host embeds the runtime IN-PROCESS, that
	// handler is adaptor.FiberApp, which copies r.RequestURI into the fasthttp
	// request verbatim — so an empty one erased the path, fasthttp normalized it
	// to "/", no API route matched, and the request fell through to the console
	// web provider, which answers http.NotFound. That is why the unified door
	// answered every relayed op
	//   404 {"status":404,"error":"404 page not found"}
	// — /version, /health, /global/config, /users/me, all 353 typed ops — while
	// the standalone runtime served the identical paths 200. The three native
	// probes (livez/healthz/readyz) kept working precisely because mountHealth
	// dispatches them itself and never comes through here, which is what made the
	// break look like an auth problem instead of a lost path.
	//
	// The out-of-process host was immune (its handler is a reverse proxy, which
	// re-derives the target from URL), so this only ever bit the embed — the
	// configuration cloud runs in production.
	req.RequestURI = target
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	caller := zip.CallerOf(ctx)
	for header, value := range map[string]string{
		zip.HeaderOrg:       caller.Org,
		zip.HeaderProject:   caller.Project,
		zip.HeaderUser:      caller.User,
		zip.HeaderUserName:  caller.Name,
		zip.HeaderUserEmail: caller.Email,
		zip.HeaderUserOwner: caller.Owner,
		zip.HeaderRequestID: caller.RequestID,
	} {
		if value != "" {
			req.Header.Set(header, value)
		}
	}
	if caller.Admin {
		req.Header.Set(zip.HeaderUserAdmin, "true")
	}
	if caller.OrgAdmin {
		req.Header.Set(zip.HeaderUserOrgAdmin, "true")
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code < http.StatusOK || rec.Code >= http.StatusMultipleChoices {
		code, reason := refusal(rec.Body.Bytes())
		return &zip.HTTPError{Status: rec.Code, Code: code, Msg: reason}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		return zip.ErrInternal("cannot read the runtime's answer: " + err.Error())
	}
	return nil
}

// query builds the parameters an op sends from name/value pairs, dropping the
// ones the caller left unset so an absent input stays absent instead of arriving
// as an empty string — or as a zero page cap, which these faces have always read
// as "no cap given". It is the ONE place a typed input is rendered onto the wire.
func query(pairs ...any) url.Values {
	params := url.Values{}
	for i := 0; i+1 < len(pairs); i += 2 {
		name, _ := pairs[i].(string)
		switch value := pairs[i+1].(type) {
		case string:
			if value != "" {
				params.Set(name, value)
			}
		case int:
			if value != 0 {
				params.Set(name, strconv.Itoa(value))
			}
		case bool:
			if value {
				params.Set(name, "true")
			}
		}
	}
	return params
}

// nanos renders a nanosecond epoch for the wire, or nothing when unset — the
// runtime reads a missing or non-positive value as "use the default window", so
// an absent input stays absent. It feeds query, which stays the ONE place
// inputs are rendered onto the wire.
func nanos(v int64) string {
	if v <= 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

// refusal reads the runtime's refusal for its code and its reason. Both of the
// shapes these faces answer with are handled — the runtime's own {error:{code,
// message}} and the gate's {msg} — and anything else falls back to the body it
// sent, so a reason is never invented and never lost.
func refusal(body []byte) (code, reason string) {
	var refused struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(body, &refused); err == nil {
		if refused.Error.Message != "" {
			return refused.Error.Code, refused.Error.Message
		}
		if refused.Msg != "" {
			return "", refused.Msg
		}
	}
	return "", strings.TrimSpace(string(body))
}

// op names an operation for the document. Several routes carried an explicit
// OpenAPI operation id on the runtime's own handler definitions; those ids are
// preserved verbatim so an operation's name survives the move into the composed
// document.
func op(id string) zip.OpOption { return zip.WithOperationID(id) }
