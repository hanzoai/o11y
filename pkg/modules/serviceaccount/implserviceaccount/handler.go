package implserviceaccount

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/binding"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/serviceaccounttypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/gorilla/mux"
)

type handler struct {
	module serviceaccount.Module
}

func NewHandler(module serviceaccount.Module) serviceaccount.Handler {
	return &handler{module: module}
}

func (handler *handler) Create(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	req := new(serviceaccounttypes.PostableServiceAccount)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}

	serviceAccount := serviceaccounttypes.NewServiceAccount(req.Name, req.Email, req.Roles, serviceaccounttypes.StatusActive, valuer.MustNewUUID(claims.OrgID))
	err = handler.module.Create(ctx, valuer.MustNewUUID(claims.OrgID), serviceAccount)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusCreated, types.Identifiable{ID: serviceAccount.ID})
}

func (handler *handler) Get(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	id, err := valuer.NewUUID(mux.Vars(r)["id"])
	if err != nil {
		render.Error(rw, err)
		return
	}

	serviceAccount, err := handler.module.Get(ctx, valuer.MustNewUUID(claims.OrgID), id)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusOK, serviceAccount)
}

func (handler *handler) List(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	serviceAccounts, err := handler.module.List(ctx, valuer.MustNewUUID(claims.OrgID))
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusOK, serviceAccounts)
}

func (handler *handler) Update(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	id, err := valuer.NewUUID(mux.Vars(r)["id"])
	if err != nil {
		render.Error(rw, err)
		return
	}

	req := new(serviceaccounttypes.UpdatableServiceAccount)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}

	serviceAccount, err := handler.module.Get(ctx, valuer.MustNewUUID(claims.OrgID), id)
	if err != nil {
		render.Error(rw, err)
		return
	}

	serviceAccount.Update(req.Name, req.Email, req.Roles)
	err = handler.module.Update(ctx, valuer.MustNewUUID(claims.OrgID), serviceAccount)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusNoContent, nil)
}

func (handler *handler) UpdateStatus(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	id, err := valuer.NewUUID(mux.Vars(r)["id"])
	if err != nil {
		render.Error(rw, err)
		return
	}

	req := new(serviceaccounttypes.UpdatableServiceAccountStatus)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}

	serviceAccount, err := handler.module.Get(ctx, valuer.MustNewUUID(claims.OrgID), id)
	if err != nil {
		render.Error(rw, err)
		return
	}

	serviceAccount.UpdateStatus(req.Status)
	err = handler.module.UpdateStatus(ctx, valuer.MustNewUUID(claims.OrgID), serviceAccount)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusNoContent, nil)
}

func (handler *handler) Delete(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	id, err := valuer.NewUUID(mux.Vars(r)["id"])
	if err != nil {
		render.Error(rw, err)
		return
	}

	err = handler.module.Delete(ctx, valuer.MustNewUUID(claims.OrgID), id)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusNoContent, nil)
}

func (handler *handler) CreateFactorAPIKey(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	id, err := valuer.NewUUID(mux.Vars(r)["id"])
	if err != nil {
		render.Error(rw, err)
		return
	}

	req := new(serviceaccounttypes.PostableFactorAPIKey)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}

	// this takes care of checking the existence of service account and the org constraint.
	serviceAccount, err := handler.module.GetWithoutRoles(ctx, valuer.MustNewUUID(claims.OrgID), id)
	if err != nil {
		render.Error(rw, err)
		return
	}

	factorAPIKey, err := serviceAccount.NewFactorAPIKey(req.Name, req.ExpiresAt)
	if err != nil {
		render.Error(rw, err)
		return
	}

	err = handler.module.CreateFactorAPIKey(ctx, factorAPIKey)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusCreated, serviceaccounttypes.NewGettableFactorAPIKeyWithKey(factorAPIKey.ID, factorAPIKey.Key))
}

func (handler *handler) ListFactorAPIKey(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	id, err := valuer.NewUUID(mux.Vars(r)["id"])
	if err != nil {
		render.Error(rw, err)
		return
	}

	serviceAccount, err := handler.module.GetWithoutRoles(ctx, valuer.MustNewUUID(claims.OrgID), id)
	if err != nil {
		render.Error(rw, err)
		return
	}

	factorAPIKeys, err := handler.module.ListFactorAPIKey(ctx, serviceAccount.ID)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusOK, serviceaccounttypes.NewGettableFactorAPIKeys(factorAPIKeys))
}

func (handler *handler) UpdateFactorAPIKey(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	id, err := valuer.NewUUID(mux.Vars(r)["id"])
	if err != nil {
		render.Error(rw, err)
		return
	}

	factorAPIKeyID, err := valuer.NewUUID(mux.Vars(r)["fid"])
	if err != nil {
		render.Error(rw, err)
		return
	}

	req := new(serviceaccounttypes.UpdatableFactorAPIKey)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}

	serviceAccount, err := handler.module.GetWithoutRoles(ctx, valuer.MustNewUUID(claims.OrgID), id)
	if err != nil {
		render.Error(rw, err)
		return
	}

	factorAPIKey, err := handler.module.GetFactorAPIKey(ctx, serviceAccount.ID, factorAPIKeyID)
	if err != nil {
		render.Error(rw, err)
		return
	}

	factorAPIKey.Update(req.Name, req.ExpiresAt)
	err = handler.module.UpdateFactorAPIKey(ctx, serviceAccount.ID, factorAPIKey)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusNoContent, nil)
}

func (handler *handler) RevokeFactorAPIKey(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}

	id, err := valuer.NewUUID(mux.Vars(r)["id"])
	if err != nil {
		render.Error(rw, err)
		return
	}

	factorAPIKeyID, err := valuer.NewUUID(mux.Vars(r)["fid"])
	if err != nil {
		render.Error(rw, err)
		return
	}

	serviceAccount, err := handler.module.GetWithoutRoles(ctx, valuer.MustNewUUID(claims.OrgID), id)
	if err != nil {
		render.Error(rw, err)
		return
	}

	err = handler.module.RevokeFactorAPIKey(ctx, serviceAccount.ID, factorAPIKeyID)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusNoContent, nil)
}
