package o11y

// The SENTRY product face's projects and issues, the ERROR-TRACKING issues, and
// the legacy EXCEPTIONS reads — eighteen routes — as TYPED ops.
//
// These reached traffic only through the delegation wildcards, and a route
// behind a wildcard is in no document: no SDK method, no CLI command, no agent
// tool, no reference page. A customer could not reach their own projects,
// issues or exceptions from anything but a hand-written HTTP call. Typing them
// is what puts the eighteen operations in the document and therefore in every
// projection built from it. The five telemetry reads of this same face
// (discover, logs, traces, one trace, stats) were typed already in
// telemetry.go; this file is the rest of it.
//
// THE WIRE DOES NOT MOVE, the same way telemetry.go's and logs.go's ops do not
// move it: these ops do not re-implement the reads and writes. Each hands the
// call to the SAME runtime handler the wildcard delegates to (relay for
// /v1/sentry, relayAt for /v1/o11y — telemetry.go's and logs.go's seams), so
// identity resolution, the org gate, the exact role check each route has always
// had, the audit record and the success envelope stay where they were and run
// in the order they always did. What is new here is the TYPE — the In a caller
// may send and the Out they get back — and the prose that goes with it.
//
// THE GATES CARRY OVER EXACTLY, enforced one layer in by the runtime the op
// relays into (pkg/apiserver/o11yapiserver/{sentry,errortracking}.go,
// pkg/query-service/app/routes_errors.go):
//
//   - Sentry projects: ViewAccess on the reads (list, get), EditAccess on the
//     writes (create, delete, rotate-key).
//   - Sentry issues: ViewAccess on list/get/events, EditAccess on update.
//   - Sentry event detail: ViewAccess.
//   - Error-tracking issues: ViewAccess on list/get, EditAccess on update.
//   - Legacy exceptions (listErrors, countErrors, errorFromErrorID,
//     errorFromGroupID, nextPrevErrorIDs): ViewAccess, every one.
//
// The FOUR ingest routes of this face — POST /v1/sentry/{project}/envelope|store/
// and POST /v1/o11y/api/{project}/envelope|store/ — are the deliberate escape
// hatches, NOT typed here. They are OpenAccess (a Sentry SDK presents a DSN key,
// not a Hanzo session) and carry an OPAQUE Sentry-envelope body — a foreign wire
// protocol we receive verbatim, like /.well-known. A typed relay JSON-encodes
// its input, which would corrupt that raw envelope, so they stay on the runtime
// mux byte-identical (pkg/apiserver/o11yapiserver/{sentry,errortracking}.go pin
// them) and out of the document, which cannot describe an opaque proxy honestly.
// This is the same call telemetry.go's livetail note makes for a stream.
//
// Collection routes register before their parameterised siblings so an id can
// never shadow a collection — specific-beats-wildcard is what the router does
// regardless of order, and both halves are what the surface has always relied on.

import (
	"context"
	"time"

	"github.com/zap-proto/zip"
)

// mountSentryErrors registers the eighteen typed ops: the sentry product face on
// the sentryRoot group, and the error-tracking and legacy exceptions faces on
// the o11yRoot group. Both roots and the seam they relay through are spelled
// once each, in relay.go.
func mountSentryErrors(app *zip.App) {
	gsentry := under{app, sentryRoot}
	opGet(gsentry, "/projects", sentryListProjects)
	opPost(gsentry, "/projects", sentryCreateProject)
	opGet(gsentry, "/projects/:id", sentryGetProject)
	opDelete(gsentry, "/projects/:id", sentryDeleteProject)
	opPost(gsentry, "/projects/:id/keys/rotate", sentryRotateProjectKey)
	opGet(gsentry, "/issues", sentryListIssues)
	opGet(gsentry, "/issues/:id", sentryGetIssue)
	opPut(gsentry, "/issues/:id", sentryUpdateIssue)
	opGet(gsentry, "/issues/:id/events", sentryIssueEvents)
	opGet(gsentry, "/events/:id", sentryGetEvent)

	go11y := under{app, o11yRoot}
	opGet(go11y, "/errortracking/issues", errorListIssues)
	opGet(go11y, "/errortracking/issues/:id", errorGetIssue)
	opPost(go11y, "/errortracking/issues/:id", errorUpdateIssue)
	opPost(go11y, "/listErrors", errorsList)
	opPost(go11y, "/countErrors", errorsCount)
	opGet(go11y, "/errorFromErrorID", errorFromErrorID)
	opGet(go11y, "/errorFromGroupID", errorFromGroupID)
	opGet(go11y, "/nextPrevErrorIDs", nextPrevErrorIDs)
}

