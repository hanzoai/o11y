package o11y

// The ACCESS-CONTROL face — roles, service accounts, service-account keys and
// the authorization probe — as TYPED ops.
//
// These twenty routes reached traffic only through the delegation wildcard, and
// a route behind a wildcard is in no document: no SDK method, no CLI command,
// no agent tool, no reference page. An operator could not grant a role or mint
// a service-account key from anything but a hand-written HTTP call. Typing them
// is what puts the twenty operations in the document and therefore in every
// projection built from it.
//
// THE WIRE DOES NOT MOVE, and the way it does not move is that these ops do not
// re-implement access control. Each hands the call to the SAME runtime handler
// the wildcard delegates to (see relay.go), so identity resolution, the org gate,
// the CheckResources/OpenAccess gate on every route, the FGA resource checks,
// the audit record and the success envelope are all still the runtime's,
// executed in the order they always were. What is new here is the TYPE — the In
// the caller may send and the Out they get back — and the prose that goes with
// it.
//
// Statuses are the RUNTIME's, not the old swaggest annotations': granting a
// role to a service account renders 204 (the annotation claimed 201, the
// handler never did), and every update/delete answers 204 with no content, the
// same bytes a caller has always received through the wildcard.
//
// TWO DISPATCHES, ONE IMPLEMENTATION: the mux registrations in
// pkg/apiserver/o11yapiserver/{role,serviceaccount,authz}.go stay — the
// standalone server has no native router to register an op on — and both
// dispatches end in the same handler.

import (
	"context"
	"net/http"
	"time"

	"github.com/zap-proto/zip"
)

// mountAccess registers the twenty typed access-control ops on the native
// router. The literal /service_accounts/me routes register ahead of the
// parameterised ones so an id can never shadow them — the same defence
// telemetry.go gives its collection routes.
func mountAccess(app *zip.App) {
	g := app.Group(o11yRoot)

	zip.Post(g, "/roles", createRole, op("CreateRole"), zip.WithStatus(http.StatusCreated))
	zip.Get(g, "/roles", listRoles, op("ListRoles"))
	zip.Get(g, "/roles/:id", getRole, op("GetRole"))
	zip.Put(g, "/roles/:id", updateRole, op("UpdateRole"), zip.WithStatus(http.StatusNoContent))
	zip.Delete(g, "/roles/:id", deleteRole, op("DeleteRole"), zip.WithStatus(http.StatusNoContent))

	zip.Post(g, "/service_accounts", createServiceAccount, op("CreateServiceAccount"), zip.WithStatus(http.StatusCreated))
	zip.Get(g, "/service_accounts", listServiceAccounts, op("ListServiceAccounts"))
	zip.Get(g, "/service_accounts/me", getMyServiceAccount, op("GetMyServiceAccount"))
	zip.Put(g, "/service_accounts/me", updateMyServiceAccount, op("UpdateMyServiceAccount"), zip.WithStatus(http.StatusNoContent))
	zip.Get(g, "/service_accounts/:id", getServiceAccount, op("GetServiceAccount"))
	zip.Put(g, "/service_accounts/:id", updateServiceAccount, op("UpdateServiceAccount"), zip.WithStatus(http.StatusNoContent))
	zip.Delete(g, "/service_accounts/:id", deleteServiceAccount, op("DeleteServiceAccount"), zip.WithStatus(http.StatusNoContent))
	zip.Get(g, "/service_accounts/:id/roles", listServiceAccountRoles, op("GetServiceAccountRoles"))
	zip.Post(g, "/service_accounts/:id/roles", grantServiceAccountRole, op("CreateServiceAccountRole"), zip.WithStatus(http.StatusNoContent))
	zip.Delete(g, "/service_accounts/:id/roles/:rid", revokeServiceAccountRole, op("DeleteServiceAccountRole"), zip.WithStatus(http.StatusNoContent))
	zip.Post(g, "/service_accounts/:id/keys", createServiceAccountKey, op("CreateServiceAccountKey"), zip.WithStatus(http.StatusCreated))
	zip.Get(g, "/service_accounts/:id/keys", listServiceAccountKeys, op("ListServiceAccountKeys"))
	zip.Put(g, "/service_accounts/:id/keys/:fid", updateServiceAccountKey, op("UpdateServiceAccountKey"), zip.WithStatus(http.StatusNoContent))
	zip.Delete(g, "/service_accounts/:id/keys/:fid", revokeServiceAccountKey, op("RevokeServiceAccountKey"), zip.WithStatus(http.StatusNoContent))

	zip.Post(g, "/authz/check", checkAccess, op("AuthzCheck"))
}

