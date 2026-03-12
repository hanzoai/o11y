package spanpercentile

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/types/spanpercentiletypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type Module interface {
	GetSpanPercentile(ctx context.Context, orgID valuer.UUID, userID valuer.UUID, req *spanpercentiletypes.SpanPercentileRequest) (*spanpercentiletypes.SpanPercentileResponse, error)
}

type Handler interface {
	GetSpanPercentileDetails(http.ResponseWriter, *http.Request)
}