// ── sentry projects ─────────────────────────────────────────────────────────

// sentryListProjects lists the caller's org's Sentry projects, each with its
// freshly-derived DSN.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func sentryListProjects(ctx context.Context, _ *struct{}) (*O11ySentryProjectsOut, error) {
	out := new(O11ySentryProjectsOut)
	return out, relay(ctx, nil, nil, out)
}

// sentryCreateProject creates a Sentry project under the caller's org and
// returns it, DSN included. Only the name, and optionally a slug and platform,
// are the caller's to set; the org, id and key are server-assigned.
//
// Callers need the editor role; the runtime's own gate enforces it.
func sentryCreateProject(ctx context.Context, in *O11ySentryPostableProject) (*O11ySentryProjectOut, error) {
	out := new(O11ySentryProjectOut)
	return out, relay(ctx, nil, in, out)
}

// sentryGetProject returns one Sentry project of the caller's org, DSN included.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func sentryGetProject(ctx context.Context, in *O11ySentryProjectRef) (*O11ySentryProjectOut, error) {
	out := new(O11ySentryProjectOut)
	return out, relay(ctx, nil, nil, out)
}

// sentryDeleteProject deletes one Sentry project of the caller's org. Its DSN
// stops resolving immediately, so ingest for that id fails closed exactly as an
// unknown project does; retained events are not touched. Answers 204.
//
// Callers need the editor role; the runtime's own gate enforces it.
func sentryDeleteProject(ctx context.Context, in *O11ySentryProjectRef) (*struct{}, error) {
	// The runtime answers 204 with a {"status":"success"} body; it is swallowed
	// so the op answers a clean 204 No Content, its declared contract.
	if err := relay(ctx, nil, nil, &struct{}{}); err != nil {
		return nil, err
	}
	return nil, nil
}

// sentryRotateProjectKey rotates a project's DSN key — bumping its rotation
// watermark so keys below it stop verifying — and returns the project with its
// new DSN.
//
// Callers need the editor role; the runtime's own gate enforces it.
func sentryRotateProjectKey(ctx context.Context, in *O11ySentryProjectRef) (*O11ySentryProjectOut, error) {
	out := new(O11ySentryProjectOut)
	return out, relay(ctx, nil, nil, out)
}

// ── sentry issues ───────────────────────────────────────────────────────────

// sentryListIssues lists the caller's org's grouped error issues, optionally
// narrowed to one project and one time window, and filtered by status, level,
// environment, service, a free-text query and a sort.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func sentryListIssues(ctx context.Context, in *O11ySentryIssuesIn) (*O11yErrorIssuesOut, error) {
	out := new(O11yErrorIssuesOut)
	return out, relay(ctx, query(
		"status", in.Status,
		"level", in.Level,
		"environment", in.Environment,
		"serviceName", in.ServiceName,
		"query", in.Query,
		"sort", in.Sort,
		"offset", in.Offset,
		"limit", in.Limit,
		"project", in.Project,
		"period", in.Period,
	), nil, out)
}

// sentryGetIssue returns one grouped issue of the caller's org with its latest
// occurrence sample.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func sentryGetIssue(ctx context.Context, in *O11ySentryIssueRef) (*O11yErrorGettableIssueOut, error) {
	out := new(O11yErrorGettableIssueOut)
	return out, relay(ctx, nil, nil, out)
}

