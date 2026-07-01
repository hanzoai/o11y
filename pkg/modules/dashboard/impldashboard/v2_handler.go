package impldashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/http/binding"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/dashboardtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/gorilla/mux"
)

func (handler *handler) CreateV2(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	orgID := valuer.MustNewUUID(claims.OrgID)

	var req dashboardtypes.PostableDashboardV2
	if err := binding.JSON.BindBody(r.Body, &req); err != nil {
		render.Error(rw, err)
		return
	}

	dashboard, err := handler.module.CreateV2(ctx, orgID, claims.Email, valuer.MustNewUUID(claims.IdentityID()), dashboardtypes.SourceUser, req)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusCreated, dashboard.ToGettableDashboardV2())
}

func (handler *handler) GetV2(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	orgID := valuer.MustNewUUID(claims.OrgID)

	id := mux.Vars(r)["id"]
	if id == "" {
		render.Error(rw, errors.Newf(errors.TypeInvalidInput, errors.CodeInvalidInput, "id is missing in the path"))
		return
	}
	dashboardID, err := valuer.NewUUID(id)
	if err != nil {
		render.Error(rw, err)
		return
	}

	dashboard, err := handler.module.GetV2(ctx, orgID, dashboardID)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusOK, dashboard.ToGettableDashboardV2())
}
