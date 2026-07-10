package errortracking

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Module is the native error/crash tracking surface (Sentry-class Issues) folded
// into the o11y plane. Occurrences are OTel exception data in the telemetry store;
// this module owns the grouped-Issue lifecycle over that data and the ingest that
// normalizes Sentry-SDK reports into it.
type Module interface {
	// Ingest groups one normalized occurrence into the caller's org (resolved from
	// the DSN by the handler), upserting the issue and — fail-soft — persisting the
	// occurrence to the reused occurrence store.
	Ingest(ctx context.Context, orgID valuer.UUID, occ *errortrackingtypes.Occurrence) error

	ListIssues(ctx context.Context, orgID valuer.UUID, q *errortrackingtypes.IssuesQuery) ([]*errortrackingtypes.Issue, int, error)
	GetIssue(ctx context.Context, orgID, id valuer.UUID) (*errortrackingtypes.GettableIssue, error)
	UpdateIssue(ctx context.Context, orgID, id valuer.UUID, in *errortrackingtypes.UpdateIssue) (*errortrackingtypes.Issue, error)
}

// Handler is the HTTP surface. The ingest endpoints are PUBLIC (OpenAccess) and
// authenticate the Sentry DSN key in-handler; the read endpoints are behind the
// shared Hanzo IAM authz middleware and are org-scoped from the validated claims.
type Handler interface {
	// EnvelopeIngest accepts the modern Sentry envelope wire format
	// (POST /api/{project}/envelope/).
	EnvelopeIngest(rw http.ResponseWriter, r *http.Request)
	// StoreIngest accepts the legacy single-event wire format
	// (POST /api/{project}/store/).
	StoreIngest(rw http.ResponseWriter, r *http.Request)

	ListIssues(rw http.ResponseWriter, r *http.Request)
	GetIssue(rw http.ResponseWriter, r *http.Request)
	UpdateIssue(rw http.ResponseWriter, r *http.Request)
}