// sentryUpdateIssue changes an issue's lifecycle — resolve, ignore, reopen or
// assign — and returns the updated issue. Fields left unset are left unchanged.
//
// Callers need the editor role; the runtime's own gate enforces it.
func sentryUpdateIssue(ctx context.Context, in *O11ySentryUpdateIssueIn) (*O11yErrorIssueOut, error) {
	out := new(O11yErrorIssueOut)
	return out, relay(ctx, nil, O11yIssueUpdate{Status: in.Status, Assignee: in.Assignee}, out)
}

// sentryIssueEvents lists one issue's captured occurrences, scoped to a project
// — a project is an isolation unit, so the caller declares which project's
// occurrences to read.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func sentryIssueEvents(ctx context.Context, in *O11ySentryIssueEventsIn) (*O11ySentryIssueEventsOut, error) {
	out := new(O11ySentryIssueEventsOut)
	return out, relay(ctx, query(
		"project", in.Project,
		"limit", in.Limit,
	), nil, out)
}

// sentryGetEvent returns one captured error event of a project, by its id.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func sentryGetEvent(ctx context.Context, in *O11ySentryEventRef) (*O11ySentryEventOut, error) {
	out := new(O11ySentryEventOut)
	return out, relay(ctx, query(
		"project", in.Project,
	), nil, out)
}

// ── error-tracking issues (the console Errors tab) ──────────────────────────

// errorListIssues lists the caller's org's grouped error issues (by
// fingerprint) with status, level, counts and first/last-seen.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func errorListIssues(ctx context.Context, in *O11yErrorIssuesIn) (*O11yErrorIssuesOut, error) {
	out := new(O11yErrorIssuesOut)
	return out, relay(ctx, query(
		"status", in.Status,
		"level", in.Level,
		"environment", in.Environment,
		"serviceName", in.ServiceName,
		"query", in.Query,
		"sort", in.Sort,
		"offset", in.Offset,
		"limit", in.Limit,
	), nil, out)
}

// errorGetIssue returns one grouped issue with its latest occurrence sample.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func errorGetIssue(ctx context.Context, in *O11yErrorIssueRef) (*O11yErrorGettableIssueOut, error) {
	out := new(O11yErrorGettableIssueOut)
	return out, relay(ctx, nil, nil, out)
}

// errorUpdateIssue changes an issue's lifecycle — resolve, ignore, reopen or
// assign — and returns the updated issue. Fields left unset are left unchanged.
//
// Callers need the editor role; the runtime's own gate enforces it.
func errorUpdateIssue(ctx context.Context, in *O11yErrorUpdateIssueIn) (*O11yErrorIssueOut, error) {
	out := new(O11yErrorIssueOut)
	return out, relay(ctx, nil, O11yIssueUpdate{Status: in.Status, Assignee: in.Assignee}, out)
}

// ── legacy exceptions reads ─────────────────────────────────────────────────

// errorsList lists the grouped exceptions in the query window — each an
// exception type with its message, count, service and first/last-seen — for the
// caller's org.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func errorsList(ctx context.Context, in *O11yErrorsListIn) (*O11yErrorsList, error) {
	out := new(O11yErrorsList)
	return out, relay(ctx, nil, in, out)
}

// errorsCount counts the grouped exceptions in the query window for the caller's
// org.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func errorsCount(ctx context.Context, in *O11yErrorsCountIn) (*O11yErrorCount, error) {
	out := new(O11yErrorCount)
	return out, relay(ctx, nil, in, out)
}

// errorFromErrorID returns one exception instance and the span it happened on,
// by its error id within a group at a timestamp.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func errorFromErrorID(ctx context.Context, in *O11yErrorLookupIn) (*O11yErrorWithSpan, error) {
	out := new(O11yErrorWithSpan)
	return out, relay(ctx, query(
		"timestamp", in.Timestamp,
		"groupID", in.GroupID,
		"errorID", in.ErrorID,
	), nil, out)
}

// errorFromGroupID returns the representative exception instance of a group at a
// timestamp, and the span it happened on.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func errorFromGroupID(ctx context.Context, in *O11yErrorLookupIn) (*O11yErrorWithSpan, error) {
	out := new(O11yErrorWithSpan)
	return out, relay(ctx, query(
		"timestamp", in.Timestamp,
		"groupID", in.GroupID,
		"errorID", in.ErrorID,
	), nil, out)
}

