package nooplicensing

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/licensing"
)

type noopLicensingAPI struct{}

func NewLicenseAPI() licensing.API {
	return &noopLicensingAPI{}
}

func (api *noopLicensingAPI) Activate(rw http.ResponseWriter, r *http.Request) {
	render.Error(rw, errors.New(errors.TypeUnsupported, licensing.ErrCodeUnsupported, "not implemented"))
}

func (api *noopLicensingAPI) GetActive(rw http.ResponseWriter, r *http.Request) {
	render.Error(rw, errors.New(errors.TypeUnsupported, licensing.ErrCodeUnsupported, "not implemented"))
}

func (api *noopLicensingAPI) Refresh(rw http.ResponseWriter, r *http.Request) {
	render.Error(rw, errors.New(errors.TypeUnsupported, licensing.ErrCodeUnsupported, "not implemented"))
}

func (api *noopLicensingAPI) Checkout(rw http.ResponseWriter, r *http.Request) {
	render.Error(rw, errors.New(errors.TypeUnsupported, licensing.ErrCodeUnsupported, "not implemented"))
}

func (api *noopLicensingAPI) Portal(rw http.ResponseWriter, r *http.Request) {
	render.Error(rw, errors.New(errors.TypeUnsupported, licensing.ErrCodeUnsupported, "not implemented"))
}
