package implsentry

import (
	"context"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// projectStore backs o11y_sentry_projects. Every method is org-scoped except Resolve
// (the DSN-authenticated ingest lookup, keyed by the unguessable project id).
type projectStore struct {
	sqlstore sqlstore.SQLStore
}

// NewProjectStore wires the relational projects store.
func NewProjectStore(sqlstore sqlstore.SQLStore) sentrytypes.ProjectStore {
	return &projectStore{sqlstore: sqlstore}
}

func (s *projectStore) Create(ctx context.Context, p *sentrytypes.Project) error {
	_, err := s.sqlstore.BunDBCtx(ctx).NewInsert().Model(p).Exec(ctx)
	if err != nil {
		return s.sqlstore.WrapAlreadyExistsErrf(err, sentrytypes.ErrCodeSentryConflict, "a project named %q already exists in the org", p.Slug)
	}
	return nil
}

func (s *projectStore) List(ctx context.Context, orgID valuer.UUID) ([]*sentrytypes.Project, error) {
	projects := make([]*sentrytypes.Project, 0)
	// MANDATORY tenant boundary, first predicate — there is no unscoped project list.
	err := s.sqlstore.BunDBCtx(ctx).
		NewSelect().
		Model(&projects).
		Where("org_id = ?", orgID).
		OrderExpr("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return projects, nil
}

func (s *projectStore) Get(ctx context.Context, orgID, id valuer.UUID) (*sentrytypes.Project, error) {
	p := new(sentrytypes.Project)
	err := s.sqlstore.BunDBCtx(ctx).
		NewSelect().
		Model(p).
		Where("org_id = ?", orgID).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, s.sqlstore.WrapNotFoundErrf(err, sentrytypes.ErrCodeSentryNotFound, "project %s not found in the org", id)
	}
	return p, nil
}

// Delete removes the project, org-scoped. Zero rows affected means it does not
// belong to the caller's org (or never existed) — reported as not-found so a caller
// can never probe another tenant's ids by watching for a different error.
//
// Retained events are deliberately left alone: they live in the columnar plane keyed
// by (org, project), and a name/naming fix must not double as a history wipe.
func (s *projectStore) Delete(ctx context.Context, orgID, id valuer.UUID) error {
	res, err := s.sqlstore.BunDBCtx(ctx).
		NewDelete().
		Model((*sentrytypes.Project)(nil)).
		Where("org_id = ?", orgID).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.Newf(errors.TypeNotFound, sentrytypes.ErrCodeSentryNotFound, "project %s not found in the org", id)
	}
	return nil
}
