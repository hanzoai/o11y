package o11y

// The DASHBOARDS product face — the v2 (Perses-shape) dashboard CRUD, cloning,
// locking, per-user pinning, the org-shared saved views, public sharing config,
// and the two anonymous public-read routes — as TYPED ops.
//
// These were twenty-two of the routes that reached traffic only through the
// delegation wildcard, and a route behind a wildcard is in no document: no SDK
// method, no CLI command, no agent tool, no reference page. A customer could not
// reach their own dashboards from anything but a hand-written HTTP call. Typing
// them is what puts the twenty-two operations in the document and therefore in
// every projection built from it.
//
// THE WIRE DOES NOT MOVE, and the way it does not move is that these ops do not
// re-implement the reads and writes. Each hands the call to the SAME runtime
// handler the wildcard delegates to (relay.go — the one seam every face relays
// on, the o11y face root), so identity resolution, the org gate, the exact role
// check each route has always run (ViewAccess on the reads and pins, EditAccess
// on the dashboard and view writes, AdminAccess on the public-sharing config,
// and the anonymous CheckWithoutClaims scoped to the public dashboard on the two
// public reads), the input validation, the audit record and the {status,data}
// envelope are all still the runtime's, executed in the order they always were.
// What is new here is the TYPE — the In the caller may send and the Out they get
// back — and the prose that goes with it.
//
// TWO shapes stay opaque on purpose, carried as json.RawMessage rather than
// re-modelled: a dashboard's `spec` (the Perses panels/queries/variables/layouts,
// whose leaves are plugin-defined open JSON — dashboardtypes.DashboardSpec builds
// a per-plugin oneOf only the swaggest reflector understands) and a widget
// query-range answer's `data`/`meta`/`warning` (the querybuildertypesv5 result
// union owned by the query layer). RawMessage is the honest bound for open JSON —
// it names the field and round-trips its bytes exactly — and it keeps the runtime
// the sole validator of those shapes, so a bad spec still refuses with the
// runtime's own error, not a second one invented here. Everything else — the
// dashboard metadata, tags, saved views, public-sharing config, and the identity
// of a created share — is named field for field.
//
// Registered ahead of the wildcard, and specific-beats-wildcard is what the
// router does regardless of registration order, so these paths dispatch here and
// every other path under the prefix still reaches the runtime untouched.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/zap-proto/zip"
)

