package o11y

// The LLM-OBSERVABILITY product face — the four gen_ai span views
// (observations, traces, sessions, users), eval scores, human annotations and
// the LLM pricing rules — as TYPED ops.
//
// These were fourteen of the routes that reached traffic only through the
// delegation wildcard, and a route behind a wildcard is in no document: no SDK
// method, no CLI command, no agent tool, no reference page. A customer could
// not reach their own LLM telemetry, nor tune their own token pricing, from
// anything but a hand-written HTTP call. Typing them is what puts the fourteen
// operations in the document and therefore in every projection built from it.
//
// THE WIRE DOES NOT MOVE, and the way it does not move is that these ops do not
// re-implement the reads and writes. Each hands the call to the SAME runtime
// handler the wildcard delegates to (see relay.go), so identity resolution, the
// org gate, the role check (viewer on every read, editor on the score and
// annotation writes, admin on the pricing-rule writes — the exact
// ViewAccess/EditAccess/AdminAccess gates the runtime has always run), the
// mandatory X-Org-Id tenant scoping of the span views, the timeouts and the
// success envelope are all still the runtime's, executed in the order they
// always were. What is new here is the TYPE — the In the caller may send and the
// Out they get back — and the prose that goes with it.
//
// The /llm/ segment keeps each name unambiguous, exactly as it does on the
// runtime's own tree: this family's traces, sessions and users are gen_ai-span
// projections, not the APM trace search, the auth sessions or the IAM users
// that own those words elsewhere under /v1/o11y.
//
// The three writes whose answer is only their status — the score delete, the
// pricing-rule bulk write and the pricing-rule delete — answer the empty 204 a
// nil Out has always meant: relay leaves the Out untouched, and zip writes a
// clean 204 with no body.
//
// The mux registrations these ops mirror STAY (pkg/apiserver/o11yapiserver):
// they are the runtime router relay forwards into, and the only router the
// standalone server has. Registered ahead of the wildcard, and
// specific-beats-wildcard is what the router does regardless of registration
// order, so these fourteen paths dispatch here and every other path under the
// prefix still reaches the runtime untouched.

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/zap-proto/zip"
)

