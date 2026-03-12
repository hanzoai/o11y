package flagger

import (
	"context"
	"net/http"
	"time"

	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/featuretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type Handler interface {
	GetFeatures(http.ResponseWriter, *http.Request)
}

type handler struct {
	flagger Flagger
}

func NewHandler(flagger Flagger) Handler {
	return &handler{
		flagger: flagger,
	}
}

func (handler *handler) GetFeatures(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	orgID, err := valuer.NewUUID(claims.OrgID)
	if err != nil {
		render.Error(rw, err)
		return
	}

	evalCtx := featuretypes.NewFlaggerEvaluationContext(orgID)

	features, err := handler.flagger.List(ctx, evalCtx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusOK, features)
}
