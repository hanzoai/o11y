// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package localauthz is o11y's authorization provider for the EMBEDDED runtime —
// the unified hanzoai/cloud one-binary that serves /v1/o11y/* in-process.
//
// It exists because the trust boundary differs from the standalone service. In
// the embed, the edge gateway (hanzoai/gateway) has ALREADY validated the
// hanzo.id JWT against Hanzo IAM's JWKS and injected the canonical, trusted
// identity headers (X-Org-Id/X-User-Id/X-User-Email), and o11y's sharder gates
// cross-org access. Authorization of an org-scoped, gateway-authenticated user
// for their OWN org's telemetry is therefore a LOCAL decision — the same
// "one IAM validates once at the edge, every service trusts the assertion"
// model the iamidentn package documents. Round-tripping BACK OUT to an external
// IAM Casbin enforcer for every telemetry read (as iamauthz does) is redundant
// with that edge trust, adds synchronous latency and an external failure mode,
// and requires a separately-provisioned enforcer + client credentials the
// one-binary does not carry.
//
// So localauthz keeps the EXACT policy iamauthz enforces — a subject is
// authorized iff it holds one of the required roles in its org — but stores the
// relationship tuples in-process rather than in Hanzo IAM. The founding grants
// come from the same iamidentn provision path (o11y-admin per (user, org) on
// first sight), so the in-process tuple set is populated before the per-route
// AuthZ check runs on that same request, and re-populated on the first request
// after a restart (both the identn `provisioned` cache and this tuple set are
// per-process and reset together). Role *metadata* (names, descriptions, org
// scoping) stays in the local SQL role store — not an authorization concern —
// exactly as in iamauthz.
package localauthz

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/authz/authzstore/sqlauthzstore"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// tupleStore is the in-process relationship-tuple set. A tuple is the triple
// (User, Object, Relation); membership is the whole of the authorization state.
type tupleStore struct {
	mu sync.RWMutex
	m  map[string]struct{}
}

func newTupleStore() *tupleStore { return &tupleStore{m: make(map[string]struct{})} }

func tupleID(t *authtypes.TupleKey) string {
	// NUL separates the fields so no field value can spoof a boundary.
	return t.GetUser() + "\x00" + t.GetObject() + "\x00" + t.GetRelation()
}

func (s *tupleStore) add(t *authtypes.TupleKey) {
	s.mu.Lock()
	s.m[tupleID(t)] = struct{}{}
	s.mu.Unlock()
}

func (s *tupleStore) remove(t *authtypes.TupleKey) {
	s.mu.Lock()
	delete(s.m, tupleID(t))
	s.mu.Unlock()
}

func (s *tupleStore) has(t *authtypes.TupleKey) bool {
	s.mu.RLock()
	_, ok := s.m[tupleID(t)]
	s.mu.RUnlock()
	return ok
}

func (s *tupleStore) all() []*authtypes.TupleKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tuples := make([]*authtypes.TupleKey, 0, len(s.m))
	for id := range s.m {
		parts := strings.SplitN(id, "\x00", 3)
		if len(parts) != 3 {
			continue
		}
		tuples = append(tuples, &authtypes.TupleKey{User: parts[0], Object: parts[1], Relation: parts[2]})
	}
	return tuples
}

type provider struct {
	tuples   *tupleStore
	store    authtypes.RoleStore
	registry *authtypes.Registry
	healthy  chan struct{}
	stopC    chan struct{}
}

// NewProviderFactory wires the embedded, edge-trusted AuthZ. Role metadata lives
// in the given SQL store; tuples live in-process.
func NewProviderFactory(store sqlstore.SQLStore) factory.ProviderFactory[authz.AuthZ, authz.Config] {
	return factory.NewProviderFactory(factory.MustNewName("local"), func(ctx context.Context, ps factory.ProviderSettings, config authz.Config) (authz.AuthZ, error) {
		return newProvider(store), nil
	})
}