// mountLLMObs registers the fourteen typed LLM-observability ops on the native
// router. They register on o11yRoot — the o11y face's one path root, shared with
// the logs and identity ops — and each op spells its own leaf, so a collection
// route sits beside its parameterised sibling exactly as the runtime's own tree
// has them: the router matches the most specific pattern.
//
// The operation ids are carried over verbatim from the routes' prior OpenAPI
// declarations, so the SDK method, the CLI command and the MCP tool a caller
// already knows keep their names as the routes cross into the document.
func mountLLMObs(app *zip.App) {
	g := under{app, o11yRoot}

	// The four gen_ai span views — projections over spans, not tables.
	zip.Get(g, "/llm/observations", llmObservations, zip.WithOperationID("ListLLMObservations"))
	zip.Get(g, "/llm/traces", llmTraces, zip.WithOperationID("ListLLMTraces"))
	zip.Get(g, "/llm/sessions", llmSessions, zip.WithOperationID("ListLLMSessions"))
	zip.Get(g, "/llm/users", llmUsers, zip.WithOperationID("ListLLMUsers"))

	// Eval scores — CRUD over a net-new table.
	zip.Get(g, "/llm/scores", llmListScores, zip.WithOperationID("ListLLMScores"))
	zip.Post(g, "/llm/scores", llmCreateScore, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateLLMScore"))
	zip.Get(g, "/llm/score/:id", llmGetScore, zip.WithOperationID("GetLLMScore"))
	zip.Delete(g, "/llm/score/:id", llmDeleteScore, zip.WithOperationID("DeleteLLMScore"))

	// Human annotations — a note plus an optional review queue.
	zip.Get(g, "/llm/annotation", llmListAnnotations, zip.WithOperationID("ListLLMAnnotations"))
	zip.Post(g, "/llm/annotation", llmCreateAnnotation, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateLLMAnnotation"))

	// LLM pricing rules — the read, the single bulk write shared by the user and
	// the Zeus sync job, and the per-rule read and delete.
	zip.Get(g, "/llm_pricing_rules", llmListPricingRules, zip.WithOperationID("ListLLMPricingRules"))
	zip.Put(g, "/llm_pricing_rules", llmUpsertPricingRules, zip.WithOperationID("CreateOrUpdateLLMPricingRules"))
	zip.Get(g, "/llm_pricing_rules/:id", llmGetPricingRule, zip.WithOperationID("GetLLMPricingRule"))
	zip.Delete(g, "/llm_pricing_rules/:id", llmDeletePricingRule, zip.WithOperationID("DeleteLLMPricingRule"))
}

// ── the fourteen operations ─────────────────────────────────────────────────────

// llmObservations lists gen_ai spans as LLM observations — each an LLM call with
// its model, token counts, cost and latency projected from gen_ai.* attributes,
// newest first, over the query window.
//
// Callers need the viewer role; the runtime's own gate enforces it, and scopes
// the read to the caller's validated tenant.
func llmObservations(ctx context.Context, in *O11yLLMViewQuery) (*O11yLLMObservationsOut, error) {
	out := new(O11yLLMObservationsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/llm/observations", viewParams(in), nil, out)
}

// llmTraces lists LLM traces — gen_ai spans grouped by trace_id, with cost,
// tokens and latency rolled up across each trace.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func llmTraces(ctx context.Context, in *O11yLLMViewQuery) (*O11yLLMTracesOut, error) {
	out := new(O11yLLMTracesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/llm/traces", viewParams(in), nil, out)
}

// llmSessions lists conversations — gen_ai spans grouped by session.id, with
// their trace and observation counts, tokens and cost.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func llmSessions(ctx context.Context, in *O11yLLMViewQuery) (*O11yLLMSessionsOut, error) {
	out := new(O11yLLMSessionsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/llm/sessions", viewParams(in), nil, out)
}

// llmUsers lists end users — gen_ai spans grouped by user.id, with their
// session, trace and observation counts, tokens and cost.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func llmUsers(ctx context.Context, in *O11yLLMViewQuery) (*O11yLLMUsersOut, error) {
	out := new(O11yLLMUsersOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/llm/users", viewParams(in), nil, out)
}

// llmListScores lists eval scores and human-feedback signals attached to traces
// and observations, newest first.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func llmListScores(ctx context.Context, in *O11yLLMScoresQuery) (*O11yLLMScoresOut, error) {
	out := new(O11yLLMScoresOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/llm/scores", query(
		"traceId", in.TraceID,
		"observationId", in.ObservationID,
		"name", in.Name,
		"source", in.Source,
		"offset", in.Offset,
		"limit", in.Limit,
	), nil, out)
}

// llmCreateScore attaches an eval score or human-feedback signal to a trace or a
// single observation.
//
// Callers need the editor role; the runtime's own gate enforces it, and it
// validates the payload and stamps the score's author and org.
func llmCreateScore(ctx context.Context, in *O11yLLMIngestScore) (*O11yLLMScoreOut, error) {
	out := new(O11yLLMScoreOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/llm/scores", nil, in, out)
}

// llmGetScore returns a single score by id.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func llmGetScore(ctx context.Context, in *O11yLLMScoreRef) (*O11yLLMScoreOut, error) {
	out := new(O11yLLMScoreOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/llm/score/"+in.ID, nil, nil, out)
}

// llmDeleteScore hard-deletes a score by id.
//
// Callers need the editor role; the runtime's own gate enforces it.
func llmDeleteScore(ctx context.Context, in *O11yLLMScoreRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/llm/score/"+in.ID, nil, nil, nil)
}

// llmListAnnotations lists human annotations on traces and observations,
// optionally scoped to one review queue.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func llmListAnnotations(ctx context.Context, in *O11yLLMAnnotationsQuery) (*O11yLLMAnnotationsOut, error) {
	out := new(O11yLLMAnnotationsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/llm/annotation", query(
		"traceId", in.TraceID,
		"queue", in.Queue,
		"status", in.Status,
		"offset", in.Offset,
		"limit", in.Limit,
	), nil, out)
}

