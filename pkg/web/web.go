package web

import (
	"net/http"
)

// Web is the browser console: bytes for a request, and nothing more.
//
// It used to also carry AddToRouter(*mux.Router) — the console's own mounting
// rule, braided into one router's type. That coupling is what kept gorilla/mux
// alive in this package, and it bought nothing: every host registered the SAME
// terminal catch-all, so the rule was the host's — spelled once, where the host
// composes its handler chain — not the console's. Dropping it leaves the value,
// which a net/http host serves directly and a zip host mounts with
// app.All("/*", zip.AdaptNetHTTP(web)) — the hanzoai/cloud webui idiom, one
// implementation reachable from both.
//
// The console is therefore ALWAYS mounted, and "this deployment has no console"
// is a status rather than an absent route: noopweb answers 404, the same bytes
// gorilla's default NotFoundHandler wrote back when the null provider registered
// nothing at all.
type Web interface {
	http.Handler
}
