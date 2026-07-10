package errortrackingtypes

import "encoding/json"

// The types below are the subset of the Sentry SDK wire payload the shim consumes:
// the JSON of a legacy `/store/` body and of an `event`-type item inside an
// `/envelope/`. They are a from-scratch reimplementation of the PUBLIC, documented
// Sentry ingest protocol (develop.sentry.dev) — no upstream (FSL-licensed) code is
// used. Only the fields error-grouping needs are modeled; everything else is ignored.

// SentryEvent is one error event as sent by any Sentry SDK.
type SentryEvent struct {
	EventID     string                     `json:"event_id"`
	Timestamp   json.RawMessage            `json:"timestamp"` // unix-seconds number OR ISO-8601 string
	Platform    string                     `json:"platform"`
	Level       string                     `json:"level"`
	Logger      string                     `json:"logger"`
	ServerName  string                     `json:"server_name"`
	Environment string                     `json:"environment"`
	Release     string                     `json:"release"`
	Transaction string                     `json:"transaction"`
	Fingerprint []string                   `json:"fingerprint"`
	Message     json.RawMessage            `json:"message"` // string OR {message,formatted,params}
	Exception   *SentryException           `json:"exception"`
	Tags        json.RawMessage            `json:"tags"` // {k:v} OR [[k,v],...]
	User        *SentryUser                `json:"user"`
	Contexts    map[string]json.RawMessage `json:"contexts"`
	SDK         *SentrySDK                 `json:"sdk"`
}

// SentryException wraps one or more exception values (chained exceptions). The
// LAST value is the primary/thrown exception.
type SentryException struct {
	Values []SentryExceptionValue `json:"values"`
}

type SentryExceptionValue struct {
	Type       string            `json:"type"`
	Value      string            `json:"value"`
	Module     string            `json:"module"`
	Stacktrace *SentryStacktrace `json:"stacktrace"`
}

// SentryStacktrace lists frames oldest-first; the crashing frame is last.
type SentryStacktrace struct {
	Frames []SentryFrame `json:"frames"`
}

type SentryFrame struct {
	Filename string `json:"filename"`
	Function string `json:"function"`
	Module   string `json:"module"`
	AbsPath  string `json:"abs_path"`
	Lineno   int    `json:"lineno"`
	Colno    int    `json:"colno"`
	InApp    *bool  `json:"in_app"`
}

type SentryUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	IPAddress string `json:"ip_address"`
}

type SentrySDK struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// SentryMessage is the object form of the top-level `message` field.
type SentryMessage struct {
	Message   string `json:"message"`
	Formatted string `json:"formatted"`
}

// EnvelopeHeader is the first line of a Sentry envelope. Only DSN is load-bearing
// (a fallback source for the ingest key when the X-Sentry-Auth header is absent).
type EnvelopeHeader struct {
	EventID string `json:"event_id"`
	DSN     string `json:"dsn"`
	SentAt  string `json:"sent_at"`
}

// EnvelopeItemHeader precedes each envelope item; Type selects the item and Length
// (when present) frames a binary/opaque payload exactly.
type EnvelopeItemHeader struct {
	Type        string `json:"type"`
	Length      *int   `json:"length"`
	ContentType string `json:"content_type"`
}