func newProvider(store sqlstore.SQLStore) authz.AuthZ {
	// Ready as soon as constructed — there is no external dependency to reach.
	healthy := make(chan struct{})
	close(healthy)

	return &provider{
		tuples:   newTupleStore(),
		store:    sqlauthzstore.NewSqlAuthzStore(store),
		registry: authtypes.NewRegistry(),
		healthy:  healthy,
		stopC:    make(chan struct{}),
	}
}

// =============================================================================
// factory.ServiceWithHealthy
// =============================================================================

// Start blocks until shutdown — the service supervisor (factory.Registry.Wait)
// treats ANY Start return as a service exit and tears the process down, so a
// no-loop provider must block until Stop/context cancellation (mirroring
// iamauthz and the other no-loop providers).
func (p *provider) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
	case <-p.stopC:
	}
	return nil
}
func (p *provider) Stop(_ context.Context) error { close(p.stopC); return nil }
func (p *provider) Healthy() <-chan struct{}     { return p.healthy }

// =============================================================================
// Authorization decisions — local tuple membership.
// =============================================================================

func (p *provider) BatchCheck(ctx context.Context, tupleReq map[string]*authtypes.TupleKey) (map[string]*authtypes.TupleKeyAuthorization, error) {
	response := make(map[string]*authtypes.TupleKeyAuthorization, len(tupleReq))
	for id, tuple := range tupleReq {
		response[id] = &authtypes.TupleKeyAuthorization{
			Tuple:      tuple,
			Authorized: p.tuples.has(tuple),
		}
	}
	return response, nil
}

func (p *provider) CheckWithTupleCreation(ctx context.Context, claims authtypes.Claims, orgID valuer.UUID, _ authtypes.Relation, _ coretypes.Resource, _ []coretypes.Selector, roleSelectors []coretypes.Selector) error {
	subject := ""
	switch claims.Principal {
	case authtypes.PrincipalUser:
		user, err := authtypes.NewSubject(coretypes.NewResourceUser(), claims.UserID, orgID, nil)
		if err != nil {
			return err
		}
		subject = user
	case authtypes.PrincipalServiceAccount:
		serviceAccount, err := authtypes.NewSubject(coretypes.NewResourceServiceAccount(), claims.ServiceAccountID, orgID, nil)
		if err != nil {
			return err
		}
		subject = serviceAccount
	}

	return p.checkAnyAuthorized(ctx, subject, orgID, roleSelectors)
}

func (p *provider) CheckWithTupleCreationWithoutClaims(ctx context.Context, orgID valuer.UUID, _ authtypes.Relation, _ coretypes.Resource, _ []coretypes.Selector, roleSelectors []coretypes.Selector) error {
	subject, err := authtypes.NewSubject(coretypes.NewResourceAnonymous(), coretypes.AnonymousUser.String(), orgID, nil)
	if err != nil {
		return err
	}

	return p.checkAnyAuthorized(ctx, subject, orgID, roleSelectors)
}

// checkAnyAuthorized authorizes the subject if it holds ANY of the role
// selectors — the shared core of the two Check* entrypoints (identical policy to
// iamauthz; only the tuple backend differs).
func (p *provider) checkAnyAuthorized(ctx context.Context, subject string, orgID valuer.UUID, roleSelectors []coretypes.Selector) error {
	tupleSlice := authtypes.NewTuples(coretypes.NewResourceRole(), subject, authtypes.Relation{Verb: coretypes.VerbAssignee}, roleSelectors, orgID)

	tuples := make(map[string]*authtypes.TupleKey, len(tupleSlice))
	for idx, tuple := range tupleSlice {
		tuples[strconv.Itoa(idx)] = tuple
	}

	response, err := p.BatchCheck(ctx, tuples)
	if err != nil {
		return err
	}

	for _, resp := range response {
		if resp.Authorized {
			return nil
		}
	}

	return errors.Newf(errors.TypeForbidden, authtypes.ErrCodeAuthZForbidden, "subjects are not authorized for requested access")
}

