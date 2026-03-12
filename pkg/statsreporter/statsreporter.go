package statsreporter

import (
	"context"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type StatsReporter interface {
	factory.Service

	Report(context.Context) error
}

type StatsCollector interface {
	Collect(context.Context, valuer.UUID) (map[string]any, error)
}
