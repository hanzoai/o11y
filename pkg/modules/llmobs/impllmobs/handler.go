package impllmobs

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/http/binding"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/modules/llmobs"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/llmobstypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

const (
	viewTimeout  = 30 * time.Second
	writeTimeout = 15 * time.Second

	// headerOrgID is the gateway-asserted tenant slug (JWT `owner` / X-Org-Id) —
	// the same trusted header identn derives claims from (HIP-0026: the gateway
	// sets it and strips any client copy). It is the tenant boundary for the span
	// views, which have no other org column.
	headerOrgID = "X-Org-Id"
)

type handler struct {
	module llmobs.Module
}

func NewHandler(module llmobs.Module) llmobs.Handler {
	return &handler{module: module}
}

// --- span views ---

func (h *handler) Observations(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()

	orgID, q, err := viewRequest(ctx, r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	items, err := h.module.ListObservations(ctx, orgID, q)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, &llmobstypes.GettableObservations{Items: items, Offset: clampOffset(q.Offset), Limit: clampLimit(q.Limit)})
}

func (h *handler) Traces(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()

	orgID, q, err := viewRequest(ctx, r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	items, err := h.module.ListTraces(ctx, orgID, q)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, &llmobstypes.GettableTraces{Items: items, Offset: clampOffset(q.Offset), Limit: clampLimit(q.Limit)})
}

func (h *handler) Sessions(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()

	orgID, q, err := viewRequest(ctx, r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	items, err := h.module.ListSessions(ctx, orgID, q)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, &llmobstypes.GettableSessions{Items: items, Offset: clampOffset(q.Offset), Limit: clampLimit(q.Limit)})
}

func (h *handler) Users(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()

	orgID, q, err := viewRequest(ctx, r)
	if err != nil {
		render.Error(rw, err)
		return
	}
	items, err := h.module.ListUsers(ctx, orgID, q)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, &llmobstypes.GettableUsers{Items: items, Offset: clampOffset(q.Offset), Limit: clampLimit(q.Limit)})
}

// --- scores ---

func (h *handler) ListScores(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()

	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	var q llmobstypes.ScoresQuery
	if err := binding.Query.BindQuery(r.URL.Query(), &q); err != nil {
		render.Error(rw, err)
		return
	}
	items, total, err := h.module.ListScores(ctx, orgID, &q)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, &llmobstypes.GettableScores{Items: items, Total: total, Offset: clampOffset(q.Offset), Limit: clampLimit(q.Limit)})
}

func (h *handler) CreateScore(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), writeTimeout)
	defer cancel()

	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	req := new(llmobstypes.IngestScore)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}
	score, err := h.module.CreateScore(ctx, valuer.MustNewUUID(claims.OrgID), claims.Email, req)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusCreated, score)
}

func (h *handler) GetScore(rw http.ResponseWriter, r *http.Request) {
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
	score, err := h.module.GetScore(ctx, orgID, id)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, score)
}

func (h *handler) DeleteScore(rw http.ResponseWriter, r *http.Request) {
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
	if err := h.module.DeleteScore(ctx, orgID, id); err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusNoContent, nil)
}

// --- annotations ---

func (h *handler) Annotations(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), viewTimeout)
	defer cancel()

	orgID, err := orgFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	var q llmobstypes.AnnotationsQuery
	if err := binding.Query.BindQuery(r.URL.Query(), &q); err != nil {
		render.Error(rw, err)
		return
	}
	items, total, err := h.module.ListAnnotations(ctx, orgID, &q)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, &llmobstypes.GettableAnnotations{Items: items, Total: total, Offset: clampOffset(q.Offset), Limit: clampLimit(q.Limit)})
}

func (h *handler) CreateAnnotation(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), writeTimeout)
	defer cancel()

	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		render.Error(rw, err)
		return
	}
	req := new(llmobstypes.IngestAnnotation)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}
	annotation, err := h.module.CreateAnnotation(ctx, valuer.MustNewUUID(claims.OrgID), claims.Email, req)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusCreated, annotation)
}

// --- shared helpers ---

func viewRequest(ctx context.Context, r *http.Request) (valuer.UUID, *llmobstypes.ViewQuery, error) {
	orgID, err := orgFromContext(ctx)
	if err != nil {
		return valuer.UUID{}, nil, err
	}
	q := new(llmobstypes.ViewQuery)
	if err := binding.Query.BindQuery(r.URL.Query(), q); err != nil {
		return valuer.UUID{}, nil, err
	}
	// MANDATORY tenant scope for the span views: set the org slug SERVER-SIDE from
	// the validated X-Org-Id, AFTER binding, so no client query param can spoof it
	// (ViewQuery.OrgSlug has no `query` tag). Fail CLOSED when the gateway asserted
	// no tenant — never run an un-scoped span query that would read every org.
	orgSlug := strings.TrimSpace(r.Header.Get(headerOrgID))
	if orgSlug == "" {
		return valuer.UUID{}, nil, errors.NewUnauthenticatedf(errors.CodeUnauthenticated, "missing tenant identity for span view")
	}
	q.OrgSlug = orgSlug
	return orgID, q, nil
}

func orgFromContext(ctx context.Context) (valuer.UUID, error) {
	claims, err := authtypes.ClaimsFromContext(ctx)
	if err != nil {
		return valuer.UUID{}, err
	}
	return valuer.MustNewUUID(claims.OrgID), nil
}

func idFromPath(r *http.Request) (valuer.UUID, error) {
	id, err := valuer.NewUUID(mux.Vars(r)["id"])
	if err != nil {
		return valuer.UUID{}, errors.Wrapf(err, errors.TypeInvalidInput, llmobstypes.ErrCodeLLMObsInvalidInput, "id is not a valid uuid")
	}
	return id, nil
}
