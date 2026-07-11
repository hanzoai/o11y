package implerrortracking

import (
	"context"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/uptrace/bun"
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

// UpsertIssues writes a whole envelope's grouped issues in ONE transaction — the
// batch is already collapsed to one row per fingerprint (Count = occurrences in the
// batch), so a request that reports N events causes at most (distinct fingerprints)
// upserts, not N transactions. New fingerprints are admitted only while the org is
// under `ceiling`; existing issues always bump. This bounds single-request write
// amplification and caps per-org issue growth (fingerprint-explosion backpressure).
//
// Portable SET exprs: `count = count + EXCLUDED.count` and `EXCLUDED.x` behave
// identically on SQLite and PostgreSQL; the boolean/enum writes use bound
// placeholders. A separate idempotent statement reopens a RESOLVED issue on
// recurrence (an IGNORED issue stays muted). Ingest never touches `version`.
func (s *store) UpsertIssues(ctx context.Context, orgID valuer.UUID, issues []*errortrackingtypes.Issue, ceiling int) (int, error) {
	if len(issues) == 0 {
		return 0, nil
	}

	tx, err := s.sqlstore.BunDB().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	current, err := tx.NewSelect().Model((*errortrackingtypes.Issue)(nil)).Where("org_id = ?", orgID).Count(ctx)
	if err != nil {
		return 0, err
	}

	// Which of the batch's fingerprints already exist for this org (they bump even at
	// ceiling); only NEW fingerprints consume headroom.
	fps := make([]string, 0, len(issues))
	for _, iss := range issues {
		fps = append(fps, iss.Fingerprint)
	}
	existing := map[string]bool{}
	var rows []struct {
		Fingerprint string `bun:"fingerprint"`
	}
	if err := tx.NewSelect().
		Model((*errortrackingtypes.Issue)(nil)).
		Column("fingerprint").
		Where("org_id = ?", orgID).
		Where("fingerprint IN (?)", bun.In(fps)).
		Scan(ctx, &rows); err != nil {
		return 0, err
	}
	for _, r := range rows {
		existing[r.Fingerprint] = true
	}

	headroom := ceiling - current
	written := 0
	for _, issue := range issues {
		if !existing[issue.Fingerprint] {
			if headroom <= 0 {
				continue // per-org ceiling reached: drop the NEW fingerprint (backpressure)
			}
			headroom--
			existing[issue.Fingerprint] = true
		}

		if _, err := tx.NewInsert().
			Model(issue).
			On("CONFLICT (org_id, fingerprint) DO UPDATE").
			Set("count = count + EXCLUDED.count").
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
			Exec(ctx); err != nil {
			return 0, err
		}

		if _, err := tx.NewUpdate().
			Model((*errortrackingtypes.Issue)(nil)).
			Set("status = ?", errortrackingtypes.StatusUnresolved).
			Set("regressed = ?", true).
			Set("updated_at = ?", issue.LastSeen).
			Where("org_id = ?", issue.OrgID).
			Where("fingerprint = ?", issue.Fingerprint).
			Where("status = ?", errortrackingtypes.StatusResolved).
			Exec(ctx); err != nil {
			return 0, err
		}
		written++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return written, nil
}

func (s *store) ListIssues(ctx context.Context, orgID valuer.UUID, q *errortrackingtypes.IssuesQuery) ([]*errortrackingtypes.Issue, int, error) {
	issues := make([]*errortrackingtypes.Issue, 0)

	// MANDATORY tenant boundary, first predicate. Every issue row belongs to exactly
	// one org; there is no code path that lists issues without this filter.
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
	// Server-only narrowing to a set of fingerprints (the Sentry project projection).
	// Never client-settable (IssuesQuery.Fingerprints has no query tag), still under
	// the mandatory org_id filter above.
	if len(q.Fingerprints) > 0 {
		query = query.Where("fingerprint IN (?)", bun.In(q.Fingerprints))
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

// UpdateIssue writes the mutable lifecycle columns with an optimistic-concurrency
// guard: the WHERE pins the loaded version, and the write bumps it. Zero rows means
// either the row vanished (not-found) or another operator wrote first (conflict) —
// distinguished by a cheap existence probe so the caller gets the right status.
func (s *store) UpdateIssue(ctx context.Context, issue *errortrackingtypes.Issue, expectedVersion int64) error {
	res, err := s.sqlstore.
		BunDBCtx(ctx).
		NewUpdate().
		Model((*errortrackingtypes.Issue)(nil)).
		Set("status = ?", issue.Status).
		Set("assignee = ?", issue.Assignee).
		Set("resolved_at = ?", issue.ResolvedAt).
		Set("regressed = ?", issue.Regressed).
		Set("updated_at = ?", issue.UpdatedAt).
		Set("version = version + 1").
		Where("org_id = ?", issue.OrgID).
		Where("id = ?", issue.ID).
		Where("version = ?", expectedVersion).
		Exec(ctx)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		exists, _ := s.sqlstore.BunDBCtx(ctx).NewSelect().
			Model((*errortrackingtypes.Issue)(nil)).
			Where("org_id = ?", issue.OrgID).
			Where("id = ?", issue.ID).
			Exists(ctx)
		if exists {
			return errors.Newf(errors.TypeAlreadyExists, errortrackingtypes.ErrCodeErrorTrackingConflict, "issue %s was modified concurrently; reload and retry", issue.ID)
		}
		return errors.Newf(errors.TypeNotFound, errortrackingtypes.ErrCodeErrorTrackingNotFound, "issue %s not found in the org", issue.ID)
	}
	return nil
}

func (s *store) DeleteStale(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.sqlstore.
		BunDBCtx(ctx).
		NewDelete().
		Model((*errortrackingtypes.Issue)(nil)).
		Where("last_seen < ?", cutoff).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
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
