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
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
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

// fakeAuthorizer records managed-role bootstraps and grants.
type fakeAuthorizer struct {
	mu           sync.Mutex
	managedRoles int
	grants       []grantCall
}

type grantCall struct {
	orgID   valuer.UUID
	names   []string
	subject string
}

func (f *fakeAuthorizer) CreateManagedRoles(_ context.Context, _ valuer.UUID, _ []*authtypes.Role) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.managedRoles++
	return nil
}

func (f *fakeAuthorizer) Grant(_ context.Context, orgID valuer.UUID, names []string, subject string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants = append(f.grants, grantCall{orgID: orgID, names: names, subject: subject})
	return nil
}

func newProvider(t *testing.T, store *fakeOrgStore, authorizer *fakeAuthorizer) *provider {
	t.Helper()
	p, err := New(instrumentationtest.New().ToProviderSettings(), store, store, authorizer)
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
	p := newProvider(t, store, authorizer)

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

	// User granted admin, scoped to the resolved org.
	if len(authorizer.grants) != 1 {
		t.Fatalf("grants = %d, want 1", len(authorizer.grants))
	}
	g := authorizer.grants[0]
	if g.orgID != wantOrg || len(g.names) != 1 || g.names[0] != authtypes.SigNozAdminRoleName {
		t.Fatalf("grant = %+v, want admin on org %s", g, wantOrg)
	}
	// The grant subject must equal what the authz middleware checks against.
	wantSubject := authtypes.MustNewSubject(coretypes.NewResourceUser(), identity.UserID.String(), wantOrg, nil)
	if g.subject != wantSubject {
		t.Fatalf("grant subject = %s, want %s", g.subject, wantSubject)
	}
}

// Repeated requests from the same session do not re-provision or re-grant.
func TestGetIdentity_Idempotent(t *testing.T) {
	store := newFakeOrgStore()
	authorizer := &fakeAuthorizer{}
	p := newProvider(t, store, authorizer)

	req := requestWithSession("hanzo", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", "z@hanzo.ai")
	for i := 0; i < 5; i++ {
		if _, err := p.GetIdentity(req); err != nil {
			t.Fatalf("GetIdentity #%d: %v", i, err)
		}
	}

	if len(store.creates) != 1 {
		t.Fatalf("org creates = %d, want 1 (cached)", len(store.creates))
	}
	if len(authorizer.grants) != 1 {
		t.Fatalf("grants = %d, want 1 (cached)", len(authorizer.grants))
	}
}

// A second user in an already-provisioned org reuses the org and is granted.
func TestGetIdentity_ExistingOrgSecondUser(t *testing.T) {
	store := newFakeOrgStore()
	authorizer := &fakeAuthorizer{}
	p := newProvider(t, store, authorizer)

	if _, err := p.GetIdentity(requestWithSession("hanzo", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", "a@hanzo.ai")); err != nil {
		t.Fatalf("first user: %v", err)
	}
	if _, err := p.GetIdentity(requestWithSession("hanzo", "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d", "b@hanzo.ai")); err != nil {
		t.Fatalf("second user: %v", err)
	}

	if len(store.creates) != 1 {
		t.Fatalf("org creates = %d, want 1 (org reused)", len(store.creates))
	}
	if len(authorizer.grants) != 2 {
		t.Fatalf("grants = %d, want 2 (one per user)", len(authorizer.grants))
	}
}

// Different Hanzo orgs get isolated, deterministic o11y org UUIDs.
func TestGetIdentity_MultiTenantIsolation(t *testing.T) {
	store := newFakeOrgStore()
	authorizer := &fakeAuthorizer{}
	p := newProvider(t, store, authorizer)

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
	p := newProvider(t, store, authorizer)

	if _, err := p.GetIdentity(requestWithSession("", "user", "z@hanzo.ai")); err == nil {
		t.Fatal("expected error when X-Org-Id is absent")
	}
	if _, err := p.GetIdentity(requestWithSession("hanzo", "", "z@hanzo.ai")); err == nil {
		t.Fatal("expected error when X-User-Id is absent")
	}
	if len(store.creates) != 0 || len(authorizer.grants) != 0 {
		t.Fatalf("no provisioning expected without a session: creates=%d grants=%d", len(store.creates), len(authorizer.grants))
	}
}

func TestTest_SignalIsOrgHeader(t *testing.T) {
	p := newProvider(t, newFakeOrgStore(), &fakeAuthorizer{})
	if !p.Test(requestWithSession("hanzo", "user", "")) {
		t.Fatal("Test should match when X-Org-Id is present")
	}
	if p.Test(requestWithSession("", "user", "")) {
		t.Fatal("Test should not match without X-Org-Id")
	}
}
