package quickfilter

import (
	"context"
	"net/http"

	v3 "github.com/hanzoai/o11y/pkg/query-service/model/v3"
	"github.com/hanzoai/o11y/pkg/types/quickfiltertypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type Module interface {
	GetQuickFilters(ctx context.Context, orgID valuer.UUID) ([]*quickfiltertypes.SignalFilters, error)
	UpdateQuickFilters(ctx context.Context, orgID valuer.UUID, signal quickfiltertypes.Signal, filters []v3.AttributeKey) error
	GetSignalFilters(ctx context.Context, orgID valuer.UUID, signal quickfiltertypes.Signal) (*quickfiltertypes.SignalFilters, error)
	SetDefaultConfig(ctx context.Context, orgID valuer.UUID) error
}

type Handler interface {
	GetQuickFilters(http.ResponseWriter, *http.Request)
	UpdateQuickFilters(http.ResponseWriter, *http.Request)
	GetSignalFilters(http.ResponseWriter, *http.Request)
}
