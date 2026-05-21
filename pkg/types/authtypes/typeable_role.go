package authtypes

import (
	"github.com/hanzoai/o11y/pkg/valuer"
)

var _ Typeable = new(typeableRole)

type typeableRole struct{}

func (typeableRole *typeableRole) Tuples(subject string, relation Relation, selectors []Selector, orgID valuer.UUID) ([]*TupleKey, error) {
	tuples := make([]*TupleKey, 0)

	for _, selector := range selectors {
		object := typeableRole.Prefix(orgID) + "/" + selector.String()
		tuples = append(tuples, &TupleKey{User: subject, Relation: relation.StringValue(), Object: object})
	}

	return tuples, nil
}

func (typeableRole *typeableRole) Type() Type {
	return TypeRole
}

func (typeableRole *typeableRole) Name() Name {
	return MustNewName("role")
}

// example: role:organization/0199c47d-f61b-7833-bc5f-c0730f12f046/role
func (typeableRole *typeableRole) Prefix(orgID valuer.UUID) string {
	return typeableRole.Type().StringValue() + ":" + "organization" + "/" + orgID.StringValue() + "/" + typeableRole.Name().String()
}

func (typeableRole *typeableRole) Scope(relation Relation) string {
	return typeableRole.Name().String() + ":" + relation.StringValue()
}
