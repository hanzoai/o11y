package apiserver

import (
	"github.com/hanzoai/o11y/pkg/http/routing"
)

type APIServer interface {
	// AddToRouter registers every route this API server serves on the host's
	// router. There is no second router and no Router() to read one back from:
	// the routes live where the host serves them, and the census of what was
	// registered is the router's own Table.
	AddToRouter(router routing.Router)
}
