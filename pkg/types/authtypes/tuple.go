package authtypes

import (
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

var (
	ErrCodeAuthZUnavailable = errors.MustNewCode("authz_unavailable")
	ErrCodeAuthZForbidden   = errors.MustNewCode("authz_forbidden")
	ErrCodeAuthZInvalidType = errors.MustNewCode("authz_invalid_type")
	ErrCodeTypeableNotFound = errors.MustNewCode("typeable_not_found")
)

// TupleKey is a relationship tuple: subject User has Relation on Object.
//
// This is o11y's Hanzo-native authorization primitive — a plain (user,
// relation, object) triple. It replaces the OpenFGA proto TupleKey: o11y no
// longer embeds an OpenFGA engine, and the enforcement decision is delegated to
// Hanzo IAM (see pkg/authz/iamauthz). The triple maps directly onto a Casbin
// request — (User, Object, Relation) → (sub, obj, act).
type TupleKey struct {
	User     string
	Relation string
	Object   string
}

func (t *TupleKey) GetUser() string {
	if t == nil {
		return ""
	}
	return t.User
}

func (t *TupleKey) GetRelation() string {
	if t == nil {
		return ""
	}
	return t.Relation
}

func (t *TupleKey) GetObject() string {
	if t == nil {
		return ""
	}
	return t.Object
}

// ReadRequestTupleKey is a filter over stored tuples; empty fields match any.
type ReadRequestTupleKey struct {
	User     string
	Relation string
	Object   string
}

type TupleKeyAuthorization struct {
	Tuple      *TupleKey
	Authorized bool
}

func NewTuples(resource coretypes.Resource, subject string, relation Relation, selectors []coretypes.Selector, orgID valuer.UUID) []*TupleKey {
	tuples := make([]*TupleKey, 0)

	for _, selector := range selectors {
		object := resource.Object(orgID, selector.String())
		tuples = append(tuples, &TupleKey{User: subject, Relation: relation.StringValue(), Object: object})
	}

	return tuples
}

func NewTuplesFromTransactions(transactions []*Transaction, subject string, orgID valuer.UUID) (map[string]*TupleKey, error) {
	tuples := make(map[string]*TupleKey, len(transactions))
	for _, txn := range transactions {
		resource, err := coretypes.NewResourceFromTypeAndKind(txn.Object.Resource.Type, txn.Object.Resource.Kind)
		if err != nil {
			return nil, err
		}

		txnTuples := NewTuples(resource, subject, txn.Relation, []coretypes.Selector{txn.Object.Selector}, orgID)

		// Each transaction produces one tuple, keyed by transaction ID
		tuples[txn.ID.StringValue()] = txnTuples[0]
	}

	return tuples, nil
}

func NewTuplesFromTransactionGroups(name string, orgID valuer.UUID, transactionGroups []*TransactionGroup) ([]*TupleKey, error) {
	tuples := make([]*TupleKey, 0)
	subject := MustNewSubject(coretypes.NewResourceRole(), name, orgID, &coretypes.VerbAssignee)

	for _, transactionGroup := range transactionGroups {
		if err := coretypes.ErrIfVerbNotValidForResource(transactionGroup.Relation.Verb, transactionGroup.ObjectGroup.Resource); err != nil {
			return nil, err
		}

		resource, err := coretypes.NewResourceFromTypeAndKind(transactionGroup.ObjectGroup.Resource.Type, transactionGroup.ObjectGroup.Resource.Kind)
		if err != nil {
			return nil, err
		}

		objectGroupTuples := NewTuples(resource, subject, transactionGroup.Relation, transactionGroup.ObjectGroup.Selectors, orgID)
		tuples = append(tuples, objectGroupTuples...)
	}

	return tuples, nil
}

func MustNewTransactionGroupsFromTuples(tuples []*TupleKey) []*TransactionGroup {
	objectsByRelation := make(map[string][]*coretypes.Object)

	for _, tuple := range tuples {
		verb, err := coretypes.NewVerb(tuple.GetRelation())
		if err != nil {
			panic(err)
		}

		object := coretypes.MustNewObjectFromString(tuple.GetObject())
		objectsByRelation[verb.StringValue()] = append(objectsByRelation[verb.StringValue()], object)
	}

	transactionGroups := make([]*TransactionGroup, 0)
	for _, verb := range coretypes.Verbs {
		objects := objectsByRelation[verb.StringValue()]
		if len(objects) == 0 {
			continue
		}

		for _, objectGroup := range coretypes.NewObjectGroupsFromObjects(objects) {
			transactionGroups = append(transactionGroups, &TransactionGroup{
				Relation:    Relation{Verb: verb},
				ObjectGroup: *objectGroup,
			})
		}
	}

	return transactionGroups
}

func NewTuplesFromTransactionsWithCorrelations(transactions []*Transaction, subject string, orgID valuer.UUID) (tuples map[string]*TupleKey, correlations map[string][]string, err error) {
	tuples = make(map[string]*TupleKey)
	correlations = make(map[string][]string)

	for _, txn := range transactions {
		resource, err := coretypes.NewResourceFromTypeAndKind(txn.Object.Resource.Type, txn.Object.Resource.Kind)
		if err != nil {
			return nil, nil, err
		}

		txnID := txn.ID.StringValue()

		txnTuples := NewTuples(resource, subject, txn.Relation, []coretypes.Selector{txn.Object.Selector}, orgID)
		tuples[txnID] = txnTuples[0]

		if txn.Object.Selector.String() != coretypes.WildCardSelectorString {
			wildcardSelector := txn.Object.Resource.Type.MustSelector(coretypes.WildCardSelectorString)
			wildcardTuples := NewTuples(resource, subject, txn.Relation, []coretypes.Selector{wildcardSelector}, orgID)

			correlationID := valuer.GenerateUUID().StringValue()
			tuples[correlationID] = wildcardTuples[0]
			correlations[txnID] = append(correlations[txnID], correlationID)
		}
	}

	return tuples, correlations, nil
}

func NewTuplesFromTransactionsWithManagedRoles(
	transactions []*Transaction,
	subject string,
	orgID valuer.UUID,
	managedRolesByTransaction map[string][]string,
) (tuples map[string]*TupleKey, preResolved map[string]bool, roleCorrelations map[string][]string, err error) {
	tuples = make(map[string]*TupleKey)
	preResolved = make(map[string]bool)
	roleCorrelations = make(map[string][]string)

	for _, txn := range transactions {
		txnID := txn.ID.StringValue()

		if txn.Object.Resource.Type.Equals(coretypes.TypeRole) && txn.Relation.Verb == coretypes.VerbAssignee {
			resource, err := coretypes.NewResourceFromTypeAndKind(txn.Object.Resource.Type, txn.Object.Resource.Kind)
			if err != nil {
				return nil, nil, nil, err
			}

			txnTuples := NewTuples(resource, subject, txn.Relation, []coretypes.Selector{txn.Object.Selector}, orgID)

			tuples[txnID] = txnTuples[0]
			continue
		}

		roleNames, found := managedRolesByTransaction[txn.TransactionKey()]
		if !found || len(roleNames) == 0 {
			preResolved[txnID] = false
			continue
		}

		for _, roleName := range roleNames {
			roleSelector := coretypes.TypeRole.MustSelector(roleName)
			roleTuples := NewTuples(coretypes.ResourceRole, subject, Relation{Verb: coretypes.VerbAssignee}, []coretypes.Selector{roleSelector}, orgID)

			correlationID := valuer.GenerateUUID().StringValue()
			tuples[correlationID] = roleTuples[0]
			roleCorrelations[txnID] = append(roleCorrelations[txnID], correlationID)
		}
	}

	return tuples, preResolved, roleCorrelations, nil
}

func NewTransactionWithAuthorizationFromBatchResults(
	transactions []*Transaction,
	batchResults map[string]*TupleKeyAuthorization,
	preResolved map[string]bool,
	roleCorrelations map[string][]string,
) []*TransactionWithAuthorization {
	output := make([]*TransactionWithAuthorization, len(transactions))
	for i, txn := range transactions {
		txnID := txn.ID.StringValue()

		if authorized, ok := preResolved[txnID]; ok {
			output[i] = &TransactionWithAuthorization{
				Transaction: txn,
				Authorized:  authorized,
			}
			continue
		}

		if txn.Object.Resource.Type.Equals(coretypes.TypeRole) && txn.Relation.Verb == coretypes.VerbAssignee {
			output[i] = &TransactionWithAuthorization{
				Transaction: txn,
				Authorized:  batchResults[txnID].Authorized,
			}
			continue
		}

		correlationIDs := roleCorrelations[txnID]
		authorized := false
		for _, correlationID := range correlationIDs {
			if result, exists := batchResults[correlationID]; exists && result.Authorized {
				authorized = true
				break
			}
		}

		output[i] = &TransactionWithAuthorization{
			Transaction: txn,
			Authorized:  authorized,
		}
	}

	return output
}
