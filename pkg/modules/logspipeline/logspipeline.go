package logspipeline

import "github.com/hanzoai/o11y/pkg/statsreporter"

type Module interface {
	statsreporter.StatsCollector
}