// llmCreateAnnotation adds a human annotation to a trace or observation,
// optionally in a review queue.
//
// Callers need the editor role; the runtime's own gate enforces it, and it
// validates the payload and stamps the annotation's author and org.
func llmCreateAnnotation(ctx context.Context, in *O11yLLMIngestAnnotation) (*O11yLLMAnnotationOut, error) {
	out := new(O11yLLMAnnotationOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/llm/annotation", nil, in, out)
}

// llmListPricingRules returns the LLM pricing rules for the caller's org, with
// pagination and an optional search and override filter.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func llmListPricingRules(ctx context.Context, in *O11yLLMPricingRulesQuery) (*O11yLLMPricingRulesOut, error) {
	out := new(O11yLLMPricingRulesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/llm_pricing_rules", query(
		"q", in.Search,
		"isOverride", in.IsOverride,
		"offset", in.Offset,
		"limit", in.Limit,
	), nil, out)
}

// llmUpsertPricingRules writes the pricing-rule batch — the single write
// endpoint used by both the user and the Zeus sync job. Per-rule match is by id,
// then sourceId, then insert; an override row is fully preserved when the
// request omits isOverride, only its synced_at stamped.
//
// Callers need the admin role; the runtime's own gate enforces it.
func llmUpsertPricingRules(ctx context.Context, in *O11yLLMUpdatablePricingRules) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/llm_pricing_rules", nil, in, nil)
}

// llmGetPricingRule returns a single LLM pricing rule by id.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func llmGetPricingRule(ctx context.Context, in *O11yLLMPricingRuleRef) (*O11yLLMPricingRuleOut, error) {
	out := new(O11yLLMPricingRuleOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/llm_pricing_rules/"+in.ID, nil, nil, out)
}

// llmDeletePricingRule hard-deletes a pricing rule by id. If the rule was
// auto-synced, the next sync cycle recreates it.
//
// Callers need the admin role; the runtime's own gate enforces it.
func llmDeletePricingRule(ctx context.Context, in *O11yLLMPricingRuleRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/llm_pricing_rules/"+in.ID, nil, nil, nil)
}

// viewParams renders the shared span-view filter onto the wire — the ONE place
// the four views' query is encoded, so they cannot drift. Start and End are
// unix-millisecond epochs; a non-positive value is left off, and the runtime
// reads an absent window as the last 24h. The tenant is NOT among these: it is
// the validated X-Org-Id relay forwards, never a caller-supplied param.
func viewParams(in *O11yLLMViewQuery) url.Values {
	params := query(
		"traceId", in.TraceID,
		"sessionId", in.SessionID,
		"userId", in.UserID,
		"name", in.Name,
		"model", in.Model,
		"offset", in.Offset,
		"limit", in.Limit,
	)
	if in.Start > 0 {
		params.Set("start", strconv.FormatInt(in.Start, 10))
	}
	if in.End > 0 {
		params.Set("end", strconv.FormatInt(in.End, 10))
	}
	return params
}

// ── inputs ──────────────────────────────────────────────────────────────────────
//
// Every type here carries the O11yLLM qualifier because these names enter a
// fleet-wide document, where one name may have exactly one shape.

