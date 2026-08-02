package binding

import (
	"github.com/hanzoai/o11y/pkg/errors"
)

const (
	ErrMessageInvalidQuery string = "request query contains invalid fields, please verify the format and try again."
)

var _ BindingQuery = (*queryBinding)(nil)

type queryBinding struct{}

func (b *queryBinding) BindQuery(query map[string][]string, obj any) error {
	err := bindValues(obj, query, "query")
	if err != nil {
		return errors.New(errors.TypeInvalidInput, ErrCodeInvalidRequestQuery, ErrMessageInvalidQuery).WithAdditional(err.Error())
	}

	return nil
}