func (p *provider) CheckTransactions(ctx context.Context, subject string, orgID valuer.UUID, transactions []*authtypes.Transaction) ([]*authtypes.TransactionWithAuthorization, error) {
	if len(transactions) == 0 {
		return make([]*authtypes.TransactionWithAuthorization, 0), nil
	}

	tuples, preResolved, roleCorrelations, err := authtypes.NewTuplesFromTransactionsWithManagedRoles(transactions, subject, orgID, p.registry.ManagedRolesByTransaction())
	if err != nil {
		return nil, err
	}

	if len(tuples) == 0 {
		return authtypes.NewTransactionWithAuthorizationFromBatchResults(transactions, nil, preResolved, roleCorrelations), nil
	}

	batchResults, err := p.BatchCheck(ctx, tuples)
	if err != nil {
		return nil, err
	}

	return authtypes.NewTransactionWithAuthorizationFromBatchResults(transactions, batchResults, preResolved, roleCorrelations), nil
}

// =============================================================================
// Relationship-tuple store — in-process.
// =============================================================================

func (p *provider) Write(ctx context.Context, additions []*authtypes.TupleKey, deletions []*authtypes.TupleKey) error {
	for _, tuple := range additions {
		p.tuples.add(tuple)
	}
	for _, tuple := range deletions {
		p.tuples.remove(tuple)
	}
	return nil
}

func (p *provider) ReadTuples(ctx context.Context, filter *authtypes.ReadRequestTupleKey) ([]*authtypes.TupleKey, error) {
	tuples := p.tuples.all()
	if filter == nil {
		return tuples, nil
	}

	filtered := make([]*authtypes.TupleKey, 0, len(tuples))
	for _, tuple := range tuples {
		if filter.User != "" && tuple.User != filter.User {
			continue
		}
		if filter.Relation != "" && tuple.Relation != filter.Relation {
			continue
		}
		if filter.Object != "" && tuple.Object != filter.Object {
			continue
		}
		filtered = append(filtered, tuple)
	}
	return filtered, nil
}

func (p *provider) ListObjects(ctx context.Context, subject string, relation authtypes.Relation, objectType coretypes.Type) ([]*coretypes.Object, error) {
	prefix := objectType.StringValue() + ":"
	objectStrings := make([]string, 0)
	for _, tuple := range p.tuples.all() {
		if tuple.User != subject || tuple.Relation != relation.StringValue() {
			continue
		}
		if !strings.HasPrefix(tuple.Object, prefix) {
			continue
		}
		objectStrings = append(objectStrings, tuple.Object)
	}

	return coretypes.MustNewObjectsFromStringSlice(objectStrings), nil
}

// =============================================================================
// Grants — thin translations onto the tuple store.
// =============================================================================

func (p *provider) Grant(ctx context.Context, orgID valuer.UUID, names []string, subject string) error {
	tuples := p.roleTuples(orgID, names, subject)
	if err := p.Write(ctx, tuples, nil); err != nil {
		return errors.WithAdditionalf(err, "failed to grant roles: %v to subject: %s", names, subject)
	}
	return nil
}

func (p *provider) Revoke(ctx context.Context, orgID valuer.UUID, names []string, subject string) error {
	tuples := p.roleTuples(orgID, names, subject)
	if err := p.Write(ctx, nil, tuples); err != nil {
		return errors.WithAdditionalf(err, "failed to revoke roles: %v to subject: %s", names, subject)
	}
	return nil
}

func (p *provider) ModifyGrant(ctx context.Context, orgID valuer.UUID, existingRoleNames []string, updatedRoleNames []string, subject string) error {
	if err := p.Revoke(ctx, orgID, existingRoleNames, subject); err != nil {
		return err
	}
	return p.Grant(ctx, orgID, updatedRoleNames, subject)
}

