package pprof

import "github.com/hanzoai/o11y/pkg/factory"

// PProf is the interface that wraps the pprof service lifecycle.
type PProf interface {
	factory.Service
}
