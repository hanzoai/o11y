package implerrortracking

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/http/binding"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

const (
	viewTimeout  = 30 * time.Second
	writeTimeout = 15 * time.Second
)

type handler struct {
	module errortracking.Module
}

// NewHandler builds the read surface over the issue lifecycle. Wire ingest belongs to
// /v1/sentry, which owns the DSN credential and the event.error write.
func NewHandler(module errortracking.Module) errortracking.Handler {
	return &handler{module: module}
}

// --- reads (Hanzo IAM authz, org-scoped) ---

func (h *handler) ListIssues(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()

	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	var q errortrackingtypes.IssuesQuery
	if err := binding.Query.BindQuery(r.URL.Query(), &q); err != nil {
		render.Error(rw, err)
		return
	}
	items, total, err := h.module.ListIssues(ctx, orgID, &q)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, &errortrackingtypes.GettableIssues{
		Items: items, Total: total, Offset: clampOffset(q.Offset), Limit: clampLimit(q.Limit),
	})
}

func (h *handler) GetIssue(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()

	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	id, err := idFromPath(r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	issue, err := h.module.GetIssue(ctx, orgID, id)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, issue)
}

func (h *handler) UpdateIssue(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), writeTimeout)
	defer cancel()

	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	id, err := idFromPath(r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	req := new(errortrackingtypes.UpdateIssue)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}
	issue, err := h.module.UpdateIssue(ctx, orgID, id, req)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, issue)
}

// --- shared helpers ---

// orgFromContext resolves the caller's org UUID from the gateway-asserted claims.
// It never panics on a malformed claim (a non-UUID org id fails closed as an
// unauthenticated request rather than crashing the handler).
func orgFromContext(ctx context.Context) (valuer.UUID, error) {
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		return valuer.UUID{}, err
	}
	orgID, err := valuer.NewUUID(claims.OrgID)
	if err != nil {
		return valuer.UUID{}, errors.Wrapf(err, errors.TypeUnauthenticated, errortrackingtypes.ErrCodeErrorTrackingUnauthorized, "identity carries no valid org")
	}
	return orgID, nil
}

func idFromPath(r *http.Request) (valuer.UUID, error) {
	id, err := valuer.NewUUID(mux.Vars(r)["id"])
	if err != nil {
		return valuer.UUID{}, errors.Wrapf(err, errors.TypeInvalidInput, errortrackingtypes.ErrCodeErrorTrackingInvalidInput, "id is not a valid uuid")
	}
	return id, nil
}
