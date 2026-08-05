package o11y

// The TRACE-FUNNELS product face — the funnel definitions themselves, and the
// twelve analytics reads over them — as TYPED ops.
//
// These were eighteen of the routes that reached traffic only through the
// delegation wildcard, and a route behind a wildcard is in no document: no SDK
// method, no CLI command, no agent tool, no reference page. Typing them is what
// puts the eighteen operations in the document and therefore in every projection
// built from it.
//
// TWO ANALYTICS FAMILIES, and they are not duplicates of each other. Six routes
// analyse a SAVED funnel, named by its id in the path, with only a window in the
// body. Six more analyse an AD-HOC funnel the caller describes inline — the
// steps travel in the body and nothing is stored — which is what the funnel
// builder calls while a funnel is still being drafted. They compute the same six
// things, so their answers share a type, and their inputs differ exactly where
// the funnel comes from. The runtime spells them as separate handler pairs
// (handleX / handleXWithPayload); here they are separate ops for the same
// reason, and the naming says which is which.
//
// THE WIRE DOES NOT MOVE, and the way it does not move is that these ops do not
// re-implement anything. Each hands the call to the SAME runtime handler the
// wildcard delegates to (relay.go), so identity resolution, the org gate, the
// role check (viewer on the reads, editor on the writes) and the success
// envelope are all still the runtime's, executed in the order they always were.
// What is new here is the TYPE and the prose that goes with it.

//go:generate go run github.com/zap-proto/zip/cmd/zipdoc

import (
	"context"

	tf "github.com/hanzoai/o11y/pkg/types/tracefunneltypes"
	"github.com/zap-proto/zip"
)

// mountTraceFunnels registers the eighteen typed trace-funnel ops. The literal
// /list, /new, /steps/update and /analytics/* routes register ahead of the
// parameterised /:funnel_id ones so a funnel id can never shadow them — the same
// discipline the mux tree keeps by registering the literals first.
func mountTraceFunnels(app *zip.App) {
	g := under{app, o11yRoot + "/trace-funnels"}

	opPost(g, "/new", funnelCreate, op("CreateTraceFunnel"))
	opGet(g, "/list", funnelList, op("ListTraceFunnels"))
	opPut(g, "/steps/update", funnelStepsUpdate, op("UpdateTraceFunnelSteps"))

	// The ad-hoc family: the funnel travels in the body, nothing is stored.
	opPost(g, "/analytics/validate", draftFunnelValidate, op("ValidateDraftFunnelTraces"))
	opPost(g, "/analytics/overview", draftFunnelOverview, op("GetDraftFunnelOverview"))
	opPost(g, "/analytics/steps", draftFunnelSteps, op("GetDraftFunnelStepMetrics"))
	opPost(g, "/analytics/steps/overview", draftFunnelStepOverview, op("GetDraftFunnelStepOverview"))
	opPost(g, "/analytics/slow-traces", draftFunnelSlowTraces, op("GetDraftFunnelSlowTraces"))
	opPost(g, "/analytics/error-traces", draftFunnelErrorTraces, op("GetDraftFunnelErrorTraces"))

	opGet(g, "/:funnel_id", funnelGet, op("GetTraceFunnel"))
	opPut(g, "/:funnel_id", funnelUpdate, op("UpdateTraceFunnel"))
	opDelete(g, "/:funnel_id", funnelDelete, op("DeleteTraceFunnel"))

	// The saved family: the funnel is named by the path, the body is the window.
	opPost(g, "/:funnel_id/analytics/validate", funnelValidate, op("ValidateTraceFunnelTraces"))
	opPost(g, "/:funnel_id/analytics/overview", funnelOverview, op("GetTraceFunnelOverview"))
	opPost(g, "/:funnel_id/analytics/steps", funnelSteps, op("GetTraceFunnelStepMetrics"))
	opPost(g, "/:funnel_id/analytics/steps/overview", funnelStepOverview, op("GetTraceFunnelStepOverview"))
	opPost(g, "/:funnel_id/analytics/slow-traces", funnelSlowTraces, op("GetTraceFunnelSlowTraces"))
	opPost(g, "/:funnel_id/analytics/error-traces", funnelErrorTraces, op("GetTraceFunnelErrorTraces"))
}

