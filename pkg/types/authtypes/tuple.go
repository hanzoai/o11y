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
		typeable, err := NewTypeableFromType(txn.Object.Resource.Type, txn.Object.Resource.Name)
		if err != nil {
			return nil, err
		}

		txnTuples, err := typeable.Tuples(subject, txn.Relation, []Selector{txn.Object.Selector}, orgID)
		if err != nil {
			return nil, err
		}

		// Each transaction produces one tuple, keyed by transaction ID
		tuples[txn.ID.StringValue()] = txnTuples[0]
	}

	return tuples, nil
}