// nextPrevErrorIDs returns the ids of the exception instances immediately after
// and before a given one within its group — the paging cursor the error detail
// view walks.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func nextPrevErrorIDs(ctx context.Context, in *O11yErrorLookupIn) (*O11yNextPrevErrorIDs, error) {
	out := new(O11yNextPrevErrorIDs)
	return out, relay(ctx, query(
		"timestamp", in.Timestamp,
		"groupID", in.GroupID,
		"errorID", in.ErrorID,
	), nil, out)
}

// ── inputs ──────────────────────────────────────────────────────────────────
//
// Every type here carries the O11y qualifier because these names enter a
// fleet-wide document, where one name may have exactly one shape. Nothing
// embeds: the schema builder walks reflected fields and cannot name an embedded
// one, so an embedded shape would publish a schema missing every field it
// carries — which is why these MIRROR the runtime's types field for field
// rather than importing them (the runtime rows embed bun.BaseModel and the
// id/time audit mixins).

// O11ySentryPostableProject creates a Sentry project.
type O11ySentryPostableProject struct {
	// Name is the project's display name. Required.
	Name string `json:"name" validate:"required"`
	// Slug is the project's short name. Server-assigned from Name when empty.
	Slug string `json:"slug,omitempty"`
	// Platform is the reporting runtime, e.g. go, python, javascript.
	Platform string `json:"platform,omitempty"`
}

// O11ySentryProjectRef names one Sentry project by its id.
type O11ySentryProjectRef struct {
	// ID is the project id.
	ID string `json:"id" validate:"required"`
}

// O11ySentryIssuesIn selects and filters a page of a project-or-org's grouped
// issues.
type O11ySentryIssuesIn struct {
	// Status narrows to one lifecycle state: unresolved, resolved or ignored.
	Status string `json:"status,omitempty"`
	// Level narrows to one severity, e.g. error, warning, info.
	Level string `json:"level,omitempty"`
	// Environment narrows to one deployment environment.
	Environment string `json:"environment,omitempty"`
	// ServiceName narrows to one reporting service.
	ServiceName string `json:"serviceName,omitempty"`
	// Query narrows to issues whose text contains it.
	Query string `json:"query,omitempty"`
	// Sort orders the page, e.g. lastSeen, firstSeen, count.
	Sort string `json:"sort,omitempty"`
	// Offset is how many issues to skip. Zero starts at the first.
	Offset int `json:"offset,omitempty"`
	// Limit caps how many issues come back. Zero means the default.
	Limit int `json:"limit,omitempty"`
	// Project narrows the org's issues to one project, by its id.
	Project string `json:"project,omitempty"`
	// Period is the window to read, relative to now — 1h, 24h, 7d, 14d, 30d.
	Period string `json:"period,omitempty"`
}

// O11ySentryIssueRef names one issue by its id.
type O11ySentryIssueRef struct {
	// ID is the issue id.
	ID string `json:"id" validate:"required"`
}

// O11ySentryUpdateIssueIn is a lifecycle change to one issue: the id names it,
// and the set fields change it. Nil fields are left unchanged.
type O11ySentryUpdateIssueIn struct {
	// ID is the issue id.
	ID string `json:"id" validate:"required"`
	// Status is the new lifecycle state: unresolved, resolved or ignored.
	Status *string `json:"status,omitempty"`
	// Assignee is who the issue is assigned to.
	Assignee *string `json:"assignee,omitempty"`
}

// O11ySentryIssueEventsIn selects a page of one issue's occurrences within a
// project.
type O11ySentryIssueEventsIn struct {
	// ID is the issue id.
	ID string `json:"id" validate:"required"`
	// Project is the project whose occurrences to read, by its id. Required.
	Project string `json:"project" validate:"required"`
	// Limit caps how many occurrences come back. Zero means the default.
	Limit int `json:"limit,omitempty"`
}

// O11ySentryEventRef names one captured event within a project.
type O11ySentryEventRef struct {
	// ID is the event id.
	ID string `json:"id" validate:"required"`
	// Project is the project the event belongs to, by its id. Required.
	Project string `json:"project" validate:"required"`
}

