package errortrackingtypes

import (
	"context"
	"time"

	"github.com/hanzoai/o11y/pkg/valuer"
)

// Store persists the one net-new table (o11y_issues). Every method is org-scoped:
// writes stamp org_id, reads filter it. There is deliberately no "list all issues"
// or cross-org accessor.
type Store interface {
	// UpsertIssues groups a batch of occurrences (already collapsed to one Issue per
	// fingerprint, with Count = occurrences-in-batch) in ONE transaction. New
	// fingerprints are admitted only while the org is under `ceiling` (the per-org
	// issue cap — backpressure against a fingerprint-explosion DoS); existing issues
	// always bump count/last-seen and reopen-on-regression. Returns issues written.
	UpsertIssues(ctx context.Context, orgID valuer.UUID, issues []*Issue, ceiling int) (int, error)

	ListIssues(ctx context.Context, orgID valuer.UUID, q *IssuesQuery) ([]*Issue, int, error)
	GetIssue(ctx context.Context, orgID, id valuer.UUID) (*Issue, error)

	// UpdateIssue applies a lifecycle change to one issue scoped by (org_id, id) with
	// OPTIMISTIC concurrency: it writes only when the row's version still equals
	// expectedVersion, bumping it. A stale version is a conflict, not a silent clobber.
	UpdateIssue(ctx context.Context, issue *Issue, expectedVersion int64) error

	// DeleteStale purges issues whose last_seen predates cutoff (retention/TTL sweep).
	DeleteStale(ctx context.Context, cutoff time.Time) (int64, error)
}
