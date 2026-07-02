// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

// Package iamauthz is o11y's sole authorization provider. Every authz decision
// is delegated to Hanzo IAM's Casbin enforce endpoint over plain HTTP+JSON —
// there is no embedded authorization engine and no pluggable alternative. This
// is the house rule: o11y uses Hanzo IAM for authorization, natively.
//
// Concerns are kept orthogonal:
//   - Authorization decisions (Check*, BatchCheck, CheckTransactions) and the
//     relationship-tuple store (Write, ReadTuples, ListObjects, Grant, Revoke)
//     are delegated to Hanzo IAM via iamClient.
//   - Role *metadata* (names, descriptions, org scoping) is plain relational
//     data and stays in the local SQL role store — it is not an IAM concern.
package iamauthz

import (
	"context"
	"strconv"
	"strings"

	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/authz/authzstore/sqlauthzstore"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type provider struct {
	iam      *iamClient
	store    authtypes.RoleStore
	registry *authtypes.Registry
	healthy  chan struct{}
}

// NewProviderFactory wires an IAM-backed AuthZ. It is the single authorization
// provider factory — o11y no longer selects between authz backends.
func NewProviderFactory(store sqlstore.SQLStore) factory.ProviderFactory[authz.AuthZ, authz.Config] {
	return factory.NewProviderFactory(factory.MustNewName("iam"), func(ctx context.Context, ps factory.ProviderSettings, config authz.Config) (authz.AuthZ, error) {
		return newProvider(config, store), nil
	})
}

func newProvider(config authz.Config, store sqlstore.SQLStore) authz.AuthZ {
	// IAM is an external service; the provider is ready as soon as it is
	// constructed. Signal healthy immediately.
	healthy := make(chan struct{})
	close(healthy)

	return &provider{
		iam:      newIAMClient(config.IAM),
		store:    sqlauthzstore.NewSqlAuthzStore(store),
		registry: authtypes.NewRegistry(),
		healthy:  healthy,
	}
}

// =============================================================================
// factory.ServiceWithHealthy
// =============================================================================

func (p *provider) Start(_ context.Context) error { return nil }
func (p *provider) Stop(_ context.Context) error  { return nil }
func (p *provider) Healthy() <-chan struct{}      { return p.healthy }

// =============================================================================
// Authorization decisions — delegated to Hanzo IAM /enforce.
// =============================================================================

func (p *provider) BatchCheck(ctx context.Context, tupleReq map[string]*authtypes.TupleKey) (map[string]*authtypes.TupleKeyAuthorization, error) {
	ids := make([]string, 0, len(tupleReq))
	requests := make([][]string, 0, len(tupleReq))
	for id, tuple := range tupleReq {
		ids = append(ids, id)
		requests = append(requests, requestFromTuple(tuple))
	}

	allowed, err := p.iam.batchEnforce(ctx, requests)
	if err != nil {
		return nil, errors.Newf(errors.TypeInternal, authtypes.ErrCodeAuthZUnavailable, "authorization server is unavailable").WithAdditional(err.Error())
	}

	response := make(map[string]*authtypes.TupleKeyAuthorization, len(tupleReq))
	for idx, id := range ids {
		authorized := idx < len(allowed) && allowed[idx]
		response[id] = &authtypes.TupleKeyAuthorization{
			Tuple:      tupleReq[id],
			Authorized: authorized,
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
// selectors — the shared core of the two Check* entrypoints.
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
// Relationship-tuple store — delegated to Hanzo IAM policies.
// =============================================================================

func (p *provider) Write(ctx context.Context, additions []*authtypes.TupleKey, deletions []*authtypes.TupleKey) error {
	for _, tuple := range additions {
		if err := p.iam.addPolicy(ctx, tuple); err != nil {
			return errors.Wrapf(err, errors.TypeInternal, authtypes.ErrCodeAuthZUnavailable, "failed to write tuple to authorization server")
		}
	}
	for _, tuple := range deletions {
		if err := p.iam.removePolicy(ctx, tuple); err != nil {
			return errors.Wrapf(err, errors.TypeInternal, authtypes.ErrCodeAuthZUnavailable, "failed to delete tuple from authorization server")
		}
	}
	return nil
}

func (p *provider) ReadTuples(ctx context.Context, filter *authtypes.ReadRequestTupleKey) ([]*authtypes.TupleKey, error) {
	tuples, err := p.iam.getPolicies(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, errors.TypeInternal, authtypes.ErrCodeAuthZUnavailable, "failed to read tuples from authorization server")
	}

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
	tuples, err := p.iam.getPolicies(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, errors.TypeInternal, authtypes.ErrCodeAuthZUnavailable, "cannot list objects for subject %s with relation %s for type %s", subject, relation.StringValue(), objectType.StringValue())
	}

	prefix := objectType.StringValue() + ":"
	objectStrings := make([]string, 0, len(tuples))
	for _, tuple := range tuples {
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
// Role metadata — the local SQL role store (not an IAM concern).
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
	return p.Grant(ctx, orgID, []string{authtypes.SigNozAdminRoleName}, subject)
}

// =============================================================================
// Role transaction-group CRUD — owned by Hanzo IAM's policy administration, not
// by o11y. These are unsupported here (parity with the prior provider).
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