// O11yErrorIssuesIn selects and filters a page of the caller's org's grouped
// issues (the console Errors tab), org-scoped with no project dimension.
type O11yErrorIssuesIn struct {
	// Status narrows to one lifecycle state: unresolved, resolved or ignored.
	Status string `json:"status,omitempty"`
	// Level narrows to one severity, e.g. error, warning, info.
	Level string `json:"level,omitempty"`
	// Environment narrows to one deployment environment.
	Environment string `json:"environment,omitempty"`
	// ServiceName narrows to one reporting service.
	ServiceName string `json:"serviceName,omitempty"`
	// Query narrows to issues whose text contains it.
	Query string `json:"query,omitempty"`
	// Sort orders the page, e.g. lastSeen, firstSeen, count.
	Sort string `json:"sort,omitempty"`
	// Offset is how many issues to skip. Zero starts at the first.
	Offset int `json:"offset,omitempty"`
	// Limit caps how many issues come back. Zero means the default.
	Limit int `json:"limit,omitempty"`
}

// O11yErrorIssueRef names one issue by its id.
type O11yErrorIssueRef struct {
	// ID is the issue id.
	ID string `json:"id" validate:"required"`
}

// O11yErrorUpdateIssueIn is a lifecycle change to one issue: the id names it,
// and the set fields change it. Nil fields are left unchanged.
type O11yErrorUpdateIssueIn struct {
	// ID is the issue id.
	ID string `json:"id" validate:"required"`
	// Status is the new lifecycle state: unresolved, resolved or ignored.
	Status *string `json:"status,omitempty"`
	// Assignee is who the issue is assigned to.
	Assignee *string `json:"assignee,omitempty"`
}

// O11yIssueUpdate is the lifecycle-change body the update ops send to the
// runtime — the id travels in the URL, so the body carries only the change.
type O11yIssueUpdate struct {
	// Status is the new lifecycle state: unresolved, resolved or ignored.
	Status *string `json:"status,omitempty"`
	// Assignee is who the issue is assigned to.
	Assignee *string `json:"assignee,omitempty"`
}

// O11yErrorsListIn selects a page of grouped exceptions over a window.
type O11yErrorsListIn struct {
	// Start is the window start, as a nanosecond epoch spelled as a string.
	Start string `json:"start"`
	// End is the window end, as a nanosecond epoch spelled as a string.
	End string `json:"end"`
	// Limit caps how many exception groups come back. Required, non-zero.
	Limit int64 `json:"limit"`
	// OrderParam is the column to order by, e.g. exceptionCount, lastSeen.
	OrderParam string `json:"orderParam"`
	// Order is the direction: ascending or descending.
	Order string `json:"order"`
	// Offset is how many groups to skip.
	Offset int64 `json:"offset"`
	// ServiceName narrows to one reporting service.
	ServiceName string `json:"serviceName"`
	// ExceptionType narrows to one exception type.
	ExceptionType string `json:"exceptionType"`
	// Tags narrow the scan to spans carrying the given tag values.
	Tags []O11yTagQuery `json:"tags"`
}

// O11yErrorsCountIn selects a window and filters for counting grouped
// exceptions.
type O11yErrorsCountIn struct {
	// Start is the window start, as a nanosecond epoch spelled as a string.
	Start string `json:"start"`
	// End is the window end, as a nanosecond epoch spelled as a string.
	End string `json:"end"`
	// ServiceName narrows to one reporting service.
	ServiceName string `json:"serviceName"`
	// ExceptionType narrows to one exception type.
	ExceptionType string `json:"exceptionType"`
	// Tags narrow the scan to spans carrying the given tag values.
	Tags []O11yTagQuery `json:"tags"`
}