// ── roles ─────────────────────────────────────────────────────────────────────

// createRole creates a custom role in the caller's org from a name, an optional
// description and the transaction groups it grants, answering the new role's id.
//
// Names are lowercase letters and hyphens only, and may not start with the
// reserved managed-role prefix; the runtime refuses anything else.
func createRole(ctx context.Context, in *O11yRoleCreateIn) (*O11yRoleCreateOut, error) {
	out := new(O11yRoleCreateOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/roles", nil, in, out)
}

// listRoles lists every role in the caller's org — the managed ones the
// platform seeds and the custom ones its admins created.
func listRoles(ctx context.Context, in *O11yRolesIn) (*O11yRolesOut, error) {
	out := new(O11yRolesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/roles", nil, nil, out)
}

// getRole returns one role with the transaction groups it grants.
func getRole(ctx context.Context, in *O11yRoleGetIn) (*O11yRoleOut, error) {
	out := new(O11yRoleOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/roles/"+in.ID, nil, nil, out)
}

// updateRole replaces a custom role's description and transaction groups.
// Both fields are mandatory — send an empty string or an empty array to clear
// one — and managed roles cannot be edited.
func updateRole(ctx context.Context, in *O11yRoleUpdateIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/roles/"+in.ID, nil, in, nil)
}

// deleteRole deletes a custom role. A role that still has user or
// service-account assignees, or an auth-domain mapping, is refused; managed
// roles cannot be deleted.
func deleteRole(ctx context.Context, in *O11yRoleDeleteIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/roles/"+in.ID, nil, nil, nil)
}

// ── service accounts ──────────────────────────────────────────────────────────

// createServiceAccount creates a service account in the caller's org,
// answering its id. The name — a lowercase letter followed by lowercase
// letters, digits or hyphens — becomes the account's email local part.
func createServiceAccount(ctx context.Context, in *O11yServiceAccountCreateIn) (*O11yServiceAccountCreateOut, error) {
	out := new(O11yServiceAccountCreateOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/service_accounts", nil, in, out)
}

// listServiceAccounts lists the caller's org's service accounts.
func listServiceAccounts(ctx context.Context, in *O11yServiceAccountsIn) (*O11yServiceAccountsOut, error) {
	out := new(O11yServiceAccountsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/service_accounts", nil, nil, out)
}

// getMyServiceAccount returns the calling service account itself, with the
// roles it holds — the self-inspection read for a key-authenticated caller.
func getMyServiceAccount(ctx context.Context, in *O11yMyServiceAccountIn) (*O11yServiceAccountOut, error) {
	out := new(O11yServiceAccountOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/service_accounts/me", nil, nil, out)
}

// updateMyServiceAccount renames the calling service account.
func updateMyServiceAccount(ctx context.Context, in *O11yMyServiceAccountUpdateIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/service_accounts/me", nil, in, nil)
}

// getServiceAccount returns one service account with the roles it holds.
func getServiceAccount(ctx context.Context, in *O11yServiceAccountGetIn) (*O11yServiceAccountOut, error) {
	out := new(O11yServiceAccountOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/service_accounts/"+in.ID, nil, nil, out)
}

// updateServiceAccount renames a service account.
func updateServiceAccount(ctx context.Context, in *O11yServiceAccountUpdateIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/service_accounts/"+in.ID, nil, in, nil)
}

// deleteServiceAccount deletes a service account and revokes every key it
// holds.
func deleteServiceAccount(ctx context.Context, in *O11yServiceAccountDeleteIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/service_accounts/"+in.ID, nil, nil, nil)
}

// listServiceAccountRoles lists the roles a service account holds.
func listServiceAccountRoles(ctx context.Context, in *O11yServiceAccountRolesIn) (*O11yServiceAccountRolesOut, error) {
	out := new(O11yServiceAccountRolesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/service_accounts/"+in.ID+"/roles", nil, nil, out)
}

// grantServiceAccountRole assigns a role, named by its id, to a service
// account.
func grantServiceAccountRole(ctx context.Context, in *O11yServiceAccountRoleGrantIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPost, o11yRoot+"/service_accounts/"+in.ServiceAccountID+"/roles", nil, in, nil)
}

// revokeServiceAccountRole removes a role from a service account.
func revokeServiceAccountRole(ctx context.Context, in *O11yServiceAccountRoleRevokeIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/service_accounts/"+in.ID+"/roles/"+in.RoleID, nil, nil, nil)
}

// ── service-account keys ──────────────────────────────────────────────────────

// createServiceAccountKey mints an API key for a service account and answers
// the key's id and its secret — the one time the secret is ever shown.
//
// ExpiresAt is a unix timestamp in seconds; zero means the key never expires,
// and a timestamp in the past is refused.
func createServiceAccountKey(ctx context.Context, in *O11yAPIKeyCreateIn) (*O11yAPIKeyCreateOut, error) {
	out := new(O11yAPIKeyCreateOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/service_accounts/"+in.ServiceAccountID+"/keys", nil, in, out)
}

// listServiceAccountKeys lists a service account's API keys — metadata only,
// never the secrets.
func listServiceAccountKeys(ctx context.Context, in *O11yAPIKeysIn) (*O11yAPIKeysOut, error) {
	out := new(O11yAPIKeysOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/service_accounts/"+in.ID+"/keys", nil, nil, out)
}

// updateServiceAccountKey renames an API key or moves its expiry.
func updateServiceAccountKey(ctx context.Context, in *O11yAPIKeyUpdateIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/service_accounts/"+in.ServiceAccountID+"/keys/"+in.KeyID, nil, in, nil)
}

// revokeServiceAccountKey revokes an API key. Revocation is immediate and
// permanent.
func revokeServiceAccountKey(ctx context.Context, in *O11yAPIKeyRevokeIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/service_accounts/"+in.ID+"/keys/"+in.KeyID, nil, nil, nil)
}

// ── authorization probe ───────────────────────────────────────────────────────

// checkAccess evaluates a batch of transactions — relation plus object — for
// the authenticated caller and answers each with its authorization verdict, in
// the order they were asked. It is the read a UI uses to decide which controls
// to show.
func checkAccess(ctx context.Context, in *O11yCheckIn) (*O11yCheckOut, error) {
	out := new(O11yCheckOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/authz/check", nil, in, out)
}

// ── inputs ────────────────────────────────────────────────────────────────────
//
// Every type carries the O11y qualifier because these names enter a fleet-wide
// document, where one name may have exactly one shape. Path-bound fields are
// `json:"-" url:"…"` so they never leak into a relayed body; body fields whose
// PRESENCE the runtime tests (the update bodies' nil checks) are pointers with
// omitempty, so an absent field stays absent and an intentionally empty one
// stays empty.

// O11yRoleCreateIn describes the role to create.
type O11yRoleCreateIn struct {
	// Name is the role's name: lowercase letters and hyphens, at most 50
	// characters, not starting with the reserved managed-role prefix. Required.
	Name string `json:"name"`
	// Description says what the role is for.
	Description string `json:"description,omitempty"`
	// TransactionGroups are the grants the role carries.
	TransactionGroups []O11yTransactionGroup `json:"transactionGroups,omitempty"`
}

// O11yRolesIn has no inputs; the org is the caller's own.
type O11yRolesIn struct{}

// O11yRoleGetIn names one role.
type O11yRoleGetIn struct {
	// ID is the role id.
	ID string `json:"-" url:"id"`
}

// O11yRoleUpdateIn replaces a role's description and grants.
type O11yRoleUpdateIn struct {
	// ID is the role id.
	ID string `json:"-" url:"id"`
	// Description says what the role is for. Required — send an empty string to
	// clear it.
	Description *string `json:"description,omitempty"`
	// TransactionGroups are the grants the role carries. Required — send an
	// empty array to clear them.
	TransactionGroups *[]O11yTransactionGroup `json:"transactionGroups,omitempty"`
}

// O11yRoleDeleteIn names the role to delete.
type O11yRoleDeleteIn struct {
	// ID is the role id.
	ID string `json:"-" url:"id"`
}

// O11yServiceAccountCreateIn describes the service account to create.
type O11yServiceAccountCreateIn struct {
	// Name is the account's name: a lowercase letter followed by lowercase
	// letters, digits or hyphens, at most 50 characters. Required.
	Name string `json:"name"`
}

// O11yServiceAccountsIn has no inputs; the org is the caller's own.
type O11yServiceAccountsIn struct{}

// O11yMyServiceAccountIn has no inputs; the account is the caller itself.
type O11yMyServiceAccountIn struct{}

// O11yMyServiceAccountUpdateIn renames the calling service account.
type O11yMyServiceAccountUpdateIn struct {
	// Name is the account's new name, under the same rules it was created with.
	// Required.
	Name string `json:"name"`
}

// O11yServiceAccountGetIn names one service account.
type O11yServiceAccountGetIn struct {
	// ID is the service account id.
	ID string `json:"-" url:"id"`
}

// O11yServiceAccountUpdateIn renames a service account.
type O11yServiceAccountUpdateIn struct {
	// ID is the service account id.
	ID string `json:"-" url:"id"`
	// Name is the account's new name, under the same rules it was created with.
	// Required.
	Name string `json:"name"`
}

// O11yServiceAccountDeleteIn names the service account to delete.
type O11yServiceAccountDeleteIn struct {
	// ID is the service account id.
	ID string `json:"-" url:"id"`
}

// O11yServiceAccountRolesIn names the service account whose roles to list.
type O11yServiceAccountRolesIn struct {
	// ID is the service account id.
	ID string `json:"-" url:"id"`
}

// O11yServiceAccountRoleGrantIn assigns a role to a service account. The path
// names the account; the body names the role — two different ids both spelled
// "id" on the wire, kept apart by the url/json split.
type O11yServiceAccountRoleGrantIn struct {
	// ServiceAccountID is the service account id.
	ServiceAccountID string `json:"-" url:"id"`
	// RoleID is the id of the role to assign. Required.
	RoleID string `json:"id" url:"-"`
}

// O11yServiceAccountRoleRevokeIn removes a role from a service account.
type O11yServiceAccountRoleRevokeIn struct {
	// ID is the service account id.
	ID string `json:"-" url:"id"`
	// RoleID is the id of the role to remove.
	RoleID string `json:"-" url:"rid"`
}

// O11yAPIKeyCreateIn describes the API key to mint.
type O11yAPIKeyCreateIn struct {
	// ServiceAccountID is the service account the key belongs to.
	ServiceAccountID string `json:"-" url:"id"`
	// Name is the key's name: a lowercase letter followed by lowercase letters,
	// digits or hyphens, at most 80 characters. Required.
	Name string `json:"name"`
	// ExpiresAt is when the key stops working, as a unix timestamp in seconds.
	// Zero means it never expires; a past timestamp is refused.
	ExpiresAt uint64 `json:"expiresAt"`
}

// O11yAPIKeysIn names the service account whose keys to list.
type O11yAPIKeysIn struct {
	// ID is the service account id.
	ID string `json:"-" url:"id"`
}

// O11yAPIKeyUpdateIn renames an API key or moves its expiry.
type O11yAPIKeyUpdateIn struct {
	// ServiceAccountID is the service account the key belongs to.
	ServiceAccountID string `json:"-" url:"id"`
	// KeyID is the key id.
	KeyID string `json:"-" url:"fid"`
	// Name is the key's new name, under the same rules it was created with.
	// Required.
	Name string `json:"name"`
	// ExpiresAt is when the key stops working, as a unix timestamp in seconds.
	// Zero means it never expires; a past timestamp is refused.
	ExpiresAt uint64 `json:"expiresAt"`
}

// O11yAPIKeyRevokeIn names the API key to revoke.
type O11yAPIKeyRevokeIn struct {
	// ID is the service account id.
	ID string `json:"-" url:"id"`
	// KeyID is the key id.
	KeyID string `json:"-" url:"fid"`
}

// O11yCheckIn is a batch of transactions to evaluate for the caller. The wire
// body is the bare array itself — the shape this route has always taken.
type O11yCheckIn []O11yTransaction

// O11yTransaction is one authorization question: may the caller do RELATION to
// OBJECT.
type O11yTransaction struct {
	// Relation is the verb being asked about, e.g. read, create, update, delete.
	Relation string `json:"relation"`
	// Object is the resource the verb would act on.
	Object O11yObject `json:"object"`
}

// O11yObject is one addressable resource: what kind of thing it is, and which
// one.
type O11yObject struct {
	// Resource is the resource's type and kind.
	Resource O11yResourceRef `json:"resource"`
	// Selector picks the instance — an FGA object string, wildcard allowed.
	Selector string `json:"selector"`
}

// O11yResourceRef names a resource type.
type O11yResourceRef struct {
	// Type is the resource type, e.g. role, dashboard, serviceaccount.
	Type string `json:"type"`
	// Kind is the resource kind the type belongs to.
	Kind string `json:"kind"`
}

// O11yTransactionGroup is one grant a role carries: a relation over a group of
// objects.
type O11yTransactionGroup struct {
	// Relation is the verb the grant allows.
	Relation string `json:"relation"`
	// ObjectGroup is the set of objects it allows the verb on.
	ObjectGroup O11yObjectGroup `json:"objectGroup"`
}

// O11yObjectGroup is a set of same-typed objects, named by selectors.
type O11yObjectGroup struct {
	// Resource is the objects' type and kind.
	Resource O11yResourceRef `json:"resource"`
	// Selectors pick the instances; a wildcard selects them all.
	Selectors []string `json:"selectors"`
}

// ── outputs ───────────────────────────────────────────────────────────────────
//
// Each Out is the runtime's answer NAMED, field for field, tag for tag —
// including the status/data envelope every read on this face has always
// answered with. Written flat (nothing embeds), so the document and the bytes
// agree; access_test.go re-encodes each Out and compares it byte for byte with
// what the runtime wrote. The 204 operations answer no content at all, which is
// what their callers have always received.

// O11yRoleCreateOut answers a role creation with the new role's id.
type O11yRoleCreateOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data carries the new role's id.
	Data O11yCreated `json:"data"`
}

// O11yRoleOut is one role with its grants.
type O11yRoleOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the role and its transaction groups.
	Data O11yRoleDetail `json:"data"`
}

// O11yRoleDetail is one role together with the grants it carries.
type O11yRoleDetail struct {
	// ID is the role id.
	ID string `json:"id"`
	// CreatedAt is when the role was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Name is the role's name.
	Name string `json:"name"`
	// Description says what the role is for.
	Description string `json:"description"`
	// Type is custom or managed.
	Type string `json:"type"`
	// OrgID is the org the role belongs to.
	OrgID string `json:"orgId"`
	// TransactionGroups are the grants the role carries.
	TransactionGroups []O11yTransactionGroup `json:"transactionGroups"`
}

// O11yServiceAccountCreateOut answers a service-account creation with the new
// account's id.
type O11yServiceAccountCreateOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data carries the new account's id.
	Data O11yCreated `json:"data"`
}

// O11yServiceAccountsOut is every service account in the org.
type O11yServiceAccountsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the service accounts.
	Data []O11yServiceAccount `json:"data"`
}

// O11yServiceAccountOut is one service account with the roles it holds.
type O11yServiceAccountOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the account and its role assignments.
	Data O11yServiceAccountDetail `json:"data"`
}

// O11yServiceAccount is one service account.
type O11yServiceAccount struct {
	// ID is the service account id.
	ID string `json:"id"`
	// CreatedAt is when the account was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Name is the account's name.
	Name string `json:"name"`
	// Email is the address the account authenticates as.
	Email string `json:"email"`
	// Status is active or deleted.
	Status string `json:"status"`
	// OrgID is the org the account belongs to.
	OrgID string `json:"orgId"`
}

// O11yServiceAccountDetail is one service account together with its role
// assignments.
type O11yServiceAccountDetail struct {
	// ID is the service account id.
	ID string `json:"id"`
	// CreatedAt is when the account was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Name is the account's name.
	Name string `json:"name"`
	// Email is the address the account authenticates as.
	Email string `json:"email"`
	// Status is active or deleted.
	Status string `json:"status"`
	// OrgID is the org the account belongs to.
	OrgID string `json:"orgId"`
	// ServiceAccountRoles are the account's role assignments.
	ServiceAccountRoles []O11yServiceAccountRole `json:"serviceAccountRoles"`
}

// O11yServiceAccountRole is one role assignment on a service account.
type O11yServiceAccountRole struct {
	// ID is the assignment's own id.
	ID string `json:"id"`
	// CreatedAt is when the role was assigned.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the assignment last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// ServiceAccountID is the account holding the role.
	ServiceAccountID string `json:"serviceAccountId"`
	// RoleID is the role held.
	RoleID string `json:"roleId"`
	// Role is the role itself.
	Role *O11yRole `json:"role"`
}

// O11yServiceAccountRolesOut is the roles a service account holds.
type O11yServiceAccountRolesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the roles.
	Data []O11yRole `json:"data"`
}

// O11yAPIKeyCreateOut answers a key mint with the key's id and its secret —
// shown here and never again.
type O11yAPIKeyCreateOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data carries the key's id and secret.
	Data O11yAPIKeySecret `json:"data"`
}

// O11yAPIKeySecret is a freshly minted key: its id and its secret.
type O11yAPIKeySecret struct {
	// ID is the key id.
	ID string `json:"id"`
	// Key is the secret the service account authenticates with.
	Key string `json:"key"`
}

// O11yAPIKeysOut is a service account's keys, metadata only.
type O11yAPIKeysOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the keys.
	Data []O11yAPIKey `json:"data"`
}

// O11yAPIKey is one API key's metadata; the secret is never listed.
type O11yAPIKey struct {
	// ID is the key id.
	ID string `json:"id"`
	// CreatedAt is when the key was minted.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Name is the key's name.
	Name string `json:"name"`
	// ExpiresAt is when the key stops working, as a unix timestamp in seconds;
	// zero means never.
	ExpiresAt uint64 `json:"expiresAt"`
	// LastObservedAt is when the key was last seen authenticating.
	LastObservedAt time.Time `json:"lastObservedAt"`
	// ServiceAccountID is the account the key belongs to.
	ServiceAccountID string `json:"serviceAccountId"`
}

// O11yCheckOut answers an authorization probe, one verdict per transaction
// asked, in the order they were asked.
type O11yCheckOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the verdicts.
	Data []O11yTransactionResult `json:"data"`
}

// O11yTransactionResult is one evaluated transaction.
type O11yTransactionResult struct {
	// Relation is the verb that was asked about.
	Relation string `json:"relation"`
	// Object is the resource it would act on.
	Object O11yObject `json:"object"`
	// Authorized says whether the caller may do it.
	Authorized bool `json:"authorized"`
}

// ── the seam ──────────────────────────────────────────────────────────────────

// ── role records ──────────────────────────────────────────────────────────────
//
// These lived in identity.go while the identity face still administered members
// and the roles they held. That face is gone — Hanzo IAM administers members —
// but ROLES themselves are not identity: they are o11y's own vocabulary for what
// may be done to o11y's own dashboards, alerts and views, which is this file's
// subject. So the records moved to the face that uses them.

// O11yRolesOut is a list of roles.
type O11yRolesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the roles.
	Data []O11yRole `json:"data"`
}

// O11yRole is one role.
type O11yRole struct {
	// ID is the role id.
	ID string `json:"id"`
	// CreatedAt is when the role was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Name is the role's name.
	Name string `json:"name"`
	// Description says what the role is for.
	Description string `json:"description"`
	// Type is how the role came to be — managed by the platform or custom.
	Type string `json:"type"`
	// OrgID is the org the role belongs to.
	OrgID string `json:"orgId"`
}

// O11yCreated is a newly created record, named by id.
type O11yCreated struct {
	// ID is the new record's id.
	ID string `json:"id"`
}
