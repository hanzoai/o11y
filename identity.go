package o11y

// The IDENTITY face — users, invites, passwords, roles-on-users, sessions,
// auth domains, my-org, preferences and quick filters — as TYPED ops.
//
// These forty-five operations reached traffic only through the delegation
// wildcard, and a route behind a wildcard is in no document: no SDK method, no
// CLI command, no agent tool, no reference page. Whoever a tenant is — who its
// members are, how they sign in, what they may do, what they prefer — could not
// be read or changed from anything but a hand-written HTTP call. Typing the
// face is what puts it in the document and therefore in every projection built
// from it.
//
// THE WIRE DOES NOT MOVE, the same way telemetry.go's five reads did not move
// it: no op here re-implements anything. Each hands the call to the SAME
// runtime handler the wildcard delegates to (see relay.go), so identity
// resolution, the org gate, the ROLE CHECK the mux registration declared —
// AdminAccess, ViewAccess, SelfAccess, OpenAccess — the audit record, the
// timeout and the success envelope are all still the runtime's, executed in
// the order they always were. The gate is not re-stated here because it is not
// enforced here; each op's comment names the gate its route has always had,
// and the runtime keeps enforcing it one layer in.
//
// THREE routes of the face stay on the wildcard, deliberately: the Google,
// SAML and OIDC sign-in callbacks (/complete/google, /complete/saml,
// /complete/oidc). They are browser redirect flows — every answer, success or
// refusal, is a 303 with a Location header — and a typed op declares a 2xx
// JSON contract, which would be a lie about them. The wildcard is the honest
// projection of a redirect.
//
// Registered ahead of the wildcard, and specific-beats-wildcard is what the
// router does regardless of registration order, so these paths dispatch here
// and every other path under the prefix still reaches the runtime untouched
// (identity_test.go pins both halves).

import (
	"context"
	"net/http"
	"time"

	"github.com/zap-proto/zip"
)