// O11yTagQuery is one tag predicate over the trace store: a key, the value set
// to test in the value's own type, and the operator that combines them.
type O11yTagQuery struct {
	// Key is the tag to test.
	Key string `json:"key"`
	// TagType is where the tag lives, e.g. ResourceAttribute, SpanAttribute.
	TagType string `json:"tagType"`
	// StringValues are the string values to test against.
	StringValues []string `json:"stringValues"`
	// BoolValues are the boolean values to test against.
	BoolValues []bool `json:"boolValues"`
	// NumberValues are the numeric values to test against.
	NumberValues []float64 `json:"numberValues"`
	// Operator is the comparison, e.g. in, nin, contains, exists.
	Operator string `json:"operator"`
}

// O11yErrorLookupIn names one exception instance to read — the group and
// timestamp that locate it, plus the error id where the route needs one.
type O11yErrorLookupIn struct {
	// Timestamp is the instance's time as a nanosecond epoch spelled as a
	// string. Required.
	Timestamp string `json:"timestamp" validate:"required"`
	// GroupID is the exception group the instance belongs to. Required.
	GroupID string `json:"groupID" validate:"required"`
	// ErrorID is the exception instance id. Required by errorFromErrorID and
	// nextPrevErrorIDs; unused by errorFromGroupID.
	ErrorID string `json:"errorID,omitempty"`
}

// ── outputs ─────────────────────────────────────────────────────────────────
//
// Each Out is the runtime's answer NAMED, field for field, tag for tag. The
// sentry and error-tracking reads answer inside the {status, data} envelope
// render.Success writes, so those Outs carry it; the legacy exceptions reads
// answer their payload BARE (the older WriteJSON writes no envelope), so those
// Outs are the payload itself. Nothing embeds, so the document and the bytes
// agree.

// O11ySentryProjectsOut is the {status, data} envelope around a page of
// projects.
type O11ySentryProjectsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the projects.
	Data O11ySentryProjects `json:"data,omitempty"`
}

// O11ySentryProjects is a page of Sentry projects.
type O11ySentryProjects struct {
	// Items are the projects.
	Items []O11ySentryProject `json:"items"`
	// Total is how many the org has.
	Total int `json:"total"`
}

// O11ySentryProjectOut is the {status, data} envelope around one project.
type O11ySentryProjectOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the project.
	Data O11ySentryProject `json:"data,omitempty"`
}

// O11ySentryProject is a Sentry project — a DSN-bearing unit under an org. The
// DSN is derived on demand, never stored.
type O11ySentryProject struct {
	// ID is the project id.
	ID string `json:"id"`
	// CreatedAt is when the project was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the project last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Name is the project's display name.
	Name string `json:"name"`
	// Slug is the project's short name.
	Slug string `json:"slug"`
	// Platform is the reporting runtime, e.g. go, python, javascript.
	Platform string `json:"platform,omitempty"`
	// Status is the project's lifecycle state: active or disabled.
	Status string `json:"status"`
	// DSN is the project's freshly-derived ingest DSN.
	DSN string `json:"dsn"`
}

// O11yErrorIssuesOut is the {status, data} envelope around a page of grouped
// issues — the answer to both the sentry and the error-tracking issue lists.
type O11yErrorIssuesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the issues.
	Data O11yErrorIssues `json:"data,omitempty"`
}

// O11yErrorIssues is a page of grouped issues with its paging cursor.
type O11yErrorIssues struct {
	// Items are the issues.
	Items []O11yErrorIssue `json:"items"`
	// Total is how many matched the filter.
	Total int `json:"total"`
	// Offset is how many were skipped.
	Offset int `json:"offset"`
	// Limit is the page cap that was applied.
	Limit int `json:"limit"`
}

// O11yErrorGettableIssueOut is the {status, data} envelope around one issue's
// detail.
type O11yErrorGettableIssueOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the issue and its latest occurrence.
	Data O11yErrorGettableIssue `json:"data,omitempty"`
}

// O11yErrorGettableIssue is one grouped issue plus its latest occurrence sample.
type O11yErrorGettableIssue struct {
	// Issue is the lifecycle row.
	Issue *O11yErrorIssue `json:"issue"`
	// LatestEvent is the most recent occurrence that landed on the issue.
	LatestEvent *O11yOccurrence `json:"latestEvent,omitempty"`
}

