package implsentry

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/http/binding"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

const (
	viewTimeout  = 30 * time.Second
	writeTimeout = 15 * time.Second
)

type handler struct {
	module sentry.Module
}

// NewHandler builds the /v1/o11y/sentinel HTTP surface. Errors enter through
// /v1/event only; this face reads and manages what the event plane holds.
func NewHandler(module sentry.Module) sentry.Handler {
	return &handler{module: module}
}

// --- projects ---

func (h *handler) ListProjects(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()
	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	out, err := h.module.ListProjects(ctx, orgID)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, out)
}

func (h *handler) CreateProject(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), writeTimeout)
	defer cancel()
	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	req := new(sentrytypes.PostableProject)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}
	p, err := h.module.CreateProject(ctx, orgID, req)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, p)
}

func (h *handler) GetProject(rw http.ResponseWriter, r *http.Request) {
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
	p, err := h.module.GetProject(ctx, orgID, id)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, p)
}

func (h *handler) DeleteProject(rw http.ResponseWriter, r *http.Request) {
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
	if err := h.module.DeleteProject(ctx, orgID, id); err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusNoContent, nil)
}

// --- issues ---

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
	var projectID *valuer.UUID
	if raw := r.URL.Query().Get("project"); raw != "" {
		id, err := valuer.NewUUID(raw)
		if err != nil {
			render.Error(rw, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "project is not a valid uuid"))
			return
		}
		projectID = &id
	}
	out, err := h.module.ListIssues(ctx, orgID, projectID, &q, resolveWindow(r.URL.Query().Get("period"), time.Now().UTC()))
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, out)
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

func (h *handler) IssueEvents(rw http.ResponseWriter, r *http.Request) {
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
	projectID, err := projectFromQuery(r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	events, err := h.module.IssueEvents(ctx, orgID, id, projectID, queryLimit(r))
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, map[string]any{"items": events})
}

// --- discover / events / logs / traces / stats ---

func (h *handler) Discover(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()
	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	req := new(sentrytypes.DiscoverRequest)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}
	out, err := h.module.Discover(ctx, orgID, req)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, out)
}

func (h *handler) GetEvent(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()
	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	projectID, err := projectFromQuery(r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	event, err := h.module.GetEvent(ctx, orgID, projectID, coretypes.Param(r, "id"))
	if err != nil {
		render.Error(rw, err)
		return
	}
	if event == nil {
		render.Error(rw, errors.Newf(errors.TypeNotFound, sentrytypes.ErrCodeSentryNotFound, "event not found"))
		return
	}
	render.Success(rw, http.StatusOK, event)
}

func (h *handler) ListLogs(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()
	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	projectID, err := projectFromQuery(r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	events, err := h.module.ListLogs(ctx, orgID, projectID, r.URL.Query().Get("query"), r.URL.Query().Get("period"), queryLimit(r))
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, map[string]any{"items": events})
}

func (h *handler) ListTraces(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()
	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	projectID, err := projectFromQuery(r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	traces, err := h.module.ListTraces(ctx, orgID, projectID, r.URL.Query().Get("period"), queryLimit(r))
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, map[string]any{"items": traces})
}

func (h *handler) GetTrace(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()
	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	projectID, err := projectFromQuery(r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	detail, err := h.module.TraceDetail(ctx, orgID, projectID, coretypes.Param(r, "id"))
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, detail)
}

func (h *handler) Stats(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()
	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	projectID, err := projectFromQuery(r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	points, err := h.module.Stats(ctx, orgID, projectID, r.URL.Query().Get("field"), r.URL.Query().Get("period"))
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, map[string]any{"items": points})
}

// --- shared helpers ---

// orgFromContext resolves the caller's org UUID from the gateway-asserted claims. A
// malformed/absent org fails closed as unauthenticated rather than panicking.
func orgFromContext(ctx context.Context) (valuer.UUID, error) {
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		return valuer.UUID{}, err
	}
	orgID, err := valuer.NewUUID(claims.OrgID)
	if err != nil {
		return valuer.UUID{}, errors.Wrapf(err, errors.TypeUnauthenticated, sentrytypes.ErrCodeSentryUnauthorized, "identity carries no valid org")
	}
	return orgID, nil
}

func idFromPath(r *http.Request) (valuer.UUID, error) {
	id, err := valuer.NewUUID(coretypes.Param(r, "id"))
	if err != nil {
		return valuer.UUID{}, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "id is not a valid uuid")
	}
	return id, nil
}

// projectFromQuery reads and validates the mandatory ?project= param for the
// project-scoped reads (logs/traces/stats). The value must be a UUID; ownership is
// enforced downstream by the org-scoped project lookup in the module.
func projectFromQuery(r *http.Request) (valuer.UUID, error) {
	id, err := valuer.NewUUID(r.URL.Query().Get("project"))
	if err != nil {
		return valuer.UUID{}, errors.Newf(errors.TypeInvalidInput, sentrytypes.ErrCodeSentryInvalidInput, "a valid project query param is required")
	}
	return id, nil
}

func queryLimit(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return n
}
