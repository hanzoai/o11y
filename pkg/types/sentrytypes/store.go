package sentrytypes

import (
	"context"

	"github.com/hanzoai/o11y/pkg/valuer"
)

// ProjectStore persists o11y_sentry_projects. Every method is org-scoped EXCEPT
// Resolve, the ingest-time lookup that maps an unguessable project id to its owning
// org + key watermark (ingest carries no IAM principal — it authenticates the DSN).
type ProjectStore interface {
	Create(ctx context.Context, p *Project) error
	List(ctx context.Context, orgID valuer.UUID) ([]*Project, error)
	Get(ctx context.Context, orgID, id valuer.UUID) (*Project, error)

	// Rotate bumps the project's KeyVersion (invalidating below-watermark DSNs) and
	// returns the new version, org-scoped and idempotent-safe.
	Rotate(ctx context.Context, orgID, id valuer.UUID) (int, error)

	// Resolve maps a project id to its owning org, current key version and status —
	// the ONLY non-org-scoped read, used by the public DSN-authenticated ingest path.
	// Fail-closed: an unknown project returns found=false.
	Resolve(ctx context.Context, id valuer.UUID) (orgID valuer.UUID, keyVersion int, status ProjectStatus, found bool, err error)
}

// EventStore is the columnar events plane on the ONE datastore. Insert is the finished
// ingest sink; the reads back Discover / event detail / issue occurrences / logs /
// traces / stats. Every read takes the org (mandatory tenant boundary) and a project
// as separate, server-validated arguments — no query shape carries a client-named
// tenant.
type EventStore interface {
	// Insert writes a batch of occurrences for one (org, project). Fail-soft is the
	// caller's contract: the durable issue upsert must not depend on this write.
	Insert(ctx context.Context, orgID, projectID valuer.UUID, events []*Event) error

	// Discover runs a bounded, allowlist-checked aggregation scoped to (org, project).
	Discover(ctx context.Context, orgID, projectID valuer.UUID, req *DiscoverRequest, w Window) (*DiscoverResult, error)

	// GetEvent returns one event by id within (org, project) — a project is an
	// isolation unit, so a cross-project id in the same tenant returns not-found.
	// Not found => (nil, nil).
	GetEvent(ctx context.Context, orgID, projectID valuer.UUID, eventID string) (*Event, error)

	// ListForFingerprint returns the latest occurrences of an issue (by fingerprint)
	// within (org, project), newest first.
	ListForFingerprint(ctx context.Context, orgID, projectID valuer.UUID, fingerprint string, limit int) ([]*Event, error)

	// ListForTrace returns the (org, project)-scoped error events referencing a trace
	// id — the tenant-safe "errors in this trace" detail (the o11y_traces span plane is
	// NOT read: it has no general org column and cannot be tenant-scoped).
	ListForTrace(ctx context.Context, orgID, projectID valuer.UUID, traceID string, limit int) ([]*Event, error)

	// DistinctFingerprints returns the set of issue fingerprints seen for (org,
	// project) inside the window — the project→issue projection used to scope the
	// org-grouped issue list to a project.
	DistinctFingerprints(ctx context.Context, orgID, projectID valuer.UUID, w Window) ([]string, error)

	// ListLogs returns error-event log lines for (org, project), newest first,
	// optionally narrowed by a free-text query over message/value.
	ListLogs(ctx context.Context, orgID, projectID valuer.UUID, query string, w Window, limit int) ([]*Event, error)

	// ListTraces returns error-correlated traces for (org, project) in the window.
	ListTraces(ctx context.Context, orgID, projectID valuer.UUID, w Window, limit int) ([]*TraceSummary, error)

	// Stats returns an event-count timeseries bucketed across the window for (org,
	// project). field selects the counted subset (allowlist).
	Stats(ctx context.Context, orgID, projectID valuer.UUID, field string, w Window) ([]StatsPoint, error)
}
