package authtypes

import (
	"github.com/hanzoai/o11y/pkg/valuer"
)

var _ Typeable = new(typeableUser)

type typeableUser struct{}

func (typeableUser *typeableUser) Tuples(subject string, relation Relation, selectors []Selector, orgID valuer.UUID) ([]*TupleKey, error) {
	tuples := make([]*TupleKey, 0)

	for _, selector := range selectors {
		object := typeableUser.Prefix(orgID) + "/" + selector.String()
		tuples = append(tuples, &TupleKey{User: subject, Relation: relation.StringValue(), Object: object})
	}

	return tuples, nil
}

func (typeableUser *typeableUser) Type() Type {
	return TypeUser
}

func (typeableUser *typeableUser) Name() Name {
	return MustNewName("user")
}

// example: user:organization/0199c47d-f61b-7833-bc5f-c0730f12f046/user
func (typeableUser *typeableUser) Prefix(orgID valuer.UUID) string {
	return typeableUser.Type().StringValue() + ":" + "organization" + "/" + orgID.StringValue() + "/" + typeableUser.Name().String()
}

func (typeableUser *typeableUser) Scope(relation Relation) string {
	return typeableUser.Name().String() + ":" + relation.StringValue()
}
