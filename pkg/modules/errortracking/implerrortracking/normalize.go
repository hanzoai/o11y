package implerrortracking

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
)

const (
	maxValueLen   = 8192
	maxCulpritLen = 512
	maxFrames     = 250
)

// ingestOpts carries the request-scoped ingest policy into the (otherwise pure)
// normalizer — currently just whether to retain end-user PII.
type ingestOpts struct {
	capturePII bool
}

// normalizeEvent turns a decoded Sentry event into the canonical Occurrence and
// stamps its fingerprint. It is total: any missing/odd field degrades to a safe
// default rather than erroring, because an ingest endpoint must never 500 on a
// malformed client payload. Secrets are always redacted and PII scrubbed (unless
// capture is enabled) BEFORE the value enters the fingerprint. The default (no
// opts) is fail-secure: scrub.
func normalizeEvent(e *errortrackingtypes.SentryEvent, opts ...ingestOpts) *errortrackingtypes.Occurrence {
	var o ingestOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	occ := &errortrackingtypes.Occurrence{
		EventID:     e.EventID,
		Level:       firstNonEmpty(strings.ToLower(e.Level), errortrackingtypes.DefaultLevel),
		Platform:    e.Platform,
		Timestamp:   parseTimestamp(e.Timestamp),
		Environment: e.Environment,
		Release:     e.Release,
		ServerName:  e.ServerName,
		Transaction: e.Transaction,
		Tags:        parseTags(e.Tags),
	}

	if val := primaryException(e.Exception); val != nil {
		occ.Type = strings.TrimSpace(val.Type)
		occ.Value = truncate(val.Value, maxValueLen)
		if val.Stacktrace != nil {
			occ.Frames = convertFrames(val.Stacktrace.Frames)
		}
	}

	// Message-only event (no exception): the message IS the grouping value.
	if occ.Type == "" && occ.Value == "" {
		if msg := parseMessage(e.Message); msg != "" {
			occ.Type = "Message"
			occ.Value = truncate(msg, maxValueLen)
		}
	}

	occ.ServiceName = serviceName(e)
	occ.TraceID, occ.SpanID = traceContext(e.Contexts)
	occ.User = convertUser(e.User)
	occ.Culprit = truncate(culprit(occ, e), maxCulpritLen)

	// Redact secrets (always) + PII (unless captured) before the value is hashed, so
	// grouping is stable and nothing sensitive reaches the fingerprint or storage.
	sanitizeOccurrence(occ, o.capturePII)

	occ.Fingerprint = computeFingerprint(occ, e.Fingerprint)
	return occ
}

// primaryException returns the thrown exception — the last value with content —
// following the Sentry convention that chained causes precede the raised error.
func primaryException(ex *errortrackingtypes.SentryException) *errortrackingtypes.SentryExceptionValue {
	if ex == nil || len(ex.Values) == 0 {
		return nil
	}
	for i := len(ex.Values) - 1; i >= 0; i-- {
		v := ex.Values[i]
		if v.Type != "" || v.Value != "" || v.Stacktrace != nil {
			return &ex.Values[i]
		}
	}
	return &ex.Values[len(ex.Values)-1]
}

func convertFrames(in []errortrackingtypes.SentryFrame) []errortrackingtypes.Frame {
	if len(in) > maxFrames {
		in = in[len(in)-maxFrames:]
	}
	out := make([]errortrackingtypes.Frame, 0, len(in))
	for _, f := range in {
		out = append(out, errortrackingtypes.Frame{
			Function: f.Function,
			Module:   f.Module,
			Filename: f.Filename,
			AbsPath:  f.AbsPath,
			Lineno:   f.Lineno,
			Colno:    f.Colno,
			InApp:    f.InApp != nil && *f.InApp,
		})
	}
	return out
}

func convertUser(u *errortrackingtypes.SentryUser) *errortrackingtypes.EventUser {
	if u == nil {
		return nil
	}
	if u.ID == "" && u.Email == "" && u.Username == "" && u.IPAddress == "" {
		return nil
	}
	return &errortrackingtypes.EventUser{ID: u.ID, Email: u.Email, Username: u.Username, IP: u.IPAddress}
}

// culprit is the human-readable location shown on the issue: the transaction if
// set, else the crash frame rendered readably, else the logger.
func culprit(occ *errortrackingtypes.Occurrence, e *errortrackingtypes.SentryEvent) string {
	if e.Transaction != "" {
		return e.Transaction
	}
	if f := pickCrashFrame(occ.Frames); f != nil {
		loc := firstNonEmpty(f.Module, baseName(f.Filename), baseName(f.AbsPath))
		switch {
		case f.Function != "" && loc != "":
			return f.Function + " in " + loc
		case f.Function != "":
			return f.Function
		default:
			return loc
		}
	}
	return e.Logger
}

// serviceName resolves the service the error belongs to: an explicit `server_name`
// tag / the `service_name` tag / the SDK-reported server, defaulting to the SDK name.
func serviceName(e *errortrackingtypes.SentryEvent) string {
	tags := parseTags(e.Tags)
	if v := tags["service_name"]; v != "" {
		return v
	}
	if v := tags["server_name"]; v != "" {
		return v
	}
	if e.ServerName != "" {
		return e.ServerName
	}
	if e.SDK != nil {
		return e.SDK.Name
	}
	return ""
}

// traceContext extracts distributed-trace linkage so an error can be pivoted to
// its trace in the same o11y plane.
func traceContext(contexts map[string]json.RawMessage) (traceID, spanID string) {
	raw, ok := contexts["trace"]
	if !ok {
		return "", ""
	}
	var tc struct {
		TraceID string `json:"trace_id"`
		SpanID  string `json:"span_id"`
	}
	if err := json.Unmarshal(raw, &tc); err != nil {
		return "", ""
	}
	return tc.TraceID, tc.SpanID
}

// parseTimestamp accepts the two shapes the SDKs emit: a unix-seconds number
// (possibly fractional) or an ISO-8601 string. Unparseable → now.
func parseTimestamp(raw json.RawMessage) time.Time {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return time.Now().UTC()
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.999999", "2006-01-02T15:04:05"} {
				if t, err := time.Parse(layout, str); err == nil {
					return t.UTC()
				}
			}
		}
		return time.Now().UTC()
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 {
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		return time.Unix(sec, nsec).UTC()
	}
	return time.Now().UTC()
}

// parseMessage accepts the top-level message as a string or {message,formatted}.
func parseMessage(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if s[0] == '"' {
		var str string
		_ = json.Unmarshal(raw, &str)
		return str
	}
	var m errortrackingtypes.SentryMessage
	if err := json.Unmarshal(raw, &m); err == nil {
		return firstNonEmpty(m.Formatted, m.Message)
	}
	return ""
}

// parseTags accepts the two tag encodings: {k:v} or [[k,v],...]. Values are
// coerced to strings; non-scalar values are dropped.
func parseTags(raw json.RawMessage) map[string]string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return nil
	}
	out := map[string]string{}
	switch s[0] {
	case '{':
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil
		}
		for k, v := range m {
			if sv := scalarString(v); sv != "" {
				out[k] = sv
			}
		}
	case '[':
		var pairs [][]any
		if err := json.Unmarshal(raw, &pairs); err != nil {
			return nil
		}
		for _, p := range pairs {
			if len(p) == 2 {
				if k := scalarString(p[0]); k != "" {
					out[k] = scalarString(p[1])
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func scalarString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case json.Number:
		return x.String()
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func baseName(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
