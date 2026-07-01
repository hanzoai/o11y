package meterreporter

import (
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
)

var (
	ErrCodeInvalidInput = errors.MustNewCode("meterreporter_invalid_input")
)

type Reporter interface {
	factory.ServiceWithHealthy
}
