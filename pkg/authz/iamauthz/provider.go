// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package iamauthz delegates every authz decision to Hanzo IAM.
//
// o11y no longer ships its own authorization engine. Per the project rule
// "kill auth module, use IAM for auth only": this provider is the single
// integration point. All Check / Write / List calls translate to HTTP
// requests against the Hanzo IAM API (default https://iam.hanzo.ai/v1).
//
// Zero gRPC, zero protobuf, zero OpenFGA. Plain HTTP+JSON.
package iamauthz

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/types/roletypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
)

// provider is the IAM-backed authz implementation.
//
// It uses HTTP+JSON against Hanzo IAM. The sqlstore handle is retained
// only so legacy callers that expect a sqlstore-touching authz provider
// keep working; iamauthz itself does not write to it.
type provider struct {
	sqlstore sqlstore.SQLStore
	baseURL  string
	httpc    *http.Client
}

// NewProviderFactory returns a factory.ProviderFactory that wires an
// IAM-backed AuthZ. Drop-in for openfgaauthz.NewProviderFactory.
//
// Configuration is via env:
//   O11Y_IAM_URL  — base URL of the IAM API (default https://iam.hanzo.ai/v1).
//   O11Y_IAM_TOKEN — bearer token used on every request (optional).
//
// The factory does no I/O at construction time; failure modes show up
// on the first Check call.
func NewProviderFactory(store sqlstore.SQLStore) factory.ProviderFactory[authz.AuthZ, authz.Config] {
	return factory.NewProviderFactory(factory.MustNewName("iam"), func(ctx context.Context, ps factory.ProviderSettings, c authz.Config) (authz.AuthZ, error) {
		return New(store), nil
	})
}

// New constructs an iamauthz provider.
func New(store sqlstore.SQLStore) authz.AuthZ {
	base := strings.TrimRight(os.Getenv("O11Y_IAM_URL"), "/")
	if base == "" {
		base = "https://iam.hanzo.ai/v1"
	}
	return &provider{
		sqlstore: store,
		baseURL:  base,
		httpc:    &http.Client{},
	}
}

// errNotImplemented is the placeholder error returned by every method that
// hasn't been wired to a real IAM endpoint yet. The interface is wide
// (~20 methods) and most of it is role-management plumbing that o11y
// shouldn't be doing — that responsibility now lives in IAM. Methods
// stay on the interface for source-compat with callers; the body returns
// this error until each is mapped to its IAM HTTP equivalent.
var errNotImplemented = errors.New("iamauthz: method delegated to Hanzo IAM — wire the HTTP call in this provider")

// factory.Service: must return a stable name for telemetry/logging.
func (p *provider) Name() string { return "iam" }

// Start / Stop are no-ops — the HTTP client doesn't hold long-lived state.
func (p *provider) Start(_ context.Context) error { return nil }
func (p *provider) Stop(_ context.Context) error  { return nil }

// Healthy reports readiness. The IAM-backed provider holds no long-lived
// state, so it is always healthy; return a closed channel.
func (p *provider) Healthy() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (p *provider) ReadTuples(_ context.Context, _ *openfgav1.ReadRequestTupleKey) ([]*openfgav1.TupleKey, error) {
	return nil, errNotImplemented
}

// =============================================================================
// Check methods — the hot path. Wire to IAM /enforce first.
// =============================================================================

func (p *provider) CheckWithTupleCreation(ctx context.Context, _ authtypes.Claims, _ valuer.UUID, _ authtypes.Relation, _ coretypes.Resource, _ []coretypes.Selector, _ []coretypes.Selector) error {
	// TODO: POST {baseURL}/enforce with subject=claims.UserID, action=relation,
	// resource=typeable.Type/selectors. Until wired, fail closed.
	return errNotImplemented
}

func (p *provider) CheckWithTupleCreationWithoutClaims(ctx context.Context, _ valuer.UUID, _ authtypes.Relation, _ coretypes.Resource, _ []coretypes.Selector, _ []coretypes.Selector) error {
	return errNotImplemented
}

