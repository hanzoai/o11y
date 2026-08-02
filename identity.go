package o11y

// The IDENTITY face, as TYPED ops — what is left of it.
//
// It used to be forty-five operations: users, invites, passwords, reset tokens,
// roles-on-users, sessions, auth domains, my-org, preferences and quick filters.
// Thirty-two of them are GONE, with the handlers, the stores and the tables
// behind them, because they were o11y's SECOND answer to questions Hanzo IAM
// already answers. o11y mints no session, holds no password, sends no invite and
// keeps no SSO domain. Identity is asserted once, at the edge, by the IAM
// session the gateway validated; every service downstream reads the assertion.
// (pkg/identn/iamidentn documents the header contract.)
//
// What is left is three kinds of thing, and none of them is a credential:
//
//	getMyUser        — WHO the caller is, echoed back from the CLAIMS the edge
//	                   asserted. The console's identity provider. It reads no
//	                   row, so o11y's own bookkeeping can never be the reason a
//	                   signed-in person cannot open the app.
//	my organization  — the tenant's own record and its per-signal quick filters.
//	                   o11y's data about an org, not about who may enter it.
//	preferences      — per-user and per-org console settings.
//
// WHAT REPLACED THE REST, so nobody has to guess:
//
//	sign in / sign up / sign out    hanzo.id (Hanzo IAM), reached through the
//	                                edge guard. There is no o11y login page.
//	password, reset, forgot         Hanzo IAM. o11y stores no password to reset.
//	invite a member, list members,  the Hanzo IAM console. o11y sees a member
//	edit a member, assign a role     the first time the edge sends them.
//	SSO / auth domains              Hanzo IAM.
//	what may I do?                  POST /v1/o11y/authz/check — o11y's OWN roles
//	                                over o11y's OWN dashboards, alerts and
//	                                views, which is the one thing about a person
//	                                that IS o11y's to answer.
//
// THE WIRE DOES NOT MOVE. No op here re-implements anything: each hands the call
// to the SAME runtime handler the delegation wildcard reaches (see relay.go), so
// identity resolution, the org gate, the role check the mux registration
// declared, the audit record, the timeout and the success envelope are all still
// the runtime's, executed in the order they always were.
//
// Registered ahead of the wildcard, and specific-beats-wildcard is what the
// router does regardless of registration order, so these paths dispatch here and
// every other path under the prefix still reaches the runtime untouched
// (identity_test.go pins both halves).

import (
	"context"
	"net/http"
	"time"

	"github.com/zap-proto/zip"
)

// mountIdentity registers the identity face's typed ops on the native router.
func mountIdentity(app *zip.App) {
	g := app.Group(o11yRoot)

	// who the caller is
	zip.Get(g, "/users/me", getMyUser, zip.WithOperationID("GetMyUser"))

	// my organization and its quick filters
	zip.Get(g, "/orgs/me", getMyOrg, zip.WithOperationID("GetMyOrganization"))
	zip.Put(g, "/orgs/me", updateMyOrg, zip.WithOperationID("UpdateMyOrganization"))
	zip.Get(g, "/orgs/me/filters", getQuickFilters, zip.WithOperationID("GetQuickFilters"))
	zip.Get(g, "/orgs/me/filters/:signal", getSignalFilters, zip.WithOperationID("GetSignalFilters"))
	zip.Put(g, "/orgs/me/filters", updateQuickFilters, zip.WithOperationID("UpdateQuickFilters"))

	// preferences, per user and per org
	zip.Get(g, "/user/preferences", listUserPreferences, zip.WithOperationID("ListUserPreferences"))
	zip.Get(g, "/user/preferences/:name", getUserPreference, zip.WithOperationID("GetUserPreference"))
	zip.Put(g, "/user/preferences/:name", updateUserPreference, zip.WithOperationID("UpdateUserPreference"))
	zip.Get(g, "/org/preferences", listOrgPreferences, zip.WithOperationID("ListOrgPreferences"))
	zip.Get(g, "/org/preferences/:name", getOrgPreference, zip.WithOperationID("GetOrgPreference"))
	zip.Put(g, "/org/preferences/:name", updateOrgPreference, zip.WithOperationID("UpdateOrgPreference"))
}

// ── who the caller is ─────────────────────────────────────────────────────────

