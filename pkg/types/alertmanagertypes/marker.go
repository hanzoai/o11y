package alertmanagertypes

import (
	"github.com/prometheus/alertmanager/types"
	"github.com/luxfi/metric"
)

type MemMarker = types.MemMarker

func NewMarker(r metric.Registerer) *MemMarker {
	return types.NewMarker(r)
}
