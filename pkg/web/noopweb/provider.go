package noopweb

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/web"
)

type provider struct{}

func NewFactory() factory.ProviderFactory[web.Web, web.Config] {
	return factory.NewProviderFactory(factory.MustNewName("noop"), New)
}

func New(ctx context.Context, settings factory.ProviderSettings, config web.Config) (web.Web, error) {
	return &provider{}, nil
}

// ServeHTTP is the honest null: this deployment serves no console (the shipped
// image runs headless — the SPA is served at the edge by hanzoai/static — so
// web.enabled=false selects this provider), and a path that reached the console
// matched no API route either. 404 is what that has always meant on the wire:
// these are the same bytes gorilla's default NotFoundHandler wrote when the null
// provider registered no route at all.
func (provider *provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
