package implsentry

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/http/binding"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/modules/errortracking/implerrortracking"
	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

const (
	viewTimeout   = 30 * time.Second
	writeTimeout  = 15 * time.Second
	ingestTimeout = 15 * time.Second
)

// eventParser turns a decoded ingest body into events; the two wire formats differ
// only here (reused engine parsers).
type eventParser func([]byte) ([]*errortrackingtypes.SentryEvent, error)

type handler struct {
	module        sentry.Module
	ingestEnabled bool
	capturePII    bool
}

// NewHandler builds the /v1/sentry HTTP surface. ingestEnabled reflects whether the
// KMS ingest secret is configured (empty => ingest fails closed 503, reads still
// work); capturePII retains end-user PII on ingest when true (default false = scrub).
func NewHandler(module sentry.Module, ingestEnabled, capturePII bool) sentry.Handler {
	return &handler{module: module, ingestEnabled: ingestEnabled, capturePII: capturePII}
}

// --- ingest (public, DSN-authenticated) ---

func (h *handler) EnvelopeIngest(rw http.ResponseWriter, r *http.Request) {
	h.ingest(rw, r, implerrortracking.ParseEnvelope)
}

func (h *handler) StoreIngest(rw http.ResponseWriter, r *http.Request) {
	h.ingest(rw, r, implerrortracking.ParseStoreBody)
}

// ingest is the shared pipeline: enabled-check → parse the project id → resolve org +
// verify the DSN key against the project watermark (fail-closed) → per-project rate
// limit → bounded read+decode → parse (event-count capped) → normalize (scrub) →
// persist to the events plane AND the issue lifecycle. Every failure fails closed and
// leaks no internal detail to the untrusted client. Reuses the errortracking engine
// verbatim for decode/parse/normalize/key-verify/rate-limit.
func (h *handler) ingest(rw http.ResponseWriter, r *http.Request, parse eventParser) {
	ctx, cancel := context.WithTimeout(r.Context(), ingestTimeout)
	defer cancel()

	if !h.ingestEnabled {
		http.Error(rw, "sentry ingest is not configured", http.StatusServiceUnavailable)
		return
	}

	projectID, err := valuer.NewUUID(mux.Vars(r)["project"])
	if err != nil {
		http.Error(rw, "invalid project", http.StatusBadRequest)
		return
	}

	orgID, ok := h.module.ResolveIngest(ctx, projectID, implerrortracking.SentryKeyFromRequest(r))
	if !ok {
		// Sentry SDKs treat 401 as "bad DSN" and drop the event (no retry storm).
		http.Error(rw, "invalid ingest key", http.StatusUnauthorized)
		return
	}

	if !h.module.RateAllow(projectID) {
		rw.Header().Set("Retry-After", "1")
		http.Error(rw, "rate limited", http.StatusTooManyRequests)
		return
	}

	r.Body = http.MaxBytesReader(rw, r.Body, implerrortracking.MaxCompressedBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(rw, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	decoded, err := implerrortracking.DecodeBody(raw, r.Header.Get("Content-Encoding"))
	if err != nil {
		http.Error(rw, "cannot decode body", http.StatusBadRequest)
		return
	}
	events, err := parse(decoded)
	if err != nil {
		http.Error(rw, "invalid payload", http.StatusBadRequest)
		return
	}

	occs := make([]*errortrackingtypes.Occurrence, 0, len(events))
	lastID := ""
	for _, ev := range events {
		occ := implerrortracking.NormalizeEvent(ev, h.capturePII)
		if occ.Fingerprint == "" {
			continue
		}
		occs = append(occs, occ)
		lastID = occ.EventID
	}

	if err := h.module.Ingest(ctx, orgID, projectID, occs); err != nil {
		http.Error(rw, "ingest failed", http.StatusInternalServerError)
		return
	}
	render.Success(rw, http.StatusOK, map[string]string{"id": lastID})
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

func (h *handler) RotateProjectKey(rw http.ResponseWriter, r *http.Request) {
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
	p, err := h.module.RotateProjectKey(ctx, orgID, id)
	if err != nil {
		render.Error(rw, err)
		return
	}
	render.Success(rw, http.StatusOK, p)
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
	event, err := h.module.GetEvent(ctx, orgID, projectID, mux.Vars(r)["id"])
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
	detail, err := h.module.TraceDetail(ctx, orgID, projectID, mux.Vars(r)["id"])
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
	id, err := valuer.NewUUID(mux.Vars(r)["id"])
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