// ── the funnel itself ─────────────────────────────────────────────────────────

// funnelCreate creates an empty funnel with a name, answering the funnel it
// created. Steps are added afterwards with the steps update.
//
// Callers need the editor role; the runtime's own gate enforces it.
func funnelCreate(ctx context.Context, in *O11yFunnelCreateIn) (*O11yFunnelOut, error) {
	out := new(O11yFunnelOut)
	return out, relay(ctx, nil, in, out)
}

// funnelList lists the caller's org's funnels, each with its steps and who last
// touched it.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func funnelList(ctx context.Context, _ *struct{}) (*O11yFunnelsOut, error) {
	out := new(O11yFunnelsOut)
	return out, relay(ctx, nil, nil, out)
}

// funnelStepsUpdate replaces a funnel's steps — the funnel is named in the body
// rather than the path — and answers the funnel as it now stands. A name or
// description sent alongside is applied too; an empty one leaves it as it was.
//
// Callers need the editor role; the runtime's own gate enforces it.
func funnelStepsUpdate(ctx context.Context, in *O11yFunnelStepsUpdateIn) (*O11yFunnelOut, error) {
	out := new(O11yFunnelOut)
	return out, relay(ctx, nil, in, out)
}

// funnelGet returns one funnel with its steps.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func funnelGet(ctx context.Context, in *O11yFunnelRef) (*O11yFunnelOut, error) {
	out := new(O11yFunnelOut)
	return out, relay(ctx, nil, nil, out)
}

// funnelUpdate renames a funnel or rewrites its description, answering the
// funnel as it now stands.
//
// Callers need the editor role; the runtime's own gate enforces it.
func funnelUpdate(ctx context.Context, in *O11yFunnelUpdateIn) (*O11yFunnelOut, error) {
	out := new(O11yFunnelOut)
	return out, relay(ctx, nil, in, out)
}

// funnelDelete deletes a funnel. The answer carries no data — the runtime
// acknowledges with the success envelope alone, which is what this Out says.
//
// Callers need the editor role; the runtime's own gate enforces it.
func funnelDelete(ctx context.Context, in *O11yFunnelRef) (*O11yFunnelDeleteOut, error) {
	out := new(O11yFunnelDeleteOut)
	return out, relay(ctx, nil, nil, out)
}

// ── analytics over a SAVED funnel ─────────────────────────────────────────────

// funnelValidate lists the traces that match a saved funnel over a window — the
// read that answers "is this funnel finding anything at all".
func funnelValidate(ctx context.Context, in *O11yFunnelWindowIn) (*O11yFunnelRowsOut, error) {
	return funnelAnalytics(ctx, in.FunnelID, "validate", in.TimeRange)
}

// funnelOverview returns a saved funnel's conversion overview over a window:
// how many entered, how many converted, the rate and the latency.
func funnelOverview(ctx context.Context, in *O11yFunnelStepWindowIn) (*O11yFunnelRowsOut, error) {
	return funnelAnalytics(ctx, in.FunnelID, "overview", in.StepTransitionRequest)
}

// funnelSteps returns a saved funnel's per-step metrics over a window — the
// counts and latencies at each step, in step order.
func funnelSteps(ctx context.Context, in *O11yFunnelWindowIn) (*O11yFunnelRowsOut, error) {
	return funnelAnalytics(ctx, in.FunnelID, "steps", in.TimeRange)
}

// funnelStepOverview returns the conversion between two named steps of a saved
// funnel — the step-to-step drill-down behind the overview.
func funnelStepOverview(ctx context.Context, in *O11yFunnelStepWindowIn) (*O11yFunnelRowsOut, error) {
	return funnelAnalytics(ctx, in.FunnelID, "steps/overview", in.StepTransitionRequest)
}

// funnelSlowTraces returns the slowest traces through a step transition of a
// saved funnel — the entry point for "why is this step slow".
func funnelSlowTraces(ctx context.Context, in *O11yFunnelStepWindowIn) (*O11yFunnelRowsOut, error) {
	return funnelAnalytics(ctx, in.FunnelID, "slow-traces", in.StepTransitionRequest)
}