// O11yLLMViewQuery is the shared filter for the four gen_ai span views. A zero
// window defaults to the last 24h.
type O11yLLMViewQuery struct {
	// Start is the start of the window as a unix-millisecond epoch. Zero means
	// 24h before the end.
	Start int64 `json:"start,omitempty"`
	// End is the end of the window as a unix-millisecond epoch. Zero means now.
	End int64 `json:"end,omitempty"`
	// TraceID narrows the view to one trace.
	TraceID string `json:"traceId,omitempty"`
	// SessionID narrows the view to one conversation.
	SessionID string `json:"sessionId,omitempty"`
	// UserID narrows the view to one end user.
	UserID string `json:"userId,omitempty"`
	// Name narrows the view to observations of one name.
	Name string `json:"name,omitempty"`
	// Model narrows the view to one model.
	Model string `json:"model,omitempty"`
	// Offset is how many rows to skip, for paging.
	Offset int `json:"offset,omitempty"`
	// Limit caps how many rows come back.
	Limit int `json:"limit,omitempty"`
}

// O11yLLMScoresQuery is the filter for the scores list.
type O11yLLMScoresQuery struct {
	// TraceID narrows to scores on one trace.
	TraceID string `json:"traceId,omitempty"`
	// ObservationID narrows to scores on one observation.
	ObservationID string `json:"observationId,omitempty"`
	// Name narrows to scores of one name.
	Name string `json:"name,omitempty"`
	// Source narrows to scores from one source, e.g. API, EVAL.
	Source string `json:"source,omitempty"`
	// Offset is how many rows to skip, for paging.
	Offset int `json:"offset,omitempty"`
	// Limit caps how many rows come back.
	Limit int `json:"limit,omitempty"`
}

// O11yLLMScoreRef names one score by id.
type O11yLLMScoreRef struct {
	// ID is the score's id.
	ID string `json:"-" url:"id" validate:"required"`
}

// O11yLLMIngestScore is the create payload for a score.
type O11yLLMIngestScore struct {
	// TraceID is the trace the score attaches to. Required.
	TraceID string `json:"traceId"`
	// ObservationID is the single observation the score attaches to, when
	// narrowed to one.
	ObservationID string `json:"observationId,omitempty"`
	// Name is the score's name, e.g. helpfulness. Required.
	Name string `json:"name"`
	// Value is the numeric score.
	Value float64 `json:"value"`
	// StringValue is the categorical score, when the score is categorical.
	StringValue string `json:"stringValue,omitempty"`
	// DataType is the score's kind — NUMERIC or CATEGORICAL. Defaults from the
	// value when empty.
	DataType string `json:"dataType,omitempty"`
	// Comment is a free-text note.
	Comment string `json:"comment,omitempty"`
	// Source is where the score came from, e.g. API, EVAL. Defaults to API.
	Source string `json:"source,omitempty"`
}

// O11yLLMAnnotationsQuery is the filter for the annotations list.
type O11yLLMAnnotationsQuery struct {
	// TraceID narrows to annotations on one trace.
	TraceID string `json:"traceId,omitempty"`
	// Queue narrows to one review queue.
	Queue string `json:"queue,omitempty"`
	// Status narrows to one review status, e.g. PENDING.
	Status string `json:"status,omitempty"`
	// Offset is how many rows to skip, for paging.
	Offset int `json:"offset,omitempty"`
	// Limit caps how many rows come back.
	Limit int `json:"limit,omitempty"`
}

// O11yLLMIngestAnnotation is the create payload for an annotation.
type O11yLLMIngestAnnotation struct {
	// TraceID is the trace the annotation attaches to. Required.
	TraceID string `json:"traceId"`
	// ObservationID is the single observation the annotation attaches to, when
	// narrowed to one.
	ObservationID string `json:"observationId,omitempty"`
	// Queue is the review queue to file the annotation in.
	Queue string `json:"queue,omitempty"`
	// Content is the note itself. Required.
	Content string `json:"content"`
	// Status is the annotation's initial review status. Defaults to PENDING.
	Status string `json:"status,omitempty"`
}