// getMyUser returns the identity the edge asserted for the caller — id, org,
// address — as the console's one way of learning who it is rendering for. Open
// to any authenticated caller.
func getMyUser(ctx context.Context, _ *struct{}) (*O11yUserOut, error) {
	out := new(O11yUserOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/users/me", nil, nil, out)
}

// ── my organization and its quick filters ─────────────────────────────────────

// getMyOrg returns the caller's own organization. Admin gate.
func getMyOrg(ctx context.Context, _ *struct{}) (*O11yOrganizationOut, error) {
	out := new(O11yOrganizationOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/orgs/me", nil, nil, out)
}

// updateMyOrg rewrites the caller's own organization record — display name,
// name, alias — always addressed as "me", never by id. Admin gate.
func updateMyOrg(ctx context.Context, in *O11yOrganization) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/orgs/me", nil, in, nil)
}

// getQuickFilters returns the org's quick filters for every signal — the
// attribute shortlists its explorers offer as one-click filters. Viewer gate.
func getQuickFilters(ctx context.Context, _ *struct{}) (*O11yQuickFiltersOut, error) {
	out := new(O11yQuickFiltersOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/orgs/me/filters", nil, nil, out)
}

// getSignalFilters returns the org's quick filters for one signal — traces,
// logs, metrics, exceptions or api_monitoring. Viewer gate.
func getSignalFilters(ctx context.Context, in *O11ySignalRef) (*O11ySignalFiltersOut, error) {
	out := new(O11ySignalFiltersOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/orgs/me/filters/"+in.Signal, nil, nil, out)
}

// updateQuickFilters replaces the org's quick filters for one signal with the
// attribute list given. Admin gate.
func updateQuickFilters(ctx context.Context, in *O11yUpdatableQuickFilters) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/orgs/me/filters", nil, in, nil)
}

// ── preferences ───────────────────────────────────────────────────────────────

// listUserPreferences lists every preference of the calling user, each with
// its current and default value. Viewer gate.
func listUserPreferences(ctx context.Context, _ *struct{}) (*O11yPreferencesOut, error) {
	out := new(O11yPreferencesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/user/preferences", nil, nil, out)
}

// getUserPreference returns one preference of the calling user, by name.
// Viewer gate.
func getUserPreference(ctx context.Context, in *O11yPreferenceRef) (*O11yPreferenceOut, error) {
	out := new(O11yPreferenceOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/user/preferences/"+in.Name, nil, nil, out)
}

// updateUserPreference sets one preference of the calling user, by name.
// Viewer gate.
func updateUserPreference(ctx context.Context, in *O11yUpdatablePreference) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/user/preferences/"+in.Name, nil, in, nil)
}

// listOrgPreferences lists every org-scoped preference, each with its current
// and default value. Admin gate.
func listOrgPreferences(ctx context.Context, _ *struct{}) (*O11yPreferencesOut, error) {
	out := new(O11yPreferencesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/org/preferences", nil, nil, out)
}

// getOrgPreference returns one org-scoped preference, by name. Admin gate.
func getOrgPreference(ctx context.Context, in *O11yPreferenceRef) (*O11yPreferenceOut, error) {
	out := new(O11yPreferenceOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/org/preferences/"+in.Name, nil, nil, out)
}

// updateOrgPreference sets one org-scoped preference, by name. Admin gate.
func updateOrgPreference(ctx context.Context, in *O11yUpdatablePreference) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/org/preferences/"+in.Name, nil, in, nil)
}

// ── inputs ────────────────────────────────────────────────────────────────────

// O11yOrganization is an org record — the answer of getMyOrg and, minus the
// server-owned id, the body of updateMyOrg.
type O11yOrganization struct {
	// CreatedAt is when the org was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// ID is the org id. On update it is ignored: the call always addresses the
	// caller's own org.
	ID string `json:"id"`
	// Name is the org's short name.
	Name string `json:"name"`
	// Alias is an alternate name the org also answers to.
	Alias string `json:"alias"`
	// Key is the org's stable numeric key, derived from its id.
	Key uint32 `json:"key"`
	// DisplayName is what the console shows for the org.
	DisplayName string `json:"displayName"`
}

// O11ySignalRef names one signal's quick filters.
type O11ySignalRef struct {
	// Signal is the signal — traces, logs, metrics, exceptions or
	// api_monitoring.
	Signal string `json:"-" url:"signal" validate:"required"`
}

// O11yUpdatableQuickFilters replaces one signal's quick filters.
type O11yUpdatableQuickFilters struct {
	// Signal is the signal whose filters are being replaced.
	Signal string `json:"signal"`
	// Filters are the attributes to offer, in the order to offer them.
	Filters []O11yFilterKey `json:"filters"`
}

// O11yPreferenceRef names one preference.
type O11yPreferenceRef struct {
	// Name is the preference name.
	Name string `json:"-" url:"name" validate:"required"`
}

// O11yUpdatablePreference sets one preference, addressed by name.
type O11yUpdatablePreference struct {
	// Name is the preference name.
	Name string `json:"-" url:"name" validate:"required"`
	// Value is the value to set; its JSON type must match the preference's
	// declared value type.
	Value any `json:"value"`
}

// ── outputs ───────────────────────────────────────────────────────────────────

// O11yUserOut is the caller's own identity.
type O11yUserOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the identity.
	Data O11yUser `json:"data,omitempty"`
}

// O11yUser is WHO the caller is, as the edge asserted it. There is no role here:
// what the caller may do is answered by POST /v1/o11y/authz/check, which is the
// one place that answer lives.
type O11yUser struct {
	// ID is the user id — the Hanzo IAM subject.
	ID string `json:"id"`
	// DisplayName is what the console shows for them.
	DisplayName string `json:"displayName"`
	// Email is their address.
	Email string `json:"email"`
	// OrgID is the org they belong to.
	OrgID string `json:"orgId"`
	// IsRoot marks the org's root user — always false for an IAM session.
	IsRoot bool `json:"isRoot"`
	// Status is their lifecycle state.
	Status string `json:"status"`
	// CreatedAt is when the projection row was written, when there is one.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed, when there is one.
	UpdatedAt time.Time `json:"updatedAt"`
}

// O11yOrganizationOut is one organization.
type O11yOrganizationOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the organization.
	Data O11yOrganization `json:"data,omitempty"`
}

// O11yQuickFiltersOut is every signal's quick filters.
type O11yQuickFiltersOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds one entry per signal.
	Data []O11ySignalFilters `json:"data"`
}

// O11ySignalFiltersOut is one signal's quick filters.
type O11ySignalFiltersOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the signal's entry.
	Data O11ySignalFilters `json:"data,omitempty"`
}

