package errortrackingtypes

import (
	"context"

	"github.com/hanzoai/o11y/pkg/valuer"
)

// Store persists the one net-new table (o11y_issues). Every method is org-scoped:
// writes stamp org_id, reads filter it. There is deliberately no "list all issues"
// or cross-org accessor.
type Store interface {
	// UpsertIssue groups an occurrence into its issue: inserts the fingerprint
	// bucket on first sight, otherwise atomically bumps count/last-seen/latest
	// sample and reopens a resolved issue as a regression. Keyed by (org_id,
	// fingerprint).
	UpsertIssue(ctx context.Context, issue *Issue) error

	ListIssues(ctx context.Context, orgID valuer.UUID, q *IssuesQuery) ([]*Issue, int, error)
	GetIssue(ctx context.Context, orgID, id valuer.UUID) (*Issue, error)

	// UpdateIssue applies a lifecycle change to a single issue scoped by (org_id, id).
	UpdateIssue(ctx context.Context, issue *Issue) error
}