// O11yErrorIssueOut is the {status, data} envelope around one issue — the answer
// to both the sentry and the error-tracking update.
type O11yErrorIssueOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the issue.
	Data O11yErrorIssue `json:"data,omitempty"`
}

// O11yErrorIssue is a grouped error — a fingerprint bucket. Only the lifecycle
// that cannot be derived from telemetry lives on it; occurrences live in the
// telemetry store.
type O11yErrorIssue struct {
	// ID is the issue id.
	ID string `json:"id"`
	// CreatedAt is when the issue was first recorded.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the issue last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Fingerprint is the grouping key that puts like errors in one issue.
	Fingerprint string `json:"fingerprint"`
	// Type is the exception type.
	Type string `json:"type"`
	// Value is the exception value.
	Value string `json:"value"`
	// Culprit is where it came from — the function or route blamed for it.
	Culprit string `json:"culprit"`
	// Level is the issue's severity, e.g. error, warning, info.
	Level string `json:"level"`
	// Platform is the reporting runtime, e.g. go, python, javascript.
	Platform string `json:"platform,omitempty"`
	// Status is the lifecycle state: unresolved, resolved or ignored.
	Status string `json:"status"`
	// Assignee is who the issue is assigned to.
	Assignee string `json:"assignee,omitempty"`
	// FirstSeen is when the earliest occurrence was recorded.
	FirstSeen time.Time `json:"firstSeen"`
	// LastSeen is when the latest was.
	LastSeen time.Time `json:"lastSeen"`
	// Count is how many occurrences have landed on the issue.
	Count int64 `json:"count"`
	// ResolvedAt is when the issue was resolved, if it is.
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	// Regressed marks an issue that reopened after being resolved.
	Regressed bool `json:"regressed"`
	// Environment is the deployment the issue was seen in.
	Environment string `json:"environment,omitempty"`
	// Release is the version that produced it.
	Release string `json:"release,omitempty"`
	// ServiceName is the service that reported it.
	ServiceName string `json:"serviceName,omitempty"`
}

// O11yOccurrence is one normalized error occurrence — the "latest event" sample
// kept on an issue for its detail view.
type O11yOccurrence struct {
	// EventID is the occurrence's id.
	EventID string `json:"eventId"`
	// Fingerprint is the grouping key it was bucketed by.
	Fingerprint string `json:"fingerprint"`
	// Type is the exception type.
	Type string `json:"type"`
	// Value is the exception value.
	Value string `json:"value"`
	// Culprit is where it came from.
	Culprit string `json:"culprit"`
	// Level is its severity, e.g. error, warning, info.
	Level string `json:"level"`
	// Platform is the reporting runtime.
	Platform string `json:"platform,omitempty"`
	// Timestamp is when the error happened.
	Timestamp time.Time `json:"timestamp"`
	// Environment is the deployment it happened in.
	Environment string `json:"environment,omitempty"`
	// Release is the version that produced it.
	Release string `json:"release,omitempty"`
	// ServiceName is the service that reported it.
	ServiceName string `json:"serviceName,omitempty"`
	// ServerName is the host that reported it.
	ServerName string `json:"serverName,omitempty"`
	// Transaction is the operation it happened in.
	Transaction string `json:"transaction,omitempty"`
	// TraceID is the trace it belonged to.
	TraceID string `json:"traceId,omitempty"`
	// SpanID is the span it belonged to.
	SpanID string `json:"spanId,omitempty"`
	// Frames are the stack, innermost first.
	Frames []O11yOccurrenceFrame `json:"frames,omitempty"`
	// Tags are the reporter's own key/value labels.
	Tags map[string]string `json:"tags,omitempty"`
	// User is the affected end-user context, when the reporter attached one.
	User *O11yEventUser `json:"user,omitempty"`
}

// O11yOccurrenceFrame is one stack frame of a normalized occurrence.
type O11yOccurrenceFrame struct {
	// Function is the function the frame is in.
	Function string `json:"function,omitempty"`
	// Module is the module the function is in.
	Module string `json:"module,omitempty"`
	// Filename is the file it is in.
	Filename string `json:"filename,omitempty"`
	// AbsPath is the file's absolute path.
	AbsPath string `json:"absPath,omitempty"`
	// Lineno is the line number.
	Lineno int `json:"lineno,omitempty"`
	// Colno is the column number.
	Colno int `json:"colno,omitempty"`
	// InApp marks a frame in the reporting application's own code.
	InApp bool `json:"inApp"`
}

