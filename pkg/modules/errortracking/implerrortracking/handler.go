package implerrortracking

import (
	"context"
	"io"
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
	viewTimeout   = 30 * time.Second
	writeTimeout  = 15 * time.Second
	ingestTimeout = 15 * time.Second

	// maxCompressedBody bounds the raw request body before decompression; the
	// decoded payload is separately bounded by maxDecodedBytes.
	maxCompressedBody = 6 << 20
)

// eventParser turns a decoded ingest body into events; the two wire formats differ
// only here (envelope framing vs a single store event).
type eventParser func([]byte) ([]*errortrackingtypes.SentryEvent, error)

type handler struct {
	module errortracking.Module
	// ingestSecret is the KMS-sourced platform ingest secret used to verify DSN
	// public keys. Empty => ingest is disabled (fail closed), reads still work.
	ingestSecret []byte
}

// NewHandler builds the HTTP surface. ingestSecret should be the KMS-synced
// platform error-ingest secret; when empty the ingest routes fail closed (503).
func NewHandler(module errortracking.Module, ingestSecret []byte) errortracking.Handler {
	return &handler{module: module, ingestSecret: ingestSecret}
}

// --- ingest (public, DSN-authenticated) ---

func (h *handler) EnvelopeIngest(rw http.ResponseWriter, r *http.Request) {
	h.ingest(rw, r, parseEnvelope)
}

func (h *handler) StoreIngest(rw http.ResponseWriter, r *http.Request) {
	h.ingest(rw, r, parseStoreBody)
}

// ingest is the shared pipeline for both wire formats: enabled-check → resolve org
// from the DSN project → verify the DSN key (constant-time) → bounded read+decode →
// parse → normalize+group each event under the resolved org. Every failure mode
// fails closed and never leaks internal detail to the untrusted client.
func (h *handler) ingest(rw http.ResponseWriter, r *http.Request, parse eventParser) {
	ctx, cancel := context.WithTimeout(r.Context(), ingestTimeout)
	defer cancel()

	if len(h.ingestSecret) == 0 {
		http.Error(rw, "error ingest is not configured", http.StatusServiceUnavailable)
		return
	}

	project := mux.Vars(r)["project_id"]
	orgID, ok := orgUUIDFromProject(project)
	if !ok {
		http.Error(rw, "missing project", http.StatusBadRequest)
		return
	}

	if !verifyKey(h.ingestSecret, project, sentryKeyFromRequest(r)) {
		// Sentry SDKs treat 401 as "bad DSN" and drop the event (no retry storm).
		http.Error(rw, "invalid ingest key", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(rw, r.Body, maxCompressedBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(rw, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	decoded, err := decodeBody(raw, r.Header.Get("Content-Encoding"))
	if err != nil {
		http.Error(rw, "cannot decode body", http.StatusBadRequest)
		return
	}

	events, err := parse(decoded)
	if err != nil {
		http.Error(rw, "invalid payload", http.StatusBadRequest)
		return
	}

	lastID := ""
	for _, ev := range events {
		occ := normalizeEvent(ev)
		if occ.Fingerprint == "" {
			continue
		}
		if err := h.module.Ingest(ctx, orgID, occ); err != nil {
			// A store failure is ours, not the client's — 500 so the SDK retries.
			http.Error(rw, "ingest failed", http.StatusInternalServerError)
			return
		}
		lastID = occ.EventID
	}

	// Sentry SDKs only require 200; echo the last event id for parity.
	render.Success(rw, http.StatusOK, map[string]string{"id": lastID})
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
		return valuer.UUID{}, errors.Wrapf(err, errors.TypeInvalidInput, errortrackingtypes.ErrCodeErrorTrackingInvalidInput, "id is not a valid uuid")
	}
	return id, nil
}
