package llmobstypes

import "time"

// The four view types below are NOT tables — they are projections computed on
// the fly over gen_ai spans via the querier. Observations are raw spans; traces,
// sessions and users are span aggregations grouped by trace_id / session.id /
// user.id respectively.

// Observation is a single gen_ai span rendered as an LLM observation.
type Observation struct {
	ID               string    `json:"id"`
	TraceID          string    `json:"traceId"`
	ParentID         string    `json:"parentObservationId,omitempty"`
	Type             string    `json:"type"`
	Name             string    `json:"name"`
	StartTime        time.Time `json:"startTime"`
	LatencyMs        float64   `json:"latencyMs"`
	Model            string    `json:"model,omitempty"`
	Provider         string    `json:"provider,omitempty"`
	PromptTokens     int64     `json:"promptTokens"`
	CompletionTokens int64     `json:"completionTokens"`
	TotalTokens      int64     `json:"totalTokens"`
	TotalCost        float64   `json:"totalCost"`
	SessionID        string    `json:"sessionId,omitempty"`
	UserID           string    `json:"userId,omitempty"`
	ServiceName      string    `json:"serviceName,omitempty"`
	StatusCode       string    `json:"statusCode,omitempty"`
}

// Trace is one gen_ai trace (all its observations rolled up).
type Trace struct {
	ID               string  `json:"id"`
	SessionID        string  `json:"sessionId,omitempty"`
	UserID           string  `json:"userId,omitempty"`
	ServiceName      string  `json:"serviceName,omitempty"`
	Observations     int64   `json:"observations"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	TotalTokens      int64   `json:"totalTokens"`
	TotalCost        float64 `json:"totalCost"`
	LatencyMs        float64 `json:"latencyMs"`
}

// Session is a conversation (all traces/observations sharing a session.id).
type Session struct {
	ID               string  `json:"id"`
	UserID           string  `json:"userId,omitempty"`
	Traces           int64   `json:"traces"`
	Observations     int64   `json:"observations"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	TotalTokens      int64   `json:"totalTokens"`
	TotalCost        float64 `json:"totalCost"`
}

// User is one end user (all their sessions/traces/observations).
type User struct {
	ID               string  `json:"id"`
	Sessions         int64   `json:"sessions"`
	Traces           int64   `json:"traces"`
	Observations     int64   `json:"observations"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	TotalTokens      int64   `json:"totalTokens"`
	TotalCost        float64 `json:"totalCost"`
}

// ViewQuery is the shared filter for the four span-view endpoints. Start/End
// are unix-millisecond epochs; a zero window defaults to the last 24h.
type ViewQuery struct {
	Start     int64  `query:"start" json:"start"`
	End       int64  `query:"end" json:"end"`
	TraceID   string `query:"traceId" json:"traceId"`
	SessionID string `query:"sessionId" json:"sessionId"`
	UserID    string `query:"userId" json:"userId"`
	Name      string `query:"name" json:"name"`
	Model     string `query:"model" json:"model"`
	Offset    int    `query:"offset" json:"offset"`
	Limit     int    `query:"limit" json:"limit"`

	// OrgSlug is the TENANT the query is scoped to — the validated X-Org-Id the
	// gateway asserted, matching the gen_ai.hanzo.org_id the ai emit path tags on
	// every span. It has NO `query` tag on purpose: it is server-set by the handler
	// from the validated identity AFTER binding, never populated from client input.
	// It is a MANDATORY, non-empty equality predicate on every span-view query —
	// the sole tenant boundary for Datastore telemetry (the span views carry no
	// other org column), so an empty value must fail closed, never read all orgs.
	OrgSlug string `json:"-"`
}

type GettableObservations struct {
	Items  []*Observation `json:"items" required:"true"`
	Offset int            `json:"offset" required:"true"`
	Limit  int            `json:"limit" required:"true"`
}

type GettableTraces struct {
	Items  []*Trace `json:"items" required:"true"`
	Offset int      `json:"offset" required:"true"`
	Limit  int      `json:"limit" required:"true"`
}

type GettableSessions struct {
	Items  []*Session `json:"items" required:"true"`
	Offset int        `json:"offset" required:"true"`
	Limit  int        `json:"limit" required:"true"`
}

type GettableUsers struct {
	Items  []*User `json:"items" required:"true"`
	Offset int     `json:"offset" required:"true"`
	Limit  int     `json:"limit" required:"true"`
}