// O11yLLMPricingRulesQuery is the filter for the pricing-rules list.
type O11yLLMPricingRulesQuery struct {
	// Search matches rules by model or provider.
	Search string `json:"q,omitempty"`
	// IsOverride, when "true" or "false", narrows to user-pinned rules or to
	// synced ones; empty returns both. It is a string because a query param is a
	// string on the wire, and the runtime reads absent as "no filter".
	IsOverride string `json:"isOverride,omitempty"`
	// Offset is how many rows to skip, for paging.
	Offset int `json:"offset,omitempty"`
	// Limit caps how many rows come back.
	Limit int `json:"limit,omitempty"`
}

// O11yLLMPricingRuleRef names one pricing rule by id.
type O11yLLMPricingRuleRef struct {
	// ID is the rule's id.
	ID string `json:"-" url:"id" validate:"required"`
}

// O11yLLMUpdatablePricingRules is the bulk upsert batch.
type O11yLLMUpdatablePricingRules struct {
	// Rules are the rules to create or update, matched per rule.
	Rules []O11yLLMUpdatablePricingRule `json:"rules"`
}

// O11yLLMUpdatablePricingRule is one entry of the bulk upsert. ID set matches by
// id; else SourceID set matches by source; else it inserts a new custom rule.
// IsOverride is a pointer so "not sent" is distinct from "set to false".
type O11yLLMUpdatablePricingRule struct {
	// ID matches an existing rule by its id.
	ID *string `json:"id,omitempty"`
	// SourceID matches an existing rule by its upstream source id.
	SourceID *string `json:"sourceId,omitempty"`
	// Model is the model the rule prices. Required.
	Model string `json:"modelName"`
	// Provider is the model's provider. Required.
	Provider string `json:"provider"`
	// ModelPattern are the model-name globs the rule matches. Required.
	ModelPattern []string `json:"modelPattern"`
	// Unit is the pricing unit, e.g. per_million_tokens. Required.
	Unit string `json:"unit"`
	// Pricing is the per-unit cost. Required.
	Pricing O11yLLMRulePricing `json:"pricing"`
	// IsOverride pins the rule so the sync job skips it. Omit to leave a matched
	// override untouched.
	IsOverride *bool `json:"isOverride,omitempty"`
	// Enabled turns the rule on.
	Enabled bool `json:"enabled"`
}

// ── outputs ─────────────────────────────────────────────────────────────────────
//
// Each Out is the runtime's answer NAMED, field for field, in the same
// status/data envelope render.Success has always written — so re-encoding the
// decoded Out reproduces the runtime's own bytes. Nothing embeds: the schema
// builder walks reflected fields and cannot name an embedded one, so the audit
// columns the runtime carries by embedding (id, createdAt, updatedAt, createdBy,
// updatedBy) are written flat here, in the order they marshal.

// O11yLLMObservationsOut is a page of LLM observations.
type O11yLLMObservationsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the page.
	Data O11yLLMObservationsPage `json:"data,omitempty"`
}

// O11yLLMObservationsPage is one page of observations.
type O11yLLMObservationsPage struct {
	// Items are the observations, newest first.
	Items []O11yLLMObservation `json:"items"`
	// Offset is the row offset this page started at.
	Offset int `json:"offset"`
	// Limit is the page cap the read ran with.
	Limit int `json:"limit"`
}

// O11yLLMObservation is one gen_ai span rendered as an LLM observation.
type O11yLLMObservation struct {
	// ID is the observation's id (the span id).
	ID string `json:"id"`
	// TraceID is the trace the observation belongs to.
	TraceID string `json:"traceId"`
	// ParentID is the parent observation, when the span has one.
	ParentID string `json:"parentObservationId,omitempty"`
	// Type is the observation kind, e.g. chat, embeddings, tool.
	Type string `json:"type"`
	// Name is the observation's name.
	Name string `json:"name"`
	// StartTime is when the observation started.
	StartTime time.Time `json:"startTime"`
	// LatencyMs is how long it took, in milliseconds.
	LatencyMs float64 `json:"latencyMs"`
	// Model is the model that served it.
	Model string `json:"model,omitempty"`
	// Provider is the model's provider.
	Provider string `json:"provider,omitempty"`
	// PromptTokens is the input token count.
	PromptTokens int64 `json:"promptTokens"`
	// CompletionTokens is the output token count.
	CompletionTokens int64 `json:"completionTokens"`
	// TotalTokens is the sum of prompt and completion tokens.
	TotalTokens int64 `json:"totalTokens"`
	// TotalCost is the observation's cost.
	TotalCost float64 `json:"totalCost"`
	// SessionID is the conversation the observation belongs to.
	SessionID string `json:"sessionId,omitempty"`
	// UserID is the end user the observation is attributed to.
	UserID string `json:"userId,omitempty"`
	// ServiceName is the app that emitted it.
	ServiceName string `json:"serviceName,omitempty"`
	// StatusCode is the observation's status, e.g. OK, ERROR.
	StatusCode string `json:"statusCode,omitempty"`
}

