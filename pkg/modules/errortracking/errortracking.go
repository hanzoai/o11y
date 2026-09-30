package errortracking

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Module is the grouped-Issue lifecycle over the error facts of the event plane
// (event.fact, signal 'error'). Occurrences are those facts; this module owns only
// what cannot be derived from them: status, assignee, first/last seen, count and
// regression, keyed (org, fingerprint).
type Module interface {
	// Ingest groups a BATCH of occurrences into the org's issues: they are collapsed
	// by fingerprint and upserted in one transaction under the per-org issue ceiling,
	// bounding the write amplification of a single batch. It returns the issues the
	// batch CREATED, so a caller can announce a new failure exactly once.
	Ingest(ctx context.Context, orgID valuer.UUID, occs []*errortrackingtypes.Occurrence) ([]*errortrackingtypes.Issue, error)

	ListIssues(ctx context.Context, orgID valuer.UUID, q *errortrackingtypes.IssuesQuery) ([]*errortrackingtypes.Issue, int, error)
	GetIssue(ctx context.Context, orgID, id valuer.UUID) (*errortrackingtypes.GettableIssue, error)
	UpdateIssue(ctx context.Context, orgID, id valuer.UUID, in *errortrackingtypes.UpdateIssue) (*errortrackingtypes.Issue, error)
}

// Handler is the HTTP surface: behind the shared Hanzo IAM authz middleware and
// org-scoped from the validated claims. Errors enter through /v1/event only.
type Handler interface {
	ListIssues(rw http.ResponseWriter, r *http.Request)
	GetIssue(rw http.ResponseWriter, r *http.Request)
	UpdateIssue(rw http.ResponseWriter, r *http.Request)
}
