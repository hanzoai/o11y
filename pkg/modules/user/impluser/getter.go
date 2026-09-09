package impluser

import (
	"context"
	"slices"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/featuretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

type getter struct {
	store         types.UserStore
	userRoleStore authtypes.UserRoleStore
	flagger       flagger.Flagger
}

func NewGetter(store types.UserStore, userRoleStore authtypes.UserRoleStore, flagger flagger.Flagger) user.Getter {
	return &getter{store: store, userRoleStore: userRoleStore, flagger: flagger}
}

func (module *getter) GetRootUserByOrgID(ctx context.Context, orgID valuer.UUID) (*types.User, []*authtypes.UserRole, error) {
	rootUser, err := module.store.GetRootUserByOrgID(ctx, orgID)
	if err != nil {
		return nil, nil, err
	}

	userRoles, err := module.userRoleStore.GetUserRolesByUserID(ctx, rootUser.ID)
	if err != nil {
		return nil, nil, err
	}

	return rootUser, userRoles, nil
}

func (module *getter) ListUsersByOrgID(ctx context.Context, orgID valuer.UUID) ([]*types.User, error) {
	users, err := module.store.ListUsersByOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// filter root users if feature flag `hide_root_users` is true
	evalCtx := featuretypes.NewFlaggerEvaluationContext(orgID)
	hideRootUsers := module.flagger.BooleanOrEmpty(ctx, flagger.FeatureHideRootUser, evalCtx)

	if hideRootUsers {
		users = slices.DeleteFunc(users, func(user *types.User) bool { return user.IsRoot })
	}

	return users, nil
}

func (module *getter) GetUserByOrgIDAndID(ctx context.Context, orgID valuer.UUID, userID valuer.UUID) (*types.User, error) {
	return module.store.GetByOrgIDAndID(ctx, orgID, userID)
}

func (module *getter) CountByOrgID(ctx context.Context, orgID valuer.UUID) (int64, error) {
	count, err := module.store.CountByOrgID(ctx, orgID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (module *getter) CountByOrgIDAndStatuses(ctx context.Context, orgID valuer.UUID, statuses []string) (map[valuer.String]int64, error) {
	counts, err := module.store.CountByOrgIDAndStatuses(ctx, orgID, statuses)
	if err != nil {
		return nil, err
	}

	return counts, nil
}

func (module *getter) GetUsersByOrgIDAndRoleID(ctx context.Context, orgID valuer.UUID, roleID valuer.UUID) ([]*types.User, error) {
	return module.store.GetUsersByOrgIDAndRoleID(ctx, orgID, roleID)
}

func (module *getter) OnBeforeRoleDelete(ctx context.Context, orgID valuer.UUID, roleID valuer.UUID, _ string) error {
	users, err := module.GetUsersByOrgIDAndRoleID(ctx, orgID, roleID)
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return errors.New(errors.TypeInvalidInput, authtypes.ErrCodeRoleHasUserAssignees, "role has active user assignments, remove them before deleting")
	}
	return nil
}