// O11yLLMTracesOut is a page of LLM traces.
type O11yLLMTracesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the page.
	Data O11yLLMTracesPage `json:"data,omitempty"`
}

// O11yLLMTracesPage is one page of traces.
type O11yLLMTracesPage struct {
	// Items are the traces, newest first.
	Items []O11yLLMTrace `json:"items"`
	// Offset is the row offset this page started at.
	Offset int `json:"offset"`
	// Limit is the page cap the read ran with.
	Limit int `json:"limit"`
}

// O11yLLMTrace is one gen_ai trace — its observations rolled up.
type O11yLLMTrace struct {
	// ID is the trace id.
	ID string `json:"id"`
	// SessionID is the conversation the trace belongs to.
	SessionID string `json:"sessionId,omitempty"`
	// UserID is the end user the trace is attributed to.
	UserID string `json:"userId,omitempty"`
	// ServiceName is the app that emitted it.
	ServiceName string `json:"serviceName,omitempty"`
	// Observations is how many observations the trace holds.
	Observations int64 `json:"observations"`
	// PromptTokens is the trace's total input tokens.
	PromptTokens int64 `json:"promptTokens"`
	// CompletionTokens is the trace's total output tokens.
	CompletionTokens int64 `json:"completionTokens"`
	// TotalTokens is the trace's total tokens.
	TotalTokens int64 `json:"totalTokens"`
	// TotalCost is the trace's total cost.
	TotalCost float64 `json:"totalCost"`
	// LatencyMs is the trace's span, in milliseconds.
	LatencyMs float64 `json:"latencyMs"`
}

// O11yLLMSessionsOut is a page of conversations.
type O11yLLMSessionsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the page.
	Data O11yLLMSessionsPage `json:"data,omitempty"`
}

// O11yLLMSessionsPage is one page of conversations.
type O11yLLMSessionsPage struct {
	// Items are the conversations, newest first.
	Items []O11yLLMSession `json:"items"`
	// Offset is the row offset this page started at.
	Offset int `json:"offset"`
	// Limit is the page cap the read ran with.
	Limit int `json:"limit"`
}

// O11yLLMSession is one conversation — all traces sharing a session.id.
type O11yLLMSession struct {
	// ID is the session id.
	ID string `json:"id"`
	// UserID is the end user the conversation is attributed to.
	UserID string `json:"userId,omitempty"`
	// Traces is how many traces the conversation holds.
	Traces int64 `json:"traces"`
	// Observations is how many observations the conversation holds.
	Observations int64 `json:"observations"`
	// PromptTokens is the conversation's total input tokens.
	PromptTokens int64 `json:"promptTokens"`
	// CompletionTokens is the conversation's total output tokens.
	CompletionTokens int64 `json:"completionTokens"`
	// TotalTokens is the conversation's total tokens.
	TotalTokens int64 `json:"totalTokens"`
	// TotalCost is the conversation's total cost.
	TotalCost float64 `json:"totalCost"`
}

// O11yLLMUsersOut is a page of end users.
type O11yLLMUsersOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the page.
	Data O11yLLMUsersPage `json:"data,omitempty"`
}