// funnelErrorTraces returns the errored traces through a step transition of a
// saved funnel — the entry point for "why is this step failing".
func funnelErrorTraces(ctx context.Context, in *O11yFunnelStepWindowIn) (*O11yFunnelRowsOut, error) {
	return funnelAnalytics(ctx, in.FunnelID, "error-traces", in.StepTransitionRequest)
}

// ── analytics over an AD-HOC funnel ───────────────────────────────────────────

// draftFunnelValidate lists the traces that match a funnel described inline —
// the builder's "try this" before anything is saved.
func draftFunnelValidate(ctx context.Context, in *O11yDraftFunnelIn) (*O11yFunnelRowsOut, error) {
	return draftAnalytics(ctx, "validate", in)
}

// draftFunnelOverview returns the conversion overview of a funnel described
// inline.
func draftFunnelOverview(ctx context.Context, in *O11yDraftFunnelIn) (*O11yFunnelRowsOut, error) {
	return draftAnalytics(ctx, "overview", in)
}

// draftFunnelSteps returns the per-step metrics of a funnel described inline.
func draftFunnelSteps(ctx context.Context, in *O11yDraftFunnelIn) (*O11yFunnelRowsOut, error) {
	return draftAnalytics(ctx, "steps", in)
}

// draftFunnelStepOverview returns the conversion between two steps of a funnel
// described inline.
func draftFunnelStepOverview(ctx context.Context, in *O11yDraftFunnelIn) (*O11yFunnelRowsOut, error) {
	return draftAnalytics(ctx, "steps/overview", in)
}

// draftFunnelSlowTraces returns the slowest traces through a step transition of
// a funnel described inline.
func draftFunnelSlowTraces(ctx context.Context, in *O11yDraftFunnelIn) (*O11yFunnelRowsOut, error) {
	return draftAnalytics(ctx, "slow-traces", in)
}

// draftFunnelErrorTraces returns the errored traces through a step transition of
// a funnel described inline.
func draftFunnelErrorTraces(ctx context.Context, in *O11yDraftFunnelIn) (*O11yFunnelRowsOut, error) {
	return draftAnalytics(ctx, "error-traces", in)
}

// ── the two shapes of the same call ───────────────────────────────────────────

// funnelPath is the one place a funnel id becomes a path. The id goes on
// VERBATIM, as the segment the router matched: it arrived percent-encoded or it
// did not, and re-encoding it here would hand the runtime a different id than
// the caller named.
func funnelPath(id string) string { return o11yRoot + "/trace-funnels/" + id }

// funnelAnalytics runs one of the six reads against a saved funnel. Twelve ops
// and two call shapes, because the twelve differ only in which of the six
// computations they name and where the funnel came from.
func funnelAnalytics(ctx context.Context, id, view string, body any) (*O11yFunnelRowsOut, error) {
	out := new(O11yFunnelRowsOut)
	return out, relay(ctx, nil, body, out)
}

// draftAnalytics runs one of the six reads against a funnel described inline.
func draftAnalytics(ctx context.Context, view string, in *O11yDraftFunnelIn) (*O11yFunnelRowsOut, error) {
	out := new(O11yFunnelRowsOut)
	return out, relay(ctx, nil, in, out)
}

// ── inputs ────────────────────────────────────────────────────────────────────

// O11yFunnelRef names one saved funnel.
type O11yFunnelRef struct {
	// FunnelID is the funnel's id. Required.
	FunnelID string `json:"-" url:"funnel_id" validate:"required"`
}

// O11yFunnelCreateIn describes the funnel to create.
type O11yFunnelCreateIn struct {
	// Name is the funnel's name.
	Name string `json:"funnel_name"`
	// Timestamp is when the funnel was created, as a millisecond epoch. Zero
	// takes the runtime's own clock.
	Timestamp int64 `json:"timestamp,omitempty"`
}

// O11yFunnelUpdateIn renames a funnel or rewrites its description. The id
// travels in the path; the runtime reads the rest from the body.
type O11yFunnelUpdateIn struct {
	// FunnelID is the funnel to update. Required.
	FunnelID string `json:"-" url:"funnel_id" validate:"required"`
	// Name replaces the funnel's name. Empty leaves it as it was.
	Name string `json:"funnel_name,omitempty"`
	// Description replaces the funnel's description. Empty leaves it as it was.
	Description string `json:"description,omitempty"`
	// Timestamp is when the change was made, as a millisecond epoch.
	Timestamp int64 `json:"timestamp,omitempty"`
}

