// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package iamidentn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// fakeOrgStore is an in-memory OrgResolver + OrgCreator.
type fakeOrgStore struct {
	mu       sync.Mutex
	orgs     map[string]*types.Organization // by id
	getCalls int
	creates  []*types.Organization
}

func newFakeOrgStore() *fakeOrgStore {
	return &fakeOrgStore{orgs: map[string]*types.Organization{}}
}

func (f *fakeOrgStore) GetByIDOrName(_ context.Context, id valuer.UUID, name string) (*types.Organization, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	if org, ok := f.orgs[id.String()]; ok {
		return org, false, nil
	}
	for _, org := range f.orgs {
		if org.Name == name {
			return org, true, nil
		}
	}
	return nil, false, errors.NewNotFoundf(errors.CodeNotFound, "organization not found")
}

func (f *fakeOrgStore) Create(ctx context.Context, org *types.Organization, createManagedRoles func(context.Context, valuer.UUID) error) error {
	f.mu.Lock()
	if _, ok := f.orgs[org.ID.String()]; ok {
		f.mu.Unlock()
		return errors.Newf(errors.TypeAlreadyExists, errors.CodeAlreadyExists, "already exists")
	}
	f.orgs[org.ID.String()] = org
	f.creates = append(f.creates, org)
	f.mu.Unlock()
	return createManagedRoles(ctx, org.ID)
}

// fakeAuthorizer records managed-role bootstraps.
type fakeAuthorizer struct {
	mu           sync.Mutex
	managedRoles int
}

func (f *fakeAuthorizer) CreateManagedRoles(_ context.Context, _ valuer.UUID, _ []*authtypes.Role) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.managedRoles++
	return nil
}

// fakeUserStore is an in-memory UserResolver + UserCreator. CreateUser stands in
// for the real setter, which performs the Hanzo IAM grant AND the local user_role
// rows in the one call — so what this records IS the founding grant.
type fakeUserStore struct {
	mu    sync.Mutex
	users map[string]*types.User // by "<orgID>/<userID>", the key GET /users/me reads
	seats []seatCall
}

type seatCall struct {
	orgID  valuer.UUID
	userID valuer.UUID
	email  string
	roles  []string
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{users: map[string]*types.User{}}
}

func userKey(orgID, userID valuer.UUID) string { return orgID.String() + "/" + userID.String() }

func (f *fakeUserStore) GetUserByOrgIDAndID(_ context.Context, orgID, userID valuer.UUID) (*types.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[userKey(orgID, userID)]; ok {
		return u, nil
	}
	return nil, errors.NewNotFoundf(types.ErrCodeUserNotFound, "user not found")
}

func (f *fakeUserStore) CreateUser(_ context.Context, u *types.User, opts ...user.CreateUserOption) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := userKey(u.OrgID, u.ID)
	if _, ok := f.users[k]; ok {
		return errors.Newf(errors.TypeAlreadyExists, errors.CodeAlreadyExists, "already exists")
	}
	f.users[k] = u
	f.seats = append(f.seats, seatCall{orgID: u.OrgID, userID: u.ID, email: u.Email.String(), roles: user.NewCreateUserOptions(opts...).RoleNames})
	return nil
}

func newProvider(t *testing.T, store *fakeOrgStore, authorizer *fakeAuthorizer, users *fakeUserStore) *provider {
	t.Helper()
	p, err := New(instrumentationtest.New().ToProviderSettings(), store, store, authorizer, users, users)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p.(*provider)
}

func requestWithSession(org, user, email string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/v1/o11y/observations", nil)
	if org != "" {
		req.Header.Set(headerOrgID, org)
	}
	if user != "" {
		req.Header.Set(headerUserID, user)
	}
	if email != "" {
		req.Header.Set(headerEmail, email)
	}
	return req
}

