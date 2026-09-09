package impluser

import (
	"context"

	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/factory"
	root "github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The setter writes ONE thing: the projection row for an IAM subject, with the
// roles it holds. Its only caller is identn/iamidentn.
//
// It used to also mint invites, hash and rotate passwords, issue reset tokens,
// email people links, promote pending invites, edit and delete members. Those
// are IAM's, and they are deleted rather than left unrouted — so it no longer
// needs a tokenizer to mint anything, an emailer to send anything, an org
// setter to bootstrap anything, or delete-cleanup callbacks for a delete it
// cannot perform.
type setter struct {
	store         types.UserStore
	userRoleStore authtypes.UserRoleStore
	settings      factory.ScopedProviderSettings
	authz         authz.AuthZ
	analytics     analytics.Analytics
}

func NewSetter(store types.UserStore, providerSettings factory.ProviderSettings, authz authz.AuthZ, analytics analytics.Analytics, userRoleStore authtypes.UserRoleStore) root.Setter {
	return &setter{
		store:         store,
		userRoleStore: userRoleStore,
		settings:      factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/modules/user/impluser"),
		authz:         authz,
		analytics:     analytics,
	}
}

// CreateUser writes the row and the role grant TOGETHER.
//
// The order matters and is the reason localauthz can rehydrate: the authz grant
// (a tuple, in-process for the local provider) and the user_role rows (durable)
// describe the same fact, and only the rows survive a restart. Writing both here
// is what makes the rows a complete record of the grant rather than half of it.
func (module *setter) CreateUser(ctx context.Context, user *types.User, opts ...root.CreateUserOption) error {
	createUserOpts := root.NewCreateUserOptions(opts...)

	// Grant is idempotent, so a retry after a partial failure is safe.
	if len(createUserOpts.RoleNames) > 0 {
		err := module.authz.Grant(
			ctx,
			user.OrgID,
			createUserOpts.RoleNames,
			authtypes.MustNewSubject(coretypes.NewResourceUser(), user.ID.StringValue(), user.OrgID, nil),
		)
		if err != nil {
			return err
		}
	}

	if err := module.store.RunInTx(ctx, func(ctx context.Context) error {
		if err := module.store.CreateUser(ctx, user); err != nil {
			return err
		}

		if len(createUserOpts.RoleNames) > 0 {
			return module.createUserRoleEntries(ctx, user.OrgID, user.ID, createUserOpts.RoleNames)
		}

		return nil
	}); err != nil {
		return err
	}

	traitsOrProperties := types.NewTraitsFromUser(user)
	module.analytics.IdentifyUser(ctx, user.OrgID.String(), user.ID.String(), traitsOrProperties)
	module.analytics.TrackUser(ctx, user.OrgID.String(), user.ID.String(), "User Created", traitsOrProperties)

	return nil
}

func (module *setter) createUserRoleEntries(ctx context.Context, orgID, userID valuer.UUID, roleNames []string) error {
	roles, err := module.authz.ListByOrgIDAndNames(ctx, orgID, roleNames)
	if err != nil {
		return err
	}

	return module.userRoleStore.CreateUserRoles(ctx, authtypes.NewUserRoles(userID, roles))
}

func (module *setter) Collect(ctx context.Context, orgID valuer.UUID) (map[string]any, error) {
	stats := make(map[string]any)
	counts, err := module.store.CountByOrgIDAndStatuses(ctx, orgID, []string{types.UserStatusActive.StringValue(), types.UserStatusDeleted.StringValue(), types.UserStatusPendingInvite.StringValue()})
	if err == nil {
		stats["user.count"] = counts[types.UserStatusActive] + counts[types.UserStatusDeleted] + counts[types.UserStatusPendingInvite]
		stats["user.count.active"] = counts[types.UserStatusActive]
		stats["user.count.deleted"] = counts[types.UserStatusDeleted]
		stats["user.count.pending_invite"] = counts[types.UserStatusPendingInvite]
	}

	return stats, nil
}
