package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// Query progress streaming. 2 routes, one handler, two mount points — the
// long-poll path under /v1/o11y and the websocket path under /ws. They are in
// one file so the pair is retired together.
//
// ESCAPE HATCH. Both are deliberately NOT typed query-core ops: GetQueryProgressUpdates
// upgrades the connection to a websocket, and the query-core relay buffers a whole
// answer through an httptest recorder that cannot hijack a connection — a typed
// progress op would never complete the handshake. They stay where they are,
// byte-identical; see the escape-hatch record in querycore.go.

// mountQueryProgress registers the /v1/o11y long-poll path on the shared
// subrouter owned by RegisterQueryRangeV3Routes. 1 route.
//
// TODO(Raj): Remove this handler after /ws based path has been completely rolled out.
func (aH *APIHandler) mountQueryProgress(subRouter *mux.Router, am *middleware.AuthZ) {
	subRouter.HandleFunc("/query_progress", am.ViewAccess(aH.GetQueryProgressUpdates)).Methods(http.MethodGet)
}

// mountWebSocketQueryProgress registers the websocket path on the /ws
// subrouter owned by RegisterWebSocketPaths. 1 route.
func (aH *APIHandler) mountWebSocketQueryProgress(subRouter *mux.Router, am *middleware.AuthZ) {
	subRouter.HandleFunc("/query_progress", am.ViewAccess(aH.GetQueryProgressUpdates)).Methods(http.MethodGet)
}
