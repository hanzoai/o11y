package ruler

import "github.com/hanzoai/o11y/pkg/statsreporter"

type Ruler interface {
	statsreporter.StatsCollector
}
