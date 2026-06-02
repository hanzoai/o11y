package authtypes

import "github.com/hanzoai/o11y/pkg/valuer"

func NewSubject(resource coretypes.Resource, selector string, orgID valuer.UUID, relation *coretypes.Verb) (string, error) {
	if relation == nil {
		return resource.Prefix(orgID) + "/" + selector, nil
	}

	return resource.Prefix(orgID) + "/" + selector + "#" + relation.StringValue(), nil
}

func MustNewSubject(resource coretypes.Resource, selector string, orgID valuer.UUID, relation *coretypes.Verb) string {
	subject, err := NewSubject(resource, selector, orgID, relation)
	if err != nil {
		panic(err)
	}

	return subject
}
