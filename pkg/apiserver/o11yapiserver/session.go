package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// The four JSON routes here (email_password, context, rotate, delete) are ALSO
// declared as typed ops at the module's mount seam (identity.go in the repo
// root) — a second DISPATCH onto this router, never a second implementation;
// the composed binary serves them as ops, this router serves the standalone
// process. The three /complete/* callbacks are NOT typed: they answer with 303
// redirects whatever happens, and a typed op declares a 2xx JSON contract,
// which would be a lie about them. They reach this router through the
// delegation wildcard in both deployments.
func (provider *provider) addSessionRoutes(router routing.Router) {
	router.Post("/v1/o11y/sessions/email_password", handler.New(provider.authzMiddleware.OpenAccess(provider.sessionHandler.CreateSessionByEmailPassword), handler.OpenAPIDef{
		ID:                  "CreateSessionByEmailPassword",
		Tags:                []string{"sessions"},
		Summary:             "Create session by email and password",
		Description:         "This endpoint creates a session for a user using email and password.",
		Request:             new(authtypes.PostableEmailPasswordSession),
		RequestContentType:  "application/json",
		Response:            new(authtypes.GettableToken),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{},
	}))

	router.Get("/v1/o11y/sessions/context", handler.New(provider.authzMiddleware.OpenAccess(provider.sessionHandler.GetSessionContext), handler.OpenAPIDef{
		ID:                  "GetSessionContext",
		Tags:                []string{"sessions"},
		Summary:             "Get session context",
		Description:         "This endpoint returns the context for the session",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(authtypes.SessionContext),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{},
	}))

	router.Post("/v1/o11y/sessions/rotate", handler.New(provider.authzMiddleware.OpenAccess(provider.sessionHandler.RotateSession), handler.OpenAPIDef{
		ID:                  "RotateSession",
		Tags:                []string{"sessions"},
		Summary:             "Rotate session",
		Description:         "This endpoint rotates the session",
		Request:             new(authtypes.PostableRotateToken),
		RequestContentType:  "application/json",
		Response:            new(authtypes.GettableToken),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{},
	}))

	router.Delete("/v1/o11y/sessions", handler.New(provider.authzMiddleware.OpenAccess(provider.sessionHandler.DeleteSession), handler.OpenAPIDef{
		ID:                  "DeleteSession",
		Tags:                []string{"sessions"},
		Summary:             "Delete session",
		Description:         "This endpoint deletes the session",
		Request:             nil,
		RequestContentType:  "",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{{Name: authtypes.IdentNProviderTokenizer.StringValue()}},
	}))

	router.Get("/v1/o11y/complete/google", handler.New(provider.authzMiddleware.OpenAccess(provider.sessionHandler.CreateSessionByGoogleCallback), handler.OpenAPIDef{
		ID:                  "CreateSessionByGoogleCallback",
		Tags:                []string{"sessions"},
		Summary:             "Create session by google callback",
		Description:         "This endpoint creates a session for a user using google callback",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(authtypes.GettableToken),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusSeeOther,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{},
	}))

	router.Post("/v1/o11y/complete/saml", handler.New(provider.authzMiddleware.OpenAccess(provider.sessionHandler.CreateSessionBySAMLCallback), handler.OpenAPIDef{
		ID:          "CreateSessionBySAMLCallback",
		Tags:        []string{"sessions"},
		Summary:     "Create session by saml callback",
		Description: "This endpoint creates a session for a user using saml callback",
		Request: struct {
			RelayState   string `form:"RelayState"`
			SAMLResponse string `form:"SAMLResponse"`
		}{},
		RequestContentType:  "application/x-www-form-urlencoded",
		Response:            new(authtypes.GettableToken),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusSeeOther,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound, http.StatusUnavailableForLegalReasons},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{},
	}))

	router.Get("/v1/o11y/complete/oidc", handler.New(provider.authzMiddleware.OpenAccess(provider.sessionHandler.CreateSessionByOIDCCallback), handler.OpenAPIDef{
		ID:                  "CreateSessionByOIDCCallback",
		Tags:                []string{"sessions"},
		Summary:             "Create session by oidc callback",
		Description:         "This endpoint creates a session for a user using oidc callback",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(authtypes.GettableToken),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusSeeOther,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound, http.StatusUnavailableForLegalReasons},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{},
	}))
}
