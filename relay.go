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
// "/v1/o11y", and prefix was "/v1/sentinel". A name per place instead of a name
// per value — so a change to the seam had five sites to find, and a change to
// the root had eight.
//
// THE ADDRESS IS NOT A PARAMETER OF THE TRANSPORT. That collapse left one seam
// and one root, and one thing still spelled twice: the address. Every op stated
// it at its registration and then stated it AGAIN, by hand, in the call —
//
//	opGet(g, "/traces/:traceId", traceSpans)                    // once
//	relay(ctx, "GET", o11yRoot+"/traces/"+in.TraceID, …)        // and again
//
// — 706 spellings of 367 addresses, and the second spelling is the one that got
// requested, so a route pattern that stopped agreeing with it was never
// consulted and never contradicted. That is not hypothetical: three of these
// addresses name their segment traceId while the runtime registered traceID, and
// nothing in the process could tell, because a router matches by POSITION and a
// parameter's name is invisible to it.
//
// So the address is stated once, at the registration, and travels with the call
// (see claim.go's addressed). relay takes no method and no path: it reads the
// address of the door that was knocked on, which is a fact about the call and
// not a decision this function makes. The 119 hand-built paths are gone with it —
// zip.Address renders the concrete one from the same input zip bound the segments
// into, so the round trip is exact by construction.
//
// What relay is: it hands a typed op's call to the handler the runtime serves AT
// THAT ADDRESS — the same handler the runtime's own router would dispatch to, so
// the request runs the whole chain it always ran, in order. The auth middleware
// resolves the same identity, the SAME ViewAccess/EditAccess/AdminAccess gate the
// route has always had refuses the same callers, the audit record is written
// where it always was, and the bytes it answers with are the bytes the runtime
// wrote. There is no policy here: no tenant is resolved, no role is checked,
// nothing is scoped. Those all happen where they already do, one layer in. That
// is what keeps a typed op a NAMING of the wire rather than a second
// implementation of it.
//
// What is GONE is the second match. The old seam handed the request to the
// runtime's whole router, which parsed the path this function had just built to
// find the handler this function had already named — an in-process HTTP round
// trip to answer a question the caller had the answer to. Naming the address
// deletes it, and deletes with it the entire failure class the reachability
// census exists for: a request cannot lose its route to a router that is never
// consulted.
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
//   - sentinelRoot is the error-tracking face's own contract, kept separate
//     because a Sentry SDK reaches it with a DSN key rather than a session.
const (
	o11yRoot   = "/v1/o11y"
	sentinelRoot = "/v1/sentinel"
)

// relay hands one typed op's call to the runtime handler at the op's OWN address
// and decodes the answer into the op's Out. Pass out == nil for the operations
// whose answer is a 204 with no body.
//
// The address comes from the context, not from the caller — see the file comment
// and claim.go's addressed.
func relay(ctx context.Context, params url.Values, body, out any) error {
	a, known := addressOf(ctx)
	if !known {
		// Only reachable by calling an op's function directly rather than through
		// the declaration that registered it, which is a programming error and not
		// a request that can arrive.
		return zip.ErrInternal("o11y: this operation was invoked outside its own registration, so it has no address to relay to")
	}
	h, err := at(a.method, a.template)
	if err != nil {
		return err
	}

	target := a.path
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
	req, err := http.NewRequestWithContext(ctx, a.method, target, payload)
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
	// The in-process case no longer depends on it — nothing matches this path any
	// more, because the handler was chosen by name. It stays for the case that
	// still does: a runtime in ANOTHER process is reached through [Whole], and
	// behind that one door is a proxy that has to forward a request-target. It is
	// also the field a middleware logs, and a blank one there reads as a lost
	// request.
	//
	// The bug it was written for is worth keeping in view, because it is the whole
	// argument for naming addresses. When the host embedded the runtime
	// IN-PROCESS, that single handler was adaptor.FiberApp, which copies
	// r.RequestURI into the fasthttp request verbatim — so an empty one erased the
	// path, fasthttp normalized it to "/", no API route matched, and the request
	// fell through to the console web provider, which answers http.NotFound. That
	// is why the unified door answered every relayed op
	//   404 {"status":404,"error":"404 page not found"}
	// — /version, /health, /global/config, /users/me, all 353 typed ops — while
	// the standalone runtime served the identical paths 200.
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
