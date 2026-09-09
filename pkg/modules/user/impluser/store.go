package impluser

import (
	"context"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/uptrace/bun"
)

type store struct {
	sqlstore sqlstore.SQLStore
	settings factory.ProviderSettings
}

func NewStore(sqlstore sqlstore.SQLStore, settings factory.ProviderSettings) types.UserStore {
	return &store{sqlstore: sqlstore, settings: settings}
}

func (store *store) CreateUser(ctx context.Context, user *types.User) error {
	_, err := store.
		sqlstore.
		BunDBCtx(ctx).
		NewInsert().
		Model(user).
		Exec(ctx)
	if err != nil {
		return store.sqlstore.WrapAlreadyExistsErrf(err, types.ErrUserAlreadyExists, "user with email %s already exists in org %s", user.Email, user.OrgID)
	}
	return nil
}

func (store *store) GetByOrgIDAndID(ctx context.Context, orgID valuer.UUID, id valuer.UUID) (*types.User, error) {
	user := new(types.User)

	err := store.
		sqlstore.
		BunDBCtx(ctx).
		NewSelect().
		Model(user).
		Where("org_id = ?", orgID).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, store.sqlstore.WrapNotFoundErrf(err, types.ErrCodeUserNotFound, "user with id %s does not exist", id)
	}

	return user, nil
}

func (store *store) ListUsersByOrgID(ctx context.Context, orgID valuer.UUID) ([]*types.User, error) {
	users := []*types.User{}

	err := store.
		sqlstore.
		BunDBCtx(ctx).
		NewSelect().
		Model(&users).
		Where("org_id = ?", orgID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (store *store) CountByOrgID(ctx context.Context, orgID valuer.UUID) (int64, error) {
	user := new(types.User)

	count, err := store.
		sqlstore.
		BunDB().
		NewSelect().
		Model(user).
		Where("org_id = ?", orgID).
		Count(ctx)
	if err != nil {
		return 0, err
	}

	return int64(count), nil
}

func (store *store) CountByOrgIDAndStatuses(ctx context.Context, orgID valuer.UUID, statuses []string) (map[valuer.String]int64, error) {
	user := new(types.User)
	var results []struct {
		Status valuer.String `bun:"status"`
		Count  int64         `bun:"count"`
	}

	err := store.
		sqlstore.
		BunDBCtx(ctx).
		NewSelect().
		Model(user).
		ColumnExpr("status").
		ColumnExpr("COUNT(*) AS count").
		Where("org_id = ?", orgID.StringValue()).
		Where("status IN (?)", bun.In(statuses)).
		GroupExpr("status").
		Scan(ctx, &results)
	if err != nil {
		return nil, err
	}

	counts := make(map[valuer.String]int64, len(results))
	for _, r := range results {
		counts[r.Status] = r.Count
	}

	return counts, nil
}

func (store *store) RunInTx(ctx context.Context, cb func(ctx context.Context) error) error {
	return store.sqlstore.RunInTxCtx(ctx, nil, func(ctx context.Context) error {
		return cb(ctx)
	})
}

func (store *store) GetRootUserByOrgID(ctx context.Context, orgID valuer.UUID) (*types.User, error) {
	user := new(types.User)
	err := store.
		sqlstore.
		BunDBCtx(ctx).
		NewSelect().
		Model(user).
		Where("org_id = ?", orgID).
		Where("is_root = ?", true).
		Scan(ctx)
	if err != nil {
		return nil, store.sqlstore.WrapNotFoundErrf(err, types.ErrCodeUserNotFound, "root user for org %s not found", orgID)
	}
	return user, nil
}

func (store *store) GetUsersByOrgIDAndRoleID(ctx context.Context, orgID valuer.UUID, roleID valuer.UUID) ([]*types.User, error) {
	users := []*types.User{}

	err := store.
		sqlstore.
		BunDBCtx(ctx).
		NewSelect().
		Model(&users).
		Join(`JOIN user_role ON user_role.user_id = "users".id`).
		Where(`"users".org_id = ?`, orgID).
		Where("user_role.role_id = ?", roleID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return users, nil
}
