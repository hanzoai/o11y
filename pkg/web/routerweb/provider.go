package routerweb

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/web"
)

// apiPrefixes are the request-path roots the console must NEVER answer. They
// belong to the API planes — /v1 (every o11y and sentry route) and /ws (the
// query-progress socket) — which are registered on the host's router BEFORE the
// console and therefore win for every route that actually exists. An UNMATCHED
// path under one of them (a typo, a client on a route this build does not serve,
// a method the route does not take) must be a real 404, not the SPA shell:
// answering /v1/o11y/pods/bogus with 200 text/html is exactly the "200 that was
// HTML where JSON belonged" failure — and it is what this catch-all did, because
// gorilla resumes the parent router when a subrouter's prefix matches but none
// of its routes do, so every miss inside /v1/o11y fell through to here.
var apiPrefixes = []string{"/v1"}

// provider serves the o11y console: the templated index shell, the built assets
// beside it, and the shell again for any client-side route. A stdlib
// http.Handler and nothing else — the same shape hanzoai/cloud's webui console
// has, and the reason is the same: the console must be servable from a net/http
// host (the standalone query server's middleware chain) AND from a zip host
// (app.All("/*", zip.AdaptNetHTTP(web))), and one handler answers both
// identically. zip.Static was the alternative and cannot do this job: it serves
// bytes out of an fs.FS, and the shell is not a file in the tree — it is
// rendered once at startup with this deployment's base href and settings, so
// zip.Static's WithIndex/WithFallback would serve the raw index.html and hand
// the browser a console with no base href and no boot data.
type provider struct {
	config        web.Config
	indexContents []byte
	fileHandler   http.Handler
	handler       http.Handler
}

func NewFactory(globalConfig global.Config) factory.ProviderFactory[web.Web, web.Config] {
	return factory.NewProviderFactory(factory.MustNewName("router"), func(ctx context.Context, settings factory.ProviderSettings, config web.Config) (web.Web, error) {
		return New(ctx, settings, config, globalConfig)
	})
}

func New(ctx context.Context, settings factory.ProviderSettings, config web.Config, globalConfig global.Config) (web.Web, error) {
	fi, err := os.Stat(config.Directory)
	if err != nil {
		return nil, errors.WrapInvalidInputf(err, errors.CodeInvalidInput, "cannot access web directory")
	}

	if !fi.IsDir() {
		return nil, errors.NewInvalidInputf(errors.CodeInvalidInput, "web directory is not a directory")
	}

	indexPath := filepath.Join(config.Directory, config.Index)
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, errors.WrapInvalidInputf(err, errors.CodeInvalidInput, "cannot read %q in web directory", config.Index)
	}

	webSettings := web.NewSettings(config)
	settingsJSON, err := json.Marshal(webSettings)
	if err != nil {
		return nil, errors.WrapInternalf(err, errors.CodeInternal, "cannot marshal web settings to JSON")
	}

	logger := factory.NewScopedProviderSettings(settings, "github.com/hanzoai/o11y/pkg/web/routerweb").Logger()
	indexContents := web.NewIndex(ctx, logger, config.Index, raw, web.TemplateData{
		BaseHref: globalConfig.ExternalPathTrailing(),
		Settings: template.JS(settingsJSON),
	})

	provider := &provider{
		config:        config,
		indexContents: indexContents,
		fileHandler:   http.FileServer(http.Dir(config.Directory)),
	}
	// The console carries its own cache policy, taken from the ONE place that
	// policy is written, so every host serves it with the same headers instead of
	// each remembering to wrap it.
	provider.handler = middleware.NewCache(0).Wrap(http.HandlerFunc(provider.serve))

	return provider, nil
}

func (provider *provider) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	provider.handler.ServeHTTP(rw, req)
}

func (provider *provider) serve(rw http.ResponseWriter, req *http.Request) {
	if isAPI(req.URL.Path) {
		http.NotFound(rw, req)
		return
	}

	// Join internally calls path.Clean to prevent directory traversal.
	path := filepath.Join(provider.config.Directory, req.URL.Path)

	fi, err := os.Stat(path)
	switch {
	case err == nil && !fi.IsDir():
		// A real asset: http.FileServer types it from its extension, so a hashed
		// bundle answers as javascript and never as the shell's text/html.
		provider.fileHandler.ServeHTTP(rw, req)
	case err == nil, os.IsNotExist(err):
		// A directory, or a path with no file behind it: a client-side route.
		provider.serveIndex(rw)
	default:
		// Stat failed for a reason other than absence — the file is there and we
		// cannot read it. Say so; do not answer a broken deployment with a shell.
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (provider *provider) serveIndex(rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = rw.Write(provider.indexContents)
}

// isAPI reports whether path belongs to an API plane. The exact prefix counts
// too ("/v1" itself), so the boundary is the segment and not the substring: it
// must not swallow a console route that merely starts with the same letters
// (/v1beta-something is the console's, /v1/anything is not).
func isAPI(path string) bool {
	for _, prefix := range apiPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