// A Hanzo IAM session (gateway-injected headers) on a fresh datastore
// auto-provisions the org, grants the user admin, and yields a scoped identity —
// no onboarding, no setup, no native user record.
func TestGetIdentity_AutoProvisionsAndAuthorizes(t *testing.T) {
	store := newFakeOrgStore()
	authorizer := &fakeAuthorizer{}
	users := newFakeUserStore()
	p := newProvider(t, store, authorizer, users)

	const userUUID = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	identity, err := p.GetIdentity(requestWithSession("hanzo", userUUID, "z@hanzo.ai"))
	if err != nil {
		t.Fatalf("GetIdentity: %v", err)
	}

	wantOrg := toUUID("org", "hanzo")
	if identity.OrgID != wantOrg {
		t.Fatalf("org id = %s, want deterministic %s", identity.OrgID, wantOrg)
	}
	if identity.UserID.String() != userUUID {
		t.Fatalf("user id = %s, want passthrough %s", identity.UserID, userUUID)
	}
	if identity.Email.String() != "z@hanzo.ai" {
		t.Fatalf("email = %s, want z@hanzo.ai", identity.Email)
	}
	if identity.Principal != authtypes.PrincipalUser {
		t.Fatalf("principal = %s, want user", identity.Principal.StringValue())
	}
	if identity.IdenNProvider != authtypes.IdentNProviderIAM {
		t.Fatalf("provider = %s, want iam", identity.IdenNProvider.StringValue())
	}

	// Org created once, with managed roles bootstrapped.
	if len(store.creates) != 1 {
		t.Fatalf("org creates = %d, want 1", len(store.creates))
	}
	if store.creates[0].Name != "hanzo" || store.creates[0].ID != wantOrg {
		t.Fatalf("created org = %+v, want name=hanzo id=%s", store.creates[0], wantOrg)
	}
	if authorizer.managedRoles != 1 {
		t.Fatalf("managed role bootstraps = %d, want 1", authorizer.managedRoles)
	}

	// User SEATED in the org with the admin role — the row GET /users/me reads.
	if len(users.seats) != 1 {
		t.Fatalf("seats = %d, want 1", len(users.seats))
	}
	seat := users.seats[0]
	if seat.orgID != wantOrg || len(seat.roles) != 1 || seat.roles[0] != authtypes.O11yAdminRoleName {
		t.Fatalf("seat = %+v, want admin on org %s", seat, wantOrg)
	}
	// The row's id IS the asserted IAM subject, or GET /users/me — which looks it
	// up by exactly (orgID, claims.UserID) — answers user_not_found to a person the
	// gateway authenticated, and the console has no identity to render.
	if seat.userID != identity.UserID {
		t.Fatalf("seated user id = %s, want the asserted subject %s", seat.userID, identity.UserID)
	}
	if _, err := users.GetUserByOrgIDAndID(context.Background(), wantOrg, identity.UserID); err != nil {
		t.Fatalf("GET /users/me would fail for the session just provisioned: %v", err)
	}
	if seat.email != "z@hanzo.ai" {
		t.Fatalf("seated email = %s, want z@hanzo.ai", seat.email)
	}
}

