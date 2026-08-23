// Package sentry is Hanzo Sentry — the Sentry-parity error/log/trace product face
// under /v1/o11y/sentinel. It is a COMPOSITION, not a refork: the ingest engine (envelope
// parse, fingerprint, DSN verify, scrub, rate-limit) is the reused errortracking
// engine; identity is Hanzo IAM; storage is the ONE datastore (columnar events) plus
// o11y_issues (grouped-issue lifecycle, reused verbatim). This package owns only the
// product surface: projects, the events plane, and the read/query shapes that give
// Discover / logs / traces / stats their Sentry semantics.
package sentry

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Module is the org-scoped business surface. Ingest is the only method that takes a
// project (resolved from the DSN, no principal); every other method is scoped to the
// caller's org and validates any project against it.
type Module interface {
	// Ingest persists a request's occurrences for (org, project): the columnar events
	// plane (Insert) AND the grouped-issue lifecycle (reused errortracking upsert).
	Ingest(ctx context.Context, orgID, projectID valuer.UUID, occs []*errortrackingtypes.Occurrence) error

	// Projects — org-scoped CRUD + DSN rotation. Create/Get/List/Rotate all stamp or
	// filter org_id; the DSN is derived, never stored.
	CreateProject(ctx context.Context, orgID valuer.UUID, in *sentrytypes.PostableProject) (*sentrytypes.GettableProject, error)
	ListProjects(ctx context.Context, orgID valuer.UUID) (*sentrytypes.GettableProjects, error)
	GetProject(ctx context.Context, orgID, id valuer.UUID) (*sentrytypes.GettableProject, error)
	RotateProjectKey(ctx context.Context, orgID, id valuer.UUID) (*sentrytypes.GettableProject, error)

	// DeleteProject removes a project. Its DSN stops resolving immediately, so ingest
	// for that id fails closed exactly as an unknown project does. Retained events are
	// not touched — deleting a project must not double as a history wipe.
	DeleteProject(ctx context.Context, orgID, id valuer.UUID) error

	// ResolveIngest maps a DSN project id to its owning org, verifying the presented
	// DSN key against the project's rotation watermark. Fail-closed: an unknown,
	// disabled or below-watermark project/key returns ok=false.
	ResolveIngest(ctx context.Context, projectID valuer.UUID, presentedKey string) (orgID valuer.UUID, ok bool)

	// RateAllow reports whether the project is within its ingest rate budget.
	RateAllow(projectID valuer.UUID) bool

	// Issues — reused errortracking lifecycle, org-scoped, optionally narrowed to a
	// project via the events-plane fingerprint projection.
	ListIssues(ctx context.Context, orgID valuer.UUID, projectID *valuer.UUID, q *errortrackingtypes.IssuesQuery, w sentrytypes.Window) (*errortrackingtypes.GettableIssues, error)
	GetIssue(ctx context.Context, orgID, id valuer.UUID) (*errortrackingtypes.GettableIssue, error)
	UpdateIssue(ctx context.Context, orgID, id valuer.UUID, in *errortrackingtypes.UpdateIssue) (*errortrackingtypes.Issue, error)
	// IssueEvents lists an issue's occurrences scoped to (org, project) — a project is
	// an isolation unit, so the caller declares which project's occurrences to read.
	IssueEvents(ctx context.Context, orgID, id, projectID valuer.UUID, limit int) ([]*sentrytypes.Event, error)

	// Discover / event detail / logs / traces / stats — all over the events plane.
	Discover(ctx context.Context, orgID valuer.UUID, req *sentrytypes.DiscoverRequest) (*sentrytypes.DiscoverResult, error)
	GetEvent(ctx context.Context, orgID, projectID valuer.UUID, eventID string) (*sentrytypes.Event, error)
	ListLogs(ctx context.Context, orgID valuer.UUID, projectID valuer.UUID, query, period string, limit int) ([]*sentrytypes.Event, error)
	ListTraces(ctx context.Context, orgID valuer.UUID, projectID valuer.UUID, period string, limit int) ([]*sentrytypes.TraceSummary, error)
	TraceDetail(ctx context.Context, orgID, projectID valuer.UUID, traceID string) (any, error)
	Stats(ctx context.Context, orgID, projectID valuer.UUID, field, period string) ([]sentrytypes.StatsPoint, error)
}

// Handler is the HTTP surface of Hanzo Sentry: the product face under /v1/o11y/sentinel,
// behind Hanzo IAM authz and org-scoped from the validated claims, plus the ingest
// endpoint under /v1/event, which is PUBLIC and DSN-authenticated in-handler.
type Handler interface {
	// Ingest (public, DSN-auth): POST /v1/event/{project}/envelope|store/.
	EnvelopeIngest(http.ResponseWriter, *http.Request)
	StoreIngest(http.ResponseWriter, *http.Request)

	// Projects.
	ListProjects(http.ResponseWriter, *http.Request)
	CreateProject(http.ResponseWriter, *http.Request)
	GetProject(http.ResponseWriter, *http.Request)
	DeleteProject(http.ResponseWriter, *http.Request)
	RotateProjectKey(http.ResponseWriter, *http.Request)

	// Issues.
	ListIssues(http.ResponseWriter, *http.Request)
	GetIssue(http.ResponseWriter, *http.Request)
	UpdateIssue(http.ResponseWriter, *http.Request)
	IssueEvents(http.ResponseWriter, *http.Request)

	// Discover / events / logs / traces / stats.
	Discover(http.ResponseWriter, *http.Request)
	GetEvent(http.ResponseWriter, *http.Request)
	ListLogs(http.ResponseWriter, *http.Request)
	ListTraces(http.ResponseWriter, *http.Request)
	GetTrace(http.ResponseWriter, *http.Request)
	Stats(http.ResponseWriter, *http.Request)
}
