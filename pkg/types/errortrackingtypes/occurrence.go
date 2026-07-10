package errortrackingtypes

import "time"

// Occurrence is a single normalized error event (one exception instance). It is
// derived from a Sentry event (envelope / legacy store item) or, later, from an
// OTel exception span-event. It is the OTel-shaped occurrence the shim persists
// to o11y_logs (the reused occurrence store) and the "latest event" sample kept
// on the issue for the detail view. Purely a value — no store, no tags of its own.
type Occurrence struct {
	EventID     string            `json:"eventId"`
	Fingerprint string            `json:"fingerprint"`
	Type        string            `json:"type"`
	Value       string            `json:"value"`
	Culprit     string            `json:"culprit"`
	Level       string            `json:"level"`
	Platform    string            `json:"platform,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	Environment string            `json:"environment,omitempty"`
	Release     string            `json:"release,omitempty"`
	ServiceName string            `json:"serviceName,omitempty"`
	ServerName  string            `json:"serverName,omitempty"`
	Transaction string            `json:"transaction,omitempty"`
	TraceID     string            `json:"traceId,omitempty"`
	SpanID      string            `json:"spanId,omitempty"`
	Frames      []Frame           `json:"frames,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	User        *EventUser        `json:"user,omitempty"`
}

// Frame is a single stack frame, normalized from the Sentry frame shape. Frames
// are stored innermost-last (crash site last), matching the Sentry convention.
type Frame struct {
	Function string `json:"function,omitempty"`
	Module   string `json:"module,omitempty"`
	Filename string `json:"filename,omitempty"`
	AbsPath  string `json:"absPath,omitempty"`
	Lineno   int    `json:"lineno,omitempty"`
	Colno    int    `json:"colno,omitempty"`
	InApp    bool   `json:"inApp"`
}

// EventUser is the reporting end-user context (used for "users affected"); PII is
// the caller's responsibility — we store only what the SDK sent.
type EventUser struct {
	ID       string `json:"id,omitempty"`
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
	IP       string `json:"ipAddress,omitempty"`
}