// O11yEventUser is the reporting end-user context of an occurrence.
type O11yEventUser struct {
	// ID identifies the affected end user.
	ID string `json:"id,omitempty"`
	// Email is that user's email.
	Email string `json:"email,omitempty"`
	// Username is that user's name.
	Username string `json:"username,omitempty"`
	// IP is that user's address.
	IP string `json:"ipAddress,omitempty"`
}

// O11ySentryIssueEventsOut is the {status, data} envelope around a page of an
// issue's occurrences. The occurrences are events on the columnar plane, so
// they carry the same shape sentry logs answer with (see telemetry.go's
// O11yEvent).
type O11ySentryIssueEventsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the occurrences.
	Data O11yEvents `json:"data,omitempty"`
}

// O11ySentryEventOut is the {status, data} envelope around one captured event.
type O11ySentryEventOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the event.
	Data O11yEvent `json:"data,omitempty"`
}

// O11yErrorsList is a page of grouped exceptions, answered BARE — the legacy
// exceptions face writes no envelope.
type O11yErrorsList []O11yListError

// O11yListError is one grouped exception: a type with its message, count,
// service and the span of time it was seen.
type O11yListError struct {
	// ExceptionType is the exception's type.
	ExceptionType string `json:"exceptionType"`
	// ExceptionMsg is its message.
	ExceptionMsg string `json:"exceptionMessage"`
	// ExceptionCount is how many instances the group holds in the window.
	ExceptionCount uint64 `json:"exceptionCount"`
	// LastSeen is when the latest instance was recorded.
	LastSeen time.Time `json:"lastSeen"`
	// FirstSeen is when the earliest was.
	FirstSeen time.Time `json:"firstSeen"`
	// ServiceName is the service that reported them.
	ServiceName string `json:"serviceName"`
	// GroupID is the group's id.
	GroupID string `json:"groupID"`
}

// O11yErrorCount is the count of grouped exceptions in the window, answered BARE
// as a JSON number.
type O11yErrorCount uint64

// O11yErrorWithSpan is one exception instance and the span it happened on,
// answered BARE.
type O11yErrorWithSpan struct {
	// ErrorID is the exception instance id.
	ErrorID string `json:"errorId"`
	// ExceptionType is the exception's type.
	ExceptionType string `json:"exceptionType"`
	// ExceptionStacktrace is the captured stack trace.
	ExceptionStacktrace string `json:"exceptionStacktrace"`
	// ExceptionEscaped marks an exception that escaped its span uncaught.
	ExceptionEscaped bool `json:"exceptionEscaped"`
	// ExceptionMsg is the exception's message.
	ExceptionMsg string `json:"exceptionMessage"`
	// Timestamp is when it happened.
	Timestamp time.Time `json:"timestamp"`
	// SpanID is the span it happened on.
	SpanID string `json:"spanID"`
	// TraceID is the trace the span belonged to.
	TraceID string `json:"traceID"`
	// ServiceName is the service that reported it.
	ServiceName string `json:"serviceName"`
	// GroupID is the exception group it belongs to.
	GroupID string `json:"groupID"`
}

// O11yNextPrevErrorIDs is the paging cursor around one exception instance within
// its group, answered BARE.
type O11yNextPrevErrorIDs struct {
	// NextErrorID is the id of the instance immediately after this one.
	NextErrorID string `json:"nextErrorID"`
	// NextTimestamp is that instance's time.
	NextTimestamp time.Time `json:"nextTimestamp"`
	// PrevErrorID is the id of the instance immediately before this one.
	PrevErrorID string `json:"prevErrorID"`
	// PrevTimestamp is that instance's time.
	PrevTimestamp time.Time `json:"prevTimestamp"`
	// GroupID is the group both belong to.
	GroupID string `json:"groupID"`
}
