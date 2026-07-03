// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package iamidentn is o11y's sole human-identity provider. o11y mints and
// stores no identity of its own: identity is asserted by the Hanzo IAM session
// that the edge gateway (hanzoai/gateway) already validated. The gateway
// verifies the hanzo.id JWT against IAM's JWKS and injects canonical, trusted
// identity headers — stripping any client-supplied copies first, per HIP-0026:
//
//	X-Org-Id     the org slug (JWT "owner" claim) — the tenant
//	X-User-Id    the user id  (JWT "sub" claim)
//	X-User-Email the user email
//
// o11y trusts these exactly like every other Hanzo service (cloud, ai,
// commerce): one IAM validates once at the edge, every service trusts the
// assertion. There is no o11y-native login, registration, invite, or token
// minting — Hanzo IAM is the only identity.
//
// The tenant is auto-provisioned with zero onboarding: on first sight of an
// (org, user) pair the matching o11y organization row is created if absent
// (idempotent) and the user is granted its admin role in Hanzo IAM.
// Authorization itself stays with iamauthz — every runtime access check is
// still a Hanzo IAM batch-enforce; this provider only ensures the founding
// grant exists so a logged-in Hanzo user is authorized for their own org.
package iamidentn

import (
	"context"
	"net/http"
	"sync"

	"github.com/google/uuid"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/identn"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Canonical identity headers injected by hanzoai/gateway after it validates the
// Hanzo IAM session. Client-supplied copies are stripped by the gateway, so
// within the cluster these are trusted. They are a fixed contract, not config.
const (
	headerOrgID  = "X-Org-Id"
	headerUserID = "X-User-Id"
	headerEmail  = "X-User-Email"
)

// OrgResolver resolves an existing o11y organization. Declared narrowly so this
// package depends only on what it uses; satisfied by organization.Getter.
type OrgResolver interface {
	GetByIDOrName(context.Context, valuer.UUID, string) (*types.Organization, bool, error)
}

// OrgCreator creates an o11y organization, invoking the callback to bootstrap
// its managed roles inside the same transaction; satisfied by organization.Setter.
type OrgCreator interface {
	Create(context.Context, *types.Organization, func(context.Context, valuer.UUID) error) error
}

// Authorizer bootstraps an org's managed roles and grants a subject a role.
// Satisfied by authz.AuthZ (Hanzo IAM). Every runtime access check still flows
// through the same authz service's batch-enforce.
type Authorizer interface {
	CreateManagedRoles(context.Context, valuer.UUID, []*authtypes.Role) error
	Grant(context.Context, valuer.UUID, []string, string) error
}

type provider struct {
	settings   factory.ScopedProviderSettings
	resolver   OrgResolver
	creator    OrgCreator
	authorizer Authorizer

	// provisioned records "<userID>@<orgID>" pairs already resolved this run, so
	// the org lookup + grant happen once, not on every request. Both underlying
	// operations are idempotent, so this is a fast path, not a correctness gate.
	provisioned sync.Map
}

func NewFactory(resolver OrgResolver, creator OrgCreator, authorizer Authorizer) factory.ProviderFactory[identn.IdentN, identn.Config] {
	return factory.NewProviderFactory(factory.MustNewName(authtypes.IdentNProviderIAM.StringValue()), func(ctx context.Context, providerSettings factory.ProviderSettings, config identn.Config) (identn.IdentN, error) {
		return New(providerSettings, resolver, creator, authorizer)
	})
}

func New(providerSettings factory.ProviderSettings, resolver OrgResolver, creator OrgCreator, authorizer Authorizer) (identn.IdentN, error) {
	return &provider{
		settings:   factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/identn/iamidentn"),
		resolver:   resolver,
		creator:    creator,
		authorizer: authorizer,
	}, nil
}

func (p *provider) Name() authtypes.IdentNProvider {
	return authtypes.IdentNProviderIAM
}

// Test matches when the gateway has asserted a tenant — the presence of the
// trusted X-Org-Id header is the Hanzo IAM session signal.
func (p *provider) Test(req *http.Request) bool {
	return req.Header.Get(headerOrgID) != ""
}

func (p *provider) GetIdentity(req *http.Request) (*authtypes.Identity, error) {
	ctx := req.Context()

	orgSlug := req.Header.Get(headerOrgID)
	userRef := req.Header.Get(headerUserID)
	if orgSlug == "" || userRef == "" {
		return nil, errors.NewUnauthenticatedf(errors.CodeUnauthenticated, "missing Hanzo IAM identity headers")
	}

	orgID := toUUID("org", orgSlug)
	userID := toUUID("user", userRef)

	// Email is descriptive (attribution/display), not load-bearing for authz;
	// tolerate absence or an odd value rather than reject the session.
	email, err := valuer.NewEmail(req.Header.Get(headerEmail))
	if err != nil {
		email = valuer.Email{}
	}

	if err := p.provision(ctx, orgID, orgSlug, userID); err != nil {
		return nil, err
	}

	return authtypes.NewPrincipalUserIdentity(userID, orgID, email, authtypes.IdentNProviderIAM), nil
}

// provision resolves-or-creates the tenant org and grants the user its admin
// role, once per (user, org). Idempotent and safe under concurrency.
func (p *provider) provision(ctx context.Context, orgID valuer.UUID, orgSlug string, userID valuer.UUID) error {
	key := userID.String() + "@" + orgID.String()
	if _, ok := p.provisioned.Load(key); ok {
		return nil
	}

	if err := p.ensureOrg(ctx, orgID, orgSlug); err != nil {
		return err
	}

	subject := authtypes.MustNewSubject(coretypes.NewResourceUser(), userID.String(), orgID, nil)
	if err := p.authorizer.Grant(ctx, orgID, []string{authtypes.SigNozAdminRoleName}, subject); err != nil {
		return err
	}

	p.provisioned.Store(key, struct{}{})
	return nil
}

// ensureOrg creates the o11y org for a Hanzo org on first sight, complete with
// managed roles and default configs (mirroring first-user creation). Existing
// orgs and a lost create race both resolve to success.
func (p *provider) ensureOrg(ctx context.Context, orgID valuer.UUID, orgSlug string) error {
	if _, _, err := p.resolver.GetByIDOrName(ctx, orgID, orgSlug); err == nil {
		return nil
	} else if !errors.Ast(err, errors.TypeNotFound) {
		return err
	}

	org := types.NewOrganizationWithID(orgID, orgSlug, orgSlug)
	managedRoles := authtypes.NewManagedRoles(orgID)
	err := p.creator.Create(ctx, org, func(ctx context.Context, id valuer.UUID) error {
		return p.authorizer.CreateManagedRoles(ctx, id, managedRoles)
	})
	if err != nil && !errors.Ast(err, errors.TypeAlreadyExists) {
		return err
	}

	p.settings.Logger().InfoContext(ctx, "auto-provisioned o11y org for Hanzo IAM session", "org_id", orgID.String(), "org", orgSlug)
	return nil
}

// toUUID maps a Hanzo IAM identifier to a stable o11y UUID. A value that is
// already a UUID (the user "sub") is used as-is; a slug (the org "owner") is
// mapped deterministically via UUIDv5, so the same Hanzo org always resolves to
// the same o11y org — the whole of the tenant mapping, no state required.
func toUUID(kind, value string) valuer.UUID {
	if u, err := valuer.NewUUID(value); err == nil {
		return u
	}
	derived := uuid.NewSHA1(uuid.NameSpaceURL, []byte("hanzo:o11y:"+kind+":"+value))
	return valuer.MustNewUUID(derived.String())
}