func (p *provider) roleTuples(orgID valuer.UUID, names []string, subject string) []*authtypes.TupleKey {
	selectors := make([]coretypes.Selector, len(names))
	for idx, name := range names {
		selectors[idx] = coretypes.TypeRole.MustSelector(name)
	}
	return authtypes.NewTuples(coretypes.NewResourceRole(), subject, authtypes.Relation{Verb: coretypes.VerbAssignee}, selectors, orgID)
}

// =============================================================================
// Role metadata — the local SQL role store (identical to iamauthz).
// =============================================================================

func (p *provider) Get(ctx context.Context, orgID valuer.UUID, id valuer.UUID) (*authtypes.Role, error) {
	return p.store.Get(ctx, orgID, id)
}

func (p *provider) GetByOrgIDAndName(ctx context.Context, orgID valuer.UUID, name string) (*authtypes.Role, error) {
	return p.store.GetByOrgIDAndName(ctx, orgID, name)
}

func (p *provider) List(ctx context.Context, orgID valuer.UUID) ([]*authtypes.Role, error) {
	return p.store.List(ctx, orgID)
}

func (p *provider) ListByOrgIDAndNames(ctx context.Context, orgID valuer.UUID, names []string) ([]*authtypes.Role, error) {
	return p.store.ListByOrgIDAndNames(ctx, orgID, names)
}

func (p *provider) ListByOrgIDAndIDs(ctx context.Context, orgID valuer.UUID, ids []valuer.UUID) ([]*authtypes.Role, error) {
	return p.store.ListByOrgIDAndIDs(ctx, orgID, ids)
}

func (p *provider) Collect(ctx context.Context, orgID valuer.UUID) (map[string]any, error) {
	roles, err := p.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return authtypes.NewStatsFromRoles(roles), nil
}

func (p *provider) CreateManagedRoles(ctx context.Context, _ valuer.UUID, managedRoles []*authtypes.Role) error {
	return p.store.RunInTx(ctx, func(ctx context.Context) error {
		for _, role := range managedRoles {
			if err := p.store.Create(ctx, role); err != nil {
				return err
			}
		}
		return nil
	})
}

func (p *provider) CreateManagedUserRoleTransactions(ctx context.Context, orgID valuer.UUID, userID valuer.UUID) error {
	subject := authtypes.MustNewSubject(coretypes.NewResourceUser(), userID.String(), orgID, nil)
	return p.Grant(ctx, orgID, []string{authtypes.O11yAdminRoleName}, subject)
}

// =============================================================================
// Role transaction-group CRUD — unsupported here, matching iamauthz. Role
// administration is a separate admin surface; the embed's telemetry reads never
// take this path.
// =============================================================================

func (p *provider) Create(_ context.Context, _ valuer.UUID, _ *authtypes.RoleWithTransactionGroups) error {
	return errors.Newf(errors.TypeUnsupported, authtypes.ErrCodeRoleUnsupported, "role administration is managed by Hanzo IAM")
}

func (p *provider) GetOrCreate(_ context.Context, _ valuer.UUID, _ *authtypes.Role) (*authtypes.Role, error) {
	return nil, errors.Newf(errors.TypeUnsupported, authtypes.ErrCodeRoleUnsupported, "role administration is managed by Hanzo IAM")
}

func (p *provider) Update(_ context.Context, _ valuer.UUID, _ *authtypes.RoleWithTransactionGroups) error {
	return errors.Newf(errors.TypeUnsupported, authtypes.ErrCodeRoleUnsupported, "role administration is managed by Hanzo IAM")
}

func (p *provider) Delete(_ context.Context, _ valuer.UUID, _ valuer.UUID) error {
	return errors.Newf(errors.TypeUnsupported, authtypes.ErrCodeRoleUnsupported, "role administration is managed by Hanzo IAM")
}

func (p *provider) GetWithTransactionGroups(_ context.Context, _ valuer.UUID, _ valuer.UUID) (*authtypes.RoleWithTransactionGroups, error) {
	return nil, errors.Newf(errors.TypeUnsupported, authtypes.ErrCodeRoleUnsupported, "role administration is managed by Hanzo IAM")
}