// mountDashboards registers the twenty-two typed dashboard ops on the native
// router. Collection routes register before the parameterised ones, mirroring the
// mux tree's discipline so a param can never shadow a sibling literal. Each
// carries the operation id its mux registration always declared, so the document
// names it exactly as it was named before.
func mountDashboards(app *zip.App) {
	g := under{app, o11yRoot}

	// Dashboards — list, per-user list, create, clone, read, update, patch, delete.
	opGet(g, "/dashboards", dashboardListV2, zip.WithOperationID("ListDashboardsV2"))
	opGet(g, "/users/me/dashboards", dashboardListForUserV2, zip.WithOperationID("ListDashboardsForUserV2"))
	opPost(g, "/dashboards", dashboardCreateV2, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateDashboardV2"))
	opPost(g, "/dashboards/:id/clone", dashboardCloneV2, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CloneDashboardV2"))
	opGet(g, "/dashboards/:id", dashboardGetV2, zip.WithOperationID("GetDashboardV2"))
	opPut(g, "/dashboards/:id", dashboardUpdateV2, zip.WithOperationID("UpdateDashboardV2"))
	opPatch(g, "/dashboards/:id", dashboardPatchV2, zip.WithOperationID("PatchDashboardV2"))
	opDelete(g, "/dashboards/:id", dashboardDeleteV2, zip.WithOperationID("DeleteDashboardV2"))

	// Lock / unlock — only the creator or an org admin; the runtime decides.
	opPut(g, "/dashboards/:id/lock", dashboardLockV2, zip.WithOperationID("LockDashboardV2"))
	opDelete(g, "/dashboards/:id/lock", dashboardUnlockV2, zip.WithOperationID("UnlockDashboardV2"))

	// Per-user pins — a viewer bookmark, mutating only the caller's pin list.
	opPut(g, "/users/me/dashboards/:id/pins", dashboardPinV2, zip.WithOperationID("PinDashboardV2"))
	opDelete(g, "/users/me/dashboards/:id/pins", dashboardUnpinV2, zip.WithOperationID("UnpinDashboardV2"))

	// Saved views — org-shared dashboard-listing state.
	opGet(g, "/dashboard_views", dashboardListViews, zip.WithOperationID("ListDashboardViews"))
	opPost(g, "/dashboard_views", dashboardCreateView, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateDashboardView"))
	opPut(g, "/dashboard_views/:id", dashboardUpdateView, zip.WithOperationID("UpdateDashboardView"))
	opDelete(g, "/dashboard_views/:id", dashboardDeleteView, zip.WithOperationID("DeleteDashboardView"))

	// Public sharing config — org-admin only.
	opPost(g, "/dashboards/:id/public", dashboardCreatePublic, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreatePublicDashboard"))
	opGet(g, "/dashboards/:id/public", dashboardGetPublic, zip.WithOperationID("GetPublicDashboard"))
	opPut(g, "/dashboards/:id/public", dashboardUpdatePublic, zip.WithOperationID("UpdatePublicDashboard"))
	opDelete(g, "/dashboards/:id/public", dashboardDeletePublic, zip.WithOperationID("DeletePublicDashboard"))

	// Anonymous public reads — scoped to the public dashboard's read scope.
	opGet(g, "/public/dashboards/:id", dashboardGetPublicData, zip.WithOperationID("GetPublicDashboardData"))
	opGet(g, "/public/dashboards/:id/widgets/:idx/query_range", dashboardGetPublicWidgetQueryRange, zip.WithOperationID("GetPublicDashboardWidgetQueryRange"))
}

// ── the twenty-two operations ───────────────────────────────────────────────────

// dashboardListV2 returns a page of v2-shape dashboards for the org. This is the
// pure, user-independent list — it carries no pin state; use
// dashboardListForUserV2 for the personalized, pin-aware list. Supports a filter
// DSL (query), sort (updated_at/created_at/name), order (asc/desc), and
// offset-based pagination (limit/offset).
//
// Callers need the viewer role; the runtime's own gate enforces it.
func dashboardListV2(ctx context.Context, in *O11yDashboardListParams) (*O11yDashboardListOut, error) {
	out := new(O11yDashboardListOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/dashboards", listParams(in), nil, out)
}

// dashboardListForUserV2 is dashboardListV2 personalized for the calling user:
// each dashboard carries the caller's pinned state, and pinned dashboards float to
// the top of the requested ordering. Supports the same filter DSL, sort, order and
// pagination.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func dashboardListForUserV2(ctx context.Context, in *O11yDashboardListParams) (*O11yDashboardListForUserOut, error) {
	out := new(O11yDashboardListForUserOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/users/me/dashboards", listParams(in), nil, out)
}

// dashboardCreateV2 creates a dashboard in the v2 format that follows the Perses
// spec and answers with the stored dashboard.
//
// Callers need the editor role; the runtime's own gate enforces it.
func dashboardCreateV2(ctx context.Context, in *O11yDashboardPostable) (*O11yDashboardOut, error) {
	out := new(O11yDashboardOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/dashboards", nil, in, out)
}

// dashboardCloneV2 clones an existing v2-shape dashboard. User and integration
// dashboards can be cloned; system dashboards are rejected. The clone keeps the
// source's display name, panels and tags, but gets a freshly generated unique
// internal name and is always created as an unlocked user dashboard owned by the
// caller.
//
// Callers need the editor role; the runtime's own gate enforces it.
func dashboardCloneV2(ctx context.Context, in *O11yDashboardIDIn) (*O11yDashboardOut, error) {
	out := new(O11yDashboardOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/dashboards/"+in.ID+"/clone", nil, nil, out)
}

// dashboardGetV2 returns a v2-shape dashboard.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func dashboardGetV2(ctx context.Context, in *O11yDashboardIDIn) (*O11yDashboardOut, error) {
	out := new(O11yDashboardOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/dashboards/"+in.ID, nil, nil, out)
}

// dashboardUpdateV2 updates a v2-shape dashboard's metadata, spec and tag set.
// The name is immutable and locked dashboards are rejected.
//
// Callers need the editor role; the runtime's own gate enforces it.
func dashboardUpdateV2(ctx context.Context, in *O11yDashboardUpdateIn) (*O11yDashboardOut, error) {
	out := new(O11yDashboardOut)
	return out, relay(ctx, http.MethodPut, o11yRoot+"/dashboards/"+in.ID, nil, in.O11yDashboardUpdatable, out)
}

// dashboardPatchV2 applies an RFC 6902 JSON Patch to a v2-shape dashboard. The
// patch is applied against the postable view (metadata, spec, tags), so individual
// panels, queries, variables, layouts or tags can be updated without re-sending the
// rest. Apply is lenient — remove on a missing path is a no-op and add creates any
// missing parent objects — and the result is still validated. Locked dashboards are
// rejected. The request body is the bare JSON Patch operations array.
//
// Callers need the editor role; the runtime's own gate enforces it.
func dashboardPatchV2(ctx context.Context, in *O11yDashboardPatchIn) (*O11yDashboardOut, error) {
	out := new(O11yDashboardOut)
	// The body goes on as the bare ops array the runtime's patch decoder reads;
	// the id rode in on the path, not the body.
	return out, relay(ctx, http.MethodPatch, o11yRoot+"/dashboards/"+in.ID, nil, in.Ops, out)
}

// dashboardDeleteV2 deletes a v2-shape dashboard along with its tag relations.
// Locked dashboards are rejected.
//
// Callers need the editor role; the runtime's own gate enforces it.
func dashboardDeleteV2(ctx context.Context, in *O11yDashboardIDIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/dashboards/"+in.ID, nil, nil, nil)
}

// dashboardLockV2 locks a v2-shape dashboard. Only the dashboard's creator or an
// org admin may lock or unlock.
//
// Callers need the editor role; the runtime's own gate enforces it.
func dashboardLockV2(ctx context.Context, in *O11yDashboardIDIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/dashboards/"+in.ID+"/lock", nil, nil, nil)
}

// dashboardUnlockV2 unlocks a v2-shape dashboard. Only the dashboard's creator or
// an org admin may lock or unlock.
//
// Callers need the editor role; the runtime's own gate enforces it.
func dashboardUnlockV2(ctx context.Context, in *O11yDashboardIDIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/dashboards/"+in.ID+"/lock", nil, nil, nil)
}

// dashboardPinV2 pins a dashboard for the calling user. A user can pin at most ten
// dashboards; pinning at the limit refuses with the runtime's conflict. Re-pinning
// an already-pinned dashboard is a no-op success. Pinning mutates only the caller's
// pin list, not the dashboard, so a viewer may pin what a viewer may read.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func dashboardPinV2(ctx context.Context, in *O11yDashboardIDIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/users/me/dashboards/"+in.ID+"/pins", nil, nil, nil)
}

// dashboardUnpinV2 removes the caller's pin for a dashboard. Idempotent —
// unpinning a dashboard that was not pinned still succeeds.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func dashboardUnpinV2(ctx context.Context, in *O11yDashboardIDIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/users/me/dashboards/"+in.ID+"/pins", nil, nil, nil)
}

// dashboardListViews returns every saved view in the calling user's org. Saved
// views are shared org-wide.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func dashboardListViews(ctx context.Context, _ *struct{}) (*O11yDashboardViewListOut, error) {
	out := new(O11yDashboardViewListOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/dashboard_views", nil, nil, out)
}

// dashboardCreateView persists the calling user's dashboard-listing state (query,
// sort, order) as a named, reusable view shared across the org.
//
// Callers need the editor role; the runtime's own gate enforces it.
func dashboardCreateView(ctx context.Context, in *O11yDashboardViewPostable) (*O11yDashboardViewOut, error) {
	out := new(O11yDashboardViewOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/dashboard_views", nil, in, out)
}

// dashboardUpdateView replaces a saved view's name and data. Saved views are shared
// org-wide.
//
// Callers need the editor role; the runtime's own gate enforces it.
func dashboardUpdateView(ctx context.Context, in *O11yDashboardViewUpdateIn) (*O11yDashboardViewOut, error) {
	out := new(O11yDashboardViewOut)
	return out, relay(ctx, http.MethodPut, o11yRoot+"/dashboard_views/"+in.ID, nil, in.O11yDashboardViewPostable, out)
}

// dashboardDeleteView removes a saved view. Saved views are shared org-wide.
// Deleting a non-existent view refuses with the runtime's not-found.
//
// Callers need the editor role; the runtime's own gate enforces it.
func dashboardDeleteView(ctx context.Context, in *O11yDashboardIDIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/dashboard_views/"+in.ID, nil, nil, nil)
}

// dashboardCreatePublic creates the public-sharing config for a dashboard and
// enables public sharing, answering with the new share's id.
//
// Callers need the admin role; the runtime's own gate enforces it.
func dashboardCreatePublic(ctx context.Context, in *O11yPublicDashboardWriteIn) (*O11yIdentifiableOut, error) {
	out := new(O11yIdentifiableOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/dashboards/"+in.ID+"/public", nil, in.O11yPublicDashboardWrite, out)
}

// dashboardGetPublic returns the public-sharing config for a dashboard.
//
// Callers need the admin role; the runtime's own gate enforces it.
func dashboardGetPublic(ctx context.Context, in *O11yDashboardIDIn) (*O11yPublicDashboardOut, error) {
	out := new(O11yPublicDashboardOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/dashboards/"+in.ID+"/public", nil, nil, out)
}

// dashboardUpdatePublic updates the public-sharing config for a dashboard.
//
// Callers need the admin role; the runtime's own gate enforces it.
func dashboardUpdatePublic(ctx context.Context, in *O11yPublicDashboardWriteIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/dashboards/"+in.ID+"/public", nil, in.O11yPublicDashboardWrite, nil)
}

// dashboardDeletePublic deletes the public-sharing config and disables public
// sharing of a dashboard.
//
// Callers need the admin role; the runtime's own gate enforces it.
func dashboardDeletePublic(ctx context.Context, in *O11yDashboardIDIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/dashboards/"+in.ID+"/public", nil, nil, nil)
}

// dashboardGetPublicData returns the sanitized dashboard data for public access —
// the read a shared dashboard's public page makes.
//
// Anonymous, scoped to the public dashboard's read scope; the runtime's own gate
// enforces it.
func dashboardGetPublicData(ctx context.Context, in *O11yDashboardIDIn) (*O11yPublicDashboardDataOut, error) {
	out := new(O11yPublicDashboardDataOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/public/dashboards/"+in.ID, nil, nil, out)
}

// dashboardGetPublicWidgetQueryRange returns the query-range result for one widget
// of a public dashboard. When the share fixes its own time range the caller's
// startTime/endTime are ignored; otherwise they bound the window as millisecond
// epochs.
//
// Anonymous, scoped to the public dashboard's read scope; the runtime's own gate
// enforces it.
func dashboardGetPublicWidgetQueryRange(ctx context.Context, in *O11yWidgetQueryRangeIn) (*O11yWidgetQueryRangeOut, error) {
	out := new(O11yWidgetQueryRangeOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/public/dashboards/"+in.ID+"/widgets/"+in.Idx+"/query_range", query("startTime", in.StartTime, "endTime", in.EndTime), nil, out)
}

// ── the list query ──────────────────────────────────────────────────────────────

// listParams renders the dashboard list query onto the wire — the ONE place a
// list input becomes URL parameters, dropping the unset ones so an absent filter
// stays absent and the runtime applies its own defaults.
func listParams(in *O11yDashboardListParams) url.Values {
	return query(
		"query", in.Query,
		"sort", in.Sort,
		"order", in.Order,
		"limit", in.Limit,
		"offset", in.Offset,
	)
}

// ── inputs ────────────────────────────────────────────────────────────────────

// O11yDashboardListParams selects and pages the dashboard list. Every type here
// carries the O11yDashboard qualifier because these names enter a fleet-wide
// document, where one name may have exactly one shape.
type O11yDashboardListParams struct {
	// Query is the filter DSL over dashboard columns and tags, e.g.
	// `name:cpu source:user`. Empty lists everything.
	Query string `json:"query,omitempty"`
	// Sort is the sort field: updated_at, created_at or name. Empty sorts by
	// updated_at.
	Sort string `json:"sort,omitempty"`
	// Order is the sort direction: asc or desc. Empty orders desc.
	Order string `json:"order,omitempty"`
	// Limit caps how many dashboards come back. Zero means the default of 20;
	// the runtime caps it at 200.
	Limit int `json:"limit,omitempty"`
	// Offset is how many dashboards to skip for pagination.
	Offset int `json:"offset,omitempty"`
}

// O11yDashboardIDIn addresses one dashboard, saved view or public share by the id
// on the path.
type O11yDashboardIDIn struct {
	// ID is the resource id from the path.
	ID string `json:"id"`
}

// O11yDashboardPostable is a new v2 dashboard. The spec is carried verbatim as the
// Perses shape the runtime validates.
type O11yDashboardPostable struct {
	// SchemaVersion is the dashboard schema version; must be the current v6.
	SchemaVersion string `json:"schemaVersion"`
	// Image is an optional cover image reference.
	Image string `json:"image,omitempty"`
	// Name is the dashboard's unique internal name (a DNS-1123 label). Omit it
	// with generateName to derive one from the display name.
	Name string `json:"name,omitempty"`
	// GenerateName derives a fresh unique name from spec.display.name instead of
	// taking Name.
	GenerateName bool `json:"generateName,omitempty"`
	// Tags are the dashboard's tags; at most ten, and none may use a reserved DSL
	// key.
	Tags []O11yDashboardPostableTag `json:"tags"`
	// Spec is the Perses dashboard spec (display, variables, panels, layouts,
	// datasources). Its plugin-defined leaves are open JSON, carried verbatim.
	Spec json.RawMessage `json:"spec"`
}

// O11yDashboardUpdatable is the mutable body of a v2 dashboard update: everything
// but the immutable name is replaced.
type O11yDashboardUpdatable struct {
	// SchemaVersion is the dashboard schema version; must be the current v6.
	SchemaVersion string `json:"schemaVersion"`
	// Image is an optional cover image reference.
	Image string `json:"image,omitempty"`
	// Name must equal the dashboard's existing name — it is immutable.
	Name string `json:"name"`
	// Tags replace the dashboard's tags; at most ten.
	Tags []O11yDashboardPostableTag `json:"tags"`
	// Spec is the replacement Perses spec, carried verbatim as open JSON.
	Spec json.RawMessage `json:"spec"`
}

// O11yDashboardUpdateIn is a dashboard update: the id on the path plus the
// replacement body.
type O11yDashboardUpdateIn struct {
	// ID is the dashboard id from the path.
	ID string `json:"id"`
	O11yDashboardUpdatable
}

// O11yDashboardPatchOp is one RFC 6902 JSON Patch operation against a dashboard's
// postable shape.
type O11yDashboardPatchOp struct {
	// Op is the verb: add, remove, replace, move, copy or test.
	Op string `json:"op"`
	// Path is a JSON Pointer into the postable dashboard, e.g.
	// /spec/display/name, /spec/panels/<id>, /tags/-.
	Path string `json:"path"`
	// Value is the value to add/replace/test; its type depends on Path. Required
	// for add, replace and test; ignored otherwise.
	Value json.RawMessage `json:"value,omitempty"`
	// From is the source JSON Pointer for move and copy; ignored otherwise.
	From string `json:"from,omitempty"`
}

// O11yDashboardPatchIn is a dashboard patch: the id on the path plus the bare RFC
// 6902 operations array as the request body.
type O11yDashboardPatchIn struct {
	// ID is the dashboard id from the path.
	ID string `json:"id"`
	// Ops are the JSON Patch operations, applied in order. On the wire this IS the
	// request body — a bare array, not an object.
	Ops []O11yDashboardPatchOp `json:"ops"`
}

// UnmarshalJSON reads the bare RFC 6902 operations array that is the request body
// into Ops; the id is bound from the path, not the body.
func (in *O11yDashboardPatchIn) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &in.Ops)
}

// O11yDashboardViewData is a saved view's captured listing state.
type O11yDashboardViewData struct {
	// Version is the saved-view schema version; must be v1.
	Version string `json:"version"`
	// Query is the captured filter DSL.
	Query string `json:"query"`
	// Sort is the captured sort field.
	Sort string `json:"sort"`
	// Order is the captured sort direction.
	Order string `json:"order"`
}

// O11yDashboardViewPostable creates or replaces a saved view.
type O11yDashboardViewPostable struct {
	// Name is the saved view's name; at most 32 characters, no surrounding space.
	Name string `json:"name"`
	// Data is the listing state the view captures.
	Data O11yDashboardViewData `json:"data"`
}

// O11yDashboardViewUpdateIn is a saved-view update: the id on the path plus the
// replacement body.
type O11yDashboardViewUpdateIn struct {
	// ID is the saved view id from the path.
	ID string `json:"id"`
	O11yDashboardViewPostable
}

// O11yPublicDashboardWrite is the public-sharing config a caller sets.
type O11yPublicDashboardWrite struct {
	// TimeRangeEnabled lets the public page pick its own time range; when false
	// the share fixes DefaultTimeRange.
	TimeRangeEnabled bool `json:"timeRangeEnabled"`
	// DefaultTimeRange is the fixed window as a Go duration, e.g. 1h, 30m.
	DefaultTimeRange string `json:"defaultTimeRange"`
}

// O11yPublicDashboardWriteIn is a public-sharing write: the dashboard id on the
// path plus the config body.
type O11yPublicDashboardWriteIn struct {
	// ID is the dashboard id from the path.
	ID string `json:"id"`
	O11yPublicDashboardWrite
}

// O11yWidgetQueryRangeIn addresses one widget of a public dashboard and bounds its
// query window.
type O11yWidgetQueryRangeIn struct {
	// ID is the public dashboard id from the path.
	ID string `json:"id"`
	// Idx is the widget's index from the path.
	Idx string `json:"idx"`
	// StartTime is the window start as a millisecond epoch. Used only when the
	// share enables a caller-chosen time range.
	StartTime string `json:"startTime,omitempty"`
	// EndTime is the window end as a millisecond epoch. Used only when the share
	// enables a caller-chosen time range.
	EndTime string `json:"endTime,omitempty"`
}

// ── shared payload shapes ───────────────────────────────────────────────────────

// O11yDashboardTag is one dashboard tag: a key and its value.
type O11yDashboardTag struct {
	// Key is the tag key.
	Key string `json:"key" required:"true"`
	// Value is the tag value.
	Value string `json:"value" required:"true"`
}

// O11yDashboardPostableTag is one tag as a caller writes it — the same key/value
// shape, named apart because a postable tag is a distinct wire role.
type O11yDashboardPostableTag struct {
	// Key is the tag key.
	Key string `json:"key" required:"true"`
	// Value is the tag value.
	Value string `json:"value" required:"true"`
}

// O11yDashboardDisplay is a dashboard's display metadata.
type O11yDashboardDisplay struct {
	// Name is the human-facing dashboard name.
	Name string `json:"name" required:"true"`
	// Description says what the dashboard is for.
	Description string `json:"description,omitempty"`
}

// O11yDashboard is a v2-shape dashboard: its identity and audit stamps, the org it
// belongs to, its lock state and source, the schema metadata, name and tags, and
// the Perses spec carried verbatim.
type O11yDashboard struct {
	// ID is the dashboard's id.
	ID string `json:"id"`
	// CreatedAt is when the dashboard was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// CreatedBy is who created it.
	CreatedBy string `json:"createdBy"`
	// UpdatedBy is who last changed it.
	UpdatedBy string `json:"updatedBy"`
	// OrgID is the org the dashboard belongs to.
	OrgID string `json:"orgId"`
	// Locked reports whether the dashboard is locked against edits.
	Locked bool `json:"locked"`
	// Source is where the dashboard came from: user, system or integration.
	Source string `json:"source"`
	// SchemaVersion is the dashboard schema version.
	SchemaVersion string `json:"schemaVersion"`
	// Image is an optional cover image reference.
	Image string `json:"image,omitempty"`
	// Name is the dashboard's unique internal name.
	Name string `json:"name"`
	// Tags are the dashboard's tags.
	Tags []O11yDashboardTag `json:"tags"`
	// Spec is the Perses dashboard spec, carried verbatim as open JSON.
	Spec json.RawMessage `json:"spec"`
}

// O11yDashboardListSpec is the trimmed spec a list row carries — display only, not
// the panels.
type O11yDashboardListSpec struct {
	// Display is the dashboard's display metadata.
	Display O11yDashboardDisplay `json:"display,omitempty"`
}

// O11yDashboardListItem is one row of the dashboard list: full metadata, but only
// the display slice of the spec.
type O11yDashboardListItem struct {
	// ID is the dashboard's id.
	ID string `json:"id"`
	// CreatedAt is when the dashboard was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// CreatedBy is who created it.
	CreatedBy string `json:"createdBy"`
	// UpdatedBy is who last changed it.
	UpdatedBy string `json:"updatedBy"`
	// OrgID is the org the dashboard belongs to.
	OrgID string `json:"orgId"`
	// Locked reports whether the dashboard is locked against edits.
	Locked bool `json:"locked"`
	// Source is where the dashboard came from: user, system or integration.
	Source string `json:"source"`
	// SchemaVersion is the dashboard schema version.
	SchemaVersion string `json:"schemaVersion"`
	// Name is the dashboard's unique internal name.
	Name string `json:"name"`
	// Image is an optional cover image reference.
	Image string `json:"image,omitempty"`
	// Tags are the dashboard's tags.
	Tags []O11yDashboardTag `json:"tags"`
	// Spec is the display-only slice of the dashboard spec.
	Spec O11yDashboardListSpec `json:"spec"`
}

// O11yDashboardListItemForUser is a list row plus the calling user's pin state.
type O11yDashboardListItemForUser struct {
	O11yDashboardListItem
	// Pinned reports whether the calling user has pinned this dashboard.
	Pinned bool `json:"pinned"`
}

// O11yDashboardList is a page of the pure dashboard list.
type O11yDashboardList struct {
	// Dashboards are the rows for this page.
	Dashboards []O11yDashboardListItem `json:"dashboards"`
	// Total is the count across all pages.
	Total int64 `json:"total"`
	// Tags are all tags in use across the org's dashboards.
	Tags []O11yDashboardTag `json:"tags"`
}

// O11yDashboardListForUser is a page of the per-user dashboard list, pins included.
type O11yDashboardListForUser struct {
	// Dashboards are the rows for this page, each with the caller's pin state.
	Dashboards []O11yDashboardListItemForUser `json:"dashboards"`
	// Total is the count across all pages.
	Total int64 `json:"total"`
	// Tags are all tags in use across the org's dashboards.
	Tags []O11yDashboardTag `json:"tags"`
}

// O11yDashboardView is one saved view: its identity and audit stamps, name,
// captured listing state and org.
type O11yDashboardView struct {
	// ID is the saved view's id.
	ID string `json:"id"`
	// CreatedAt is when the view was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Name is the saved view's name.
	Name string `json:"name"`
	// Data is the listing state the view captures.
	Data O11yDashboardViewData `json:"data"`
	// OrgID is the org the view belongs to.
	OrgID string `json:"orgId"`
}

// O11yDashboardViewList is the org's saved views.
type O11yDashboardViewList struct {
	// Views are the saved views, shared org-wide.
	Views []O11yDashboardView `json:"views"`
}

// O11yPublicDashboard is a dashboard's public-sharing config as read back.
type O11yPublicDashboard struct {
	// TimeRangeEnabled reports whether the public page may pick its own range.
	TimeRangeEnabled bool `json:"timeRangeEnabled"`
	// DefaultTimeRange is the fixed window when the range is not caller-chosen.
	DefaultTimeRange string `json:"defaultTimeRange"`
	// PublicPath is the public URL path the share is reachable at.
	PublicPath string `json:"publicPath"`
}

// O11yPublicDashboardV1 is the sanitized v1-shape dashboard a public page reads:
// its identity and audit stamps, lock state, org, source, and the widget data as
// stored (open JSON, with query internals stripped to what a public viewer may
// see).
type O11yPublicDashboardV1 struct {
	// CreatedAt is when the dashboard was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// CreatedBy is who created it.
	CreatedBy string `json:"createdBy"`
	// UpdatedBy is who last changed it.
	UpdatedBy string `json:"updatedBy"`
	// ID is the dashboard's id.
	ID string `json:"id"`
	// Data is the sanitized widget data as stored, carried verbatim as open JSON.
	Data json.RawMessage `json:"data"`
	// Locked reports whether the dashboard is locked.
	Locked bool `json:"locked"`
	// OrgID is the org the dashboard belongs to.
	OrgID string `json:"org_id"`
	// Source is where the dashboard came from.
	Source string `json:"source"`
}

// O11yPublicDashboardData is the anonymous public read: the sanitized dashboard
// and its public-sharing config.
type O11yPublicDashboardData struct {
	// Dashboard is the sanitized dashboard.
	Dashboard *O11yPublicDashboardV1 `json:"dashboard"`
	// PublicDashboard is the public-sharing config.
	PublicDashboard *O11yPublicDashboard `json:"publicDashboard"`
}

// O11yIdentifiable is a bare id — the answer to a create that returns only the new
// resource's identity.
type O11yIdentifiable struct {
	// ID is the created resource's id.
	ID string `json:"id" required:"true"`
}

// O11yWidgetQueryRange is a widget's query-range answer. The result body is the
// querybuildertypesv5 union owned by the query layer, carried verbatim as open
// JSON so its one authoritative shape is not restated here.
type O11yWidgetQueryRange struct {
	// Type is the request type the result answers, e.g. time_series, scalar.
	Type string `json:"type"`
	// Data is the query result payload, carried verbatim as open JSON.
	Data json.RawMessage `json:"data"`
	// Meta is the execution stats, carried verbatim as open JSON.
	Meta json.RawMessage `json:"meta"`
	// Warning is any query warning, carried verbatim as open JSON when present.
	Warning json.RawMessage `json:"warning,omitempty"`
}

// ── outputs ───────────────────────────────────────────────────────────────────
//
// Every dashboard route answers through render.Success, so each Out is the
// runtime's {status,data} envelope NAMED: a status string and the typed payload.
// The op decodes exactly those bytes and re-answers them, so the envelope the
// caller sees is byte-for-byte the one the runtime wrote. The void routes carry no
// payload and answer 204 — their handlers return a nil *struct{}, which zip
// answers as a bodyless 204 the way the routes always have — so they need no Out
// shape.

// O11yDashboardOut is a single v2 dashboard, enveloped.
type O11yDashboardOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the dashboard.
	Data *O11yDashboard `json:"data,omitempty"`
}

// O11yDashboardListOut is a page of the pure dashboard list, enveloped.
type O11yDashboardListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the list page.
	Data *O11yDashboardList `json:"data,omitempty"`
}

// O11yDashboardListForUserOut is a page of the per-user dashboard list, enveloped.
type O11yDashboardListForUserOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the list page with pins.
	Data *O11yDashboardListForUser `json:"data,omitempty"`
}

// O11yDashboardViewOut is a single saved view, enveloped.
type O11yDashboardViewOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the saved view.
	Data *O11yDashboardView `json:"data,omitempty"`
}

// O11yDashboardViewListOut is the org's saved views, enveloped.
type O11yDashboardViewListOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the saved views.
	Data *O11yDashboardViewList `json:"data,omitempty"`
}

// O11yIdentifiableOut is a bare id, enveloped — the answer to creating a public
// share.
type O11yIdentifiableOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the created resource's id.
	Data *O11yIdentifiable `json:"data,omitempty"`
}

// O11yPublicDashboardOut is a dashboard's public-sharing config, enveloped.
type O11yPublicDashboardOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the public-sharing config.
	Data *O11yPublicDashboard `json:"data,omitempty"`
}

// O11yPublicDashboardDataOut is the anonymous public dashboard read, enveloped.
type O11yPublicDashboardDataOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the sanitized dashboard and its share config.
	Data *O11yPublicDashboardData `json:"data,omitempty"`
}

// O11yWidgetQueryRangeOut is a widget's query-range answer, enveloped.
type O11yWidgetQueryRangeOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the query-range result.
	Data *O11yWidgetQueryRange `json:"data,omitempty"`
}