// mountIdentity registers the identity face's typed ops on the native router.
// Collection routes sit beside their parameterised siblings safely: the router
// matches the most specific pattern, so /users/me can never be swallowed by
// /users/:id, which is the same disambiguation the runtime's own tree makes.
func mountIdentity(app *zip.App) {
	g := under{app, o11yRoot}

	// users and invites (runtime gate per op: see each op's comment)
	zip.Post(g, "/invite", createInvite, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateInvite"))
	zip.Post(g, "/invite/bulk", createBulkInvite, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateBulkInvite"))
	zip.Get(g, "/user", listUsersDeprecated, zip.WithOperationID("ListUsersDeprecated"))
	zip.Get(g, "/users", listUsers, zip.WithOperationID("ListUsers"))
	zip.Get(g, "/user/me", getMyUserDeprecated, zip.WithOperationID("GetMyUserDeprecated"))
	zip.Get(g, "/users/me", getMyUser, zip.WithOperationID("GetMyUser"))
	zip.Post(g, "/users", createUser, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateUser"))
	zip.Put(g, "/users/me", updateMyUser, zip.WithOperationID("UpdateMyUserV2"))
	zip.Get(g, "/user/:id", getUserDeprecated, zip.WithOperationID("GetUserDeprecated"))
	zip.Get(g, "/users/:id", getUser, zip.WithOperationID("GetUser"))
	zip.Put(g, "/user/:id", updateUserDeprecated, zip.WithOperationID("UpdateUserDeprecated"))
	zip.Put(g, "/users/:id", updateUser, zip.WithOperationID("UpdateUser"))
	zip.Delete(g, "/user/:id", deleteUserDeprecated, zip.WithOperationID("DeleteUserDeprecated"))
	zip.Delete(g, "/users/:id", deleteUser, zip.WithOperationID("DeleteUser"))

	// passwords and their reset tokens
	zip.Get(g, "/getResetPasswordToken/:id", getResetTokenDeprecated, zip.WithOperationID("GetResetPasswordTokenDeprecated"))
	zip.Get(g, "/users/:id/reset_password_tokens", getResetToken, zip.WithOperationID("GetResetPasswordToken"))
	zip.Put(g, "/users/:id/reset_password_tokens", createResetToken, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateResetPasswordToken"))
	zip.Post(g, "/reset_password_tokens/verify", verifyResetToken, zip.WithOperationID("VerifyResetPasswordToken"))
	zip.Post(g, "/resetPassword", resetPassword, zip.WithOperationID("ResetPassword"))
	zip.Put(g, "/users/me/factor_password", changeMyPassword, zip.WithOperationID("UpdateMyPassword"))
	zip.Post(g, "/factor_password/forgot", forgotPassword, zip.WithOperationID("ForgotPassword"))

	// roles on users
	zip.Get(g, "/users/:id/roles", listUserRoles, zip.WithOperationID("GetRolesByUserID"))
	zip.Post(g, "/users/:id/roles", setUserRole, zip.WithOperationID("SetRoleByUserID"))
	zip.Delete(g, "/users/:id/roles/:roleId", removeUserRole, zip.WithOperationID("RemoveUserRoleByUserIDAndRoleID"))
	zip.Get(g, "/roles/:id/users", listRoleUsers, zip.WithOperationID("GetUsersByRoleID"))

	// sessions (the three /complete/* sign-in callbacks stay on the wildcard;
	// they answer with redirects, not JSON — see the package comment)
	zip.Post(g, "/sessions/email_password", createEmailPasswordSession, zip.WithOperationID("CreateSessionByEmailPassword"))
	zip.Get(g, "/sessions/context", getSessionContext, zip.WithOperationID("GetSessionContext"))
	zip.Post(g, "/sessions/rotate", rotateSession, zip.WithOperationID("RotateSession"))
	zip.Delete(g, "/sessions", deleteSession, zip.WithOperationID("DeleteSession"))

	// auth domains
	zip.Get(g, "/domains", listAuthDomains, zip.WithOperationID("ListAuthDomains"))
	zip.Post(g, "/domains", createAuthDomain, zip.WithStatus(http.StatusCreated), zip.WithOperationID("CreateAuthDomain"))
	zip.Get(g, "/domains/:id", getAuthDomain, zip.WithOperationID("GetAuthDomain"))
	zip.Put(g, "/domains/:id", updateAuthDomain, zip.WithOperationID("UpdateAuthDomain"))
	zip.Delete(g, "/domains/:id", deleteAuthDomain, zip.WithOperationID("DeleteAuthDomain"))

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

// ── users and invites ─────────────────────────────────────────────────────────

// createInvite invites one person to the caller's org by email, with the role
// they will hold when they accept. Deprecated in favor of creating users
// directly; kept because callers still hold it. Admin gate, enforced by the
// runtime this op relays to.
func createInvite(ctx context.Context, in *O11yInviteIn) (*O11yInviteOut, error) {
	out := new(O11yInviteOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/invite", nil, in, out)
}

// createBulkInvite invites several people to the caller's org in one call,
// refusing the whole batch when any email repeats. Deprecated alongside
// createInvite. Admin gate.
func createBulkInvite(ctx context.Context, in *O11yBulkInviteIn) (*O11yAck, error) {
	out := new(O11yAck)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/invite/bulk", nil, in, out)
}

// listUsersDeprecated lists the org's members with their single legacy role.
// Deprecated in favor of listUsers, which answers without the role. Admin gate.
func listUsersDeprecated(ctx context.Context, _ *struct{}) (*O11yDeprecatedUsersOut, error) {
	out := new(O11yDeprecatedUsersOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/user", nil, nil, out)
}

// listUsers lists the caller's org members. Admin gate.
func listUsers(ctx context.Context, _ *struct{}) (*O11yUsersOut, error) {
	out := new(O11yUsersOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/users", nil, nil, out)
}

// getMyUserDeprecated returns the calling user with their single legacy role.
// Deprecated in favor of getMyUser. Open to any authenticated caller.
func getMyUserDeprecated(ctx context.Context, _ *struct{}) (*O11yDeprecatedUserOut, error) {
	out := new(O11yDeprecatedUserOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/user/me", nil, nil, out)
}

// getMyUser returns the calling user together with every role they hold. Open
// to any authenticated caller.
func getMyUser(ctx context.Context, _ *struct{}) (*O11yUserWithRolesOut, error) {
	out := new(O11yUserWithRolesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/users/me", nil, nil, out)
}

// createUser creates a member of the caller's org in the pending-invite state
// and mails them their invitation; the answer is the new user's id. Admin gate.
func createUser(ctx context.Context, in *O11yPostableUser) (*O11yCreatedOut, error) {
	out := new(O11yCreatedOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/users", nil, in, out)
}

// updateMyUser renames the calling user. Open to any authenticated caller.
func updateMyUser(ctx context.Context, in *O11yUpdatableUser) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/users/me", nil, in, nil)
}

// getUserDeprecated returns one org member with their single legacy role, by
// user id. Admins may read anyone; a non-admin only themselves (the runtime's
// self-access gate).
func getUserDeprecated(ctx context.Context, in *O11yUserRef) (*O11yDeprecatedUserOut, error) {
	out := new(O11yDeprecatedUserOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/user/"+in.ID, nil, nil, out)
}

// getUser returns one org member together with every role they hold, by user
// id. Admin gate.
func getUser(ctx context.Context, in *O11yUserRef) (*O11yUserWithRolesOut, error) {
	out := new(O11yUserWithRolesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/users/"+in.ID, nil, nil, out)
}

// updateUserDeprecated renames one org member and may move their legacy role,
// answering with the updated record. Admins may update anyone; a non-admin
// only themselves (the runtime's self-access gate).
func updateUserDeprecated(ctx context.Context, in *O11yDeprecatedUserUpdate) (*O11yDeprecatedUserOut, error) {
	out := new(O11yDeprecatedUserOut)
	return out, relay(ctx, http.MethodPut, o11yRoot+"/user/"+in.ID, nil, in, out)
}

// updateUser renames one org member, by user id — someone else, never the
// caller, who renames themselves through updateMyUser. Admin gate.
func updateUser(ctx context.Context, in *O11yUserUpdate) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/users/"+in.ID, nil, in, nil)
}

// deleteUserDeprecated removes one org member, by user id. The same operation
// as deleteUser on the legacy singular path. Admin gate.
func deleteUserDeprecated(ctx context.Context, in *O11yUserRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/user/"+in.ID, nil, nil, nil)
}

// deleteUser removes one org member, by user id. Admin gate.
func deleteUser(ctx context.Context, in *O11yUserRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/users/"+in.ID, nil, nil, nil)
}

// ── passwords and their reset tokens ──────────────────────────────────────────

// getResetTokenDeprecated returns a user's password-reset token, creating one
// if none is live. Deprecated in favor of the reset_password_tokens pair,
// which separates reading from minting. Admin gate.
func getResetTokenDeprecated(ctx context.Context, in *O11yUserRef) (*O11yResetTokenOut, error) {
	out := new(O11yResetTokenOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/getResetPasswordToken/"+in.ID, nil, nil, out)
}

// getResetToken returns the reset-password token a user already has; absent
// one, the answer is a not-found rather than a fresh token. Admin gate.
func getResetToken(ctx context.Context, in *O11yUserRef) (*O11yResetTokenOut, error) {
	out := new(O11yResetTokenOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/users/"+in.ID+"/reset_password_tokens", nil, nil, out)
}

// createResetToken creates or regenerates a user's reset-password token: a
// live token is returned as it is, an expired one is replaced. Admin gate.
func createResetToken(ctx context.Context, in *O11yUserRef) (*O11yResetTokenOut, error) {
	out := new(O11yResetTokenOut)
	return out, relay(ctx, http.MethodPut, o11yRoot+"/users/"+in.ID+"/reset_password_tokens", nil, nil, out)
}

// verifyResetToken checks that a reset-password token exists and has not
// expired, without consuming it. Unauthenticated: the token is the proof.
func verifyResetToken(ctx context.Context, in *O11yResetTokenRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPost, o11yRoot+"/reset_password_tokens/verify", nil, in, nil)
}

// resetPassword sets a new password for whoever the reset token was minted
// for, consuming the token. Unauthenticated: the token is the proof.
func resetPassword(ctx context.Context, in *O11yResetPasswordIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPost, o11yRoot+"/resetPassword", nil, in, nil)
}

// changeMyPassword replaces the calling user's password, refusing when the old
// one does not match. Open to any authenticated caller.
func changeMyPassword(ctx context.Context, in *O11yChangePasswordIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/users/me/factor_password", nil, in, nil)
}

// forgotPassword starts the forgotten-password flow: the named user is mailed
// a reset link. Unauthenticated by design, and deliberately quiet about
// whether the address exists.
func forgotPassword(ctx context.Context, in *O11yForgotPasswordIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPost, o11yRoot+"/factor_password/forgot", nil, in, nil)
}

// ── roles on users ────────────────────────────────────────────────────────────

// listUserRoles returns every role one org member holds, by user id. Admin
// gate.
func listUserRoles(ctx context.Context, in *O11yUserRef) (*O11yRolesOut, error) {
	out := new(O11yRolesOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/users/"+in.ID+"/roles", nil, nil, out)
}

// setUserRole assigns a role, by role name, to one org member — someone else,
// never the caller. Admin gate.
func setUserRole(ctx context.Context, in *O11ySetRoleIn) (*O11yAck, error) {
	out := new(O11yAck)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/users/"+in.ID+"/roles", nil, in, out)
}

// removeUserRole takes a role away from one org member, by user id and role
// id — someone else, never the caller. Admin gate.
func removeUserRole(ctx context.Context, in *O11yUserRoleRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/users/"+in.ID+"/roles/"+in.RoleID, nil, nil, nil)
}

// listRoleUsers returns every org member holding a role, by role id. Admin
// gate.
func listRoleUsers(ctx context.Context, in *O11yRoleUsersIn) (*O11yUsersOut, error) {
	out := new(O11yUsersOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/roles/"+in.ID+"/users", nil, nil, out)
}

// ── sessions ──────────────────────────────────────────────────────────────────

// createEmailPasswordSession signs a user in with email and password and
// answers with the session's token pair. Unauthenticated: this call is how
// authentication begins.
func createEmailPasswordSession(ctx context.Context, in *O11yEmailPasswordSessionIn) (*O11yTokenOut, error) {
	out := new(O11yTokenOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/sessions/email_password", nil, in, out)
}

// getSessionContext tells a sign-in page what an email address can do: which
// orgs the address belongs to and, per org, which password and SSO routes are
// open to it. Unauthenticated: it runs before any session exists.
func getSessionContext(ctx context.Context, in *O11ySessionContextIn) (*O11ySessionContextOut, error) {
	out := new(O11ySessionContextOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/sessions/context", query(
		"email", in.Email,
		"ref", in.Ref,
	), nil, out)
}

// rotateSession exchanges a refresh token for a fresh token pair, retiring the
// old pair. The access token being rotated identifies the session.
func rotateSession(ctx context.Context, in *O11yRotateSessionIn) (*O11yTokenOut, error) {
	out := new(O11yTokenOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/sessions/rotate", nil, in, out)
}

// deleteSession signs the calling session out, invalidating its tokens. The
// access token on the call names the session to end.
func deleteSession(ctx context.Context, _ *struct{}) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/sessions", nil, nil, nil)
}

// ── auth domains ──────────────────────────────────────────────────────────────

// listAuthDomains lists the org's auth domains — the email domains whose SSO
// configuration this org owns. Admin gate.
func listAuthDomains(ctx context.Context, _ *struct{}) (*O11yAuthDomainsOut, error) {
	out := new(O11yAuthDomainsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/domains", nil, nil, out)
}

// createAuthDomain claims an email domain for the org and configures how its
// users sign in; the answer is the new domain's id. Admin gate.
func createAuthDomain(ctx context.Context, in *O11yPostableAuthDomain) (*O11yCreatedOut, error) {
	out := new(O11yCreatedOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/domains", nil, in, out)
}

// getAuthDomain returns one auth domain with its SSO configuration, by id.
// Admin gate.
func getAuthDomain(ctx context.Context, in *O11yDomainRef) (*O11yAuthDomainOut, error) {
	out := new(O11yAuthDomainOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/domains/"+in.ID, nil, nil, out)
}

// updateAuthDomain replaces one auth domain's SSO configuration, by id. Admin
// gate.
func updateAuthDomain(ctx context.Context, in *O11yUpdatableAuthDomain) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPut, o11yRoot+"/domains/"+in.ID, nil, in, nil)
}

// deleteAuthDomain releases an email domain and discards its SSO
// configuration, by id. Admin gate.
func deleteAuthDomain(ctx context.Context, in *O11yDomainRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, o11yRoot+"/domains/"+in.ID, nil, nil, nil)
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
//
// Every type carries the O11y qualifier because these names enter a fleet-wide
// document, where one name may have exactly one shape. A field bound from the
// URL says so with `url:` and stays out of the body with `json:"-"` — the URL
// is the addressing authority, and a body must not carry a second copy of it.

// O11yInviteIn is one person to invite.
type O11yInviteIn struct {
	// Name is the invitee's display name.
	Name string `json:"name"`
	// Email is the address the invitation goes to.
	Email string `json:"email"`
	// Role is the role they will hold on accepting — ADMIN, EDITOR or VIEWER.
	Role string `json:"role"`
	// FrontendBaseUrl is the console origin the invite link is built on.
	FrontendBaseUrl string `json:"frontendBaseUrl"`
}

// O11yBulkInviteIn is a batch of people to invite in one call.
type O11yBulkInviteIn struct {
	// Invites are the invitations to create; an email may appear only once.
	Invites []O11yInviteIn `json:"invites"`
}

// O11yPostableUser is a member to create in the pending-invite state.
type O11yPostableUser struct {
	// DisplayName is the new member's display name.
	DisplayName string `json:"displayName"`
	// Email is the new member's address. Required.
	Email string `json:"email"`
	// FrontendBaseUrl is the console origin the invite link is built on.
	FrontendBaseUrl string `json:"frontendBaseUrl"`
	// UserRoles are the roles the member starts with, each by id.
	UserRoles []O11yRoleID `json:"userRoles"`
}

// O11yRoleID names one role by id.
type O11yRoleID struct {
	// ID is the role id.
	ID string `json:"id"`
}

// O11yUpdatableUser is the one mutable thing on the calling user.
type O11yUpdatableUser struct {
	// DisplayName is the new display name.
	DisplayName string `json:"displayName"`
}

// O11yUserRef names one user by id.
type O11yUserRef struct {
	// ID is the user id.
	ID string `json:"-" url:"id" validate:"required"`
}

// O11yUserUpdate renames one user, addressed by id.
type O11yUserUpdate struct {
	// ID is the user id.
	ID string `json:"-" url:"id" validate:"required"`
	// DisplayName is the new display name.
	DisplayName string `json:"displayName"`
}

// O11yDeprecatedUserUpdate renames one user and may move their legacy role,
// addressed by id.
type O11yDeprecatedUserUpdate struct {
	// ID is the user id.
	ID string `json:"-" url:"id" validate:"required"`
	// DisplayName is the new display name; empty leaves it unchanged.
	DisplayName string `json:"displayName"`
	// Role is the legacy role to move to — ADMIN, EDITOR or VIEWER; empty
	// leaves it unchanged.
	Role string `json:"role"`
}

// O11yResetTokenRef is a reset-password token to verify.
type O11yResetTokenRef struct {
	// Token is the reset-password token.
	Token string `json:"token"`
}

// O11yResetPasswordIn redeems a reset token for a new password.
type O11yResetPasswordIn struct {
	// Password is the new password.
	Password string `json:"password"`
	// Token is the reset-password token authorizing the change.
	Token string `json:"token"`
}

// O11yChangePasswordIn replaces the calling user's password.
type O11yChangePasswordIn struct {
	// OldPassword is the current password; the change is refused when it does
	// not match.
	OldPassword string `json:"oldPassword"`
	// NewPassword is the password to set.
	NewPassword string `json:"newPassword"`
}

// O11yForgotPasswordIn asks for a reset link by email.
type O11yForgotPasswordIn struct {
	// OrgID is the org the address belongs to. Required.
	OrgID string `json:"orgId"`
	// Email is the address to mail the reset link to. Required.
	Email string `json:"email"`
	// FrontendBaseURL is the console origin the reset link is built on.
	FrontendBaseURL string `json:"frontendBaseURL"`
}

// O11ySetRoleIn assigns a role by name to a user addressed by id.
type O11ySetRoleIn struct {
	// ID is the user id.
	ID string `json:"-" url:"id" validate:"required"`
	// Name is the role name to assign.
	Name string `json:"name"`
}

// O11yUserRoleRef names one role held by one user, both by id.
type O11yUserRoleRef struct {
	// ID is the user id.
	ID string `json:"-" url:"id" validate:"required"`
	// RoleID is the role id.
	RoleID string `json:"-" url:"roleId" validate:"required"`
}

// O11yRoleUsersIn names one role by id.
type O11yRoleUsersIn struct {
	// ID is the role id.
	ID string `json:"-" url:"id" validate:"required"`
}

// O11yEmailPasswordSessionIn is an email-and-password sign-in.
type O11yEmailPasswordSessionIn struct {
	// Email is the account's address. Required.
	Email string `json:"email"`
	// Password is the account's password. Required.
	Password string `json:"password"`
	// OrgID picks the org to sign into when the address belongs to several.
	OrgID string `json:"orgId"`
}

// O11ySessionContextIn asks what an email address can do on a sign-in page.
type O11ySessionContextIn struct {
	// Email is the address about to sign in. Required.
	Email string `json:"email"`
	// Ref is the page the sign-in started from, carried into SSO redirects.
	Ref string `json:"ref"`
}

// O11yRotateSessionIn exchanges a refresh token for a fresh pair.
type O11yRotateSessionIn struct {
	// RefreshToken is the refresh token being redeemed.
	RefreshToken string `json:"refreshToken"`
}

// O11yPostableAuthDomain claims an email domain and configures it.
type O11yPostableAuthDomain struct {
	// Config is the domain's SSO configuration.
	Config O11yAuthDomainConfig `json:"config"`
	// Name is the email domain being claimed, e.g. example.com.
	Name string `json:"name"`
}

// O11yDomainRef names one auth domain by id.
type O11yDomainRef struct {
	// ID is the auth domain id.
	ID string `json:"-" url:"id" validate:"required"`
}

// O11yUpdatableAuthDomain replaces one auth domain's configuration, addressed
// by id.
type O11yUpdatableAuthDomain struct {
	// ID is the auth domain id.
	ID string `json:"-" url:"id" validate:"required"`
	// Config is the SSO configuration to store.
	Config O11yAuthDomainConfig `json:"config"`
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
//
// Each Out is the runtime's answer NAMED, field for field, tag for tag —
// including the status/data envelope every JSON answer on this face has always
// worn. Nothing embeds and nothing is borrowed from the storage types: an
// embedded pointer publishes a hollow schema, and a storage type's custom
// marshalers say string where reflection sees struct. Written flat, the
// document and the bytes agree, and identity_test.go re-encodes Outs and
// compares them byte for byte with what the runtime wrote.

// O11yAck is the bare success envelope — an answer that carries no data.
type O11yAck struct {
	// Status is "success".
	Status string `json:"status"`
}

// O11yInviteOut is one created invitation.
type O11yInviteOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the invitation.
	Data O11yInvite `json:"data,omitempty"`
}

// O11yInvite is one pending invitation.
type O11yInvite struct {
	// ID is the invitation's id.
	ID string `json:"id"`
	// CreatedAt is when it was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when it last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Name is the invitee's display name.
	Name string `json:"name"`
	// Email is the address it was sent to.
	Email string `json:"email"`
	// Token is the secret that redeems it.
	Token string `json:"token"`
	// Role is the role the invitee will hold.
	Role string `json:"role"`
	// OrgID is the org they are invited into.
	OrgID string `json:"orgId"`
	// InviteLink is the full link mailed to them.
	InviteLink string `json:"inviteLink"`
}

// O11yUsersOut is a list of org members.
type O11yUsersOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the members.
	Data []O11yUser `json:"data"`
}

// O11yUser is one org member.
type O11yUser struct {
	// ID is the user id.
	ID string `json:"id"`
	// DisplayName is what the console shows for them.
	DisplayName string `json:"displayName"`
	// Email is their address.
	Email string `json:"email"`
	// OrgID is the org they belong to.
	OrgID string `json:"orgId"`
	// IsRoot marks the org's root user, which cannot be deleted or demoted.
	IsRoot bool `json:"isRoot"`
	// Status is their lifecycle state — active, pending_invite or deleted.
	Status string `json:"status"`
	// CreatedAt is when they joined.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when their record last changed.
	UpdatedAt time.Time `json:"updatedAt"`
}

// O11yDeprecatedUsersOut is a list of org members in the legacy single-role
// shape.
type O11yDeprecatedUsersOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the members.
	Data []O11yDeprecatedUser `json:"data"`
}

// O11yDeprecatedUserOut is one org member in the legacy single-role shape.
type O11yDeprecatedUserOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the member.
	Data O11yDeprecatedUser `json:"data,omitempty"`
}

// O11yDeprecatedUser is one org member with their single legacy role.
type O11yDeprecatedUser struct {
	// ID is the user id.
	ID string `json:"id"`
	// DisplayName is what the console shows for them.
	DisplayName string `json:"displayName"`
	// Email is their address.
	Email string `json:"email"`
	// OrgID is the org they belong to.
	OrgID string `json:"orgId"`
	// IsRoot marks the org's root user.
	IsRoot bool `json:"isRoot"`
	// Status is their lifecycle state — active, pending_invite or deleted.
	Status string `json:"status"`
	// CreatedAt is when they joined.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when their record last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Role is their legacy role — ADMIN, EDITOR or VIEWER.
	Role string `json:"role"`
}

// O11yUserWithRolesOut is one org member with every role they hold.
type O11yUserWithRolesOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the member.
	Data O11yUserWithRoles `json:"data,omitempty"`
}

// O11yUserWithRoles is one org member and their role assignments.
type O11yUserWithRoles struct {
	// ID is the user id.
	ID string `json:"id"`
	// DisplayName is what the console shows for them.
	DisplayName string `json:"displayName"`
	// Email is their address.
	Email string `json:"email"`
	// OrgID is the org they belong to.
	OrgID string `json:"orgId"`
	// IsRoot marks the org's root user.
	IsRoot bool `json:"isRoot"`
	// Status is their lifecycle state — active, pending_invite or deleted.
	Status string `json:"status"`
	// CreatedAt is when they joined.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when their record last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// UserRoles are their role assignments.
	UserRoles []O11yUserRole `json:"userRoles"`
}

// O11yUserRole is one role assignment on one user.
type O11yUserRole struct {
	// ID is the assignment's own id.
	ID string `json:"id"`
	// UserID is the user holding the role.
	UserID string `json:"userId"`
	// RoleID is the role held.
	RoleID string `json:"roleId"`
	// CreatedAt is when it was assigned.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the assignment last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Role is the role itself.
	Role *O11yRole `json:"role"`
}

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

// O11yCreatedOut answers a create with the new record's id.
type O11yCreatedOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data carries the id.
	Data O11yCreated `json:"data,omitempty"`
}

// O11yCreated is a newly created record, named by id.
type O11yCreated struct {
	// ID is the new record's id.
	ID string `json:"id"`
}

// O11yResetTokenOut is one reset-password token.
type O11yResetTokenOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the token.
	Data O11yResetToken `json:"data,omitempty"`
}

// O11yResetToken is a password-reset grant.
type O11yResetToken struct {
	// ID is the grant's id.
	ID string `json:"id"`
	// Token is the secret that redeems it.
	Token string `json:"token"`
	// PasswordID is the password record it resets.
	PasswordID string `json:"passwordId"`
	// ExpiresAt is when it stops working.
	ExpiresAt time.Time `json:"expiresAt"`
}

// O11yTokenOut is a session's token pair.
type O11yTokenOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the pair.
	Data O11yToken `json:"data,omitempty"`
}

// O11yToken is the credential pair a session lives on.
type O11yToken struct {
	// TokenType is how to present the access token, e.g. bearer.
	TokenType string `json:"tokenType"`
	// AccessToken authenticates requests until it expires.
	AccessToken string `json:"accessToken"`
	// RefreshToken buys the next pair via rotateSession.
	RefreshToken string `json:"refreshToken"`
	// ExpiresIn is the access token's lifetime in seconds.
	ExpiresIn int `json:"expiresIn"`
}

// O11ySessionContextOut is what a sign-in page may offer an email address.
type O11ySessionContextOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the context.
	Data O11ySessionContext `json:"data,omitempty"`
}

// O11ySessionContext says whether an address is known and how each of its orgs
// signs in.
type O11ySessionContext struct {
	// Exists says whether any account carries the address.
	Exists bool `json:"exists"`
	// Orgs are the orgs the address belongs to, each with its sign-in routes.
	Orgs []O11ySessionOrg `json:"orgs"`
}

// O11ySessionOrg is one org an address may sign into.
type O11ySessionOrg struct {
	// ID is the org id.
	ID string `json:"id"`
	// Name is the org's display name.
	Name string `json:"name"`
	// AuthNSupport lists the org's open sign-in routes.
	AuthNSupport O11yAuthNSupport `json:"authNSupport"`
	// Warning reports an org whose SSO is configured but not currently usable,
	// in the platform's error shape.
	Warning *O11yErrorDetail `json:"warning,omitempty"`
}

// O11yAuthNSupport is the sign-in routes an org offers.
type O11yAuthNSupport struct {
	// Callback are the SSO routes; each is begun by visiting its URL.
	Callback []O11yCallbackAuthN `json:"callback"`
	// Password are the password routes.
	Password []O11yPasswordAuthN `json:"password"`
}

// O11yCallbackAuthN is one SSO sign-in route.
type O11yCallbackAuthN struct {
	// Provider is the route's provider — google_auth, saml or oidc.
	Provider string `json:"provider"`
	// URL is where the browser goes to begin the flow.
	URL string `json:"url"`
}

// O11yPasswordAuthN is one password sign-in route.
type O11yPasswordAuthN struct {
	// Provider is the route's provider, e.g. email_password.
	Provider string `json:"provider"`
}

// O11yErrorDetail is the platform's error shape, carried inline when an answer
// reports a problem without failing.
type O11yErrorDetail struct {
	// Type is the error's category, e.g. invalid_input, not_found.
	Type string `json:"type"`
	// Code is the machine-readable code.
	Code string `json:"code"`
	// Message is the human-readable reason.
	Message string `json:"message"`
	// Url points at documentation for the error, when there is any.
	Url string `json:"url,omitempty"`
	// Errors are further details, one message and its suggestions each.
	Errors []O11yErrorItem `json:"errors"`
	// Retry says when it is worth trying again, for errors that pass.
	Retry *O11yRetry `json:"retry,omitempty"`
	// Suggestions say what to try instead.
	Suggestions []string `json:"suggestions"`
}

// O11yRetry is the platform's retry hint on a transient error.
type O11yRetry struct {
	// Delay is how long to wait before retrying, in nanoseconds.
	Delay int64 `json:"delay"`
}

// O11yErrorItem is one detail of an error.
type O11yErrorItem struct {
	// Message is the detail.
	Message string `json:"message"`
	// Suggestions say what to try about this detail.
	Suggestions []string `json:"suggestions"`
}

// O11yAuthDomainsOut is a list of auth domains.
type O11yAuthDomainsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the domains.
	Data []O11yAuthDomain `json:"data"`
}

// O11yAuthDomainOut is one auth domain.
type O11yAuthDomainOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the domain.
	Data O11yAuthDomain `json:"data,omitempty"`
}

// O11yAuthDomain is one claimed email domain and its SSO configuration.
type O11yAuthDomain struct {
	// ID is the auth domain id.
	ID string `json:"id"`
	// Name is the email domain, e.g. example.com.
	Name string `json:"name"`
	// OrgID is the org that claimed it.
	OrgID string `json:"orgId"`
	// CreatedAt is when it was claimed.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when its configuration last changed.
	UpdatedAt time.Time `json:"updatedAt"`
	// Config is the domain's SSO configuration.
	Config O11yAuthDomainConfig `json:"config"`
	// AuthNProviderInfo is provider detail the console needs to finish setup.
	AuthNProviderInfo *O11yAuthNProviderInfo `json:"authNProviderInfo"`
}

// O11yAuthNProviderInfo is provider-specific setup detail.
type O11yAuthNProviderInfo struct {
	// RelayStatePath is the relay-state path a SAML IdP must be configured
	// with, when the provider needs one.
	RelayStatePath *string `json:"relayStatePath"`
}

// O11yAuthDomainConfig is how a domain's users sign in. Exactly one provider
// block matches SSOType; the others are null.
type O11yAuthDomainConfig struct {
	// SSOEnabled turns enforced SSO on for the domain.
	SSOEnabled bool `json:"ssoEnabled"`
	// SSOType picks the provider — saml, google_auth or oidc.
	SSOType string `json:"ssoType"`
	// SAML is the SAML provider's settings, when SSOType is saml.
	SAML *O11ySAMLConfig `json:"samlConfig"`
	// Google is the Google provider's settings, when SSOType is google_auth.
	Google *O11yGoogleConfig `json:"googleAuthConfig"`
	// OIDC is the OIDC provider's settings, when SSOType is oidc.
	OIDC *O11yOIDCConfig `json:"oidcConfig"`
	// RoleMapping maps the provider's groups onto roles for new users.
	RoleMapping *O11yRoleMapping `json:"roleMapping"`
}

// O11ySAMLConfig is a SAML identity provider.
type O11ySAMLConfig struct {
	// SamlEntity is the IdP's entityID.
	SamlEntity string `json:"samlEntity"`
	// SamlIdp is the IdP's single-sign-on endpoint.
	SamlIdp string `json:"samlIdp"`
	// SamlCert is the IdP's signing certificate.
	SamlCert string `json:"samlCert"`
	// InsecureSkipAuthNRequestsSigned skips signing outgoing AuthN requests,
	// for IdPs that refuse signed ones.
	InsecureSkipAuthNRequestsSigned bool `json:"insecureSkipAuthNRequestsSigned"`
	// AttributeMapping names the assertion attributes to read identity from.
	AttributeMapping O11yAttributeMapping `json:"attributeMapping"`
}

// O11yGoogleConfig is a Google Workspace identity provider.
type O11yGoogleConfig struct {
	// ClientID is the OAuth application's id.
	ClientID string `json:"clientId"`
	// ClientSecret is the OAuth application's secret.
	ClientSecret string `json:"clientSecret"`
	// RedirectURI is the callback the flow returns to.
	RedirectURI string `json:"redirectURI"`
	// FetchGroups reads the user's Workspace groups for role mapping.
	FetchGroups bool `json:"fetchGroups"`
	// ServiceAccountJSON is the service-account credential used to read
	// groups, when FetchGroups is on.
	ServiceAccountJSON string `json:"serviceAccountJson,omitempty"`
	// DomainToAdminEmail maps each Workspace domain to the admin the service
	// account impersonates; "*" is the fallback.
	DomainToAdminEmail map[string]string `json:"domainToAdminEmail,omitempty"`
	// FetchTransitiveGroupMembership also reads groups held through other
	// groups.
	FetchTransitiveGroupMembership bool `json:"fetchTransitiveGroupMembership,omitempty"`
	// AllowedGroups, when set, admits only members of these groups.
	AllowedGroups []string `json:"allowedGroups,omitempty"`
	// InsecureSkipEmailVerified admits addresses Google has not verified.
	InsecureSkipEmailVerified bool `json:"insecureSkipEmailVerified"`
}

// O11yOIDCConfig is an OIDC identity provider.
type O11yOIDCConfig struct {
	// Issuer is the provider's issuer URL.
	Issuer string `json:"issuer"`
	// IssuerAlias overrides the issuer for providers whose discovery document
	// disagrees with their issuer URL.
	IssuerAlias string `json:"issuerAlias"`
	// ClientID is the OAuth application's id.
	ClientID string `json:"clientId"`
	// ClientSecret is the OAuth application's secret.
	ClientSecret string `json:"clientSecret"`
	// ClaimMapping names the token claims to read identity from.
	ClaimMapping O11yAttributeMapping `json:"claimMapping"`
	// InsecureSkipEmailVerified admits addresses the provider has not
	// verified.
	InsecureSkipEmailVerified bool `json:"insecureSkipEmailVerified"`
	// GetUserInfo also queries the userinfo endpoint, for providers whose id
	// tokens are thin.
	GetUserInfo bool `json:"getUserInfo"`
}

// O11yAttributeMapping names where identity lives in a provider's claims or
// assertion attributes.
type O11yAttributeMapping struct {
	// Email is the key carrying the email; defaults to "email".
	Email string `json:"email"`
	// Name is the key carrying the display name; defaults to "name".
	Name string `json:"name"`
	// Groups is the key carrying the group list; defaults to "groups".
	Groups string `json:"groups"`
	// Role is the key carrying the role; defaults to "role".
	Role string `json:"role"`
}

// O11yRoleMapping decides the role a new SSO user starts with.
type O11yRoleMapping struct {
	// DefaultRole is the role when no group mapping applies.
	DefaultRole string `json:"defaultRole"`
	// GroupMappings maps a provider group name to a role name.
	GroupMappings map[string]string `json:"groupMappings"`
	// UseRoleAttribute reads the role straight from the provider's role claim
	// instead of the group mappings.
	UseRoleAttribute bool `json:"useRoleAttribute"`
}

// O11yOrganizationOut is one organization.
type O11yOrganizationOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the organization.
	Data O11yOrganization `json:"data,omitempty"`
}

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

// ── the seam ──────────────────────────────────────────────────────────────────
