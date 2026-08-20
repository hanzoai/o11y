package errortracking

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Module owns the grouped-Issue lifecycle over captured errors: the mutable state per
// (org, group) that cannot be derived from the facts themselves — status, assignee,
// counts, regression. The facts are event.error rows.
//
// It does not accept wire traffic. Errors enter through the ONE ingest face,
// /v1/sentry, which writes event.error and then calls Ingest here to fold the batch
// into its issues. One writer per table.
type Module interface {
	// Ingest groups a BATCH of normalized occurrences into the caller's org (resolved
	// from the DSN by the Sentry ingest handler): occurrences are collapsed by
	// fingerprint and upserted in one transaction under the per-org issue ceiling,
	// bounding the write amplification of a single request. Returns issues written.
	Ingest(ctx context.Context, orgID valuer.UUID, occs []*errortrackingtypes.Occurrence) (int, error)

	ListIssues(ctx context.Context, orgID valuer.UUID, q *errortrackingtypes.IssuesQuery) ([]*errortrackingtypes.Issue, int, error)
	GetIssue(ctx context.Context, orgID, id valuer.UUID) (*errortrackingtypes.GettableIssue, error)
	UpdateIssue(ctx context.Context, orgID, id valuer.UUID, in *errortrackingtypes.UpdateIssue) (*errortrackingtypes.Issue, error)
}

// Handler is the read surface: the Issues list/detail/update the console Errors tab
// consumes. Every endpoint is behind the shared Hanzo IAM authz middleware and is
// org-scoped from the validated claims.
type Handler interface {
	ListIssues(rw http.ResponseWriter, r *http.Request)
	GetIssue(rw http.ResponseWriter, r *http.Request)
	UpdateIssue(rw http.ResponseWriter, r *http.Request)
}
