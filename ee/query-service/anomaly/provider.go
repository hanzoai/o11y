package anomaly

import (
	"context"

	"github.com/hanzoai/o11y/pkg/valuer"
)

type Provider interface {
	GetAnomalies(ctx context.Context, orgID valuer.UUID, req *GetAnomaliesRequest) (*GetAnomaliesResponse, error)
}
