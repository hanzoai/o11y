package implsentry

import (
	"context"
	"database/sql"
	stderrors "errors"

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

// Rotate bumps the project's key watermark (invalidating below-version DSNs). The
// bump is org-scoped and the loaded row's version drives the returned new version;
// zero rows affected means the project does not belong to the caller's org.
func (s *projectStore) Rotate(ctx context.Context, orgID, id valuer.UUID) (int, error) {
	p, err := s.Get(ctx, orgID, id)
	if err != nil {
		return 0, err
	}
	res, err := s.sqlstore.BunDBCtx(ctx).
		NewUpdate().
		Model((*sentrytypes.Project)(nil)).
		Set("key_version = key_version + 1").
		Set("updated_at = ?", nowUTC()).
		Where("org_id = ?", orgID).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, errors.Newf(errors.TypeNotFound, sentrytypes.ErrCodeSentryNotFound, "project %s not found in the org", id)
	}
	return p.KeyVersion + 1, nil
}

// Resolve is the ingest-time lookup: it maps a project id to its owning org, current
// key version and status WITHOUT an org filter (ingest carries no IAM principal — the
// DSN key is the credential, verified by the caller). Fail-closed: an unknown project
// returns found=false, so a forged/unknown DSN never resolves to a tenant.
func (s *projectStore) Resolve(ctx context.Context, id valuer.UUID) (valuer.UUID, int, sentrytypes.ProjectStatus, bool, error) {
	p := new(sentrytypes.Project)
	err := s.sqlstore.BunDB().
		NewSelect().
		Model(p).
		Column("org_id", "key_version", "status").
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return valuer.UUID{}, 0, "", false, nil // fail-closed: unknown project never resolves
		}
		return valuer.UUID{}, 0, "", false, err
	}
	return p.OrgID, p.KeyVersion, p.Status, true, nil
}
