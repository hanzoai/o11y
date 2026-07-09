package o11y

import (
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/hanzoai/cloud"
	luxlog "github.com/luxfi/log"
	"github.com/zap-proto/zip"
)

// Mount registers Hanzo o11y's HTTP surface under /v1/o11y per HIP-0106.
//
// The o11y runtime (metrics, traces, logs, dashboards, alerts) is heavy:
// telemetry stores, rule manager, websocket attachments, opamp server.
// The standalone cmd/server boot path constructs it all. To keep the
// route layer composable with the unified cloud binary, Mount delegates
// to a handler registered by the runtime via SetHandler.
//
// Routing model:
//
//   - Standalone: cmd/server/server.go constructs *Server, calls
//     o11y.SetHandler(server.PublicHandler()), then cloud.MountAll wires it.
//   - Cloud binary: same SetHandler call, executed from the cloud bootstrapper
//     after o11y.New + app.NewServer.
//   - Until a handler is registered, the routes 503 with a clear error.
//
// All traffic under /v1/o11y is delegated to the registered http.Handler via
// zip.AdaptNetHTTP; handlerAdapter normalizes the /v1/o11y/<resource> public
// contract onto the two internal route families HERE — the ONE Hanzo-owned seam —
// so the embedded SigNoz route literals stay untouched (see rewriteExternalPath).
func Mount(app *zip.App, deps cloud.Deps) error {
	log := deps.Logger
	if log == nil {
		log = luxlog.New("module", "o11y")
	}
	log.Info("o11y: mounting routes", "prefix", "/v1/o11y")

	app.All("/v1/o11y/*", zip.AdaptNetHTTP(handlerAdapter{}))
	return nil
}

func init() {
	cloud.Register("o11y", 70, func(app any, deps cloud.Deps) error {
		return Mount(app.(*zip.App), deps)
	})
}

// handlerAdapter forwards each request to the registered runtime handler
// or returns 503 if none is set yet.
type handlerAdapter struct{}

func (handlerAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := getHandler()
	if h == nil {
		http.Error(w, "o11y runtime not initialized", http.StatusServiceUnavailable)
		return
	}
	rewriteExternalPath(r.URL)
	h.ServeHTTP(w, r)
}

// rewriteExternalPath maps the ONE public o11y contract — api.hanzo.ai/v1/o11y/<resource>,
// one /v1/, no /api/ — onto the two internal route families, at this single Hanzo-owned
// seam. It is done HERE, never by editing the embedded SigNoz route literals: SigNoz's
// whole frontend and backend speak /api/vN, and rewriting those literals is a fork diff
// that a later upstream re-sync silently reverts (it already happened once — see
// o11y/CLAUDE.md).
//
//	SigNoz native (registered at /api/vN/*):
//	  /v1/o11y/vN/…      → /api/vN/…   (canonical — the /api/ never surfaces)
//	  /v1/o11y/api/vN/…  → /api/vN/…   (deprecated alias: the leaked form callers emit
//	                                    today. Drop once every consumer emits the
//	                                    canonical form — one and one way.)
//	Hanzo llmobs (registered natively at /v1/o11y/{traces,observations,…}): passed
//	through unchanged.
//
// This requires the embedded SigNoz StripPrefix wrapper to be OFF — cloud CR
// O11Y_GLOBAL_EXTERNAL__URL="" — so a /v1/o11y/* llmobs path survives to the router.
func rewriteExternalPath(u *url.URL) {
	rest, ok := strings.CutPrefix(u.Path, "/v1/o11y/")
	if !ok {
		return
	}
	switch {
	case strings.HasPrefix(rest, "api/v"): // deprecated leaked alias: /v1/o11y/api/vN/x
		setPath(u, "/"+rest) // → /api/vN/x
	case isVersionSegment(rest): // canonical SigNoz form: /v1/o11y/vN/x
		setPath(u, "/api/"+rest) // → /api/vN/x
	default: // Hanzo llmobs / native resource — the router owns /v1/o11y/x directly.
	}
}

// isVersionSegment reports whether rest begins with a SigNoz API version segment
// (v followed by a digit — "v1/health", "v3/query_range"): the marker that tells an
// embedded-SigNoz route apart from a Hanzo-native llmobs resource (traces, sessions, …).
func isVersionSegment(rest string) bool {
	return len(rest) >= 2 && rest[0] == 'v' && rest[1] >= '0' && rest[1] <= '9'
}

// setPath rewrites the request path, clearing RawPath so EscapedPath re-derives from the
// new value — the rewritten SigNoz paths contain no characters needing escaping, and
// llmobs paths (the only ones carrying an {id}) are never rewritten.
func setPath(u *url.URL, p string) {
	u.Path = p
	u.RawPath = ""
}

var (
	hmu        sync.RWMutex
	registered http.Handler
)

// SetHandler registers the o11y runtime's public HTTP handler. The
// standalone server calls this after app.NewServer; the unified cloud
// binary calls it after constructing the same runtime in-process.
// Safe for concurrent use; pass nil to unset.
func SetHandler(h http.Handler) {
	hmu.Lock()
	registered = h
	hmu.Unlock()
}

func getHandler() http.Handler {
	hmu.RLock()
	h := registered
	hmu.RUnlock()
	return h
}
