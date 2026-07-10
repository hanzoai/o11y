package implerrortracking

import (
	"context"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

const (
	defaultIssueLimit = 50
	maxIssueLimit     = 100
)

type store struct {
	sqlstore sqlstore.SQLStore
}

// NewStore backs the o11y_issues lifecycle table. Every method is org-scoped.
func NewStore(sqlstore sqlstore.SQLStore) errortrackingtypes.Store {
	return &store{sqlstore: sqlstore}
}

// UpsertIssue inserts the fingerprint bucket on first sight, otherwise atomically
// bumps the running count, advances last-seen and refreshes the display fields +
// latest sample. First-seen and lifecycle status are preserved on update. A
// separate, idempotent statement reopens a RESOLVED issue as a regression on
// recurrence (an IGNORED issue stays muted). Both run in one transaction.
//
// The SET expressions are dialect-portable: `count = count + 1` and `EXCLUDED.x`
// resolve identically on SQLite and PostgreSQL, and the boolean/en'um writes go
// through bound placeholders — no dialect-specific literals.
func (s *store) UpsertIssue(ctx context.Context, issue *errortrackingtypes.Issue) error {
	tx, err := s.sqlstore.BunDB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.NewInsert().
		Model(issue).
		On("CONFLICT (org_id, fingerprint) DO UPDATE").
		Set("count = count + 1").
		Set("last_seen = EXCLUDED.last_seen").
		Set("value = EXCLUDED.value").
		Set("level = EXCLUDED.level").
		Set("culprit = EXCLUDED.culprit").
		Set("platform = EXCLUDED.platform").
		Set("environment = EXCLUDED.environment").
		Set("release = EXCLUDED.release").
		Set("service_name = EXCLUDED.service_name").
		Set("sample_event = EXCLUDED.sample_event").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	if err != nil {
		return err
	}

	if _, err = tx.NewUpdate().
		Model((*errortrackingtypes.Issue)(nil)).
		Set("status = ?", errortrackingtypes.StatusUnresolved).
		Set("regressed = ?", true).
		Set("updated_at = ?", issue.LastSeen).
		Where("org_id = ?", issue.OrgID).
		Where("fingerprint = ?", issue.Fingerprint).
		Where("status = ?", errortrackingtypes.StatusResolved).
		Exec(ctx); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *store) ListIssues(ctx context.Context, orgID valuer.UUID, q *errortrackingtypes.IssuesQuery) ([]*errortrackingtypes.Issue, int, error) {
	issues := make([]*errortrackingtypes.Issue, 0)

	// MANDATORY tenant boundary, first predicate. Every issue row belongs to exactly
	// one org; org_id is the o11y org UUID that both ingest (from the DSN) and this
	// read (from the validated claims) resolve to. There is no code path that lists
	// issues without this filter.
	query := s.sqlstore.
		BunDBCtx(ctx).
		NewSelect().
		Model(&issues).
		Where("org_id = ?", orgID)

	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.Level != "" {
		query = query.Where("level = ?", q.Level)
	}
	if q.Environment != "" {
		query = query.Where("environment = ?", q.Environment)
	}
	if q.ServiceName != "" {
		query = query.Where("service_name = ?", q.ServiceName)
	}
	if q.Query != "" {
		like := "%" + q.Query + "%"
		query = query.Where("(type LIKE ? OR value LIKE ? OR culprit LIKE ?)", like, like, like)
	}

	count, err := query.
		OrderExpr(sortColumn(q.Sort)).
		Offset(clampOffset(q.Offset)).
		Limit(clampLimit(q.Limit)).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	return issues, count, nil
}

func (s *store) GetIssue(ctx context.Context, orgID, id valuer.UUID) (*errortrackingtypes.Issue, error) {
	issue := new(errortrackingtypes.Issue)
	err := s.sqlstore.
		BunDBCtx(ctx).
		NewSelect().
		Model(issue).
		Where("org_id = ?", orgID).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, s.sqlstore.WrapNotFoundErrf(err, errortrackingtypes.ErrCodeErrorTrackingNotFound, "issue %s not found in the org", id)
	}
	return issue, nil
}

// UpdateIssue writes the mutable lifecycle columns for one issue, scoped by
// (org_id, id). The caller has already loaded the issue for this org, so a
// zero-row result means it vanished (racy delete) — reported as not-found.
func (s *store) UpdateIssue(ctx context.Context, issue *errortrackingtypes.Issue) error {
	res, err := s.sqlstore.
		BunDBCtx(ctx).
		NewUpdate().
		Model(issue).
		Column("status", "assignee", "resolved_at", "regressed", "updated_at").
		Where("org_id = ?", issue.OrgID).
		Where("id = ?", issue.ID).
		Exec(ctx)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.Newf(errors.TypeNotFound, errortrackingtypes.ErrCodeErrorTrackingNotFound, "issue %s not found in the org", issue.ID)
	}
	return nil
}

// sortColumn maps the API sort key to a safe ORDER BY expression (never user text).
func sortColumn(sort string) string {
	switch sort {
	case "firstSeen":
		return "first_seen DESC"
	case "count":
		return "count DESC"
	default:
		return "last_seen DESC"
	}
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultIssueLimit
	}
	if limit > maxIssueLimit {
		return maxIssueLimit
	}
	return limit
}

func clampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

// nowUTC is the single time source for lifecycle writes (overridable in tests).
var nowUTC = func() time.Time { return time.Now().UTC() }