// O11yLLMUsersPage is one page of end users.
type O11yLLMUsersPage struct {
	// Items are the end users, newest first.
	Items []O11yLLMUser `json:"items"`
	// Offset is the row offset this page started at.
	Offset int `json:"offset"`
	// Limit is the page cap the read ran with.
	Limit int `json:"limit"`
}

// O11yLLMUser is one end user — all their sessions, traces and observations.
type O11yLLMUser struct {
	// ID is the end user's id (user.id).
	ID string `json:"id"`
	// Sessions is how many conversations they had.
	Sessions int64 `json:"sessions"`
	// Traces is how many traces they produced.
	Traces int64 `json:"traces"`
	// Observations is how many observations they produced.
	Observations int64 `json:"observations"`
	// PromptTokens is their total input tokens.
	PromptTokens int64 `json:"promptTokens"`
	// CompletionTokens is their total output tokens.
	CompletionTokens int64 `json:"completionTokens"`
	// TotalTokens is their total tokens.
	TotalTokens int64 `json:"totalTokens"`
	// TotalCost is their total cost.
	TotalCost float64 `json:"totalCost"`
}

// O11yLLMScoresOut is a page of scores.
type O11yLLMScoresOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the page.
	Data O11yLLMScoresPage `json:"data,omitempty"`
}

// O11yLLMScoresPage is one page of scores.
type O11yLLMScoresPage struct {
	// Items are the scores, newest first.
	Items []O11yLLMScore `json:"items"`
	// Total is how many scores match, across all pages.
	Total int `json:"total"`
	// Offset is the row offset this page started at.
	Offset int `json:"offset"`
	// Limit is the page cap the read ran with.
	Limit int `json:"limit"`
}

// O11yLLMScoreOut is a single score — the read and the create both answer with
// one, in the status/data envelope.
type O11yLLMScoreOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the score.
	Data O11yLLMScore `json:"data,omitempty"`
}

// O11yLLMScore is an eval score or human-feedback signal on a trace or
// observation.
type O11yLLMScore struct {
	// ID is the score's id.
	ID string `json:"id"`
	// CreatedAt is when the score was stored.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the score last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// TraceID is the trace the score attaches to.
	TraceID string `json:"traceId"`
	// ObservationID is the observation the score attaches to, when narrowed.
	ObservationID string `json:"observationId,omitempty"`
	// Name is the score's name.
	Name string `json:"name"`
	// Value is the numeric score.
	Value float64 `json:"value"`
	// StringValue is the categorical score, when the score is categorical.
	StringValue string `json:"stringValue,omitempty"`
	// DataType is the score's kind — NUMERIC or CATEGORICAL.
	DataType string `json:"dataType"`
	// Comment is a free-text note.
	Comment string `json:"comment,omitempty"`
	// Source is where the score came from, e.g. API, EVAL.
	Source string `json:"source"`
	// Timestamp is the score's own event time.
	Timestamp time.Time `json:"timestamp"`
	// CreatedBy is who created the score.
	CreatedBy string `json:"createdBy,omitempty"`
}

// O11yLLMAnnotationsOut is a page of annotations.
type O11yLLMAnnotationsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the page.
	Data O11yLLMAnnotationsPage `json:"data,omitempty"`
}

// O11yLLMAnnotationsPage is one page of annotations.
type O11yLLMAnnotationsPage struct {
	// Items are the annotations, newest first.
	Items []O11yLLMAnnotation `json:"items"`
	// Total is how many annotations match, across all pages.
	Total int `json:"total"`
	// Offset is the row offset this page started at.
	Offset int `json:"offset"`
	// Limit is the page cap the read ran with.
	Limit int `json:"limit"`
}

// O11yLLMAnnotationOut is a single annotation — the create answers with one, in
// the status/data envelope.
type O11yLLMAnnotationOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the annotation.
	Data O11yLLMAnnotation `json:"data,omitempty"`
}