func (p *provider) BatchCheck(_ context.Context, _ map[string]*openfgav1.TupleKey) (map[string]*authtypes.TupleKeyAuthorization, error) {
	return nil, errNotImplemented
}

func (p *provider) CheckTransactions(_ context.Context, _ string, _ valuer.UUID, _ []*authtypes.Transaction) ([]*authtypes.TransactionWithAuthorization, error) {
	return nil, errNotImplemented
}

// =============================================================================
// Write methods — IAM owns the role/tuple store. These translate to
// IAM's role-assignment endpoints.
// =============================================================================

func (p *provider) Write(_ context.Context, _ []*openfgav1.TupleKey, _ []*openfgav1.TupleKey) error {
	return errNotImplemented
}

func (p *provider) ListObjects(_ context.Context, _ string, _ authtypes.Relation, _ coretypes.Type) ([]*coretypes.Object, error) {
	return nil, errNotImplemented
}

// =============================================================================
// Role CRUD — IAM is the system of record. These map onto IAM's
// /roles, /roles/{id}, /roles/{id}/objects endpoints.
// =============================================================================

func (p *provider) Create(_ context.Context, _ valuer.UUID, _ *roletypes.Role) error {
	return errNotImplemented
}

func (p *provider) GetOrCreate(_ context.Context, _ valuer.UUID, _ *roletypes.Role) (*roletypes.Role, error) {
	return nil, errNotImplemented
}

func (p *provider) GetObjects(_ context.Context, _ valuer.UUID, _ valuer.UUID, _ authtypes.Relation) ([]*coretypes.Object, error) {
	return nil, errNotImplemented
}

func (p *provider) GetResources(_ context.Context) []*coretypes.Resource {
	return nil
}

func (p *provider) Patch(_ context.Context, _ valuer.UUID, _ *roletypes.Role) error {
	return errNotImplemented
}

func (p *provider) PatchObjects(_ context.Context, _ valuer.UUID, _ string, _ authtypes.Relation, _ []*coretypes.Object, _ []*coretypes.Object) error {
	return errNotImplemented
}

func (p *provider) Delete(_ context.Context, _ valuer.UUID, _ valuer.UUID) error {
	return errNotImplemented
}

func (p *provider) Get(_ context.Context, _ valuer.UUID, _ valuer.UUID) (*roletypes.Role, error) {
	return nil, errNotImplemented
}

func (p *provider) GetByOrgIDAndName(_ context.Context, _ valuer.UUID, _ string) (*roletypes.Role, error) {
	return nil, errNotImplemented
}

func (p *provider) List(_ context.Context, _ valuer.UUID) ([]*roletypes.Role, error) {
	return nil, errNotImplemented
}

func (p *provider) ListByOrgIDAndNames(_ context.Context, _ valuer.UUID, _ []string) ([]*roletypes.Role, error) {
	return nil, errNotImplemented
}

func (p *provider) ListByOrgIDAndIDs(_ context.Context, _ valuer.UUID, _ []valuer.UUID) ([]*roletypes.Role, error) {
	return nil, errNotImplemented
}

func (p *provider) Grant(_ context.Context, _ valuer.UUID, _ []string, _ string) error {
	return errNotImplemented
}

func (p *provider) Revoke(_ context.Context, _ valuer.UUID, _ []string, _ string) error {
	return errNotImplemented
}

func (p *provider) ModifyGrant(_ context.Context, _ valuer.UUID, _ []string, _ []string, _ string) error {
	return errNotImplemented
}

func (p *provider) CreateManagedRoles(_ context.Context, _ valuer.UUID, _ []*roletypes.Role) error {
	return errNotImplemented
}

func (p *provider) CreateManagedUserRoleTransactions(_ context.Context, _ valuer.UUID, _ valuer.UUID) error {
	return errNotImplemented
}
