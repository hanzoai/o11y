package o11yapiserver

import (
	"net/http"

	"github.com/gorilla/mux"
	openapi "github.com/swaggest/openapi-go"

	"github.com/hanzoai/o11y/pkg/factory"
	pkghandler "github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/render"
)

type healthOpenAPIHandler struct {
	handlerFunc http.HandlerFunc
	id          string
	summary     string
}

func newHealthOpenAPIHandler(handlerFunc http.HandlerFunc, id, summary string) pkghandler.Handler {
	return &healthOpenAPIHandler{
		handlerFunc: handlerFunc,
		id:          id,
		summary:     summary,
	}
}

func (handler *healthOpenAPIHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	handler.handlerFunc.ServeHTTP(rw, req)
}

func (handler *healthOpenAPIHandler) ServeOpenAPI(opCtx openapi.OperationContext) {
	opCtx.SetID(handler.id)
	opCtx.SetTags("health")
	opCtx.SetSummary(handler.summary)

	response := render.SuccessResponse{
		Status: render.StatusSuccess.String(),
		Data:   new(factory.Response),
	}

	opCtx.AddRespStructure(
		response,
		openapi.WithContentType("application/json"),
		openapi.WithHTTPStatus(http.StatusOK),
	)
	opCtx.AddRespStructure(
		response,
		openapi.WithContentType("application/json"),
		openapi.WithHTTPStatus(http.StatusServiceUnavailable),
	)
}

func (handler *healthOpenAPIHandler) AuditDef() *pkghandler.AuditDef {
	// Health endpoints are not audited since they don't represent user actions and are called frequently by monitoring systems, which would create noise in the audit logs.
	return nil
}

func (provider *provider) addRegistryRoutes(router *mux.Router) error {
	if err := router.Handle("/v1/o11y/v2/healthz", newHealthOpenAPIHandler(
		provider.authzMiddleware.OpenAccess(provider.factoryHandler.Healthz),
		"Healthz",
		"Health check",
	)).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/v2/readyz", newHealthOpenAPIHandler(
		provider.authzMiddleware.OpenAccess(provider.factoryHandler.Readyz),
		"Readyz",
		"Readiness check",
	)).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/v2/livez", pkghandler.New(provider.authzMiddleware.OpenAccess(provider.factoryHandler.Livez),
		pkghandler.OpenAPIDef{
			ID:                  "Livez",
			Tags:                []string{"health"},
			Summary:             "Liveness check",
			Response:            new(factory.Response),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
		},
	)).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	return nil
}
