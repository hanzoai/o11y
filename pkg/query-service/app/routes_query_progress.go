package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// Query progress streaming. 1 route, one handler, one mount point.
//
// It was two — the long-poll under /v1/o11y and a websocket under /ws — for one
// handler that reads the same progress either way. The Upgrade is a property of
// the REQUEST, not of the address: GetQueryProgressUpdates upgrades when the
// caller asks and long-polls when it does not, so a second path bought a second
// spelling of one read and nothing else. /ws was also the last address this
// service answered outside /v1.
//
// ESCAPE HATCH. It is deliberately NOT a typed query-core op:
// GetQueryProgressUpdates upgrades the connection to a websocket, and the
// query-core relay buffers a whole answer through an httptest recorder that
// cannot hijack a connection — a typed progress op would never complete the
// handshake. It stays a hatch, byte-identical; see the escape-hatch record in
// querycore.go.

// mountQueryProgress registers the progress path on the shared /v1/o11y
// subrouter owned by RegisterQueryRangeV3Routes. 1 route, both protocols.
func (aH *APIHandler) mountQueryProgress(subRouter routing.Router, am *middleware.AuthZ) {
	subRouter.Get("/query_progress", am.ViewAccess(aH.GetQueryProgressUpdates))
}