// O11ySignalFilters is the quick filters of one signal.
type O11ySignalFilters struct {
	// Signal is the signal the filters belong to.
	Signal string `json:"signal"`
	// Filters are the attributes offered, in display order.
	Filters []O11yFilterKey `json:"filters"`
}

// O11yFilterKey is one filterable attribute.
type O11yFilterKey struct {
	// Key is the attribute name.
	Key string `json:"key"`
	// DataType is the attribute's value type — string, int64, float64 or bool.
	DataType string `json:"dataType"`
	// Type says where the attribute lives — tag or resource.
	Type string `json:"type"`
	// IsColumn marks an attribute stored as its own column.
	IsColumn bool `json:"isColumn"`
	// IsJSON marks an attribute read out of a JSON body.
	IsJSON bool `json:"isJSON"`
}

// O11yPreferencesOut is a list of preferences.
type O11yPreferencesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the preferences.
	Data []O11yPreference `json:"data"`
}

// O11yPreferenceOut is one preference.
type O11yPreferenceOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the preference.
	Data O11yPreference `json:"data,omitempty"`
}

// O11yPreference is one preference with its current value.
type O11yPreference struct {
	// Name is the preference name.
	Name string `json:"name"`
	// Description says what the preference does.
	Description string `json:"description"`
	// ValueType is the JSON type a value must have — string, integer, float or
	// boolean.
	ValueType string `json:"valueType"`
	// DefaultValue is the value before anyone set one.
	DefaultValue any `json:"defaultValue"`
	// AllowedScopes are the scopes the preference may be set at — org, user.
	AllowedScopes []string `json:"allowedScopes"`
	// AllowedValues restricts a string preference to these values.
	AllowedValues []string `json:"allowedValues"`
	// Value is the current value.
	Value any `json:"value"`
}
