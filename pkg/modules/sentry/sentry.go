// Package sentry is Sentinel — the Sentry-parity error/log/trace product face under
// /v1/o11y/sentinel. It is a COMPOSITION, not a refork: errors enter through /v1/event
// only and land on the ONE event plane (event.fact, signal 'error'); a consumer of the
// event.error subject hands each batch to Ingest, which keeps the grouped-issue
// lifecycle (o11y_issues, reused verbatim); identity is Hanzo IAM. This package owns
// only the product surface: projects, the reads over the error facts, and the query
// shapes that give Discover / logs / traces / stats their Sentry semantics.
package sentry

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Module is the org-scoped business surface. Ingest is the only method that takes
// the plane's names (an org slug and a product) rather than ids: its caller is the
// event.error consumer, which reads them off the fact. Every other method is scoped to
// the caller's org and validates any project against it.
type Module interface {
	// Ingest files one (org, product)'s error facts into the org's issues and returns
	// the issues it created. An empty product files the issues and lists no project. The facts are already on the event plane; this writes the
	// lifecycle only. Each occurrence's Fingerprint is the fact's `issue`.
	Ingest(ctx context.Context, org, product string, occs []*errortrackingtypes.Occurrence) ([]*errortrackingtypes.Issue, error)

	// Projects — org-scoped CRUD. A project is a product: its slug is the product
	// name the event plane stores, and Ingest creates one on a product's first error.
	CreateProject(ctx context.Context, orgID valuer.UUID, in *sentrytypes.PostableProject) (*sentrytypes.GettableProject, error)
	ListProjects(ctx context.Context, orgID valuer.UUID) (*sentrytypes.GettableProjects, error)
	GetProject(ctx context.Context, orgID, id valuer.UUID) (*sentrytypes.GettableProject, error)

	// DeleteProject removes a project. Retained events are not touched — deleting a
	// project must not double as a history wipe.
	DeleteProject(ctx context.Context, orgID, id valuer.UUID) error

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

// Handler is the HTTP surface of Sentinel: the product face under /v1/o11y/sentinel,
// behind Hanzo IAM authz and org-scoped from the validated claims.
type Handler interface {
	// Projects.
	ListProjects(http.ResponseWriter, *http.Request)
	CreateProject(http.ResponseWriter, *http.Request)
	GetProject(http.ResponseWriter, *http.Request)
	DeleteProject(http.ResponseWriter, *http.Request)

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