// O11yLLMAnnotation is a human note on a trace or observation.
type O11yLLMAnnotation struct {
	// ID is the annotation's id.
	ID string `json:"id"`
	// CreatedAt is when the annotation was stored.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the annotation last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// TraceID is the trace the annotation attaches to.
	TraceID string `json:"traceId"`
	// ObservationID is the observation the annotation attaches to, when narrowed.
	ObservationID string `json:"observationId,omitempty"`
	// Queue is the review queue the annotation sits in, when queued.
	Queue string `json:"queue,omitempty"`
	// Content is the note itself.
	Content string `json:"content"`
	// Status is the annotation's review status, e.g. PENDING.
	Status string `json:"status"`
	// Author is who wrote it.
	Author string `json:"author,omitempty"`
}

// O11yLLMPricingRulesOut is a page of pricing rules.
type O11yLLMPricingRulesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the page.
	Data O11yLLMPricingRulesPage `json:"data,omitempty"`
}

// O11yLLMPricingRulesPage is one page of pricing rules.
type O11yLLMPricingRulesPage struct {
	// Items are the rules.
	Items []O11yLLMPricingRule `json:"items"`
	// Total is how many rules match, across all pages.
	Total int `json:"total"`
	// Offset is the row offset this page started at.
	Offset int `json:"offset"`
	// Limit is the page cap the read ran with.
	Limit int `json:"limit"`
}

// O11yLLMPricingRuleOut is a single pricing rule, in the status/data envelope.
type O11yLLMPricingRuleOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the rule.
	Data O11yLLMPricingRule `json:"data,omitempty"`
}

// O11yLLMPricingRule is one LLM pricing rule — how one model's tokens are
// costed.
type O11yLLMPricingRule struct {
	// ID is the rule's id.
	ID string `json:"id"`
	// CreatedAt is when the rule was stored.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the rule last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// CreatedBy is who created the rule.
	CreatedBy string `json:"createdBy"`
	// UpdatedBy is who last changed it.
	UpdatedBy string `json:"updatedBy"`
	// OrgID is the org the rule belongs to.
	OrgID string `json:"orgId"`
	// SourceID is the upstream source the rule was synced from, when synced.
	SourceID string `json:"sourceId,omitempty"`
	// Model is the model the rule prices.
	Model string `json:"modelName"`
	// Provider is the model's provider.
	Provider string `json:"provider"`
	// ModelPattern are the model-name globs the rule matches.
	ModelPattern []string `json:"modelPattern"`
	// Unit is the pricing unit, e.g. per_million_tokens.
	Unit string `json:"unit"`
	// Pricing is the per-unit cost.
	Pricing O11yLLMRulePricing `json:"pricing"`
	// IsOverride marks the rule user-pinned; when true the sync job skips it.
	IsOverride bool `json:"isOverride"`
	// SyncedAt is when the rule was last synced, when it is synced.
	SyncedAt *time.Time `json:"syncedAt,omitempty"`
	// Enabled says whether the rule is on.
	Enabled bool `json:"enabled"`
}

// O11yLLMRulePricing is a rule's per-unit cost.
type O11yLLMRulePricing struct {
	// Input is the cost per unit of input tokens.
	Input float64 `json:"input"`
	// Output is the cost per unit of output tokens.
	Output float64 `json:"output"`
	// Cache is the cost of cached tokens, when the model prices them.
	Cache *O11yLLMPricingCacheCosts `json:"cache,omitempty"`
}

// O11yLLMPricingCacheCosts is the cost of a model's cached tokens.
type O11yLLMPricingCacheCosts struct {
	// Mode is how cached tokens are counted — subtract (inside input_tokens,
	// OpenAI-style), additive (reported separately, Anthropic-style) or unknown.
	Mode string `json:"mode"`
	// Read is the cost per unit of cache-read tokens.
	Read float64 `json:"read"`
	// Write is the cost per unit of cache-write tokens.
	Write float64 `json:"write"`
}