// O11yFunnelStepsUpdateIn replaces a funnel's steps. The funnel is named in the
// BODY here, not the path — that is the route the console has always called and
// the shape is preserved rather than tidied.
type O11yFunnelStepsUpdateIn struct {
	// FunnelID is the funnel to update.
	FunnelID string `json:"funnel_id"`
	// Steps are the funnel's steps, in order. At least two are needed before
	// any analytics read will answer.
	Steps []*tf.FunnelStep `json:"steps"`
	// Name replaces the funnel's name. Empty leaves it as it was.
	Name string `json:"funnel_name,omitempty"`
	// Description replaces the funnel's description. Empty leaves it as it was.
	Description string `json:"description,omitempty"`
	// Timestamp is when the change was made, as a millisecond epoch.
	Timestamp int64 `json:"timestamp,omitempty"`
}

// O11yFunnelWindowIn names a saved funnel and the window to read it over.
type O11yFunnelWindowIn struct {
	// FunnelID is the funnel to analyse. Required.
	FunnelID string `json:"-" url:"funnel_id" validate:"required"`

	tf.TimeRange
}

// O11yFunnelStepWindowIn names a saved funnel, the window, and which pair of
// steps the transition runs between.
type O11yFunnelStepWindowIn struct {
	// FunnelID is the funnel to analyse. Required.
	FunnelID string `json:"-" url:"funnel_id" validate:"required"`

	tf.StepTransitionRequest
}

// O11yDraftFunnelIn is a funnel described inline, with the window and the step
// pair to analyse. Nothing here is stored.
//
// NOTHING IS MARKED REQUIRED, deliberately. The runtime decodes this body and
// then applies its OWN rules — "funnel must have at least 2 steps" on some
// reads, nothing at all on others — and a validate:"required" here would refuse
// the call earlier, with a different status and a different message than the
// face has always given. A typed op names the wire; it does not tighten it.
type O11yDraftFunnelIn struct {
	// Steps are the funnel's steps, in order. At least two are needed.
	Steps []*tf.FunnelStep `json:"steps"`
	// StartTime is the start of the window, as a millisecond epoch.
	StartTime int64 `json:"start_time,omitempty"`
	// EndTime is the end of the window, as a millisecond epoch.
	EndTime int64 `json:"end_time,omitempty"`
	// StepStart is the step the transition runs from, 1-based. Ignored by the
	// reads that span the whole funnel.
	StepStart int64 `json:"step_start,omitempty"`
	// StepEnd is the step the transition runs to, 1-based.
	StepEnd int64 `json:"step_end,omitempty"`
}

// ── answers ───────────────────────────────────────────────────────────────────

// O11yFunnelOut is one funnel.
type O11yFunnelOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the funnel with its steps.
	Data tf.GettableFunnel `json:"data"`
}

// O11yFunnelsOut is the caller's org's funnels.
type O11yFunnelsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data are the funnels.
	Data []tf.GettableFunnel `json:"data"`
}

// O11yFunnelDeleteOut acknowledges a delete. The runtime answers the success
// envelope and NOTHING else — its data field is omitempty and a delete carries
// no payload — so this type has no data field either. Declaring one would put a
// "data": null in the document that the wire has never sent.
type O11yFunnelDeleteOut struct {
	// Status is "success".
	Status string `json:"status"`
}

// O11yFunnelRowsOut is what all twelve analytics reads answer: a table of rows,
// each a timestamp and an open object of columns. The columns differ per read —
// that is the runtime's query shape, not a fixed schema — so the row is an open
// object and naming its keys here would be a fiction.
type O11yFunnelRowsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data are the rows.
	Data []O11yFunnelRow `json:"data"`
}

// O11yFunnelRow is one row of an analytics answer.
type O11yFunnelRow struct {
	// Timestamp is the row's time.
	Timestamp string `json:"timestamp"`
	// Data are the row's columns, keyed by column name.
	Data map[string]any `json:"data"`
}
