package authtypes

import (
	"github.com/hanzoai/o11y/pkg/valuer"
)

// TupleKey is the canonical relationship-tuple type. Mirrors the
// openfga.v1.TupleKey wire shape (User / Relation / Object), but lives
// here so the authz API stays transport-agnostic — keeps grpc and
// protobuf out of the default dep graph.
type TupleKey struct {
	User     string
	Relation string
	Object   string
}

type TupleKeyAuthorization struct {
	Tuple      *TupleKey
	Authorized bool
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

// NewTuplesFromTransactionsWithCorrelations converts transactions to tuples for BatchCheck,
// and for each transaction whose selector is not already a wildcard, generates an additional
// tuple with the wildcard selector. This ensures that permissions granted via wildcard
// selectors (e.g., dashboard:*) are checked alongside exact selectors (e.g., dashboard:abc-123).
//
// Returns:
//   - tuples: all tuples to check (exact + correlated), keyed by transaction ID or generated correlation ID
//   - correlations: maps transaction ID to a slice of correlation IDs for the additional tuples
func NewTuplesFromTransactionsWithCorrelations(transactions []*Transaction, subject string, orgID valuer.UUID) (tuples map[string]*openfgav1.TupleKey, correlations map[string][]string, err error) {
	tuples = make(map[string]*openfgav1.TupleKey)
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

// NewTuplesFromTransactionsWithManagedRoles converts transactions to tuples for BatchCheck.
// Direct role-assignment transactions (TypeRole + VerbAssignee) produce one tuple keyed by txn ID.
// Other transactions are expanded via managedRolesByTransaction into role-assignee checks, keyed by "txnID:roleName".
// Transactions with no managed role mapping are marked as pre-resolved (false) in the returned map.
func NewTuplesFromTransactionsWithManagedRoles(
	transactions []*Transaction,
	subject string,
	orgID valuer.UUID,
	managedRolesByTransaction map[string][]string,
) (tuples map[string]*openfgav1.TupleKey, preResolved map[string]bool, roleCorrelations map[string][]string, err error) {
	tuples = make(map[string]*openfgav1.TupleKey)
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

// NewTransactionWithAuthorizationFromBatchResults merges batch check results into an ordered
// slice of TransactionWithAuthorization matching the input transactions order.
// preResolved contains txn IDs whose authorization was determined without BatchCheck.
// roleCorrelations maps txn IDs to correlation IDs used for managed role checks.
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
