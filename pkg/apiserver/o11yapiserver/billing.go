package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/billingtypes"
	"github.com/gorilla/mux"
)

func (provider *provider) addBillingRoutes(router *mux.Router) error {
	if err := router.Handle("/api/v2/billing/profiles", handler.New(provider.authZ.AdminAccess(provider.billingHandler.PutProfile), handler.OpenAPIDef{
		ID:                  "PutProfile",
		Tags:                []string{"billing"},
		Summary:             "Put profile for a deployment.",
		Description:         "This endpoint saves the profile of a deployment.",
		Request:             new(billingtypes.PostableProfile),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	})).Methods(http.MethodPut).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/api/v2/billing/hosts", handler.New(provider.authZ.AdminAccess(provider.billingHandler.GetHosts), handler.OpenAPIDef{
		ID:                  "GetHosts",
		Tags:                []string{"billing"},
		Summary:             "Get host info.",
		Description:         "This endpoint gets the host info.",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(billingtypes.GettableHost),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/api/v2/billing/hosts", handler.New(provider.authZ.AdminAccess(provider.billingHandler.PutHost), handler.OpenAPIDef{
		ID:                  "PutHost",
		Tags:                []string{"billing"},
		Summary:             "Put host for a deployment.",
		Description:         "This endpoint saves the host of a deployment.",
		Request:             new(billingtypes.PostableHost),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	})).Methods(http.MethodPut).GetError(); err != nil {
		return err
	}

	return nil
}