// Repeated requests from the same session do not re-provision or re-grant.
func TestGetIdentity_Idempotent(t *testing.T) {
	store := newFakeOrgStore()
	authorizer := &fakeAuthorizer{}
	users := newFakeUserStore()
	p := newProvider(t, store, authorizer, users)

	req := requestWithSession("hanzo", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", "z@hanzo.ai")
	for i := 0; i < 5; i++ {
		if _, err := p.GetIdentity(req); err != nil {
			t.Fatalf("GetIdentity #%d: %v", i, err)
		}
	}

	if len(store.creates) != 1 {
		t.Fatalf("org creates = %d, want 1 (cached)", len(store.creates))
	}
	if len(users.seats) != 1 {
		t.Fatalf("seats = %d, want 1 (cached)", len(users.seats))
	}
}

// A second user in an already-provisioned org reuses the org and is granted.
func TestGetIdentity_ExistingOrgSecondUser(t *testing.T) {
	store := newFakeOrgStore()
	authorizer := &fakeAuthorizer{}
	users := newFakeUserStore()
	p := newProvider(t, store, authorizer, users)

	if _, err := p.GetIdentity(requestWithSession("hanzo", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", "a@hanzo.ai")); err != nil {
		t.Fatalf("first user: %v", err)
	}
	if _, err := p.GetIdentity(requestWithSession("hanzo", "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d", "b@hanzo.ai")); err != nil {
		t.Fatalf("second user: %v", err)
	}

	if len(store.creates) != 1 {
		t.Fatalf("org creates = %d, want 1 (org reused)", len(store.creates))
	}
	if len(users.seats) != 2 {
		t.Fatalf("seats = %d, want 2 (one per user)", len(users.seats))
	}
}

// Different Hanzo orgs get isolated, deterministic o11y org UUIDs.
func TestGetIdentity_MultiTenantIsolation(t *testing.T) {
	store := newFakeOrgStore()
	authorizer := &fakeAuthorizer{}
	users := newFakeUserStore()
	p := newProvider(t, store, authorizer, users)

	a, err := p.GetIdentity(requestWithSession("hanzo", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", "z@hanzo.ai"))
	if err != nil {
		t.Fatalf("org a: %v", err)
	}
	b, err := p.GetIdentity(requestWithSession("zoo", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", "z@zoo.ngo"))
	if err != nil {
		t.Fatalf("org b: %v", err)
	}
	if a.OrgID == b.OrgID {
		t.Fatalf("distinct Hanzo orgs mapped to the same o11y org %s", a.OrgID)
	}
	if a.OrgID != toUUID("org", "hanzo") || b.OrgID != toUUID("org", "zoo") {
		t.Fatalf("org mapping not deterministic: a=%s b=%s", a.OrgID, b.OrgID)
	}
}

// Without the gateway-asserted identity headers there is no session: unauthenticated,
// and nothing is provisioned.
func TestGetIdentity_MissingHeaders(t *testing.T) {
	store := newFakeOrgStore()
	authorizer := &fakeAuthorizer{}
	users := newFakeUserStore()
	p := newProvider(t, store, authorizer, users)

	if _, err := p.GetIdentity(requestWithSession("", "user", "z@hanzo.ai")); err == nil {
		t.Fatal("expected error when X-Org-Id is absent")
	}
	if _, err := p.GetIdentity(requestWithSession("hanzo", "", "z@hanzo.ai")); err == nil {
		t.Fatal("expected error when X-User-Id is absent")
	}
	if len(store.creates) != 0 || len(users.seats) != 0 {
		t.Fatalf("no provisioning expected without a session: creates=%d seats=%d", len(store.creates), len(users.seats))
	}
}

func TestTest_SignalIsOrgHeader(t *testing.T) {
	p := newProvider(t, newFakeOrgStore(), &fakeAuthorizer{}, newFakeUserStore())
	if !p.Test(requestWithSession("hanzo", "user", "")) {
		t.Fatal("Test should match when X-Org-Id is present")
	}
	if p.Test(requestWithSession("", "user", "")) {
		t.Fatal("Test should not match without X-Org-Id")
	}
}

// A session that asserts an org and a subject but no address cannot be seated —
// a user row must carry an email, and o11y must not invent an address for a
// person. It refuses at the seam that knows, rather than seating a half-built
// identity that fails later and further away.
func TestGetIdentity_RefusesToSeatWithoutEmail(t *testing.T) {
	store := newFakeOrgStore()
	authorizer := &fakeAuthorizer{}
	users := newFakeUserStore()
	p := newProvider(t, store, authorizer, users)

	if _, err := p.GetIdentity(requestWithSession("hanzo", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", "")); err == nil {
		t.Fatal("expected a refusal when the session carries no X-User-Email")
	}
	if len(users.seats) != 0 {
		t.Fatalf("seats = %d, want 0 — nothing half-built", len(users.seats))
	}
}

// A person already seated is not re-seated, and their existing row is used as
// is: re-provisioning must never overwrite a row the tenant has been using.
func TestGetIdentity_ExistingSeatReused(t *testing.T) {
	store := newFakeOrgStore()
	authorizer := &fakeAuthorizer{}
	users := newFakeUserStore()
	p := newProvider(t, store, authorizer, users)

	const userUUID = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	orgID, userID := toUUID("org", "hanzo"), valuer.MustNewUUID(userUUID)
	seeded, err := types.NewUserWithID(userID, "Seeded Human", valuer.MustNewEmail("z@hanzo.ai"), orgID, types.UserStatusActive)
	if err != nil {
		t.Fatalf("NewUserWithID: %v", err)
	}
	users.users[userKey(orgID, userID)] = seeded

	if _, err := p.GetIdentity(requestWithSession("hanzo", userUUID, "z@hanzo.ai")); err != nil {
		t.Fatalf("GetIdentity: %v", err)
	}
	if len(users.seats) != 0 {
		t.Fatalf("seats = %d, want 0 — an existing row is reused, never rewritten", len(users.seats))
	}
	if got := users.users[userKey(orgID, userID)].DisplayName; got != "Seeded Human" {
		t.Fatalf("display name = %q, want the untouched %q", got, "Seeded Human")
	}
}
